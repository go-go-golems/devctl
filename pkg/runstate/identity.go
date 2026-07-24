package runstate

import "github.com/pkg/errors"

var ErrProcessIdentityUnsupported = errors.New("process identity is unsupported on this platform")

type ProcessStatus string

const (
	ProcessAbsent   ProcessStatus = "absent"
	ProcessMatches  ProcessStatus = "matches"
	ProcessMismatch ProcessStatus = "mismatch"
)
