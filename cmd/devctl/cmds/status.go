package cmds

import (
	"encoding/json"
	"fmt"

	"github.com/go-go-golems/devctl/pkg/state"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show status of supervised services",
		RunE: func(cmd *cobra.Command, args []string) error {
			opts, err := getRootOptions(cmd)
			if err != nil {
				return err
			}
			st, err := state.Load(opts.RepoRoot)
			if err != nil {
				return err
			}

			type svc struct {
				Name   string `json:"name"`
				PID    int    `json:"pid"`
				Alive  bool   `json:"alive"`
				Stdout string `json:"stdout_log"`
				Stderr string `json:"stderr_log"`
			}
			var services []svc
			for _, s := range st.Services {
				services = append(services, svc{
					Name:   s.Name,
					PID:    s.PID,
					Alive:  state.ProcessAlive(s.PID),
					Stdout: s.StdoutLog,
					Stderr: s.StderrLog,
				})
			}

			b, err := json.MarshalIndent(map[string]any{"services": services}, "", "  ")
			if err != nil {
				return errors.Wrap(err, "marshal status")
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(b))
			return nil
		},
	}
}
