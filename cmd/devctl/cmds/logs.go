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
	var tail int

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
				if tail != 0 {
					if err := writeTail(cmd.OutOrStdout(), logPath, tail); err != nil {
						return err
					}
				}
				ctx, cancel := context.WithCancel(cmd.Context())
				defer cancel()
				return followFile(ctx, logPath, cmd.OutOrStdout())
			}
			if tail == 0 {
				b, err := os.ReadFile(logPath)
				if err != nil {
					return err
				}
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(b))
				return nil
			}
			return writeTail(cmd.OutOrStdout(), logPath, tail)
		},
	}

	cmd.Flags().StringVar(&service, "service", "", "Service name")
	cmd.Flags().BoolVar(&stderr, "stderr", false, "Show stderr log instead of stdout")
	cmd.Flags().BoolVar(&follow, "follow", false, "Follow log output")
	cmd.Flags().IntVar(&tail, "tail", 50, "Number of lines to show from the end (0 for all)")
	AddRepoFlags(cmd)
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

func writeTail(w io.Writer, path string, tail int) error {
	lines, err := readTailLines(path, tail)
	if err != nil {
		return err
	}
	for _, line := range lines {
		_, _ = fmt.Fprintln(w, line)
	}
	return nil
}

func readTailLines(path string, tail int) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	if tail <= 0 {
		b, err := io.ReadAll(f)
		if err != nil {
			return nil, err
		}
		text := string(b)
		if text == "" {
			return nil, nil
		}
		return splitLines(text), nil
	}

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	lines := make([]string, 0, tail)
	for scanner.Scan() {
		line := scanner.Text()
		if len(lines) < tail {
			lines = append(lines, line)
			continue
		}
		copy(lines, lines[1:])
		lines[len(lines)-1] = line
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return lines, nil
}

func splitLines(text string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(text); i++ {
		if text[i] == '\n' {
			line := text[start:i]
			if len(line) > 0 && line[len(line)-1] == '\r' {
				line = line[:len(line)-1]
			}
			lines = append(lines, line)
			start = i + 1
		}
	}
	if start < len(text) {
		line := text[start:]
		if len(line) > 0 && line[len(line)-1] == '\r' {
			line = line[:len(line)-1]
		}
		lines = append(lines, line)
	}
	return lines
}
