package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type layoutKind int

const (
	layoutCompact layoutKind = iota
	layoutStandard
	layoutWide
)

func layoutFor(width int) layoutKind {
	switch {
	case width >= 100:
		return layoutWide
	case width >= 70:
		return layoutStandard
	default:
		return layoutCompact
	}
}

func panel(title, content string, width int) string {
	width = max(16, width)
	inner := max(1, width-4)
	body := lipgloss.NewStyle().Width(inner).Render(content)
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorPurple).
		Padding(0, 1).
		Width(inner).
		Render(accentStyle.Render(title) + "\n" + body)
}

func joinPanels(left, right string, width int) string {
	gap := 2
	leftWidth := max(34, width*58/100)
	rightWidth := max(30, width-leftWidth-gap)
	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		panel("Services", left, leftWidth),
		strings.Repeat(" ", gap),
		panel("Current service", right, rightWidth),
	)
}

func renderKey(key, label string) string {
	return keyStyle.Render("["+key+"]") + " " + label
}

func clipLine(value string, width int) string {
	if width <= 0 {
		return ""
	}
	return truncate(value, width)
}

func summaryBadge(label string, value int) string {
	return fmt.Sprintf("%s %s", mutedStyle.Render(label), accentStyle.Render(fmt.Sprint(value)))
}
