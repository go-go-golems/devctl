package smoketest

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/go-go-golems/devctl/pkg/runtime"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

func NewCmd() *cobra.Command {
	var pluginPath string
	var timeout time.Duration

	cmd := &cobra.Command{
		Use:   "smoketest",
		Short: "Run devctl smoke/integration tests (dev-only)",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
			defer cancel()

			repoRoot, err := os.Getwd()
			if err != nil {
				return err
			}
			meta := runtime.RequestMeta{RepoRoot: repoRoot, Cwd: repoRoot}

			if pluginPath == "" {
				pluginPath = filepath.Join(repoRoot, "testdata", "plugins", "ok-python", "plugin.py")
			}

			absPluginPath, err := filepath.Abs(pluginPath)
			if err != nil {
				return err
			}

			factory := runtime.NewFactory(runtime.FactoryOptions{
				HandshakeTimeout: 2 * time.Second,
				ShutdownTimeout:  2 * time.Second,
			})

			client, err := factory.Start(ctx, runtime.PluginSpec{
				ID:      "smoketest",
				Path:    "python3",
				Args:    []string{absPluginPath},
				WorkDir: repoRoot,
				Env:     map[string]string{},
			}, runtime.StartOptions{Meta: meta})
			if err != nil {
				return err
			}
			defer func() {
				closeCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()
				_ = client.Close(closeCtx)
			}()

			var out struct {
				Pong bool `json:"pong"`
			}
			err = client.Call(ctx, "ping", map[string]any{"message": "hi"}, &out)
			if err != nil {
				return err
			}
			if !out.Pong {
				return errors.New("unexpected ping output: pong=false")
			}

			log.Info().Msg("smoketest ok")
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "ok")
			return nil
		},
	}

	cmd.Flags().StringVar(&pluginPath, "plugin", "", "Path to a plugin script (defaults to testdata ok plugin)")
	cmd.Flags().DurationVar(&timeout, "timeout", 5*time.Second, "Overall timeout for the smoke test")

	cmd.AddCommand(
		newSuperviseCmd(),
		newE2ECmd(),
		newLogsCmd(),
		newFailuresCmd(),
	)

	return cmd
}
