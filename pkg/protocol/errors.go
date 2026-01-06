package protocol

const (
	ErrProtocolStdoutContamination = "E_PROTOCOL_STDOUT_CONTAMINATION"
	ErrProtocolInvalidJSON         = "E_PROTOCOL_INVALID_JSON"
	ErrProtocolUnexpectedFrame     = "E_PROTOCOL_UNEXPECTED_FRAME"
	ErrProtocolInvalidHandshake    = "E_PROTOCOL_INVALID_HANDSHAKE"
	ErrUnsupported                 = "E_UNSUPPORTED"
	ErrTimeout                     = "E_TIMEOUT"
	ErrCanceled                    = "E_CANCELED"
	ErrRuntime                     = "E_RUNTIME"
)
