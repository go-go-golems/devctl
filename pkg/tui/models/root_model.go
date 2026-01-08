package models

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/go-go-golems/devctl/pkg/tui"
	"github.com/go-go-golems/devctl/pkg/tui/styles"
	"github.com/go-go-golems/devctl/pkg/tui/widgets"
)

type ViewID string

const (
	ViewDashboard ViewID = "dashboard"
	ViewService   ViewID = "service"
	ViewEvents    ViewID = "events"
	ViewPipeline  ViewID = "pipeline"
	ViewPlugins   ViewID = "plugins"
)

type RootModel struct {
	width  int
	height int

	active ViewID
	help   bool

	dashboard DashboardModel
	service   ServiceModel
	events    EventLogModel
	pipeline  PipelineModel
	plugins   PluginModel

	publishAction      func(tui.ActionRequest) error
	publishStreamStart func(tui.StreamStartRequest) error
	publishStreamStop  func(tui.StreamStopRequest) error

	statusLine   string
	statusOk     bool
	startedAt    time.Time
	systemStatus string
}

type RootModelOptions struct {
	PublishAction      func(tui.ActionRequest) error
	PublishStreamStart func(tui.StreamStartRequest) error
	PublishStreamStop  func(tui.StreamStopRequest) error
}

func NewRootModel(opts RootModelOptions) RootModel {
	const defaultWidth = 80
	const defaultHeight = 24

	m := RootModel{
		width:              defaultWidth,
		height:             defaultHeight,
		active:             ViewDashboard,
		dashboard:          NewDashboardModel(),
		service:            NewServiceModel(),
		events:             NewEventLogModel(),
		pipeline:           NewPipelineModel(),
		plugins:            NewPluginModel(),
		publishAction:      opts.PublishAction,
		publishStreamStart: opts.PublishStreamStart,
		publishStreamStop:  opts.PublishStreamStop,
	}
	m = m.applyChildSizes()
	return m
}

func (m RootModel) Init() tea.Cmd { return nil }

func (m RootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch v := msg.(type) {
	case tea.WindowSizeMsg:
		w, h := v.Width, v.Height
		if w <= 0 {
			w = 80
		}
		if h <= 0 {
			h = 24
		}

		m.width, m.height = w, h
		m = m.applyChildSizes()
		return m, nil
	case tea.KeyMsg:
		switch v.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "?":
			m.help = !m.help
			return m, nil
		case "tab":
			if m.active == ViewService {
				var cmd tea.Cmd
				m.service, cmd = m.service.Update(v)
				return m, cmd
			}
			switch m.active {
			case ViewDashboard:
				m.active = ViewEvents
			case ViewEvents:
				m.active = ViewPipeline
			case ViewPipeline:
				m.active = ViewPlugins
			default:
				m.active = ViewDashboard
			}
			return m, nil
		}
		switch m.active {
		case ViewDashboard:
			var cmd tea.Cmd
			m.dashboard, cmd = m.dashboard.Update(v)
			return m, cmd
		case ViewService:
			switch v.String() {
			case "esc":
				m.active = ViewDashboard
				return m, nil
			}
			var cmd tea.Cmd
			m.service, cmd = m.service.Update(v)
			return m, cmd
		case ViewEvents:
			var cmd tea.Cmd
			m.events, cmd = m.events.Update(v)
			return m, cmd
		case ViewPipeline:
			var cmd tea.Cmd
			m.pipeline, cmd = m.pipeline.Update(v)
			return m, cmd
		case ViewPlugins:
			var cmd tea.Cmd
			m.plugins, cmd = m.plugins.Update(v)
			return m, cmd
		}
	case tui.StateSnapshotMsg:
		m.dashboard = m.dashboard.WithSnapshot(v.Snapshot)
		m.service = m.service.WithSnapshot(v.Snapshot)
		m.plugins = m.plugins.WithPlugins(v.Snapshot.Plugins)
		// Update system status for header
		if v.Snapshot.Exists && v.Snapshot.State != nil {
			m.startedAt = v.Snapshot.State.CreatedAt
			m.systemStatus = "Running"
		} else if !v.Snapshot.Exists {
			m.systemStatus = "Stopped"
			m.startedAt = time.Time{}
		} else if v.Snapshot.Error != "" {
			m.systemStatus = "Error"
			m.startedAt = time.Time{}
		}
		return m, nil
	case tui.EventLogAppendMsg:
		m.events = m.events.Append(v.Entry)
		m.dashboard = m.dashboard.AppendEvent(v.Entry) // Update dashboard's recent events
		if s := strings.TrimSpace(v.Entry.Text); s != "" {
			if strings.HasPrefix(s, "action failed:") ||
				strings.HasPrefix(s, "action publish failed:") ||
				strings.HasPrefix(s, "failed SIGTERM") {
				m.statusLine = s
				m.statusOk = false
				m = m.applyChildSizes()
			} else if strings.HasPrefix(s, "action ok:") ||
				strings.HasPrefix(s, "sent SIGTERM") {
				m.statusLine = s
				m.statusOk = true
				m = m.applyChildSizes()
			}
		}
		return m, nil
	case tui.PipelineRunStartedMsg:
		m.dashboard = m.dashboard.WithPipelineStarted(v.Run)
		var cmd tea.Cmd
		m.pipeline, cmd = m.pipeline.Update(v)
		return m, cmd
	case tui.PipelineRunFinishedMsg:
		m.dashboard = m.dashboard.WithPipelineFinished(v.Run.Ok)
		var cmd tea.Cmd
		m.pipeline, cmd = m.pipeline.Update(v)
		return m, cmd
	case tui.PipelinePhaseStartedMsg:
		m.dashboard = m.dashboard.WithPipelinePhase(v.Event.Phase)
		var cmd tea.Cmd
		m.pipeline, cmd = m.pipeline.Update(v)
		return m, cmd
	case tui.PipelinePhaseFinishedMsg:
		var cmd tea.Cmd
		m.pipeline, cmd = m.pipeline.Update(v)
		return m, cmd
	case tui.PipelineBuildResultMsg:
		var cmd tea.Cmd
		m.pipeline, cmd = m.pipeline.Update(v)
		return m, cmd
	case tui.PipelinePrepareResultMsg:
		var cmd tea.Cmd
		m.pipeline, cmd = m.pipeline.Update(v)
		return m, cmd
	case tui.PipelineValidateResultMsg:
		var cmd tea.Cmd
		m.pipeline, cmd = m.pipeline.Update(v)
		return m, cmd
	case tui.PipelineLaunchPlanMsg:
		var cmd tea.Cmd
		m.pipeline, cmd = m.pipeline.Update(v)
		return m, cmd
	case tui.NavigateToServiceMsg:
		m.service = m.service.WithService(v.Name)
		m.active = ViewService
		if m.service.follow {
			return m, m.service.tickCmd()
		}
	case tui.NavigateBackMsg:
		// Go back to dashboard from service/plugin view
		m.active = ViewDashboard
		return m, nil
	case tui.ActionRequestMsg:
		if m.publishAction == nil {
			m.events = m.events.Append(tui.EventLogEntry{
				At:     time.Now(),
				Source: "ui",
				Level:  tui.LogLevelWarn,
				Text:   fmt.Sprintf("action ignored: %s (no publisher)", v.Request.Kind),
			})
			return m, nil
		}
		req := v.Request
		return m, func() tea.Msg {
			if err := m.publishAction(req); err != nil {
				return tui.EventLogAppendMsg{Entry: tui.EventLogEntry{
					At:     time.Now(),
					Source: "system",
					Level:  tui.LogLevelError,
					Text:   fmt.Sprintf("action publish failed: %v", err),
				}}
			}
			return tui.EventLogAppendMsg{Entry: tui.EventLogEntry{
				At:     time.Now(),
				Source: "system",
				Level:  tui.LogLevelInfo,
				Text:   fmt.Sprintf("action requested: %s", req.Kind),
			}}
		}
	case tui.StreamStartRequestMsg:
		if m.publishStreamStart == nil {
			m.events = m.events.Append(tui.EventLogEntry{
				At:     time.Now(),
				Source: "ui",
				Level:  tui.LogLevelWarn,
				Text:   fmt.Sprintf("stream start ignored: %s (no publisher)", v.Request.Op),
			})
			return m, nil
		}
		req := v.Request
		return m, func() tea.Msg {
			if err := m.publishStreamStart(req); err != nil {
				return tui.EventLogAppendMsg{Entry: tui.EventLogEntry{
					At:     time.Now(),
					Source: "system",
					Level:  tui.LogLevelError,
					Text:   fmt.Sprintf("stream start publish failed: %v", err),
				}}
			}
			return tui.EventLogAppendMsg{Entry: tui.EventLogEntry{
				At:     time.Now(),
				Source: "system",
				Level:  tui.LogLevelInfo,
				Text:   fmt.Sprintf("stream start requested: %s", req.Op),
			}}
		}
	case tui.StreamStopRequestMsg:
		if m.publishStreamStop == nil {
			m.events = m.events.Append(tui.EventLogEntry{
				At:     time.Now(),
				Source: "ui",
				Level:  tui.LogLevelWarn,
				Text:   fmt.Sprintf("stream stop ignored: %s (no publisher)", v.Request.StreamKey),
			})
			return m, nil
		}
		req := v.Request
		return m, func() tea.Msg {
			if err := m.publishStreamStop(req); err != nil {
				return tui.EventLogAppendMsg{Entry: tui.EventLogEntry{
					At:     time.Now(),
					Source: "system",
					Level:  tui.LogLevelError,
					Text:   fmt.Sprintf("stream stop publish failed: %v", err),
				}}
			}
			return tui.EventLogAppendMsg{Entry: tui.EventLogEntry{
				At:     time.Now(),
				Source: "system",
				Level:  tui.LogLevelInfo,
				Text:   fmt.Sprintf("stream stop requested: %s", req.StreamKey),
			}}
		}
	}

	switch m.active {
	case ViewService:
		var cmd tea.Cmd
		m.service, cmd = m.service.Update(msg)
		return m, cmd
	case ViewEvents:
		var cmd tea.Cmd
		m.events, cmd = m.events.Update(msg)
		return m, cmd
	case ViewPipeline:
		var cmd tea.Cmd
		m.pipeline, cmd = m.pipeline.Update(msg)
		return m, cmd
	case ViewPlugins:
		var cmd tea.Cmd
		m.plugins, cmd = m.plugins.Update(msg)
		return m, cmd
	case ViewDashboard:
		var cmd tea.Cmd
		m.dashboard, cmd = m.dashboard.Update(msg)
		return m, cmd
	default:
		return m, nil
	}
}

func (m RootModel) View() string {
	theme := styles.DefaultTheme()

	// Build header
	statusIcon := styles.IconSystem
	statusOk := true
	switch m.systemStatus {
	case "Running":
		statusIcon = styles.IconSuccess
		statusOk = true
	case "Stopped":
		statusIcon = styles.IconPending
		statusOk = false
	case "Error":
		statusIcon = styles.IconError
		statusOk = false
	}

	var uptime time.Duration
	if !m.startedAt.IsZero() {
		uptime = time.Since(m.startedAt)
	}

	viewLabel := string(m.active)
	header := widgets.NewHeader(fmt.Sprintf("DevCtl — %s", viewLabel)).
		WithStatus(statusIcon, m.systemStatus, statusOk).
		WithUptime(uptime).
		WithKeybinds([]widgets.Keybind{
			{Key: "tab", Label: "switch"},
			{Key: "?", Label: "help"},
			{Key: "q", Label: "quit"},
		}).
		WithWidth(m.width)

	// Build status line if present
	var statusSection string
	if m.statusLine != "" {
		statusStyle := theme.StatusRunning
		icon := styles.IconSuccess
		if !m.statusOk {
			statusStyle = theme.StatusDead
			icon = styles.IconError
		}
		statusSection = lipgloss.JoinHorizontal(lipgloss.Center,
			statusStyle.Render(icon),
			" ",
			theme.TitleMuted.Render(m.statusLine),
		)
	}

	// Build main content
	var content string
	switch m.active {
	case ViewService:
		content = m.service.View()
	case ViewEvents:
		content = m.events.View()
	case ViewPipeline:
		content = m.pipeline.View()
	case ViewPlugins:
		content = m.plugins.View()
	default:
		content = m.dashboard.View()
	}

	// Build footer
	footerKeybinds := m.footerKeybinds()
	footer := widgets.NewFooter(footerKeybinds).WithWidth(m.width)

	// Help overlay
	var helpSection string
	if m.help {
		helpStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(theme.Secondary).
			Padding(1, 2)

		helpContent := lipgloss.JoinVertical(lipgloss.Left,
			theme.Title.Render("Help"),
			"",
			theme.KeybindKey.Render("Global")+":",
			"  "+theme.TitleMuted.Render("tab switch view, ? toggle help, q quit"),
			"",
			theme.KeybindKey.Render("Dashboard")+":",
			"  "+theme.TitleMuted.Render("↑/↓ select, enter/l logs, u up, d down, r restart, x kill"),
			"",
			theme.KeybindKey.Render("Service")+":",
			"  "+theme.TitleMuted.Render("tab stdout/stderr, f follow, / filter, esc back"),
			"",
			theme.KeybindKey.Render("Events")+":",
			"  "+theme.TitleMuted.Render("/ filter, ctrl+l clear, c clear events"),
			"",
			theme.KeybindKey.Render("Pipeline")+":",
			"  "+theme.TitleMuted.Render("b build, p prepare, v validation, ↑/↓ select, enter details"),
			"",
			theme.KeybindKey.Render("Plugins")+":",
			"  "+theme.TitleMuted.Render("↑/↓ select, enter expand, a expand all, A collapse all, esc back"),
		)
		helpSection = helpStyle.Render(helpContent)
	}

	// Compose layout
	sections := []string{header.Render()}

	if statusSection != "" {
		sections = append(sections, "", statusSection)
	}

	sections = append(sections, "", content)

	if helpSection != "" {
		sections = append(sections, "", helpSection)
	}

	sections = append(sections, footer.Render())

	// Join and ensure full width to prevent stray characters from previous renders
	output := lipgloss.JoinVertical(lipgloss.Left, sections...)

	// Pad each line to full width to clear any leftover characters
	lines := strings.Split(output, "\n")
	for i, line := range lines {
		lineWidth := lipgloss.Width(line)
		if lineWidth < m.width {
			lines[i] = line + strings.Repeat(" ", m.width-lineWidth)
		}
	}

	return strings.Join(lines, "\n")
}

func (m RootModel) footerKeybinds() []widgets.Keybind {
	switch m.active {
	case ViewDashboard:
		return []widgets.Keybind{
			{Key: "↑/↓", Label: "select"},
			{Key: "l", Label: "logs"},
			{Key: "u", Label: "up"},
			{Key: "d", Label: "down"},
			{Key: "r", Label: "restart"},
		}
	case ViewService:
		return []widgets.Keybind{
			{Key: "tab", Label: "stream"},
			{Key: "f", Label: "follow"},
			{Key: "/", Label: "filter"},
			{Key: "esc", Label: "back"},
		}
	case ViewEvents:
		return []widgets.Keybind{
			{Key: "/", Label: "filter"},
			{Key: "c", Label: "clear"},
			{Key: "↑/↓", Label: "scroll"},
		}
	case ViewPipeline:
		return []widgets.Keybind{
			{Key: "b", Label: "build"},
			{Key: "p", Label: "prepare"},
			{Key: "v", Label: "validate"},
			{Key: "↑/↓", Label: "select"},
		}
	case ViewPlugins:
		return []widgets.Keybind{
			{Key: "↑/↓", Label: "select"},
			{Key: "enter", Label: "expand"},
			{Key: "a/A", Label: "all"},
			{Key: "esc", Label: "back"},
		}
	default:
		return nil
	}
}

func (m RootModel) headerLines() int {
	// Header (2 lines: title + separator) + blank
	lines := 3
	if m.statusLine != "" {
		// Status line + blank
		lines += 2
	}
	// Footer (2 lines: separator + keybinds)
	lines += 2
	return lines
}

func (m RootModel) applyChildSizes() RootModel {
	childHeight := m.height - m.headerLines()
	if childHeight < 0 {
		childHeight = 0
	}
	m.dashboard = m.dashboard.WithSize(m.width, childHeight)
	m.service = m.service.WithSize(m.width, childHeight)
	m.events = m.events.WithSize(m.width, childHeight)
	m.pipeline = m.pipeline.WithSize(m.width, childHeight)
	m.plugins = m.plugins.WithSize(m.width, childHeight)
	return m
}
