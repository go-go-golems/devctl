---
Title: Streams TUI Integration - Investigation Report
Ticket: STREAMS-TUI
Status: active
Topics:
    - devctl
    - tui
    - streams
DocType: analysis
Intent: long-term
Owners: []
RelatedFiles:
    - Path: pkg/tui/stream_runner.go
      Note: Contains context bug causing immediate stream cancellation
    - Path: pkg/tui/models/streams_model.go
      Note: TUI model that works correctly once bug is fixed
    - Path: cmd/devctl/cmds/stream.go
      Note: CLI stream command works correctly (no context issue)
ExternalSources: []
Summary: Investigation into why streams do not appear to work in the TUI, identifying a critical context cancellation bug and UX issues.
LastUpdated: 2026-01-08
WhatFor: Reference for fixing streams TUI and planning improvements.
WhenToUse: When implementing stream fixes and UX improvements.
---

# Streams TUI Integration - Investigation Report

## Executive Summary

The streams TUI integration has a **critical bug** that prevents any streams from running successfully. Additionally, there are several UX issues that make the streams feature hard to discover and use.

### Critical Bug

**Stream context is derived from message context, causing immediate cancellation.**

- Location: `pkg/tui/stream_runner.go:181`
- Impact: All streams fail with "context canceled" immediately after starting
- Fix: Use `context.Background()` instead of message context for stream lifecycle

### UX Issues

1. **Poor discoverability**: Streams view is the last tab (5th), easy to miss
2. **JSON input required**: Users must construct JSON to start streams
3. **No plugin capability display**: Users can't see which plugins offer streams
4. **No auto-start**: No way to configure streams to start automatically
5. **Limited stream information**: Only shows op and plugin, not input or duration

## Investigation Methodology

### Test Environment Setup

```bash
# Created demo repo at /tmp/devctl-stream-demo
mkdir -p /tmp/devctl-stream-demo/plugins
cat > /tmp/devctl-stream-demo/.devctl.yaml << 'EOF'
plugins:
  - id: telemetry
    path: python3
    args:
      - plugins/telemetry.py
    priority: 10
EOF
cp testdata/plugins/telemetry/plugin.py /tmp/devctl-stream-demo/plugins/telemetry.py
```

### CLI Stream Test (Baseline)

```bash
cd /tmp/devctl-stream-demo
devctl stream start --op telemetry.stream --input-json '{"count":5,"interval_ms":100}'
```

**Result**: ✅ Works perfectly

```
plugin=telemetry op=telemetry.stream stream_id=telemetry-telemetry-1
[metric] {"name":"counter","unit":"count","value":0}
[metric] {"name":"counter","unit":"count","value":1}
[metric] {"name":"counter","unit":"count","value":2}
[metric] {"name":"counter","unit":"count","value":3}
[metric] {"name":"counter","unit":"count","value":4}
[end ok=true]
```

### TUI Stream Test

```bash
# Run TUI with tmux for automation
tmux new-session -d -s devctl-test devctl tui --alt-screen=false
# Navigate to Streams (tab x4)
# Press 'n' for new stream
# Enter JSON: {"op":"telemetry.stream","plugin_id":"telemetry","input":{"count":10}}
# Press Enter
```

**Result**: ❌ Stream starts then immediately fails

```
> running telemetry.stream (plugin=telemetry)
[... briefly ...]
> error telemetry.stream (plugin=telemetry)
[end] context canceled
```

### Code Analysis

Traced the message flow:

1. `StreamsModel` emits `StreamStartRequestMsg` on Enter
2. `RootModel` calls `publishStreamStart(req)` 
3. `PublishStreamStart` publishes to `TopicUIActions`
4. `UIStreamRunner` handler receives message with `msg.Context()`
5. `handleStart(ctx, req)` called with message context
6. Plugin started with message context ✅ (short-lived, works)
7. Stream started with timeout context ✅ (2s, works)
8. **BUG**: `streamCtx, cancel := context.WithCancel(ctx)` uses message context
9. `go forwardEvents(streamCtx, h, events)` starts
10. `handleStart` returns, message acked
11. Message context canceled → streamCtx canceled → stream ends

### Root Cause

```go
// pkg/tui/stream_runner.go:181
streamCtx, cancel := context.WithCancel(ctx)  // ctx is msg.Context()
```

The stream context is a child of the message context. When the handler returns and the message is acknowledged, the context is canceled, terminating the stream.

## Findings

### What Works

1. ✅ Protocol layer streams (`runtime.Client.StartStream`)
2. ✅ CLI streams (`devctl stream start`)
3. ✅ Message bus wiring (transform, forward)
4. ✅ StreamsModel UI rendering
5. ✅ Stream event display (when events are received)
6. ✅ Stream start/stop message publishing

### What Doesn't Work

1. ❌ **Stream lifecycle** - context bug kills streams immediately
2. ❌ **Stream events** - never received because stream dies

### UX Observations

1. **Navigation**: Tab cycle goes Dashboard → Events → Pipeline → Plugins → Streams. Streams is last and easy to miss.

2. **Empty state**: Shows "No active streams" with instruction to press [n]. Good, but doesn't explain what streams are or show available stream ops.

3. **JSON input**: Users must know the exact JSON schema. The placeholder helps but is still technical:
   ```
   {"op":"telemetry.stream","plugin_id":"","input":{"count":3,"interval_ms":250}}
   ```

4. **No discovery**: Users can't see which plugins offer stream operations. The Plugins view shows capabilities but doesn't have an action to start streams.

5. **Stream list**: Shows minimal info (status, op, plugin). Missing:
   - Stream start time / duration
   - Event count
   - Input parameters
   - Last event preview

## Recommendations

### Critical Fix (Required)

Fix the context bug in `stream_runner.go`:

```go
// BEFORE (line 181)
streamCtx, cancel := context.WithCancel(ctx)

// AFTER
streamCtx, cancel := context.WithCancel(context.Background())
```

### UX Improvements (Prioritized)

1. **P0: Context fix** - Without this, streams are unusable
2. **P1: Show stream ops in Plugins view** - Add indicator/action for stream-capable plugins
3. **P2: Stream picker** - Instead of JSON, show a list of available stream ops
4. **P2: Better stream info** - Show duration, event count, last event
5. **P3: Quick-start from Plugins** - Select plugin → select stream op → start
6. **P3: Dashboard stream widget** - Show active streams count/status

## Testing Plan After Fix

1. Start telemetry stream, verify events appear
2. Start multiple streams, verify independent operation
3. Stop stream with 'x', verify clean termination
4. Clear stream events with 'c', verify UI updates
5. Navigate away and back, verify stream state preserved
6. Close TUI, verify clean shutdown of all streams
