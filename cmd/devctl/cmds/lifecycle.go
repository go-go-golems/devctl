package cmds

import (
	"context"
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
	Explain      bool     `glazed:"explain"`
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
			fields.New("explain", fields.TypeBool, fields.WithDefault(false), fields.WithHelp("Describe the effect-free lifecycle recipe without running plugins")),
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
	if settings.Explain {
		recipe, err := (operator.PipelinePlanner{}).ResolveRecipe(ctx, c.kind, operator.UpRequest{
			RepoRoot: repositoryContext.RepoRoot,
			Profile:  repositoryContext.Profile,
			Select:   selection,
			Policy:   policy,
		})
		if err != nil {
			return err
		}
		return addLifecycleRecipeRows(ctx, processor, recipe)
	}
	controller, err := newOperatorController(repositoryContext.RepoRoot)
	if err != nil {
		return err
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

func addLifecycleRecipeRows(
	ctx context.Context,
	processor middlewares.Processor,
	recipe operator.LifecycleRecipe,
) error {
	for _, phase := range recipe.Phases {
		if err := processor.AddRow(ctx, types.NewRow(
			types.MRP("recipe_version", recipe.Version),
			types.MRP("recipe_id", recipe.ID),
			types.MRP("operation", recipe.Operation),
			types.MRP("repository_fingerprint", recipe.RepositoryFingerprint),
			types.MRP("profile", recipe.ProfileName),
			types.MRP("services", recipe.Selection.Services),
			types.MRP("phase", phase.Name),
			types.MRP("enabled", phase.Enabled),
			types.MRP("steps", phase.Steps),
			types.MRP("unresolved", phase.Unresolved),
		)); err != nil {
			return err
		}
	}
	return nil
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

func buildGlazedCommand(command glazedcmds.Command) (*cobra.Command, error) {
	return cli.BuildCobraCommandFromCommand(command,
		cli.WithCobraShortHelpSections(schema.DefaultSlug),
	)
}

func buildDualGlazedCommand(command glazedcmds.Command) (*cobra.Command, error) {
	return cli.BuildCobraCommandFromCommand(command,
		cli.WithDualMode(true),
		cli.WithGlazeToggleFlag("with-glaze-output"),
		cli.WithCobraShortHelpSections(schema.DefaultSlug),
	)
}

func newUpCmd() (*cobra.Command, error) {
	command, err := NewLifecycleCommand("up")
	if err != nil {
		return nil, err
	}
	return buildGlazedCommand(command)
}

func newDownCmd() (*cobra.Command, error) {
	command, err := NewLifecycleCommand("down")
	if err != nil {
		return nil, err
	}
	return buildGlazedCommand(command)
}

func newRestartCmd() (*cobra.Command, error) {
	command, err := NewLifecycleCommand("restart")
	if err != nil {
		return nil, err
	}
	return buildGlazedCommand(command)
}

func processPID(identity *runstate.ProcessIdentity) int {
	if identity == nil {
		return 0
	}
	return identity.PID
}

func healthStatus(phase runstate.RunPhase, health *runstate.HealthResult) string {
	return string(runstate.ProjectHealth(phase, health).Current)
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

func artifactID(artifact *runstate.ArtifactRecord) string {
	if artifact == nil {
		return ""
	}
	return artifact.ID
}

func artifactPath(artifact *runstate.ArtifactRecord) string {
	if artifact == nil {
		return ""
	}
	return artifact.Path
}

func artifactSHA256(artifact *runstate.ArtifactRecord) string {
	if artifact == nil {
		return ""
	}
	return artifact.SHA256
}

func artifactSize(artifact *runstate.ArtifactRecord) int64 {
	if artifact == nil {
		return 0
	}
	return artifact.SizeBytes
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
