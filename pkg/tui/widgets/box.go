package widgets

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/go-go-golems/devctl/pkg/tui/styles"
)

// Box renders a bordered container with an optional title.
type Box struct {
	Title      string
	TitleRight string
	Content    string
	Width      int
	Height     int
	Style      lipgloss.Style
	theme      styles.Theme
}

// NewBox creates a new box with default styling.
func NewBox(title string) Box {
	theme := styles.DefaultTheme()
	return Box{
		Title: title,
		Style: theme.Border,
		theme: theme,
	}
}

// WithContent sets the box content.
func (b Box) WithContent(content string) Box {
	b.Content = content
	return b
}

// WithTitleRight sets the right-aligned title text (e.g., keybindings).
func (b Box) WithTitleRight(text string) Box {
	b.TitleRight = text
	return b
}

// WithSize sets the box dimensions.
func (b Box) WithSize(width, height int) Box {
	b.Width = width
	b.Height = height
	return b
}

// WithStyle sets a custom style for the box.
func (b Box) WithStyle(s lipgloss.Style) Box {
	b.Style = s
	return b
}

// Render returns the styled box as a string.
func (b Box) Render() string {
	// Build title line
	titleStyle := b.theme.Title
	titleRightStyle := b.theme.TitleMuted

	titleLeft := ""
	if b.Title != "" {
		titleLeft = titleStyle.Render(b.Title)
	}

	titleRight := ""
	if b.TitleRight != "" {
		titleRight = titleRightStyle.Render(b.TitleRight)
	}

	// Calculate content width (account for border padding)
	contentWidth := b.Width - 2 // borders
	if contentWidth < 0 {
		contentWidth = 0
	}

	// Build the header line
	header := ""
	if titleLeft != "" || titleRight != "" {
		// Create a line with title left and right
		leftLen := lipgloss.Width(titleLeft)
		rightLen := lipgloss.Width(titleRight)
		spacing := contentWidth - leftLen - rightLen
		if spacing < 1 {
			spacing = 1
		}
		spacer := lipgloss.NewStyle().Width(spacing).Render("")
		header = lipgloss.JoinHorizontal(lipgloss.Top, titleLeft, spacer, titleRight)
	}

	// Build content with header
	var fullContent string
	if header != "" {
		fullContent = header + "\n" + b.Content
	} else {
		fullContent = b.Content
	}

	// Apply border and sizing
	style := b.Style
	if b.Width > 0 {
		style = style.Width(contentWidth)
	}
	if b.Height > 0 {
		// Account for header and borders
		innerHeight := b.Height - 2 // top and bottom borders
		if header != "" {
			innerHeight-- // header line
		}
		if innerHeight < 0 {
			innerHeight = 0
		}
		style = style.Height(innerHeight)
	}

	return style.Render(fullContent)
}

// SimpleBox renders a simple bordered box without a title.
func SimpleBox(content string, width int) string {
	return NewBox("").WithContent(content).WithSize(width, 0).Render()
}

