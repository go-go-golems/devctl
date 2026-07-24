package cmds

import (
	"context"
	stderrors "errors"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-go-golems/devctl/pkg/runlog"
	"github.com/go-go-golems/devctl/pkg/runstate"
	"github.com/go-go-golems/devctl/pkg/state"
	"github.com/go-go-golems/devctl/pkg/supervise"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

func newWrapServiceCmd() *cobra.Command {
	var requestPath string

	cmd := &cobra.Command{
		Use:    "__wrap-service --request PATH",
		Short:  "Internal: supervise wrapper to record ownership and exit info",
		Hidden: true,
		Args:   cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			zerolog.SetGlobalLevel(zerolog.Disabled)
			log.Logger = zerolog.New(io.Discard)

			request, err := supervise.LoadWrapperRequest(requestPath)
			if err != nil {
				return err
			}
			if err := os.Remove(requestPath); err != nil {
				return errors.Wrap(err, "remove consumed wrapper request")
			}
			return runWrappedService(request)
		},
	}

	cmd.Flags().StringVar(&requestPath, "request", "", "Versioned wrapper request path")
	_ = cmd.MarkFlagRequired("request")
	return cmd
}

func runWrappedService(request *supervise.WrapperRequest) error {
	if err := request.Validate(); err != nil {
		return err
	}
	if err := syscall.Setpgid(0, 0); err != nil {
		return errors.Wrap(err, "isolate wrapper process group")
	}

	wrapperIdentity, err := runstate.ReadProcessIdentity(os.Getpid())
	if err != nil {
		return errors.Wrap(err, "read wrapper process identity")
	}
	owner := supervise.OwnerRecord{
		Version:   supervise.HandshakeVersion,
		RunID:     request.RunID,
		Service:   request.Service,
		Wrapper:   *wrapperIdentity,
		WrittenAt: time.Now().UTC(),
	}
	if err := supervise.WriteOwnerRecord(request.OwnerPath(), owner); err != nil {
		return err
	}

	stdoutFile, err := os.OpenFile(request.StdoutPath(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return recordWrapperSetupFailure(request, errors.Wrap(err, "open stdout log"))
	}
	defer func() { _ = stdoutFile.Close() }()

	stderrFile, err := os.OpenFile(request.StderrPath(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return recordWrapperSetupFailure(request, errors.Wrap(err, "open stderr log"))
	}
	defer func() { _ = stderrFile.Close() }()

	journalFile, err := os.OpenFile(request.JournalPath(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return recordWrapperSetupFailure(request, errors.Wrap(err, "open structured log journal"))
	}
	defer func() { _ = journalFile.Close() }()

	startedAt := time.Now().UTC()
	// #nosec G204 -- command comes from the validated repository service spec.
	child := exec.Command(request.Command[0], request.Command[1:]...)
	child.Dir = request.Cwd
	child.Env = mergeEnv(os.Environ(), request.Environment)
	child.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	stdoutPipe, err := child.StdoutPipe()
	if err != nil {
		return recordWrapperSetupFailure(request, errors.Wrap(err, "create child stdout pipe"))
	}
	stderrPipe, err := child.StderrPipe()
	if err != nil {
		return recordWrapperSetupFailure(request, errors.Wrap(err, "create child stderr pipe"))
	}

	signalChannel := make(chan os.Signal, 8)
	signal.Notify(signalChannel, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)
	signalDone := make(chan struct{})
	defer func() {
		signal.Stop(signalChannel)
		close(signalDone)
	}()

	if err := child.Start(); err != nil {
		startErr := errors.Wrap(err, "start child")
		if recordErr := writeWrapperExit(request, state.ExitInfo{
			Service:   request.Service,
			StartedAt: startedAt,
			ExitedAt:  time.Now().UTC(),
			Error:     startErr.Error(),
		}); recordErr != nil {
			return errors.Wrapf(startErr, "also failed to write exit record: %v", recordErr)
		}
		return startErr
	}
	captureDone := make(chan error, 1)
	go func() {
		captureDone <- runlog.Capture(
			context.Background(),
			runlog.CaptureOptions{RunID: request.RunID, Service: request.Service},
			journalFile,
			runlog.CaptureStream{Kind: runlog.StreamStdout, Read: stdoutPipe, Raw: stdoutFile},
			runlog.CaptureStream{Kind: runlog.StreamStderr, Read: stderrPipe, Raw: stderrFile},
		)
	}()

	childPGID, err := syscall.Getpgid(child.Process.Pid)
	if err != nil {
		return terminateChildAfterHandshakeFailure(request, child, captureDone, startedAt, errors.Wrap(err, "read child process group"))
	}
	childIdentity, err := runstate.ReadProcessIdentity(child.Process.Pid)
	if err != nil {
		return terminateChildAfterHandshakeFailure(request, child, captureDone, startedAt, errors.Wrap(err, "read child process identity"))
	}
	if childPGID != child.Process.Pid {
		return terminateChildAfterHandshakeFailure(
			request,
			child,
			captureDone,
			startedAt,
			errors.Errorf("child process group %d does not match child PID %d", childPGID, child.Process.Pid),
		)
	}

	go forwardWrapperSignals(signalDone, signalChannel, childPGID)

	ready := supervise.ReadyRecord{
		Version:   supervise.HandshakeVersion,
		RunID:     request.RunID,
		Service:   request.Service,
		Wrapper:   *wrapperIdentity,
		Child:     *childIdentity,
		ChildPGID: childPGID,
		WrittenAt: time.Now().UTC(),
	}
	if err := supervise.WriteReadyRecord(request.ReadyPath(), ready); err != nil {
		return terminateChildAfterHandshakeFailure(request, child, captureDone, startedAt, err)
	}

	waitDone := make(chan error, 1)
	go func() {
		waitDone <- child.Wait()
	}()
	var waitErr error
	var captureErr error
	select {
	case captureErr = <-captureDone:
		if captureErr != nil {
			_ = syscall.Kill(-child.Process.Pid, syscall.SIGKILL)
		}
		waitErr = <-waitDone
	case waitErr = <-waitDone:
		captureErr = <-captureDone
	}
	exitedAt := time.Now().UTC()
	exitInfo := state.ExitInfo{
		Service:   request.Service,
		PID:       child.Process.Pid,
		StartedAt: startedAt,
		ExitedAt:  exitedAt,
	}
	if waitErr != nil {
		exitInfo.Error = waitErr.Error()
		var exitError *exec.ExitError
		if stderrors.As(waitErr, &exitError) {
			if waitStatus, ok := exitError.Sys().(syscall.WaitStatus); ok {
				if waitStatus.Signaled() {
					exitInfo.Signal = waitStatus.Signal().String()
				}
				if waitStatus.Exited() {
					code := waitStatus.ExitStatus()
					exitInfo.ExitCode = &code
				}
			}
		}
	} else {
		code := 0
		exitInfo.ExitCode = &code
	}
	if captureErr != nil {
		if exitInfo.Error == "" {
			exitInfo.Error = captureErr.Error()
		} else {
			exitInfo.Error = exitInfo.Error + "; log capture: " + captureErr.Error()
		}
	}

	if lines, err := state.TailLines(request.StderrPath(), request.TailLines, 2<<20); err == nil {
		exitInfo.StderrTail = lines
	}
	if err := writeWrapperExit(request, exitInfo); err != nil {
		return err
	}

	if exitInfo.ExitCode != nil && *exitInfo.ExitCode != 0 {
		return errors.New("wrapped service exited non-zero")
	}
	if exitInfo.Signal != "" {
		return errors.New("wrapped service exited by signal")
	}
	return nil
}

func forwardWrapperSignals(done <-chan struct{}, signals <-chan os.Signal, childPGID int) {
	for {
		select {
		case <-done:
			return
		case signalValue := <-signals:
			signalNumber, ok := signalValue.(syscall.Signal)
			if ok {
				_ = syscall.Kill(-childPGID, signalNumber)
			}
		}
	}
}

func recordWrapperSetupFailure(request *supervise.WrapperRequest, setupErr error) error {
	recordErr := writeWrapperExit(request, state.ExitInfo{
		Service:  request.Service,
		ExitedAt: time.Now().UTC(),
		Error:    setupErr.Error(),
	})
	if recordErr != nil {
		return errors.Wrapf(setupErr, "also failed to write exit record: %v", recordErr)
	}
	return setupErr
}

func terminateChildAfterHandshakeFailure(
	request *supervise.WrapperRequest,
	child *exec.Cmd,
	captureDone <-chan error,
	startedAt time.Time,
	handshakeErr error,
) error {
	if child.Process != nil {
		_ = syscall.Kill(-child.Process.Pid, syscall.SIGKILL)
		_ = child.Wait()
	}
	captureErr := <-captureDone
	if captureErr != nil {
		handshakeErr = errors.Wrapf(handshakeErr, "log capture also failed: %v", captureErr)
	}
	recordErr := writeWrapperExit(request, state.ExitInfo{
		Service:   request.Service,
		PID:       child.Process.Pid,
		StartedAt: startedAt,
		ExitedAt:  time.Now().UTC(),
		Error:     handshakeErr.Error(),
	})
	if recordErr != nil {
		return errors.Wrapf(handshakeErr, "also failed to write exit record: %v", recordErr)
	}
	return handshakeErr
}

func writeWrapperExit(request *supervise.WrapperRequest, info state.ExitInfo) error {
	if err := state.WriteExitInfo(request.ExitPath(), info); err != nil {
		return errors.Wrap(err, "write wrapper exit record")
	}
	return nil
}

func mergeEnv(base []string, extra map[string]string) []string {
	if len(extra) == 0 {
		return base
	}
	out := append([]string{}, base...)
	for key, value := range extra {
		out = append(out, key+"="+value)
	}
	return out
}
