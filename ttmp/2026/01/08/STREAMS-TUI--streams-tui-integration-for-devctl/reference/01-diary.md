---
Title: Diary
Ticket: STREAMS-TUI
Status: active
Topics:
    - devctl
    - tui
    - streams
DocType: reference
Intent: long-term
Owners: []
RelatedFiles:
    - Path: devctl/cmd/devctl/cmds/tui.go
      Note: Defines TUI lifetime context and errgroup wiring.
    - Path: devctl/pkg/runtime/factory.go
      Note: Plugin process lifetime is bound to exec.CommandContext.
    - Path: devctl/pkg/tui/action_runner.go
      Note: Uses message context for long-running actions (lifetime analysis).
    - Path: devctl/pkg/tui/bus.go
      Note: Router run context and shutdown behavior for TUI.
    - Path: devctl/pkg/tui/models/streams_model.go
      Note: Enhanced with duration/event count (Step 3)
    - Path: devctl/pkg/tui/state_watcher.go
      Note: Baseline for correctly scoped background context.
    - Path: devctl/pkg/tui/stream_runner.go
      Note: Contains context bug fix (Step 2)
ExternalSources: []
Summary: Step-by-step implementation diary for Streams TUI integration.
LastUpdated: 2026-01-08T00:00:00Z
WhatFor: Track implementation progress and decisions.
WhenToUse: Reference for continuing work or reviewing changes.
---


# Diary

## Goal

Fix and enhance the Streams TUI in devctl to make streams usable and discoverable.

## Step 1: Analysis and Investigation

Analyzed the existing streams infrastructure to understand what was implemented and why streams weren't working in the TUI.

### What I did
- Read all stream-related source files (stream_runner.go, streams_model.go, etc.)
- Documented the complete architecture in analysis report
- Set up a demo repo at `/tmp/devctl-stream-demo` with telemetry plugin
- Tested CLI streams (`devctl stream start`) - works correctly
- Tested TUI streams - discovered critical bug
- Used tmux automation to reproduce the issue

### What worked
- CLI streams work perfectly, confirming protocol/runtime layer is correct
- Message bus wiring is complete (transform, forward)
- StreamsModel renders and responds to keyboard input

### What didn't work
- TUI streams fail immediately with "context canceled"
- Stream shows "running" briefly then "error"
- No events are ever displayed

### What I learned
- The entire streams infrastructure is implemented and well-designed
- The bug is isolated to a single line in stream_runner.go
- UX needs improvement but core functionality is sound

### What was tricky to build
- N/A (analysis phase)

### What warrants a second pair of eyes
- The context usage pattern in message handlers

### What should be done in the future
- Consider adding stream descriptions to plugin handshake protocol
- Consider stream persistence across TUI restarts

### Technical details

Root cause identified:
```go
// pkg/tui/stream_runner.go:181
streamCtx, cancel := context.WithCancel(ctx)  // ctx is msg.Context()
```

The `ctx` is the Watermill message context, which is canceled when the handler returns. This kills the stream immediately.

---

## Step 2: Fix Context Cancellation Bug

Fixed the critical bug that prevents streams from running.

**Commit (code):** f1b1761 — "Fix: use background context for stream lifecycle"

### What I did
- Modified `pkg/tui/stream_runner.go` in 3 places:
  1. Line 125: factory.Start() for explicit plugin ID case
  2. Line 152: factory.Start() for auto-discovery loop case
  3. Line 187: streamCtx creation for forwardEvents goroutine
- Changed all from message context to `context.Background()`
- Rebuilt binary and tested with demo repo

### Why
The stream context and plugin process were derived from the Watermill message context. When the message handler returned, the context was canceled, which:
1. Killed the plugin process (via exec.CommandContext)
2. Canceled the stream context (triggering forwardEvents exit)

This caused streams to immediately fail with "context canceled".

### What worked
- All 10 metric events received
- Stream shows "ended" status (not "error")
- Clean termination with `[end]` event

### What was tricky to build
- There were actually 3 places using the message context, not just the obvious streamCtx
- The factory.Start calls also pass context to exec.CommandContext, which kills the process on cancellation

### What warrants a second pair of eyes
- Confirm no other context usages in stream_runner.go need similar treatment
- The unused `ctx` parameter in handleStart could be removed or documented

### Code review instructions
- Look at `stream_runner.go` lines 125, 152, and 187
- Verify all context.Background() usages are correct
- Run test: `cd /tmp/devctl-stream-demo && devctl tui`, navigate to Streams, press 'n', enter `{"op":"telemetry.stream","plugin_id":"telemetry","input":{"count":10}}`

---

## Step 3: Enhance Stream Row Display

Added duration and event count to stream rows for better visibility.

**Commit (code):** 946fcc3 — "Enhance: add duration and event count to stream rows"

### What I did
- Added `EventCount` field to `streamRow` struct
- Updated `onStreamEvent` to increment event count for each event
- Enhanced `renderStreamList` to display:
  - Status icon (●/○/✗)
  - Status text
  - Operation name
  - Plugin ID
  - Duration (using existing `formatDuration` from service_model.go)
  - Event count

### What worked
- Stream row now shows: `> ○ ended  telemetry.stream  telemetry  4s  11 events`
- Much more informative than the old: `> ended telemetry.stream (plugin=telemetry)`

### What was tricky to build
- Had to reuse the existing `formatDuration` function from service_model.go rather than creating a duplicate

### Code review instructions
- Look at `streams_model.go` streamRow struct and renderStreamList function
- Run TUI, start a stream, verify the new display format

---

## Step 4: Improve Empty State and Plugin Stream Indicator

Added helpful instructions to empty state and prepared plugin stream indicator.

**Commit (code):** d50557b — "Enhance: improve streams empty state and add plugin stream indicator"

### What I did
- Updated streams empty state with:
  - Explanation of how to start a stream
  - JSON format example
  - Reference to Plugins view and CLI alternative
- Added Ops and Streams fields to PluginSummary struct
- Added stream indicator (📊 stream) to plugin row title line
- Updated PluginModel.WithPlugins to pass through Ops/Streams

### What worked
- Empty state now shows helpful instructions
- Infrastructure for stream indicator is in place

### What requires future work
- **Plugin introspection**: To show stream capabilities, we need to start plugins
  and read their handshake. The state watcher doesn't do this currently.
- **Quick-start picker**: Without introspection, we can't show available stream ops.

### Technical decision
Rather than implementing runtime plugin introspection (expensive, starts all plugins),
I focused on improving the UX with better empty state instructions. Users can still:
1. Check plugin documentation for available stream ops
2. Use `devctl stream start --op <op>` to discover ops (fails fast if unsupported)
3. Look at plugin source code to see handshake capabilities

---

## Step 5: Summary

### Completed
1. ✅ Fixed critical context cancellation bug (commit f1b1761)
2. ✅ Enhanced stream row display with duration/event count (commit 946fcc3)
3. ✅ Improved empty state with instructions (commit d50557b)
4. ✅ Added plugin stream indicator infrastructure (commit d50557b)

### Deferred (requires plugin introspection)
- [ ] Populate plugin Ops/Streams from runtime handshake
- [ ] Add quick-start stream picker [q]
- [ ] Add streams widget to Dashboard

### All commits
```
d50557b Enhance: improve streams empty state and add plugin stream indicator
946fcc3 Enhance: add duration and event count to stream rows
5f9483e Docs: add STREAMS-TUI ticket with analysis and design
f1b1761 Fix: use background context for stream lifecycle
```

### Testing Performed
- Started telemetry stream with 10 events
- Verified all events received
- Verified stream completes with "ended" status
- Verified duration and event count display correctly
- Verified empty state shows helpful instructions

---

## Step 6: Analyze TUI Context Lifetimes for Streams

Mapped the TUI context lifetimes across Bubbletea, the Watermill bus, and the stream runner to understand why stream work outlives the UI. Documented how message contexts are set in Watermill and why background contexts are currently used in stream runner.

Captured a detailed analysis document that evaluates the correctness of the original diagnosis and proposes solutions that scope streams to the TUI lifetime.

### What I did
- Read `stream_runner.go`, `action_runner.go`, `tui.go`, `bus.go`, `state_watcher.go`, and `runtime/factory.go`
- Verified Watermill message context behavior in module source
- Wrote the analysis doc: `analysis/04-streams-tui-context-lifetime-analysis.md`

### Why
- The stream runner currently uses background contexts, which lets streams outlive the TUI
- We need a documented, consistent lifecycle model before changing context wiring

### What worked
- Built a clear context-lifetime map of the TUI, bus, and stream runner
- Identified action runner as a similar context-lifetime mismatch

### What didn't work
- N/A

### What I learned
- Watermill message contexts default to `context.Background()` and are not canceled when handlers return
- Plugin processes are bound to the context passed to `exec.CommandContext` in `runtime.Factory`

### What was tricky to build
- Separating the original “msg.Context canceled” hypothesis from the actual TUI-lifetime mismatch

### What warrants a second pair of eyes
- Confirm the proposed TUI-scoped context wiring for stream runner and action runner before implementing

### What should be done in the future
- Implement TUI-scoped context injection for stream runner (and likely action runner)
- Decide on shutdown semantics for stream events when the TUI exits

### Code review instructions
- Start with `analysis/04-streams-tui-context-lifetime-analysis.md`
- Review `devctl/pkg/tui/stream_runner.go` and `devctl/cmd/devctl/cmds/tui.go` for lifecycle wiring

### Technical details
- Key finding: background contexts keep streams alive after UI exit; use a TUI-scoped parent instead
