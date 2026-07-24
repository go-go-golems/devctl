package runstate

import "github.com/pkg/errors"

var ErrProcessIdentityUnsupported = errors.New("process identity is unsupported on this platform")
