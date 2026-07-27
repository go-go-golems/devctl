package cmds

import (
	"context"
	stderrors "errors"
	"io"
	"os"
	"time"

	"github.com/go-go-golems/devctl/pkg/operator"
	"github.com/go-go-golems/devctl/pkg/runlog"
	"github.com/go-go-golems/devctl/pkg/runstate"
	"github.com/go-go-golems/glazed/pkg/cli"
	glazedcmds "github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/fields"
	"github.com/go-go-golems/glazed/pkg/cmds/schema"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	"github.com/go-go-golems/glazed/pkg/middlewares"
	glazedsettings "github.com/go-go-golems/glazed/pkg/settings"
	"github.com/go-go-golems/glazed/pkg/types"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

type LifecycleCommand struct {
	*glazedcmds.CommandDescription
	kind string
}

type LifecycleSettings struct {
	Services     []string `glazed:"services"`
	SkipValidate bool     `glazed:"skip-validate"`
	SkipBuild    bool     `glazed:"skip-build"`
	SkipPrepare  bool     `glazed:"skip-prepare"`
	BuildSteps   []string `glazed:"build-step"`
	PrepareSteps []string `glazed:"prepare-step"`
}

var _ glazedcmds.GlazeCommand = (*LifecycleCommand)(nil)

func NewLifecycleCommand(kind string) (*LifecycleCommand, error) {
	repoSection, err := getRepoLayer()
	if err != nil {
		return nil, err
	}
	short := map[string]string{
		"up":      "Start all or selected services",
		"down":    "Stop all or selected services",
		"restart": "Restart all or selected services",
	}[kind]
	if short == "" {
		return nil, errors.Errorf("unknown lifecycle command %q", kind)
	}
	options := []glazedcmds.CommandDescriptionOption{
		glazedcmds.WithShort(short),
		glazedcmds.WithArguments(
			fields.New("services", fields.TypeStringList, fields.WithHelp("Service names; empty selects all")),
		),
		glazedcmds.WithSections(repoSection),
	}
	if kind != "down" {
		options = append(options, glazedcmds.WithFlags(
			fields.New("skip-validate", fields.TypeBool, fields.WithDefault(false), fields.WithHelp("Skip validation")),
			fields.New("skip-build", fields.TypeBool, fields.WithDefault(false), fields.WithHelp("Skip build")),
			fields.New("skip-prepare", fields.TypeBool, fields.WithDefault(false), fields.WithHelp("Skip prepare")),
			fields.New("build-step", fields.TypeStringList, fields.WithHelp("Build step names")),
			fields.New("prepare-step", fields.TypeStringList, fields.WithHelp("Prepare step names")),
		))
	}
	return &LifecycleCommand{
		CommandDescription: glazedcmds.NewCommandDescription(kind, options...),
		kind:               kind,
	}, nil
}

func (c *LifecycleCommand) RunIntoGlazeProcessor(
	ctx context.Context,
	vals *values.Values,
	processor middlewares.Processor,
) error {
	settings := LifecycleSettings{}
	if err := vals.DecodeSectionInto(schema.DefaultSlug, &settings); err != nil {
		return errors.Wrap(err, "decode lifecycle settings")
	}
	repositoryContext, err := RepoContextFromParsedLayers(vals)
	if err != nil {
		return err
	}
	controller, err := newOperatorController(repositoryContext.RepoRoot)
	if err != nil {
		return err
	}
	selection := operator.Selection{Services: settings.Services}
	policy := operator.PipelinePolicy{
		ConfigPath:   repositoryContext.ConfigPath,
		Cwd:          repositoryContext.Cwd,
		Strict:       repositoryContext.Strict,
		DryRun:       repositoryContext.DryRun,
		Timeout:      repositoryContext.Timeout,
		SkipBuild:    settings.SkipBuild,
		SkipPrepare:  settings.SkipPrepare,
		SkipValidate: settings.SkipValidate,
		BuildSteps:   settings.BuildSteps,
		PrepareSteps: settings.PrepareSteps,
	}
	var result operator.OperationResult
	var operationErr error
	switch c.kind {
	case "up":
		result, operationErr = controller.Up(ctx, operator.UpRequest{
			RepoRoot: repositoryContext.RepoRoot,
			Profile:  repositoryContext.Profile,
			Select:   selection,
			Policy:   policy,
		}, nil)
	case "down":
		result, operationErr = controller.Down(ctx, operator.DownRequest{
			RepoRoot: repositoryContext.RepoRoot,
			Select:   selection,
			Timeout:  policy.Timeout,
		}, nil)
	case "restart":
		result, operationErr = controller.Restart(ctx, operator.RestartRequest{
			RepoRoot: repositoryContext.RepoRoot,
			Profile:  repositoryContext.Profile,
			Select:   selection,
			Policy:   policy,
		}, nil)
	default:
		return errors.Errorf("unsupported lifecycle command %q", c.kind)
	}
	if err := addOperationRows(ctx, processor, result); err != nil {
		return err
	}
	return operationErr
}

func addOperationRows(
	ctx context.Context,
	processor middlewares.Processor,
	result operator.OperationResult,
) error {
	if len(result.Outcomes) == 0 {
		return processor.AddRow(ctx, types.NewRow(
			types.MRP("operation_id", result.OperationID),
			types.MRP("operation", result.Kind),
			types.MRP("status", result.Status),
			types.MRP("service", ""),
			types.MRP("changed", false),
			types.MRP("duration_ms", result.FinishedAt.Sub(result.StartedAt).Milliseconds()),
		))
	}
	for _, outcome := range result.Outcomes {
		errorCode := ""
		errorMessage := ""
		if outcome.Error != nil {
			errorCode = outcome.Error.Code
			errorMessage = outcome.Error.Message
		}
		row := types.NewRow(
			types.MRP("operation_id", result.OperationID),
			types.MRP("operation", result.Kind),
			types.MRP("status", result.Status),
			types.MRP("service", outcome.Service),
			types.MRP("run_id", outcome.RunID),
			types.MRP("before", string(outcome.Before)),
			types.MRP("after", string(outcome.After)),
			types.MRP("changed", outcome.Changed),
			types.MRP("error_code", errorCode),
			types.MRP("error", errorMessage),
			types.MRP("duration_ms", result.FinishedAt.Sub(result.StartedAt).Milliseconds()),
		)
		if err := processor.AddRow(ctx, row); err != nil {
			return err
		}
	}
	return nil
}

func newOperatorController(repoRoot string) (operator.Controller, error) {
	executable, err := os.Executable()
	if err != nil {
		return nil, errors.Wrap(err, "resolve devctl executable")
	}
	logReader, err := runlog.NewFileReader(repoRoot)
	if err != nil {
		return nil, err
	}
	return operator.NewController(operator.ControllerOptions{
		WrapperExe: executable,
		LogReader:  logReader,
	})
}

func buildGlazedCommand(command glazedcmds.GlazeCommand) *cobra.Command {
	description := command.Description().Clone(true)
	if _, exists := description.Schema.Get(glazedsettings.GlazedSlug); !exists {
		glazedSection, err := glazedsettings.NewGlazedSection()
		cobra.CheckErr(err)
		description.Schema.Set(glazedsettings.GlazedSlug, glazedSection)
	}
	built := &cobra.Command{
		Use: description.Name, Short: description.Short, Long: description.Long,
	}
	if configurable, ok := command.(interface{ ConfigureCobra(*cobra.Command) }); ok {
		configurable.ConfigureCobra(built)
	}
	parser, err := cli.NewCobraParserFromSections(description.Schema, &cli.CobraParserConfig{
		SkipCommandSettingsSection: false,
	})
	cobra.CheckErr(err)
	cobra.CheckErr(parser.AddToCobraCommand(built))
	built.RunE = func(cmd *cobra.Command, args []string) error {
		if receiver, ok := command.(interface{ SetCobraArgs([]string) error }); ok {
			if err := receiver.SetCobraArgs(args); err != nil {
				return err
			}
		}
		parsedValues, err := parser.Parse(cmd, args)
		if err != nil {
			return err
		}
		if preparer, ok := command.(interface{ PrepareGlazedValues(*values.Values) error }); ok {
			if err := preparer.PrepareGlazedValues(parsedValues); err != nil {
				return err
			}
		}
		glazedValues, exists := parsedValues.Get(glazedsettings.GlazedSlug)
		if !exists {
			return errors.New("glazed output settings are unavailable")
		}
		var processor middlewares.Processor
		custom := false
		if builder, ok := command.(interface {
			BuildGlazedProcessor(*values.Values, io.Writer) (middlewares.Processor, bool, error)
		}); ok {
			processor, custom, err = builder.BuildGlazedProcessor(parsedValues, cmd.OutOrStdout())
			if err != nil {
				return err
			}
		}
		if !custom {
			tableProcessor, setupErr := glazedsettings.SetupTableProcessor(glazedValues)
			err = setupErr
			if err != nil {
				return err
			}
			processor = tableProcessor
			if _, err := glazedsettings.SetupProcessorOutput(tableProcessor, glazedValues, cmd.OutOrStdout()); err != nil {
				return err
			}
		}
		runErr := command.RunIntoGlazeProcessor(cmd.Context(), parsedValues, processor)
		closeErr := processor.Close(cmd.Context())
		return stderrors.Join(runErr, closeErr)
	}
	return built
}

func newUpCmd() *cobra.Command {
	command, err := NewLifecycleCommand("up")
	cobra.CheckErr(err)
	return buildGlazedCommand(command)
}

func newDownCmd() *cobra.Command {
	command, err := NewLifecycleCommand("down")
	cobra.CheckErr(err)
	return buildGlazedCommand(command)
}

func newRestartCmd() *cobra.Command {
	command, err := NewLifecycleCommand("restart")
	cobra.CheckErr(err)
	return buildGlazedCommand(command)
}

func processPID(identity *runstate.ProcessIdentity) int {
	if identity == nil {
		return 0
	}
	return identity.PID
}

func healthStatus(health *runstate.HealthResult) string {
	switch {
	case health == nil:
		return ""
	case health.Healthy:
		return "healthy"
	default:
		return "unhealthy"
	}
}

func exitCode(exit *runstate.ExitSummary) any {
	if exit == nil || exit.ExitCode == nil {
		return nil
	}
	return *exit.ExitCode
}

func exitSignal(exit *runstate.ExitSummary) string {
	if exit == nil {
		return ""
	}
	return exit.Signal
}

func lastErrorCode(lastError *runstate.ErrorRecord) string {
	if lastError == nil {
		return ""
	}
	return lastError.Code
}

func uptime(now time.Time, service operator.ServiceSnapshot) time.Duration {
	if service.CreatedAt.IsZero() {
		return 0
	}
	end := now
	if service.Exit != nil && !service.Exit.ExitedAt.IsZero() {
		end = service.Exit.ExitedAt
	}
	return end.Sub(service.CreatedAt)
}
