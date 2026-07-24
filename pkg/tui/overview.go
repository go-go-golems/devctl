package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/go-go-golems/devctl/pkg/operator"
)

type OverviewModel struct {
	Snapshot operator.Snapshot
	Selected int
}

func (m *OverviewModel) SetSnapshot(snapshot operator.Snapshot) {
	m.Snapshot = snapshot
	if len(snapshot.Services) == 0 {
		m.Selected = 0
	} else if m.Selected >= len(snapshot.Services) {
		m.Selected = len(snapshot.Services) - 1
	}
}

func (m *OverviewModel) Move(delta int) {
	if len(m.Snapshot.Services) == 0 {
		return
	}
	m.Selected = (m.Selected + delta + len(m.Snapshot.Services)) % len(m.Snapshot.Services)
}

func (m OverviewModel) SelectedServices() []string {
	if len(m.Snapshot.Services) == 0 {
		return nil
	}
	return []string{m.Snapshot.Services[m.Selected].Service}
}

func (m OverviewModel) View(width int) string {
	if !m.Snapshot.Exists && len(m.Snapshot.Services) == 0 {
		return "No environment state.\n\n[u] start configured services"
	}
	var output strings.Builder
	output.WriteString("Services\n")
	output.WriteString("  NAME             DESIRED   STATE      HEALTH    PID\n")
	for index, service := range m.Snapshot.Services {
		cursor := " "
		if index == m.Selected {
			cursor = ">"
		}
		health := "-"
		if service.Health != nil {
			if service.Health.Healthy {
				health = "healthy"
			} else {
				health = "unhealthy"
			}
		}
		pid := "-"
		if service.Child != nil {
			pid = fmt.Sprintf("%d", service.Child.PID)
		}
		name := truncate(service.Service, max(8, min(16, width/4)))
		_, _ = fmt.Fprintf(
			&output, "%s %-16s %-9s %-10s %-9s %s\n",
			cursor, name, service.Desired, service.Phase, health, pid,
		)
	}
	if len(m.Snapshot.Services) > 0 {
		selected := m.Snapshot.Services[m.Selected]
		_, _ = fmt.Fprintf(&output, "\nSelected: %s\nRun: %s", selected.Service, emptyDash(selected.RunID))
		if selected.Exit != nil && selected.Exit.ExitCode != nil {
			_, _ = fmt.Fprintf(&output, "  exit=%d", *selected.Exit.ExitCode)
		}
		if selected.LastError != nil {
			_, _ = fmt.Fprintf(&output, "\nLast error: %s: %s", selected.LastError.Code, selected.LastError.Message)
		}
		if !selected.UpdatedAt.IsZero() {
			_, _ = fmt.Fprintf(&output, "\nUpdated: %s ago", shortAge(time.Since(selected.UpdatedAt)))
		}
	}
	output.WriteString("\n\n[enter] logs  [u] up  [d] down  [r] restart  [j/k] select")
	return output.String()
}

func truncate(value string, width int) string {
	if width < 2 || len(value) <= width {
		return value
	}
	return value[:width-1] + "…"
}

func emptyDash(value string) string {
	if value == "" {
		return "-"
	}
	return value
}

func shortAge(duration time.Duration) string {
	if duration < 0 {
		duration = 0
	}
	if duration < time.Minute {
		return fmt.Sprintf("%ds", int(duration.Seconds()))
	}
	if duration < time.Hour {
		return fmt.Sprintf("%dm", int(duration.Minutes()))
	}
	return fmt.Sprintf("%dh", int(duration.Hours()))
}
