package cmds

import (
	"context"
	"time"

	"github.com/go-go-golems/devctl/pkg/engine"
	"github.com/go-go-golems/devctl/pkg/patch"
	"github.com/go-go-golems/devctl/pkg/repository"
	"github.com/go-go-golems/devctl/pkg/runtime"
	glazedcmds "github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	"github.com/go-go-golems/glazed/pkg/middlewares"
	"github.com/go-go-golems/glazed/pkg/types"
	"github.com/spf13/cobra"
)

type PlanCommand struct {
	*glazedcmds.CommandDescription
}

var _ glazedcmds.GlazeCommand = (*PlanCommand)(nil)

func NewPlanCommand() (*PlanCommand, error) {
	repoSection, err := getRepoLayer()
	if err != nil {
		return nil, err
	}
	return &PlanCommand{CommandDescription: glazedcmds.NewCommandDescription(
		"plan",
		glazedcmds.WithShort("Compute a merged launch plan from selected plugins"),
		glazedcmds.WithSections(repoSection),
	)}, nil
}

func (c *PlanCommand) RunIntoGlazeProcessor(
	ctx context.Context,
	vals *values.Values,
	processor middlewares.Processor,
) error {
	repositoryContext, err := RepoContextFromParsedLayers(vals)
	if err != nil {
		return err
	}
	repo, err := repository.Load(repository.Options{
		RepoRoot: repositoryContext.RepoRoot, ConfigPath: repositoryContext.ConfigPath,
		ProfileName: repositoryContext.Profile, Cwd: repositoryContext.Cwd,
		DryRun: repositoryContext.DryRun,
	})
	if err != nil {
		return err
	}
	configuration := patch.Config{}
	plan := engine.LaunchPlan{Services: []engine.ServiceSpec{}}
	if len(repo.Specs) == 0 {
		return addPlanRow(ctx, processor, configuration, plan, 0)
	}
	strict := repositoryContext.Strict || repo.Config.Strictness == "error"
	factory := runtime.NewFactory(runtime.FactoryOptions{
		HandshakeTimeout: 2 * time.Second,
		ShutdownTimeout:  2 * time.Second,
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
	configuration, err = pipeline.MutateConfig(operationContext, configuration)
	cancel()
	if err != nil {
		return err
	}
	operationContext, cancel = context.WithTimeout(ctx, repositoryContext.Timeout)
	plan, err = pipeline.LaunchPlan(operationContext, configuration)
	cancel()
	if err != nil {
		return err
	}
	return addPlanRow(ctx, processor, configuration, plan, len(clients))
}

func addPlanRow(
	ctx context.Context,
	processor middlewares.Processor,
	configuration patch.Config,
	plan engine.LaunchPlan,
	pluginCount int,
) error {
	return processor.AddRow(ctx, types.NewRow(
		types.MRP("config", configuration),
		types.MRP("plan", plan),
		types.MRP("plugin_count", pluginCount),
		types.MRP("service_count", len(plan.Services)),
	))
}

func newPlanCmd() *cobra.Command {
	command, err := NewPlanCommand()
	cobra.CheckErr(err)
	return buildGlazedCommand(command)
}
