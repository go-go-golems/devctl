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

type NavigateBackMsg struct{}

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

// PipelineLiveOutputMsg carries a line of live output from a build step.
type PipelineLiveOutputMsg struct {
	Output PipelineLiveOutput
}

// PipelineConfigPatchesMsg carries config patches applied during a pipeline run.
type PipelineConfigPatchesMsg struct {
	Patches PipelineConfigPatches
}

// PipelineStepProgressMsg carries progress updates for a running step.
type PipelineStepProgressMsg struct {
	RunID   string `json:"run_id"`
	Step    string `json:"step"`
	Percent int    `json:"percent"` // 0-100
}
