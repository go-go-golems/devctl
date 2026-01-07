package protocol

import "encoding/json"

type ProtocolVersion string

const ProtocolV1 ProtocolVersion = "v1"
const ProtocolV2 ProtocolVersion = "v2"

type FrameType string

const (
	FrameHandshake FrameType = "handshake"
	FrameRequest   FrameType = "request"
	FrameResponse  FrameType = "response"
	FrameEvent     FrameType = "event"
)

type Capabilities struct {
	Ops      []string      `json:"ops,omitempty"`
	Streams  []string      `json:"streams,omitempty"`
	Commands []CommandSpec `json:"commands,omitempty"`
}

type CommandSpec struct {
	Name     string       `json:"name"`
	Help     string       `json:"help,omitempty"`
	ArgsSpec []CommandArg `json:"args_spec,omitempty"`
}

type CommandArg struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type Handshake struct {
	Type            FrameType       `json:"type"`
	ProtocolVersion ProtocolVersion `json:"protocol_version"`
	PluginName      string          `json:"plugin_name"`
	Capabilities    Capabilities    `json:"capabilities"`
	Declares        map[string]any  `json:"declares,omitempty"`
}

type RequestContext struct {
	RepoRoot   string `json:"repo_root,omitempty"`
	Cwd        string `json:"cwd,omitempty"`
	DeadlineMs int64  `json:"deadline_ms,omitempty"`
	DryRun     bool   `json:"dry_run,omitempty"`
}

type Request struct {
	Type      FrameType       `json:"type"`
	RequestID string          `json:"request_id"`
	Op        string          `json:"op"`
	Ctx       RequestContext  `json:"ctx"`
	Input     json.RawMessage `json:"input,omitempty"`
}

type Response struct {
	Type      FrameType       `json:"type"`
	RequestID string          `json:"request_id"`
	Ok        bool            `json:"ok"`
	Output    json.RawMessage `json:"output,omitempty"`
	Warnings  []Note          `json:"warnings,omitempty"`
	Notes     []Note          `json:"notes,omitempty"`
	Error     *Error          `json:"error,omitempty"`
}

type Event struct {
	Type     FrameType      `json:"type"`
	StreamID string         `json:"stream_id"`
	Event    string         `json:"event"`
	Level    string         `json:"level,omitempty"`
	Message  string         `json:"message,omitempty"`
	Fields   map[string]any `json:"fields,omitempty"`
	Ok       *bool          `json:"ok,omitempty"`
}

type Error struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

type Note struct {
	Level   string `json:"level"`
	Message string `json:"message"`
}
