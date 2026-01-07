package cmds

import (
	stderrors "errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/go-go-golems/devctl/pkg/state"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

func newWrapServiceCmd() *cobra.Command {
	var serviceName string
	var cwd string
	var stdoutLog string
	var stderrLog string
	var exitInfoPath string
	var readyFile string
	var envPairs []string
	var tailLines int

	cmd := &cobra.Command{
		Use:    "__wrap-service -- [cmd args...]",
		Short:  "Internal: supervise wrapper to record exit info",
		Hidden: true,
		Args:   cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			zerolog.SetGlobalLevel(zerolog.Disabled)
			log.Logger = zerolog.New(io.Discard)

			if serviceName == "" {
				return errors.New("missing --service")
			}
			if cwd == "" {
				return errors.New("missing --cwd")
			}
			if stdoutLog == "" || stderrLog == "" {
				return errors.New("missing --stdout-log or --stderr-log")
			}
			if exitInfoPath == "" {
				return errors.New("missing --exit-info")
			}

			if err := os.MkdirAll(filepath.Dir(stdoutLog), 0o755); err != nil {
				return errors.Wrap(err, "mkdir stdout dir")
			}
			if err := os.MkdirAll(filepath.Dir(stderrLog), 0o755); err != nil {
				return errors.Wrap(err, "mkdir stderr dir")
			}
			if err := os.MkdirAll(filepath.Dir(exitInfoPath), 0o755); err != nil {
				return errors.Wrap(err, "mkdir exit dir")
			}

			stdoutFile, err := os.OpenFile(stdoutLog, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
			if err != nil {
				return errors.Wrap(err, "open stdout log")
			}
			defer func() { _ = stdoutFile.Close() }()

			stderrFile, err := os.OpenFile(stderrLog, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
			if err != nil {
				return errors.Wrap(err, "open stderr log")
			}
			defer func() { _ = stderrFile.Close() }()

			startedAt := time.Now()

			if err := syscall.Setpgid(0, 0); err != nil {
				return errors.Wrap(err, "setpgid")
			}

			child := exec.Command(args[0], args[1:]...) //nolint:gosec
			child.Dir = cwd
			child.Env = mergeEnv(os.Environ(), parseEnvPairs(envPairs))
			child.Stdout = stdoutFile
			child.Stderr = stderrFile

			pgid := os.Getpid()
			child.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pgid: pgid}

			sigCh := make(chan os.Signal, 8)
			signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)
			defer signal.Stop(sigCh)
			go func() {
				for s := range sigCh {
					_ = syscall.Kill(-pgid, s.(syscall.Signal))
				}
			}()

			if err := child.Start(); err != nil {
				_ = state.WriteExitInfo(exitInfoPath, state.ExitInfo{
					Service:    serviceName,
					PID:        0,
					StartedAt:  startedAt,
					ExitedAt:   time.Now(),
					Error:      errors.Wrap(err, "start").Error(),
					StderrTail: nil,
				})
				return errors.Wrap(err, "start child")
			}

			if readyFile != "" {
				_ = os.MkdirAll(filepath.Dir(readyFile), 0o755)
				_ = os.WriteFile(readyFile, []byte(fmt.Sprintf("%d\n", child.Process.Pid)), 0o644)
			}

			waitErr := child.Wait()
			exitedAt := time.Now()

			exitInfo := state.ExitInfo{
				Service:   serviceName,
				PID:       child.Process.Pid,
				StartedAt: startedAt,
				ExitedAt:  exitedAt,
			}

			if waitErr != nil {
				exitInfo.Error = waitErr.Error()
				var ee *exec.ExitError
				if stderrors.As(waitErr, &ee) {
					if ws, ok := ee.Sys().(syscall.WaitStatus); ok {
						if ws.Signaled() {
							exitInfo.Signal = ws.Signal().String()
						}
						if ws.Exited() {
							code := ws.ExitStatus()
							exitInfo.ExitCode = &code
						}
					}
				}
			} else {
				code := 0
				exitInfo.ExitCode = &code
			}

			_ = stdoutFile.Sync()
			_ = stderrFile.Sync()

			if tailLines <= 0 {
				tailLines = 25
			}
			if lines, err := state.TailLines(stderrLog, tailLines, 2<<20); err == nil {
				exitInfo.StderrTail = lines
			}

			_ = state.WriteExitInfo(exitInfoPath, exitInfo)

			if exitInfo.ExitCode != nil && *exitInfo.ExitCode != 0 {
				return errors.New("wrapped service exited non-zero")
			}
			if exitInfo.Signal != "" {
				return errors.New("wrapped service exited by signal")
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&serviceName, "service", "", "Service name")
	cmd.Flags().StringVar(&cwd, "cwd", "", "Working directory")
	cmd.Flags().StringVar(&stdoutLog, "stdout-log", "", "Stdout log path")
	cmd.Flags().StringVar(&stderrLog, "stderr-log", "", "Stderr log path")
	cmd.Flags().StringVar(&exitInfoPath, "exit-info", "", "Exit info JSON path")
	cmd.Flags().StringVar(&readyFile, "ready-file", "", "Write child PID to this file once started")
	cmd.Flags().StringSliceVar(&envPairs, "env", nil, "Extra env (KEY=VAL), repeatable")
	cmd.Flags().IntVar(&tailLines, "tail-lines", 25, "How many stderr lines to record on exit")
	return cmd
}

func parseEnvPairs(pairs []string) map[string]string {
	out := map[string]string{}
	for _, p := range pairs {
		k, v, ok := strings.Cut(p, "=")
		if !ok || k == "" {
			continue
		}
		out[k] = v
	}
	return out
}

func mergeEnv(base []string, extra map[string]string) []string {
	if len(extra) == 0 {
		return base
	}
	out := append([]string{}, base...)
	for k, v := range extra {
		out = append(out, k+"="+v)
	}
	return out
}
