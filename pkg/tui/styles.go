package tui

import "github.com/charmbracelet/lipgloss"

var (
	colorPurple = lipgloss.Color("#9D7CD8")
	colorCyan   = lipgloss.Color("#7DCFFF")
	colorGreen  = lipgloss.Color("#9ECE6A")
	colorYellow = lipgloss.Color("#E0AF68")
	colorRed    = lipgloss.Color("#F7768E")
	colorMuted  = lipgloss.Color("#737DA0")

	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(colorPurple)
	accentStyle   = lipgloss.NewStyle().Bold(true).Foreground(colorCyan)
	activeStyle   = lipgloss.NewStyle().Bold(true).Foreground(colorPurple).Underline(true)
	mutedStyle    = lipgloss.NewStyle().Foreground(colorMuted)
	healthyStyle  = lipgloss.NewStyle().Foreground(colorGreen)
	warningStyle  = lipgloss.NewStyle().Foreground(colorYellow)
	errorStyle    = lipgloss.NewStyle().Foreground(colorRed)
	selectedStyle = lipgloss.NewStyle().Bold(true).Foreground(colorCyan)
	keyStyle      = lipgloss.NewStyle().Bold(true).Foreground(colorPurple)
	borderStyle   = lipgloss.NewStyle().Foreground(colorPurple)
)

func stateStyle(value string) lipgloss.Style {
	switch value {
	case "ready", "healthy", "running", "succeeded", "ok":
		return healthyStyle
	case "failed", "unhealthy", "partial", "error":
		return errorStyle
	case "starting", "stopping", "planned", "unknown":
		return warningStyle
	default:
		return mutedStyle
	}
}
