---
Title: Rich typed TUI and human-first dual command design and implementation guide
Ticket: DEVCTL-RICH-TUI-001
Status: active
Topics:
    - devctl
    - tui
    - ui-components
    - cli
    - refactor
DocType: design-doc
Intent: long-term
Owners: []
RelatedFiles:
    - Path: repo://cmd/devctl/cmds/logs.go
      Note: Log validation, query, follow, and structured sink contract to share across modes
    - Path: repo://cmd/devctl/cmds/status.go
      Note: Status query and structured row contract to split into dual render paths
    - Path: repo://pkg/operator/results.go
      Note: Existing snapshot and operation result contracts supply all requested presentation data
    - Path: repo://pkg/tui/logs.go
      Note: Current bounded log buffer, filters, and rendering seam
    - Path: repo://pkg/tui/model.go
      Note: Typed Bubble Tea state ownership, message loop, palette, and operation event handling
    - Path: repo://pkg/tui/overview.go
      Note: Current overview selection and service rendering seam
    - Path: repo://pkg/tui/runs.go
      Note: Current session operation and durable-attempt presentation
ExternalSources: []
Summary: A bounded implementation design for restoring devctl's rich three-view TUI and converting status and logs to Glazed dual commands with human-first output, without expanding the operator backend.
LastUpdated: 2026-07-27T13:45:20.288372007-04:00
WhatFor: Guide an intern through a presentation-layer restoration that preserves the typed operator architecture.
WhenToUse: Use while implementing or reviewing the rich TUI restoration and the status/logs dual-command conversion.
---


# Rich typed TUI and human-first dual command design and implementation guide

## Executive Summary

This ticket restores the visual and operational quality of the `devctl` terminal interface without reversing the architectural cleanup that produced the current typed operator layer. The work is deliberately narrow. The application keeps its three current views—Overview, Logs, and Runs—and continues to obtain state, logs, lifecycle results, and progress events through `operator.Controller`. No new persistent state, event journal, telemetry subsystem, lifecycle operation, or service-management backend is introduced.

The TUI work restores a styled shell, an environment summary, a detailed selected-service panel, a live operation panel, stronger log controls, a useful command palette, responsive layouts, and deterministic visual tests. The existing data contracts already contain health, process identifiers, timestamps, exit details, errors, and log paths. The existing message loop already receives live operation events. The implementation should format this data instead of creating replacement sources.

The CLI work converts `devctl status` and `devctl logs` into Glazed dual commands. Their default execution path implements `cmds.BareCommand` and writes intentionally designed human output. Their explicit structured path implements `cmds.GlazeCommand` and is selected with `--with-glaze-output`. Existing row schemas remain the automation contract. Human output must not be produced by rendering the structured table processor with different defaults.

The implementation is split into two phases:

1. Restore the rich typed TUI using only existing operator and log-reader contracts.
2. Convert `status` and `logs` to human-first Glazed dual commands while sharing query and validation logic between their two renderers.

This order makes the most visible regression independently reviewable. Neither phase requires a backend feature addition. A new query helper is permitted only if implementation proves that the same read operation is otherwise duplicated; it must remain an internal extraction, not a new persistence or control capability.

## Problem Statement

### What changed

The current TUI is structurally sound but visually sparse. `pkg/tui/model.go` owns one typed Bubble Tea model and delegates content to `OverviewModel`, `LogsModel`, and `RunsModel`. That architecture is preferable to the former collection of UI-specific controllers. During the operator refactor, however, the old visual component layer was deleted rather than adapted. The current views render mostly unstyled strings.

The result is functional but insufficient for sustained use:

- The active view and environment identity do not have strong visual hierarchy.
- Overview combines a compact service list and a few selected-service fields rather than presenting a complete detail panel.
- Health, PID, uptime, exit state, error state, and log paths are present in the snapshot but are not presented together.
- Live operation events are reduced to one status string even though the model receives typed events.
- Log filters and follow state are textual metadata rather than directly legible controls.
- The command palette exposes only refresh, doctor, and a plugin command hint.
- Existing exact-size golden tests preserve layout, but they currently preserve the bare presentation.

The CLI has a separate usability problem. `status` and `logs` currently implement only `cmds.GlazeCommand`, so the structured data path is also the default operator experience. These commands are read frequently by humans. Their default form should prioritize scanning and diagnosis, while preserving explicit structured output for scripts.

### Scope boundaries

The following are required:

- Styled three-view shell.
- Environment summary.
- Detailed current-service panel.
- Health, PID, uptime, exit, error, and stdout/stderr paths.
- Rich live operation display.
- Better logs interaction.
- More capable command palette.
- Responsive layouts and exact visual tests.
- Human-first `status` and `logs` implemented as Glazed dual commands.
- Structured output available only through the explicit Glazed toggle.

The following are out of scope:

- New lifecycle behavior or operation cancellation.
- Resource telemetry such as CPU, memory, sockets, or filesystem metrics.
- Durable operation history or a new event database.
- New plugin or pipeline service APIs.
- Reintroducing the former six-view TUI.
- A general-purpose widget framework.
- Runtime theme selection or user-configurable palettes.
- Compatibility adapters for the deleted TUI implementation.
- Changes to the operator's persistence model.

## Proposed Solution

### System orientation for a new contributor

`devctl` separates interaction from process management. The CLI and TUI are clients of the same operator controller. Durable service state is stored through `runstate`; structured logs are read through `runlog`. UI code must not inspect process files, open state databases, or launch child processes directly.

```text
                         operator.Controller
                        /         |          \
              Snapshot()      Logs()       Up/Down/Restart/Doctor
                  |              |                 |
                  v              v                 v
         operator.Snapshot   runlog.Reader   result + event stream
                  \              |                 /
                   \             |                /
                    +------ typed UI messages ----+
                                  |
                           pkg/tui.Model
                         /       |       \
                    Overview    Logs     Runs
                         \       |       /
                          styled shell
```

The central contracts are:

- `operator.Snapshot` and `operator.ServiceSnapshot` in `pkg/operator/results.go`. A service snapshot includes desired state, run phase, run ID, wrapper and child process identities, health, creation/update times, exit summary, last error, and stdout/stderr paths.
- `operator.OperationResult` in the same file. It records an operation's kind, time interval, result status, and per-service outcomes.
- `operator.OperatorEvent`, consumed as `EventMsg` by `pkg/tui/model.go`. This is the live progress source.
- `runlog.Reader`, reached through `operator.Controller.Logs()`. It supplies historical queries and follow mode.
- `tui.Model`, which owns active view, terminal size, status, confirmation state, palette state, search state, cursors, and the current operation channel.

The dependency rule is:

```text
views may depend on presentation structs
presentation structs may be derived from operator/runlog values
operator and runlog must not depend on TUI or CLI presentation
```

### Phase 1: restore the rich typed TUI

#### 1. Add a small static presentation layer

Add one static theme and a small set of rendering helpers inside `pkg/tui`. This is not a reusable widget library. Suggested files are:

- `styles.go`: color roles and Lip Gloss styles.
- `layout.go`: shell dimensions, responsive breakpoint selection, and panel composition.
- Existing `overview.go`, `logs.go`, and `runs.go`: view-specific formatting.

The theme needs only semantic roles:

```go
type Theme struct {
    Header       lipgloss.Style
    ActiveTab    lipgloss.Style
    InactiveTab  lipgloss.Style
    Panel        lipgloss.Style
    PanelTitle   lipgloss.Style
    SelectedRow  lipgloss.Style
    Healthy      lipgloss.Style
    Warning      lipgloss.Style
    Error        lipgloss.Style
    Muted        lipgloss.Style
    Key          lipgloss.Style
}
```

Use a single checked-in theme. Restore the recognizable purple/cyan accent, rounded panels, selected-row emphasis, and semantic success/warning/error colors from the prior interface, but do not port the deleted component hierarchy wholesale.

#### 2. Render a consistent shell

Every view uses the same shell:

```text
┌ devctl ─ profile: local ─ 5 services ─ 4 running ─ 1 unhealthy ┐
│ [1 Overview] [2 Logs] [3 Runs]                     : Commands │
├───────────────────────────────────────────────────────────────┤
│                                                               │
│                     active view content                       │
│                                                               │
├───────────────────────────────────────────────────────────────┤
│ operation/status line                          key hints       │
└───────────────────────────────────────────────────────────────┘
```

The shell is a pure render step. It receives the active tab, environment summary, content string, current operation display, key hints, width, and height. It must not own application state.

The environment summary is derived from the current snapshot:

```text
profile = snapshot.Profile, or "default" when absent
total = len(snapshot.Services)
running = count(service.Phase is an active phase)
unhealthy = count(service.Health != nil and !service.Health.Healthy)
stopped = count(service.Phase is terminal or absent)
```

Use the actual `runstate.RunPhase` constants when implementing these counts; do not classify phases by substring.

#### 3. Expand Overview without expanding the backend

Overview consists of an environment summary, service list, and selected-service detail. On wide terminals, the service list and detail panel are side by side. On normal terminals, they stack. On compact terminals, the list remains primary and details use a concise label/value form.

The selected-service panel should display:

- Service name, desired state, and current phase.
- Health state and, when available, the health result's diagnostic text.
- Child PID and wrapper PID as separate fields.
- Run ID.
- Uptime derived from `CreatedAt` and the render clock.
- Exit code and signal when an exit summary exists.
- Last error code and message.
- Stdout path and stderr path.
- Last update age.

No field requires a new operator query. Empty values render as an em dash. Paths may be visually truncated in narrow layouts, but the command palette should offer a “show full selected-service details” overlay so information remains accessible.

Time-dependent rendering should receive a clock value:

```go
func serviceDetails(now time.Time, service operator.ServiceSnapshot) ServiceDetails
```

This prevents unstable uptime and age snapshots in tests.

#### 4. Promote operation events from a status string to a live panel

The current model already handles `EventMsg` and `OperationDoneMsg`. Retain a bounded presentation record for the active operation:

```go
type ActiveOperation struct {
    ID        string
    Kind      string
    StartedAt time.Time
    LastEvent operator.OperatorEvent
    Recent    []operator.OperatorEvent // bounded, for example 8
}
```

This is ephemeral UI state, not a backend journal. It is populated from events already delivered on `operationCh`, cleared or converted into the existing `RunsModel` entry on completion, and discarded when the TUI exits.

Render:

- Operation kind and current status.
- Current phase or service.
- Elapsed time.
- A bounded list of recent phase/service results.
- A clear terminal success or failure state.

The update path remains:

```text
operator operation
    -> event channel
    -> EventMsg
    -> update ActiveOperation
    -> render operation panel
    -> OperationDoneMsg
    -> RunsModel.Add(...)
```

#### 5. Improve log interaction using existing controls

Keep the bounded in-memory log buffer and existing reader/follow design. Improve discoverability and manipulation:

- Display filter chips for selected services and stdout/stderr.
- Show follow, paused, wrap, search, visible count, and dropped count as a compact status row.
- Add direct keys to toggle stdout and stderr.
- Add a key to cycle service scope among all services, the currently selected service, and an explicit selection.
- Retain `/` for search, with an obvious search prompt and match count.
- Retain pause, follow, and wrap controls.
- Add clear-buffer as a presentation-only action.
- Preserve sanitization of embedded terminal escape sequences.
- Color the stream label, not arbitrary log payloads.

Do not add server-side streaming controls. These actions modify `LogsModel` filters or the existing `runlog.Query`.

#### 6. Make the command palette useful

The palette should expose existing safe actions and navigation:

- Refresh snapshot.
- Run doctor.
- Go to Overview, Logs, or Runs.
- Start, stop, or restart the selected service through the existing confirmation flow.
- Show logs for the selected service.
- Toggle follow, pause, wrapping, stdout, and stderr.
- Clear the local log buffer.
- Show full selected-service details.
- Show the existing plugin inspection command hint.

Palette entries are data, and execution remains a switch over known action identifiers. Do not introduce a command registry or reflection.

```go
type paletteAction struct {
    ID          actionID
    Label       string
    Enabled     func(Model) bool
}
```

Disabled actions should be visibly disabled and should explain why in the footer.

#### 7. Use three responsive layouts

Use explicit, testable breakpoints rather than a constraint solver:

- Wide: width at least 100 columns. Side-by-side overview panels and a full footer.
- Standard: width 70–99. Stacked panels with abbreviated hints.
- Compact: width below 70. Single-column content, shortened labels, and minimal footer.

Height determines the number of service, log, event, and operation rows. Rendering must never assume a minimum terminal size. At extremely small sizes, show the header, active content summary, and a “resize for details” message.

### Phase 2: human-first Glazed dual commands

#### Dual-command contract

Each command implements both interfaces:

```go
var _ glazedcmds.BareCommand = (*StatusCommand)(nil)
var _ glazedcmds.GlazeCommand = (*StatusCommand)(nil)

func (c *StatusCommand) Run(ctx context.Context, vals *values.Values) error {
    result, err := c.query(ctx, vals)
    if err != nil { return err }
    return renderHumanStatus(c.writer(vals), result)
}

func (c *StatusCommand) RunIntoGlazeProcessor(
    ctx context.Context,
    vals *values.Values,
    processor middlewares.Processor,
) error {
    result, err := c.query(ctx, vals)
    if err != nil { return err }
    return addStatusRows(ctx, processor, result)
}
```

Build both commands with the supported Glazed API:

```go
cli.BuildCobraCommand(
    command,
    cli.WithDualMode(true),
    cli.WithGlazeToggleFlag("with-glaze-output"),
)
```

The default paths are:

```text
devctl status
devctl logs api
devctl logs api --follow
```

The structured paths are explicit:

```text
devctl status --with-glaze-output --output json
devctl logs api --with-glaze-output --output csv
devctl logs api --follow --with-glaze-output --output json
```

#### Temporary devctl builder workaround

Glazed issue [#611](https://github.com/go-go-golems/glazed/issues/611) currently tracks a limitation in the standard dual-mode builder: it installs a `cmd.Run` callback and calls `cobra.CheckErr`, so errors never return to the embedding application's `Execute()` and application-specific exit-code classification is lost. The same builder does not currently invoke devctl's custom processor preparation hooks.

Until that Glazed fix is available, devctl uses `buildDualGlazedCommand` in `cmd/devctl/cmds/lifecycle.go` for only `status` and `logs`. The local builder keeps the desired public dual-command contract, but uses `RunE`, returns errors to `cmd/devctl/main.go`, and preserves `PrepareGlazedValues` and `BuildGlazedProcessor` for streaming logs. This is an intentional temporary implementation boundary, not a second command API. Once Glazed provides equivalent behavior, replace the local builder and retain the same CLI tests.

Do not preserve the current implicit structured default through an adapter. Update documentation and tests to the new command contract.

#### Share semantics, not rendering

Extract command-internal query functions so validation, service selection, time parsing, history selection, and follow cursors execute identically in both modes. Renderers consume typed results.

For status:

```go
type statusResult struct {
    Snapshot operator.Snapshot
    Services []operator.ServiceSnapshot
    Now      time.Time
}

queryStatus(ctx, vals) (statusResult, error)
renderHumanStatus(w, result) error
addStatusRows(ctx, processor, result) error
```

For logs, use a sink abstraction already compatible with follow mode:

```go
type logRecordSink interface {
    Add(context.Context, runlog.LogRecord) error
}

queryOrFollowLogs(ctx, settings, sink) error
humanLogSink.Add(...)       // writes one readable line and flushes
processorLogSink.Add(...)   // preserves existing structured row schema
```

This is an internal querying extraction, not a backend feature. The existing `runlog.Reader.Query` and `Follow` APIs remain unchanged.

#### Human status output

Default status output should be stable, concise, and terminal-oriented:

```text
Environment  local       Services 5   Running 4   Unhealthy 1

SERVICE       DESIRED   STATE      HEALTH      PID    UPTIME
api           running   running    healthy    4123    18m
worker        running   failed     unhealthy     -     -

worker: E_PROCESS_EXIT: process exited with code 1
  stdout: .devctl/runs/worker/.../stdout.jsonl
  stderr: .devctl/runs/worker/.../stderr.jsonl
```

Detailed error and path lines should appear only when relevant. Respect the command writer; do not call `fmt.Println`, because tests and embedding need output injection.

#### Human logs output

Default log output should optimize chronological scanning:

```text
13:04:12.118 api      stdout  listening on :8080
13:04:12.302 worker   stderr  connection refused
```

Existing `--timestamps`, `--no-prefix`, and `--ansi` flags control human rendering. Follow mode writes and flushes one record at a time. The structured follow case retains JSON Lines behavior when explicitly requested.

### Error and output rules

- Human data goes to stdout.
- Structured data goes to stdout through Glazed processors.
- Diagnostics and Cobra errors remain on stderr.
- Operational and usage errors retain their typed exit classification.
- Empty status is a readable “environment stopped” result, not an error.
- Unknown selected services remain usage errors.
- Broken log records retain existing `runlog.ReadError` behavior.

## Design Decisions

### Preserve the typed operator boundary

All requested data is already present in `operator.ServiceSnapshot`, and live events already reach the model. Adding a second backend would create competing truth sources. Presentation derives from the existing contracts.

### Restore visual primitives, not the former UI architecture

The deleted TUI contained useful visual choices, but its larger component and control structure predates the current operator boundary. A static theme and a handful of pure render helpers recover the visual quality without restoring obsolete ownership.

### Keep three views

Overview, Logs, and Runs match the current operator tasks and the user's explicit scope. Additional top-level views would increase navigation and testing cost without adding required information.

### Make human output the unmarked CLI path

Operators should not need structured-output flags for routine inspection. Automation remains explicit and composable through the standard Glazed toggle and output processors.

### Test time and color explicitly

Uptime and styling are otherwise environment-dependent. Rendering receives a fixed time and a renderer with a fixed color profile in tests. This makes visual fixtures exact across CI and developer terminals.

## Alternatives Considered

- Reintroduce the deleted TUI wholesale. Rejected because it would restore obsolete structure and more views than requested.
- Add richer snapshot endpoints or persistent operation history. Rejected because existing snapshot and event contracts cover the requested UI.
- Build a generic widget DSL. Rejected because three fixed views need a small local presentation layer.
- Use Glazed table output as the default human renderer. Rejected because status detail and streaming log presentation require deliberate human formatting.
- Keep structured output as the default and add `--human`. Rejected because it contradicts the requested human-first contract.
- Add configurable themes and breakpoint settings. Rejected as unnecessary for the MVP.

## Implementation Plan

### Phase 1 checklist: TUI

1. Add `styles.go` and `layout.go` with one theme, three breakpoints, panel helpers, and shell composition.
2. Add deterministic presentation helpers for environment and service details.
3. Rewrite `OverviewModel.View` around list/detail layouts while preserving selection behavior.
4. Add bounded `ActiveOperation` state and update it from existing `EventMsg` values.
5. Improve `LogsModel.View` and key handling for stream toggles, service scope, search feedback, and clear-buffer.
6. Expand palette data and route actions to existing update paths.
7. Style Runs using the shared shell and semantic statuses.
8. Replace current golden fixtures with exact compact, standard, and wide fixtures; add focused behavior tests.

### Phase 2 checklist: CLI

1. Extract status query/selection logic into a command-local typed result.
2. Implement `StatusCommand.Run` and retain `RunIntoGlazeProcessor`.
3. Extract logs query/follow orchestration behind record sinks.
4. Implement a flushing human sink and retain the processor sink.
5. Replace the local structured-only builder calls for these two commands with Glazed dual-mode construction.
6. Update CLI contract tests so bare invocation asserts human output and explicit Glazed invocation asserts rows/JSON.
7. Verify follow cancellation, partial-record handling, exit codes, stdout/stderr separation, and help text.

### File-level change map

| File | Intended change |
|---|---|
| `pkg/tui/model.go` | Shell coordination, active-operation presentation state, keys, and palette actions |
| `pkg/tui/overview.go` | Environment summary, service list, selected-service detail |
| `pkg/tui/logs.go` | Styled filters, stream/service controls, search feedback, log rows |
| `pkg/tui/runs.go` | Styled session operations and outcomes |
| `pkg/tui/styles.go` | New static semantic styles |
| `pkg/tui/layout.go` | New responsive layout and shell helpers |
| `pkg/tui/model_test.go` | Behavior and deterministic visual tests |
| `pkg/tui/testdata/*` | Exact compact, standard, and wide fixtures |
| `cmd/devctl/cmds/status.go` | Shared query plus Bare/Glaze render paths |
| `cmd/devctl/cmds/logs.go` | Shared query/follow plus human and processor sinks |
| `cmd/devctl/cmds/cli_contract_test.go` | Human-default and explicit-structured contracts |

### Test strategy

Run focused tests while developing:

```bash
go test ./pkg/tui
go test ./cmd/devctl/cmds -run 'TestCLIContract|TestHelpTree'
```

Then run repository validation:

```bash
go test ./...
go build ./...
```

For exact visual tests:

- Fix `now` to one timestamp.
- Use one deterministic Lip Gloss color profile.
- Render representative snapshots at 44×16, 80×24, and 120×30.
- Store exact output fixtures, including ANSI sequences when testing styled output.
- Keep separate behavior assertions for palette enablement, log filtering, and active-operation transitions so failures are diagnosable.
- Exercise the TUI manually in tmux and inspect it with `tmux capture-pane`.

### Acceptance criteria

- The TUI still has exactly Overview, Logs, and Runs.
- All three views use one styled shell and semantic status colors.
- Overview presents environment counts and all requested selected-service fields.
- Live operations show current phase/service, elapsed time, recent events, and completion.
- Logs expose visible filter state and usable keyboard controls without changing runlog storage.
- The command palette includes navigation, existing lifecycle actions, log actions, refresh, doctor, and detail display.
- 44×16, 80×24, and 120×30 renderings are readable and covered by exact fixtures.
- `devctl status` and `devctl logs` default to human output.
- `--with-glaze-output` enables existing structured schemas and processors.
- `logs --follow --with-glaze-output --output json` remains streaming JSON Lines.
- No new operator persistence, telemetry, lifecycle, or event-history feature is added.
- `go test ./...` and `go build ./...` pass.

## Open Questions

There are no design-blocking questions. During implementation, the author should choose precise colors and compact labels by testing in the supported terminal, but these choices do not alter the architecture.

## References

- `pkg/operator/results.go:19` — operation and snapshot result contracts.
- `pkg/tui/model.go:15` — TUI options, model state, message handling, and palette.
- `pkg/tui/overview.go:39` — current overview renderer.
- `pkg/tui/logs.go:15` — bounded log model and current interactions.
- `pkg/tui/runs.go:10` — session operation presentation model.
- `cmd/devctl/cmds/status.go:18` — current structured-only status command.
- `cmd/devctl/cmds/logs.go:24` — current structured-only logs command and follow path.
- `cmd/devctl/cmds/lifecycle.go:195` — current local Glazed command builder.
- `cmd/devctl/cmds/cli_contract_test.go:17` — existing status and logs CLI contracts.
- `pkg/tui/model_test.go:177` — existing exact-size golden tests.
- Glazed v1.2.5 example: `cmd/examples/new-api-dual-mode/main.go`.
- `ttmp/2026/01/06/MO-008-IMPROVE-TUI-LOOKS--improve-tui-looks-and-architecture` — prior visual design history.
- `ttmp/2026/07/24/DEVCTL-OPERATOR-UX-001--operator-first-runtime-ux-and-typed-control-plane` — operator refactor design and scope.
