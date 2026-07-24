package runlog

import (
	"context"
	"time"
)

type SourceKind string
type StreamKind string

const (
	SourceService  SourceKind = "service"
	SourcePipeline SourceKind = "pipeline"
	SourcePlugin   SourceKind = "plugin"
	SourceSystem   SourceKind = "system"

	StreamStdout StreamKind = "stdout"
	StreamStderr StreamKind = "stderr"
	StreamEvent  StreamKind = "event"
)

type LogRecord struct {
	Version  int            `json:"version"`
	RunID    string         `json:"run_id"`
	Sequence uint64         `json:"sequence"`
	Time     time.Time      `json:"time"`
	Source   SourceKind     `json:"source"`
	Service  string         `json:"service,omitempty"`
	Stream   StreamKind     `json:"stream"`
	Level    string         `json:"level,omitempty"`
	Text     string         `json:"text"`
	Partial  bool           `json:"partial,omitempty"`
	Fields   map[string]any `json:"fields,omitempty"`
}

type Cursor struct {
	RunID    string `json:"run_id"`
	Sequence uint64 `json:"sequence"`
}

type Query struct {
	RunIDs   []string
	Services []string
	Sources  []SourceKind
	Streams  []StreamKind
	Levels   []string
	Since    *time.Time
	Until    *time.Time
	Tail     int
	Contains string
}

type FollowRequest struct {
	Query Query
	After map[string]Cursor
}

type LogSink interface {
	Add(context.Context, LogRecord) error
}

type Reader interface {
	Query(context.Context, Query) ([]LogRecord, error)
	Follow(context.Context, FollowRequest, LogSink) error
}
