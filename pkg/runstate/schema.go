package runstate

import (
	"time"

	"github.com/google/uuid"
	"github.com/pkg/errors"
)

const (
	StateSchemaVersion = 2
	RunSchemaVersion   = 1
)

type DesiredState string

const (
	DesiredRunning DesiredState = "running"
	DesiredStopped DesiredState = "stopped"
)

type RunPhase string

const (
	RunPlanned  RunPhase = "planned"
	RunStarting RunPhase = "starting"
	RunReady    RunPhase = "ready"
	RunStopping RunPhase = "stopping"
	RunExited   RunPhase = "exited"
	RunFailed   RunPhase = "failed"
	RunUnknown  RunPhase = "unknown"
)

type EnvironmentState struct {
	Version   int                    `json:"version"`
	RepoRoot  string                 `json:"repo_root"`
	Profile   string                 `json:"profile,omitempty"`
	Revision  uint64                 `json:"revision"`
	CreatedAt time.Time              `json:"created_at"`
	UpdatedAt time.Time              `json:"updated_at"`
	Services  map[string]ServiceSlot `json:"services"`
}

type ServiceSlot struct {
	Name         string       `json:"name"`
	CurrentRunID string       `json:"current_run_id,omitempty"`
	LastRunID    string       `json:"last_run_id,omitempty"`
	Desired      DesiredState `json:"desired"`
}

type RunRecord struct {
	Version     int               `json:"version"`
	RunID       string            `json:"run_id"`
	Service     string            `json:"service"`
	Phase       RunPhase          `json:"phase"`
	Spec        ServiceSpecRecord `json:"spec"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	Wrapper     *ProcessIdentity  `json:"wrapper,omitempty"`
	Child       *ProcessIdentity  `json:"child,omitempty"`
	ChildPGID   int               `json:"child_pgid,omitempty"`
	Health      *HealthResult     `json:"health,omitempty"`
	Exit        *ExitSummary      `json:"exit,omitempty"`
	ArtifactDir string            `json:"artifact_dir"`
	LastError   *ErrorRecord      `json:"last_error,omitempty"`
}

type ServiceSpecRecord struct {
	Name        string             `json:"name"`
	Command     []string           `json:"command"`
	Cwd         string             `json:"cwd,omitempty"`
	Environment map[string]string  `json:"environment,omitempty"`
	Health      *HealthCheckRecord `json:"health,omitempty"`
}

type HealthCheckRecord struct {
	Type      string `json:"type"`
	Address   string `json:"address,omitempty"`
	URL       string `json:"url,omitempty"`
	TimeoutMs int64  `json:"timeout_ms,omitempty"`
}

type HealthResult struct {
	Healthy    bool      `json:"healthy"`
	CheckedAt  time.Time `json:"checked_at"`
	DurationMs int64     `json:"duration_ms,omitempty"`
	Detail     string    `json:"detail,omitempty"`
}

type ExitSummary struct {
	ExitedAt time.Time `json:"exited_at"`
	ExitCode *int      `json:"exit_code,omitempty"`
	Signal   string    `json:"signal,omitempty"`
	Error    string    `json:"error,omitempty"`
}

type ErrorRecord struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

type ProcessIdentity struct {
	PID        int    `json:"pid"`
	StartToken string `json:"start_token"`
}

func NewRunID() (string, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return "", errors.Wrap(err, "generate UUIDv7 run ID")
	}
	return id.String(), nil
}

func validDesiredState(state DesiredState) bool {
	return state == DesiredRunning || state == DesiredStopped
}

func validRunPhase(phase RunPhase) bool {
	switch phase {
	case RunPlanned, RunStarting, RunReady, RunStopping, RunExited, RunFailed, RunUnknown:
		return true
	default:
		return false
	}
}
