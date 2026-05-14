package cmds

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/go-go-golems/devctl/pkg/engine"
	"github.com/go-go-golems/devctl/pkg/patch"
	"github.com/go-go-golems/devctl/pkg/repository"
	"github.com/go-go-golems/devctl/pkg/runtime"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

type phaseRunner struct {
	opts rootOptions
	repo *repository.Repository
	pipe *engine.Pipeline
}

func withPhaseRunner(cmd *cobra.Command, fn func(context.Context, *phaseRunner, patch.Config) error) error {
	opts, err := getRootOptions(cmd)
	if err != nil {
		return err
	}

	meta, err := requestMetaFromRootOptions(opts)
	if err != nil {
		return err
	}
	repo, err := repository.Load(repository.Options{RepoRoot: opts.RepoRoot, ConfigPath: opts.Config, ProfileName: opts.Profile, Cwd: meta.Cwd, DryRun: opts.DryRun})
	if err != nil {
		return err
	}
	if !opts.Strict && repo.Config.Strictness == "error" {
		opts.Strict = true
	}
	if len(repo.Specs) == 0 {
		return errors.New("no plugins configured (add .devctl.yaml)")
	}

	factory := runtime.NewFactory(runtime.FactoryOptions{
		HandshakeTimeout: 2 * time.Second,
		ShutdownTimeout:  3 * time.Second,
	})
	clients, err := repo.StartClients(cmd.Context(), factory)
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

	opCtx, cancel := context.WithTimeout(cmd.Context(), opts.Timeout)
	conf, err := p.MutateConfig(opCtx, patch.Config{})
	cancel()
	if err != nil {
		return err
	}

	return fn(cmd.Context(), &phaseRunner{opts: opts, repo: repo, pipe: p}, conf)
}

func printIndentedJSON(w io.Writer, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(w, string(b))
	return err
}

func newBuildCmd() *cobra.Command {
	var steps []string

	cmd := &cobra.Command{
		Use:   "build",
		Short: "Run the build phase (config.mutate + build.run)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return withPhaseRunner(cmd, func(ctx context.Context, r *phaseRunner, conf patch.Config) error {
				opCtx, cancel := context.WithTimeout(ctx, r.opts.Timeout)
				br, err := r.pipe.Build(opCtx, conf, steps)
				cancel()
				if err != nil {
					return err
				}
				return printIndentedJSON(cmd.OutOrStdout(), map[string]any{
					"config": conf,
					"build":  br,
				})
			})
		},
	}
	cmd.Flags().StringSliceVar(&steps, "step", nil, "Build step name (repeatable)")
	AddRepoFlags(cmd)
	return cmd
}

func newPrepareCmd() *cobra.Command {
	var steps []string

	cmd := &cobra.Command{
		Use:   "prepare",
		Short: "Run the prepare phase (config.mutate + prepare.run)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return withPhaseRunner(cmd, func(ctx context.Context, r *phaseRunner, conf patch.Config) error {
				opCtx, cancel := context.WithTimeout(ctx, r.opts.Timeout)
				pr, err := r.pipe.Prepare(opCtx, conf, steps)
				cancel()
				if err != nil {
					return err
				}
				return printIndentedJSON(cmd.OutOrStdout(), map[string]any{
					"config":  conf,
					"prepare": pr,
				})
			})
		},
	}
	cmd.Flags().StringSliceVar(&steps, "step", nil, "Prepare step name (repeatable)")
	AddRepoFlags(cmd)
	return cmd
}

func newValidateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Run the validation phase (config.mutate + validate.run)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return withPhaseRunner(cmd, func(ctx context.Context, r *phaseRunner, conf patch.Config) error {
				opCtx, cancel := context.WithTimeout(ctx, r.opts.Timeout)
				vr, err := r.pipe.Validate(opCtx, conf)
				cancel()
				if err != nil {
					return err
				}

				if err := printIndentedJSON(cmd.OutOrStdout(), map[string]any{
					"config":   conf,
					"validate": vr,
				}); err != nil {
					return err
				}
				if !vr.Valid {
					return errors.New("validation failed")
				}
				return nil
			})
		},
	}
	AddRepoFlags(cmd)
	return cmd
}
