---
Title: TUI Architecture and Visual Improvement Analysis
Ticket: MO-008-IMPROVE-TUI-LOOKS
Status: active
Topics:
  - tui
  - ui-components
  - backend
DocType: analysis
Intent: long-term
Owners: []
RelatedFiles:
  - Path: devctl/ttmp/2026/01/06/MO-006-DEVCTL-TUI--create-a-devctl-tui/sources/local/01-devctl-tui-layout.md
    Note: Original ASCII baseline mockups defining the target UX
  - Path: devctl/ttmp/2026/01/06/MO-006-DEVCTL-TUI--create-a-devctl-tui/design-doc/01-devctl-tui-layout-and-implementation-design.md
    Note: Implementation design and milestone plan
  - Path: devctl/ttmp/2026/01/06/MO-006-DEVCTL-TUI--create-a-devctl-tui/working-note/01-devctl-tui-code-mapping-and-integration-analysis.md
    Note: Watermill→Bubble Tea architecture and model composition design
  - Path: devctl/pkg/tui/models/root_model.go
    Note: Current root model implementation (coordinator)
  - Path: devctl/pkg/tui/models/dashboard_model.go
    Note: Current dashboard model (services table, actions)
  - Path: devctl/pkg/tui/models/service_model.go
    Note: Current service detail model (logs viewport)
  - Path: devctl/pkg/tui/models/pipeline_model.go
    Note: Current pipeline model (phases, validation)
  - Path: devctl/pkg/tui/models/eventlog_model.go
    Note: Current event log model (timeline)
  - Path: devctl/pkg/tui/msgs.go
    Note: Bubble Tea message types
  - Path: devctl/pkg/tui/pipeline_events.go
    Note: Pipeline event and phase definitions
ExternalSources: []
Summary: Comprehensive gap analysis between the MO-006 design baseline and current TUI implementation, with detailed refactoring recommendations for visual improvement using lipgloss and bubbles.
LastUpdated: 2026-01-06T20:22:00-05:00
WhatFor: Guide the refactoring effort to bring the TUI to the polished state defined in MO-006.
WhenToUse: When planning or implementing TUI visual and architectural improvements.
---

# TUI Architecture and Visual Improvement Analysis

## Executive Summary

This document analyzes the gap between the **target UX** defined in MO-006 (ASCII baseline mockups) and the **current implementation** in `devctl/pkg/tui`. The current TUI is functional but visually rough—it uses plain text rendering with minimal formatting. The target UX shows bordered boxes, status icons, color-coded states, and a polished dashboard layout.

**Key findings:**
1. The current model structure is sound and matches the MO-006 architecture
2. Visual rendering is the primary gap—no lipgloss styling, no bordered boxes, no icons
3. The widget decomposition needs refinement to enable reusable styled components
4. Dependencies (`lipgloss v1.1.1`, `bubbles v0.21.1`) are already available

## Current State ASCII Screenshots

### Current Dashboard View

```
devctl tui — dashboard  (tab switch, ? help, q quit)

Status: action ok: restart

System: Running

RepoRoot: /tmp/devctl-fixture-abc123
Started: 2026-01-06 14:23:45

Services (3):  (↑/↓ select, enter logs, u up, d down, r restart, x kill)
> backend              pid=12847  alive
  frontend             pid=12849  alive
  postgres             pid=12843  dead (exit=2)

```

### Current Service Detail View

```
devctl tui — service  (tab switch, ? help, q quit)

Service: backend  (alive)  supervisor_pid=12847  stdout  follow=on
tab switch stream, f follow, / filter, ctrl+l clear, esc back

Path: /tmp/devctl-fixture-abc123/.devctl/logs/backend.stdout.log

14:23:45.234  INFO  Request GET /api/moments?limit=20
14:23:45.456  INFO  Query completed in 234ms (45 rows)
14:23:42.123  INFO  WebSocket connection established
14:23:40.891  INFO  Database connection pool initialized
14:23:40.567  INFO  Starting HTTP server on :8083
```

### Current Pipeline View

```
devctl tui — pipeline  (tab switch, ? help, q quit)

Pipeline: up  run=abc123  (ok)
Started: 2026-01-06 14:23:45

Focus: build  (b build, p prepare, v validation; ↑/↓ select; enter details)

Phases:
- mutate_config: ok (0.2s)
- build: ok (5.3s)
- prepare: ok (1.3s)
- validate: ok (0.5s)
- launch_plan: ok (0.1s)
- supervise: ok (2.1s)
- state_save: ok (0.0s)

Build steps:
> backend-deps: ok (2.1s)
  frontend-deps: ok (3.2s)

Validate: ok (0 warnings)

Launch plan: 3 services
Services: backend, frontend, postgres
```

### Current Event Log View

```
devctl tui — events  (tab switch, ? help, q quit)

Events:
scroll, / filter, ctrl+l clear filter, c clear events

- 14:23:45 action requested: up
- 14:23:45 pipeline: mutate_config started
- 14:23:46 pipeline: build started
- 14:23:51 pipeline: prepare started
- 14:23:52 pipeline: validate started
- 14:23:53 pipeline: launch_plan started
- 14:23:53 pipeline: supervise started
- 14:23:55 action ok: up
```

## Target State ASCII Screenshots (from MO-006)

### Target Dashboard

```
┌─ DevCtl - moments ──────────────────────────────────────────────────── [↑/↓/q] ─┐
│                                                                                   │
│ ● System Status: Running                                      Uptime: 2h 34m 12s │
│ ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━ │
│                                                                                   │
│ Services (3)                                                    [l] logs [r] restart │
│ ┌─────────────────────────────────────────────────────────────────────────────┐ │
│ │ ✓ backend           Running    PID 12847   Healthy   CPU 12%   MEM 245MB   │ │
│ │ ✓ frontend          Running    PID 12849   Healthy   CPU  3%   MEM  89MB   │ │
│ │ ✓ postgres          Running    PID 12843   Healthy   CPU  1%   MEM 156MB   │ │
│ └─────────────────────────────────────────────────────────────────────────────┘ │
│                                                                                   │
│ Recent Events (5)                                             [f] follow [c] clear │
│ ┌─────────────────────────────────────────────────────────────────────────────┐ │
│ │ 14:23:45  backend    ℹ  Request completed in 234ms                          │ │
│ │ 14:23:42  frontend   ℹ  Asset compilation complete                          │ │
│ │ 14:23:40  backend    ℹ  Connected to database                               │ │
│ │ 14:23:38  postgres   ℹ  Database ready for connections                      │ │
│ │ 14:23:35  system     ✓  All health checks passed                            │ │
│ └─────────────────────────────────────────────────────────────────────────────┘ │
│                                                                                   │
│ Plugins (3 active)                                                                │
│  • moments-config   (priority: 10)   ops: 2   commands: 0                        │
│  • moments-build    (priority: 15)   ops: 3   commands: 1                        │
│  • moments-launch   (priority: 20)   ops: 4   commands: 2                        │
│                                                                                   │
│ [s] services [p] plugins [e] events [h] help [q] quit                            │
└───────────────────────────────────────────────────────────────────────────────────┘
```

### Target Service Detail

```
┌─ Service: backend ────────────────────────────────────────────────── [ESC] back ─┐
│                                                                                   │
│ Status: ✓ Running                                           Started: 2h 34m ago  │
│ Health: Healthy (http://localhost:8083/rpc/v1/health)      Last check: 2s ago    │
│ ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━ │
│                                                                                   │
│ Process Info                                                                      │
│  PID:         12847                                                               │
│  Command:     go run ./cmd/moments-server serve                                   │
│  Working Dir: /home/user/moments/backend                                          │
│  CPU:         12.3%                                                               │
│  Memory:      245.8 MB                                                            │
│                                                                                   │
│ Environment                                                                       │
│  PORT=8083  DB_URL=postgresql://localhost:5432/moments  ENV=development           │
│                                                                                   │
│ ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━ │
│ Live Logs                                                    [↑/↓] scroll [f] find │
│ ┌─────────────────────────────────────────────────────────────────────────────┐ │
│ │ 14:23:45.234  INFO  Request GET /api/moments?limit=20                       │ │
│ │ 14:23:45.456  INFO  Query completed in 234ms (45 rows)                      │ │
│ │ 14:23:42.123  INFO  WebSocket connection established (client: web-abc123)   │ │
│ │ 14:23:40.891  INFO  Database connection pool initialized (max: 10)          │ │
│ │ 14:23:40.567  INFO  Starting HTTP server on :8083                           │ │
│ └─────────────────────────────────────────────────────────────────────────────┘ │
│                                                                                   │
│ [r] restart [s] stop [k] kill [d] detach [ESC] back                              │
└───────────────────────────────────────────────────────────────────────────────────┘
```

### Target Pipeline Progress

```
┌─ DevCtl - moments ──────────────────────────────────────────────────────────────┐
│                                                                                   │
│ ⚙ System Status: Starting                                    Phase: Build [2/5]  │
│ ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━ │
│                                                                                   │
│ Pipeline Progress                                                                 │
│ ┌─────────────────────────────────────────────────────────────────────────────┐ │
│ │ ✓ Config Mutation     3 plugins applied              0.2s                   │ │
│ │ ▶ Build               Running...                     5.3s                   │ │
│ │   Prepare             Pending                                                │ │
│ │   Validate            Pending                                                │ │
│ │   Launch              Pending                                                │ │
│ └─────────────────────────────────────────────────────────────────────────────┘ │
│                                                                                   │
│ Build Steps (2/3 complete)                                                        │
│ ┌─────────────────────────────────────────────────────────────────────────────┐ │
│ │ ✓ backend-deps        pnpm install                   2.1s                   │ │
│ │ ✓ frontend-deps       pnpm install                   3.2s                   │ │
│ │ ▶ backend-compile     go build ./cmd/moments-server  5.3s (running)         │ │
│ │   frontend-build      pnpm build                          (pending)         │ │
│ └─────────────────────────────────────────────────────────────────────────────┘ │
│                                                                                   │
│ Live Output                                                                       │
│ ┌─────────────────────────────────────────────────────────────────────────────┐ │
│ │ [backend-compile] Building target: cmd/moments-server                       │ │
│ │ [backend-compile] Compiling 247 packages...                                 │ │
│ │ [backend-compile] ██████████████████░░░░░░░░░░ 65%                          │ │
│ └─────────────────────────────────────────────────────────────────────────────┘ │
│                                                                                   │
│ [Ctrl+C] cancel                                                                   │
└───────────────────────────────────────────────────────────────────────────────────┘
```

### Target Validation Error State

```
┌─ DevCtl - moments ──────────────────────────────────────────────────────────────┐
│                                                                                   │
│ ✗ System Status: Failed                                   Phase: Validate [4/5]  │
│ ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━ │
│                                                                                   │
│ Validation Errors (2)                                                             │
│ ┌─────────────────────────────────────────────────────────────────────────────┐ │
│ │ ✗ EPORT_IN_USE                                          (moments-config)    │ │
│ │   Port 8083 is already in use                                               │ │
│ │   Fix: Run 'devctl stop' or choose another port in config                   │ │
│ │                                                                              │ │
│ │ ✗ EENVMISSING                                           (moments-build)     │ │
│ │   Required environment variable not set: DATABASE_URL                       │ │
│ │   Fix: Add DATABASE_URL to backend/.env                                     │ │
│ └─────────────────────────────────────────────────────────────────────────────┘ │
│                                                                                   │
│ Actions                                                                           │
│  [r] retry   [f] fix manually   [l] view logs   [q] quit                         │
│                                                                                   │
└───────────────────────────────────────────────────────────────────────────────────┘
```

## Gap Analysis

### 1. Visual Styling (Major Gap)

| Aspect | Current | Target | Gap |
|--------|---------|--------|-----|
| Box borders | None | Unicode box-drawing (`┌─┐│└─┘`) | lipgloss `Border()` |
| Status icons | Text ("alive"/"dead") | Unicode icons (`✓`, `✗`, `●`, `▶`, `⚙`) | Icon constants |
| Color coding | None | Status-aware colors (green/red/yellow) | lipgloss `Foreground()` |
| Headers | Plain text | Styled title bars with keybind hints | lipgloss composition |
| Separators | None | Horizontal rules (`━━━`) | lipgloss styling |
| Selection highlight | `>` cursor | Background color + bold | lipgloss `Background()` |
| Log levels | Plain text | Color-coded (INFO=cyan, WARN=yellow, ERROR=red) | lipgloss conditional |

### 2. Layout Structure (Moderate Gap)

| Aspect | Current | Target | Gap |
|--------|---------|--------|-----|
| Dashboard sections | Flat text blocks | Bordered boxes (Services, Events, Plugins) | Widget refactoring |
| Status bar | Text line at top | Styled header with uptime | lipgloss fixed-height |
| Footer | Help text inline | Fixed footer with keybindings | lipgloss fixed-height |
| Responsive sizing | Basic height math | Proper flex layout | lipgloss `Width()`/`Height()` |
| Viewport integration | Uses `viewport.Model` | Same, but styled container | Wrapper component |

### 3. Widget Decomposition (Refactoring Needed)

Current widget responsibilities are correct but rendering is monolithic:

```
Current Structure:

RootModel
├── DashboardModel      → plain fmt.Sprintf rendering
├── ServiceModel        → viewport.Model but unstyled
├── PipelineModel       → plain text lists
└── EventLogModel       → viewport.Model but unstyled

Target Structure:

RootModel
├── HeaderWidget            → styled title bar + status indicator
├── DashboardModel
│   ├── ServicesTableWidget → bordered table with selection
│   ├── EventsWidget        → bordered log with icons
│   └── PluginsWidget       → bordered list
├── ServiceModel
│   ├── ProcessInfoWidget   → bordered key-value section
│   ├── LogViewportWidget   → bordered viewport with header
│   └── ExitInfoWidget      → conditional bordered section
├── PipelineModel
│   ├── PhaseListWidget     → phase icons + progress
│   ├── StepsListWidget     → step selection + details
│   └── ValidationWidget    → error/warning cards
├── EventLogModel
│   └── TimelineWidget      → timestamped entries with icons
└── FooterWidget            → keybindings bar
```

### 4. Model Data Structures (Minor Adjustments)

Current data structures are adequate. Minor additions needed:

| Type | Current Location | Change Needed |
|------|------------------|---------------|
| `ViewID` | `root_model.go` | Already correct |
| `LogStream` | `service_model.go` | Already correct |
| `PipelinePhase` | `pipeline_events.go` | Already correct |
| `pipelineFocus` | `pipeline_model.go` | Already correct |
| `EventLogEntry` | `domain.go` | Add `Level` field for icon selection |
| `ServiceStatus` | (missing) | Add enum for status icon/color mapping |
| `StyleConfig` | (missing) | Add centralized theme configuration |

## Proposed Architecture Refactoring

### New Package Structure

```
devctl/pkg/tui/
├── bus.go                    # Watermill bus (unchanged)
├── domain.go                 # Domain types (unchanged)
├── envelope.go               # Event envelopes (unchanged)
├── forward.go                # UI forwarder (unchanged)
├── msgs.go                   # Bubble Tea messages (unchanged)
├── pipeline_events.go        # Pipeline events (unchanged)
├── topics.go                 # Topic constants (unchanged)
├── transform.go              # Domain→UI transformer (unchanged)
│
├── styles/                   # NEW: Centralized styling
│   ├── theme.go              # Theme definition + colors
│   ├── icons.go              # Unicode icon constants
│   └── components.go         # Reusable styled builders
│
├── widgets/                  # NEW: Reusable UI widgets
│   ├── box.go                # Bordered box wrapper
│   ├── header.go             # Title bar with status
│   ├── footer.go             # Keybindings bar
│   ├── table.go              # Styled table with selection
│   ├── list.go               # Styled list with cursor
│   └── statusbar.go          # Fixed status indicator
│
└── models/                   # REFACTORED: Use widgets
    ├── root_model.go         # Compose header/footer/view
    ├── dashboard_model.go    # Use table/list widgets
    ├── service_model.go      # Use box/viewport widgets
    ├── pipeline_model.go     # Use list/box widgets
    └── eventlog_model.go     # Use timeline widget
```

### Proposed Type Signatures

#### `styles/theme.go`

```go
type Theme struct {
    Primary       lipgloss.Color
    Secondary     lipgloss.Color
    Success       lipgloss.Color
    Warning       lipgloss.Color
    Error         lipgloss.Color
    Muted         lipgloss.Color
    
    Border        lipgloss.Style
    Title         lipgloss.Style
    Selected      lipgloss.Style
    Keybind       lipgloss.Style
    KeybindKey    lipgloss.Style
}

func DefaultTheme() Theme
func (t Theme) WithWidth(w int) Theme
```

#### `styles/icons.go`

```go
const (
    IconSuccess   = "✓"
    IconError     = "✗"
    IconWarning   = "⚠"
    IconInfo      = "ℹ"
    IconRunning   = "▶"
    IconPending   = " "
    IconSkipped   = "⊘"
    IconSystem    = "●"
    IconGear      = "⚙"
    IconBullet    = "•"
)

func StatusIcon(status string) string
func PhaseIcon(phase PipelinePhase, ok *bool) string
func LogLevelIcon(level string) string
```

#### `widgets/box.go`

```go
type Box struct {
    Title       string
    TitleRight  string    // e.g., "[l] logs [r] restart"
    Content     string
    Width       int
    Height      int
    Style       lipgloss.Style
}

func NewBox(title string) Box
func (b Box) WithContent(s string) Box
func (b Box) WithTitleRight(s string) Box
func (b Box) WithSize(w, h int) Box
func (b Box) Render() string
```

#### `widgets/table.go`

```go
type TableColumn struct {
    Header string
    Width  int
    Align  lipgloss.Position
}

type TableRow struct {
    Cells    []string
    Icon     string
    Selected bool
    Style    lipgloss.Style
}

type Table struct {
    Columns  []TableColumn
    Rows     []TableRow
    Cursor   int
    Width    int
    Height   int
}

func NewTable(cols []TableColumn) Table
func (t Table) WithRows(rows []TableRow) Table
func (t Table) WithCursor(idx int) Table
func (t Table) WithSize(w, h int) Table
func (t Table) Render() string
```

#### `widgets/header.go`

```go
type Header struct {
    Title      string
    Status     string
    StatusIcon string
    Uptime     string
    Width      int
    Keybinds   []Keybind
}

type Keybind struct {
    Key   string
    Label string
}

func NewHeader(title string) Header
func (h Header) WithStatus(icon, status string) Header
func (h Header) WithUptime(d time.Duration) Header
func (h Header) WithKeybinds(kb []Keybind) Header
func (h Header) WithWidth(w int) Header
func (h Header) Render() string
```

#### `widgets/footer.go`

```go
type Footer struct {
    Keybinds []Keybind
    Width    int
}

func NewFooter(keybinds []Keybind) Footer
func (f Footer) WithWidth(w int) Footer
func (f Footer) Render() string
```

### Model View() Signature Changes

The existing `View() string` signatures remain, but internal rendering changes:

```go
// Before (dashboard_model.go)
func (m DashboardModel) View() string {
    var b strings.Builder
    b.WriteString(fmt.Sprintf("Services (%d):\n", len(services)))
    for i, svc := range services {
        cursor := " "
        if i == m.selected { cursor = ">" }
        b.WriteString(fmt.Sprintf("%s %s pid=%d %s\n", cursor, svc.Name, svc.PID, status))
    }
    return b.String()
}

// After (dashboard_model.go)
func (m DashboardModel) View() string {
    // Build services table
    rows := make([]widgets.TableRow, len(services))
    for i, svc := range services {
        rows[i] = widgets.TableRow{
            Icon:     styles.StatusIcon(status),
            Cells:    []string{svc.Name, statusText, pidText, healthText},
            Selected: i == m.selected,
        }
    }
    servicesBox := widgets.NewBox("Services").
        WithTitleRight("[l] logs [r] restart").
        WithContent(widgets.NewTable(serviceColumns).WithRows(rows).Render()).
        WithSize(m.width, serviceBoxHeight)
    
    // Similar for events and plugins boxes...
    
    return lipgloss.JoinVertical(lipgloss.Left,
        servicesBox.Render(),
        eventsBox.Render(),
        pluginsBox.Render(),
    )
}
```

## Implementation Roadmap

### Phase 1: Styles Foundation ✅
- [x] Create `styles/theme.go` with `DefaultTheme()`
- [x] Create `styles/icons.go` with status/phase/log icons
- [ ] Create `styles/components.go` with lipgloss style builders (deferred - not needed for initial implementation)

### Phase 2: Widget Library ✅
- [x] Implement `widgets/box.go` (bordered container)
- [x] Implement `widgets/header.go` (title bar)
- [x] Implement `widgets/footer.go` (keybindings)
- [x] Implement `widgets/table.go` (services table)
- [ ] Implement `widgets/list.go` (generic selectable list) (deferred)

### Phase 3: Dashboard Refactoring ✅
- [x] Refactor `DashboardModel.View()` to use widgets
- [x] Add services table with icons and selection styling
- [ ] Add events preview box (last 5 events) (optional enhancement)
- [ ] Add plugins summary box (optional enhancement)

### Phase 4: Service View Refactoring ✅
- [x] Add process info section with bordered box
- [x] Style log viewport with header
- [x] Add exit info section styling (renderStyledExitInfo)
- [x] Stream selector with styled tabs (stdout/stderr)
- [x] Follow indicator with icon

### Phase 5: Pipeline View Refactoring ✅
- [x] Style phase list with icons (✓/▶/○/✗)
- [x] Style step lists with selection
- [x] Style validation errors/warnings
- [x] Added helper methods: phaseIconAndStyle, renderStyledSteps, renderStyledValidation

### Phase 6: Event Log Refactoring ✅
- [x] Add icons for event types (content-based detection)
- [x] Style timestamps
- [x] Add level-based coloring (error=red, success=green, warning=yellow)

## Key Files Modified ✅

| File | Changes |
|------|---------|
| `models/root_model.go` | ✅ Added header/footer rendering, compose child views |
| `models/dashboard_model.go` | ✅ Using table widget for services, box widget for sections |
| `models/service_model.go` | ✅ Using box widget for process info, styled log viewport, exit info |
| `models/pipeline_model.go` | ✅ Using icons for phases/steps, styled validation |
| `models/eventlog_model.go` | ✅ Added icons based on content, styled timestamps |

## Key Symbols Referenced

### Current Implementation

- `models.RootModel` - Root coordinator
- `models.RootModel.View()` - Main render function
- `models.RootModel.Update()` - Message router
- `models.DashboardModel` - Dashboard state
- `models.DashboardModel.WithSnapshot()` - State update
- `models.ServiceModel` - Service detail state
- `models.ServiceModel.lookupService()` - Service record lookup
- `models.PipelineModel` - Pipeline progress state
- `models.PipelineModel.phase()` - Phase state accessor
- `models.EventLogModel` - Event timeline state
- `tui.StateSnapshot` - State data structure
- `tui.PipelinePhase` - Phase enum
- `tui.EventLogEntry` - Event data structure

### lipgloss API (to use)

- `lipgloss.NewStyle()` - Create style
- `lipgloss.Style.Border()` - Add border
- `lipgloss.Style.BorderStyle()` - Border type (rounded, normal, thick)
- `lipgloss.Style.Foreground()` - Text color
- `lipgloss.Style.Background()` - Background color
- `lipgloss.Style.Bold()` - Bold text
- `lipgloss.Style.Width()` - Fixed width
- `lipgloss.Style.Height()` - Fixed height
- `lipgloss.Style.Padding()` - Internal padding
- `lipgloss.Style.Margin()` - External margin
- `lipgloss.Style.Align()` - Text alignment
- `lipgloss.JoinHorizontal()` - Horizontal composition
- `lipgloss.JoinVertical()` - Vertical composition
- `lipgloss.Place()` - Positioned placement

### bubbles Components (already used)

- `viewport.Model` - Scrollable content
- `textinput.Model` - Filter input
- `viewport.New()` - Create viewport
- `textinput.New()` - Create text input

## Testing Considerations

The playbook at `devctl/ttmp/2026/01/06/MO-006-DEVCTL-TUI--create-a-devctl-tui/playbook/01-playbook-testing-devctl-tui-in-tmux.md` provides the testing workflow. Key adjustments for visual testing:

1. **tmux capture may strip colors** - Use `tmux capture-pane -e` for escape sequences
2. **Screenshot comparison** - Consider adding golden file tests for rendered output
3. **Responsive testing** - Test at different terminal sizes (80x24, 120x40, etc.)

## References

- MO-006 ASCII baseline: `devctl/ttmp/2026/01/06/MO-006-DEVCTL-TUI--create-a-devctl-tui/sources/local/01-devctl-tui-layout.md`
- MO-006 implementation design: `devctl/ttmp/2026/01/06/MO-006-DEVCTL-TUI--create-a-devctl-tui/design-doc/01-devctl-tui-layout-and-implementation-design.md`
- MO-006 architecture: `devctl/ttmp/2026/01/06/MO-006-DEVCTL-TUI--create-a-devctl-tui/working-note/01-devctl-tui-code-mapping-and-integration-analysis.md`
- lipgloss documentation: https://github.com/charmbracelet/lipgloss
- bubbles documentation: https://github.com/charmbracelet/bubbles
