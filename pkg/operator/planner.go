package operator

import (
	"context"
	"time"

	"github.com/go-go-golems/devctl/pkg/engine"
	"github.com/go-go-golems/devctl/pkg/patch"
	"github.com/go-go-golems/devctl/pkg/repository"
	"github.com/go-go-golems/devctl/pkg/runtime"
	"github.com/pkg/errors"
)

type PlanResult struct {
	Plan        engine.LaunchPlan
	ProfileName string
}

type Planner interface {
	Plan(context.Context, UpRequest) (PlanResult, error)
}

type PipelinePlanner struct{}

var _ Planner = PipelinePlanner{}

func (PipelinePlanner) Plan(ctx context.Context, request UpRequest) (PlanResult, error) {
	timeout := request.Policy.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	cwd := request.Policy.Cwd
	if cwd == "" {
		cwd = request.RepoRoot
	}
	repo, err := repository.Load(repository.Options{
		RepoRoot:    request.RepoRoot,
		ConfigPath:  request.Policy.ConfigPath,
		ProfileName: request.Profile,
		Cwd:         cwd,
		DryRun:      request.Policy.DryRun,
	})
	if err != nil {
		return PlanResult{}, errors.Wrap(err, "load repository for operator plan")
	}
	if len(repo.Specs) == 0 {
		return PlanResult{}, errors.New("operator plan: no plugins configured")
	}

	strict := request.Policy.Strict || repo.Config.Strictness == "error"
	factory := runtime.NewFactory(runtime.FactoryOptions{
		HandshakeTimeout: 2 * time.Second,
		ShutdownTimeout:  3 * time.Second,
	})
	clients, err := repo.StartClients(ctx, factory)
	if err != nil {
		return PlanResult{}, errors.Wrap(err, "start operator plan plugins")
	}
	defer func() {
		closeContext, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = repository.CloseClients(closeContext, clients)
	}()

	pipeline := &engine.Pipeline{
		Clients: clients,
		Opts: engine.Options{
			Strict: strict,
			DryRun: request.Policy.DryRun,
		},
	}

	phaseContext, cancel := context.WithTimeout(ctx, timeout)
	config, err := pipeline.MutateConfig(phaseContext, patch.Config{})
	cancel()
	if err != nil {
		return PlanResult{}, errors.Wrap(err, "mutate operator configuration")
	}
	if !request.Policy.SkipBuild {
		phaseContext, cancel = context.WithTimeout(ctx, timeout)
		_, err = pipeline.Build(phaseContext, config, request.Policy.BuildSteps)
		cancel()
		if err != nil {
			return PlanResult{}, errors.Wrap(err, "run operator build phase")
		}
	}
	if !request.Policy.SkipPrepare {
		phaseContext, cancel = context.WithTimeout(ctx, timeout)
		_, err = pipeline.Prepare(phaseContext, config, request.Policy.PrepareSteps)
		cancel()
		if err != nil {
			return PlanResult{}, errors.Wrap(err, "run operator prepare phase")
		}
	}
	if !request.Policy.SkipValidate {
		phaseContext, cancel = context.WithTimeout(ctx, timeout)
		validation, validationErr := pipeline.Validate(phaseContext, config)
		cancel()
		if validationErr != nil {
			return PlanResult{}, errors.Wrap(validationErr, "run operator validation phase")
		}
		if !validation.Valid {
			return PlanResult{}, errors.Errorf("operator validation failed with %d error(s)", len(validation.Errors))
		}
	}
	phaseContext, cancel = context.WithTimeout(ctx, timeout)
	plan, err := pipeline.LaunchPlan(phaseContext, config)
	cancel()
	if err != nil {
		return PlanResult{}, errors.Wrap(err, "resolve operator launch plan")
	}
	return PlanResult{Plan: plan, ProfileName: repo.ProfileName}, nil
}
