package cmds

import (
	"context"
	"time"

	"github.com/go-go-golems/devctl/pkg/operator"
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
	settings := StatusSettings{}
	if err := vals.DecodeSectionInto(schema.DefaultSlug, &settings); err != nil {
		return errors.Wrap(err, "decode status settings")
	}
	repositoryContext, err := RepoContextFromParsedLayers(vals)
	if err != nil {
		return err
	}
	controller, err := newOperatorController(repositoryContext.RepoRoot)
	if err != nil {
		return err
	}
	snapshot, err := controller.Snapshot(ctx, operator.SnapshotRequest{
		RepoRoot: repositoryContext.RepoRoot, IncludeRuns: true, IncludeHealth: true,
	})
	if err != nil {
		return err
	}
	if !snapshot.Exists {
		if len(settings.Services) > 0 {
			return &operator.OperatorError{
				Code:    operator.CodeServiceUnknown,
				Message: "service is not present because no environment state exists",
				Service: settings.Services[0],
			}
		}
		return processor.AddRow(ctx, types.NewRow(
			types.MRP("environment", "stopped"),
			types.MRP("profile", ""),
			types.MRP("service", ""),
			types.MRP("desired", "stopped"),
		))
	}
	selected := stringSelection(settings.Services)
	now := time.Now().UTC()
	found := map[string]bool{}
	for _, service := range snapshot.Services {
		if len(selected) > 0 && !selected[service.Service] {
			continue
		}
		found[service.Service] = true
		row := types.NewRow(
			types.MRP("environment", "present"),
			types.MRP("profile", snapshot.Profile),
			types.MRP("service", service.Service),
			types.MRP("desired", string(service.Desired)),
			types.MRP("phase", string(service.Phase)),
			types.MRP("run_id", service.RunID),
			types.MRP("wrapper_pid", processPID(service.Wrapper)),
			types.MRP("child_pid", processPID(service.Child)),
			types.MRP("health", healthStatus(service.Health)),
			types.MRP("started_at", service.CreatedAt),
			types.MRP("uptime", uptime(now, service).String()),
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
	for service := range selected {
		if !found[service] {
			return &operator.OperatorError{
				Code:    operator.CodeServiceUnknown,
				Message: "service is not present in environment state",
				Service: service,
			}
		}
	}
	return nil
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
	return buildGlazedCommand(command)
}
