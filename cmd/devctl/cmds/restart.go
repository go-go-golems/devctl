package cmds

import (
	"context"
	"fmt"
	"os"

	"github.com/go-go-golems/devctl/pkg/state"
	"github.com/go-go-golems/devctl/pkg/supervise"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

func newRestartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "restart <service-name>",
		Short: "Restart a single supervised service",
		Long: `Restart a single supervised service without affecting other services.

The service is stopped (SIGTERM to its process group) and then restarted
from its original ServiceSpec (command, env, cwd, health check) that was
stored during the last "devctl up". This does NOT re-run the plugin pipeline,
so any config changes since the last "up" will not be reflected.

For a full re-evaluation, use "devctl down && devctl up".`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			serviceName := args[0]
			opts, err := getRootOptions(cmd)
			if err != nil {
				return err
			}

			st, err := state.Load(opts.RepoRoot)
			if err != nil {
				return err
			}

			found := false
			for _, svc := range st.Services {
				if svc.Name == serviceName {
					found = true
					break
				}
			}
			if !found {
				return errors.Errorf("service %q not found in state", serviceName)
			}

			wrapperExe, _ := os.Executable()
			sup := supervise.New(supervise.Options{
				RepoRoot:        opts.RepoRoot,
				ShutdownTimeout: opts.Timeout,
				ReadyTimeout:    opts.Timeout,
				WrapperExe:      wrapperExe,
			})

			ctx, cancel := context.WithTimeout(cmd.Context(), opts.Timeout)
			defer cancel()

			if err := sup.RestartService(ctx, st, serviceName); err != nil {
				return err
			}

			log.Info().Str("service", serviceName).Msg("restarted")
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "ok")
			return nil
		},
	}

	AddRepoFlags(cmd)
	return cmd
}
