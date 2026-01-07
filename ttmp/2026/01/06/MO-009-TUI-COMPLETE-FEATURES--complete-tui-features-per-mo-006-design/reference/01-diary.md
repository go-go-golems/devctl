---
Title: Diary
Ticket: MO-009-TUI-COMPLETE-FEATURES
Status: active
Topics:
    - backend
    - ui-components
DocType: reference
Intent: long-term
Owners: []
RelatedFiles:
    - Path: devctl/pkg/tui/domain.go
      Note: Define LogLevel and extend EventLogEntry
    - Path: devctl/pkg/tui/models/dashboard_model.go
      Note: Set Source/Level for kill/SIGTERM events
    - Path: devctl/pkg/tui/models/eventlog_model.go
      Note: Render [source] prefix and log-level icon
    - Path: devctl/pkg/tui/models/root_model.go
      Note: Set Source/Level for action-related events
    - Path: devctl/pkg/tui/transform.go
      Note: Populate Source/Level when transforming domain events
ExternalSources: []
Summary: 'Implementation diary for MO-009: complete devctl TUI features'
LastUpdated: 2026-01-07T02:43:04-05:00
WhatFor: Track implementation steps, decisions, and validation for MO-009
WhenToUse: When implementing or reviewing MO-009 changes
---


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

### Technical References

- Original Design: `MO-006-DEVCTL-TUI/.../01-devctl-tui-layout.md`
- Gap Analysis: `MO-008-IMPROVE-TUI-LOOKS/.../03-gap-analysis-vs-design.md`
- Current TUI Code: `pkg/tui/models/*.go`
- Current Widgets: `pkg/tui/widgets/*.go`
- Current Styles: `pkg/tui/styles/*.go`
- State Types: `pkg/state/types.go`
- TUI Domain: `pkg/tui/domain.go`

---

## Step 3: Start implementing Events source/level metadata

Picked up MO-009 by starting with Phase 4 “Events View Enhancements”, specifically adding structured `source` and `level` metadata to `EventLogEntry` and rendering it in the Events view. This is intended to replace the current heuristic “scan the text for keywords” styling and make it possible to add proper filters (Phase 4.3/4.4) without guessing.

No code changes in this step yet; this entry captures the initial orientation and intended implementation approach before editing.

**Commit (code):** N/A

### What I did
- Read the ticket implementation plan and task list.
- Located the current Events implementation in `devctl/pkg/tui/domain.go`, `devctl/pkg/tui/models/eventlog_model.go`, and the domain→UI transformer in `devctl/pkg/tui/transform.go`.

### Why
- Phase 4.1/4.2 are low-dependency, high-visibility improvements and set up Phase 4.3+ filters.

### What worked
- Confirmed Events are currently styled via keyword scanning in `EventLogModel.refreshViewportContent`, and `EventLogEntry` currently has only `{At, Text}`.

### What didn't work
- N/A

### What I learned
- `styles.LogLevelIcon()` already exists, but no typed log level exists in `tui`, and the Events UI isn’t using the icon consistently because it derives level from text.

### What was tricky to build
- N/A

### What warrants a second pair of eyes
- N/A

### What should be done in the future
- N/A

### Code review instructions
- Start at `devctl/pkg/tui/domain.go` and `devctl/pkg/tui/models/eventlog_model.go`.
- Validate by running `go test ./...` in `devctl/` after implementation.

### Technical details
- Target tasks: 4.1.1, 4.1.2, 4.2.1, 4.2.3.

---

## Step 4: Implement structured events (source + level) and render it

Implemented structured event metadata for the Events view by extending `EventLogEntry` with `source` and `level`, populating those fields in the domain→UI transformer and UI-generated events, and rendering them as a `[source]` prefix with a log-level icon. This makes event styling deterministic and sets up Phase 4.3/4.4 filtering without needing to infer semantics from free-form text.

This change also removes the previous “scan for keywords” logic from the Events renderer in favor of level-driven rendering, while keeping a conservative default (`INFO`) if an entry arrives without an explicit level.

**Commit (code):** 060bd82217e1e05cc46c42d3ff023adb34b12175 — "tui: add event source and level metadata"

### What I did
- Added `tui.LogLevel` and `EventLogEntry.Source`/`EventLogEntry.Level`.
- Populated source/level in `devctl/pkg/tui/transform.go` for key domain events (state snapshot, service exit, pipeline).
- Updated UI-generated events (kill confirmation + action publish statuses) to set `source`/`level`.
- Updated `EventLogModel` rendering to show `[source]` and use `styles.LogLevelIcon(level)` rather than text keyword scanning.
- Ran `gofmt -w ...` and `go test ./... -count=1` from `devctl/`.

### Why
- Enable Phase 4.1/4.2 parity with MO-006/MO-008 expectations for the Events view.
- Provide a stable foundation for upcoming filter toggles (Phase 4.3/4.4).

### What worked
- `go test ./...` passed after the refactor.
- Rendering is now consistent: level→icon/style, plus source prefix.

### What didn't work
- N/A

### What I learned
- The existing `styles.LogLevelIcon()` was already available and could be reused directly once the domain had a typed `LogLevel`.

### What was tricky to build
- Ensuring all event creation paths set the new fields so the renderer doesn’t regress to “guessing” in normal operation.

### What warrants a second pair of eyes
- Whether `service exit` should be rendered as `WARN` vs `ERROR` (currently `WARN`) and whether the default source labels (`system`, `ui`, `pipeline`) match the desired UX vocabulary.

### What should be done in the future
- Implement Phase 4.3/4.4 filters using the new structured fields (service/source and log level).

### Code review instructions
- Start at `devctl/pkg/tui/domain.go` (new `LogLevel`, updated `EventLogEntry`).
- Follow through `devctl/pkg/tui/transform.go` (how source/level are assigned).
- Review `devctl/pkg/tui/models/eventlog_model.go` for rendering behavior.
- Validate with `cd devctl && go test ./... -count=1`, then manually run `cd devctl && go run ./cmd/devctl tui` and generate a few actions/events.

### Technical details
- Implemented tasks: 4.1.1, 4.1.2, 4.2.1, 4.2.3 (and effectively 4.2.2 by using the existing `styles.LogLevelIcon()`).
