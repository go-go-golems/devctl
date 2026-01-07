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

	DomainTypePipelineRunStarted     = "pipeline.run.started"
	DomainTypePipelineRunFinished    = "pipeline.run.finished"
	DomainTypePipelinePhaseStarted   = "pipeline.phase.started"
	DomainTypePipelinePhaseFinished  = "pipeline.phase.finished"
	DomainTypePipelineBuildResult    = "pipeline.build.result"
	DomainTypePipelinePrepareResult  = "pipeline.prepare.result"
	DomainTypePipelineValidateResult = "pipeline.validate.result"
	DomainTypePipelineLaunchPlan     = "pipeline.launch.plan"
)

const (
	UITypeStateSnapshot = "tui.state.snapshot"
	UITypeEventAppend   = "tui.event.append"
	UITypeActionRequest = "tui.action.request"

	UITypePipelineRunStarted     = "tui.pipeline.run.started"
	UITypePipelineRunFinished    = "tui.pipeline.run.finished"
	UITypePipelinePhaseStarted   = "tui.pipeline.phase.started"
	UITypePipelinePhaseFinished  = "tui.pipeline.phase.finished"
	UITypePipelineBuildResult    = "tui.pipeline.build.result"
	UITypePipelinePrepareResult  = "tui.pipeline.prepare.result"
	UITypePipelineValidateResult = "tui.pipeline.validate.result"
	UITypePipelineLaunchPlan     = "tui.pipeline.launch.plan"
)
