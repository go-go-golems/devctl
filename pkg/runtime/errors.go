package runtime

import (
	"errors"
	"fmt"

	"github.com/go-go-golems/devctl/pkg/protocol"
)

var ErrUnsupported = errors.New(protocol.ErrUnsupported)

type OpError struct {
	PluginID string
	Op       string
	Code     string
	Message  string
	Details  map[string]any
}

func (e *OpError) Error() string {
	if e.Message == "" {
		return fmt.Sprintf("%s: plugin=%q op=%q", e.Code, e.PluginID, e.Op)
	}
	return fmt.Sprintf("%s: plugin=%q op=%q: %s", e.Code, e.PluginID, e.Op, e.Message)
}

func (e *OpError) Is(target error) bool {
	if target == ErrUnsupported {
		return e.Code == protocol.ErrUnsupported
	}
	return false
}
