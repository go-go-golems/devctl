package widgets

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/go-go-golems/devctl/pkg/tui/styles"
)

// TableColumn defines a column in the table.
type TableColumn struct {
	Header string
	Width  int
	Align  lipgloss.Position
}

// TableRow represents a row in the table.
type TableRow struct {
	Icon     string
	Cells    []string
	Selected bool
}

// Table renders a styled table with selection support.
type Table struct {
	Columns []TableColumn
	Rows    []TableRow
	Cursor  int
	Width   int
	Height  int
	theme   styles.Theme
}

// NewTable creates a new table.
func NewTable(cols []TableColumn) Table {
	return Table{
		Columns: cols,
		theme:   styles.DefaultTheme(),
	}
}

// WithRows sets the table rows.
func (t Table) WithRows(rows []TableRow) Table {
	t.Rows = rows
	return t
}

// WithCursor sets the selected row index.
func (t Table) WithCursor(idx int) Table {
	t.Cursor = idx
	return t
}

// WithSize sets the table dimensions.
func (t Table) WithSize(width, height int) Table {
	t.Width = width
	t.Height = height
	return t
}

// Render returns the styled table as a string.
func (t Table) Render() string {
	if len(t.Rows) == 0 {
		return t.theme.TitleMuted.Render("(no data)")
	}

	theme := t.theme
	var lines []string

	// Calculate column widths if not specified
	cols := t.Columns
	if len(cols) == 0 && len(t.Rows) > 0 {
		// Auto-generate columns from first row
		cols = make([]TableColumn, len(t.Rows[0].Cells))
		for i := range cols {
			cols[i] = TableColumn{Width: 20}
		}
	}

	// Render rows
	for i, row := range t.Rows {
		isSelected := i == t.Cursor

		// Icon + cells
		var parts []string

		// Cursor indicator
		cursor := "  "
		if isSelected {
			cursor = theme.KeybindKey.Render("> ")
		}
		parts = append(parts, cursor)

		// Icon
		if row.Icon != "" {
			iconStyle := theme.StatusRunning
			switch row.Icon {
			case styles.IconError:
				iconStyle = theme.StatusDead
			case styles.IconPending, styles.IconSkipped:
				iconStyle = theme.StatusPending
			}
			parts = append(parts, iconStyle.Render(row.Icon)+" ")
		}

		// Cells
		for j, cell := range row.Cells {
			width := 20 // default
			if j < len(cols) && cols[j].Width > 0 {
				width = cols[j].Width
			}

			// Truncate or pad
			cellStr := cell
			if len(cellStr) > width {
				cellStr = cellStr[:width-1] + "…"
			}

			cellStyle := lipgloss.NewStyle().Width(width)
			if j < len(cols) {
				cellStyle = cellStyle.Align(cols[j].Align)
			}

			if isSelected {
				cellStyle = cellStyle.Bold(true).Foreground(theme.Text)
			} else {
				cellStyle = cellStyle.Foreground(theme.TextDim)
			}

			parts = append(parts, cellStyle.Render(cellStr))
		}

		line := lipgloss.JoinHorizontal(lipgloss.Top, parts...)

		// Apply selection background
		if isSelected {
			line = theme.Selected.Width(t.Width).Render(line)
		}

		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}

// ServiceRow is a convenience function for creating service table rows.
func ServiceRow(icon, name, status, pid, extra string, selected bool) TableRow {
	return TableRow{
		Icon:     icon,
		Cells:    []string{name, status, pid, extra},
		Selected: selected,
	}
}
