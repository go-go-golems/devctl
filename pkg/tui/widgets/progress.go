package widgets

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ProgressBar renders a horizontal progress bar with percentage.
type ProgressBar struct {
	percent    int
	width      int
	style      lipgloss.Style
	filledChar rune
	emptyChar  rune
	showText   bool
}

// NewProgressBar creates a new progress bar with the given percentage (0-100).
func NewProgressBar(percent int) ProgressBar {
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	return ProgressBar{
		percent:    percent,
		width:      20,
		filledChar: '█',
		emptyChar:  '░',
		showText:   true,
	}
}

// WithWidth sets the width of the progress bar (not including percentage text).
func (p ProgressBar) WithWidth(width int) ProgressBar {
	if width < 5 {
		width = 5
	}
	p.width = width
	return p
}

// WithStyle sets the style for the filled portion.
func (p ProgressBar) WithStyle(style lipgloss.Style) ProgressBar {
	p.style = style
	return p
}

// WithChars sets the characters used for filled and empty portions.
func (p ProgressBar) WithChars(filled, empty rune) ProgressBar {
	p.filledChar = filled
	p.emptyChar = empty
	return p
}

// WithShowText controls whether to show the percentage text.
func (p ProgressBar) WithShowText(show bool) ProgressBar {
	p.showText = show
	return p
}

// Render returns the rendered progress bar string.
func (p ProgressBar) Render() string {
	filled := p.width * p.percent / 100
	empty := p.width - filled

	filledStr := strings.Repeat(string(p.filledChar), filled)
	emptyStr := strings.Repeat(string(p.emptyChar), empty)

	bar := p.style.Render(filledStr) + emptyStr

	if p.showText {
		return fmt.Sprintf("%s %3d%%", bar, p.percent)
	}
	return bar
}

// RenderCompact returns a compact progress bar (e.g., for inline display).
func (p ProgressBar) RenderCompact() string {
	// Use a smaller width for compact display
	compactWidth := 10
	filled := compactWidth * p.percent / 100
	empty := compactWidth - filled

	filledStr := strings.Repeat(string(p.filledChar), filled)
	emptyStr := strings.Repeat(string(p.emptyChar), empty)

	return fmt.Sprintf("[%s%s]", p.style.Render(filledStr), emptyStr)
}
