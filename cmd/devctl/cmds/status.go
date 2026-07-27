package cmds

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/go-go-golems/devctl/pkg/operator"
	"github.com/go-go-golems/devctl/pkg/runstate"
	glazedcmds "github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/fields"
	"github.com/go-go-golems/glazed/pkg/cmds/schema"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	"github.com/go-go-golems/glazed/pkg/middlewares"
	"github.com/go-go-golems/glazed/pkg/types"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

type StatusCommand struct {
	*glazedcmds.CommandDescription
}

type StatusSettings struct {
	Services []string `glazed:"services"`
}

var _ glazedcmds.GlazeCommand = (*StatusCommand)(nil)
var _ glazedcmds.BareCommand = (*StatusCommand)(nil)

func NewStatusCommand() (*StatusCommand, error) {
	repoSection, err := getRepoLayer()
	if err != nil {
		return nil, err
	}
	return &StatusCommand{CommandDescription: glazedcmds.NewCommandDescription(
		"status",
		glazedcmds.WithShort("Show durable service status"),
		glazedcmds.WithArguments(
			fields.New("services", fields.TypeStringList, fields.WithHelp("Service names; empty selects all")),
		),
		glazedcmds.WithSections(repoSection),
	)}, nil
}

func (c *StatusCommand) RunIntoGlazeProcessor(
	ctx context.Context,
	vals *values.Values,
	processor middlewares.Processor,
) error {
	result, err := queryStatus(ctx, vals)
	if err != nil {
		return err
	}
	return addStatusRows(ctx, processor, result)
}

type statusResult struct {
	Snapshot operator.Snapshot
	Services []operator.ServiceSnapshot
	Now      time.Time
}

func (c *StatusCommand) Run(ctx context.Context, vals *values.Values) error {
	result, err := queryStatus(ctx, vals)
	if err != nil {
		return err
	}
	return renderHumanStatus(result)
}

func queryStatus(ctx context.Context, vals *values.Values) (statusResult, error) {
	settings := StatusSettings{}
	if err := vals.DecodeSectionInto(schema.DefaultSlug, &settings); err != nil {
		return statusResult{}, errors.Wrap(err, "decode status settings")
	}
	repositoryContext, err := RepoContextFromParsedLayers(vals)
	if err != nil {
		return statusResult{}, err
	}
	controller, err := newOperatorController(repositoryContext.RepoRoot)
	if err != nil {
		return statusResult{}, err
	}
	snapshot, err := controller.Snapshot(ctx, operator.SnapshotRequest{
		RepoRoot: repositoryContext.RepoRoot, IncludeRuns: true, IncludeHealth: true,
	})
	if err != nil {
		return statusResult{}, err
	}
	if !snapshot.Exists {
		if len(settings.Services) > 0 {
			return statusResult{}, &operator.OperatorError{
				Code:    operator.CodeServiceUnknown,
				Message: "service is not present because no environment state exists",
				Service: settings.Services[0],
			}
		}
		return statusResult{Snapshot: snapshot, Now: time.Now().UTC()}, nil
	}
	selected := stringSelection(settings.Services)
	found := map[string]bool{}
	services := make([]operator.ServiceSnapshot, 0, len(snapshot.Services))
	for _, service := range snapshot.Services {
		if len(selected) > 0 && !selected[service.Service] {
			continue
		}
		found[service.Service] = true
		services = append(services, service)
	}
	for service := range selected {
		if !found[service] {
			return statusResult{}, &operator.OperatorError{
				Code: operator.CodeServiceUnknown, Message: "service is not present in environment state", Service: service,
			}
		}
	}
	return statusResult{Snapshot: snapshot, Services: services, Now: time.Now().UTC()}, nil
}

func addStatusRows(ctx context.Context, processor middlewares.Processor, result statusResult) error {
	if !result.Snapshot.Exists {
		return processor.AddRow(ctx, types.NewRow(
			types.MRP("environment", "stopped"),
			types.MRP("profile", ""),
			types.MRP("service", ""),
			types.MRP("desired", "stopped"),
		))
	}
	for _, service := range result.Services {
		row := types.NewRow(
			types.MRP("environment", "present"),
			types.MRP("profile", result.Snapshot.Profile),
			types.MRP("service", service.Service),
			types.MRP("desired", string(service.Desired)),
			types.MRP("phase", string(service.Phase)),
			types.MRP("run_id", service.RunID),
			types.MRP("wrapper_pid", processPID(service.Wrapper)),
			types.MRP("child_pid", processPID(service.Child)),
			types.MRP("health", healthStatus(service.Health)),
			types.MRP("started_at", service.CreatedAt),
			types.MRP("uptime", uptime(result.Now, service).String()),
			types.MRP("exit_code", exitCode(service.Exit)),
			types.MRP("signal", exitSignal(service.Exit)),
			types.MRP("last_error_code", lastErrorCode(service.LastError)),
			types.MRP("stdout_path", service.StdoutPath),
			types.MRP("stderr_path", service.StderrPath),
		)
		if err := processor.AddRow(ctx, row); err != nil {
			return err
		}
	}
	return nil
}

func renderHumanStatus(result statusResult) error {
	if !result.Snapshot.Exists {
		_, err := fmt.Fprintln(os.Stdout, "Environment stopped\n\nNo durable service state exists.")
		return err
	}
	running, unhealthy := 0, 0
	for _, service := range result.Services {
		if service.Phase == runstate.RunReady || service.Phase == runstate.RunStarting {
			running++
		}
		if service.Health != nil && !service.Health.Healthy {
			unhealthy++
		}
	}
	if _, err := fmt.Fprintf(os.Stdout, "Environment  %-10s Services %d   Running %d   Unhealthy %d\n\n", result.Snapshot.Profile, len(result.Services), running, unhealthy); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(os.Stdout, "SERVICE          DESIRED   STATE      HEALTH      PID     UPTIME"); err != nil {
		return err
	}
	for _, service := range result.Services {
		if _, err := fmt.Fprintf(os.Stdout, "%-16s %-9s %-10s %-11s %-7s %s\n",
			service.Service, service.Desired, service.Phase, healthStatus(service.Health),
			humanPID(service.Child), humanUptime(result.Now, service)); err != nil {
			return err
		}
		if service.LastError != nil {
			if _, err := fmt.Fprintf(os.Stdout, "  error: %s: %s\n", service.LastError.Code, service.LastError.Message); err != nil {
				return err
			}
		}
		if service.Exit != nil {
			parts := []string{}
			if service.Exit.ExitCode != nil {
				parts = append(parts, fmt.Sprintf("code %d", *service.Exit.ExitCode))
			}
			if service.Exit.Signal != "" {
				parts = append(parts, "signal "+service.Exit.Signal)
			}
			if len(parts) > 0 {
				if _, err := fmt.Fprintf(os.Stdout, "  exit: %s\n", strings.Join(parts, ", ")); err != nil {
					return err
				}
			}
		}
		if service.StdoutPath != "" {
			_, _ = fmt.Fprintf(os.Stdout, "  stdout: %s\n", service.StdoutPath)
		}
		if service.StderrPath != "" {
			_, _ = fmt.Fprintf(os.Stdout, "  stderr: %s\n", service.StderrPath)
		}
	}
	return nil
}

func humanPID(identity *runstate.ProcessIdentity) string {
	if identity == nil {
		return "-"
	}
	return fmt.Sprint(identity.PID)
}

func humanUptime(now time.Time, service operator.ServiceSnapshot) string {
	if service.CreatedAt.IsZero() || service.Phase == runstate.RunExited || service.Phase == runstate.RunFailed {
		return "-"
	}
	return uptime(now, service).Round(time.Second).String()
}

func stringSelection(values []string) map[string]bool {
	selected := make(map[string]bool, len(values))
	for _, value := range values {
		selected[value] = true
	}
	return selected
}

func newStatusCmd() *cobra.Command {
	command, err := NewStatusCommand()
	cobra.CheckErr(err)
	return buildDualGlazedCommand(command)
}
