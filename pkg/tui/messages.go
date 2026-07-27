package tui

import (
	"github.com/go-go-golems/devctl/pkg/operator"
	"github.com/go-go-golems/devctl/pkg/runlog"
)

type SnapshotMsg struct {
	Snapshot operator.Snapshot
}

type EventMsg struct {
	Event operator.OperatorEvent
}

type LogMsg struct {
	Record runlog.LogRecord
}

type OperationDoneMsg struct {
	Result operator.OperationResult
	Events []operator.OperatorEvent
	Err    error
}

type DoctorMsg struct {
	Report operator.DoctorReport
	Err    error
}

type ErrorMsg struct {
	Operation string
	Err       error
}

type snapshotTickMsg struct{}

type followStoppedMsg struct {
	Err error
}

type ViewKind int

const (
	ViewOverview ViewKind = iota
	ViewLogs
	ViewRuns
)

func (v ViewKind) String() string {
	switch v {
	case ViewOverview:
		return "Overview"
	case ViewLogs:
		return "Logs"
	case ViewRuns:
		return "Runs"
	default:
		return "Unknown"
	}
}
