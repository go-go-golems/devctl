package cmds

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
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

func newUpCmd() *cobra.Command {
	var force bool
	var skipValidate bool

	cmd := &cobra.Command{
		Use:   "up",
		Short: "Start the dev environment (config.mutate + validate.run + launch.plan + supervise)",
		RunE: func(cmd *cobra.Command, args []string) error {
			opts, err := getRootOptions(cmd)
			if err != nil {
				return err
			}

			if _, err := os.Stat(state.StatePath(opts.RepoRoot)); err == nil {
				if !force {
					return errors.New("state exists; run devctl down first or use --force")
				}
				log.Info().Msg("existing state found; stopping first (--force)")
				if err := stopFromState(cmd.Context(), opts); err != nil {
					return err
				}
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
				return errors.New("no plugins configured (add .devctl.yaml)")
			}

			ctx := cmd.Context()
			factory := runtime.NewFactory(runtime.FactoryOptions{
				HandshakeTimeout: 2 * time.Second,
				ShutdownTimeout:  3 * time.Second,
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
				closeCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
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

			opCtx, cancel := context.WithTimeout(ctx, opts.Timeout)
			conf, err := p.MutateConfig(opCtx, patch.Config{})
			cancel()
			if err != nil {
				return err
			}

			if !skipValidate {
				opCtx, cancel = context.WithTimeout(ctx, opts.Timeout)
				vr, err := p.Validate(opCtx, conf)
				cancel()
				if err != nil {
					return err
				}
				if !vr.Valid {
					b, _ := json.MarshalIndent(vr, "", "  ")
					_, _ = fmt.Fprintln(cmd.ErrOrStderr(), string(b))
					return errors.New("validation failed")
				}
			}

			opCtx, cancel = context.WithTimeout(ctx, opts.Timeout)
			plan, err := p.LaunchPlan(opCtx, conf)
			cancel()
			if err != nil {
				return err
			}

			sup := supervise.New(supervise.Options{RepoRoot: opts.RepoRoot, ReadyTimeout: opts.Timeout})
			st, err := sup.Start(ctx, plan)
			if err != nil {
				return err
			}
			if err := state.Save(opts.RepoRoot, st); err != nil {
				_ = sup.Stop(context.Background(), st)
				return err
			}

			log.Info().Int("services", len(st.Services)).Msg("up complete")
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "ok")
			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Stop existing state before starting")
	cmd.Flags().BoolVar(&skipValidate, "skip-validate", false, "Skip validate.run")
	return cmd
}

func stopFromState(ctx context.Context, opts rootOptions) error {
	st, err := state.Load(opts.RepoRoot)
	if err != nil {
		return err
	}
	sup := supervise.New(supervise.Options{RepoRoot: opts.RepoRoot, ReadyTimeout: opts.Timeout})
	stopCtx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()
	_ = sup.Stop(stopCtx, st)
	return state.Remove(opts.RepoRoot)
}
