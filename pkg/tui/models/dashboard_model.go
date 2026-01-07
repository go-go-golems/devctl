package models

import (
	"fmt"
	"os"
	"syscall"
	"time"

	"github.com/charmbracelet/lipgloss"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/go-go-golems/devctl/pkg/state"
	"github.com/go-go-golems/devctl/pkg/tui"
	"github.com/go-go-golems/devctl/pkg/tui/styles"
	"github.com/go-go-golems/devctl/pkg/tui/widgets"
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

	exitSummary map[string]string
}

func NewDashboardModel() DashboardModel { return DashboardModel{} }

func (m DashboardModel) WithSize(width, height int) DashboardModel {
	m.width, m.height = width, height
	return m
}

func (m DashboardModel) WithSnapshot(s tui.StateSnapshot) DashboardModel {
	m.last = &s
	m.selected = clampInt(m.selected, 0, maxInt(0, len(m.serviceNames())-1))
	m.exitSummary = map[string]string{}
	if s.State != nil {
		for _, svc := range s.State.Services {
			alive := false
			if s.Alive != nil {
				alive = s.Alive[svc.Name]
			}
			if alive {
				continue
			}
			if svc.ExitInfo == "" {
				continue
			}
			if _, err := os.Stat(svc.ExitInfo); err != nil {
				continue
			}
			ei, err := state.ReadExitInfo(svc.ExitInfo)
			if err != nil {
				continue
			}
			if ei.Signal != "" {
				m.exitSummary[svc.Name] = "sig=" + ei.Signal
				continue
			}
			if ei.ExitCode != nil {
				m.exitSummary[svc.Name] = fmt.Sprintf("exit=%d", *ei.ExitCode)
				continue
			}
		}
	}
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
			if m.last != nil && m.last.Exists && m.last.State != nil {
				m.confirmAction = true
				m.confirmReq = tui.ActionRequest{Kind: tui.ActionRestart}
				m.confirmText = "State exists; run restart (down then up)"
				return m, nil
			}
			return m, func() tea.Msg { return tui.ActionRequestMsg{Request: tui.ActionRequest{Kind: tui.ActionUp}} }
		}
	}
	return m, nil
}

func (m DashboardModel) View() string {
	theme := styles.DefaultTheme()

	if m.last == nil {
		return theme.TitleMuted.Render("Loading state...")
	}

	s := m.last
	if !s.Exists {
		return m.renderStopped(theme)
	}
	if s.Error != "" {
		return m.renderError(theme, s.Error)
	}
	if s.State == nil {
		return theme.TitleMuted.Render("System: Unknown (state missing)")
	}

	// Build services table
	services := s.State.Services
	rows := make([]widgets.TableRow, len(services))
	for i, svc := range services {
		alive := false
		if s.Alive != nil {
			alive = s.Alive[svc.Name]
		}

		icon := styles.StatusIcon(alive)
		status := "Running"
		if !alive {
			status = "Dead"
			if extra, ok := m.exitSummary[svc.Name]; ok && extra != "" {
				status = fmt.Sprintf("Dead (%s)", extra)
			}
		}

		pidText := fmt.Sprintf("PID %d", svc.PID)

		rows[i] = widgets.TableRow{
			Icon:     icon,
			Cells:    []string{svc.Name, status, pidText},
			Selected: i == m.selected,
		}
	}

	serviceColumns := []widgets.TableColumn{
		{Header: "Name", Width: 18},
		{Header: "Status", Width: 16},
		{Header: "PID", Width: 10},
	}

	table := widgets.NewTable(serviceColumns).
		WithRows(rows).
		WithCursor(m.selected).
		WithSize(m.width-4, 0)

	// Calculate box height based on service count
	tableHeight := len(services) + 2 // rows + header + padding
	if tableHeight < 5 {
		tableHeight = 5
	}

	servicesBox := widgets.NewBox(fmt.Sprintf("Services (%d)", len(services))).
		WithTitleRight("[l] logs  [r] restart  [x] kill").
		WithContent(table.Render()).
		WithSize(m.width, tableHeight)

	// Build main layout
	var sections []string

	// Repo info
	repoStyle := theme.TitleMuted
	repoInfo := repoStyle.Render(fmt.Sprintf("RepoRoot: %s", s.RepoRoot))
	startedInfo := repoStyle.Render(fmt.Sprintf("Started: %s", s.State.CreatedAt.Format("2006-01-02 15:04:05")))
	sections = append(sections, lipgloss.JoinVertical(lipgloss.Left, repoInfo, startedInfo, ""))

	// Services box
	sections = append(sections, servicesBox.Render())

	// Confirmation dialogs
	if m.confirmKill {
		confirmStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(theme.Warning).
			Padding(0, 1)
		confirmBox := confirmStyle.Render(fmt.Sprintf("%s Kill %s pid=%d? %s",
			styles.IconWarning,
			m.confirmName,
			m.confirmPID,
			theme.KeybindKey.Render("[y/n]")))
		sections = append(sections, "", confirmBox)
	}
	if m.confirmAction {
		confirmStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(theme.Warning).
			Padding(0, 1)
		confirmBox := confirmStyle.Render(fmt.Sprintf("%s %s? %s",
			styles.IconWarning,
			m.confirmText,
			theme.KeybindKey.Render("[y/n]")))
		sections = append(sections, "", confirmBox)
	}

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func (m DashboardModel) renderStopped(theme styles.Theme) string {
	icon := theme.StatusPending.Render(styles.IconSystem)
	status := theme.Title.Render("System: Stopped")
	hint := theme.TitleMuted.Render("Press [u] to start")

	box := widgets.NewBox("Dashboard").
		WithContent(lipgloss.JoinVertical(lipgloss.Left,
			lipgloss.JoinHorizontal(lipgloss.Center, icon, " ", status),
			"",
			hint,
		)).
		WithSize(m.width, 6)

	return box.Render()
}

func (m DashboardModel) renderError(theme styles.Theme, errText string) string {
	icon := theme.StatusDead.Render(styles.IconError)
	status := theme.Title.Render("System: Error")
	errStyle := theme.StatusDead
	errMsg := errStyle.Render(errText)

	box := widgets.NewBox("Dashboard").
		WithContent(lipgloss.JoinVertical(lipgloss.Left,
			lipgloss.JoinHorizontal(lipgloss.Center, icon, " ", status),
			"",
			errMsg,
		)).
		WithSize(m.width, 8)

	return box.Render()
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
