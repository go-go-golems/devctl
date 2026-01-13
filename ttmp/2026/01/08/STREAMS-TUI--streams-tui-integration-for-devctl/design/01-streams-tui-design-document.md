---
Title: Streams TUI - Design Document
Ticket: STREAMS-TUI
Status: active
Topics:
    - devctl
    - tui
    - streams
DocType: design
Intent: long-term
Owners: []
RelatedFiles:
    - Path: pkg/tui/stream_runner.go
      Note: Stream lifecycle management (needs context fix)
    - Path: pkg/tui/models/streams_model.go
      Note: Current streams view implementation
    - Path: pkg/tui/models/plugin_model.go
      Note: Plugin view (add stream capability display)
    - Path: pkg/tui/models/dashboard_model.go
      Note: Dashboard (add streams widget)
ExternalSources: []
Summary: Design for improving the Streams TUI with better UX, discoverability, and information display.
LastUpdated: 2026-01-08
WhatFor: Guide implementation of Streams TUI improvements.
WhenToUse: When implementing Streams TUI features.
---

# Streams TUI - Design Document

## Goals

1. **Fix critical bug**: Context cancellation prevents any stream usage
2. **Improve discoverability**: Make streams feature visible and accessible
3. **Simplify stream creation**: Reduce friction to start a stream
4. **Enhance information display**: Show more useful stream metadata

## Non-Goals

- Persist streams across TUI restarts
- Add protocol-level stream cancellation (separate effort)
- Replace service log tailing with streams (future consideration)

## Current State

### Navigation
```
Dashboard → Events → Pipeline → Plugins → Streams → (back to Dashboard)
    [1]       [2]       [3]        [4]        [5]
```

### Current Streams View
```
┌──────────────────────────────────────────────────────────────────────────────┐
│ DevCtl — streams   ○ Stopped              [tab] switch [?] help [q] quit     │
├──────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│ ╭────────────────────────────────────────────────────────────────────────╮   │
│ │Streams                                                                 │   │
│ │No active streams.                                                      │   │
│ │                                                                        │   │
│ │Press [n] to start a new stream.                                        │   │
│ ╰────────────────────────────────────────────────────────────────────────╯   │
│                                                                              │
├──────────────────────────────────────────────────────────────────────────────┤
│                   [n] new [j/k] select [↑/↓] scroll [x] stop                 │
└──────────────────────────────────────────────────────────────────────────────┘
```

### Problems
1. Empty state doesn't guide users
2. [n] requires JSON knowledge
3. No indication of available stream operations
4. Stream list shows minimal information

## Proposed Design

### Phase 1: Critical Bug Fix

Fix context cancellation in `stream_runner.go`:

```go
// Line 181: Change from
streamCtx, cancel := context.WithCancel(ctx)
// To
streamCtx, cancel := context.WithCancel(context.Background())
```

### Phase 2: Enhanced Streams View

#### Empty State (No Active Streams)

```
┌──────────────────────────────────────────────────────────────────────────────┐
│ DevCtl — streams   ○ Stopped              [tab] switch [?] help [q] quit     │
├──────────────────────────────────────────────────────────────────────────────┤
│ 0 Streams  [n] new  [j/k] select  [↑/↓] scroll  [x] stop  [esc] back         │
│                                                                              │
│ ╭─ No Active Streams ────────────────────────────────────────────────────╮   │
│ │                                                                        │   │
│ │  No streams are currently running.                                     │   │
│ │                                                                        │   │
│ │  Available stream operations:                                          │   │
│ │    • telemetry.stream (plugin: telemetry)                              │   │
│ │                                                                        │   │
│ │  Press [n] to start a new stream                                       │   │
│ │  Press [q] to start a quick stream picker                              │   │
│ │                                                                        │   │
│ ╰────────────────────────────────────────────────────────────────────────╯   │
│                                                                              │
├──────────────────────────────────────────────────────────────────────────────┤
│            [n] new (JSON) [q] quick-start [j/k] select [x] stop              │
└──────────────────────────────────────────────────────────────────────────────┘
```

#### Active Streams with Events

```
┌──────────────────────────────────────────────────────────────────────────────┐
│ DevCtl — streams   ● Running              [tab] switch [?] help [q] quit     │
├──────────────────────────────────────────────────────────────────────────────┤
│ 2 Streams  [n] new  [j/k] select  [↑/↓] scroll  [x] stop  [c] clear  [esc]   │
│                                                                              │
│ ╭─ Active Streams ───────────────────────────────────────────────────────╮   │
│ │ > ● running  telemetry.stream     telemetry   2m 34s   127 events      │   │
│ │   ○ ended    metrics.collect      monitor     5m 12s   342 events      │   │
│ ╰────────────────────────────────────────────────────────────────────────╯   │
│                                                                              │
│ ╭─ Stream Events: telemetry.stream ──────────────────────────── [↑/↓] ───╮   │
│ │ [metric] {"name":"cpu.percent","value":12.3,"unit":"%"}                │   │
│ │ [metric] {"name":"mem.mb","value":482.1,"unit":"MB"}                   │   │
│ │ [metric] {"name":"cpu.percent","value":11.8,"unit":"%"}                │   │
│ │ [metric] {"name":"mem.mb","value":481.9,"unit":"MB"}                   │   │
│ │ [snapshot] {"cpu":11.8,"mem":481.9,"service":"backend"}                │   │
│ │ ▼ (127 events total, showing last 5)                                   │   │
│ ╰────────────────────────────────────────────────────────────────────────╯   │
│                                                                              │
├──────────────────────────────────────────────────────────────────────────────┤
│       [n] new [q] quick [j/k] select [↑/↓] scroll [x] stop [c] clear         │
└──────────────────────────────────────────────────────────────────────────────┘
```

#### Stream List Row Format

```
[cursor] [status] [op]                    [plugin]    [duration] [event count]
   >     ● run    telemetry.stream        telemetry   2m 34s     127 events
         ○ end    metrics.collect         monitor     5m 12s     342 events
         ✗ err    failing.stream          broken      0m 02s     0 events
```

Status indicators:
- `● running` (green) - Stream is active
- `○ ended` (gray) - Stream completed successfully
- `✗ error` (red) - Stream failed

### Phase 3: Quick-Start Stream Picker

When user presses [q] for quick-start:

```
┌──────────────────────────────────────────────────────────────────────────────┐
│ DevCtl — streams   ○ Stopped              [tab] switch [?] help [q] quit     │
├──────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│ ╭─ Quick Start Stream ───────────────────────────────────────────────────╮   │
│ │                                                                        │   │
│ │  Select a stream operation:                                            │   │
│ │                                                                        │   │
│ │  > telemetry.stream         Emit telemetry metrics        [telemetry]  │   │
│ │    logs.follow              Follow service logs           [monitor]    │   │
│ │    metrics.collect          Collect system metrics        [monitor]    │   │
│ │                                                                        │   │
│ │  [enter] Start with defaults  [e] Edit input  [esc] Cancel             │   │
│ │                                                                        │   │
│ ╰────────────────────────────────────────────────────────────────────────╯   │
│                                                                              │
├──────────────────────────────────────────────────────────────────────────────┤
│                        [↑/↓] select [enter] start [esc] cancel               │
└──────────────────────────────────────────────────────────────────────────────┘
```

If user presses [e] to edit input:

```
│ ╭─ Start Stream: telemetry.stream ───────────────────────────────────────╮   │
│ │                                                                        │   │
│ │  Plugin: telemetry                                                     │   │
│ │  Operation: telemetry.stream                                           │   │
│ │                                                                        │   │
│ │  Input (JSON):                                                         │   │
│ │  ┌────────────────────────────────────────────────────────────────┐    │   │
│ │  │ {"count": 100, "interval_ms": 250}                             │    │   │
│ │  └────────────────────────────────────────────────────────────────┘    │   │
│ │                                                                        │   │
│ │  [enter] Start  [esc] Cancel                                           │   │
│ │                                                                        │   │
│ ╰────────────────────────────────────────────────────────────────────────╯   │
```

### Phase 4: Plugin View Enhancement

Add stream capability indicator to Plugins view:

```
╭─ Plugins (3) ──────────────────────────────────── [↑/↓] select [enter] ──╮
│                                                                          │
│ ▸ telemetry          priority: 10                              📊 stream │
│   monitor            priority: 20                              📊 stream │
│   deploy             priority: 30                                        │
│                                                                          │
╰──────────────────────────────────────────────────────────────────────────╯
```

Expanded plugin with streams:

```
│ ▾ telemetry          priority: 10                              📊 stream │
│   Path: python3 plugins/telemetry.py                                     │
│   Ops: telemetry.stream                                                  │
│   Streams: telemetry.stream                                              │
│   └─ [s] Start stream                                                    │
```

### Phase 5: Dashboard Streams Widget

Add streams summary to dashboard:

```
╭─ Dashboard ──────────────────────────────────────────────────────────────╮
│ ● System: Running (5m 23s)                                               │
│                                                                          │
│ Services (3):  ● backend ● frontend ● worker                             │
│                                                                          │
│ Streams (2):                                                             │
│   ● telemetry.stream (127 events, 2m 34s)                                │
│   ● metrics.collect (342 events, 5m 12s)                                 │
│   [tab→streams] Manage streams                                           │
╰──────────────────────────────────────────────────────────────────────────╯
```

## Implementation Plan

### Phase 1: Critical Fix (P0)
- [ ] Fix context cancellation in `stream_runner.go`
- [ ] Test with demo repo
- [ ] Verify streams run to completion

### Phase 2: Enhanced Streams View (P1)
- [ ] Add available stream ops to empty state
- [ ] Enhance stream row display (duration, event count)
- [ ] Add event count summary in viewport
- [ ] Track stream start time for duration display

### Phase 3: Quick-Start Picker (P2)
- [ ] Add [q] keybind for quick-start
- [ ] Create stream op picker model
- [ ] Query plugins for stream capabilities
- [ ] Pre-populate with default inputs
- [ ] Allow input editing before start

### Phase 4: Plugin View Enhancement (P2)
- [ ] Add stream indicator to plugin rows
- [ ] Add [s] action to start stream from plugin
- [ ] Show stream ops in expanded plugin view

### Phase 5: Dashboard Widget (P3)
- [ ] Add streams summary section
- [ ] Show active stream count and names
- [ ] Link to streams view

## Data Model Changes

### StreamsModel Additions

```go
type streamRow struct {
    Key        string
    PluginID   string
    Op         string
    StreamID   string
    Status     string // "running" | "ended" | "error"
    StartedAt  time.Time  // NEW: for duration calculation
    EventCount int        // NEW: track total events
    LastEvent  string     // NEW: preview of last event
}

type StreamsModel struct {
    // ... existing fields ...
    
    // NEW: Available stream operations for empty state / quick-start
    availableOps []streamOpInfo
    
    // NEW: Quick-start picker state
    picking    bool
    pickList   []streamOpInfo
    pickIdx    int
}

type streamOpInfo struct {
    PluginID    string
    Op          string
    Description string // from plugin metadata if available
}
```

## Open Questions

1. **Stream descriptions**: Should plugins provide descriptions for stream ops in handshake? Currently not in protocol.

2. **Default inputs**: Where should default inputs come from? Hardcoded? Plugin metadata? Persisted from last use?

3. **Stream persistence**: Should running streams survive TUI restart? Would need background daemon.

## References

- Architecture Report: `analysis/01-streams-tui-integration-architecture-report.md`
- Investigation Report: `analysis/02-streams-tui-integration-investigation-report.md`
- Bug Report: `analysis/03-bug-stream-context-canceled-immediately-after-start.md`
