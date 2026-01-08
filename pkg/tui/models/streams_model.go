package models

import (
	"encoding/json"
	"fmt"
	"sort"
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

type StreamsModel struct {
	width  int
	height int

	streams  []streamRow
	selected int

	eventsByKey map[string][]string

	creating  bool
	createIn  textinput.Model
	createErr string

	vp      viewport.Model
	vpReady bool
}

type streamRow struct {
	Key        string
	PluginID   string
	Op         string
	StreamID   string
	Status     string // "running" | "ended" | "error"
	At         time.Time
	EventCount int // Total events received
}

func NewStreamsModel() StreamsModel {
	in := textinput.New()
	in.Placeholder = `{"op":"telemetry.stream","plugin_id":"","input":{"count":3,"interval_ms":250}}`
	in.Prompt = "new> "
	in.CharLimit = 2000

	m := StreamsModel{
		eventsByKey: map[string][]string{},
		createIn:    in,
	}
	m.vp = viewport.New(0, 0)
	return m
}

func (m StreamsModel) WithSize(width, height int) StreamsModel {
	m.width, m.height = width, height
	m = m.resizeViewport()
	return m
}

func (m StreamsModel) Update(msg tea.Msg) (StreamsModel, tea.Cmd) {
	switch v := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = v.Width, v.Height
		m = m.resizeViewport()
		return m, nil
	case tea.KeyMsg:
		if m.creating {
			switch v.String() {
			case "esc":
				m.creating = false
				m.createErr = ""
				m.createIn.Blur()
				return m, nil
			case "enter":
				req, err := parseStreamStartJSON(m.createIn.Value())
				if err != nil {
					m.createErr = err.Error()
					return m, nil
				}
				m.creating = false
				m.createErr = ""
				m.createIn.SetValue("")
				m.createIn.Blur()
				return m, func() tea.Msg { return tui.StreamStartRequestMsg{Request: req} }
			}
			var cmd tea.Cmd
			m.createIn, cmd = m.createIn.Update(v)
			return m, cmd
		}

		switch v.String() {
		case "n":
			m.creating = true
			m.createErr = ""
			m.createIn.SetValue("")
			m.createIn.Focus()
			return m, nil
		case "esc":
			return m, func() tea.Msg { return tui.NavigateBackMsg{} }
		case "j":
			if m.selected < len(m.streams)-1 {
				m.selected++
				m = m.refreshViewportContent(true)
			}
			return m, nil
		case "k":
			if m.selected > 0 {
				m.selected--
				m = m.refreshViewportContent(true)
			}
			return m, nil
		case "x":
			if key := m.selectedKey(); key != "" {
				return m, func() tea.Msg { return tui.StreamStopRequestMsg{Request: tui.StreamStopRequest{StreamKey: key}} }
			}
			return m, nil
		case "c":
			if key := m.selectedKey(); key != "" {
				delete(m.eventsByKey, key)
				m = m.refreshViewportContent(true)
			}
			return m, nil
		}

		var cmd tea.Cmd
		m.vp, cmd = m.vp.Update(v)
		return m, cmd
	case tui.StreamStartedMsg:
		m = m.onStreamStarted(v.Stream)
		return m, nil
	case tui.StreamEventMsg:
		m = m.onStreamEvent(v.Event)
		return m, nil
	case tui.StreamEndedMsg:
		m = m.onStreamEnded(v.End)
		return m, nil
	default:
		return m, nil
	}
}

func (m StreamsModel) View() string {
	theme := styles.DefaultTheme()

	if len(m.streams) == 0 && !m.creating {
		box := widgets.NewBox("Streams").
			WithContent(lipgloss.JoinVertical(lipgloss.Left,
				theme.TitleMuted.Render("No active streams."),
				"",
				theme.TitleMuted.Render("Press [n] to start a new stream."),
			)).
			WithSize(m.width, 6)
		return box.Render()
	}

	var headerLines []string
	headerLines = append(headerLines,
		lipgloss.JoinHorizontal(lipgloss.Center,
			theme.Title.Render(fmt.Sprintf("%d Streams", len(m.streams))),
			"  ",
			theme.TitleMuted.Render("[n] new  [j/k] select  [↑/↓] scroll  [x] stop  [c] clear  [esc] back"),
		),
		"",
	)

	list := m.renderStreamList(theme)
	eventsBox := m.renderEventsBox(theme)

	sections := []string{
		lipgloss.JoinVertical(lipgloss.Left, headerLines...),
		list,
		"",
		eventsBox,
	}

	if m.creating {
		sections = append(sections, "", m.renderCreateBox(theme))
	}

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func (m StreamsModel) renderStreamList(theme styles.Theme) string {
	var lines []string
	maxRows := 6
	start := 0
	if m.selected >= maxRows {
		start = m.selected - maxRows + 1
	}
	end := start + maxRows
	if end > len(m.streams) {
		end = len(m.streams)
	}

	now := time.Now()
	for i := start; i < end; i++ {
		r := m.streams[i]
		cursor := "  "
		if i == m.selected {
			cursor = theme.KeybindKey.Render("> ")
		}

		statusStyle := theme.TitleMuted
		statusIcon := "○"
		switch r.Status {
		case "running":
			statusStyle = theme.StatusRunning
			statusIcon = "●"
		case "ended":
			statusStyle = theme.StatusPending
			statusIcon = "○"
		case "error":
			statusStyle = theme.StatusDead
			statusIcon = "✗"
		}

		// Format duration
		duration := now.Sub(r.At)
		durationStr := formatDuration(duration)

		// Format event count
		eventsStr := fmt.Sprintf("%d events", r.EventCount)

		lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Left,
			cursor,
			statusStyle.Render(statusIcon+" "+r.Status),
			"  ",
			theme.Title.Render(r.Op),
			"  ",
			theme.TitleMuted.Render(r.PluginID),
			"  ",
			theme.TitleMuted.Render(durationStr),
			"  ",
			theme.TitleMuted.Render(eventsStr),
		))
	}

	content := lipgloss.JoinVertical(lipgloss.Left, lines...)
	return widgets.NewBox("Active Streams").
		WithContent(content).
		WithSize(m.width, maxInt(6, len(lines)+3)).
		Render()
}


func (m StreamsModel) renderEventsBox(theme styles.Theme) string {
	title := "Stream Events"
	if r := m.selectedRow(); r != nil {
		title = fmt.Sprintf("Stream Events: %s", r.Op)
	}
	return widgets.NewBox(title).
		WithTitleRight("[↑/↓] scroll").
		WithContent(m.vp.View()).
		WithSize(m.width, maxInt(6, m.height-12)).
		Render()
}

func (m StreamsModel) renderCreateBox(theme styles.Theme) string {
	var lines []string
	lines = append(lines, theme.TitleMuted.Render("Paste JSON: {op, plugin_id?, input?}, then press Enter."))
	lines = append(lines, m.createIn.View())
	if m.createErr != "" {
		lines = append(lines, theme.StatusDead.Render(m.createErr))
	}
	return widgets.NewBox("Start Stream").
		WithTitleRight("[enter] start  [esc] cancel").
		WithContent(lipgloss.JoinVertical(lipgloss.Left, lines...)).
		WithSize(m.width, 6).
		Render()
}

func (m StreamsModel) onStreamStarted(ev tui.StreamStarted) StreamsModel {
	row := streamRow{
		Key:      ev.StreamKey,
		PluginID: ev.PluginID,
		Op:       ev.Op,
		StreamID: ev.StreamID,
		Status:   "running",
		At:       ev.At,
	}

	found := false
	for i := range m.streams {
		if m.streams[i].Key == ev.StreamKey {
			m.streams[i] = row
			found = true
			break
		}
	}
	if !found {
		m.streams = append(m.streams, row)
	}
	m.streams = sortStreams(m.streams)
	m.selected = indexOfKey(m.streams, ev.StreamKey)
	if m.selected < 0 {
		m.selected = 0
	}
	m = m.refreshViewportContent(true)
	return m
}

func (m StreamsModel) onStreamEvent(ev tui.StreamEvent) StreamsModel {
	line := formatStreamEventLine(ev)
	m.eventsByKey[ev.StreamKey] = append(m.eventsByKey[ev.StreamKey], line)
	const maxLines = 500
	if len(m.eventsByKey[ev.StreamKey]) > maxLines {
		m.eventsByKey[ev.StreamKey] = m.eventsByKey[ev.StreamKey][len(m.eventsByKey[ev.StreamKey])-maxLines:]
	}
	// Increment event count for the stream row
	for i := range m.streams {
		if m.streams[i].Key == ev.StreamKey {
			m.streams[i].EventCount++
			break
		}
	}
	if m.selectedKey() == ev.StreamKey {
		m = m.refreshViewportContent(false)
	}
	return m
}

func (m StreamsModel) onStreamEnded(ev tui.StreamEnded) StreamsModel {
	for i := range m.streams {
		if m.streams[i].Key == ev.StreamKey {
			if ev.Ok {
				m.streams[i].Status = "ended"
			} else {
				m.streams[i].Status = "error"
			}
			break
		}
	}
	if ev.Error != "" {
		m.eventsByKey[ev.StreamKey] = append(m.eventsByKey[ev.StreamKey], themeLine("end", ev.Error))
	}
	if m.selectedKey() == ev.StreamKey {
		m = m.refreshViewportContent(false)
	}
	return m
}

func (m StreamsModel) resizeViewport() StreamsModel {
	m.vp.Width = maxInt(0, m.width-4)
	m.vp.Height = maxInt(0, m.height-16)
	m.vpReady = true
	m = m.refreshViewportContent(false)
	return m
}

func (m StreamsModel) refreshViewportContent(gotoBottom bool) StreamsModel {
	if !m.vpReady {
		return m
	}
	key := m.selectedKey()
	lines := m.eventsByKey[key]
	if len(lines) == 0 {
		m.vp.SetContent("(no events yet)\n")
	} else {
		m.vp.SetContent(strings.Join(lines, "\n") + "\n")
	}
	if gotoBottom {
		m.vp.GotoBottom()
	}
	return m
}

func (m StreamsModel) selectedKey() string {
	r := m.selectedRow()
	if r == nil {
		return ""
	}
	return r.Key
}

func (m StreamsModel) selectedRow() *streamRow {
	if m.selected < 0 || m.selected >= len(m.streams) {
		return nil
	}
	return &m.streams[m.selected]
}

func parseStreamStartJSON(s string) (tui.StreamStartRequest, error) {
	var raw map[string]any
	if err := json.Unmarshal([]byte(s), &raw); err != nil {
		return tui.StreamStartRequest{}, err
	}
	op, _ := raw["op"].(string)
	pluginID, _ := raw["plugin_id"].(string)
	label, _ := raw["label"].(string)

	var input map[string]any
	if v, ok := raw["input"].(map[string]any); ok {
		input = v
	}
	if op == "" {
		return tui.StreamStartRequest{}, fmt.Errorf("missing op")
	}
	return tui.StreamStartRequest{PluginID: pluginID, Op: op, Input: input, Label: label}, nil
}

func sortStreams(in []streamRow) []streamRow {
	out := append([]streamRow{}, in...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Status != out[j].Status {
			if out[i].Status == "running" {
				return true
			}
			if out[j].Status == "running" {
				return false
			}
		}
		if !out[i].At.Equal(out[j].At) {
			return out[i].At.Before(out[j].At)
		}
		return out[i].Key < out[j].Key
	})
	return out
}

func indexOfKey(rows []streamRow, key string) int {
	for i := range rows {
		if rows[i].Key == key {
			return i
		}
	}
	return -1
}

func formatStreamEventLine(ev tui.StreamEvent) string {
	prefix := ev.Event.Event
	if prefix == "" {
		prefix = "event"
	}
	msg := ev.Event.Message
	if msg == "" && len(ev.Event.Fields) > 0 {
		if b, err := json.Marshal(ev.Event.Fields); err == nil {
			msg = string(b)
		}
	}
	if msg == "" {
		msg = "-"
	}
	return fmt.Sprintf("[%s] %s", prefix, msg)
}

func themeLine(prefix, text string) string {
	if prefix == "" {
		return text
	}
	return fmt.Sprintf("[%s] %s", prefix, text)
}
