//go:build !aix && !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !solaris

package runstate

import (
	"context"
	"time"

	"github.com/pkg/errors"
)

func withFileLock(
	_ context.Context,
	_ string,
	_ time.Duration,
	_ LockMetadata,
	_ func(context.Context) error,
) error {
	return errors.Wrap(ErrLockUnsupported, "acquire repository lock")
}
