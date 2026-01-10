---
Title: Parsed logs UX spec
Ticket: MO-016-LOGPARSER-DEVCTL-INTEGRATION
Status: active
Topics:
    - devctl
    - log-parser
    - tui
    - architecture
DocType: design-doc
Intent: long-term
Owners: []
RelatedFiles:
    - Path: devctl/pkg/logjs/types.go
      Note: parsed event schema used in payload mapping
    - Path: devctl/pkg/tui/models/eventlog_model.go
      Note: existing event log filters and stats behavior to mirror
    - Path: devctl/pkg/tui/transform.go
      Note: domain-to-UI routing constraints for parsed log events
ExternalSources: []
Summary: Exhaustive UX spec for parsed log features and their integration into devctl workflows and messaging.
LastUpdated: 2026-01-08T13:12:00-05:00
WhatFor: Define feature-level UX and integration requirements for parsed logs in devctl.
WhenToUse: Use when implementing parsed log views, commands, and stream integrations.
---


# Parsed logs UX spec

## Executive summary

This spec defines how parsed log data (produced by logjs modules) integrates into devctl workflows and the TUI, focusing on behavior, feature scope, data routing, and user interactions rather than visual styling. It assumes log-parse remains the canonical module contract and that parsed events can surface via devctl domain events or stream events. The design covers command entry points, session lifecycle, data contracts, and TUI affordances for filtering, correlation, and troubleshooting.

## Problem statement

Developers need a first-class way to parse noisy service logs into structured events inside devctl so that debugging and monitoring workflows are faster and more consistent. Today, log-parse exists as a standalone CLI and devctl has a separate TUI event system. There is no integrated workflow to:

- Attach parsing to a running service with a simple UI action.
- Navigate parsed output alongside raw logs and system events.
- Filter by module tags or derived fields in the same interface.
- Stream high-volume parsed events without overwhelming the global event log.

## Proposed solution

Introduce a parsed logs feature set that extends devctl CLI and TUI with a unified parsing lifecycle:

- **CLI**: add wrapper commands to run logjs against service logs (`devctl log-parse` and `devctl logs --parse`).
- **TUI**: add a Parsed Logs view that displays parsed events and supports filters by service, tag, module, and severity.
- **Integration**: define a new domain event type (or stream event schema) that carries parsed events and error records without polluting the global event log.
- **Controls**: allow users to start/stop parsing per service and per module set, and persist last-used settings.

## Scope

In scope:

- Parsed events from logjs (module-generated) for service logs.
- Filtering and navigation of parsed events.
- Error reporting for parser errors/timeouts.
- Support for multiple modules (fanout) with tags.

Out of scope (initially):

- Full-text search across historical parsed logs.
- Cross-repo module registry or shared marketplace.
- Complex visualization dashboards (graphs, charts).

## User experience model

### Entry points

1) CLI: quick one-shot parsing

```
$ devctl log-parse --service api --modules-dir .devctl/log-parse
```

2) CLI: parse while tailing logs

```
$ devctl logs --service api --follow --parse --module parsers/errors.js
```

3) TUI: attach parsing to a running service

```
Dashboard -> select service -> [p] parse logs
```

### Session lifecycle

- **Start**: user requests parsing with module set and source service/log path.
- **Run**: parsed events stream into Parsed Logs view; error events are captured and shown in a lightweight error ribbon.
- **Stop**: user stops parsing manually or stream ends; last settings persist for quick restart.

### Personas and tasks

- **Operator**: wants to quickly see errors and patterns; uses tag/module filters and severity filters.
- **Developer**: wants correlation with raw logs; uses raw view toggles and line number references.
- **Maintainer**: wants to validate parser behavior; inspects error records and module stats.

## Parsed logs view behavior

### Core behaviors

- Show parsed events in chronological order with source, level, tag, message, and optional key fields.
- Support a “raw line” toggle to show original log text alongside parsed data.
- Provide per-module and per-tag filters that are independent of service filters.
- Offer a pause toggle that buffers events and can resume (matching EventLogModel semantics).

### Information hierarchy

Each event row should carry these core attributes:

- Timestamp (from parsed event timestamp if present; fallback to ingest time).
- Service/source label (from state/service or log source).
- Tag (from `_tag` or `tags` array) and module name (`_module`).
- Level (mapped to INFO/WARN/ERROR/DEBUG).
- Message (from event.message or a derived summary).

### Interaction details

Key actions (suggested key bindings):

- `[p]` pause/resume parsed event stream
- `[/]` text filter (applies to message and selected fields)
- `[t]` toggle tag filter menu
- `[m]` toggle module filter menu
- `[r]` toggle raw line detail
- `[e]` open parser errors panel
- `[space]` quick filter on current service

### Filters

- **Service filter**: same as EventLogModel, uses service names from state.
- **Tag filter**: derived from `Event.tags` and `_tag` fields; supports multi-select.
- **Module filter**: derived from `_module` field.
- **Level filter**: same as EventLogModel (DEBUG/INFO/WARN/ERROR).

### Error handling UX

- Parser errors (from `ErrorRecord`) are surfaced as a compact error ribbon that can be expanded.
- Timeouts and hook errors should be distinguishable by icon or label.
- Error detail view includes hook name, module, and raw line snippet.

### Stats and rate limiting

- Show events/sec, dropped count, and buffered count.
- If a max rate is exceeded, surface a “sampling mode” indicator and show only sampled events.

## Integration with devctl messaging

### Proposed domain event type

Option 1: add a dedicated domain type:

```
DomainTypeLogParsed = "log.parsed"
UITypeLogParsed = "tui.log.parsed"
```

Payload (example):

```json
{
  "at": "2026-01-08T12:01:02.431Z",
  "source": "api",
  "level": "ERROR",
  "message": "db timeout",
  "raw": "...",
  "tags": ["errors"],
  "fields": {"_module":"db-errors","_tag":"errors","trace_id":"abc"},
  "line_number": 123
}
```

Option 2: reuse stream events and encode parsed logs in `protocol.Event`:

- `event = "log.parsed"`
- `message` and `fields` carry structured data.

### Routing behavior

- Parsed logs should be routed to the Parsed Logs view, not the global EventLogModel.
- A minimal summary (e.g., “parsed event error: db timeout”) can be optionally mirrored into the global event log when severity >= WARN.

## Feature configuration

### Module sources

- Local modules directory (per repo): `.devctl/log-parse/` (default)
- Custom paths via CLI flags `--module` and `--modules-dir`

### Defaults

- Default module set can be inferred from config (e.g., `.devctl/config.yml`).
- Default JS timeout: 50ms (align with log-parse guide examples).
- Default max event buffer: 200 (align with EventLogModel) with a parsed-logs specific override.

## Design decisions

- **Separate Parsed Logs view**: avoids overwhelming the global event log.
- **Tag/module filters**: leverage logjs fan-out metadata for navigation.
- **Stream-friendly payloads**: keep `fields` opaque and avoid custom struct per module.
- **Opt-in parsing**: keep parsing off by default to control cost and noise.

## Alternatives considered

- **Merge parsed logs into EventLogModel**: rejected due to volume and loss of structured fields.
- **Only CLI-based parsing**: insufficient for TUI and ongoing workflows.
- **Always-on parsing in supervisor**: too heavy for default behavior, but possible future mode.

## Implementation plan (feature-level)

1) Add CLI wrapper commands for log-parse integration with devctl state.
2) Define new domain/UI types or stream schema for parsed events.
3) Implement Parsed Logs model + view with filtering and error panel.
4) Add TUI actions to start/stop parsing per service.
5) Persist last-used module configuration.

## Acceptance criteria

CLI

- `devctl log-parse --service <name>` resolves the correct log path from state and emits parsed NDJSON.
- `devctl logs --parse` supports `--module`/`--modules-dir` and preserves existing `--follow`/`--tail` behavior.
- Parser error output can be captured with `--errors` and does not corrupt standard output formatting.

Parsing lifecycle

- Parsed logs can be started and stopped per service from the TUI with a clear state indicator.
- The last-used module configuration persists across restarts (per repo) and is reused by default.
- Parsing is opt-in; no service is parsed unless explicitly enabled.

Parsed Logs view

- Parsed events appear in chronological order with timestamp, source, level, message, tag, and module.
- Filters are available for service, tag, module, level, and free-text query.
- Raw line toggle displays the original log text adjacent to the parsed event.
- Pause/resume buffers events and reports dropped counts when buffer is exceeded.

Error handling

- Parser errors appear in an error ribbon/panel with module, hook, and raw line context.
- Hook timeouts are distinguishable from other errors.

Integration and routing

- Parsed events do not flood the global EventLogModel by default.
- Severity >= WARN can optionally emit a summary into the global event log without duplicating full payloads.
- Domain/stream envelope payloads are valid JSON and pass existing transformer validation.

Performance and rate limiting

- When event rate exceeds a configurable threshold, sampling activates and is surfaced in the UI.
- Events/sec and dropped counts are visible in the Parsed Logs view.

## Open questions

- How should module configuration be persisted (state file, config file, or per-service metadata)?
- Should parsing be supported for stdin/adhoc logs in the TUI (beyond services)?
- What is the maximum tolerated event rate before forced sampling?

## ASCII flow diagram

```
Service logs -> logjs fanout -> parsed events -> devctl.events/log.parsed
  -> UI transformer -> Parsed Logs view
  -> optional summary -> EventLogModel
```
