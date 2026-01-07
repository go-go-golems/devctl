package cmds

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-go-golems/devctl/pkg/engine"
	"github.com/go-go-golems/devctl/pkg/patch"
	"github.com/go-go-golems/devctl/pkg/repository"
	"github.com/go-go-golems/devctl/pkg/runtime"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

func newPlanCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "plan",
		Short: "Compute a merged launch plan from plugins (config.mutate + launch.plan)",
		RunE: func(cmd *cobra.Command, args []string) error {
			opts, err := getRootOptions(cmd)
			if err != nil {
				return err
			}

			meta, err := requestMetaFromRootOptions(opts)
			if err != nil {
				return err
			}
			repo, err := repository.Load(repository.Options{RepoRoot: opts.RepoRoot, ConfigPath: opts.Config, Cwd: meta.Cwd, DryRun: opts.DryRun})
			if err != nil {
				return err
			}
			if !opts.Strict && repo.Config.Strictness == "error" {
				opts.Strict = true
			}
			if len(repo.Specs) == 0 {
				log.Warn().Msg("no plugins configured (add .devctl.yaml)")
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "{}")
				return nil
			}

			ctx := cmd.Context()
			factory := runtime.NewFactory(runtime.FactoryOptions{
				HandshakeTimeout: 2 * time.Second,
				ShutdownTimeout:  2 * time.Second,
			})

			clients, err := repo.StartClients(ctx, factory)
			if err != nil {
				return err
			}
			defer func() {
				closeCtx, cancel := context.WithTimeout(context.Background(), opts.Timeout)
				defer cancel()
				_ = repository.CloseClients(closeCtx, clients)
			}()

			p := &engine.Pipeline{
				Clients: clients,
				Opts: engine.Options{
					Strict: opts.Strict,
					DryRun: opts.DryRun,
				},
			}

			conf := patch.Config{}
			opCtx, cancel := context.WithTimeout(ctx, opts.Timeout)
			conf, err = p.MutateConfig(opCtx, conf)
			cancel()
			if err != nil {
				return err
			}

			opCtx, cancel = context.WithTimeout(ctx, opts.Timeout)
			plan, err := p.LaunchPlan(opCtx, conf)
			cancel()
			if err != nil {
				return err
			}

			out := map[string]any{
				"config": conf,
				"plan":   plan,
			}
			b, err := json.MarshalIndent(out, "", "  ")
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(b))
			log.Info().Int("plugins", len(clients)).Int("services", len(plan.Services)).Msg("plan computed")
			return nil
		},
	}
}
