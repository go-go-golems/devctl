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

func newStartServiceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start <service-name>",
		Short: "Start a previously stopped supervised service",
		Long: `Start a single previously stopped supervised service.

This only works for services that exist in the state file but have been
stopped (pid=0). The service is started from its stored ServiceSpec
(command, env, cwd, health check) that was recorded during the last
"devctl up". This does NOT re-run the plugin pipeline.`,
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
				RepoRoot:     opts.RepoRoot,
				ReadyTimeout: opts.Timeout,
				WrapperExe:   wrapperExe,
			})

			ctx, cancel := context.WithTimeout(cmd.Context(), opts.Timeout)
			defer cancel()

			if err := sup.StartService(ctx, st, serviceName); err != nil {
				return err
			}

			log.Info().Str("service", serviceName).Msg("started")
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "ok")
			return nil
		},
	}

	AddRepoFlags(cmd)
	return cmd
}
