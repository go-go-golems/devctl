//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package supervise

import (
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

func validateChildProcessGroup(childPID int, expectedPGID int) error {
	actualPGID, err := unix.Getpgid(childPID)
	if err != nil {
		return errors.Wrap(err, "read ready child process group")
	}
	if actualPGID != expectedPGID {
		return errors.Errorf("ready child process group %d does not match actual process group %d", expectedPGID, actualPGID)
	}
	return nil
}
