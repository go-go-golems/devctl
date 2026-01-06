package cmds

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	goruntime "runtime"
	"time"

	"github.com/go-go-golems/devctl/pkg/config"
	"github.com/go-go-golems/devctl/pkg/discovery"
	"github.com/go-go-golems/devctl/pkg/engine"
	"github.com/go-go-golems/devctl/pkg/patch"
	plugruntime "github.com/go-go-golems/devctl/pkg/runtime"
	"github.com/go-go-golems/devctl/pkg/state"
	"github.com/go-go-golems/devctl/pkg/supervise"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

func newSmokeTestSuperviseCmd() *cobra.Command {
	var timeout time.Duration

	cmd := &cobra.Command{
		Use:   "smoketest-supervise",
		Short: "Smoke test: run config.mutate+launch.plan and supervise an HTTP server",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
			defer cancel()

			repoRoot, err := os.MkdirTemp("", "devctl-smoketest-*")
			if err != nil {
				return err
			}
			defer func() { _ = os.RemoveAll(repoRoot) }()

			plugin := filepath.Join(findDevctlRoot(), "testdata", "plugins", "http-service", "plugin.py")

			cfgPath := filepath.Join(repoRoot, ".devctl.yaml")
			cfgBody := []byte("plugins:\n  - id: http\n    path: python3\n    args:\n      - \"" + plugin + "\"\n    priority: 10\n")
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

			factory := plugruntime.NewFactory(plugruntime.FactoryOptions{HandshakeTimeout: 2 * time.Second, ShutdownTimeout: 2 * time.Second})
			var clients []plugruntime.Client
			for _, spec := range specs {
				c, err := factory.Start(ctx, spec)
				if err != nil {
					return err
				}
				clients = append(clients, c)
			}
			defer func() {
				closeCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()
				for _, c := range clients {
					_ = c.Close(closeCtx)
				}
			}()

			p := &engine.Pipeline{Clients: clients, Opts: engine.Options{Strict: true}}
			conf, err := p.MutateConfig(ctx, patch.Config{})
			if err != nil {
				return err
			}

			plan, err := p.LaunchPlan(ctx, conf)
			if err != nil {
				return err
			}
			if len(plan.Services) == 0 {
				return errors.New("expected at least one service")
			}

			sup := supervise.New(supervise.Options{RepoRoot: repoRoot, ReadyTimeout: timeout})
			st, err := sup.Start(ctx, plan)
			if err != nil {
				return err
			}
			defer func() { _ = sup.Stop(context.Background(), st) }()

			if err := state.Save(repoRoot, st); err != nil {
				return err
			}

			servicesAny, ok := conf["services"].(map[string]any)
			if !ok {
				return errors.New("missing services in config")
			}
			httpAny, ok := servicesAny["http"].(map[string]any)
			if !ok {
				return errors.New("missing services.http in config")
			}
			urlAny, ok := httpAny["url"]
			if !ok {
				return errors.New("missing services.http.url in config")
			}
			urlStr, ok := urlAny.(string)
			if !ok {
				return errors.New("services.http.url is not a string")
			}

			req, _ := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, nil)
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				return err
			}
			_ = resp.Body.Close()
			if resp.StatusCode < 200 || resp.StatusCode >= 500 {
				return errors.Errorf("unexpected status: %d", resp.StatusCode)
			}

			b, _ := json.MarshalIndent(map[string]any{"ok": true, "url": urlStr}, "", "  ")
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(b))
			log.Info().Msg("smoketest-supervise ok")
			return nil
		},
	}

	cmd.Flags().DurationVar(&timeout, "timeout", 10*time.Second, "Overall timeout for the smoketest")
	return cmd
}

func findDevctlRoot() string {
	_, thisFile, _, ok := goruntime.Caller(0)
	if !ok {
		wd, _ := os.Getwd()
		return wd
	}
	// this file: devctl/cmd/devctl/cmds/smoketest_supervise.go
	return filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", "..", ".."))
}
