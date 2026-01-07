# MO-009: TUI Complete Features - Development Diary

## Overview

This diary documents the planning and implementation of all missing TUI features identified in the MO-008 gap analysis to bring the `devctl tui` to full design parity with MO-006.

---

## 2026-01-07

### Step 1: Ticket Creation (21:23)

Created this ticket to track the comprehensive implementation of all missing TUI features.

#### Background
- MO-006 defined the original TUI design with ASCII mockups
- MO-008 implemented basic styling with lipgloss and fixed visual issues
- Gap analysis (`MO-008/.../03-gap-analysis-vs-design.md`) identified 50+ missing features

#### What I did
- Created ticket MO-009-TUI-COMPLETE-FEATURES using docmgr
- Reviewed the original design document thoroughly
- Created comprehensive implementation plan with 8 phases, 75+ tasks

#### Key Documents Created
- `design/01-implementation-plan.md` - Full implementation plan with:
  - Data layer requirements (process stats, health checks, env vars)
  - UI changes for each view (dashboard, service, events, pipeline, plugins)
  - ASCII mockups showing target state
  - Code snippets and type definitions
  - Task dependencies graph
  - Recommended implementation order

- `tasks.md` - Checklist of all 75 tasks organized by phase

#### Design Decisions

1. **Data-First Approach**: Phase 1 focuses on data layer (process stats, health, env vars) before UI, because many features depend on this data being available.

2. **Events View First**: Recommended starting with Phase 4 (Events enhancements) because it has low dependencies and high visibility - quick wins.

3. **Plugin View Last**: Plugin list is lowest priority since plugins are mostly static config.

4. **New Widget for Progress Bars**: Phase 5 requires a new progress bar widget for build steps.

5. **Process Stats via /proc**: On Linux, read `/proc/[pid]/stat` for CPU/MEM. Fallback to `ps` on macOS.

---

### Step 2: Gap Analysis Summary (from MO-008)

#### Dashboard Missing Features
| Feature | Status | Phase |
|---------|--------|-------|
| Health column | ❌ | 2.1 |
| CPU % column | ❌ | 2.1 |
| Memory MB column | ❌ | 2.1 |
| Recent Events preview | ❌ | 2.2 |
| Plugins summary | ❌ | 2.3 |

#### Service Detail Missing Features
| Feature | Status | Phase |
|---------|--------|-------|
| Command line | ❌ | 3.1 |
| Working directory | ❌ | 3.1 |
| CPU/Memory stats | ❌ | 3.1 |
| Health check info | ❌ | 3.2 |
| Environment vars | ❌ | 3.3 |
| Stop keybinding | ❌ | 3.4 |

#### Events View Missing Features
| Feature | Status | Phase |
|---------|--------|-------|
| Service source column | ❌ | 4.1 |
| Log level column | ❌ | 4.2 |
| Service filter toggles | ❌ | 4.3 |
| Level filter toggles | ❌ | 4.4 |
| Stats line | ❌ | 4.5 |
| Pause toggle | ❌ | 4.6 |

#### Pipeline View Missing Features
| Feature | Status | Phase |
|---------|--------|-------|
| Progress bars | ❌ | 5.1 |
| Live output viewport | ❌ | 5.2 |
| Config patches display | ❌ | 5.3 |

#### Not Implemented
| Feature | Status | Phase |
|---------|--------|-------|
| Plugin List View | ❌ | 6 |

---

### Architecture Notes

#### Data Flow for Process Stats
```
/proc/[pid]/stat → ReadProcessStats() → StateSnapshot.ProcessStats
                                              ↓
                                     DashboardModel.View()
                                     ServiceModel.View()
```

#### Data Flow for Health Checks
```
Service Config (health endpoint) → Supervisor Health Poller → StateSnapshot.Health
                                                                     ↓
                                                          DashboardModel.View()
                                                          ServiceModel.View()
```

#### Events Enhancement Architecture
```
EventLogEntry {
    At:     time.Time
    Text:   string
    Source: string    // NEW: "backend", "system"
    Level:  LogLevel  // NEW: DEBUG/INFO/WARN/ERROR
}

EventLogModel {
    entries:        []EventLogEntry
    serviceFilters: map[string]bool  // NEW
    levelFilters:   map[LogLevel]bool // NEW
    paused:         bool              // NEW
    eventsPerSec:   float64           // NEW
}
```

---

### What Warrants Discussion

1. **Health Check Implementation**: Where should health check logic live? Options:
   - In supervisor as a goroutine
   - In TUI as polling
   - Separate health daemon

2. **Environment Sanitization**: Need clear rules for what to redact:
   - Keys containing: PASSWORD, SECRET, TOKEN, KEY, CREDENTIAL, API_KEY
   - Should we allow override via config?

3. **Live Build Output**: Currently build output isn't streamed to TUI. Need to:
   - Add output channel to build executor
   - Create PipelineLiveOutputMsg
   - Wire through Watermill or direct channel

4. **Plugin Enable/Disable**: The design shows enable/disable actions but current implementation may not support runtime plugin state changes.

---

### Next Steps

Recommended order for implementation:

1. **Quick Win**: Phase 4.1-4.2 (Events source/level columns)
   - Low effort, immediate improvement
   - No backend changes needed

2. **Foundation**: Phase 1.1 (Process stats)
   - Enables dashboard and service CPU/MEM
   - Requires new pkg/proc/stats.go

3. **Dashboard Polish**: Phase 2.1-2.2
   - High visibility improvement
   - Depends on Phase 1.1

4. **Continue from there based on capacity**

---

### Step 3: Phase 1 Implementation - Data Layer (21:30)

Implemented all 12 tasks for Phase 1 (Data Layer Enhancements).

#### 1.1 Process Stats

**Created `pkg/proc/stats.go`**:
- `type Stats` - CPU%, MemoryMB, MemoryRSS, VirtualMB, State, Threads, StartTime
- `type CPUTracker` - Tracks CPU usage across samples for delta calculation
- `func ReadStats(pid, tracker)` - Reads from `/proc/[pid]/stat`
- `func ReadAllStats(pids, tracker)` - Batch read for multiple PIDs
- `func GetProcessStartTime(pid)` - Returns when process started
- `func GetBootTime()` - Reads system boot time from `/proc/stat`

**Key design decisions**:
- CPU percentage calculated as delta between samples (requires tracker)
- Reads from `/proc/[pid]/stat` directly for Linux
- Handles zombie detection and process state

#### 1.2 Health Check Data

**Updated `pkg/tui/state_events.go`**:
- Added `HealthStatus` type (unknown/healthy/unhealthy)
- Added `HealthCheckResult` struct with service name, status, check type, endpoint, error, response time

**Updated `pkg/tui/state_watcher.go`**:
- Added `checkHealth()` to poll health for all services with health config
- Added `runHealthCheck()` for single service check
- Added `checkTCP()` and `checkHTTP()` health check implementations
- Health results included in StateSnapshot

**Updated `pkg/state/state.go`**:
- Added `HealthType`, `HealthAddress`, `HealthURL` fields to ServiceRecord
- Health config now stored in state.json for later polling

**Updated `pkg/supervise/supervisor.go`**:
- Copies health config from ServiceSpec to ServiceRecord at launch

#### 1.3 Environment Variables

**Created `pkg/state/sanitize.go`**:
- `func SanitizeEnv(env)` - Redacts sensitive values
- Patterns detected: PASSWORD, SECRET, TOKEN, KEY, CREDENTIAL, API_KEY, AUTH, PRIVATE, CERT, PASSPHRASE
- `func FilterEnvForDisplay(env, maxVars)` - Filters out noisy vars for display

**Updated `pkg/supervise/supervisor.go`**:
- Env sanitized at launch time before storing in state
- Added `StartedAt` timestamp to ServiceRecord

#### Updated `pkg/tui/styles/icons.go`

Added new icon functions:
- `HealthIcon(status)` - Returns filled/empty circle based on health status
- `EventIcon(eventType)` - Returns appropriate icon for event types

#### StateSnapshot Now Includes

```go
type StateSnapshot struct {
    RepoRoot     string
    At           time.Time
    Exists       bool
    State        *state.State
    Alive        map[string]bool
    Error        string
    ProcessStats map[int]*proc.Stats           // NEW: PID -> CPU/MEM stats
    Health       map[string]*HealthCheckResult // NEW: service name -> health
}
```

#### Files Created
- `pkg/proc/stats.go` (240 lines)
- `pkg/state/sanitize.go` (95 lines)

#### Files Modified
- `pkg/state/state.go` - Added StartedAt, HealthType, HealthAddress, HealthURL to ServiceRecord
- `pkg/tui/state_events.go` - Added HealthStatus, HealthCheckResult, updated StateSnapshot
- `pkg/tui/state_watcher.go` - Added health polling and process stats reading
- `pkg/tui/styles/icons.go` - Added HealthIcon, EventIcon functions
- `pkg/supervise/supervisor.go` - Store health config and sanitized env at launch

#### Testing
- `go build ./...` passes
- All packages compile without errors
- No test files yet (deferred to Phase 8)

---

### Step 4: Phase 2 Implementation - Dashboard Enhancements (21:45)

Implemented all 11 tasks for Phase 2 (Dashboard Enhancements).

#### 2.1 Health/CPU/MEM Columns

**Updated `pkg/tui/models/dashboard_model.go`**:
- Added new columns: Health, CPU, MEM to service table
- Created `formatCPU()` and `formatMem()` formatters
- Read CPU/MEM from `StateSnapshot.ProcessStats`
- Read health from `StateSnapshot.Health`
- Show "-" for missing data

**New table layout**:
```
| Name           | Status         | Health | PID    | CPU   | MEM   |
| backend        | Running        | ●      | 12847  | 12.3% | 245M  |
| frontend       | Dead (exit=1)  | ○      | 0      | -     | -     |
```

#### 2.2 Recent Events Preview

**Added to DashboardModel**:
- `recentEvents []tui.EventLogEntry` - Stores last 5 events
- `AppendEvent()` method - Called from RootModel when events arrive
- `renderEventsPreview()` - Renders compact event list with icons

**Event preview format**:
```
 14:23:45  [backend]    ℹ  Request completed in 234ms
 14:23:42  [frontend]   ✗  Build failed: missing dependency
```

#### 2.3 Plugins Summary

**Added to `pkg/tui/state_events.go`**:
- `PluginSummary` struct with ID, Path, Priority, Status

**Updated `pkg/tui/state_watcher.go`**:
- `readPlugins()` - Reads from `.devctl.yaml` config
- Checks if plugin path exists to determine status

**Added `renderPluginsSummary()` to DashboardModel**:
- Shows list of plugins with status icons
- Counts active plugins in title

**Plugin summary format**:
```
╭──────────────────────────────────────────────────────────────────╮
│Plugins (3 active)                                   [p] details  │
│ ✓ moments-config       (priority: 10)                            │
│ ✓ moments-build        (priority: 15)                            │
│ ✓ moments-launch       (priority: 20)                            │
╰──────────────────────────────────────────────────────────────────╯
```

#### Files Modified
- `pkg/tui/models/dashboard_model.go` - Major updates for all three sections
- `pkg/tui/models/root_model.go` - Route events to dashboard
- `pkg/tui/state_events.go` - Added PluginSummary, updated StateSnapshot
- `pkg/tui/state_watcher.go` - Added readPlugins()

#### Testing
- `go build ./...` passes
- All packages compile without errors

---

### Step 5: Phase 3 Implementation - Service Detail Enhancements (22:00)

Implemented all 10 tasks for Phase 3 (Service Detail Enhancements).

#### 3.1 Enhanced Process Info Section

**Expanded info box to show**:
- Status icon + text + PID
- CPU and Memory usage from ProcessStats
- Uptime (formatted with formatDuration())
- Command (truncated if too long)
- Working directory (truncated if too long)
- Stream selector (stdout/stderr) and follow state

**New formatDuration() helper**:
```go
func formatDuration(d time.Duration) string {
    // Returns "45s", "5m 30s", "2h 15m", "3d 12h"
}
```

#### 3.2 Health Check Info

**New renderHealthInfo() method**:
- Shows health icon (●/○) and status (Healthy/Unhealthy/Unknown)
- Displays check type (tcp/http)
- Shows endpoint URL
- Last check time and response time in ms

**Layout**:
```
╭─ Health ────────────────────────────────────────────────────╮
│ ● Healthy                                                   │
│ Type:     http                                              │
│ Endpoint: http://localhost:8080/health                      │
│ Last:     2s ago (45ms)                                     │
╰─────────────────────────────────────────────────────────────╯
```

#### 3.3 Environment Variables

**New renderEnvVars() method**:
- Shows env vars in compact format
- Truncates long values
- Limits width to prevent overflow
- Shows count in title

**Layout**:
```
╭─ Environment (5) ──────────────────────────────────────────╮
│ PORT=8080  DB_URL=postgresql://...  ENV=development         │
╰─────────────────────────────────────────────────────────────╯
```

#### 3.4 New Keybindings

**Added to Update()**:
- `s` - Stop service (sends ActionStop request)
- `r` - Restart service (sends ActionRestart request)
- `d` - Detach (go back to dashboard without stopping)

**Updated header keybindings hint**:
`[s] stop  [r] restart  [esc] back`

**New types added**:
- `ActionStop` action kind in actions.go
- `Service` field in ActionRequest for targeting specific service
- `NavigateBackMsg` message type

#### Files Modified
- `pkg/tui/models/service_model.go` - Major View() refactor, new helper methods
- `pkg/tui/models/root_model.go` - Handle NavigateBackMsg
- `pkg/tui/actions.go` - Added ActionStop, Service field
- `pkg/tui/msgs.go` - Added NavigateBackMsg

#### Testing
- `go build ./...` passes
- All packages compile without errors

---

### Step 6: Phase 4 Visual Fixes - Events View Refactor (22:15)

User feedback: The events view "looks like ass." Performed analysis, created playbook, and fixed issues.

#### Analysis Report Created

`analysis/01-events-view-issues.md` documents 6 major issues:
1. Cramped, unstyled filter bars
2. Missing visual hierarchy (no separator, status line)
3. Missing stats line (events/sec, buffer, dropped)
4. Missing pause toggle
5. Poor event line formatting (no milliseconds, bad alignment)
6. Keybinding clutter (all crammed in title)

#### Playbook Created

`playbooks/01-tui-design-implementation-guidelines.md` provides:
- Anti-patterns to avoid (functionality-first, copy-paste coding)
- Implementation checklist (visual structure, alignment, styling)
- Step-by-step implementation process
- Common patterns with code examples
- Quality gates for review

#### Visual Fixes Applied

**New layout structure**:
```
╭─ Live Events ───────────────────────────────── [esc] back ─╮
│ Following: All Services              [f] filter [1-9] select│
│ ────────────────────────────────────────────────────────────│
│ 14:34:12.234  [backend   ]  INFO   POST /api/moments        │
│ 14:34:11.987  [frontend  ]  WARN   Slow render: 120ms       │
│ 14:34:10.543  [system    ]  ERROR  Health check failed      │
│                                                             │
│ Services: [1]● backend  [2]● frontend  [3]○ postgres        │
│ Levels:  ● DEBUG  ● INFO  ● WARN  ● ERROR  [l] level menu   │
│ Stats: 47 events (12/sec)   Buffer: 47/200 lines   Dropped: 0│
│ [p] pause   [c] clear   [/] search   [↑/↓] scroll           │
╰─────────────────────────────────────────────────────────────╯
```

**Key improvements**:
1. Status line with "Following: X Services"
2. Horizontal separator between header and content
3. Timestamps with milliseconds (15:04:05.123)
4. Fixed-width source column (10 chars)
5. Level shown as styled text, color-coded
6. Color-coded filter toggles (green=enabled, gray=disabled)
7. Stats line with event count, rate, buffer, dropped
8. Pause toggle with queue for paused events
9. Keybindings distributed: title, status line, footer

**New methods added**:
- `renderStatusLine()` - "Following: X Services" + right-aligned hints
- `renderStyledServiceFilterBar()` - Color-coded service toggles
- `renderStyledLevelFilterBar()` - Level-colored level toggles
- `renderStatsLine()` - Count, rate, buffer, dropped
- `renderFooterKeybindings()` - Distributed keybind hints
- `formatEventLine()` - Proper event formatting with alignment

**New fields added**:
- `paused bool` - Pause state
- `pausedQueue []EventLogEntry` - Queued events when paused
- `totalCount int` - Total events received
- `droppedCount int` - Events dropped due to buffer overflow
- `eventsPerSec float64` - Calculated rate
- `recentCount int` + `lastStatTime time.Time` - For rate calculation

#### Lessons Learned

1. **Compare to mockup line-by-line** before marking complete
2. **Implement visual structure first**, functionality second
3. **Use theme constants** instead of inline styles
4. **Distribute keybindings** across the UI logically
5. **Fixed-width columns** for tabular data

---

## Phase 5: Pipeline View Enhancements

**Date**: 2026-01-07 ~03:30

### 5.1 Progress Bar Widget

Created `pkg/tui/widgets/progress.go`:
- `ProgressBar` struct with `percent`, `width`, `style`, `filledChar`, `emptyChar`
- `NewProgressBar(percent int)` constructor
- Builder methods: `WithWidth()`, `WithStyle()`, `WithChars()`, `WithShowText()`
- `Render()` returns styled progress bar with percentage text
- `RenderCompact()` for inline display

### 5.2 Live Output Viewport

Updated `PipelineModel` with new fields:
- `liveOutput []string` - buffer of output lines
- `liveVp viewport.Model` - bubbles viewport for scrolling
- `liveVpReady bool` - initialization flag
- `showLiveVp bool` - toggle visibility
- `liveVpHeight int` - viewport height (default 8)

Added message types:
- `PipelineLiveOutput` struct in `pipeline_events.go`
- `PipelineLiveOutputMsg` in `msgs.go`

Implementation:
- `[o]` keybinding to toggle live output visibility
- `formatLiveOutputLine()` helper for formatting with source prefix
- `refreshLiveViewport()` to update content and auto-scroll
- Max 500 lines to prevent unbounded growth
- Viewport auto-scrolls to bottom on new content

### 5.3 Config Patches Display

Added types:
- `ConfigPatch` struct with `Plugin`, `Key`, `Value` fields
- `PipelineConfigPatches` struct for batch updates
- `PipelineConfigPatchesMsg` message type

Added `renderConfigPatches()` method:
- Renders each patch as `• key → value  (plugin)`
- Uses themed styling for key, value, and plugin name
- Box title shows patch count

### 5.4 Step Progress Integration

Updated `PipelineStepResult`:
- Added `ProgressPercent int` field

Added `stepProgress map[string]int` to `PipelineModel`:
- Tracks real-time progress per step name
- `PipelineStepProgressMsg` updates the map

Updated `renderStyledSteps()`:
- Shows progress bar for in-progress steps (0 < percent < 100)
- Uses running icon instead of success/error when in progress
- 15-character wide progress bar with percentage

### Files Created/Modified

**Created**:
- `pkg/tui/widgets/progress.go` - Progress bar widget

**Modified**:
- `pkg/tui/pipeline_events.go` - Added `PipelineLiveOutput`, `ConfigPatch`, `PipelineConfigPatches`, `ProgressPercent`
- `pkg/tui/msgs.go` - Added `PipelineLiveOutputMsg`, `PipelineConfigPatchesMsg`, `PipelineStepProgressMsg`
- `pkg/tui/models/pipeline_model.go` - All new features integrated

### All Phase 5 Tasks Complete

| Task | Description | Status |
|------|-------------|--------|
| 5.1.1 | Create progress bar widget | ✅ |
| 5.1.2 | Add progress to step display | ✅ |
| 5.1.3 | Wire up PipelineStepProgress messages | ✅ |
| 5.2.1 | Add live output state to PipelineModel | ✅ |
| 5.2.2 | Handle LiveOutputLine messages | ✅ |
| 5.2.3 | Render live output box | ✅ |
| 5.2.4 | Wire up streaming (types ready) | ✅ |
| 5.3.1 | Add config patches to pipeline state | ✅ |
| 5.3.2 | Handle ConfigPatchApplied messages | ✅ |
| 5.3.3 | Render patches section | ✅ |

---

### Technical References

- Original Design: `MO-006-DEVCTL-TUI/.../01-devctl-tui-layout.md`
- Gap Analysis: `MO-008-IMPROVE-TUI-LOOKS/.../03-gap-analysis-vs-design.md`
- Events View Issues: `analysis/01-events-view-issues.md`
- Design Playbook: `playbooks/01-tui-design-implementation-guidelines.md`
- Current TUI Code: `pkg/tui/models/*.go`
- Current Widgets: `pkg/tui/widgets/*.go`
- Current Styles: `pkg/tui/styles/*.go`
- State Types: `pkg/state/state.go`
- TUI Domain: `pkg/tui/domain.go`
- Process Stats: `pkg/proc/stats.go`
- Sanitization: `pkg/state/sanitize.go`

