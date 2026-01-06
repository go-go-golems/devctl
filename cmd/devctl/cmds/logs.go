package cmds

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/go-go-golems/devctl/pkg/state"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

func newLogsCmd() *cobra.Command {
	var service string
	var stderr bool
	var follow bool

	cmd := &cobra.Command{
		Use:   "logs",
		Short: "Show logs for a supervised service",
		RunE: func(cmd *cobra.Command, args []string) error {
			if service == "" {
				return errors.New("--service is required")
			}
			opts, err := getRootOptions(cmd)
			if err != nil {
				return err
			}
			st, err := state.Load(opts.RepoRoot)
			if err != nil {
				return err
			}
			var logPath string
			for _, s := range st.Services {
				if s.Name == service {
					if stderr {
						logPath = s.StderrLog
					} else {
						logPath = s.StdoutLog
					}
					break
				}
			}
			if logPath == "" {
				return errors.Errorf("unknown service %q", service)
			}

			if follow {
				ctx, cancel := context.WithCancel(cmd.Context())
				defer cancel()
				return followFile(ctx, logPath, cmd.OutOrStdout())
			}
			b, err := os.ReadFile(logPath)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(b))
			return nil
		},
	}

	cmd.Flags().StringVar(&service, "service", "", "Service name")
	cmd.Flags().BoolVar(&stderr, "stderr", false, "Show stderr log instead of stdout")
	cmd.Flags().BoolVar(&follow, "follow", false, "Follow log output")
	return cmd
}

func followFile(ctx context.Context, path string, w io.Writer) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	_, _ = f.Seek(0, io.SeekEnd)
	r := bufio.NewReader(f)

	for {
		line, err := r.ReadString('\n')
		if err == nil {
			_, _ = w.Write([]byte(line))
			continue
		}
		if errors.Is(err, io.EOF) {
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(200 * time.Millisecond):
				continue
			}
		}
		return err
	}
}
