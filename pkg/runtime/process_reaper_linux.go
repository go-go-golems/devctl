//go:build linux

package runtime

import (
	"errors"
	"sync"

	"golang.org/x/sys/unix"
)

var (
	subreaperOnce sync.Once
	subreaperErr  error
)

// enableChildSubreaper keeps orphaned plugin descendants owned by devctl so
// they can be reaped instead of depending on the host's PID 1 behavior.
func enableChildSubreaper() error {
	subreaperOnce.Do(func() {
		subreaperErr = unix.Prctl(unix.PR_SET_CHILD_SUBREAPER, 1, 0, 0, 0)
	})
	return subreaperErr
}

// reapProcessGroup reaps adopted descendants from one plugin process group.
// The group leader has already been collected by exec.Cmd.Wait before this is
// called, so Wait4 cannot race with Cmd.Wait for that process.
func reapProcessGroup(pgid int) {
	if pgid <= 0 {
		return
	}
	for {
		var status unix.WaitStatus
		pid, err := unix.Wait4(-pgid, &status, unix.WNOHANG, nil)
		if pid > 0 {
			continue
		}
		if err != nil && errors.Is(err, unix.EINTR) {
			continue
		}
		return
	}
}
