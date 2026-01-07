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

	// Recent events for preview
	recentEvents []tui.EventLogEntry

	// Pipeline status
	pipelineRunning bool
	pipelineKind    tui.ActionKind
	pipelinePhase   tui.PipelinePhase
	pipelineStarted time.Time
	pipelineOk      *bool // nil = running, true = ok, false = failed
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
					level := tui.LogLevelInfo
					if err != nil {
						text = fmt.Sprintf("failed SIGTERM %s pid=%d: %v", name, pid, err)
						level = tui.LogLevelError
					}
					return tui.EventLogAppendMsg{Entry: tui.EventLogEntry{At: time.Now(), Source: name, Level: level, Text: text}}
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
					return tui.EventLogAppendMsg{Entry: tui.EventLogEntry{At: time.Now(), Source: "ui", Level: tui.LogLevelWarn, Text: "kill: no selected pid"}}
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

	// Build services table with Health, CPU, MEM columns
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

		// Health status
		healthIcon := styles.IconUnknown
		if s.Health != nil {
			if h, ok := s.Health[svc.Name]; ok {
				healthIcon = styles.HealthIcon(string(h.Status))
			}
		}

		// CPU and Memory stats
		cpuText := "-"
		memText := "-"
		if s.ProcessStats != nil {
			if stats, ok := s.ProcessStats[svc.PID]; ok {
				cpuText = formatCPU(stats.CPUPercent)
				memText = formatMem(stats.MemoryMB)
			}
		}

		pidText := fmt.Sprintf("%d", svc.PID)

		rows[i] = widgets.TableRow{
			Icon:     icon,
			Cells:    []string{svc.Name, status, healthIcon, pidText, cpuText, memText},
			Selected: i == m.selected,
		}
	}

	serviceColumns := []widgets.TableColumn{
		{Header: "Name", Width: 14},
		{Header: "Status", Width: 16},
		{Header: "Health", Width: 8},
		{Header: "PID", Width: 8},
		{Header: "CPU", Width: 7},
		{Header: "MEM", Width: 7},
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

	// Pipeline status (if running)
	if m.pipelineRunning {
		sections = append(sections, m.renderPipelineStatus(theme))
		sections = append(sections, "")
	}

	// Services box
	sections = append(sections, servicesBox.Render())

	// Recent events preview
	if len(m.recentEvents) > 0 {
		sections = append(sections, "")
		sections = append(sections, m.renderEventsPreview(theme))
	}

	// Plugins summary
	if s.Plugins != nil && len(s.Plugins) > 0 {
		sections = append(sections, "")
		sections = append(sections, m.renderPluginsSummary(theme, s.Plugins))
	}

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

func (m DashboardModel) renderPipelineStatus(theme styles.Theme) string {
	// Determine status icon and style
	var statusIcon string
	var statusStyle lipgloss.Style
	var statusText string

	if m.pipelineOk == nil {
		// Still running
		statusIcon = styles.IconRunning
		statusStyle = theme.StatusRunning
		statusText = "Running"
	} else if *m.pipelineOk {
		statusIcon = styles.IconSuccess
		statusStyle = theme.StatusRunning
		statusText = "Complete"
	} else {
		statusIcon = styles.IconError
		statusStyle = theme.StatusDead
		statusText = "Failed"
	}

	// Build content
	var lines []string

	// Status line
	statusLine := lipgloss.JoinHorizontal(lipgloss.Center,
		statusStyle.Render(statusIcon),
		" ",
		theme.Title.Render(fmt.Sprintf("Pipeline: %s", m.pipelineKind)),
		"  ",
		statusStyle.Render(statusText),
	)
	lines = append(lines, statusLine)

	// Phase line (if running)
	if m.pipelinePhase != "" && m.pipelineOk == nil {
		elapsed := time.Since(m.pipelineStarted)
		phaseLine := lipgloss.JoinHorizontal(lipgloss.Center,
			theme.TitleMuted.Render("Phase: "),
			theme.KeybindKey.Render(string(m.pipelinePhase)),
			theme.TitleMuted.Render(fmt.Sprintf("  (%.1fs)", elapsed.Seconds())),
		)
		lines = append(lines, phaseLine)
	}

	box := widgets.NewBox("Pipeline").
		WithTitleRight("[tab] to pipeline view").
		WithContent(lipgloss.JoinVertical(lipgloss.Left, lines...)).
		WithSize(m.width, len(lines)+3)

	return box.Render()
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

func (m DashboardModel) renderEventsPreview(theme styles.Theme) string {
	var lines []string
	for _, e := range m.recentEvents {
		ts := e.At.Format("15:04:05")
		icon := styles.LogLevelIcon(string(e.Level))

		// Style based on level
		var style lipgloss.Style
		switch e.Level {
		case tui.LogLevelError:
			style = theme.StatusDead
		case tui.LogLevelWarn:
			style = lipgloss.NewStyle().Foreground(theme.Warning)
		default:
			style = theme.TitleMuted
		}

		source := e.Source
		if source == "" {
			source = "system"
		}
		if len(source) > 10 {
			source = source[:10]
		}

		// Truncate text if too long
		text := e.Text
		maxTextLen := m.width - 30
		if maxTextLen > 10 && len(text) > maxTextLen {
			text = text[:maxTextLen-3] + "..."
		}

		line := fmt.Sprintf(" %s  %-10s  %s  %s",
			theme.TitleMuted.Render(ts),
			theme.KeybindKey.Render("["+source+"]"),
			style.Render(icon),
			style.Render(text),
		)
		lines = append(lines, line)
	}

	content := lipgloss.JoinVertical(lipgloss.Left, lines...)
	box := widgets.NewBox(fmt.Sprintf("Recent Events (%d)", len(m.recentEvents))).
		WithTitleRight("[e] all events").
		WithContent(content).
		WithSize(m.width, len(lines)+2)

	return box.Render()
}

func (m DashboardModel) renderPluginsSummary(theme styles.Theme, plugins []tui.PluginSummary) string {
	var lines []string
	for _, p := range plugins {
		// Status icon
		var icon string
		var style lipgloss.Style
		switch p.Status {
		case "active":
			icon = styles.IconSuccess
			style = theme.StatusRunning
		case "error":
			icon = styles.IconError
			style = theme.StatusDead
		default:
			icon = styles.IconPending
			style = theme.TitleMuted
		}

		priority := fmt.Sprintf("(priority: %d)", p.Priority)
		line := fmt.Sprintf(" %s %-20s  %s",
			style.Render(icon),
			theme.Title.Render(p.ID),
			theme.TitleMuted.Render(priority),
		)
		lines = append(lines, line)
	}

	content := lipgloss.JoinVertical(lipgloss.Left, lines...)

	// Count active plugins
	activeCount := 0
	for _, p := range plugins {
		if p.Status == "active" {
			activeCount++
		}
	}

	box := widgets.NewBox(fmt.Sprintf("Plugins (%d active)", activeCount)).
		WithTitleRight("[p] details").
		WithContent(content).
		WithSize(m.width, len(lines)+2)

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

// AppendEvent adds an event to the recent events list (max 5).
func (m DashboardModel) AppendEvent(e tui.EventLogEntry) DashboardModel {
	m.recentEvents = append(m.recentEvents, e)
	if len(m.recentEvents) > 5 {
		m.recentEvents = m.recentEvents[len(m.recentEvents)-5:]
	}
	return m
}

// WithPipelineStarted updates the model when a pipeline starts.
func (m DashboardModel) WithPipelineStarted(run tui.PipelineRunStarted) DashboardModel {
	m.pipelineRunning = true
	m.pipelineKind = run.Kind
	m.pipelinePhase = ""
	m.pipelineStarted = run.At
	m.pipelineOk = nil
	return m
}

// WithPipelinePhase updates the current pipeline phase.
func (m DashboardModel) WithPipelinePhase(phase tui.PipelinePhase) DashboardModel {
	m.pipelinePhase = phase
	return m
}

// WithPipelineFinished updates the model when a pipeline finishes.
func (m DashboardModel) WithPipelineFinished(ok bool) DashboardModel {
	m.pipelineOk = &ok
	// Keep showing for a bit, then clear
	if ok {
		m.pipelineRunning = false
	}
	return m
}

// formatCPU formats a CPU percentage for display.
func formatCPU(pct float64) string {
	if pct < 0 {
		return "-"
	}
	if pct >= 100 {
		return fmt.Sprintf("%.0f%%", pct)
	}
	return fmt.Sprintf("%.1f%%", pct)
}

// formatMem formats memory in MB for display.
func formatMem(mb int64) string {
	if mb < 0 {
		return "-"
	}
	if mb >= 1024 {
		return fmt.Sprintf("%.1fG", float64(mb)/1024)
	}
	return fmt.Sprintf("%dM", mb)
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
