package cmds

import (
	"context"
	"fmt"
	"os"

	"github.com/go-go-golems/devctl/pkg/servicecontrol"
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

The service is stopped (SIGTERM to its process group) and then restarted.
Before starting, devctl re-runs the planning phases (config.mutate + launch.plan)
to recover the effective ServiceSpec without persisting raw environment variables
in state.json. This does not run build, prepare, or validate phases.`,
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

			ctx, cancel := context.WithTimeout(cmd.Context(), opts.Timeout)
			defer cancel()

			spec, err := servicecontrol.ResolveServiceSpec(ctx, servicecontrol.ResolveOptions{
				RepoRoot:   opts.RepoRoot,
				ConfigPath: opts.Config,
				Cwd:        opts.RepoRoot,
				DryRun:     opts.DryRun,
				Strict:     opts.Strict,
				Timeout:    opts.Timeout,
			}, serviceName)
			if err != nil {
				return err
			}

			wrapperExe, _ := os.Executable()
			sup := supervise.New(supervise.Options{
				RepoRoot:        opts.RepoRoot,
				ShutdownTimeout: opts.Timeout,
				ReadyTimeout:    opts.Timeout,
				WrapperExe:      wrapperExe,
			})

			if err := sup.RestartService(ctx, st, spec); err != nil {
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
