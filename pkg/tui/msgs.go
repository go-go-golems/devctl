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

type PipelineRunStartedMsg struct {
	Run PipelineRunStarted
}

type PipelineRunFinishedMsg struct {
	Run PipelineRunFinished
}

type PipelinePhaseStartedMsg struct {
	Event PipelinePhaseStarted
}

type PipelinePhaseFinishedMsg struct {
	Event PipelinePhaseFinished
}

type PipelineBuildResultMsg struct {
	Result PipelineBuildResult
}

type PipelinePrepareResultMsg struct {
	Result PipelinePrepareResult
}

type PipelineValidateResultMsg struct {
	Result PipelineValidateResult
}

type PipelineLaunchPlanMsg struct {
	Plan PipelineLaunchPlan
}
