package tui

import (
	"fmt"
	"strings"

	"github.com/go-go-golems/devctl/pkg/operator"
)

type RunEntry struct {
	Result operator.OperationResult
	Events []operator.OperatorEvent
	Err    error
}

type RunsModel struct {
	Entries  []RunEntry
	Selected int
}

func (m *RunsModel) Add(result operator.OperationResult, events []operator.OperatorEvent, err error) {
	m.Entries = append([]RunEntry{{Result: result, Events: events, Err: err}}, m.Entries...)
	if len(m.Entries) > 100 {
		m.Entries = m.Entries[:100]
	}
	m.Selected = 0
}

func (m *RunsModel) Move(delta int) {
	if len(m.Entries) == 0 {
		return
	}
	m.Selected = (m.Selected + delta + len(m.Entries)) % len(m.Entries)
}

func (m RunsModel) View(height int) string {
	if len(m.Entries) == 0 {
		return "No lifecycle operations in this session."
	}
	var output strings.Builder
	output.WriteString("Operations\n")
	for index, entry := range m.Entries {
		cursor := " "
		if index == m.Selected {
			cursor = ">"
		}
		_, _ = fmt.Fprintf(
			&output, "%s %-9s %-10s %s\n",
			cursor, entry.Result.Kind, entry.Result.Status, emptyDash(entry.Result.OperationID),
		)
		if index >= max(3, height/3) {
			break
		}
	}
	selected := m.Entries[m.Selected]
	output.WriteString("\nPhases and service outcomes\n")
	for _, event := range selected.Events {
		if event.Phase == "" && event.Service == "" {
			continue
		}
		label := event.Phase
		if label == "" {
			label = event.Service
		}
		_, _ = fmt.Fprintf(&output, "  %-18s %-10s %s\n", label, event.Status, event.Message)
	}
	for _, outcome := range selected.Result.Outcomes {
		status := string(outcome.After)
		if outcome.Error != nil {
			status = outcome.Error.Code
		}
		_, _ = fmt.Fprintf(&output, "  %-18s %s\n", outcome.Service, status)
	}
	if selected.Err != nil {
		_, _ = fmt.Fprintf(&output, "\nError: %v", selected.Err)
	}
	output.WriteString("\n\n[j/k] select operation  [l] related logs")
	return output.String()
}
