package runstate

import (
	"testing"
	"time"
)

func TestProjectHealth(t *testing.T) {
	checkedAt := time.Date(2026, time.September, 13, 10, 0, 0, 0, time.UTC)
	healthy := &HealthResult{Healthy: true, CheckedAt: checkedAt}
	unhealthy := &HealthResult{Healthy: false, CheckedAt: checkedAt, Detail: "connection refused"}

	tests := []struct {
		name  string
		phase RunPhase
		last  *HealthResult
		want  HealthState
	}{
		{name: "ready healthy", phase: RunReady, last: healthy, want: HealthHealthy},
		{name: "ready unhealthy", phase: RunReady, last: unhealthy, want: HealthUnhealthy},
		{name: "ready unknown", phase: RunReady, want: HealthUnknown},
		{name: "starting observation is not current", phase: RunStarting, last: healthy, want: HealthUnknown},
		{name: "exited formerly healthy", phase: RunExited, last: healthy, want: HealthNotRunning},
		{name: "failed startup", phase: RunFailed, last: unhealthy, want: HealthNotRunning},
		{name: "stopping is indeterminate", phase: RunStopping, last: healthy, want: HealthUnknown},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := ProjectHealth(test.phase, test.last)
			if got.Current != test.want {
				t.Fatalf("ProjectHealth(%q) current = %q, want %q", test.phase, got.Current, test.want)
			}
			if got.Last != test.last {
				t.Fatalf("ProjectHealth(%q) did not retain last observation", test.phase)
			}
		})
	}
}
