package tui

import (
	"time"

	"github.com/go-go-golems/devctl/pkg/protocol"
)

type PipelinePhase string

const (
	PipelinePhaseMutateConfig PipelinePhase = "mutate_config"
	PipelinePhaseBuild        PipelinePhase = "build"
	PipelinePhasePrepare      PipelinePhase = "prepare"
	PipelinePhaseValidate     PipelinePhase = "validate"
	PipelinePhaseLaunchPlan   PipelinePhase = "launch_plan"
	PipelinePhaseSupervise    PipelinePhase = "supervise"
	PipelinePhaseStateSave    PipelinePhase = "state_save"

	PipelinePhaseStopSupervise PipelinePhase = "stop_supervise"
	PipelinePhaseRemoveState   PipelinePhase = "remove_state"
)

type PipelineRunStarted struct {
	RunID    string          `json:"run_id"`
	Kind     ActionKind      `json:"kind"`
	RepoRoot string          `json:"repo_root"`
	At       time.Time       `json:"at"`
	Phases   []PipelinePhase `json:"phases,omitempty"`
}

type PipelineRunFinished struct {
	RunID      string     `json:"run_id"`
	Kind       ActionKind `json:"kind"`
	RepoRoot   string     `json:"repo_root"`
	At         time.Time  `json:"at"`
	Ok         bool       `json:"ok"`
	DurationMs int64      `json:"duration_ms,omitempty"`
	Error      string     `json:"error,omitempty"`
}

type PipelinePhaseStarted struct {
	RunID string        `json:"run_id"`
	Phase PipelinePhase `json:"phase"`
	At    time.Time     `json:"at"`
}

type PipelinePhaseFinished struct {
	RunID      string        `json:"run_id"`
	Phase      PipelinePhase `json:"phase"`
	At         time.Time     `json:"at"`
	Ok         bool          `json:"ok"`
	DurationMs int64         `json:"duration_ms,omitempty"`
	Error      string        `json:"error,omitempty"`
}

type PipelineStepResult struct {
	Name       string `json:"name"`
	Ok         bool   `json:"ok"`
	DurationMs int64  `json:"duration_ms,omitempty"`
}

type PipelineBuildResult struct {
	RunID string               `json:"run_id"`
	At    time.Time            `json:"at"`
	Steps []PipelineStepResult `json:"steps,omitempty"`
}

type PipelinePrepareResult struct {
	RunID string               `json:"run_id"`
	At    time.Time            `json:"at"`
	Steps []PipelineStepResult `json:"steps,omitempty"`
}

type PipelineValidateResult struct {
	RunID    string           `json:"run_id"`
	At       time.Time        `json:"at"`
	Valid    bool             `json:"valid"`
	Errors   []protocol.Error `json:"errors,omitempty"`
	Warnings []protocol.Error `json:"warnings,omitempty"`
}

type PipelineLaunchPlan struct {
	RunID    string    `json:"run_id"`
	At       time.Time `json:"at"`
	Services []string  `json:"services,omitempty"`
}
