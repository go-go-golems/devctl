---
Title: Diary
Ticket: MO-016-LOGPARSER-DEVCTL-INTEGRATION
Status: active
Topics:
    - devctl
    - log-parser
    - tui
    - architecture
DocType: reference
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: ""
LastUpdated: 2026-01-08T12:41:59.393976256-05:00
WhatFor: ""
WhenToUse: ""
---

# Diary

## Goal

Capture the investigation and design study for integrating log-parser into the main devctl workflow and TUI, including the key code references and UX implications.

## Step 1: Create ticket workspace and map initial entry points

I created a dedicated ticket workspace for the log-parser integration study and started by mapping the key entry points: the log-parse CLI, the embedded docs, and the devctl TUI messaging system. This established the scope and gave me a list of files to inspect for how log parsing currently works and how devctl surfaces events.

I also noted the existing log parsing documentation in `pkg/doc/topics/log-parse-guide.md` so I could align future design notes with the established module contract and helper API.

### What I did
- Created ticket MO-016-LOGPARSER-DEVCTL-INTEGRATION with analysis + diary docs.
- Scanned for log-parse-related files and devctl TUI messaging entry points.

### Why
- Establish a grounded base of source references before designing integration options.

### What worked
- Found the log-parse CLI implementation and logjs package quickly.
- Identified the TUI event bus and envelope mapping as the main messaging backbone.

### What didn't work
- N/A

### What I learned
- devctl embeds docs from `pkg/doc/topics` and already ships a full log-parse guide.

### What was tricky to build
- N/A

### What warrants a second pair of eyes
- N/A

### What should be done in the future
- N/A

### Code review instructions
- Where to start: no code changes in this step.
- How to validate: N/A

### Technical details
- Entry points noted: `devctl/cmd/log-parse/main.go`, `devctl/pkg/logjs/*`, `devctl/pkg/tui/*`.

## Step 2: Inspect log-parse runtime and helper APIs

I reviewed the log-parse CLI and the `pkg/logjs` implementation to understand the module lifecycle, fan-out behavior, event normalization rules, and error handling. This clarified which integration surfaces are available (Go API vs. CLI), what outputs are produced, and how tags are injected.

I also skimmed the JavaScript helper library so I can highlight capabilities (logfmt parsing, regex helpers, multiline buffering) in the design study and ensure TUI affordances match the kinds of structured events log-parse can emit.

### What I did
- Read `devctl/cmd/log-parse/main.go` for CLI flags and streaming loop.
- Read `devctl/pkg/logjs/module.go`, `fanout.go`, `types.go` to capture module contract and event schema.
- Read `devctl/pkg/logjs/helpers.js` to capture JS helper surface.

### Why
- The integration design should mirror existing log-parse semantics rather than inventing a new pipeline.

### What worked
- The Go API already supports multi-module fanout and error records, which fits devctl stream and TUI needs.

### What didn't work
- N/A

### What I learned
- log-parse uses synchronous hooks with optional per-hook timeout and returns normalized events containing tags and fields like `_module` and `_tag`.

### What was tricky to build
- N/A

### What warrants a second pair of eyes
- N/A

### What should be done in the future
- N/A

### Code review instructions
- Where to start: `devctl/pkg/logjs/module.go` for module contract, `devctl/pkg/logjs/fanout.go` for multi-module fanout.
- How to validate: N/A

### Technical details
- Core APIs: `logjs.LoadFanoutFromFiles`, `Module.ProcessLine`, `Options{HookTimeout: "50ms"}`.

## Step 3: Inspect devctl messaging + TUI event surfaces

I walked through the devctl TUI bus, domain envelope types, and the event log model to understand how new parsed log events could be wired in. This reveals that devctl already has a structured domain-to-UI transform path and a dedicated stream event view that can host log-parser output if we emit the right envelopes.

I also checked how devctl manages log files and how the CLI `logs` command reads them, to inform where log-parse should connect (offline tail, follow, or plugin streams).

### What I did
- Read `devctl/pkg/tui/bus.go`, `topics.go`, `envelope.go`, and `transform.go` for the message pipeline.
- Read `devctl/pkg/tui/models/eventlog_model.go` to understand filtering, stats, and UI limits.
- Reviewed `devctl/pkg/runtime/client.go` and `devctl/cmd/devctl/cmds/logs.go` for stream + log file mechanics.

### Why
- The design study needs to recommend integration points that align with existing bus semantics and UI affordances.

### What worked
- The TUI already has structured event views and stream event handling that can be reused for parsed log streams.

### What didn't work
- N/A

### What I learned
- The message flow is `devctl.events` (domain envelopes) -> `devctl.ui.msgs` (UI envelopes), and EventLogModel is capped at 200 lines with service and level filters.

### What was tricky to build
- N/A

### What warrants a second pair of eyes
- N/A

### What should be done in the future
- N/A

### Code review instructions
- Where to start: `devctl/pkg/tui/transform.go` and `devctl/pkg/tui/models/eventlog_model.go`.
- How to validate: N/A

### Technical details
- Topics: `devctl.events`, `devctl.ui.msgs`, `devctl.ui.actions`.
- Domain/UI types: `DomainTypeStreamEvent`, `UITypeStreamEvent`, `UITypeEventAppend`.

## Step 4: Draft integration study and UX design options

I wrote the integration study document, focusing on architecture options, data flow diagrams, and TUI affordances that match the current devctl message bus. The report also maps logjs event structures into devctl domain and UI events, so future implementation can stay aligned with existing APIs and UI constraints.

I emphasized both the low-effort CLI extensions and the longer-term stream-driven approach, including recommended phases and open questions for module storage, opt-in behavior, and sandboxing.

### What I did
- Authored the analysis report with multiple integration options, pros/cons, and phased recommendations.
- Included pseudocode, ASCII diagrams, and TUI screen sketches to illustrate user experience paths.

### Why
- Provide a durable design reference before any implementation work begins.
### What worked
- The existing logjs API and TUI message bus cleanly support a log-parsed event flow without major new primitives.

### What didn't work
- N/A

### What I learned
- The current event log view limits and filters strongly influence how parsed logs should be presented (separate view + tag filters).

### What was tricky to build
- N/A

### What warrants a second pair of eyes
- Ensure the proposed message types and UI additions align with the existing envelope + topic conventions.
### What should be done in the future
- N/A

### Code review instructions
- Where to start: `devctl/ttmp/2026/01/08/MO-016-LOGPARSER-DEVCTL-INTEGRATION--integrate-log-parser-into-devctl-workflow-tui/analysis/01-log-parser-integration-study.md`.
- How to validate: N/A

### Technical details
- Integration options: inline logjs, plugin stream, supervisor-level parsing, offline analysis mode.

## Step 5: Convert recommendations into tasks and UX spec

I translated the study recommendations into concrete implementation tasks and wrote a dedicated parsed logs UX spec. The UX spec is feature-focused, specifying behavior, integration routing, and configuration without locking in visual design.

This establishes a clear execution plan and a full-feature UX target, making it easier to slice implementation work by phase while keeping the integration constraints explicit.

### What I did
- Authored the parsed logs UX spec design doc with feature-level behaviors and integration mapping.
- Updated the ticket tasks list with actionable implementation steps.

### Why
- Provide an executable plan and a detailed UX contract before coding.

### What worked
- The existing TUI event log model provides a strong baseline for filters, stats, and pause behavior.

### What didn't work
- N/A

### What I learned
- The UX can remain look-agnostic while still being explicit about routing, controls, and data contracts.

### What was tricky to build
- N/A

### What warrants a second pair of eyes
- Confirm the proposed domain/stream event mapping aligns with protocol constraints and TUI routing.

### What should be done in the future
- N/A

### Code review instructions
- Where to start: `devctl/ttmp/2026/01/08/MO-016-LOGPARSER-DEVCTL-INTEGRATION--integrate-log-parser-into-devctl-workflow-tui/design-doc/01-parsed-logs-ux-spec.md` and `devctl/ttmp/2026/01/08/MO-016-LOGPARSER-DEVCTL-INTEGRATION--integrate-log-parser-into-devctl-workflow-tui/tasks.md`.
- How to validate: N/A

### Technical details
- UX scope: parsed logs view, filters, error ribbon, rate limiting, CLI entry points.

## Step 6: Add acceptance criteria to UX spec

I added explicit acceptance criteria to the parsed logs UX spec to make the implementation expectations testable and easy to verify. The criteria cover CLI behavior, parsing lifecycle, TUI view features, error handling, routing, and rate limiting.

This makes it easier to validate each phase without relying on subjective interpretation of the design intent.

### What I did
- Added a structured acceptance criteria section to the parsed logs UX spec.

### Why
- Ensure the spec has verifiable outcomes for each major feature area.

### What worked
- The criteria map cleanly onto the existing integration plan and message routing approach.

### What didn't work
- N/A

### What I learned
- Tight criteria help keep the UX spec implementation-focused without committing to visuals.

### What was tricky to build
- N/A

### What warrants a second pair of eyes
- Confirm the acceptance criteria are complete and align with desired rollout phases.

### What should be done in the future
- N/A

### Code review instructions
- Where to start: `devctl/ttmp/2026/01/08/MO-016-LOGPARSER-DEVCTL-INTEGRATION--integrate-log-parser-into-devctl-workflow-tui/design-doc/01-parsed-logs-ux-spec.md`.
- How to validate: N/A

### Technical details
- Criteria categories: CLI, parsing lifecycle, Parsed Logs view, error handling, routing, performance.
