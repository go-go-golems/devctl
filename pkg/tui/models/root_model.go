package models

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/go-go-golems/devctl/pkg/tui"
)

type ViewID string

const (
	ViewDashboard ViewID = "dashboard"
	ViewService   ViewID = "service"
	ViewEvents    ViewID = "events"
	ViewPipeline  ViewID = "pipeline"
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

	publishAction func(tui.ActionRequest) error

	statusLine string
}

type RootModelOptions struct {
	PublishAction func(tui.ActionRequest) error
}

func NewRootModel(opts RootModelOptions) RootModel {
	const defaultWidth = 80
	const defaultHeight = 24

	m := RootModel{
		width:         defaultWidth,
		height:        defaultHeight,
		active:        ViewDashboard,
		dashboard:     NewDashboardModel(),
		service:       NewServiceModel(),
		events:        NewEventLogModel(),
		pipeline:      NewPipelineModel(),
		publishAction: opts.PublishAction,
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
		}
	case tui.StateSnapshotMsg:
		m.dashboard = m.dashboard.WithSnapshot(v.Snapshot)
		m.service = m.service.WithSnapshot(v.Snapshot)
		return m, nil
	case tui.EventLogAppendMsg:
		m.events = m.events.Append(v.Entry)
		if s := strings.TrimSpace(v.Entry.Text); s != "" {
			if strings.HasPrefix(s, "action failed:") ||
				strings.HasPrefix(s, "action publish failed:") ||
				strings.HasPrefix(s, "failed SIGTERM") {
				m.statusLine = s
				m = m.applyChildSizes()
			} else if strings.HasPrefix(s, "action ok:") ||
				strings.HasPrefix(s, "sent SIGTERM") {
				m.statusLine = s
				m = m.applyChildSizes()
			}
		}
		return m, nil
	case tui.PipelineRunStartedMsg:
		var cmd tea.Cmd
		m.pipeline, cmd = m.pipeline.Update(v)
		return m, cmd
	case tui.PipelineRunFinishedMsg:
		var cmd tea.Cmd
		m.pipeline, cmd = m.pipeline.Update(v)
		return m, cmd
	case tui.PipelinePhaseStartedMsg:
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
		return m, nil
	case tui.ActionRequestMsg:
		if m.publishAction == nil {
			m.events = m.events.Append(tui.EventLogEntry{At: time.Now(), Text: fmt.Sprintf("action ignored: %s (no publisher)", v.Request.Kind)})
			return m, nil
		}
		req := v.Request
		return m, func() tea.Msg {
			if err := m.publishAction(req); err != nil {
				return tui.EventLogAppendMsg{Entry: tui.EventLogEntry{At: time.Now(), Text: fmt.Sprintf("action publish failed: %v", err)}}
			}
			return tui.EventLogAppendMsg{Entry: tui.EventLogEntry{At: time.Now(), Text: fmt.Sprintf("action requested: %s", req.Kind)}}
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
	case ViewDashboard:
		var cmd tea.Cmd
		m.dashboard, cmd = m.dashboard.Update(msg)
		return m, cmd
	default:
		return m, nil
	}
}

func (m RootModel) View() string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("devctl tui — %s  (tab switch, ? help, q quit)\n\n", m.active))
	if m.statusLine != "" {
		b.WriteString(fmt.Sprintf("Status: %s\n\n", m.statusLine))
	}
	switch m.active {
	case ViewService:
		b.WriteString(m.service.View())
	case ViewEvents:
		b.WriteString(m.events.View())
	case ViewPipeline:
		b.WriteString(m.pipeline.View())
	default:
		b.WriteString(m.dashboard.View())
	}

	if m.help {
		b.WriteString("\n--- help ---\n")
		b.WriteString("global: tab switch view, ? toggle help, q quit\n")
		b.WriteString("dashboard: ↑/↓ select service, enter/l open service logs, x kill (y/n)\n")
		b.WriteString("service: tab switch stdout/stderr, f follow, / filter, ctrl+l clear, esc back\n")
		b.WriteString("events: / filter, ctrl+l clear filter, c clear events\n")
		b.WriteString("pipeline: (read-only for now)\n")
	}
	return b.String()
}

func (m RootModel) headerLines() int {
	// Title line + blank line.
	lines := 2
	if m.statusLine != "" {
		// Status line + blank line.
		lines += 2
	}
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
	return m
}
