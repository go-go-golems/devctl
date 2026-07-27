package cmds

import (
	"context"
	"time"

	"github.com/go-go-golems/devctl/pkg/engine"
	"github.com/go-go-golems/devctl/pkg/patch"
	"github.com/go-go-golems/devctl/pkg/repository"
	"github.com/go-go-golems/devctl/pkg/runtime"
	glazedcmds "github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/fields"
	"github.com/go-go-golems/glazed/pkg/cmds/schema"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	"github.com/go-go-golems/glazed/pkg/middlewares"
	"github.com/go-go-golems/glazed/pkg/types"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

type phaseRunner struct {
	context RepoContext
	repo    *repository.Repository
	pipe    *engine.Pipeline
}

type PhaseSettings struct {
	Steps []string `glazed:"step"`
}

type PhaseCommand struct {
	*glazedcmds.CommandDescription
	kind string
}

var _ glazedcmds.GlazeCommand = (*PhaseCommand)(nil)

func NewPhaseCommand(kind string) (*PhaseCommand, error) {
	repoSection, err := getRepoLayer()
	if err != nil {
		return nil, err
	}
	short := map[string]string{
		"build":    "Run the build phase (config.mutate + build.run)",
		"prepare":  "Run the prepare phase (config.mutate + prepare.run)",
		"validate": "Run the validation phase (config.mutate + validate.run)",
	}[kind]
	if short == "" {
		return nil, errors.Errorf("unknown phase command %q", kind)
	}
	options := []glazedcmds.CommandDescriptionOption{
		glazedcmds.WithShort(short),
		glazedcmds.WithSections(repoSection),
	}
	if kind != "validate" {
		options = append(options, glazedcmds.WithFlags(
			fields.New("step", fields.TypeStringList, fields.WithHelp("Phase step name; repeatable")),
		))
	}
	return &PhaseCommand{
		CommandDescription: glazedcmds.NewCommandDescription(kind, options...),
		kind:               kind,
	}, nil
}

func (c *PhaseCommand) RunIntoGlazeProcessor(
	ctx context.Context,
	vals *values.Values,
	processor middlewares.Processor,
) error {
	settings := PhaseSettings{}
	if err := vals.DecodeSectionInto(schema.DefaultSlug, &settings); err != nil {
		return errors.Wrap(err, "decode phase settings")
	}
	repositoryContext, err := RepoContextFromParsedLayers(vals)
	if err != nil {
		return err
	}
	return withPhaseRunner(ctx, repositoryContext, func(
		ctx context.Context,
		runner *phaseRunner,
		configuration patch.Config,
	) error {
		row := types.NewRow(types.MRP("config", configuration))
		switch c.kind {
		case "build":
			result, err := runBuildPhase(ctx, runner, configuration, settings.Steps)
			if err != nil {
				return err
			}
			row.Set("build", result)
		case "prepare":
			result, err := runPreparePhase(ctx, runner, configuration, settings.Steps)
			if err != nil {
				return err
			}
			row.Set("prepare", result)
		case "validate":
			result, err := runValidatePhase(ctx, runner, configuration)
			if err != nil {
				return err
			}
			row.Set("validate", result)
			if err := processor.AddRow(ctx, row); err != nil {
				return err
			}
			if !result.Valid {
				return errors.New("validation failed")
			}
			return nil
		default:
			return errors.Errorf("unsupported phase command %q", c.kind)
		}
		return processor.AddRow(ctx, row)
	})
}

func withPhaseRunner(
	ctx context.Context,
	repositoryContext RepoContext,
	fn func(context.Context, *phaseRunner, patch.Config) error,
) error {
	repo, err := repository.Load(repository.Options{
		RepoRoot: repositoryContext.RepoRoot, ConfigPath: repositoryContext.ConfigPath,
		ProfileName: repositoryContext.Profile, Cwd: repositoryContext.Cwd,
		DryRun: repositoryContext.DryRun,
	})
	if err != nil {
		return err
	}
	if len(repo.Specs) == 0 {
		return errors.New("no plugins configured (add .devctl.yaml)")
	}
	strict := repositoryContext.Strict || repo.Config.Strictness == "error"
	factory := runtime.NewFactory(runtime.FactoryOptions{
		HandshakeTimeout: 2 * time.Second,
		ShutdownTimeout:  3 * time.Second,
	})
	clients, err := repo.StartClients(ctx, factory)
	if err != nil {
		return err
	}
	defer func() {
		closeContext, cancel := context.WithTimeout(context.Background(), repositoryContext.Timeout)
		defer cancel()
		_ = repository.CloseClients(closeContext, clients)
	}()
	pipeline := &engine.Pipeline{
		Clients: clients,
		Opts: engine.Options{
			Strict: strict, DryRun: repositoryContext.DryRun,
		},
	}
	operationContext, cancel := context.WithTimeout(ctx, repositoryContext.Timeout)
	configuration, err := pipeline.MutateConfig(operationContext, patch.Config{})
	cancel()
	if err != nil {
		return err
	}
	return fn(ctx, &phaseRunner{
		context: repositoryContext, repo: repo, pipe: pipeline,
	}, configuration)
}

func runBuildPhase(
	ctx context.Context,
	runner *phaseRunner,
	configuration patch.Config,
	steps []string,
) (engine.BuildResult, error) {
	operationContext, cancel := context.WithTimeout(ctx, runner.context.Timeout)
	defer cancel()
	return runner.pipe.Build(operationContext, configuration, steps)
}

func runPreparePhase(
	ctx context.Context,
	runner *phaseRunner,
	configuration patch.Config,
	steps []string,
) (engine.PrepareResult, error) {
	operationContext, cancel := context.WithTimeout(ctx, runner.context.Timeout)
	defer cancel()
	return runner.pipe.Prepare(operationContext, configuration, steps)
}

func runValidatePhase(
	ctx context.Context,
	runner *phaseRunner,
	configuration patch.Config,
) (engine.ValidateResult, error) {
	operationContext, cancel := context.WithTimeout(ctx, runner.context.Timeout)
	defer cancel()
	return runner.pipe.Validate(operationContext, configuration)
}

func buildPhaseCommand(kind string) *cobra.Command {
	command, err := NewPhaseCommand(kind)
	cobra.CheckErr(err)
	return buildGlazedCommand(command)
}

func newBuildCmd() *cobra.Command {
	return buildPhaseCommand("build")
}

func newPrepareCmd() *cobra.Command {
	return buildPhaseCommand("prepare")
}

func newValidateCmd() *cobra.Command {
	return buildPhaseCommand("validate")
}
