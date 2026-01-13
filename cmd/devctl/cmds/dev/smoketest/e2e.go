package smoketest

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/go-go-golems/devctl/pkg/config"
	"github.com/go-go-golems/devctl/pkg/discovery"
	"github.com/go-go-golems/devctl/pkg/engine"
	"github.com/go-go-golems/devctl/pkg/patch"
	"github.com/go-go-golems/devctl/pkg/runtime"
	"github.com/go-go-golems/devctl/pkg/state"
	"github.com/go-go-golems/devctl/pkg/supervise"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

func newE2ECmd() *cobra.Command {
	var timeout time.Duration

	cmd := &cobra.Command{
		Use:   "e2e",
		Short: "Smoke test: build test apps, run up/status/logs/down end-to-end",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
			defer cancel()

			repoRoot, err := os.MkdirTemp("", "devctl-smoketest-e2e-*")
			if err != nil {
				return err
			}
			defer func() { _ = os.RemoveAll(repoRoot) }()

			devctlRoot := findDevctlRootFromCaller()

			binDir := filepath.Join(repoRoot, "bin")
			if err := os.MkdirAll(binDir, 0o755); err != nil {
				return err
			}

			httpEchoBin := filepath.Join(binDir, "http-echo")
			logSpewerBin := filepath.Join(binDir, "log-spewer")

			if err := buildTestApp(ctx, devctlRoot, "./testapps/cmd/http-echo", httpEchoBin); err != nil {
				return err
			}
			if err := buildTestApp(ctx, devctlRoot, "./testapps/cmd/log-spewer", logSpewerBin); err != nil {
				return err
			}

			port, err := findFreeTCPPort()
			if err != nil {
				return err
			}

			pluginPath := filepath.Join(devctlRoot, "testdata", "plugins", "e2e", "plugin.py")
			cfgPath := filepath.Join(repoRoot, ".devctl.yaml")
			cfgBody := []byte(
				"plugins:\n" +
					"  - id: e2e\n" +
					"    path: python3\n" +
					"    args:\n" +
					"      - \"" + pluginPath + "\"\n" +
					"    env:\n" +
					"      DEVCTL_HTTP_ECHO_BIN: \"" + httpEchoBin + "\"\n" +
					"      DEVCTL_LOG_SPEWER_BIN: \"" + logSpewerBin + "\"\n" +
					"      DEVCTL_HTTP_ECHO_PORT: \"" + fmt.Sprint(port) + "\"\n" +
					"    priority: 10\n",
			)
			if err := os.WriteFile(cfgPath, cfgBody, 0o644); err != nil {
				return err
			}

			meta := runtime.RequestMeta{RepoRoot: repoRoot, Cwd: repoRoot}

			cfg, err := config.LoadFromFile(cfgPath)
			if err != nil {
				return err
			}
			specs, err := discovery.Discover(cfg, discovery.Options{RepoRoot: repoRoot})
			if err != nil {
				return err
			}

			factory := runtime.NewFactory(runtime.FactoryOptions{HandshakeTimeout: 2 * time.Second, ShutdownTimeout: 2 * time.Second})
			var clients []runtime.Client
			for _, spec := range specs {
				c, err := factory.Start(ctx, spec, runtime.StartOptions{Meta: meta})
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
			vr, err := p.Validate(ctx, conf)
			if err != nil {
				return err
			}
			if !vr.Valid {
				return errors.New("unexpected validate failure")
			}
			if _, err := p.Build(ctx, conf, nil); err != nil {
				return err
			}
			if _, err := p.Prepare(ctx, conf, nil); err != nil {
				return err
			}
			plan, err := p.LaunchPlan(ctx, conf)
			if err != nil {
				return err
			}
			if len(plan.Services) < 2 {
				return errors.New("expected at least two services")
			}

			sup := supervise.New(supervise.Options{RepoRoot: repoRoot, ReadyTimeout: 5 * time.Second})
			st, err := sup.Start(ctx, plan)
			if err != nil {
				return err
			}
			defer func() { _ = sup.Stop(context.Background(), st) }()

			if err := state.Save(repoRoot, st); err != nil {
				return err
			}

			time.Sleep(250 * time.Millisecond)

			if err := assertServiceAlive(st, "http"); err != nil {
				return err
			}
			if err := assertServiceAlive(st, "spewer"); err != nil {
				return err
			}

			httpStdout, err := readServiceLog(st, "http", false)
			if err != nil {
				return err
			}
			if httpStdout == "" {
				return errors.New("expected http stdout log to be non-empty")
			}

			if err := sup.Stop(ctx, st); err != nil {
				return err
			}
			if err := state.Remove(repoRoot); err != nil {
				return err
			}

			out := map[string]any{"ok": true, "repo_root": repoRoot, "services": len(plan.Services)}
			b, _ := json.MarshalIndent(out, "", "  ")
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(b))
			log.Info().Msg("smoketest e2e ok")
			return nil
		},
	}

	cmd.Flags().DurationVar(&timeout, "timeout", 20*time.Second, "Overall timeout for the smoketest")
	return cmd
}

func buildTestApp(ctx context.Context, devctlRoot string, pkg string, outPath string) error {
	c := exec.CommandContext(ctx, "go", "build", "-o", outPath, pkg)
	c.Dir = devctlRoot
	c.Env = append(os.Environ(), "GOWORK=off")
	b, err := c.CombinedOutput()
	if err != nil {
		return errors.Wrapf(err, "build %s: %s", pkg, string(b))
	}
	return nil
}

func findFreeTCPPort() (int, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer func() { _ = ln.Close() }()
	_, portStr, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		return 0, err
	}
	var port int
	_, _ = fmt.Sscanf(portStr, "%d", &port)
	if port <= 0 {
		return 0, errors.New("failed to allocate port")
	}
	return port, nil
}

func assertServiceAlive(st *state.State, name string) error {
	for _, svc := range st.Services {
		if svc.Name == name {
			if state.ProcessAlive(svc.PID) {
				return nil
			}
			return errors.Errorf("service %q is not alive", name)
		}
	}
	return errors.Errorf("missing service %q", name)
}

func readServiceLog(st *state.State, name string, stderr bool) (string, error) {
	for _, svc := range st.Services {
		if svc.Name != name {
			continue
		}
		path := svc.StdoutLog
		if stderr {
			path = svc.StderrLog
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
	return "", errors.Errorf("unknown service %q", name)
}
