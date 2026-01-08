package tui

import (
	"time"

	"github.com/go-go-golems/devctl/pkg/proc"
	"github.com/go-go-golems/devctl/pkg/state"
)

type ServiceExitObserved struct {
	Name   string    `json:"name"`
	PID    int       `json:"pid"`
	When   time.Time `json:"when"`
	Reason string    `json:"reason,omitempty"`
}

// HealthStatus represents the health check status of a service.
type HealthStatus string

const (
	HealthUnknown   HealthStatus = "unknown"
	HealthHealthy   HealthStatus = "healthy"
	HealthUnhealthy HealthStatus = "unhealthy"
)

// HealthCheckResult contains the result of a service health check.
type HealthCheckResult struct {
	ServiceName string       `json:"service_name"`
	Status      HealthStatus `json:"status"`
	LastCheck   time.Time    `json:"last_check"`
	CheckType   string       `json:"check_type,omitempty"` // "tcp", "http", "exec"
	Endpoint    string       `json:"endpoint,omitempty"`   // e.g., "http://localhost:8080/health"
	Error       string       `json:"error,omitempty"`
	ResponseMs  int64        `json:"response_ms,omitempty"`
}

// PluginSummary contains summary information about a plugin.
type PluginSummary struct {
	ID        string    `json:"id"`
	Path      string    `json:"path"`
	Priority  int       `json:"priority"`
	Status    string    `json:"status"` // "active" | "disabled" | "error"
	Protocol  string    `json:"protocol,omitempty"`
	Ops       []string  `json:"ops,omitempty"`
	Streams   []string  `json:"streams,omitempty"`
	Commands  []string  `json:"commands,omitempty"`
	CapStatus string    `json:"cap_status,omitempty"` // "unknown" | "introspecting" | "ok" | "error"
	CapError  string    `json:"cap_error,omitempty"`
	CapStart  time.Time `json:"cap_start,omitempty"`
	CapEnd    time.Time `json:"cap_end,omitempty"`
}

type StateSnapshot struct {
	RepoRoot     string                        `json:"repo_root"`
	At           time.Time                     `json:"at"`
	Exists       bool                          `json:"exists"`
	State        *state.State                  `json:"state,omitempty"`
	Alive        map[string]bool               `json:"alive,omitempty"`
	Error        string                        `json:"error,omitempty"`
	ProcessStats map[int]*proc.Stats           `json:"process_stats,omitempty"` // PID -> stats
	Health       map[string]*HealthCheckResult `json:"health,omitempty"`        // service name -> health
	Plugins      []PluginSummary               `json:"plugins,omitempty"`       // Plugin summaries
}
