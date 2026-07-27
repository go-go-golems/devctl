package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/go-go-golems/devctl/pkg/operator"
	"github.com/go-go-golems/devctl/pkg/runstate"
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
	return m.ViewAt(width, time.Now())
}

func (m OverviewModel) ViewAt(width int, now time.Time) string {
	if !m.Snapshot.Exists && len(m.Snapshot.Services) == 0 {
		return panel("Environment", "No environment state.\n\n"+renderKey("u", "start configured services"), width)
	}
	var services strings.Builder
	if layoutFor(width) == layoutCompact {
		services.WriteString("  NAME          STATE       HEALTH\n")
	} else {
		services.WriteString("  NAME             DESIRED   STATE      HEALTH      PID\n")
	}
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
		var row string
		if layoutFor(width) == layoutCompact {
			row = fmt.Sprintf("%s %-13s %-11s %s", cursor, name, service.Phase, health)
		} else {
			row = fmt.Sprintf("%s %-16s %-9s %-10s %-11s %s", cursor, name, service.Desired, service.Phase, health, pid)
		}
		if index == m.Selected {
			row = selectedStyle.Render(row)
		} else {
			row = stateStyle(string(service.Phase)).Render(row)
		}
		services.WriteString(row + "\n")
	}
	details := "No service selected."
	if len(m.Snapshot.Services) > 0 {
		selected := m.Snapshot.Services[m.Selected]
		health := "-"
		if selected.Health != nil {
			if selected.Health.Healthy {
				health = "healthy"
			} else {
				health = "unhealthy"
			}
		}
		pid := processIdentityPID(selected.Child)
		var detail strings.Builder
		_, _ = fmt.Fprintf(&detail, "%s\n", titleStyle.Render(selected.Service))
		_, _ = fmt.Fprintf(&detail, "Desired  %-10s  State   %s\n", selected.Desired, stateStyle(string(selected.Phase)).Render(string(selected.Phase)))
		_, _ = fmt.Fprintf(&detail, "Health   %-10s  Uptime  %s\n", health, serviceUptime(now, selected))
		_, _ = fmt.Fprintf(&detail, "Child PID %-9s Wrapper %s\n", pid, processIdentityPID(selected.Wrapper))
		_, _ = fmt.Fprintf(&detail, "Run      %s\n", emptyDash(selected.RunID))
		if selected.Health != nil && selected.Health.Detail != "" {
			_, _ = fmt.Fprintf(&detail, "Check    %s\n", selected.Health.Detail)
		}
		exit := "-"
		if selected.Exit != nil {
			exit = exitDescription(selected.Exit.ExitCode, selected.Exit.Signal)
		}
		_, _ = fmt.Fprintf(&detail, "Exit     %s\n", exit)
		if selected.LastError != nil {
			_, _ = fmt.Fprintf(&detail, "%s\n", errorStyle.Render("Error    "+selected.LastError.Code+": "+selected.LastError.Message))
		}
		_, _ = fmt.Fprintf(&detail, "stdout   %s\n", emptyDash(selected.StdoutPath))
		_, _ = fmt.Fprintf(&detail, "stderr   %s", emptyDash(selected.StderrPath))
		if !selected.UpdatedAt.IsZero() {
			_, _ = fmt.Fprintf(&detail, "\nUpdated  %s ago", shortAge(now.Sub(selected.UpdatedAt)))
		}
		details = detail.String()
	}
	keys := renderKey("enter", "logs") + "  " + renderKey("u", "up") + "  " +
		renderKey("d", "down") + "  " + renderKey("r", "restart") + "  " + renderKey("j/k", "select")
	if layoutFor(width) == layoutCompact {
		selected := m.Snapshot.Services[m.Selected]
		summary := fmt.Sprintf(
			"Selected %s %s pid:%s h:%s",
			selected.Service, selected.Phase, processIdentityPID(selected.Child),
			func() string {
				if selected.Health == nil {
					return "-"
				}
				if selected.Health.Healthy {
					return "healthy"
				}
				return "unhealthy"
			}(),
		)
		return panel("Overview", strings.TrimSuffix(services.String(), "\n")+"\n\n"+summary, width) +
			"\n" + renderKey("enter", "logs") + "  " + renderKey("u/d/r", "lifecycle") + "  " + renderKey("j/k", "select")
	}
	if layoutFor(width) == layoutWide {
		return joinPanels(strings.TrimSuffix(services.String(), "\n"), details, width) + "\n" + keys
	}
	return panel("Services", strings.TrimSuffix(services.String(), "\n"), width) + "\n" +
		panel("Current service", details, width) + "\n" + keys
}

func serviceUptime(now time.Time, service operator.ServiceSnapshot) string {
	if service.CreatedAt.IsZero() || service.Phase == "exited" || service.Phase == "failed" {
		return "-"
	}
	return shortAge(now.Sub(service.CreatedAt))
}

func processIdentityPID(identity *runstate.ProcessIdentity) string {
	if identity == nil {
		return "-"
	}
	return fmt.Sprint(identity.PID)
}

func exitDescription(code *int, signal string) string {
	parts := make([]string, 0, 2)
	if code != nil {
		parts = append(parts, fmt.Sprintf("code %d", *code))
	}
	if signal != "" {
		parts = append(parts, "signal "+signal)
	}
	if len(parts) == 0 {
		return "-"
	}
	return strings.Join(parts, ", ")
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
