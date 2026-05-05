package cmds

import (
	"context"
	"fmt"

	"github.com/go-go-golems/devctl/pkg/state"
	"github.com/go-go-golems/devctl/pkg/supervise"
	"github.com/spf13/cobra"
)

func newDownCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "down",
		Short: "Stop supervised services and remove state",
		RunE: func(cmd *cobra.Command, args []string) error {
			opts, err := getRootOptions(cmd)
			if err != nil {
				return err
			}
			st, err := state.Load(opts.RepoRoot)
			if err != nil {
				return err
			}

			sup := supervise.New(supervise.Options{RepoRoot: opts.RepoRoot, ShutdownTimeout: opts.Timeout})
			stopCtx, cancel := context.WithTimeout(cmd.Context(), opts.Timeout)
			defer cancel()
			_ = sup.Stop(stopCtx, st)
			if err := state.Remove(opts.RepoRoot); err != nil {
				return err
			}
			if st.Profile != "" {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "ok (profile %s)\n", st.Profile)
				return nil
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "ok")
			return nil
		},
	}
	AddRepoFlags(cmd)
	return cmd
}
