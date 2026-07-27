---
Title: Investigation diary
Ticket: DEVCTL-RICH-TUI-001
Status: active
Topics:
    - devctl
    - tui
    - ui-components
    - cli
    - refactor
DocType: reference
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: "Chronological research record for the bounded rich-TUI and human-first dual-command design."
LastUpdated: 2026-07-27T13:45:20.608675866-04:00
WhatFor: "Preserve the evidence, commands, decisions, and scope checks behind the implementation guide."
WhenToUse: "Use when reviewing the design rationale or resuming implementation in a later session."
---

# Investigation diary

## Goal

Design a deliberately bounded restoration of devctl's rich TUI and human-first `status` and `logs` commands. Record enough evidence that another engineer can reproduce the reasoning without reopening the entire refactor history.

## Context

The user requested:

> Ok, no backend feature addition (except maybe new ways of querying it), and use glazed dual commands for logs and status, with human output per default, isntead of structured.
>
> Stick to :
>
> I would split the work into two phases.
>
> First, restore the rich UI using existing data:
>
> - Styled three-view shell
> - Environment summary
> - Detailed current service panel
> - Health, PID, uptime, exit, error, and paths
> - Rich live operation display
> - Better logs interaction
> - More capable command palette
> - Responsive layouts and exact visual tests
>
> for the UI part.
>
> Create a new docmgr ticket and Create a detailed analysis / design / implementation guide that is for a new intern, explaining all the parts of the system needed to understand what it is, with prose paragraphs and bullet point sand pseudocode and diagrams and api references and file references. It should be very clear and technical. Store in the ticket and the nupload to remarkable.
>
> Don't overengineer.

## Quick Reference

### Step 1 — Establish repository and ticket state

Commands:

```bash
git status --short --branch
git rev-parse HEAD
docmgr status --summary-only
```

Result: the repository was on `task/prod-tiny-idp` at commit `6f5637cf83d2d8a73c7935ec0d179c49d09fcbf9`. The worktree was clean before ticket creation. Docmgr was initialized and the vocabulary already contained `devctl`, `tui`, `ui-components`, `cli`, and `refactor`.

Created ticket `DEVCTL-RICH-TUI-001` with one design document and this diary.

### Step 2 — Inspect current TUI ownership

Files inspected:

- `pkg/tui/model.go`
- `pkg/tui/overview.go`
- `pkg/tui/logs.go`
- `pkg/tui/runs.go`
- `pkg/tui/messages.go`
- `pkg/tui/model_test.go`

Findings:

- One Bubble Tea model owns all durable UI state.
- The model receives snapshots, log records, operator events, completion results, doctor reports, and errors as typed messages.
- Overview, Logs, and Runs are already the only three views.
- The current palette contains refresh, doctor, and a plugin-inspection hint.
- The current log buffer is bounded by records and bytes.
- Exact golden fixtures already cover 44×16, 80×24, and 120×30.
- The deficiency is presentation and interaction density, not the absence of an application model.

### Step 3 — Verify requested fields against the backend contract

Inspected `pkg/operator/results.go`.

`ServiceSnapshot` already contains:

- desired state and phase;
- run ID;
- wrapper and child process identities;
- health result;
- creation and update timestamps;
- exit summary;
- last error;
- stdout and stderr paths.

`OperationResult` already contains operation kind, start and finish time, status, outcomes, and typed errors. `EventMsg` already arrives during an operation. Therefore the requested UI does not need a new backend endpoint or persistent operation log.

### Step 4 — Inspect the visual regression history

Commands used included:

```bash
git show --stat c25c31bf2a2190ddd8b045a759cdf3dba4c9bbc1
git show c25c31bf2a2190ddd8b045a759cdf3dba4c9bbc1^:pkg/tui/theme/theme.go
git show c25c31bf2a2190ddd8b045a759cdf3dba4c9bbc1^:pkg/tui/widgets/box.go
rg -n "deleted legacy TUI|styling can be layered" ttmp
```

Findings:

- The operator refactor intentionally replaced the old TUI instead of adapting it.
- The diary for that work states that human rendering was deliberately plain and styling could be layered later.
- The older UI used a coherent purple/cyan palette, rounded boxes, semantic state colors, selected-row emphasis, and styled key hints.
- Those visual choices remain useful. The deleted ownership hierarchy and six-view structure do not.

Decision: restore a small static presentation layer, not the deleted TUI implementation.

### Step 5 — Inspect status and logs

Files inspected:

- `cmd/devctl/cmds/status.go`
- `cmd/devctl/cmds/logs.go`
- `cmd/devctl/cmds/lifecycle.go`
- `cmd/devctl/cmds/cli_contract_test.go`

Findings:

- Both commands currently implement only `glazedcmds.GlazeCommand`.
- `status` queries one operator snapshot and emits one row per selected service.
- `logs` contains validation, run resolution, historical queries, follow cursors, and processor emission.
- Follow plus JSON uses a custom JSON Lines processor.
- The local `buildGlazedCommand` helper always installs the Glazed output section and processor path.

The implementation should extract shared semantic work so Bare and Glaze renderers do not perform separate queries.

### Step 6 — Verify the Glazed dual-command API

Inspected:

```text
/home/manuel/go/pkg/mod/github.com/go-go-golems/glazed@v1.2.5/cmd/examples/new-api-dual-mode/main.go
```

The supported pattern is:

```go
var _ cmds.BareCommand = &StatusCommand{}
var _ cmds.GlazeCommand = &StatusCommand{}

cli.BuildCobraCommand(
    statusCommand,
    cli.WithDualMode(true),
    cli.WithGlazeToggleFlag("with-glaze-output"),
)
```

The Bare `Run` method is the classic default. The `GlazeCommand` method is used when the explicit toggle is present. This directly matches the requested UX.

### Step 7 — Bound the design

Rejected additions:

- backend telemetry;
- durable event history;
- new lifecycle controls;
- a generic widget framework;
- a theme configuration system;
- the old six-view navigation;
- compatibility adapters for the deleted UI.

Allowed implementation-local extraction:

- pure presentation projections;
- a bounded active-operation view record;
- shared status query results;
- a log-record sink used by human and processor renderers.

These do not change the operator's capabilities or durable model.

## Usage Examples

Use the design document as the implementation specification. Before changing code, verify that a proposed field is absent from `operator.ServiceSnapshot`. If it is present, derive it in the presentation layer. If it is absent, stop and determine whether it is truly within this ticket; the default answer is that it is out of scope.

Use the following commands for focused validation:

```bash
go test ./pkg/tui
go test ./cmd/devctl/cmds -run 'TestCLIContract|TestHelpTree'
go test ./...
go build ./...
```

Run interactive TUI checks in tmux and inspect output with `tmux capture-pane`, as required by the repository instructions.

## Related

- [Rich typed TUI and human-first dual command design and implementation guide](../design-doc/01-rich-typed-tui-and-human-first-dual-command-design-and-implementation-guide.md)
- `pkg/operator/results.go`
- `pkg/tui/model.go`
- `cmd/devctl/cmds/status.go`
- `cmd/devctl/cmds/logs.go`
