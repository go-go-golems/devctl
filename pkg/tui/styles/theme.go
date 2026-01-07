package styles

import "github.com/charmbracelet/lipgloss"

// Theme defines the color palette and base styles for the TUI.
type Theme struct {
	// Colors
	Primary   lipgloss.Color
	Secondary lipgloss.Color
	Success   lipgloss.Color
	Warning   lipgloss.Color
	Error     lipgloss.Color
	Muted     lipgloss.Color
	Text      lipgloss.Color
	TextDim   lipgloss.Color

	// Base styles
	Border        lipgloss.Style
	Title         lipgloss.Style
	TitleMuted    lipgloss.Style
	Selected      lipgloss.Style
	Keybind       lipgloss.Style
	KeybindKey    lipgloss.Style
	StatusRunning lipgloss.Style
	StatusDead    lipgloss.Style
	StatusPending lipgloss.Style
}

// DefaultTheme returns the default devctl TUI theme.
func DefaultTheme() Theme {
	primary := lipgloss.Color("#7C3AED")   // Purple
	secondary := lipgloss.Color("#06B6D4") // Cyan
	success := lipgloss.Color("#22C55E")   // Green
	warning := lipgloss.Color("#EAB308")   // Yellow
	errorC := lipgloss.Color("#EF4444")    // Red
	muted := lipgloss.Color("#6B7280")     // Gray
	text := lipgloss.Color("#F9FAFB")      // White
	textDim := lipgloss.Color("#9CA3AF")   // Light gray

	return Theme{
		Primary:   primary,
		Secondary: secondary,
		Success:   success,
		Warning:   warning,
		Error:     errorC,
		Muted:     muted,
		Text:      text,
		TextDim:   textDim,

		Border: lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(muted),

		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(text),

		TitleMuted: lipgloss.NewStyle().
			Foreground(textDim),

		Selected: lipgloss.NewStyle().
			Bold(true).
			Foreground(text).
			Background(lipgloss.Color("#374151")), // Dark gray background

		Keybind: lipgloss.NewStyle().
			Foreground(textDim),

		KeybindKey: lipgloss.NewStyle().
			Bold(true).
			Foreground(secondary),

		StatusRunning: lipgloss.NewStyle().
			Foreground(success),

		StatusDead: lipgloss.NewStyle().
			Foreground(errorC),

		StatusPending: lipgloss.NewStyle().
			Foreground(muted),
	}
}

// DefaultStyles returns the default theme for convenience.
var DefaultStyles = DefaultTheme()

