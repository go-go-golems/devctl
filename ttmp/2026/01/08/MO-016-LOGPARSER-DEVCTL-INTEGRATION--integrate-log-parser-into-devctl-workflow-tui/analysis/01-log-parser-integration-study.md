---
Title: Log-parser integration study
Ticket: MO-016-LOGPARSER-DEVCTL-INTEGRATION
Status: active
Topics:
    - devctl
    - log-parser
    - tui
    - architecture
DocType: analysis
Intent: long-term
Owners: []
RelatedFiles:
    - Path: devctl/cmd/log-parse/main.go
      Note: log-parse CLI entrypoint
    - Path: devctl/pkg/logjs/helpers.js
      Note: JS helper API available to modules
    - Path: devctl/pkg/logjs/module.go
      Note: module lifecycle
    - Path: devctl/pkg/logjs/types.go
      Note: logjs event schema and error record types
    - Path: devctl/pkg/tui/models/eventlog_model.go
      Note: current event log view limits and filters
    - Path: devctl/pkg/tui/topics.go
      Note: domain and UI topic/type definitions
    - Path: devctl/pkg/tui/transform.go
      Note: domain-to-UI transformer and message routing
ExternalSources: []
Summary: Design study for integrating log-parse (logjs) into devctl's CLI workflow and TUI surfaces.
LastUpdated: 2026-01-08T12:41:56.909874298-05:00
WhatFor: Provide architectural options, UX affordances, and API mappings for log-parser in devctl.
WhenToUse: Use when planning how devctl should parse and visualize logs with log-parse modules.
---


# Log-parser integration study

## Executive summary

log-parse already exists in devctl as a standalone CLI (`cmd/log-parse`) built on the `pkg/logjs` Go API. It provides synchronous JavaScript hooks (`parse`, `filter`, `transform`) that emit normalized `logjs.Event` records tagged by module and mapped to NDJSON. devctl’s TUI already has a structured messaging bus (Watermill topics + `tui.Envelope`) and views that accept event logs and stream events. The integration opportunity is therefore to surface log-parse output as either devctl domain events or as plugin-backed stream events that are already TUI-ready.

This study outlines three primary integration architectures (inline devctl parsing, plugin-driven stream parsing, and supervisor-level parsing), plus a smaller “offline analysis” mode. For each option, it proposes CLI and TUI affordances, maps data into existing event/message structures, and highlights risks around volume, sandboxing, and user workflow.

## Current system overview

### log-parse (logjs) runtime and API

- **CLI entrypoint**: `devctl/cmd/log-parse/main.go`
- **Go API**: `logjs.LoadFromFile`, `logjs.LoadFanoutFromFiles`, `Module.ProcessLine`, `Options{HookTimeout: "50ms"}`.
- **Event schema**: `logjs.Event` in `devctl/pkg/logjs/types.go` with `timestamp`, `level`, `message`, `fields`, `tags`, `source`, `raw`, `lineNumber`.
- **Fan-out**: `Fanout.ProcessLine` runs all modules, injects `_module` and `_tag` fields, and merges outputs.
- **Errors**: `ErrorRecord` with hook name, timeout flag, and raw line.
- **JS helpers**: `helpers.js` (e.g., `log.parseJSON`, `log.parseLogfmt`, `log.createMultilineBuffer`, `log.addTag`, `log.parseTimestamp`).

Key observations from code:

- Each module registers once (`register({ name, parse, ... })`).
- `parse` is required; `filter`, `transform`, `init`, `shutdown`, `onError` are optional.
- The runtime is synchronous and deterministic; timeouts use `goja.Runtime.Interrupt`.
- Normalization is centralized in `Module.normalizeEvent`, and tags are injected in `fanout.injectTag`.

### devctl workflow and logging sources

- Service logs are stored in `.devctl/logs` per `state.LogsDir` and `state.ServiceRecord.StdoutLog/StderrLog`.
- The `devctl logs` command (`devctl/cmd/devctl/cmds/logs.go`) reads files, tails, or follows them.
- devctl uses embedded docs via `pkg/doc` (e.g., `pkg/doc/topics/log-parse-guide.md`) and a help system.

### devctl messaging structure (TUI)

Message pipeline:

```
Domain producers -> devctl.events (Envelope) -> transform -> devctl.ui.msgs -> TUI models
```

Key structures and topics:

- Topics: `devctl.events`, `devctl.ui.msgs`, `devctl.ui.actions` (`pkg/tui/topics.go`).
- Envelope: `tui.Envelope{Type, Payload}` (`pkg/tui/envelope.go`).
- Domain-to-UI transformer: `RegisterDomainToUITransformer` (`pkg/tui/transform.go`).
- TUI views: `EventLogModel` handles event log lines with filters and stats; `StreamsModel` handles stream events.

The existing event log model is intentionally bounded (200 lines) and includes filtering by service and level (`pkg/tui/models/eventlog_model.go`). Stream events are higher-volume and displayed in a dedicated view.

## Integration goals

Architecturally:

- Reuse the existing log-parse module contract and Go API rather than inventing new parsing DSLs.
- Fit log-parse output into existing devctl messaging and stream semantics.
- Support both live log streams and offline parsing over stored logs.
- Provide an extensible configuration path for modules (per repo, per service, and per use case).

User experience goals:

- Make structured log parsing feel like a core devctl capability rather than a separate tool.
- Allow quick “turn on parsing” defaults with progressive disclosure for advanced module setup.
- Offer a TUI view that helps users correlate raw logs with parsed events and module tags.
- Provide guardrails (rate limiting, filters, targeted modules) to avoid overwhelming the UI.

## Option A: Inline log-parse inside devctl logs workflow

### Concept

Extend `devctl logs` (or a sibling command) to run logjs modules directly while reading/following log files. This uses the existing file-based log paths stored in state and produces structured events inside devctl without any plugin protocol.

### Data flow

```
[.devctl/logs/*.log] --read/follow--> logjs.Fanout
  -> logjs.Event --> devctl.events (DomainTypeLogParsed)
  -> transform --> TUI (parsed log view + event log)
```

### CLI affordances

- `devctl logs --service api --parse --modules-dir .devctl/log-parse`
- `devctl logs --parse --module parsers/errors.js --format pretty`
- `devctl logs --parse --errors errors.ndjson` (mirror log-parse CLI)

### TUI affordances

- Add a “Parsed Logs” tab next to “Live Events”.
- Add a module tag filter (auto from `_tag`) and show `_module` in the line prefix.
- Toggle “show raw line” (expand/collapse per line) for correlation.

### Pseudocode

```go
fanout, _ := logjs.LoadFanoutFromFiles(ctx, modulePaths, logjs.Options{HookTimeout: "50ms"})
for line := range tailer.Lines() {
  events, errs, _ := fanout.ProcessLine(ctx, line.Text, line.Source, line.Number)
  publishParsedEvents(events)
  publishParseErrors(errs)
}
```

### Pros

- Minimal moving parts: no plugin processes, no protocol changes.
- Reuses existing log-parse CLI semantics directly.
- Fast path for users who already have log-parse modules.

### Cons / risks

- Ties parsing to log file paths; cannot parse stream events from plugins unless they also write files.
- The devctl process now owns JS execution; sandbox and timeout defaults must be enforced.
- High-volume parsing might overwhelm the TUI unless throttled.

## Option B: Log-parse as a devctl stream plugin (recommended for flexibility)

### Concept

Expose a `logs.parse` or `logparse.stream` op from a dedicated plugin (or a built-in runtime module) that uses logjs internally. devctl starts a stream using `runtime.Client.StartStream`, and parsed events are emitted as protocol events with structured fields.

### Data flow

```
log source -> plugin stream -> protocol.Event{event:"log.parsed"}
  -> devctl stream runner -> DomainTypeStreamEvent
  -> TUI StreamsModel + optional EventLogModel summary
```

### CLI/TUI affordances

- CLI: `devctl stream start logs.parse --service api --modules-dir .devctl/log-parse`
- TUI: a “Log Streams” panel showing parsed event tags and module names.
- TUI: an “Attach” action to start parsing for the selected service.

### Pseudocode (devctl side)

```go
streamID, events, _ := client.StartStream(ctx, "logs.parse", input)
for ev := range events {
  if ev.Event == "log.parsed" { /* UI routing */ }
}
```

### Pros

- Works for any log source the plugin can access (files, tail, stdin, container logs).
- Respects devctl stream semantics already supported by the TUI.
- Decouples JS execution from the main devctl process if needed.

### Cons / risks

- Requires additional plugin packaging and protocol validation.
- Needs a schema for `protocol.Event.Fields` to carry parsed payloads.

## Option C: Supervisor-level parsing (structured logs as first-class events)

### Concept

Integrate log-parse directly into the supervise layer so that as services run, their stdout/stderr is parsed immediately and the structured events are emitted to the devctl event bus. This turns parsing into a continuous, always-on feature.

### Data flow

```
service stdout/stderr -> supervisor -> logjs -> devctl.events
  -> UI summary + parsed logs view
```

### Pros

- Log parsing becomes a core platform capability; no manual setup for each `devctl logs` call.
- Enables TUI views that display structured events from the moment services start.

### Cons / risks

- Always-on cost even if the user does not want parsing.
- Requires configuration management to map services to modules.
- Tight coupling with supervise lifecycle and potential backpressure concerns.

## Option D: Offline analysis mode (batch parsing)

### Concept

Provide a `devctl log-parse` subcommand that wraps the existing log-parse CLI but integrates with devctl’s state and log paths. This would be a thin wrapper for “parse last run logs” rather than live parsing.

### Pros

- Low risk, easy to deliver, good for quick explorations.
- Serves as a stepping stone for more interactive integration.

### Cons

- Not real-time; no TUI integration unless used as a data source.

## Data model mapping

### logjs.Event -> devctl domain event

Proposed domain payload (new type):

```json
{
  "source": "api",
  "timestamp": "2026-01-08T12:00:00Z",
  "level": "ERROR",
  "message": "db timeout",
  "raw": "...",
  "tags": ["errors"],
  "fields": {"_module":"db-errors","_tag":"errors","trace_id":"abc"},
  "line_number": 123
}
```

Mapping notes:

- `EventLogEntry` expects a short `Text` string; it can be filled with `message` and a short tag prefix (e.g., `[errors] db timeout`).
- Structured data should remain in a parsed log view, not the global event log, to avoid overflow.
- Use `_module` and `_tag` for filter chips in the TUI (modules are highly actionable in troubleshooting).

### logjs.ErrorRecord -> devctl event

- Display as warnings in the event log (`LogLevelWarn`) but keep the raw data in a separate error panel.
- Include `hook` and `timeout` flags to allow filtering.

## TUI affordances (user-centric)

### New screens / panes

1) **Parsed Logs view**

```
┌ Parsed Logs ─────────────────────────────── [esc] back ┐
│ [/] filter  [t] tag  [m] module  [r] raw  [p] pause   │
│                                                      │
│ 12:01:02.431 [api] [errors] db timeout (trace=abc)   │
│ 12:01:03.010 [api] [metrics] cache_hit=0.97          │
│ 12:01:04.552 [api] [security] deny ip=10.0.0.3       │
│                                                      │
│ tags: errors ● metrics ○ security ○                  │
│ modules: db-errors ● http-metrics ● access-deny ○    │
│ stats: 140 ev  (18/sec)  dropped 9                   │
└──────────────────────────────────────────────────────┘
```

2) **Stream Events enhancement**

- Group by `StreamKey` and render the `protocol.Event.Fields` as compact JSON when `message` is empty.
- Add a toggle to render parsed log events as a special stream event (colored by level).

### Interaction model

- **Attach parsing to a service**: from dashboard, hit `[p] parse logs` to start a stream or inline parser.
- **Toggle raw vs. parsed**: `[r]` to show raw line in a side pane.
- **Module/tag filters**: re-use the existing filter style from `EventLogModel` but add a “tag” bar.
- **Rate limiting**: if `events/sec` is too high, display a notice and show only sampled events.

## Architecture comparison (summary)

- **Inline (Option A)**: fastest to implement, constrained to file logs, simplest for CLI.
- **Plugin stream (Option B)**: most flexible, aligns with existing stream UI, requires protocol payload design.
- **Supervisor-level (Option C)**: most seamless UX, highest coupling and cost.
- **Offline analysis (Option D)**: lowest risk, limited UX impact.

## Recommended path (phased)

1) **Phase 1: Option D + A hybrid**
   - Add `devctl log-parse` wrapper that accepts `--service` and auto-resolves log paths.
   - Add `devctl logs --parse` that shells into logjs (no TUI yet).

2) **Phase 2: TUI Parsed Logs view**
   - Define a new domain type (e.g., `log.parsed`) and UI type (e.g., `tui.log.parsed`).
   - Use the same rendering patterns as `EventLogModel` but with tags/modules filters.

3) **Phase 3: Stream-based parsing**
   - Implement a log-parse stream op and wire it to the TUI stream model.
   - Provide a “start parsing” UI action from the dashboard/service view.

## Detailed API references

- `logjs.LoadFanoutFromFiles(ctx, []string, logjs.Options{HookTimeout: "50ms"})`
- `logjs.Module.ProcessLine(ctx, line, source, lineNumber)`
- `logjs.Event` and `logjs.ErrorRecord` types in `pkg/logjs/types.go`
- TUI domain/ui topics and envelopes in `pkg/tui/topics.go` + `pkg/tui/envelope.go`
- Domain-to-UI transformer in `pkg/tui/transform.go`

## Open questions

- Where should module configs live? Options: `.devctl/log-parse/`, `devctl/config`, or project-root `.log-parse/`.
- Should parsing be opt-in per service or configured globally?
- How do we scope permissions for JS modules (see sandbox notes in MO-008-REQUIRE-SANDBOX)?

## Appendix: Diagram of current vs. integrated flow

Current:

```
log-parse CLI
  stdin/file -> logjs -> NDJSON
```

Proposed (stream integration):

```
service logs -> plugin/logjs -> protocol.Event{event:"log.parsed"}
  -> devctl stream runner -> DomainTypeStreamEvent
  -> TUI Streams view + Parsed Logs view
```
