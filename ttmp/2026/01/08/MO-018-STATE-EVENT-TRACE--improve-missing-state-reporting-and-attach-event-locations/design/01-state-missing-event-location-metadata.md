---
Title: State Missing & Event Location Metadata
Ticket: MO-018-STATE-EVENT-TRACE
Status: active
Topics:
    - devctl
    - tui
    - observability
DocType: design
Intent: long-term
Owners: []
RelatedFiles:
    - Path: pkg/tui/msgs.go
      Note: EventLogEntry definitions (candidate for source metadata).
    - Path: pkg/tui/transform.go
      Note: Central place for state snapshot event logging.
ExternalSources: []
Summary: Normalize missing-state event output and add code-location metadata to UI events for easier debugging.
LastUpdated: 2026-01-08T15:21:39-05:00
WhatFor: Define the policy and data shape for missing-state events and event location metadata in the TUI pipeline.
WhenToUse: Use when updating UI event logging or adding observability fields to domain/UI events.
---


# State Missing & Event Location Metadata

## Goal

Make `state: missing` a first-class, non-warning UI event and enrich UI events with code-location metadata to aid debugging.

## Problem Statement

- `state: missing` is currently printed by the UI transformer but is treated inconsistently and sometimes appears with warning semantics.
- When UI events are emitted, there is no easy way to see where they originated in code, which makes debugging noisy logs harder.

## Requirements

1) `state: missing` should be emitted as a normal informational event, not a warning.
2) UI events should include code location metadata (file + line, or a compact stack summary).
3) The change should avoid heavy overhead in hot paths.

## Proposed Design

### Missing State Event Normalization

- Centralize state snapshot event formatting in `pkg/tui/transform.go`.
- Explicitly set `LogLevelInfo` for the "missing" case.
- Keep "error" only for `snap.Error != ""`.
- Ensure output text remains stable ("state: missing") for downstream filters.

### Event Location Metadata

Add optional metadata fields to `EventLogEntry` (or a wrapper) so events can carry source location data without changing every call site immediately.

Suggested fields:

- `SourceFile` (string)
- `SourceLine` (int)
- `SourceFunc` (string, optional)

Implementation approach:

- Provide a helper in `pkg/tui` (e.g., `NewEventLogEntryWithSource`) that captures caller info using `runtime.Caller`.
- Use the helper in the transformer where centralized event emission happens.
- Keep location optional so callers can opt in gradually.

## Data Shape

Example JSON (UI event log entry):

```
{
  "at": "2026-01-08T15:21:39Z",
  "source": "system",
  "level": "info",
  "text": "state: missing",
  "source_file": "pkg/tui/transform.go",
  "source_line": 52,
  "source_func": "RegisterDomainToUITransformer"
}
```

## Scope

- Update state snapshot event handling in `pkg/tui/transform.go`.
- Introduce source metadata fields on UI event log entries.
- Use metadata in centralized event publishing paths first.

## Risks

- Adding fields may affect consumers that assume a fixed JSON shape.
- Capturing call-site info has a minor runtime cost; keep it minimal and avoid hot per-event loops if possible.

## Validation

- Verify `state: missing` appears at info level in the UI event log.
- Confirm events include source file/line in debug output.
- Ensure no panic or performance regressions in high-volume logs.

## Open Questions

- Should source metadata be always-on or gated by a debug flag?
- Should event location data be added to domain events as well, or only UI logs?
