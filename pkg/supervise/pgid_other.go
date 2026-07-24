//go:build !aix && !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !solaris

package supervise

import (
	"github.com/go-go-golems/devctl/pkg/runstate"
	"github.com/pkg/errors"
)

func validateChildProcessGroup(_ int, _ int) error {
	return errors.Wrap(runstate.ErrProcessIdentityUnsupported, "validate child process group")
}
