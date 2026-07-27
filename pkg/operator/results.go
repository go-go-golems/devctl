package operator

import (
	"time"

	"github.com/go-go-golems/devctl/pkg/runstate"
)

type ServiceOutcome struct {
	Service string                `json:"service"`
	RunID   string                `json:"run_id,omitempty"`
	Before  runstate.RunPhase     `json:"before,omitempty"`
	After   runstate.RunPhase     `json:"after,omitempty"`
	Changed bool                  `json:"changed"`
	Exit    *runstate.ExitSummary `json:"exit,omitempty"`
	Error   *OperatorError        `json:"error,omitempty"`
}

type OperationResult struct {
	OperationID string           `json:"operation_id"`
	Kind        string           `json:"kind"`
	StartedAt   time.Time        `json:"started_at"`
	FinishedAt  time.Time        `json:"finished_at"`
	Status      string           `json:"status"`
	Outcomes    []ServiceOutcome `json:"outcomes"`
}

type Snapshot struct {
	Exists   bool              `json:"exists"`
	Profile  string            `json:"profile,omitempty"`
	Revision uint64            `json:"revision,omitempty"`
	Services []ServiceSnapshot `json:"services"`
}

type ServiceSnapshot struct {
	Service    string                    `json:"service"`
	Desired    runstate.DesiredState     `json:"desired"`
	Phase      runstate.RunPhase         `json:"phase,omitempty"`
	RunID      string                    `json:"run_id,omitempty"`
	Wrapper    *runstate.ProcessIdentity `json:"wrapper,omitempty"`
	Child      *runstate.ProcessIdentity `json:"child,omitempty"`
	Health     *runstate.HealthResult    `json:"health,omitempty"`
	CreatedAt  time.Time                 `json:"created_at,omitempty"`
	UpdatedAt  time.Time                 `json:"updated_at,omitempty"`
	Exit       *runstate.ExitSummary     `json:"exit,omitempty"`
	LastError  *runstate.ErrorRecord     `json:"last_error,omitempty"`
	StdoutPath string                    `json:"stdout_path,omitempty"`
	StderrPath string                    `json:"stderr_path,omitempty"`
}

type DoctorCheck struct {
	Check       string `json:"check"`
	Scope       string `json:"scope"`
	Status      string `json:"status"`
	Code        string `json:"code,omitempty"`
	Summary     string `json:"summary"`
	Path        string `json:"path,omitempty"`
	Service     string `json:"service,omitempty"`
	RunID       string `json:"run_id,omitempty"`
	Remediation string `json:"remediation,omitempty"`
}

type DoctorReport struct {
	Checks         []DoctorCheck        `json:"checks"`
	Reconciliation ReconciliationReport `json:"reconciliation"`
}

type ReconciliationAction struct {
	Service string            `json:"service"`
	RunID   string            `json:"run_id"`
	Before  runstate.RunPhase `json:"before,omitempty"`
	After   runstate.RunPhase `json:"after,omitempty"`
	Action  string            `json:"action"`
	Error   *OperatorError    `json:"error,omitempty"`
}

type ReconciliationReport struct {
	Actions       []ReconciliationAction `json:"actions"`
	UnindexedRuns []string               `json:"unindexed_runs"`
}
