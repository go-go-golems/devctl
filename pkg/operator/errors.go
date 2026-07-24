package operator

import (
	"fmt"
)

const (
	CodeUsage                      = "E_USAGE"
	CodeConfigMissing              = "E_CONFIG_MISSING"
	CodeConfigInvalid              = "E_CONFIG_INVALID"
	CodeStateVersion               = "E_STATE_VERSION"
	CodeStateCorrupt               = "E_STATE_CORRUPT"
	CodeOperationBusy              = "E_OPERATION_BUSY"
	CodeServiceUnknown             = "E_SERVICE_UNKNOWN"
	CodeServiceAlreadyRunning      = "E_SERVICE_ALREADY_RUNNING"
	CodeProcessIdentityUnsupported = "E_PROCESS_IDENTITY_UNSUPPORTED"
	CodeProcessIdentityMismatch    = "E_PROCESS_IDENTITY_MISMATCH"
	CodeWrapperStart               = "E_WRAPPER_START"
	CodeWrapperHandshake           = "E_WRAPPER_HANDSHAKE"
	CodeHealthTimeout              = "E_HEALTH_TIMEOUT"
	CodeStopFailed                 = "E_STOP_FAILED"
	CodePartialFailure             = "E_PARTIAL_FAILURE"
	CodeLogCorrupt                 = "E_LOG_CORRUPT"
	CodeCanceled                   = "E_CANCELED"
)

type OperatorError struct {
	Code      string         `json:"code"`
	Message   string         `json:"message"`
	Operation string         `json:"operation,omitempty"`
	Service   string         `json:"service,omitempty"`
	RunID     string         `json:"run_id,omitempty"`
	Path      string         `json:"path,omitempty"`
	Details   map[string]any `json:"details,omitempty"`
	cause     error
}

func (e *OperatorError) Error() string {
	if e.Service != "" {
		return fmt.Sprintf("%s: service %s: %s", e.Code, e.Service, e.Message)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *OperatorError) Unwrap() error {
	return e.cause
}

func newError(code string, message string, cause error) *OperatorError {
	return &OperatorError{Code: code, Message: message, cause: cause}
}
