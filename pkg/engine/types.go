package engine

import "github.com/go-go-golems/devctl/pkg/protocol"

type ServiceSpec struct {
	Name    string            `json:"name"`
	Cwd     string            `json:"cwd,omitempty"`
	Command []string          `json:"command"`
	Env     map[string]string `json:"env,omitempty"`
	Health  *HealthCheck      `json:"health,omitempty"`
}

type HealthCheck struct {
	Type      string `json:"type"` // "tcp"|"http"
	Address   string `json:"address,omitempty"`
	URL       string `json:"url,omitempty"`
	TimeoutMs int64  `json:"timeout_ms,omitempty"`
}

type LaunchPlan struct {
	Services []ServiceSpec `json:"services"`
}

type ValidateResult struct {
	Valid    bool             `json:"valid"`
	Errors   []protocol.Error `json:"errors,omitempty"`
	Warnings []protocol.Error `json:"warnings,omitempty"`
}

type StepResult struct {
	Name       string `json:"name"`
	Ok         bool   `json:"ok"`
	DurationMs int64  `json:"duration_ms,omitempty"`
}

type BuildResult struct {
	Steps     []StepResult      `json:"steps,omitempty"`
	Artifacts map[string]string `json:"artifacts,omitempty"`
}

type PrepareResult struct {
	Steps     []StepResult      `json:"steps,omitempty"`
	Artifacts map[string]string `json:"artifacts,omitempty"`
}
