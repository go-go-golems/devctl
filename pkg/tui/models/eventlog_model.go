package models

import (
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

type EventLogModel struct {
	max     int
	entries []tui.EventLogEntry

	width  int
	height int

	searching bool
	search    textinput.Model
	filter    string

	serviceFilters map[string]bool
	serviceOrder   []string

	levelMenu    bool
	levelFilters map[tui.LogLevel]bool

	paused       bool
	pausedQueue  []tui.EventLogEntry
	totalCount   int
	droppedCount int
	lastStatTime time.Time
	eventsPerSec float64
	recentCount  int

	vp viewport.Model
}

func NewEventLogModel() EventLogModel {
	search := textinput.New()
	search.Placeholder = "filter…"
	search.Prompt = "/ "
	search.CharLimit = 200

	m := EventLogModel{
		max:            200,
		entries:        nil,
		search:         search,
		serviceFilters: map[string]bool{},
		serviceOrder:   nil,
		levelFilters: map[tui.LogLevel]bool{
			tui.LogLevelDebug: true,
			tui.LogLevelInfo:  true,
			tui.LogLevelWarn:  true,
			tui.LogLevelError: true,
		},
	}
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
		if m.levelMenu {
			switch v.String() {
			case "esc", "enter", "l":
				m.levelMenu = false
				return m, nil
			case "a":
				m = m.setAllLevelFilters(true)
				m = m.refreshViewportContent(false)
				return m, nil
			case "n":
				m = m.setAllLevelFilters(false)
				m = m.refreshViewportContent(false)
				return m, nil
			case "d":
				m = m.toggleLevelFilter(tui.LogLevelDebug)
				return m, nil
			case "i":
				m = m.toggleLevelFilter(tui.LogLevelInfo)
				return m, nil
			case "w":
				m = m.toggleLevelFilter(tui.LogLevelWarn)
				return m, nil
			case "e":
				m = m.toggleLevelFilter(tui.LogLevelError)
				return m, nil
			default:
				return m, nil
			}
		}

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
			m.totalCount = 0
			m.droppedCount = 0
			m = m.refreshViewportContent(true)
			return m, nil
		case "l":
			m.levelMenu = true
			return m, nil
		case "p":
			m.paused = !m.paused
			if !m.paused && len(m.pausedQueue) > 0 {
				// Unpause: append queued events
				m.entries = append(m.entries, m.pausedQueue...)
				m.pausedQueue = nil
				if m.max > 0 && len(m.entries) > m.max {
					m.droppedCount += len(m.entries) - m.max
					m.entries = m.entries[len(m.entries)-m.max:]
				}
				m = m.refreshViewportContent(true)
			}
			return m, nil
		case " ":
			m = m.toggleServiceByName("system")
			return m, nil
		case "1", "2", "3", "4", "5", "6", "7", "8", "9":
			idx := int(v.String()[0] - '1')
			m = m.toggleServiceFilter(idx)
			return m, nil
		}

		var cmd tea.Cmd
		m.vp, cmd = m.vp.Update(v)
		return m, cmd
	}
	return m, nil
}

func (m EventLogModel) Append(e tui.EventLogEntry) EventLogModel {
	e = normalizeEventLogEntry(e)
	m.totalCount++
	m.recentCount++

	// Update events/sec every second
	if time.Since(m.lastStatTime) >= time.Second {
		m.eventsPerSec = float64(m.recentCount) / time.Since(m.lastStatTime).Seconds()
		m.recentCount = 0
		m.lastStatTime = time.Now()
	}

	// If paused, queue the event
	if m.paused {
		m.pausedQueue = append(m.pausedQueue, e)
		// Limit queue size
		if len(m.pausedQueue) > m.max {
			m.droppedCount++
			m.pausedQueue = m.pausedQueue[1:]
		}
		return m
	}

	m.entries = append(m.entries, e)
	if m.max > 0 && len(m.entries) > m.max {
		m.droppedCount++
		m.entries = append([]tui.EventLogEntry{}, m.entries[len(m.entries)-m.max:]...)
	}
	m = m.ensureServiceKnown(e.Source)
	m = m.refreshViewportContent(true)
	return m
}

func (m EventLogModel) View() string {
	theme := styles.DefaultTheme()

	var sections []string

	// Title - minimal, just the view name and back key
	title := "Live Events"
	if m.paused {
		title = "Live Events (PAUSED)"
	}
	titleRight := "[esc] back"

	// Status line: "Following: X Services" + filter hints
	statusLine := m.renderStatusLine(theme)
	sections = append(sections, statusLine)

	// Separator line
	separatorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	separator := separatorStyle.Render(strings.Repeat("─", m.width-4))
	sections = append(sections, separator)

	// Search input if active
	if m.searching {
		sections = append(sections, m.search.View())
	}

	// Events viewport
	if len(m.entries) == 0 {
		emptyMsg := theme.TitleMuted.Render("(no events yet - waiting for events...)")
		sections = append(sections, emptyMsg)
	} else {
		sections = append(sections, m.vp.View())
	}

	// Filter bars (styled)
	sections = append(sections, "")
	sections = append(sections, m.renderStyledServiceFilterBar(theme))
	sections = append(sections, m.renderStyledLevelFilterBar(theme))

	// Stats line
	sections = append(sections, m.renderStatsLine(theme))

	// Footer keybindings
	sections = append(sections, m.renderFooterKeybindings(theme))

	content := lipgloss.JoinVertical(lipgloss.Left, sections...)

	box := widgets.NewBox(title).
		WithTitleRight(titleRight).
		WithContent(content).
		WithSize(m.width, m.height)

	return box.Render()
}

func (m EventLogModel) resizeViewport() EventLogModel {
	// Calculate usable height for viewport
	// Fixed sections: status line, separator, filter bar, level bar, stats, footer, box chrome
	const statusLine = 1
	const separator = 1
	const filterBar = 1
	const levelBar = 1
	const statsLine = 1
	const footerLine = 1
	const boxChrome = 3 // top border, bottom border, empty line
	const padding = 2   // empty lines between sections

	reservedHeight := statusLine + separator + filterBar + levelBar + statsLine + footerLine + boxChrome + padding

	if m.searching {
		reservedHeight++
	}

	usableHeight := m.height - reservedHeight
	if usableHeight < 3 {
		usableHeight = 3
	}

	m.vp.Width = maxInt(0, m.width-4) // Account for box borders
	m.vp.Height = usableHeight
	m = m.refreshViewportContent(false)
	return m
}

func (m EventLogModel) refreshViewportContent(gotoBottom bool) EventLogModel {
	theme := styles.DefaultTheme()

	if len(m.entries) == 0 {
		m.vp.SetContent("")
		return m
	}

	for _, entry := range m.entries {
		entry = normalizeEventLogEntry(entry)
		m = m.ensureServiceKnown(entry.Source)
	}

	lines := make([]string, 0, len(m.entries))
	for _, e := range m.entries {
		e = normalizeEventLogEntry(e)
		if m.filter != "" && !strings.Contains(e.Text, m.filter) {
			continue
		}

		if enabled, ok := m.serviceFilters[e.Source]; ok && !enabled {
			continue
		}
		if enabled, ok := m.levelFilters[e.Level]; ok && !enabled {
			continue
		}

		line := m.formatEventLine(theme, e)
		lines = append(lines, line)
	}
	m.vp.SetContent(strings.Join(lines, "\n") + "\n")
	if gotoBottom {
		m.vp.GotoBottom()
	}
	return m
}

// formatEventLine formats a single event with proper styling and alignment.
func (m EventLogModel) formatEventLine(theme styles.Theme, e tui.EventLogEntry) string {
	ts := e.At
	if ts.IsZero() {
		ts = time.Now()
	}

	// Timestamp with milliseconds
	tsStr := ts.Format("15:04:05.000")

	// Fixed-width source (10 chars)
	source := e.Source
	if len(source) > 10 {
		source = source[:9] + "…"
	}
	sourceStr := fmt.Sprintf("[%-10s]", source)

	// Level with color
	var levelStyle lipgloss.Style
	switch e.Level {
	case tui.LogLevelDebug:
		levelStyle = theme.TitleMuted
	case tui.LogLevelInfo:
		levelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("39")) // blue
	case tui.LogLevelWarn:
		levelStyle = lipgloss.NewStyle().Foreground(theme.Warning)
	case tui.LogLevelError:
		levelStyle = theme.StatusDead
	default:
		levelStyle = theme.TitleMuted
	}
	levelStr := levelStyle.Render(fmt.Sprintf("%-5s", e.Level))

	// Text - truncate if too long
	text := e.Text
	maxTextLen := m.width - 40 // Leave room for timestamp, source, level
	if maxTextLen > 10 && len(text) > maxTextLen {
		text = text[:maxTextLen-3] + "..."
	}

	// Apply level style to text too for errors/warnings
	var textStyle lipgloss.Style
	switch e.Level {
	case tui.LogLevelDebug:
		textStyle = theme.TitleMuted
	case tui.LogLevelInfo:
		textStyle = theme.TitleMuted
	case tui.LogLevelError:
		textStyle = theme.StatusDead
	case tui.LogLevelWarn:
		textStyle = lipgloss.NewStyle().Foreground(theme.Warning)
	default:
		textStyle = theme.TitleMuted
	}

	return fmt.Sprintf("%s  %s  %s  %s",
		theme.TitleMuted.Render(tsStr),
		theme.TitleMuted.Render(sourceStr),
		levelStr,
		textStyle.Render(text),
	)
}

func (m EventLogModel) ensureServiceKnown(source string) EventLogModel {
	source = strings.TrimSpace(source)
	if source == "" {
		source = "system"
	}

	if m.serviceFilters == nil {
		m.serviceFilters = map[string]bool{}
	}
	if _, ok := m.serviceFilters[source]; ok {
		return m
	}

	m.serviceFilters[source] = true
	m.serviceOrder = append(m.serviceOrder, source)
	sort.Strings(m.serviceOrder)
	return m
}

func (m EventLogModel) toggleServiceFilter(idx int) EventLogModel {
	if idx < 0 || idx >= len(m.serviceOrder) {
		return m
	}
	return m.toggleServiceByName(m.serviceOrder[idx])
}

func (m EventLogModel) toggleServiceByName(name string) EventLogModel {
	name = strings.TrimSpace(name)
	if name == "" {
		return m
	}
	m = m.ensureServiceKnown(name)
	m.serviceFilters[name] = !m.serviceFilters[name]
	m = m.refreshViewportContent(false)
	return m
}

func (m EventLogModel) toggleLevelFilter(level tui.LogLevel) EventLogModel {
	if m.levelFilters == nil {
		m.levelFilters = map[tui.LogLevel]bool{}
	}
	if _, ok := m.levelFilters[level]; !ok {
		m.levelFilters[level] = true
	}
	m.levelFilters[level] = !m.levelFilters[level]
	m = m.refreshViewportContent(false)
	return m
}

func (m EventLogModel) setAllLevelFilters(enabled bool) EventLogModel {
	if m.levelFilters == nil {
		m.levelFilters = map[tui.LogLevel]bool{}
	}
	m.levelFilters[tui.LogLevelDebug] = enabled
	m.levelFilters[tui.LogLevelInfo] = enabled
	m.levelFilters[tui.LogLevelWarn] = enabled
	m.levelFilters[tui.LogLevelError] = enabled
	return m
}

func (m EventLogModel) renderStatusLine(theme styles.Theme) string {
	// Count enabled services
	enabledCount := 0
	for _, enabled := range m.serviceFilters {
		if enabled {
			enabledCount++
		}
	}

	var left string
	if enabledCount == len(m.serviceFilters) || len(m.serviceFilters) == 0 {
		left = theme.Title.Render("Following: All Services")
	} else if enabledCount == 0 {
		left = theme.TitleMuted.Render("Following: None (all filtered)")
	} else {
		left = theme.Title.Render(fmt.Sprintf("Following: %d of %d Services", enabledCount, len(m.serviceFilters)))
	}

	right := theme.TitleMuted.Render("[f] filter  [1-9] select service")

	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right) - 6
	if gap < 1 {
		gap = 1
	}

	return left + strings.Repeat(" ", gap) + right
}

func (m EventLogModel) renderStyledServiceFilterBar(theme styles.Theme) string {
	if len(m.serviceOrder) == 0 {
		return theme.TitleMuted.Render("Services: (none)")
	}

	var parts []string
	parts = append(parts, theme.TitleMuted.Render("Services:"))

	for i, name := range m.serviceOrder {
		var icon string
		var style lipgloss.Style
		if enabled, ok := m.serviceFilters[name]; ok && enabled {
			icon = "●"
			style = theme.StatusRunning
		} else {
			icon = "○"
			style = theme.TitleMuted
		}

		// Format: [1]● name
		keyHint := ""
		if i < 9 {
			keyHint = fmt.Sprintf("[%d]", i+1)
		}
		part := fmt.Sprintf("%s%s %s", theme.KeybindKey.Render(keyHint), style.Render(icon), name)
		parts = append(parts, part)
	}

	parts = append(parts, "  "+theme.TitleMuted.Render("[space] toggle"))

	return strings.Join(parts, "  ")
}

func (m EventLogModel) renderStyledLevelFilterBar(theme styles.Theme) string {
	levels := []struct {
		level tui.LogLevel
		color lipgloss.Color
	}{
		{tui.LogLevelDebug, lipgloss.Color("240")}, // gray
		{tui.LogLevelInfo, lipgloss.Color("39")},   // blue
		{tui.LogLevelWarn, theme.Warning},          // yellow
		{tui.LogLevelError, lipgloss.Color("196")}, // red
	}

	var parts []string
	parts = append(parts, theme.TitleMuted.Render("Levels: "))

	for _, l := range levels {
		var icon string
		style := lipgloss.NewStyle().Foreground(l.color)
		if enabled, ok := m.levelFilters[l.level]; ok && enabled {
			icon = "●"
		} else {
			icon = "○"
			style = theme.TitleMuted
		}
		parts = append(parts, style.Render(fmt.Sprintf("%s %s", icon, l.level)))
	}

	if m.levelMenu {
		parts = append(parts, "  "+theme.KeybindKey.Render("[d/i/w/e]")+" toggle")
		parts = append(parts, theme.KeybindKey.Render("[a]")+" all")
		parts = append(parts, theme.KeybindKey.Render("[n]")+" none")
		parts = append(parts, theme.KeybindKey.Render("[esc]")+" close")
	} else {
		parts = append(parts, "  "+theme.TitleMuted.Render("[l] level menu"))
	}

	return strings.Join(parts, "  ")
}

func (m EventLogModel) renderStatsLine(theme styles.Theme) string {
	stats := fmt.Sprintf("Stats: %d events (%.0f/sec)   Buffer: %d/%d lines   Dropped: %d",
		m.totalCount,
		m.eventsPerSec,
		len(m.entries),
		m.max,
		m.droppedCount,
	)
	return theme.TitleMuted.Render(stats)
}

func (m EventLogModel) renderFooterKeybindings(theme styles.Theme) string {
	var parts []string

	if m.paused {
		parts = append(parts, theme.StatusDead.Render("[p] resume"))
	} else {
		parts = append(parts, theme.KeybindKey.Render("[p]")+" pause")
	}

	parts = append(parts, theme.KeybindKey.Render("[c]")+" clear")
	parts = append(parts, theme.KeybindKey.Render("[/]")+" search")
	parts = append(parts, theme.KeybindKey.Render("[↑/↓]")+" scroll")

	return theme.TitleMuted.Render(strings.Join(parts, "   "))
}

func normalizeEventLogEntry(e tui.EventLogEntry) tui.EventLogEntry {
	if strings.TrimSpace(e.Source) == "" {
		e.Source = "system"
	}
	if e.Level == "" {
		e.Level = tui.LogLevelInfo
	}
	return e
}
