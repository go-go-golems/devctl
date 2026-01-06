package tui

const (
	TopicDevctlEvents = "devctl.events"
	TopicUIMessages   = "devctl.ui.msgs"
	TopicUIActions    = "devctl.ui.actions"
)

const (
	DomainTypeStateSnapshot = "state.snapshot"
	DomainTypeServiceExit   = "service.exit.observed"
	DomainTypeActionLog     = "action.log"
)

const (
	UITypeStateSnapshot = "tui.state.snapshot"
	UITypeEventAppend   = "tui.event.append"
	UITypeActionRequest = "tui.action.request"
)
