package models

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/go-go-golems/devctl/pkg/tui"
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
	var b strings.Builder
	filterLabel := ""
	if m.filter != "" {
		filterLabel = fmt.Sprintf(" filter=%q", m.filter)
	}
	b.WriteString(fmt.Sprintf("Events:%s\n", filterLabel))
	b.WriteString("scroll, / filter, ctrl+l clear filter, c clear events\n\n")

	if m.searching {
		b.WriteString(m.search.View())
		b.WriteString("\n\n")
	}

	if len(m.entries) == 0 {
		b.WriteString("(no events yet)\n")
		return b.String()
	}
	b.WriteString(m.vp.View())
	return b.String()
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
		lines = append(lines, fmt.Sprintf("- %s %s", ts.Format("15:04:05"), e.Text))
	}
	m.vp.SetContent(strings.Join(lines, "\n") + "\n")
	if gotoBottom {
		m.vp.GotoBottom()
	}
	return m
}
