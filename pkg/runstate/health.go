package runstate

// HealthState is the current health presentation of a service attempt.
type HealthState string

const (
	HealthHealthy    HealthState = "healthy"
	HealthUnhealthy  HealthState = "unhealthy"
	HealthUnknown    HealthState = "unknown"
	HealthNotRunning HealthState = "not_running"
)

// HealthView separates current service state from the last timed health observation.
type HealthView struct {
	Current HealthState   `json:"current"`
	Last    *HealthResult `json:"last,omitempty"`
}

// ProjectHealth derives current health without discarding historical observations.
func ProjectHealth(phase RunPhase, last *HealthResult) HealthView {
	view := HealthView{Current: HealthUnknown, Last: last}
	switch phase {
	case RunExited, RunFailed:
		view.Current = HealthNotRunning
	case RunReady:
		if last == nil {
			return view
		}
		if last.Healthy {
			view.Current = HealthHealthy
		} else {
			view.Current = HealthUnhealthy
		}
	}
	return view
}
