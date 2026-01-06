package cmds

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-go-golems/devctl/pkg/config"
	"github.com/go-go-golems/devctl/pkg/discovery"
	"github.com/go-go-golems/devctl/pkg/engine"
	"github.com/go-go-golems/devctl/pkg/patch"
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

			cfg, err := config.LoadOptional(opts.Config)
			if err != nil {
				return err
			}
			if !opts.Strict && cfg.Strictness == "error" {
				opts.Strict = true
			}
			specs, err := discovery.Discover(cfg, discovery.Options{RepoRoot: opts.RepoRoot})
			if err != nil {
				return err
			}
			if len(specs) == 0 {
				log.Warn().Msg("no plugins configured (add .devctl.yaml)")
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "{}")
				return nil
			}

			ctx := cmd.Context()
			factory := runtime.NewFactory(runtime.FactoryOptions{
				HandshakeTimeout: 2 * time.Second,
				ShutdownTimeout:  2 * time.Second,
			})

			clients := make([]runtime.Client, 0, len(specs))
			for _, spec := range specs {
				c, err := factory.Start(ctx, spec)
				if err != nil {
					for _, cc := range clients {
						_ = cc.Close(ctx)
					}
					return err
				}
				clients = append(clients, c)
			}
			defer func() {
				closeCtx, cancel := context.WithTimeout(context.Background(), opts.Timeout)
				defer cancel()
				for _, c := range clients {
					_ = c.Close(closeCtx)
				}
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
