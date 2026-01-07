# Events View Issues Analysis

## Summary

The events view implementation deviates significantly from the MO-006 design in ways that make it look "like ass" (direct user feedback). This document analyzes the issues, their root causes, and provides recommendations.

---

## Issue 1: Cramped, Unstyled Filter Bars

### Current Implementation
```
Filters: [1]● backend  [2]● frontend  [3]● postgres
Levels: ● DEBUG  ● INFO  ● WARN  ● ERROR [l] menu
```
- Plain text, no color coding
- Filled/empty circles not color-coded
- Keybindings crammed inline

### Design Specification
```
Filters: ● backend  ● frontend  ● postgres  ● system      [space] toggle
Levels:  ● DEBUG   ● INFO   ● WARN   ● ERROR              [l] level menu
```
- Service filters color-coded (enabled=green, disabled=gray)
- Level filters color-coded by level (DEBUG=gray, INFO=blue, WARN=yellow, ERROR=red)
- Keybindings right-aligned and separated

### Root Cause
Developer focused on functionality, not visual presentation. Used plain string concatenation instead of lipgloss styling.

---

## Issue 2: Missing Visual Hierarchy

### Current Implementation
```
╭─ Events (5) ────────────── [1-9] toggle [space] system... ─╮
│ Filters: [1]● backend...                                    │
│ Levels: ● DEBUG...                                          │
│ • 14:23:45 [backend]  Request completed                     │
│ • 14:23:44 [frontend] Asset compiled                        │
╰─────────────────────────────────────────────────────────────╯
```
- No separator between controls and content
- Title bar cluttered with too many keybindings
- Missing header status line

### Design Specification
```
┌─ Live Events ──────────────────────────────────── [ESC] back ──┐
│                                                                  │
│ Following: All Services              [f] filter [1-9] select    │
│ ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━ │
│                                                                  │
│ 14:34:12.234  [backend]    INFO  POST /api/moments              │
```
- Clear title with minimal keybinding
- Status line "Following: All Services" 
- Horizontal separator line
- Event content below

### Root Cause
Copy-paste coding pattern without stepping back to look at the full mockup.

---

## Issue 3: Missing Stats Line

### Current Implementation
(Not present)

### Design Specification
```
Stats: 1,247 events (18/sec)   Buffer: 500 lines   Dropped: 0
```
- Shows event count and throughput
- Shows buffer status
- Shows dropped events (for overflow detection)

### Root Cause
Feature was listed in tasks but not implemented.

---

## Issue 4: Missing Pause Toggle

### Current Implementation
No pause functionality.

### Design Specification
```
[p] pause [c] clear [s] save [/] search [ESC] back
```
- `p` pauses the event stream for reading
- Shows (PAUSED) indicator in title when paused
- Queued events shown on unpause

### Root Cause
Feature was listed but not implemented.

---

## Issue 5: Poor Event Line Formatting

### Current Implementation
```
• 14:23:45 [backend]  Request completed in 234ms
```
- Icon at start, not styled per level
- Short timestamp (no milliseconds)
- Level not shown as text
- Inconsistent spacing

### Design Specification
```
14:34:12.234  [backend]    INFO  POST /api/moments
```
- Timestamp with milliseconds
- Source in brackets, fixed width (10 chars)
- Level as styled text (INFO=blue, WARN=yellow, ERROR=red)
- Message after

### Root Cause
Developer didn't compare implementation to mockup line-by-line.

---

## Issue 6: Keybinding Clutter

### Current Implementation
All keybindings crammed into box title:
```
[1-9] toggle  [space] system  [l] levels  [/] text filter  [c] clear  [↑/↓] scroll
```
This is 75+ characters, doesn't fit on smaller terminals.

### Design Specification
Keybindings distributed:
- Box title: `[ESC] back`
- Status line: `[f] filter [1-9] select service`
- Bottom area: `[p] pause [c] clear [s] save [/] search [ESC] back`

### Root Cause
Lazy placement - put everything in one place instead of distributing logically.

---

## Summary of Root Causes

1. **Functionality-First Development**: Implemented features without visual polish
2. **Missing Visual Comparison**: Never compared implementation to ASCII mockup
3. **Copy-Paste Patterns**: Reused patterns from other views without adaptation
4. **Lazy Keybinding Placement**: Crammed all hints into title bar
5. **No Style Constants**: Used inline styles instead of theme colors
6. **Missing Refinement Pass**: Shipped first draft without refinement

---

## Recommended Fixes

### Priority 1: Visual Hierarchy
1. Add "Following: X Services" status line
2. Add horizontal separator
3. Move keybindings to appropriate locations

### Priority 2: Styling
1. Color-code filter toggles (green=enabled, gray=disabled)
2. Color-code level filters per level
3. Style event lines with fixed-width columns

### Priority 3: Missing Features
1. Add pause toggle (`p` key)
2. Add stats line (count, rate, buffer, dropped)
3. Add milliseconds to timestamps

### Priority 4: Layout Refinement
1. Fixed-width source column (10 chars)
2. Fixed-width level column (5 chars)
3. Proper alignment throughout

