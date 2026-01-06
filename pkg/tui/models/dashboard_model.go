package models

import (
	"fmt"
	"strings"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/go-go-golems/devctl/pkg/state"
	"github.com/go-go-golems/devctl/pkg/tui"
)

type DashboardModel struct {
	last *tui.StateSnapshot

	selected int

	width  int
	height int

	confirmKill bool
	confirmName string
	confirmPID  int

	confirmAction bool
	confirmReq    tui.ActionRequest
	confirmText   string
}

func NewDashboardModel() DashboardModel { return DashboardModel{} }

func (m DashboardModel) WithSize(width, height int) DashboardModel {
	m.width, m.height = width, height
	return m
}

func (m DashboardModel) WithSnapshot(s tui.StateSnapshot) DashboardModel {
	m.last = &s
	m.selected = clampInt(m.selected, 0, maxInt(0, len(m.serviceNames())-1))
	return m
}

func (m DashboardModel) Update(msg tea.Msg) (DashboardModel, tea.Cmd) {
	switch v := msg.(type) {
	case tea.KeyMsg:
		if m.confirmAction {
			switch v.String() {
			case "y":
				req := m.confirmReq
				m.confirmAction = false
				m.confirmReq = tui.ActionRequest{}
				m.confirmText = ""
				return m, func() tea.Msg { return tui.ActionRequestMsg{Request: req} }
			case "n", "esc":
				m.confirmAction = false
				m.confirmReq = tui.ActionRequest{}
				m.confirmText = ""
				return m, nil
			default:
				return m, nil
			}
		}

		if m.confirmKill {
			switch v.String() {
			case "y":
				pid := m.confirmPID
				name := m.confirmName
				m.confirmKill = false
				m.confirmName = ""
				m.confirmPID = 0
				return m, func() tea.Msg {
					err := syscall.Kill(pid, syscall.SIGTERM)
					text := fmt.Sprintf("sent SIGTERM to %s pid=%d", name, pid)
					if err != nil {
						text = fmt.Sprintf("failed SIGTERM %s pid=%d: %v", name, pid, err)
					}
					return tui.EventLogAppendMsg{Entry: tui.EventLogEntry{At: time.Now(), Text: text}}
				}
			case "n", "esc":
				m.confirmKill = false
				m.confirmName = ""
				m.confirmPID = 0
				return m, nil
			default:
				return m, nil
			}
		}
		switch v.String() {
		case "up", "k":
			m.selected = clampInt(m.selected-1, 0, maxInt(0, len(m.serviceNames())-1))
			return m, nil
		case "down", "j":
			m.selected = clampInt(m.selected+1, 0, maxInt(0, len(m.serviceNames())-1))
			return m, nil
		case "enter", "l":
			name := m.selectedServiceName()
			if name == "" {
				return m, nil
			}
			return m, func() tea.Msg { return tui.NavigateToServiceMsg{Name: name} }
		case "x":
			svc := m.selectedService()
			if svc == nil || svc.PID <= 0 {
				return m, func() tea.Msg {
					return tui.EventLogAppendMsg{Entry: tui.EventLogEntry{At: time.Now(), Text: "kill: no selected pid"}}
				}
			}
			m.confirmKill = true
			m.confirmName = svc.Name
			m.confirmPID = svc.PID
			return m, nil
		case "d":
			m.confirmAction = true
			m.confirmReq = tui.ActionRequest{Kind: tui.ActionDown}
			m.confirmText = "Stop supervised services and remove state"
			return m, nil
		case "r":
			m.confirmAction = true
			m.confirmReq = tui.ActionRequest{Kind: tui.ActionRestart}
			m.confirmText = "Restart environment (down then up)"
			return m, nil
		case "u":
			return m, func() tea.Msg { return tui.ActionRequestMsg{Request: tui.ActionRequest{Kind: tui.ActionUp}} }
		}
	}
	return m, nil
}

func (m DashboardModel) View() string {
	if m.last == nil {
		return "Loading state...\n"
	}

	s := m.last
	if !s.Exists {
		return "System: Stopped (no state)\n"
	}
	if s.Error != "" {
		return fmt.Sprintf("System: Error (state)\n\n%s\n", s.Error)
	}
	if s.State == nil {
		return "System: Unknown (state missing)\n"
	}

	var b strings.Builder
	b.WriteString("System: Running\n\n")
	b.WriteString(fmt.Sprintf("RepoRoot: %s\n", s.RepoRoot))
	b.WriteString(fmt.Sprintf("Started: %s\n\n", s.State.CreatedAt.Format("2006-01-02 15:04:05")))

	services := s.State.Services
	b.WriteString(fmt.Sprintf("Services (%d):  (↑/↓ select, enter logs, u up, d down, r restart, x kill)\n", len(services)))
	for i, svc := range services {
		a := false
		if s.Alive != nil {
			a = s.Alive[svc.Name]
		}
		status := "dead"
		if a {
			status = "alive"
		}
		cursor := " "
		if i == m.selected {
			cursor = ">"
		}
		b.WriteString(fmt.Sprintf("%s %-20s pid=%-6d %s\n", cursor, svc.Name, svc.PID, status))
	}

	if m.confirmKill {
		b.WriteString("\n")
		b.WriteString(fmt.Sprintf("Kill %s pid=%d? (y/n)\n", m.confirmName, m.confirmPID))
	}
	if m.confirmAction {
		b.WriteString("\n")
		b.WriteString(fmt.Sprintf("%s? (y/n)\n", m.confirmText))
	}

	return b.String()
}

func (m DashboardModel) selectedServiceName() string {
	names := m.serviceNames()
	if len(names) == 0 {
		return ""
	}
	if m.selected < 0 || m.selected >= len(names) {
		return ""
	}
	return names[m.selected]
}

func (m DashboardModel) serviceNames() []string {
	if m.last == nil || m.last.State == nil {
		return nil
	}
	names := make([]string, 0, len(m.last.State.Services))
	for _, svc := range m.last.State.Services {
		names = append(names, svc.Name)
	}
	return names
}

func (m DashboardModel) selectedService() *state.ServiceRecord {
	if m.last == nil || m.last.State == nil {
		return nil
	}
	if m.selected < 0 || m.selected >= len(m.last.State.Services) {
		return nil
	}
	return &m.last.State.Services[m.selected]
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
