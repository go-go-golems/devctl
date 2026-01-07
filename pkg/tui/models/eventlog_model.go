package models

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/go-go-golems/devctl/pkg/tui"
	"github.com/go-go-golems/devctl/pkg/tui/styles"
	"github.com/go-go-golems/devctl/pkg/tui/widgets"
)

type EventLogModel struct {
	max     int
	entries []tui.EventLogEntry

	width  int
	height int

	searching bool
	search    textinput.Model
	filter    string

	vp viewport.Model
}

func NewEventLogModel() EventLogModel {
	search := textinput.New()
	search.Placeholder = "filter…"
	search.Prompt = "/ "
	search.CharLimit = 200

	m := EventLogModel{max: 200, entries: nil, search: search}
	m.vp = viewport.New(0, 0)
	return m
}

func (m EventLogModel) WithSize(width, height int) EventLogModel {
	m.width, m.height = width, height
	m = m.resizeViewport()
	return m
}

func (m EventLogModel) Update(msg tea.Msg) (EventLogModel, tea.Cmd) {
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
		m = m.resizeViewport()
		return m, nil
	case tea.KeyMsg:
		if m.searching {
			switch v.String() {
			case "esc":
				m.searching = false
				m.search.Blur()
				return m, nil
			case "enter":
				m.filter = strings.TrimSpace(m.search.Value())
				m.searching = false
				m.search.Blur()
				m = m.refreshViewportContent(true)
				return m, nil
			}

			var cmd tea.Cmd
			m.search, cmd = m.search.Update(v)
			return m, cmd
		}

		switch v.String() {
		case "/":
			m.searching = true
			m.search.SetValue(m.filter)
			m.search.CursorEnd()
			m.search.Focus()
			return m, nil
		case "ctrl+l":
			m.filter = ""
			m.search.SetValue("")
			m = m.refreshViewportContent(true)
			return m, nil
		case "c":
			m.entries = nil
			m = m.refreshViewportContent(true)
			return m, nil
		}

		var cmd tea.Cmd
		m.vp, cmd = m.vp.Update(v)
		return m, cmd
	}
	return m, nil
}

func (m EventLogModel) Append(e tui.EventLogEntry) EventLogModel {
	m.entries = append(m.entries, e)
	if m.max > 0 && len(m.entries) > m.max {
		m.entries = append([]tui.EventLogEntry{}, m.entries[len(m.entries)-m.max:]...)
	}
	m = m.refreshViewportContent(true)
	return m
}

func (m EventLogModel) View() string {
	theme := styles.DefaultTheme()

	var sections []string

	// Header with filter info
	titleRight := "[/] filter  [c] clear  [↑/↓] scroll"
	if m.filter != "" {
		titleRight = fmt.Sprintf("filter=%q  %s", m.filter, titleRight)
	}

	// Search input if active
	if m.searching {
		sections = append(sections, m.search.View())
	}

	// Events viewport
	if len(m.entries) == 0 {
		emptyBox := widgets.NewBox(fmt.Sprintf("Events (%d)", len(m.entries))).
			WithTitleRight(titleRight).
			WithContent(theme.TitleMuted.Render("(no events yet)")).
			WithSize(m.width, 5)
		sections = append(sections, emptyBox.Render())
	} else {
		eventsBox := widgets.NewBox(fmt.Sprintf("Events (%d)", len(m.entries))).
			WithTitleRight(titleRight).
			WithContent(m.vp.View()).
			WithSize(m.width, m.vp.Height+3)
		sections = append(sections, eventsBox.Render())
	}

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func (m EventLogModel) resizeViewport() EventLogModel {
	usableHeight := m.height - 4
	if usableHeight < 3 {
		usableHeight = 3
	}
	m.vp.Width = maxInt(0, m.width)
	m.vp.Height = usableHeight
	m.vp.HighPerformanceRendering = false
	m = m.refreshViewportContent(false)
	return m
}

func (m EventLogModel) refreshViewportContent(gotoBottom bool) EventLogModel {
	theme := styles.DefaultTheme()

	if len(m.entries) == 0 {
		m.vp.SetContent("")
		return m
	}

	lines := make([]string, 0, len(m.entries))
	for _, e := range m.entries {
		if m.filter != "" && !strings.Contains(e.Text, m.filter) {
			continue
		}
		ts := e.At
		if ts.IsZero() {
			ts = time.Now()
		}

		// Determine icon and style based on content
		icon := styles.IconInfo
		style := theme.TitleMuted
		text := e.Text

		if strings.Contains(text, "failed") || strings.Contains(text, "error") || strings.Contains(text, "Error") {
			icon = styles.IconError
			style = theme.StatusDead
		} else if strings.Contains(text, "ok:") || strings.Contains(text, "success") || strings.Contains(text, "passed") {
			icon = styles.IconSuccess
			style = theme.StatusRunning
		} else if strings.Contains(text, "warn") || strings.Contains(text, "Warning") {
			icon = styles.IconWarning
			style = lipgloss.NewStyle().Foreground(theme.Warning)
		} else if strings.Contains(text, "started") || strings.Contains(text, "running") {
			icon = styles.IconRunning
			style = theme.StatusRunning
		}

		line := lipgloss.JoinHorizontal(lipgloss.Center,
			style.Render(icon),
			" ",
			theme.TitleMuted.Render(ts.Format("15:04:05")),
			"  ",
			style.Render(text),
		)
		lines = append(lines, line)
	}
	m.vp.SetContent(strings.Join(lines, "\n") + "\n")
	if gotoBottom {
		m.vp.GotoBottom()
	}
	return m
}
