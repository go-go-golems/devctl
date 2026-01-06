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
)

type RootModel struct {
	width  int
	height int

	active ViewID
	help   bool

	dashboard DashboardModel
	service   ServiceModel
	events    EventLogModel

	publishAction func(tui.ActionRequest) error
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
		publishAction: opts.PublishAction,
	}
	m.dashboard = m.dashboard.WithSize(defaultWidth, defaultHeight)
	m.service = m.service.WithSize(defaultWidth, defaultHeight)
	m.events = m.events.WithSize(defaultWidth, defaultHeight)
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
		m.dashboard = m.dashboard.WithSize(w, h)
		m.service = m.service.WithSize(w, h)
		m.events = m.events.WithSize(w, h)
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
			if m.active == ViewDashboard {
				m.active = ViewEvents
			} else {
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
		}
	case tui.StateSnapshotMsg:
		m.dashboard = m.dashboard.WithSnapshot(v.Snapshot)
		m.service = m.service.WithSnapshot(v.Snapshot)
		return m, nil
	case tui.EventLogAppendMsg:
		m.events = m.events.Append(v.Entry)
		return m, nil
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
	switch m.active {
	case ViewService:
		b.WriteString(m.service.View())
	case ViewEvents:
		b.WriteString(m.events.View())
	default:
		b.WriteString(m.dashboard.View())
	}

	if m.help {
		b.WriteString("\n--- help ---\n")
		b.WriteString("global: tab switch view, ? toggle help, q quit\n")
		b.WriteString("dashboard: ↑/↓ select service, enter/l open service logs, x kill (y/n)\n")
		b.WriteString("service: tab switch stdout/stderr, f follow, / filter, ctrl+l clear, esc back\n")
		b.WriteString("events: / filter, ctrl+l clear filter, c clear events\n")
	}
	return b.String()
}
