package models

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/go-go-golems/devctl/pkg/tui"
	"github.com/go-go-golems/devctl/pkg/tui/styles"
	"github.com/go-go-golems/devctl/pkg/tui/widgets"
)

// PluginInfo contains detailed information about a plugin.
type PluginInfo struct {
	ID       string
	Path     string
	Status   string // "active" | "disabled" | "error"
	Priority int
	Protocol string
	Ops      []string
	Streams  []string
	Commands []string
}

// PluginModel displays and manages the list of plugins.
type PluginModel struct {
	width    int
	height   int
	plugins  []PluginInfo
	selected int
	expanded map[int]bool
}

// NewPluginModel creates a new plugin list model.
func NewPluginModel() PluginModel {
	return PluginModel{
		expanded: map[int]bool{},
	}
}

// WithSize sets the model dimensions.
func (m PluginModel) WithSize(width, height int) PluginModel {
	m.width, m.height = width, height
	return m
}

// WithPlugins updates the plugin list from state snapshot.
func (m PluginModel) WithPlugins(plugins []tui.PluginSummary) PluginModel {
	// Convert PluginSummary to PluginInfo
	m.plugins = make([]PluginInfo, 0, len(plugins))
	for _, p := range plugins {
		m.plugins = append(m.plugins, PluginInfo{
			ID:       p.ID,
			Path:     p.Path,
			Status:   p.Status,
			Priority: p.Priority,
			Protocol: "v1", // Default protocol
			Ops:      p.Ops,
			Streams:  p.Streams,
		})
	}
	// Reset selection if out of bounds
	if m.selected >= len(m.plugins) {
		m.selected = maxInt(0, len(m.plugins)-1)
	}
	return m
}

// Update handles input events.
func (m PluginModel) Update(msg tea.Msg) (PluginModel, tea.Cmd) {
	switch v := msg.(type) {
	case tea.KeyMsg:
		switch v.String() {
		case "up", "k":
			if m.selected > 0 {
				m.selected--
			}
			return m, nil
		case "down", "j":
			if m.selected < len(m.plugins)-1 {
				m.selected++
			}
			return m, nil
		case "enter", "i":
			// Toggle expanded state for selected plugin
			m.expanded[m.selected] = !m.expanded[m.selected]
			return m, nil
		case "a":
			// Expand all
			for i := range m.plugins {
				m.expanded[i] = true
			}
			return m, nil
		case "A":
			// Collapse all
			m.expanded = map[int]bool{}
			return m, nil
		case "esc":
			return m, func() tea.Msg { return tui.NavigateBackMsg{} }
		}
	}
	return m, nil
}

// View renders the plugin list.
func (m PluginModel) View() string {
	theme := styles.DefaultTheme()

	if len(m.plugins) == 0 {
		emptyBox := widgets.NewBox("Plugins").
			WithContent(lipgloss.JoinVertical(lipgloss.Left,
				theme.TitleMuted.Render("No plugins configured."),
				"",
				theme.TitleMuted.Render("Add plugins to .devctl.yaml to extend devctl functionality."),
			)).
			WithSize(m.width, 6)
		return emptyBox.Render()
	}

	var sections []string

	// Header with count
	headerContent := lipgloss.JoinHorizontal(lipgloss.Center,
		theme.Title.Render(fmt.Sprintf("%d Plugins", len(m.plugins))),
		"  ",
		theme.TitleMuted.Render("[↑/↓] select  [enter] expand  [a/A] expand/collapse all  [esc] back"),
	)
	sections = append(sections, headerContent, "")

	// Render each plugin card
	for i, p := range m.plugins {
		isSelected := i == m.selected
		isExpanded := m.expanded[i]
		card := m.renderPluginCard(i, p, isSelected, isExpanded, theme)
		sections = append(sections, card)
	}

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// renderPluginCard renders a single plugin card.
func (m PluginModel) renderPluginCard(index int, p PluginInfo, selected, expanded bool, theme styles.Theme) string {
	// Status icon
	statusIcon := styles.IconSuccess
	statusStyle := theme.StatusRunning
	statusText := "Active"
	switch p.Status {
	case "disabled":
		statusIcon = styles.IconPending
		statusStyle = theme.StatusPending
		statusText = "Disabled"
	case "error":
		statusIcon = styles.IconError
		statusStyle = theme.StatusDead
		statusText = "Error"
	}

	// Selection cursor
	cursor := "  "
	if selected {
		cursor = theme.KeybindKey.Render("> ")
	}

	// Stream indicator
	streamIndicator := ""
	if len(p.Streams) > 0 {
		streamIndicator = theme.StatusRunning.Render("  📊 stream")
	}

	// Plugin name with status
	titleLine := lipgloss.JoinHorizontal(lipgloss.Center,
		cursor,
		statusStyle.Render(statusIcon),
		" ",
		theme.Title.Render(p.ID),
		"  ",
		theme.TitleMuted.Render(fmt.Sprintf("priority: %d", p.Priority)),
		streamIndicator,
	)

	if !expanded {
		// Compact view: just title line with path
		pathLine := theme.TitleMuted.Render(fmt.Sprintf("    Path: %s", truncatePath(p.Path, m.width-12)))
		return lipgloss.JoinVertical(lipgloss.Left, titleLine, pathLine)
	}

	// Expanded view: full details in a box
	var contentLines []string

	// Status line
	contentLines = append(contentLines,
		lipgloss.JoinHorizontal(lipgloss.Left,
			theme.TitleMuted.Width(12).Render("Status:"),
			statusStyle.Render(statusIcon),
			" ",
			statusStyle.Render(statusText),
			"    ",
			theme.TitleMuted.Render("Priority: "),
			theme.Title.Render(fmt.Sprintf("%d", p.Priority)),
		),
	)

	// Path
	contentLines = append(contentLines,
		lipgloss.JoinHorizontal(lipgloss.Left,
			theme.TitleMuted.Width(12).Render("Path:"),
			theme.Title.Render(truncatePath(p.Path, m.width-20)),
		),
	)

	// Protocol
	if p.Protocol != "" {
		contentLines = append(contentLines,
			lipgloss.JoinHorizontal(lipgloss.Left,
				theme.TitleMuted.Width(12).Render("Protocol:"),
				theme.Title.Render(p.Protocol),
			),
		)
	}

	// Capabilities section
	contentLines = append(contentLines, "")
	contentLines = append(contentLines, theme.TitleMuted.Render("Capabilities:"))

	// Ops
	opsText := "(none)"
	if len(p.Ops) > 0 {
		opsText = strings.Join(p.Ops, ", ")
	}
	contentLines = append(contentLines,
		lipgloss.JoinHorizontal(lipgloss.Left,
			theme.TitleMuted.Width(14).Render("  Ops:"),
			theme.Title.Render(opsText),
		),
	)

	// Streams
	streamsText := "(none)"
	if len(p.Streams) > 0 {
		streamsText = strings.Join(p.Streams, ", ")
	}
	contentLines = append(contentLines,
		lipgloss.JoinHorizontal(lipgloss.Left,
			theme.TitleMuted.Width(14).Render("  Streams:"),
			theme.Title.Render(streamsText),
		),
	)

	// Commands
	commandsText := "(none)"
	if len(p.Commands) > 0 {
		commandsText = strings.Join(p.Commands, ", ")
	}
	contentLines = append(contentLines,
		lipgloss.JoinHorizontal(lipgloss.Left,
			theme.TitleMuted.Width(14).Render("  Commands:"),
			theme.Title.Render(commandsText),
		),
	)

	// Build box
	box := widgets.NewBox(p.ID).
		WithContent(lipgloss.JoinVertical(lipgloss.Left, contentLines...)).
		WithSize(m.width, len(contentLines)+3)

	return box.Render()
}

// truncatePath shortens a path if it exceeds maxLen.
func truncatePath(path string, maxLen int) string {
	if maxLen < 10 {
		maxLen = 10
	}
	if len(path) <= maxLen {
		return path
	}
	// Try to preserve the filename by showing ".../<basename>"
	parts := strings.Split(path, "/")
	if len(parts) > 1 {
		basename := parts[len(parts)-1]
		if len(basename)+4 <= maxLen {
			return ".../" + basename
		}
	}
	return path[:maxLen-3] + "..."
}
