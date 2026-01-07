package cmds

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/go-go-golems/devctl/pkg/engine"
	"github.com/go-go-golems/devctl/pkg/patch"
	"github.com/go-go-golems/devctl/pkg/repository"
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
	var skipBuild bool
	var skipPrepare bool
	var buildSteps []string
	var prepareSteps []string

	cmd := &cobra.Command{
		Use:   "up",
		Short: "Start the dev environment (config.mutate + validate.run + launch.plan + supervise)",
		RunE: func(cmd *cobra.Command, args []string) error {
			opts, err := getRootOptions(cmd)
			if err != nil {
				return err
			}

			if !opts.DryRun {
				if _, err := os.Stat(state.StatePath(opts.RepoRoot)); err == nil {
					if !force {
						aliveCount, err := countAliveFromState(opts.RepoRoot)
						if err != nil {
							return err
						}

						prompt := "state exists; run devctl down first or use --force"
						if aliveCount == 0 {
							prompt = "state exists but no services appear alive; remove state and continue? (y/N): "
						} else {
							prompt = "state exists; restart (down then up)? (y/N): "
						}

						if isInteractive(cmd.InOrStdin()) {
							ok, err := promptConfirm(cmd.ErrOrStderr(), cmd.InOrStdin(), prompt)
							if err != nil {
								return err
							}
							if !ok {
								return errors.New("aborted")
							}
							log.Info().Msg("existing state found; stopping first (confirmed)")
							if err := stopFromState(cmd.Context(), opts); err != nil {
								return err
							}
						} else {
							return errors.New("state exists; run devctl down first or use --force")
						}
					} else {
						log.Info().Msg("existing state found; stopping first (--force)")
						if err := stopFromState(cmd.Context(), opts); err != nil {
							return err
						}
					}
				}
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
				return errors.New("no plugins configured (add .devctl.yaml)")
			}

			ctx := cmd.Context()
			factory := runtime.NewFactory(runtime.FactoryOptions{
				HandshakeTimeout: 2 * time.Second,
				ShutdownTimeout:  3 * time.Second,
			})

			clients, err := repo.StartClients(ctx, factory)
			if err != nil {
				return err
			}
			defer func() {
				closeCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
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

			opCtx, cancel := context.WithTimeout(ctx, opts.Timeout)
			conf, err := p.MutateConfig(opCtx, patch.Config{})
			cancel()
			if err != nil {
				return err
			}

			out := map[string]any{
				"config": conf,
			}

			if !skipBuild {
				opCtx, cancel = context.WithTimeout(ctx, opts.Timeout)
				br, err := p.Build(opCtx, conf, buildSteps)
				cancel()
				if err != nil {
					return err
				}
				out["build"] = br
			}

			if !skipPrepare {
				opCtx, cancel = context.WithTimeout(ctx, opts.Timeout)
				pr, err := p.Prepare(opCtx, conf, prepareSteps)
				cancel()
				if err != nil {
					return err
				}
				out["prepare"] = pr
			}

			if !skipValidate {
				opCtx, cancel = context.WithTimeout(ctx, opts.Timeout)
				vr, err := p.Validate(opCtx, conf)
				cancel()
				if err != nil {
					return err
				}
				out["validate"] = vr
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
			out["plan"] = plan

			if opts.DryRun {
				b, err := json.MarshalIndent(out, "", "  ")
				if err != nil {
					return err
				}
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(b))
				log.Info().Int("services", len(plan.Services)).Msg("dry-run complete")
				return nil
			}

			wrapperExe, _ := os.Executable()
			sup := supervise.New(supervise.Options{
				RepoRoot:     opts.RepoRoot,
				ReadyTimeout: opts.Timeout,
				WrapperExe:   wrapperExe,
			})
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
	cmd.Flags().BoolVar(&skipBuild, "skip-build", false, "Skip build.run")
	cmd.Flags().BoolVar(&skipPrepare, "skip-prepare", false, "Skip prepare.run")
	cmd.Flags().StringSliceVar(&buildSteps, "build-step", nil, "Build step name (repeatable)")
	cmd.Flags().StringSliceVar(&prepareSteps, "prepare-step", nil, "Prepare step name (repeatable)")
	return cmd
}

func stopFromState(ctx context.Context, opts rootOptions) error {
	st, err := state.Load(opts.RepoRoot)
	if err != nil {
		return err
	}
	wrapperExe, _ := os.Executable()
	sup := supervise.New(supervise.Options{
		RepoRoot:     opts.RepoRoot,
		ReadyTimeout: opts.Timeout,
		WrapperExe:   wrapperExe,
	})
	stopCtx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()
	_ = sup.Stop(stopCtx, st)
	return state.Remove(opts.RepoRoot)
}

func isInteractive(r io.Reader) bool {
	f, ok := r.(*os.File)
	if !ok {
		return false
	}
	st, err := f.Stat()
	if err != nil {
		return false
	}
	return st.Mode()&os.ModeCharDevice != 0
}

func promptConfirm(out io.Writer, in io.Reader, prompt string) (bool, error) {
	if _, err := fmt.Fprint(out, prompt); err != nil {
		return false, err
	}

	br := bufio.NewReader(in)
	line, err := br.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, err
	}

	line = strings.TrimSpace(strings.ToLower(line))
	if line == "y" || line == "yes" {
		return true, nil
	}
	return false, nil
}

func countAliveFromState(repoRoot string) (int, error) {
	st, err := state.Load(repoRoot)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, svc := range st.Services {
		if state.ProcessAlive(svc.PID) {
			n++
		}
	}
	return n, nil
}
