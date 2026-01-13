---
Title: 'BUG: Stream Context Canceled Immediately After Start'
Ticket: STREAMS-TUI
Status: active
Topics:
    - devctl
    - tui
    - streams
    - bug
DocType: analysis
Intent: long-term
Owners: []
RelatedFiles:
    - Path: pkg/tui/stream_runner.go
      Note: Contains the bug - stream context derived from message context
ExternalSources: []
Summary: Stream context is derived from the message handler context, which gets canceled when the handler returns, causing streams to immediately terminate.
LastUpdated: 2026-01-08
WhatFor: Bug tracking and fix reference
WhenToUse: When fixing the stream context issue
---

# BUG: Stream Context Canceled Immediately After Start

## Summary

**Severity**: High (Blocks all streams TUI functionality)
**Component**: `pkg/tui/stream_runner.go`
**Status**: Identified, needs fix

## Symptoms

1. User starts a stream in the TUI Streams view
2. Stream briefly shows as "running"
3. Stream immediately ends with error: "context canceled"
4. No stream events are received

## Reproduction Steps

```bash
# Create demo repo
mkdir -p /tmp/devctl-stream-demo/plugins
cat > /tmp/devctl-stream-demo/.devctl.yaml << 'EOF'
plugins:
  - id: telemetry
    path: python3
    args:
      - plugins/telemetry.py
    priority: 10
EOF

# Copy telemetry plugin
cp devctl/testdata/plugins/telemetry/plugin.py /tmp/devctl-stream-demo/plugins/telemetry.py

# Run TUI
cd /tmp/devctl-stream-demo
devctl tui

# Navigate to Streams view (tab 4 times)
# Press 'n' to create new stream
# Enter: {"op":"telemetry.stream","plugin_id":"telemetry","input":{"count":10}}
# Press Enter

# Expected: Stream runs, events appear
# Actual: Stream immediately shows "error" with "context canceled"
```

## Root Cause Analysis

In `pkg/tui/stream_runner.go`, line 64:

```go
func (m *streamManager) handleStart(ctx context.Context, req StreamStartRequest) error {
```

The `ctx` argument is the Watermill message context (`msg.Context()`). This context is:
1. Created per-message
2. Canceled when the message handler returns
3. Possibly canceled when the message is acknowledged

The bug is on line 181:

```go
streamCtx, cancel := context.WithCancel(ctx)
```

This creates a child context of the message context. When `handleStart` returns (after starting the forwardEvents goroutine), the message context is canceled, which cascades to `streamCtx`, which terminates the stream.

## The Problematic Flow

```
1. Message received with ctx from msg.Context()
2. handleStart(ctx, req) called
3. factory.Start(ctx, spec, ...) - uses ctx (works, short-lived)
4. client.StartStream(startCtx, op, input) - uses ctx child (works, has timeout)
5. streamCtx, cancel := context.WithCancel(ctx) - BUG: child of msg ctx
6. go forwardEvents(streamCtx, h, events) - starts goroutine
7. handleStart returns
8. Watermill acks message, may cancel ctx
9. streamCtx canceled → forwardEvents exits → StreamEnded published
```

## Fix

Replace line 181:

```go
// WRONG: context tied to message lifecycle
streamCtx, cancel := context.WithCancel(ctx)

// CORRECT: independent context for stream lifetime
streamCtx, cancel := context.WithCancel(context.Background())
```

The stream lifecycle should be independent of the message that triggered it. The stream should only end when:
1. The plugin sends an "end" event
2. The user explicitly stops the stream
3. The TUI shuts down (via a different mechanism)

## Additional Observations

The same issue may affect plugin startup:

```go
client, err = m.factory.Start(ctx, spec, runtime.StartOptions{Meta: repo.Request})
```

This uses the message context for plugin startup. If plugin startup takes longer than message processing, this could also fail. However, since handshake is typically fast (2s timeout), this is less likely to be an issue in practice.

For robustness, consider using `context.Background()` for `factory.Start` as well, or at minimum use a timeout context:

```go
startCtx, startCancel := context.WithTimeout(context.Background(), 10*time.Second)
defer startCancel()
client, err = m.factory.Start(startCtx, spec, runtime.StartOptions{Meta: repo.Request})
```

## Testing After Fix

1. Start stream in TUI
2. Verify stream shows "running" and stays running
3. Verify events appear in the events log
4. Verify stream ends with "ok" when plugin finishes
5. Verify 'x' (stop) works to cancel running stream
