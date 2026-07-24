package runstate

import "github.com/pkg/errors"

var ErrProcessIdentityUnsupported = errors.New("process identity is unsupported on this platform")
var ErrProcessGroupUnsupported = errors.New("process group inspection is unsupported on this platform")

type ProcessStatus string

const (
	ProcessAbsent   ProcessStatus = "absent"
	ProcessMatches  ProcessStatus = "matches"
	ProcessMismatch ProcessStatus = "mismatch"
)
