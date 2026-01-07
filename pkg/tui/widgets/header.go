package widgets

import (
	"fmt"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/go-go-golems/devctl/pkg/tui/styles"
)

// Keybind represents a keybinding hint.
type Keybind struct {
	Key   string
	Label string
}

// Header renders a styled title bar with status and keybindings.
type Header struct {
	Title      string
	Status     string
	StatusIcon string
	StatusOk   bool
	Uptime     time.Duration
	Width      int
	Keybinds   []Keybind
	theme      styles.Theme
}

// NewHeader creates a new header.
func NewHeader(title string) Header {
	return Header{
		Title: title,
		theme: styles.DefaultTheme(),
	}
}

// WithStatus sets the status text and icon.
func (h Header) WithStatus(icon, status string, ok bool) Header {
	h.StatusIcon = icon
	h.Status = status
	h.StatusOk = ok
	return h
}

// WithUptime sets the uptime duration.
func (h Header) WithUptime(d time.Duration) Header {
	h.Uptime = d
	return h
}

// WithKeybinds sets the keybinding hints.
func (h Header) WithKeybinds(kb []Keybind) Header {
	h.Keybinds = kb
	return h
}

// WithWidth sets the header width.
func (h Header) WithWidth(w int) Header {
	h.Width = w
	return h
}

// Render returns the styled header as a string.
func (h Header) Render() string {
	theme := h.theme

	// Title on the left
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(theme.Text).
		Background(theme.Primary).
		Padding(0, 1)
	titlePart := titleStyle.Render(h.Title)

	// Status in the middle
	statusPart := ""
	if h.Status != "" {
		var statusStyle lipgloss.Style
		if h.StatusOk {
			statusStyle = theme.StatusRunning
		} else {
			statusStyle = theme.StatusDead
		}
		icon := h.StatusIcon
		if icon == "" {
			icon = styles.IconSystem
		}
		statusPart = statusStyle.Render(icon) + " " + lipgloss.NewStyle().Foreground(theme.Text).Render(h.Status)
	}

	// Uptime on the right
	uptimePart := ""
	if h.Uptime > 0 {
		uptimePart = theme.TitleMuted.Render(fmt.Sprintf("Uptime: %s", formatDuration(h.Uptime)))
	}

	// Keybinds
	keybindsPart := ""
	if len(h.Keybinds) > 0 {
		keybindsPart = RenderKeybinds(h.Keybinds, theme)
	}

	// Layout: [Title] [Status]           [Uptime] [Keybinds]
	leftParts := titlePart
	if statusPart != "" {
		leftParts = lipgloss.JoinHorizontal(lipgloss.Center, leftParts, "  ", statusPart)
	}

	rightParts := ""
	if uptimePart != "" {
		rightParts = uptimePart
	}
	if keybindsPart != "" {
		if rightParts != "" {
			rightParts = lipgloss.JoinHorizontal(lipgloss.Center, rightParts, "  ", keybindsPart)
		} else {
			rightParts = keybindsPart
		}
	}

	// Calculate spacing
	leftWidth := lipgloss.Width(leftParts)
	rightWidth := lipgloss.Width(rightParts)
	spacing := h.Width - leftWidth - rightWidth
	if spacing < 1 {
		spacing = 1
	}

	spacer := lipgloss.NewStyle().Width(spacing).Render("")
	headerLine := lipgloss.JoinHorizontal(lipgloss.Top, leftParts, spacer, rightParts)

	// Add separator line
	separator := lipgloss.NewStyle().
		Foreground(theme.Muted).
		Width(h.Width).
		Render("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	return lipgloss.JoinVertical(lipgloss.Left, headerLine, separator[:min(h.Width, len(separator))])
}

// RenderKeybinds renders a list of keybindings.
func RenderKeybinds(keybinds []Keybind, theme styles.Theme) string {
	parts := make([]string, 0, len(keybinds)*2)
	for i, kb := range keybinds {
		if i > 0 {
			parts = append(parts, theme.TitleMuted.Render(" "))
		}
		parts = append(parts, theme.KeybindKey.Render("["+kb.Key+"]"))
		parts = append(parts, theme.Keybind.Render(" "+kb.Label))
	}
	return lipgloss.JoinHorizontal(lipgloss.Center, parts...)
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second

	if h > 0 {
		return fmt.Sprintf("%dh %dm %ds", h, m, s)
	}
	if m > 0 {
		return fmt.Sprintf("%dm %ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

