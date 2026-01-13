---
Title: Active Ticket Status Report
Ticket: MO-018-ADD-DEVCTL-README
Status: active
Topics:
    - devctl
    - documentation
    - readme
DocType: analysis
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: "Snapshot of active tickets with closure, staleness, and devctl implementation notes."
LastUpdated: 2026-01-13T17:06:45-05:00
WhatFor: "Capture ticket triage and implementation status based on `docmgr list tickets --status active` plus diary/source review."
WhenToUse: "Use when deciding which tickets to close, refresh, or re-scope, and when tracking remaining work."
---

# Active Ticket Status Report

## Snapshot

- Date: 2026-01-13
- Source: `docmgr list tickets --status active` (post-closure run)
- Criteria:
  - Close candidates: 0 open tasks in the list output.
  - Out of date: last updated on or before 2026-01-08 (>=5 days old).
  - Devctl relevance: based on diary/index review of ticket goals and scope (not topics).
  - Implementation status: based on diary steps plus source file inspection.

## Overall picture

- 10 active tickets total.
- 8 active tickets appear devctl-related based on diary/index review; 2 do not.
- 0 close candidates in the active list.
- 9 tickets look stale based on last update date.

## Closed this pass

- MO-013-PORT-STARTDEV — closed (tasks complete; plugin/docs validated in source).
- MO-010-DEVCTL-CLEANUP-PASS — closed (cleanup pass implemented; tasks complete).
- MO-015-CREATE-PLUGIN-HOST-DOCUMENTATION — closed (docs complete; tasks checked).
- RUNTIME-PLUGIN-INTROSPECTION — closed (introspection implemented; manual TUI validation shows cap: ok).
- MO-017-TUI-CONTEXT-LIFETIME-SCOPING — closed per request (validation task still open).
- MO-015-DEVCTL-PLAN-DEBUG-TRACE — closed per request (no implementation yet).
- MO-011-IMPLEMENT-STREAMS — closed per request (optional stream stop task still open).
- MO-008-IMPROVE-TUI-LOOKS — closed per request (tasks list not populated).
- MO-008-REQUIRE-SANDBOX — closed per request (sandboxing not implemented).
- MO-007-LOG-PARSER — closed per request (devctl logs integration still open).
- MO-006-DEVCTL-TUI — closed per request (milestones still open).

## Close candidates (0 open tasks)

- None in the active list after closing MO-015-CREATE-PLUGIN-HOST-DOCUMENTATION.

## Out-of-date (last updated on or before 2026-01-08)

- MO-018-PIPELINE-VIEW-STUCK-STATE — updated 2026-01-08.
- MO-018-STATE-EVENT-TRACE — updated 2026-01-08.
- MO-016-LOGPARSER-DEVCTL-INTEGRATION — updated 2026-01-08.
- STREAMS-TUI — updated 2026-01-08.
- MO-014-IMPROVE-PIPELINE-TUI — updated 2026-01-08.
- MO-012-PORT-CMDS-TO-GLAZED — updated 2026-01-08.
- MO-009-TUI-COMPLETE-FEATURES — updated 2026-01-08.
- MO-005-IMPROVE-STARTDEV — updated 2026-01-06 (non-devctl).

## Devctl relevance (active tickets)

### Devctl-related

- MO-018-ADD-DEVCTL-README
- MO-018-PIPELINE-VIEW-STUCK-STATE
- MO-018-STATE-EVENT-TRACE
- MO-016-LOGPARSER-DEVCTL-INTEGRATION
- STREAMS-TUI
- MO-014-IMPROVE-PIPELINE-TUI
- MO-012-PORT-CMDS-TO-GLAZED
- MO-009-TUI-COMPLETE-FEATURES

### Not devctl-related

- MO-017-DIARY-TAIL-APP — standalone diary tailing webapp.
- MO-005-IMPROVE-STARTDEV — moments startdev.sh analysis; devctl is only a future replacement target.

## Closed per request (not fully implemented)

- MO-017-TUI-CONTEXT-LIFETIME-SCOPING — validation task left open.
- MO-015-DEVCTL-PLAN-DEBUG-TRACE — no implementation yet.
- MO-011-IMPLEMENT-STREAMS — optional stream stop task left open.
- MO-008-IMPROVE-TUI-LOOKS — tasks list not populated.
- MO-008-REQUIRE-SANDBOX — sandboxing not implemented.
- MO-007-LOG-PARSER — devctl logs integration still open.
- MO-006-DEVCTL-TUI — milestones still open.

## Devctl ticket implementation review (diary + source)

### MO-018-ADD-DEVCTL-README — Status: not implemented (scaffolding only)

Evidence:
- Diary and report exist; tasks list is empty.
- Source checked: `devctl/ttmp/2026/01/13/MO-018-ADD-DEVCTL-README--add-devctl-readme/analysis/01-active-ticket-status-report.md`.

Left to do (recipe):
1. Define the README outline (what devctl is, install, quickstart, config, plugins, TUI).
2. Pull exact command examples from `devctl/pkg/doc/topics` and verify they run.
3. Draft the README (likely `devctl/README.md` or top-level `README.md`) with a short quickstart and links to deeper docs.
4. Update `tasks.md` and close the ticket once reviewed.

### MO-018-PIPELINE-VIEW-STUCK-STATE — Status: not implemented (analysis only)

Evidence:
- Diary has investigation and hypotheses; no code commits recorded.
- Source checked: `devctl/pkg/tui/models/pipeline_model.go` shows `PipelinePhaseStarted` clearing finished state.

Left to do (recipe):
1. Add ordering logs in `devctl/pkg/tui/action_runner.go`, `devctl/pkg/tui/transform.go`, and `devctl/pkg/tui/forward.go` to capture phase start/finish timestamps.
2. Guard `PipelinePhaseStartedMsg` to avoid overwriting a finished phase (compare timestamps or ignore late starts).
3. Add a reproduction test (tmux script or unit) and verify the build phase no longer sticks in "running...".

### MO-018-STATE-EVENT-TRACE — Status: not implemented (index only)

Evidence:
- Index and tasks exist; no diary.
- Source checked: `devctl/pkg/tui/transform.go` only emits "state: missing/loaded/error" without location metadata.

Left to do (recipe):
1. Define the event location schema (file:line, op, or stack trace metadata) and where to capture it.
2. Extend domain events to include that metadata and update `transform.go` to emit it.
3. Update the UI event rendering to show the location fields.
4. Add tests or a repro script to validate the new event payloads.

### MO-017-TUI-CONTEXT-LIFETIME-SCOPING — Status: mostly implemented (validation pending)

Evidence:
- Diary steps 1-2 describe context refactors.
- Source checked: `devctl/cmd/devctl/cmds/tui.go`, `devctl/pkg/tui/action_runner.go`, `devctl/pkg/tui/stream_runner.go` show TUI-scoped context usage and `tea.WithContext`.

Left to do (recipe):
1. Run a manual TUI session and confirm streams/actions stop when the UI exits.
2. Verify no blocked publishes or orphaned plugin processes.
3. Mark the validation task complete and close the ticket.

### MO-016-LOGPARSER-DEVCTL-INTEGRATION — Status: not implemented (design only)

Evidence:
- Diary contains investigation steps; no integration code present.
- Source checked: `devctl/cmd/devctl/cmds/logs.go` has no log-parse integration.

Left to do (recipe):
1. Decide the integration surface (CLI flag vs domain stream vs UI view).
2. Implement CLI wrappers or `logs --parse` with service-aware log paths.
3. Add UI model for parsed logs and wire `transform.go` + forwarder.
4. Add configuration persistence and document the workflow.

### STREAMS-TUI — Status: mostly implemented (core fixed, optional enhancement open)

Evidence:
- Diary shows context-cancellation fix and UI enhancements.
- Source checked: `devctl/pkg/tui/stream_runner.go`, `devctl/pkg/tui/models/streams_model.go`.

Left to do (recipe):
1. Decide whether to add protocol-level stream stop semantics to reuse clients.
2. If desired, implement stop op and update runner to avoid one-client-per-stream.
3. Close the ticket or move the optional step into a follow-up ticket.

### MO-015-DEVCTL-PLAN-DEBUG-TRACE — Status: not implemented (index only)

Evidence:
- Index exists; no diary and no code in source.
- Source checked: no plan trace or plugin I/O persistence in `devctl/pkg/*`.

Left to do (recipe):
1. Define what should be persisted (launch plan, plugin I/O transcript) and the storage format under `.devctl/`.
2. Add flags to enable/disable trace recording.
3. Wire the trace into pipeline execution and document how to read it.

### MO-014-IMPROVE-PIPELINE-TUI — Status: not implemented (analysis only)

Evidence:
- Diary documents missing pipeline data sources.
- Source checked: `devctl/pkg/tui/msgs.go` defines live output/config patch/progress msgs, but `devctl/pkg/tui/transform.go` does not publish them.

Left to do (recipe):
1. Add domain event types and publish config patches, live output, and step progress from `action_runner.go`.
2. Map those events in `transform.go` and forward them to the UI.
3. Validate Pipeline view rendering in `devctl/pkg/tui/models/pipeline_model.go`.

### MO-012-PORT-CMDS-TO-GLAZED — Status: partially implemented

Evidence:
- Diary shows the plan and initial ports.
- Source checked: `devctl/cmd/devctl/cmds/status.go` uses Glazed; `devctl/cmd/devctl/main.go` is still Cobra-rooted.

Left to do (recipe):
1. Restructure the root command to Glazed style (BuildCobraCommand + logging layer).
2. Port remaining commands (plan, logs, up, down, stream start, tui).
3. Add repo-layer normalization tests and update help docs.
4. Validate dynamic command behavior and fixture coverage.

### MO-009-TUI-COMPLETE-FEATURES — Status: mostly implemented (pipeline features still open)

Evidence:
- Tasks show phases 1-4 complete; phase 5+ open.
- Source checked: `devctl/pkg/tui/state_watcher.go` (process stats + health), `devctl/pkg/tui/models/pipeline_model.go` (live output UI present but not wired).

Left to do (recipe):
1. Wire pipeline live output, config patches, and step progress into the event pipeline.
2. Implement progress bars and step progress rendering in the Pipeline view.
3. Verify remaining phase-5 tasks and update tasks.md.

### MO-011-IMPLEMENT-STREAMS — Status: mostly implemented (optional enhancement open)

Evidence:
- Diary step 2 includes a runtime fix and tests.
- Source checked: `devctl/pkg/runtime/client.go` shows StartStream gating on ops.

Left to do (recipe):
1. Decide whether to implement a protocol-level stream stop for client reuse.
2. If yes, add the op and update the CLI/TUI runner to use it.

### MO-008-IMPROVE-TUI-LOOKS — Status: mostly implemented but not documented

Evidence:
- Diary is analysis-focused; tasks are empty.
- Source checked: `devctl/pkg/tui/styles/theme.go` and TUI models now use lipgloss/styles/widgets.

Left to do (recipe):
1. Add a diary step that links the styling changes to this ticket.
2. Define/track remaining visual polish tasks (if any).
3. Close or supersede the ticket if no work remains.

### MO-008-REQUIRE-SANDBOX — Status: not implemented (design only)

Evidence:
- Index and design doc exist; tasks are open.
- Source checked: no `goja_nodejs/require` integration in `devctl/pkg/logjs/*`.

Left to do (recipe):
1. Decide sandbox policy (root, symlinks, node_modules behavior).
2. Implement a sandboxed `SourceLoader` for `goja_nodejs/require`.
3. Add tests for allowed/denied module paths.
4. Add `--require` / `--node-console` flags and document the security model.

### MO-007-LOG-PARSER — Status: mostly implemented (integration pending)

Evidence:
- Diary shows multiple implementation steps and tests.
- Source checked: `devctl/cmd/log-parse/main.go`, `devctl/pkg/logjs/*`.

Left to do (recipe):
1. Integrate with `devctl logs` (or add `devctl log-parse` wrapper).
2. Document the integrated workflow in devctl help topics.
3. Close the ticket or spin a follow-up for sandboxed require (MO-008).

### MO-006-DEVCTL-TUI — Status: partially implemented (core exists, milestones open)

Evidence:
- Diary shows initial implementation; tasks list many open milestones.
- Source checked: `devctl/pkg/tui/*` exists with dashboard/service/pipeline/events/models.

Left to do (recipe):
1. Finish pipeline and validation UX (errors/warnings tables, next-action hints).
2. Add cancel-and-cleanup for in-flight actions.
3. Implement plugins view + capability summary and any remaining doc updates.
4. Update `tasks.md` and close or split follow-on work.

## Notes / follow-ups

- Confirm whether non-devctl tickets are intentionally outside devctl scope by reviewing their goals and task lists.
- For stale tickets, decide whether to close, rescope, or refresh with a near-term task.
- For tickets without diaries, add a diary entry or expand the index to make scope explicit.
