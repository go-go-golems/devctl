package widgets

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/go-go-golems/devctl/pkg/tui/styles"
)

// Footer renders a styled keybindings bar.
type Footer struct {
	Keybinds []Keybind
	Width    int
	theme    styles.Theme
}

// NewFooter creates a new footer.
func NewFooter(keybinds []Keybind) Footer {
	return Footer{
		Keybinds: keybinds,
		theme:    styles.DefaultTheme(),
	}
}

// WithWidth sets the footer width.
func (f Footer) WithWidth(w int) Footer {
	f.Width = w
	return f
}

// Render returns the styled footer as a string.
func (f Footer) Render() string {
	theme := f.theme

	// Separator line - generate exactly the right number of box-drawing chars
	sepWidth := f.Width
	if sepWidth <= 0 {
		sepWidth = 80
	}
	// Each ━ is 1 character wide (3 bytes UTF-8)
	sepChars := make([]rune, sepWidth)
	for i := range sepChars {
		sepChars[i] = '━'
	}
	separator := lipgloss.NewStyle().
		Foreground(theme.Muted).
		Render(string(sepChars))

	// Keybinds line
	keybindsLine := RenderKeybinds(f.Keybinds, theme)

	// Center the keybinds and pad to full width
	keybindsWidth := lipgloss.Width(keybindsLine)
	padding := (f.Width - keybindsWidth) / 2
	if padding < 0 {
		padding = 0
	}
	paddedKeybinds := lipgloss.NewStyle().
		PaddingLeft(padding).
		Width(f.Width).
		Render(keybindsLine)

	return lipgloss.JoinVertical(lipgloss.Left, separator, paddedKeybinds)
}
