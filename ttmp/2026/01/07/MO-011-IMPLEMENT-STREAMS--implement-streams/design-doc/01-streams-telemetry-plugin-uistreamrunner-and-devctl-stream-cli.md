---
Title: 'Streams: telemetry plugin, UIStreamRunner, and devctl stream CLI'
Ticket: MO-011-IMPLEMENT-STREAMS
Status: active
Topics:
    - streams
    - tui
    - plugins
DocType: design-doc
Intent: long-term
Owners: []
RelatedFiles:
    - Path: devctl/pkg/protocol/types.go
      Note: Event/handshake schema used by telemetry stream design
    - Path: devctl/pkg/runtime/client.go
      Note: StartStream API used by UIStreamRunner and devctl stream CLI
    - Path: devctl/pkg/runtime/router.go
      Note: Stream event routing/buffering behavior that shapes runner backpressure design
    - Path: devctl/pkg/tui/action_runner.go
      Note: Pattern for a centralized runner in the TUI process
    - Path: devctl/pkg/tui/forward.go
      Note: Must be extended to forward stream UI messages to Bubble Tea
    - Path: devctl/pkg/tui/transform.go
      Note: Must be extended to map domain stream events to UI messages
ExternalSources: []
Summary: 'Design for a telemetry streaming plugin shape plus two devctl surfaces that leverage protocol streams: a centralized TUI UIStreamRunner and a devctl stream CLI.'
LastUpdated: 2026-01-07T20:43:08-05:00
WhatFor: Provide an implementable plan to make protocol streams usable from the TUI and CLI while keeping stream lifecycles centralized and robust against misbehaving plugins.
WhenToUse: When implementing MO-011 (streams), adding stream-producing plugin ops (telemetry/logs/metrics), or building debugging tools around StartStream.
---


# Streams: telemetry plugin, UIStreamRunner, and devctl stream CLI

## Executive summary

This design introduces two concrete, production-facing surfaces for devctl protocol streams:

- A reference “telemetry stream” plugin pattern: capabilities, request/response, event schemas, and cancellation expectations.
- A centralized `UIStreamRunner` inside the TUI process that owns stream lifecycles and publishes typed stream events into the existing Watermill → transformer → Bubble Tea pipeline.
- A `devctl stream` CLI to start a stream op and print events, which doubles as a debugging tool and a fixture-validation harness.

Key design principle: centralize stream management (start/stop, timeouts, cancellation, cleanup) in one subsystem, instead of having Bubble Tea models call `runtime.Client.StartStream` directly.

## Problem statement

The repo already implements streams at the protocol and runtime layers:
- Protocol supports `event` frames and handshake `capabilities.streams` (`devctl/pkg/protocol/types.go`).
- Runtime supports `Client.StartStream` with buffered event delivery (`devctl/pkg/runtime/client.go`, `devctl/pkg/runtime/router.go`).

But streams are not usable in production today because:
- No production subsystem calls `StartStream` (tests only).
- The TUI bus does not have typed stream events, nor a runner that owns long-lived stream loops.
- There is no CLI affordance to start a stream and observe events (outside tests).

Additionally, there is an important robustness constraint:
- `StartStream` invocation is gated on `handshake.capabilities.ops` (authoritative). `capabilities.streams` is informational/UX-facing (“this op is stream-producing”), but does not grant permission to invoke by itself. This prevents hangs on “streams-only” plugins that never respond.

## Proposed solution

### Part A — Telemetry streaming plugin (contract + example)

This section is a “copy/paste template” for plugin authors.

#### A.1 Handshake capabilities

Telemetry streaming is modeled as a normal request op that produces a stream:
- `capabilities.ops` must include the stream-start op (authoritative for invocation).
- `capabilities.streams` should also include it (informational/UX: “this op is stream-producing”).

Example handshake:

```json
{
  "type": "handshake",
  "protocol_version": "v2",
  "plugin_name": "telemetry",
  "capabilities": {
    "ops": ["telemetry.stream"],
    "streams": ["telemetry.stream"]
  },
  "declares": {
    "side_effects": "none",
    "idempotent": true
  }
}
```

#### A.2 Request / response interaction (start stream)

Request:

```json
{
  "type": "request",
  "request_id": "telemetry-1",
  "op": "telemetry.stream",
  "ctx": {
    "repo_root": "/abs/path/to/repo",
    "deadline_ms": 30000,
    "dry_run": false
  },
  "input": {
    "interval_ms": 250,
    "sources": ["process", "service.health", "custom.app"],
    "tags": { "env": "local" }
  }
}
```

Response (exactly one):

```json
{
  "type": "response",
  "request_id": "telemetry-1",
  "ok": true,
  "output": {
    "stream_id": "telemetry-telemetry-1",
    "schema": "telemetry.v1"
  }
}
```

Notes:
- `stream_id` is required by `runtime.Client.StartStream`.
- Additional output fields are allowed but should be stable and versioned (`schema`).

#### A.3 Event frames (telemetry event types)

Events use the existing `protocol.Event` structure. For telemetry, treat:
- `event` as the “telemetry event type” (kind),
- `fields` as structured payload,
- `message` as optional human summary.

Example events:

```json
{ "type":"event", "stream_id":"telemetry-telemetry-1", "event":"metric",
  "fields": { "name":"cpu.percent", "value": 12.3, "unit":"%", "labels":{"pid":1234} } }
{ "type":"event", "stream_id":"telemetry-telemetry-1", "event":"metric",
  "fields": { "name":"mem.mb", "value": 482.1, "unit":"MB", "labels":{"pid":1234} } }
{ "type":"event", "stream_id":"telemetry-telemetry-1", "event":"snapshot",
  "fields": { "cpu.percent": 12.3, "mem.mb": 482.1, "service":"backend" } }
{ "type":"event", "stream_id":"telemetry-telemetry-1", "event":"end", "ok": true }
```

Recommended conventions:
- Use `event="metric"` for single data points, and `event="snapshot"` for batched periodic summaries.
- Put stable identifiers in `fields`: `name`, `value`, `unit`, `labels`, plus any `service`/`pid`/`source` qualifiers.
- Emit `event="end"` exactly once when the stream finishes.

#### A.4 Plugin pseudocode (Python)

Minimal, single-stream-per-request pseudocode (one stream worker per request ID):

```python
#!/usr/bin/env python3
import json, sys, threading, time

def emit(obj):
    sys.stdout.write(json.dumps(obj) + "\n")
    sys.stdout.flush()

emit({
  "type": "handshake",
  "protocol_version": "v2",
  "plugin_name": "telemetry",
  "capabilities": {"ops": ["telemetry.stream"], "streams": ["telemetry.stream"]},
})

def telemetry_worker(stream_id: str, stop: threading.Event, interval_ms: int):
  try:
    while not stop.is_set():
      emit({"type":"event","stream_id":stream_id,"event":"metric",
            "fields":{"name":"cpu.percent","value": 12.3,"unit":"%","labels":{}}})
      emit({"type":"event","stream_id":stream_id,"event":"metric",
            "fields":{"name":"mem.mb","value": 482.1,"unit":"MB","labels":{}}})
      time.sleep(interval_ms / 1000.0)
  finally:
    emit({"type":"event","stream_id":stream_id,"event":"end","ok": True})

streams = {}  # stream_id -> stop event

for line in sys.stdin:
  line = line.strip()
  if not line:
    continue
  req = json.loads(line)
  rid = req.get("request_id", "")
  op = req.get("op", "")
  if op != "telemetry.stream":
    emit({"type":"response","request_id":rid,"ok":False,
          "error":{"code":"E_UNSUPPORTED","message":"unsupported op"}})
    continue

  inp = req.get("input", {}) or {}
  interval_ms = int(inp.get("interval_ms", 250))
  stream_id = f"telemetry-{rid}"

  stop = threading.Event()
  t = threading.Thread(target=telemetry_worker, args=(stream_id, stop, interval_ms), daemon=True)
  streams[stream_id] = stop
  t.start()

  emit({"type":"response","request_id":rid,"ok":True,
        "output":{"stream_id":stream_id,"schema":"telemetry.v1"}})

# stdin closed => stop all streams
for stop in streams.values():
  stop.set()
time.sleep(0.1)
```

#### A.5 Cancellation reality (important limitation)

In the current protocol/runtime:
- There is no generic “cancel stream by stream_id” message.
- The only universal cancellation signal is stdin closing (devctl terminates the plugin).

Therefore, a first implementation should assume:
- stream lifetime == plugin client lifetime, unless we add an explicit `telemetry.stop`/`stream.stop` op later.

### Part B — `UIStreamRunner` (centralized stream management in TUI)

#### B.1 Goals

- Centralize stream start/stop, timeouts, cancellation, and cleanup.
- Provide a typed event surface to Bubble Tea models (no direct `StartStream` calls in models).
- Avoid hangs and unbounded resource usage (goroutines, plugin processes, buffered events).

#### B.2 Non-goals

- Replacing the Service view’s file-tailing logs implementation immediately.
- Adding a new protocol-level cancel/stop op (documented limitation; could be a follow-up).
- Persisting streams across TUI restarts.

#### B.3 High-level architecture

Follow the same pattern as `RegisterUIActionRunner`:

1) Bubble Tea publishes a stream start request to `TopicUIActions`.
2) `UIStreamRunner` consumes that request, starts a plugin client, and calls `StartStream`.
3) `UIStreamRunner` publishes domain stream events to `TopicDevctlEvents`.
4) Transformer maps those to UI messages and forwarder sends them into Bubble Tea.

#### B.4 Message types (proposed additions)

Add new domain and UI envelope types in `devctl/pkg/tui/topics.go`:

- Domain:
  - `DomainTypeStreamStarted = "stream.started"`
  - `DomainTypeStreamEvent   = "stream.event"`
  - `DomainTypeStreamEnded   = "stream.ended"`
- UI:
  - `UITypeStreamStarted = "tui.stream.started"`
  - `UITypeStreamEvent   = "tui.stream.event"`
  - `UITypeStreamEnded   = "tui.stream.ended"`

Add new UI action types (also in `devctl/pkg/tui/topics.go`):
- `UITypeStreamStartRequest = "tui.stream.start"`
- `UITypeStreamStopRequest  = "tui.stream.stop"`

Add corresponding Bubble Tea message types in `devctl/pkg/tui/msgs.go`:
- `StreamStartedMsg`, `StreamEventMsg`, `StreamEndedMsg`

#### B.5 Data structures (API signatures)

Define these in a new file, e.g. `devctl/pkg/tui/stream_events.go`:

```go
type StreamStartRequest struct {
  PluginID string         `json:"plugin_id,omitempty"` // optional; may be resolved by op
  Op       string         `json:"op"`
  Input    map[string]any `json:"input,omitempty"`
  Label    string         `json:"label,omitempty"`     // display name (optional)
}

type StreamStopRequest struct {
  StreamKey string `json:"stream_key"`
}

// StreamKey is local: "plugin_id/op/(hash(input))" and is NOT the protocol stream_id.
type StreamStarted struct {
  StreamKey string    `json:"stream_key"`
  PluginID  string    `json:"plugin_id"`
  Op        string    `json:"op"`
  StreamID  string    `json:"stream_id"` // protocol stream_id
  At        time.Time `json:"at"`
}

type StreamEvent struct {
  StreamKey string        `json:"stream_key"`
  PluginID  string        `json:"plugin_id"`
  Op        string        `json:"op"`
  StreamID  string        `json:"stream_id"`
  At        time.Time     `json:"at"`
  Event     protocol.Event `json:"event"`
}

type StreamEnded struct {
  StreamKey string    `json:"stream_key"`
  PluginID  string    `json:"plugin_id"`
  Op        string    `json:"op"`
  StreamID  string    `json:"stream_id"`
  At        time.Time `json:"at"`
  Ok        bool      `json:"ok"`
  Error     string    `json:"error,omitempty"`
}
```

#### B.6 Runner behavior (core algorithm)

`UIStreamRunner` owns:
- starting plugin clients via `runtime.Factory`,
- deciding which plugin to call (by `PluginID` or “first plugin supporting op”),
- starting streams (`StartStream`),
- forwarding events until the stream ends or the user stops it,
- and cleanup (closing the plugin client).

Important constraint for v1:
- One client per stream. Without a protocol-level stop op, stopping a stream requires terminating the client, which would otherwise terminate other streams.

#### B.7 Capability gating rule (must not hang)

Before calling `StartStream`, the runner must gate on:
- `client.SupportsOp(req.Op)` (authoritative)

Optionally, it may also require `req.Op` to be present in `handshake.capabilities.streams` as a stricter consistency check, but it should not treat `streams` alone as permission to invoke.

#### B.8 Timeouts and cancellation

Use two time scales:
- Start timeout (short): only for the `StartStream` request/response (getting `stream_id`).
- Stream lifetime (long): stream loop continues until plugin emits `end`, user stops it, or TUI shuts down.

Stop semantics (v1):
- Stop request closes the plugin client for that stream (EOF closes event channel).

#### B.9 Backpressure policy (telemetry is high-frequency)

Because telemetry can be high-rate, the runner should implement one of:
- sampling (push interval to plugin),
- coalescing (publish latest snapshot every N ms),
- bounded queues with drop counters.

This is an implementation detail, but it must be planned early to avoid UI flooding.

### Part C — `devctl stream` CLI (debugging harness + developer surface)

#### C.1 CLI UX

Add:

```
devctl stream start --op telemetry.stream --plugin telemetry --input-json '{"interval_ms":250}' [--json]
```

Behavior:
- load repo config and start the specified plugin client,
- gate on `capabilities.ops`,
- start the stream (short start timeout),
- print events until end or interrupt,
- close client on exit.

#### C.2 Output formats

- Default (human): one line per event (compact fields).
- `--json`: output raw `protocol.Event` JSON lines.

#### C.3 Flags (proposed)

- `--plugin <id>` (optional): choose provider plugin; if omitted, pick first plugin supporting `--op`.
- `--op <string>` (required)
- `--input-json <json>` or `--input-file <path>`
- `--start-timeout <duration>` (default 2s)
- `--timeout <duration>` (optional overall duration)
- `--json`

## Design decisions

### D1) Centralize stream management in a runner (don’t call StartStream from models)

Streams are long-lived and require cancellation/cleanup. Bubble Tea models should request streams via messages and render results; they should not own plugin processes.

### D2) Gate stream-start requests on `capabilities.ops`

Avoid the “streams-only advertised, never responds” hang class and align with the protocol: starting a stream is still invoking a request op.

### D3) One client per stream (v1)

Without a protocol-level stop op, per-stream cancellation requires terminating the plugin. One client per stream makes stop semantics safe and local.

## Alternatives considered

### A) Let models call StartStream directly

Rejected: spreads lifecycle management across UI code and increases the risk of hangs/leaks.

### B) Add protocol-level `stream.stop` now

Deferred: bigger protocol change; the v1 design works without it by scoping stream lifetime to client lifetime.

### C) Reuse one long-lived plugin client for all streams

Deferred: requires stop semantics to avoid “stop one stream kills all streams”.

## Implementation plan

1) Add stream envelope types and structs (domain + UI + Bubble Tea msgs).
2) Implement `RegisterUIStreamRunner`:
   - load config, start plugin client per stream, gate by `SupportsOp`, call `StartStream`, publish stream events.
3) Extend transformer (`devctl/pkg/tui/transform.go`) and forwarder (`devctl/pkg/tui/forward.go`) to carry stream events into the UI.
4) Add a minimal UI surface:
   - append stream events to the existing Events log, or add a dedicated “Streams” view.
5) Implement `devctl stream start` CLI.
6) Validate with fixtures:
   - positive: `devctl/testdata/plugins/stream` and `long-running-plugin`
   - negative: a “streams-advertised but never responds” plugin (must fail fast, not hang).

## Open questions

1) Should `capabilities.streams` be enforced as well (op must be in both `ops` and `streams`), or remain informational?
2) Do we want to introduce a generic stop op (`stream.stop`) to avoid one-client-per-stream overhead?
3) What is the first UI surface to ship (Events log vs new Streams view vs metrics dashboard)?

## References

- Stream analysis: `devctl/ttmp/2026/01/07/MO-011-IMPLEMENT-STREAMS--implement-streams/analysis/01-streams-codebase-analysis-and-tui-integration.md`
- Protocol schemas: `devctl/pkg/protocol/types.go`
- Runtime StartStream + router: `devctl/pkg/runtime/client.go`, `devctl/pkg/runtime/router.go`
- Current UI action runner pattern: `devctl/pkg/tui/action_runner.go`
