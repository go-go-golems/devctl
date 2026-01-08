package tui

import (
	"time"

	"github.com/go-go-golems/devctl/pkg/protocol"
)

type StreamStartRequest struct {
	PluginID string         `json:"plugin_id,omitempty"` // optional; may be resolved by op
	Op       string         `json:"op"`
	Input    map[string]any `json:"input,omitempty"`
	Label    string         `json:"label,omitempty"` // optional display name
}

type StreamStopRequest struct {
	StreamKey string `json:"stream_key"`
}

// StreamKey is a local identifier ("plugin/op/hash(input)") and is NOT the protocol stream_id.
type StreamStarted struct {
	StreamKey string    `json:"stream_key"`
	PluginID  string    `json:"plugin_id"`
	Op        string    `json:"op"`
	StreamID  string    `json:"stream_id"` // protocol stream_id
	At        time.Time `json:"at"`
}

type StreamEvent struct {
	StreamKey string         `json:"stream_key"`
	PluginID  string         `json:"plugin_id"`
	Op        string         `json:"op"`
	StreamID  string         `json:"stream_id"`
	At        time.Time      `json:"at"`
	Event     protocol.Event `json:"event"`
}

type StreamEnded struct {
	StreamKey string    `json:"stream_key"`
	PluginID  string    `json:"plugin_id"`
	Op        string    `json:"op"`
	StreamID  string    `json:"stream_id"`
	At        time.Time `json:"at"`
	Ok        bool      `json:"ok"`
	Error     string    `json:"error,omitempty"`
}
