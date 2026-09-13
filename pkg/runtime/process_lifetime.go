package runtime

import (
	"context"
	stderrors "errors"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/pkg/errors"
)

type ShutdownMode string

const (
	ShutdownEOF         ShutdownMode = "eof"
	ShutdownTerminated  ShutdownMode = "term"
	ShutdownKilled      ShutdownMode = "kill"
	ShutdownUnconfirmed ShutdownMode = "unconfirmed"
)

type ShutdownResult struct {
	Mode     ShutdownMode
	ExitCode *int
	Error    error
}

type processExit struct {
	err      error
	exitCode *int
}

type processLifetime struct {
	cmd          *exec.Cmd
	pgid         int
	done         chan struct{}
	exit         processExit
	shutdownOnce sync.Once
	shutdownDone chan struct{}
	shutdown     ShutdownResult
}

func newProcessLifetime(cmd *exec.Cmd) *processLifetime {
	lifetime := &processLifetime{cmd: cmd, done: make(chan struct{}), shutdownDone: make(chan struct{})}
	if cmd != nil && cmd.Process != nil {
		if pgid, err := syscall.Getpgid(cmd.Process.Pid); err == nil {
			lifetime.pgid = pgid
		}
	}
	go func() {
		err := cmd.Wait()
		lifetime.exit = processExit{err: err, exitCode: processExitCode(err)}
		close(lifetime.done)
	}()
	return lifetime
}

func processExitCode(err error) *int {
	var exitErr *exec.ExitError
	if !stderrors.As(err, &exitErr) {
		if err == nil {
			code := 0
			return &code
		}
		return nil
	}
	code := exitErr.ExitCode()
	return &code
}

func (l *processLifetime) beginShutdown(closeInput func() error, eofGrace, signalGrace time.Duration) {
	l.shutdownOnce.Do(func() {
		go func() {
			defer close(l.shutdownDone)
			if closeInput != nil {
				if err := closeInput(); err != nil && !stderrors.Is(err, syscall.EPIPE) {
					l.shutdown.Error = errors.Wrap(err, "close plugin stdin")
				}
			}
			if l.wait(eofGrace) {
				l.shutdown.Mode = ShutdownEOF
				l.shutdown.ExitCode = l.exit.exitCode
				if l.exit.err != nil {
					l.shutdown.Error = stderrors.Join(l.shutdown.Error, errors.Wrap(l.exit.err, "plugin exited after EOF"))
				}
				return
			}
			if err := l.signal(syscall.SIGTERM); err != nil {
				l.shutdown.Error = stderrors.Join(l.shutdown.Error, err)
			}
			if l.wait(signalGrace) {
				l.shutdown.Mode = ShutdownTerminated
				l.shutdown.ExitCode = l.exit.exitCode
				return
			}
			if err := l.signal(syscall.SIGKILL); err != nil {
				l.shutdown.Error = stderrors.Join(l.shutdown.Error, err)
			}
			if l.wait(signalGrace) {
				l.shutdown.Mode = ShutdownKilled
				l.shutdown.ExitCode = l.exit.exitCode
				return
			}
			l.shutdown.Mode = ShutdownUnconfirmed
			l.shutdown.Error = stderrors.Join(l.shutdown.Error, errors.New("plugin process exit remained unconfirmed after SIGKILL"))
		}()
	})
}

func (l *processLifetime) awaitShutdown(ctx context.Context) (ShutdownResult, error) {
	select {
	case <-l.shutdownDone:
		return l.shutdown, l.shutdown.Error
	case <-ctx.Done():
		return ShutdownResult{}, ctx.Err()
	}
}

func (l *processLifetime) wait(timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for {
		pluginExited := false
		select {
		case <-l.done:
			pluginExited = true
		default:
		}
		if pluginExited && !l.processGroupAlive() {
			return true
		}
		if timeout <= 0 || !time.Now().Before(deadline) {
			return false
		}
		delay := 10 * time.Millisecond
		if remaining := time.Until(deadline); remaining < delay {
			delay = remaining
		}
		timer := time.NewTimer(delay)
		<-timer.C
	}
}

func (l *processLifetime) processGroupAlive() bool {
	if l.pgid <= 0 {
		return false
	}
	err := syscall.Kill(-l.pgid, 0)
	return err == nil || stderrors.Is(err, syscall.EPERM)
}

func (l *processLifetime) signal(signal syscall.Signal) error {
	if l.pgid > 0 {
		if err := syscall.Kill(-l.pgid, signal); err != nil && !stderrors.Is(err, syscall.ESRCH) {
			return errors.Wrapf(err, "signal plugin process group with %s", signal)
		}
		return nil
	}
	select {
	case <-l.done:
		return nil
	default:
	}
	if l.cmd == nil || l.cmd.Process == nil {
		return nil
	}
	if err := l.cmd.Process.Signal(signal); err != nil && !stderrors.Is(err, os.ErrProcessDone) {
		return errors.Wrapf(err, "signal plugin process with %s", signal)
	}
	return nil
}
