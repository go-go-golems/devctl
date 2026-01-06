package tui

type StateSnapshotMsg struct {
	Snapshot StateSnapshot
}

type EventLogAppendMsg struct {
	Entry EventLogEntry
}

type NavigateToServiceMsg struct {
	Name string
}

type ActionRequestMsg struct {
	Request ActionRequest
}
