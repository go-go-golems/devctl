# TUI Design Implementation Guidelines

## Purpose

This playbook provides guidelines for implementing TUI views to avoid the "myopic implementation" anti-pattern where functionality is implemented without visual polish, resulting in UIs that "look like ass."

---

## The Anti-Patterns We're Avoiding

### 1. Functionality-First Development
**Problem**: Implement all features, then realize it looks terrible.
**Solution**: Implement visual structure first, then fill in functionality.

### 2. Missing Visual Comparison
**Problem**: Never open the mockup next to the implementation.
**Solution**: Have ASCII mockup visible while coding. Compare line-by-line.

### 3. Copy-Paste Coding
**Problem**: Reuse patterns from other views without adaptation.
**Solution**: Each view has unique requirements. Adapt, don't copy.

### 4. Lazy Keybinding Placement
**Problem**: Cram all hints into one location.
**Solution**: Distribute keybindings logically across the UI.

### 5. Inline Styling
**Problem**: Use `lipgloss.NewStyle().Foreground(lipgloss.Color("240"))` everywhere.
**Solution**: Use theme constants from `styles.DefaultTheme()`.

---

## The Implementation Checklist

Before marking a view as "complete", verify each item:

### Visual Structure
- [ ] **Title bar** has minimal content (view name + 1-2 most important keys)
- [ ] **Status line** shows current state (e.g., "Following: All Services")
- [ ] **Separator** visually divides controls from content
- [ ] **Content area** uses the full available space
- [ ] **Footer/bottom area** has keybinding hints

### Alignment & Spacing
- [ ] **Fixed-width columns** for tabular data (source: 10, level: 5, PID: 8, etc.)
- [ ] **Consistent padding** around content (usually 1 space)
- [ ] **Vertical spacing** between sections (empty line or separator)
- [ ] **Timestamps** have consistent format (15:04:05 or 15:04:05.123)

### Color & Styling
- [ ] **Status indicators** use semantic colors (running=green, dead=red, pending=gray)
- [ ] **Log levels** are color-coded (DEBUG=gray, INFO=blue, WARN=yellow, ERROR=red)
- [ ] **Filter toggles** show enabled/disabled state (●=green enabled, ○=gray disabled)
- [ ] **Selected items** have highlight styling
- [ ] **Muted content** uses `theme.TitleMuted`

### Keybindings
- [ ] **Distributed** across title, status line, and footer
- [ ] **Grouped logically** (navigation together, actions together)
- [ ] **Not too long** - if >40 chars in one location, redistribute
- [ ] **Most used** keys are most visible

### Interactive Elements
- [ ] **Filter bars** show current state clearly
- [ ] **Toggle states** are obvious (filled vs empty circle)
- [ ] **Active mode** indicated (searching, paused, level menu open)
- [ ] **Error states** styled distinctly (red border/text)

---

## The Implementation Process

### Step 1: Study the Mockup (5 min)
1. Open the ASCII mockup
2. Count the distinct visual sections
3. Note the keybinding distribution
4. Note the color/style hints (✓, ●, colors mentioned)

### Step 2: Scaffold the Layout (10 min)
```go
func (m Model) View() string {
    theme := styles.DefaultTheme()
    var sections []string
    
    // 1. Status line
    sections = append(sections, m.renderStatusLine(theme))
    
    // 2. Separator
    sections = append(sections, m.renderSeparator())
    
    // 3. Main content
    sections = append(sections, m.renderContent(theme))
    
    // 4. Filter bars
    sections = append(sections, m.renderFilters(theme))
    
    // 5. Stats line
    sections = append(sections, m.renderStats(theme))
    
    // Wrap in box
    content := lipgloss.JoinVertical(lipgloss.Left, sections...)
    box := widgets.NewBox("Title").
        WithTitleRight("[esc] back").
        WithContent(content).
        WithSize(m.width, m.height)
    
    return box.Render()
}
```

### Step 3: Implement Each Section (30 min)
For each `render*` method:
1. Look at mockup for that section
2. Implement with proper styling
3. Test at different widths (80, 120, 200 cols)

### Step 4: Visual Comparison (5 min)
1. Run the TUI
2. Take a screenshot
3. Put side-by-side with mockup
4. Fix any discrepancies

### Step 5: Refinement Pass (10 min)
1. Check alignment issues
2. Check color consistency
3. Check spacing
4. Test edge cases (empty state, overflow)

---

## Common Patterns

### Pattern 1: Status Line with Right-Aligned Keybindings
```go
func (m Model) renderStatusLine(theme styles.Theme) string {
    left := theme.Title.Render("Following: All Services")
    right := theme.TitleMuted.Render("[f] filter [1-9] select")
    
    gap := m.width - lipgloss.Width(left) - lipgloss.Width(right) - 4
    if gap < 1 { gap = 1 }
    
    return left + strings.Repeat(" ", gap) + right
}
```

### Pattern 2: Color-Coded Toggle Bars
```go
func (m Model) renderFilterBar(theme styles.Theme) string {
    var parts []string
    parts = append(parts, theme.TitleMuted.Render("Filters:"))
    
    for i, name := range m.services {
        var icon string
        var style lipgloss.Style
        if m.enabled[name] {
            icon = "●"
            style = theme.StatusRunning // green
        } else {
            icon = "○"
            style = theme.TitleMuted // gray
        }
        part := fmt.Sprintf("%s %s", style.Render(icon), name)
        if i < 9 {
            part = fmt.Sprintf("[%d]%s", i+1, part)
        }
        parts = append(parts, part)
    }
    
    return strings.Join(parts, "  ")
}
```

### Pattern 3: Level-Colored Log Lines
```go
func (m Model) renderLogLine(e Entry, theme styles.Theme) string {
    ts := e.At.Format("15:04:05.000")
    
    // Fixed-width source
    source := fmt.Sprintf("[%-10s]", e.Source)
    if len(e.Source) > 10 {
        source = fmt.Sprintf("[%s…]", e.Source[:9])
    }
    
    // Level-colored
    var levelStyle lipgloss.Style
    switch e.Level {
    case "DEBUG": levelStyle = theme.TitleMuted
    case "INFO":  levelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("39")) // blue
    case "WARN":  levelStyle = lipgloss.NewStyle().Foreground(theme.Warning)
    case "ERROR": levelStyle = theme.StatusDead
    }
    level := levelStyle.Render(fmt.Sprintf("%-5s", e.Level))
    
    return fmt.Sprintf("%s  %s  %s  %s",
        theme.TitleMuted.Render(ts),
        theme.TitleMuted.Render(source),
        level,
        e.Text,
    )
}
```

### Pattern 4: Horizontal Separator
```go
func (m Model) renderSeparator() string {
    // Use thin horizontal line
    line := strings.Repeat("─", m.width-4)
    return lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render(line)
}
```

### Pattern 5: Stats Line
```go
func (m Model) renderStats(theme styles.Theme) string {
    stats := fmt.Sprintf("Stats: %d events (%.0f/sec)   Buffer: %d lines   Dropped: %d",
        m.totalCount,
        m.eventsPerSec,
        len(m.entries),
        m.droppedCount,
    )
    return theme.TitleMuted.Render(stats)
}
```

---

## Quality Gates

### Gate 1: Self-Review
Before creating a PR:
1. Run the TUI
2. Navigate to the view
3. Compare to mockup for 60 seconds
4. List 3 differences
5. Fix them

### Gate 2: Screenshot Comparison
1. Capture terminal screenshot
2. Put next to mockup image/ASCII
3. Differences should be intentional improvements, not omissions

### Gate 3: Width Testing
Test at:
- 80 columns (minimum)
- 120 columns (common)
- 200 columns (wide)

### Gate 4: Edge Cases
- Empty state (no events, no services)
- Overflow (100 events, 20 services)
- Long content (200 char message)

---

## Red Flags

If you see these in your View() code, stop and refactor:

❌ **Keybindings string > 50 chars** - Redistribute
❌ **`lipgloss.NewStyle().Foreground(...)` repeated** - Use theme
❌ **`fmt.Sprintf` with many `%s`** - Use structured rendering
❌ **No separator between header and content** - Add visual division
❌ **All content rendered in one loop** - Break into sections
❌ **Copy-pasted code from another model** - Adapt for this view

---

## Summary

The key insight is: **Implement structure and styling first, functionality second.**

A view that looks correct but has bugs is easier to fix than a view that works but looks terrible. Visual issues are often deep in the code structure and require significant refactoring.

When in doubt, open the mockup and compare line-by-line.

