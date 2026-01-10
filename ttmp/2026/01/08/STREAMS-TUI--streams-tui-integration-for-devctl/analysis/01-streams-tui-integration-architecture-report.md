---
Title: Streams TUI Integration - Architecture Report
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
      Note: UIStreamRunner implementation - centralized stream management
    - Path: pkg/tui/stream_events.go
      Note: Stream event types (StreamStarted, StreamEvent, StreamEnded)
    - Path: pkg/tui/stream_actions.go
      Note: Stream action publishing (PublishStreamStart, PublishStreamStop)
    - Path: pkg/tui/models/streams_model.go
      Note: Bubble Tea model for streams view
    - Path: pkg/tui/transform.go
      Note: Domain-to-UI transformer including stream events
    - Path: pkg/tui/forward.go
      Note: UI forwarder that sends stream messages to Bubble Tea
    - Path: pkg/tui/topics.go
      Note: Topic and type constants for stream events
    - Path: pkg/tui/msgs.go
      Note: Bubble Tea message types including stream messages
    - Path: pkg/tui/models/root_model.go
      Note: Root model with ViewStreams and stream message handling
    - Path: cmd/devctl/cmds/tui.go
      Note: TUI command that wires up RegisterUIStreamRunner
    - Path: cmd/devctl/cmds/stream.go
      Note: CLI stream command for testing streams
    - Path: testdata/plugins/telemetry/plugin.py
      Note: Test telemetry plugin that produces streams
    - Path: testdata/plugins/stream/plugin.py
      Note: Basic test stream plugin
ExternalSources: []
Summary: Complete architecture analysis of the streams TUI integration in devctl, documenting all implemented components and their relationships.
LastUpdated: 2026-01-08
WhatFor: Understanding the current streams architecture before investigating issues and designing improvements.
WhenToUse: Reference when debugging streams issues or extending streams functionality.
---

# Streams TUI Integration - Architecture Report

## Executive Summary

The devctl TUI has **comprehensive streams infrastructure implemented**, including:
- A centralized `UIStreamRunner` that manages stream lifecycles
- Complete message flow from domain events to Bubble Tea UI
- A dedicated `StreamsModel` view with full CRUD operations
- CLI tooling (`devctl stream start`) for testing

However, the streams feature may not be visible in typical usage because:
1. Users need to manually start streams using the [n] key and paste JSON
2. No plugins are auto-discovered that expose stream operations
3. The stream view is the last tab in the navigation cycle

## Architecture Overview

### Component Diagram

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              Bubble Tea TUI                                 │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │                          RootModel                                   │   │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐  │   │
│  │  │Dashboard │ │ Events   │ │ Pipeline │ │ Plugins  │ │ Streams  │  │   │
│  │  │ Model    │ │ Model    │ │ Model    │ │ Model    │ │ Model    │  │   │
│  │  └──────────┘ └──────────┘ └──────────┘ └──────────┘ └────┬─────┘  │   │
│  └───────────────────────────────────────────────────────────┼─────────┘   │
│                                                              │             │
│  User Actions (n=new, x=stop)                               │             │
│         │                                                    │             │
│         ▼                                                    ▼             │
│  ┌────────────────────────────────────────────────────────────────────┐   │
│  │              StreamStartRequestMsg / StreamStopRequestMsg           │   │
│  └─────────────────────────────────┬──────────────────────────────────┘   │
└────────────────────────────────────┼───────────────────────────────────────┘
                                     │
                                     ▼
┌────────────────────────────────────────────────────────────────────────────┐
│                           Watermill Bus                                     │
│  ┌─────────────────┐      ┌─────────────────┐      ┌─────────────────┐    │
│  │ TopicUIActions  │ ───► │TopicDevctlEvents│ ───► │ TopicUIMessages │    │
│  │ tui.stream.*    │      │ stream.*        │      │ tui.stream.*    │    │
│  └────────┬────────┘      └────────┬────────┘      └────────┬────────┘    │
└───────────┼────────────────────────┼────────────────────────┼──────────────┘
            │                        │                        │
            ▼                        │                        ▼
┌────────────────────────┐          │          ┌────────────────────────┐
│   UIStreamRunner       │          │          │   UIForwarder          │
│   (stream_runner.go)   │──────────┘          │   (forward.go)         │
│                        │                      │                        │
│  - Start plugin client │                      │  - Converts envelopes  │
│  - Call StartStream    │                      │    to tea.Msg types    │
│  - Forward events      │                      │  - Sends to program    │
│  - Handle stop/cleanup │                      └────────────────────────┘
└───────────┬────────────┘
            │
            ▼
┌────────────────────────┐
│   Plugin Process       │
│   (runtime.Client)     │
│                        │
│  - Handshake           │
│  - Request/Response    │
│  - Stream events       │
└────────────────────────┘
```

### Message Flow

#### Starting a Stream

1. **User Input** → Press `n` in Streams view, paste JSON, press Enter
2. **StreamsModel** → Parses JSON, returns `StreamStartRequestMsg`
3. **RootModel** → Calls `publishStreamStart(req)`
4. **PublishStreamStart** → Publishes to `TopicUIActions` with type `tui.stream.start`
5. **UIStreamRunner** → Receives request, starts plugin, calls `StartStream`
6. **Plugin** → Responds with `stream_id`, begins emitting events
7. **UIStreamRunner** → Publishes `StreamStarted` to `TopicDevctlEvents`
8. **Transformer** → Maps to `UITypeStreamStarted`, publishes to `TopicUIMessages`
9. **Forwarder** → Sends `StreamStartedMsg` to Bubble Tea program
10. **RootModel** → Forwards to `StreamsModel.Update()`
11. **StreamsModel** → Adds stream to list, updates view

#### Stream Events

1. **Plugin** → Emits `{"type":"event", "stream_id":"...", ...}`
2. **runtime.Client** → Routes to stream's event channel
3. **UIStreamRunner.forwardEvents** → Reads from channel, publishes `StreamEvent`
4. **Transformer** → Maps to `UITypeStreamEvent`
5. **Forwarder** → Sends `StreamEventMsg`
6. **StreamsModel** → Appends to `eventsByKey[streamKey]`

#### Stopping a Stream

1. **User Input** → Press `x` on selected stream
2. **StreamsModel** → Returns `StreamStopRequestMsg`
3. **RootModel** → Calls `publishStreamStop(req)`
4. **UIStreamRunner** → Finds handle, sets `stoppedByUser=true`, cancels context, closes client
5. **forwardEvents defer** → Publishes `StreamEnded`
6. **StreamsModel** → Updates stream status to "ended" or "error"

## Key Components

### 1. Stream Events (`pkg/tui/stream_events.go`)

Data structures for stream lifecycle:

```go
type StreamStartRequest struct {
    PluginID string         `json:"plugin_id,omitempty"`
    Op       string         `json:"op"`
    Input    map[string]any `json:"input,omitempty"`
    Label    string         `json:"label,omitempty"`
}

type StreamStarted struct {
    StreamKey string    `json:"stream_key"`
    PluginID  string    `json:"plugin_id"`
    Op        string    `json:"op"`
    StreamID  string    `json:"stream_id"` // protocol stream_id
    At        time.Time `json:"at"`
}

type StreamEvent struct {
    StreamKey string         `json:"stream_key"`
    Event     protocol.Event `json:"event"`
    // ... timestamp, plugin info
}

type StreamEnded struct {
    StreamKey string    `json:"stream_key"`
    Ok        bool      `json:"ok"`
    Error     string    `json:"error,omitempty"`
    // ...
}
```

**StreamKey** is a local identifier (`pluginID:op:hash(input)`) distinct from the protocol's `stream_id`.

### 2. UIStreamRunner (`pkg/tui/stream_runner.go`)

Centralized stream management:

```go
type streamManager struct {
    mu      sync.Mutex
    byKey   map[string]*streamHandle
    factory *runtime.Factory
    opts    RootOptions
    pub     message.Publisher
}

func RegisterUIStreamRunner(bus *Bus, opts RootOptions)
```

Key behaviors:
- **Plugin resolution**: If `PluginID` specified, use that plugin; otherwise try each until one supports the op
- **Capability gating**: Checks `client.SupportsOp(op)` before calling `StartStream`
- **One client per stream**: Clean stop semantics (closing client terminates stream)
- **Cleanup on exit**: defer block ensures `StreamEnded` is always published

### 3. StreamsModel (`pkg/tui/models/streams_model.go`)

Bubble Tea model for the Streams view:

```go
type StreamsModel struct {
    streams     []streamRow       // List of active/ended streams
    selected    int               // Currently selected stream
    eventsByKey map[string][]string // Event log per stream
    creating    bool              // In "new stream" mode
    createIn    textinput.Model   // JSON input for new stream
    vp          viewport.Model    // Scrollable event display
}
```

Keybindings:
- `n` → New stream (JSON input mode)
- `j/k` → Select stream
- `↑/↓` → Scroll event log
- `x` → Stop selected stream
- `c` → Clear selected stream's events
- `esc` → Navigate back

### 4. Topics and Types (`pkg/tui/topics.go`)

```go
// Domain events (from UIStreamRunner)
DomainTypeStreamStarted = "stream.started"
DomainTypeStreamEvent   = "stream.event"
DomainTypeStreamEnded   = "stream.ended"

// UI messages (to Bubble Tea)
UITypeStreamStarted      = "tui.stream.started"
UITypeStreamEvent        = "tui.stream.event"
UITypeStreamEnded        = "tui.stream.ended"

// UI actions (from Bubble Tea)
UITypeStreamStartRequest = "tui.stream.start"
UITypeStreamStopRequest  = "tui.stream.stop"
```

### 5. Root Model Integration (`pkg/tui/models/root_model.go`)

```go
type RootModel struct {
    // ...
    streams   StreamsModel
    
    publishStreamStart func(tui.StreamStartRequest) error
    publishStreamStop  func(tui.StreamStopRequest) error
}
```

Navigation:
- `ViewStreams` is the 6th tab (after Plugins)
- Tab cycle: Dashboard → Events → Pipeline → Plugins → Streams → Dashboard

### 6. CLI Testing (`cmd/devctl/cmds/stream.go`)

```bash
devctl stream start --op telemetry.stream --plugin telemetry --input-json '{"count":5}'
```

Useful for testing streams outside the TUI.

## Test Plugins

### telemetry/plugin.py

- Op: `telemetry.stream`
- Behavior: Emits N metrics at configurable interval, then ends
- Input: `{"count": 3, "interval_ms": 10}`

### stream/plugin.py

- Op: `stream.start`
- Behavior: Emits "hello", "world", then ends

## Wiring in tui.go

```go
tui.RegisterUIStreamRunner(bus, tui.RootOptions{...})

model := models.NewRootModel(models.RootModelOptions{
    PublishStreamStart: func(req tui.StreamStartRequest) error {
        return tui.PublishStreamStart(bus.Publisher, req)
    },
    PublishStreamStop: func(req tui.StreamStopRequest) error {
        return tui.PublishStreamStop(bus.Publisher, req)
    },
})
```

## Summary

The streams infrastructure is **fully implemented** at all layers:
- ✅ Protocol support (runtime.Client.StartStream)
- ✅ UIStreamRunner (centralized lifecycle management)
- ✅ Message bus wiring (transform, forward)
- ✅ Bubble Tea model (StreamsModel)
- ✅ Root model integration (navigation, message routing)
- ✅ CLI tooling (devctl stream start)
- ✅ Test plugins (telemetry, stream)

The next step is to investigate **why streams don't appear to work** in practice.
