package smoketest

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/go-go-golems/devctl/pkg/engine"
	"github.com/go-go-golems/devctl/pkg/state"
	"github.com/go-go-golems/devctl/pkg/supervise"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

func newLogsCmd() *cobra.Command {
	var timeout time.Duration

	cmd := &cobra.Command{
		Use:   "logs",
		Short: "Smoke test: follow logs and cancel promptly",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
			defer cancel()

			repoRoot, err := os.MkdirTemp("", "devctl-smoketest-logs-*")
			if err != nil {
				return err
			}
			defer func() { _ = os.RemoveAll(repoRoot) }()

			devctlRoot := findDevctlRootFromCaller()
			binDir := filepath.Join(repoRoot, "bin")
			if err := os.MkdirAll(binDir, 0o755); err != nil {
				return err
			}
			logSpewerBin := filepath.Join(binDir, "log-spewer")
			if err := buildLogSpewer(ctx, devctlRoot, logSpewerBin); err != nil {
				return err
			}

			sup := supervise.New(supervise.Options{RepoRoot: repoRoot, ReadyTimeout: 2 * time.Second})
			st, err := sup.Start(ctx, engine.LaunchPlan{
				Services: []engine.ServiceSpec{
					{
						Name:    "spewer",
						Command: []string{logSpewerBin, "--interval", "25ms", "--lines", "50"},
					},
				},
			})
			if err != nil {
				return err
			}
			defer func() { _ = sup.Stop(context.Background(), st) }()

			if err := state.Save(repoRoot, st); err != nil {
				return err
			}

			var stdoutLog string
			for _, svc := range st.Services {
				if svc.Name == "spewer" {
					stdoutLog = svc.StdoutLog
				}
			}
			if stdoutLog == "" {
				return errors.New("missing spewer stdout log path")
			}

			var buf bytes.Buffer
			followCtx, followCancel := context.WithCancel(ctx)
			done := make(chan error, 1)
			go func() {
				done <- followFile(followCtx, stdoutLog, &buf)
			}()

			time.Sleep(250 * time.Millisecond)
			followCancel()

			select {
			case err := <-done:
				if err != nil {
					return err
				}
			case <-time.After(2 * time.Second):
				return errors.New("follow did not stop promptly after cancel")
			}

			if buf.Len() == 0 {
				return errors.New("expected some log output while following")
			}

			if err := sup.Stop(ctx, st); err != nil {
				return err
			}
			if err := state.Remove(repoRoot); err != nil {
				return err
			}

			out := map[string]any{"ok": true, "bytes": buf.Len()}
			b, _ := json.MarshalIndent(out, "", "  ")
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(b))
			log.Info().Msg("smoketest logs ok")
			return nil
		},
	}

	cmd.Flags().DurationVar(&timeout, "timeout", 15*time.Second, "Overall timeout for the smoketest")
	return cmd
}

func buildLogSpewer(ctx context.Context, devctlRoot string, outPath string) error {
	c := exec.CommandContext(ctx, "go", "build", "-o", outPath, "./testapps/cmd/log-spewer")
	c.Dir = devctlRoot
	c.Env = append(os.Environ(), "GOWORK=off")
	b, err := c.CombinedOutput()
	if err != nil {
		return errors.Wrapf(err, "build log-spewer: %s", string(b))
	}
	return nil
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
