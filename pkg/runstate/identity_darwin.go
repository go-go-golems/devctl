//go:build darwin

package runstate

import (
	stderrors "errors"
	"fmt"

	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

const darwinZombieState = 5

func ReadProcessGroupID(pid int) (int, error) {
	if pid <= 0 {
		return 0, errors.Wrapf(ErrInvalidState, "invalid process PID %d", pid)
	}
	pgid, err := unix.Getpgid(pid)
	if err != nil {
		return 0, errors.Wrap(err, "read Darwin process group")
	}
	return pgid, nil
}

func ReadProcessIdentity(pid int) (*ProcessIdentity, error) {
	if pid <= 0 {
		return nil, errors.Wrapf(ErrInvalidState, "invalid process PID %d", pid)
	}
	process, err := unix.SysctlKinfoProc("kern.proc.pid", pid)
	if err != nil {
		return nil, errors.Wrap(err, "read Darwin process information")
	}
	if int(process.Proc.P_pid) != pid {
		return nil, errors.Wrapf(ErrInvalidState, "Darwin process information returned PID %d for %d", process.Proc.P_pid, pid)
	}
	start := process.Proc.P_starttime
	return &ProcessIdentity{
		PID:        pid,
		StartToken: fmt.Sprintf("darwin:%d:%d", start.Sec, start.Usec),
	}, nil
}

func MatchesProcess(identity *ProcessIdentity) (bool, error) {
	status, err := InspectProcess(identity)
	if err != nil {
		return false, err
	}
	return status == ProcessMatches, nil
}

func InspectProcess(identity *ProcessIdentity) (ProcessStatus, error) {
	if identity == nil || identity.PID <= 0 || identity.StartToken == "" {
		return ProcessAbsent, errors.Wrap(ErrInvalidState, "incomplete process identity")
	}
	current, err := ReadProcessIdentity(identity.PID)
	if err != nil {
		cause := errors.Cause(err)
		if stderrors.Is(cause, unix.ESRCH) || stderrors.Is(cause, unix.ENOENT) {
			return ProcessAbsent, nil
		}
		return ProcessAbsent, err
	}
	if current.StartToken != identity.StartToken {
		return ProcessMismatch, nil
	}
	process, err := unix.SysctlKinfoProc("kern.proc.pid", identity.PID)
	if err != nil {
		cause := errors.Cause(err)
		if stderrors.Is(cause, unix.ESRCH) || stderrors.Is(cause, unix.ENOENT) {
			return ProcessAbsent, nil
		}
		return ProcessAbsent, errors.Wrap(err, "inspect Darwin process state")
	}
	if process.Proc.P_stat == darwinZombieState {
		return ProcessAbsent, nil
	}
	return ProcessMatches, nil
}
