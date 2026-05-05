package cmds

import (
	"context"
	"fmt"
	"os"

	"github.com/go-go-golems/devctl/pkg/state"
	"github.com/go-go-golems/devctl/pkg/supervise"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

func newStopServiceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stop-service <service-name>",
		Short: "Stop a single supervised service (leaves others running)",
		Long: `Stop a single supervised service without affecting other services.

The service record is kept in the state file (with PID cleared) so it can be
restarted later with "devctl restart <service-name>". Use "devctl down" to
stop all services and remove the state file entirely.`,
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

			wrapperExe, _ := os.Executable()
			sup := supervise.New(supervise.Options{
				RepoRoot:        opts.RepoRoot,
				ShutdownTimeout: opts.Timeout,
				WrapperExe:      wrapperExe,
			})

			ctx, cancel := context.WithTimeout(cmd.Context(), opts.Timeout)
			defer cancel()

			if err := sup.StopService(ctx, st, serviceName); err != nil {
				return err
			}

			log.Info().Str("service", serviceName).Msg("stopped")
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "ok")
			return nil
		},
	}

	AddRepoFlags(cmd)
	return cmd
}
