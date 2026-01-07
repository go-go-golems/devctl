package cmds

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/go-go-golems/devctl/pkg/config"
	"github.com/go-go-golems/devctl/pkg/discovery"
	"github.com/go-go-golems/devctl/pkg/engine"
	"github.com/go-go-golems/devctl/pkg/patch"
	"github.com/go-go-golems/devctl/pkg/runtime"
	"github.com/go-go-golems/devctl/pkg/supervise"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

func newSmokeTestFailuresCmd() *cobra.Command {
	var timeout time.Duration

	cmd := &cobra.Command{
		Use:   "smoketest-failures",
		Short: "Smoke test: validate fail, launch fail, and plugin timeout behaviors",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
			defer cancel()

			if err := smoketestValidateFails(ctx); err != nil {
				return err
			}
			if err := smoketestLaunchFails(ctx); err != nil {
				return err
			}
			if err := smoketestPluginTimeout(ctx); err != nil {
				return err
			}

			out := map[string]any{"ok": true}
			b, _ := json.MarshalIndent(out, "", "  ")
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(b))
			log.Info().Msg("smoketest-failures ok")
			return nil
		},
	}

	cmd.Flags().DurationVar(&timeout, "timeout", 15*time.Second, "Overall timeout for the smoketest")
	return cmd
}

func smoketestValidateFails(ctx context.Context) error {
	repoRoot, err := os.MkdirTemp("", "devctl-smoketest-validatefail-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(repoRoot) }()

	devctlRoot := findDevctlRootFromCaller()
	plugin := filepath.Join(devctlRoot, "testdata", "plugins", "validate-passfail", "plugin.py")

	cfgPath := filepath.Join(repoRoot, ".devctl.yaml")
	cfgBody := []byte("plugins:\n  - id: v\n    path: python3\n    args:\n      - \"" + plugin + "\"\n    env:\n      DEVCTL_VALIDATE_FAIL: \"1\"\n    priority: 10\n")
	if err := os.WriteFile(cfgPath, cfgBody, 0o644); err != nil {
		return err
	}

	cfg, err := config.LoadFromFile(cfgPath)
	if err != nil {
		return err
	}
	specs, err := discovery.Discover(cfg, discovery.Options{RepoRoot: repoRoot})
	if err != nil {
		return err
	}

	opCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	factory := runtime.NewFactory(runtime.FactoryOptions{HandshakeTimeout: 2 * time.Second, ShutdownTimeout: 2 * time.Second})
	clients, err := startClients(opCtx, factory, specs, runtime.RequestMeta{RepoRoot: repoRoot, Cwd: repoRoot})
	if err != nil {
		return err
	}
	defer closeClients(context.Background(), clients)

	p := &engine.Pipeline{Clients: clients, Opts: engine.Options{Strict: true}}
	conf, err := p.MutateConfig(opCtx, patch.Config{})
	if err != nil {
		return err
	}
	vr, err := p.Validate(opCtx, conf)
	if err != nil {
		return err
	}
	if vr.Valid {
		return errors.New("expected validate.run to fail")
	}
	return nil
}

func smoketestLaunchFails(ctx context.Context) error {
	repoRoot, err := os.MkdirTemp("", "devctl-smoketest-launchfail-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(repoRoot) }()

	devctlRoot := findDevctlRootFromCaller()
	plugin := filepath.Join(devctlRoot, "testdata", "plugins", "launch-fail", "plugin.py")

	cfgPath := filepath.Join(repoRoot, ".devctl.yaml")
	cfgBody := []byte("plugins:\n  - id: f\n    path: python3\n    args:\n      - \"" + plugin + "\"\n    priority: 10\n")
	if err := os.WriteFile(cfgPath, cfgBody, 0o644); err != nil {
		return err
	}

	cfg, err := config.LoadFromFile(cfgPath)
	if err != nil {
		return err
	}
	specs, err := discovery.Discover(cfg, discovery.Options{RepoRoot: repoRoot})
	if err != nil {
		return err
	}

	opCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	factory := runtime.NewFactory(runtime.FactoryOptions{HandshakeTimeout: 2 * time.Second, ShutdownTimeout: 2 * time.Second})
	clients, err := startClients(opCtx, factory, specs, runtime.RequestMeta{RepoRoot: repoRoot, Cwd: repoRoot})
	if err != nil {
		return err
	}
	defer closeClients(context.Background(), clients)

	p := &engine.Pipeline{Clients: clients, Opts: engine.Options{Strict: true}}
	conf, err := p.MutateConfig(opCtx, patch.Config{})
	if err != nil {
		return err
	}
	plan, err := p.LaunchPlan(opCtx, conf)
	if err != nil {
		return err
	}

	sup := supervise.New(supervise.Options{RepoRoot: repoRoot, ReadyTimeout: 2 * time.Second})
	_, err = sup.Start(opCtx, plan)
	if err == nil {
		return errors.New("expected supervise.Start to fail")
	}
	return nil
}

func smoketestPluginTimeout(ctx context.Context) error {
	devctlRoot := findDevctlRootFromCaller()
	plugin := filepath.Join(devctlRoot, "testdata", "plugins", "timeout", "plugin.py")

	opCtx, cancel := context.WithTimeout(ctx, 250*time.Millisecond)
	defer cancel()

	factory := runtime.NewFactory(runtime.FactoryOptions{HandshakeTimeout: 2 * time.Second, ShutdownTimeout: 2 * time.Second})
	c, err := factory.Start(opCtx, runtime.PluginSpec{
		ID:      "timeout",
		Path:    "python3",
		Args:    []string{plugin},
		WorkDir: devctlRoot,
	}, runtime.StartOptions{Meta: runtime.RequestMeta{RepoRoot: devctlRoot, Cwd: devctlRoot}})
	if err != nil {
		return err
	}
	defer func() { _ = c.Close(context.Background()) }()

	var out struct {
		Pong bool `json:"pong"`
	}
	err = c.Call(opCtx, "ping", map[string]any{}, &out)
	if err == nil {
		return errors.New("expected plugin call to time out")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		return errors.Wrap(err, "expected context deadline exceeded")
	}
	return nil
}

func startClients(ctx context.Context, factory *runtime.Factory, specs []runtime.PluginSpec, meta runtime.RequestMeta) ([]runtime.Client, error) {
	clients := make([]runtime.Client, 0, len(specs))
	for _, spec := range specs {
		c, err := factory.Start(ctx, spec, runtime.StartOptions{Meta: meta})
		if err != nil {
			closeClients(context.Background(), clients)
			return nil, err
		}
		clients = append(clients, c)
	}
	return clients, nil
}

func closeClients(ctx context.Context, clients []runtime.Client) {
	for _, c := range clients {
		_ = c.Close(ctx)
	}
}
