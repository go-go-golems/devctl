package servicecontrol

import (
	"context"
	"time"

	"github.com/go-go-golems/devctl/pkg/engine"
	"github.com/go-go-golems/devctl/pkg/patch"
	"github.com/go-go-golems/devctl/pkg/repository"
	"github.com/go-go-golems/devctl/pkg/runtime"
	"github.com/pkg/errors"
)

type ResolveOptions struct {
	RepoRoot    string
	ConfigPath  string
	ProfileName string
	Cwd         string
	DryRun      bool
	Strict      bool
	Timeout     time.Duration
}

// ResolveServiceSpec recomputes the effective launch plan and returns the named service.
// This intentionally runs the planning phases (config.mutate + launch.plan) instead of
// reading a raw environment from state.json, so service start/restart can recover plugin-
// computed environment variables without persisting secrets.
func ResolveServiceSpec(ctx context.Context, opts ResolveOptions, serviceName string) (engine.ServiceSpec, error) {
	if serviceName == "" {
		return engine.ServiceSpec{}, errors.New("missing service name")
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 30 * time.Second
	}
	cwd := opts.Cwd
	if cwd == "" {
		cwd = opts.RepoRoot
	}

	repo, err := repository.Load(repository.Options{
		RepoRoot:    opts.RepoRoot,
		ConfigPath:  opts.ConfigPath,
		ProfileName: opts.ProfileName,
		Cwd:         cwd,
		DryRun:      opts.DryRun,
	})
	if err != nil {
		return engine.ServiceSpec{}, err
	}
	strict := opts.Strict
	if !strict && repo.Config.Strictness == "error" {
		strict = true
	}
	if len(repo.Specs) == 0 {
		return engine.ServiceSpec{}, errors.New("no plugins configured (add .devctl.yaml)")
	}

	factory := runtime.NewFactory(runtime.FactoryOptions{
		HandshakeTimeout: 2 * time.Second,
		ShutdownTimeout:  3 * time.Second,
	})
	clients, err := repo.StartClients(ctx, factory)
	if err != nil {
		return engine.ServiceSpec{}, err
	}
	defer func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = repository.CloseClients(closeCtx, clients)
	}()

	p := &engine.Pipeline{
		Clients: clients,
		Opts: engine.Options{
			Strict: strict,
			DryRun: opts.DryRun,
		},
	}

	opCtx, cancel := context.WithTimeout(ctx, opts.Timeout)
	conf, err := p.MutateConfig(opCtx, patch.Config{})
	cancel()
	if err != nil {
		return engine.ServiceSpec{}, err
	}

	opCtx, cancel = context.WithTimeout(ctx, opts.Timeout)
	plan, err := p.LaunchPlan(opCtx, conf)
	cancel()
	if err != nil {
		return engine.ServiceSpec{}, err
	}

	for _, svc := range plan.Services {
		if svc.Name == serviceName {
			return svc, nil
		}
	}
	return engine.ServiceSpec{}, errors.Errorf("service %q not found in launch plan", serviceName)
}
