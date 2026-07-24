//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package runstate

import (
	"context"
	stderrors "errors"
	"os"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

func withFileLock(
	ctx context.Context,
	path string,
	pollInterval time.Duration,
	metadata LockMetadata,
	fn func(context.Context) error,
) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return errors.Wrap(err, "open repository lock")
	}
	defer func() { _ = file.Close() }()

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		err = unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB)
		if err == nil {
			break
		}
		if !stderrors.Is(err, unix.EWOULDBLOCK) && !stderrors.Is(err, unix.EAGAIN) {
			return errors.Wrap(err, "acquire repository lock")
		}
		select {
		case <-ctx.Done():
			return &BusyError{Owner: readLockMetadata(path), Cause: ctx.Err()}
		case <-ticker.C:
		}
	}
	defer func() { _ = unix.Flock(int(file.Fd()), unix.LOCK_UN) }()

	if err := writeLockMetadata(file, metadata); err != nil {
		return err
	}
	return fn(ctx)
}
