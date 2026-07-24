---
Title: Investigation Diary
Ticket: DEVCTL-OPERATOR-UX-001
Status: active
Topics:
    - devctl
    - tui
    - architecture
    - supervisor
    - cli
    - refactor
DocType: reference
Intent: long-term
Owners: []
RelatedFiles:
    - Path: repo://.ttmp.yaml
      Note: Repository-local docmgr ownership boundary discovered during Step 1
    - Path: repo://cmd/devctl/cmds/wrap_service.go
      Note: Existing supervision correction that anchors the reliability case study
    - Path: repo://cmd/devctl/cmds/wrap_service_test.go
      Note: Regression evidence for the committed process-group fix
ExternalSources: []
Summary: Chronological evidence, experiments, failures, and decisions for the devctl operator-experience research ticket.
LastUpdated: 2026-07-24T13:22:12.311783489-04:00
WhatFor: Preserve how the architecture findings and redesign recommendations were derived.
WhenToUse: Read when reviewing the design, reproducing an observation, or resuming the investigation.
---


# Investigation Diary

## Goal

Record the evidence-first investigation of devctl's supervision, state,
logging, CLI, and TUI behavior, including failed attempts and the reasoning
behind each design recommendation. The diary is deliberately separate from
the polished design guide: it preserves chronology and uncertainty so a future
engineer can audit how conclusions were reached.

## Step 1: Establish the repository and ticket baseline

This step established the correct documentation root, created the research
ticket, and converted the broad request into explicit tasks. It also fixed a
workspace-local tooling ambiguity before any research artifact could be placed
in the wrong repository.

The scope is analysis and design. The branch already contains the focused
supervision correction at commit `39ba416`; this ticket will explain and
evaluate it, but will not opportunistically implement the wider redesign.

### Prompt Context

**User prompt (verbatim):**

> Create a new ticket in devctl to fix devctl. Furthermore, before you start implementating, research about possible improvements you could make to devctl while you are at it, especially in terms of properly displaying and handling logs, ergonomic improvements to the CLI and TUI, things that could be removed and simplified that don't seem useful, etc...
>
> Review it all from the perspective of an actual heavy user like you are, with potentially more complex setups too.
>
> Create  a detailed analysis / design / implementation guide that is for a new intern, explaining all the parts of the system needed to understand what it is, with prose paragraphs and bullet point sand pseudocode and diagrams and api references and file references. It should be very clear and technical. Store in the ticket and the nupload to remarkable.

**Assistant interpretation:** Create a documentation-first devctl ticket that
audits the complete operator experience, compares it with mature tools,
recommends evidence-based simplifications, and supplies a decision-complete
future implementation guide without carrying out the broad refactor.

**Inferred user intent:** Make devctl dependable and pleasant enough for daily
use in complex repositories, while reducing accidental complexity and giving
a new contributor enough technical context to implement the redesign safely.

### What I did

- Verified the devctl branch and worktree were clean at
  `39ba416 fix(supervise): isolate wrapper from service signals`.
- Ran `docmgr status --summary-only` from the devctl worktree.
- Discovered that docmgr selected the parent workspace configuration and
  reported the TinyIDP documentation root.
- Added a repository-local `.ttmp.yaml` pointing to `devctl/ttmp` and its
  vocabulary.
- Re-ran docmgr status and confirmed the root was
  `/home/manuel/workspaces/2026-07-07/prod-tiny-idp/devctl/ttmp`.
- Created ticket `DEVCTL-OPERATOR-UX-001`, its primary design document, this
  diary, and eleven stable-ID tasks.

### Why

- A ticket created under the parent configuration would have violated the
  explicit requirement to store the work in devctl.
- Stable tasks make the research auditable and prevent a polished narrative
  from hiding incomplete evidence lanes.
- The existing supervision fix must be separated from proposed improvements so
  observed code and future design are not conflated.

### What worked

- A nearest-repository `.ttmp.yaml` cleanly overrides the parent workspace
  configuration.
- The existing devctl vocabulary already contains the required topics:
  `devctl`, `tui`, `architecture`, `supervisor`, `cli`, and `refactor`.
- Docmgr created the expected index, task list, changelog, design document,
  diary, and supporting directories.

### What didn't work

The first status check from `devctl/` resolved the wrong root:

```text
root=/home/manuel/workspaces/2026-07-07/prod-tiny-idp/tiny-idp/ttmp
config=/home/manuel/workspaces/2026-07-07/prod-tiny-idp/.ttmp.yaml
```

No ticket was created before this was corrected. The repository-local
configuration changed the result to:

```text
root=/home/manuel/workspaces/2026-07-07/prod-tiny-idp/devctl/ttmp
config=/home/manuel/workspaces/2026-07-07/prod-tiny-idp/devctl/.ttmp.yaml
```

The first commit gate then stopped because the changelog updater left an extra
blank line at the end of its generated section:

```text
ttmp/2026/07/24/DEVCTL-OPERATOR-UX-001--heavy-user-reliability-logging-cli-and-tui-analysis-and-design/changelog.md:15: new blank line at EOF.
```

No commit was created. The trailing blank line was removed and the same
`git diff --cached --check` gate was rerun.

### What I learned

- A nested worktree can inherit an unrelated parent docmgr root even when it
  already contains its own `ttmp/` directory.
- The existence of `ttmp/` is not enough to establish docmgr ownership; the
  nearest `.ttmp.yaml` is the controlling boundary.
- Current ticket count and staleness metrics are only meaningful after that
  boundary is verified.

### What was tricky to build

- The wrong root was syntactically valid and writable, so this failure mode
  would not have produced an error. The only safe approach was to inspect the
  resolved root before mutation.
- The research scope spans several partially independent systems. The task
  list separates them so evidence from one surface, such as service logs,
  cannot be used to imply completion of another, such as TUI action handling.

### What warrants a second pair of eyes

- Confirm that committing `.ttmp.yaml` is the desired long-term behavior for
  devctl when it is checked out inside larger workspaces. It matches sibling
  repositories in this workspace.
- Review the final pruning decisions carefully because the user explicitly
  permits removal without compatibility adapters, but removal still requires
  evidence that a surface is not carrying important workflows.

### What should be done in the future

- Keep ticket-local source captures and probes docmgr-compatible.
- Distinguish current observations, measured behavior, external precedents,
  and proposed design throughout the primary guide.

### Code review instructions

- Start with `.ttmp.yaml` and verify `docmgr status --summary-only` resolves
  `devctl/ttmp`.
- Inspect `tasks.md` and confirm every requested analysis lane is represented.
- Verify the branch begins with the supervision fix at `39ba416`.

### Technical details

The evidence hierarchy for the ticket is:

1. Executed behavior and captured artifacts.
2. Current source code and tests with line references.
3. Current repository documentation and historical ticket records.
4. Official upstream documentation for comparison tools.
5. Design inference, explicitly labeled as proposed.

## Step 2: Map supervision and persistent state

This step followed the lifecycle from `devctl up`, through wrapper creation and
readiness, into `state.json`, and then through status, stop, start, and restart.
It also introduced two ticket-local read-only probes. The first inventories
source and ticket surfaces. The second builds devctl in an isolated temporary
directory and records representative CLI output and exit codes.

### Prompt Context

**User prompt (verbatim):**

> Ok, now implement the ticket, keep a detailed diary frequently, check off
> tasks as you go, commit at appropriate intervals.

**Assistant interpretation:** Complete the documentation ticket through
evidence gathering, design, validation, commits, and reMarkable delivery,
updating the diary and stable-ID task list throughout the work.

**Inferred user intent:** Preserve enough operational and reasoning history
that a future engineer can reproduce the findings rather than trusting a
single polished design narrative.

### What I did

- Added `scripts/01-architecture-inventory.sh` to enumerate the current
  supervision, logging, CLI, TUI, test, and historical-ticket surfaces.
- Added `scripts/02-cli-contract-probe.sh` to build a temporary devctl binary
  and capture help, status, logs, down, plan, and profile behavior without
  modifying repository state.
- Read the complete supervision path in:
  - `cmd/devctl/cmds/up.go`
  - `pkg/supervise/supervisor.go`
  - `cmd/devctl/cmds/wrap_service.go`
  - `pkg/state/state.go`
  - `pkg/state/exit_info.go`
  - `pkg/servicecontrol/resolve.go`
- Traced the focused process-group fix at commit `39ba416` through the wrapper
  and its regression test.
- Added the current-state lifecycle model and reliability findings to the
  primary design document.
- Checked task `ud53` only after the lifecycle, health, state, exit, and
  restart paths all had source-backed coverage.

### Why

Devctl's operator behavior cannot be evaluated from command names alone.
The wrapper PID, child process group, readiness marker, state record, log
paths, and exit record are separate artifacts created at different times.
Correct redesign decisions depend on knowing which artifact is authoritative
at each transition and what survives an abrupt interruption.

### What worked

- Both probes use temporary build/cache directories and leave the repository
  unchanged.
- The architecture probe produced 654 lines of reproducible inventory.
- The CLI probe produced 120 lines with command text, stdout/stderr, and exit
  status for each case.
- The source establishes that commit `39ba416` fixes the immediate recursive
  signal problem: the wrapper and child occupy different process groups, and
  the wrapper forwards termination to the child's group.

### What didn't work

The first architecture probe run ended with:

```text
rg: regex parse error:
    (?:Use:|NewCommandDescription\\()
    ^
error: unclosed group
```

The shell single-quoted expression had been over-escaped. I replaced the
combined regular expression with two explicit `rg -e` expressions and reran
the whole script successfully.

The initial source-read command requested
`cmd/devctl/cmds/start.go`, but the command is stored under a different
filename:

```text
nl: cmd/devctl/cmds/start.go: No such file or directory
```

No conclusion depends on that failed path. The command inventory will be used
to resolve and inspect the actual filename during the CLI pass.

### What I learned

- `Supervisor.Start` starts every wrapper first, performs readiness checks
  second, and only returns a state object afterward. `up` persists that object
  after `Start` returns. A devctl crash or terminal loss in this interval can
  leave live wrappers without a `state.json` record.
- The persisted PID identifies the wrapper, while exit metadata identifies the
  child. The state does not persist child PID, process-group ID, Linux process
  start time, or a unique run identifier.
- Artifact names have one-second timestamp resolution. Multiple starts of the
  same service within one second can select the same log, exit, and readiness
  paths.
- Wrapper readiness is represented by file existence. The wrapper ignores
  errors while creating and writing that marker; the supervisor does not read
  or validate its child PID.
- `HealthCheck.TimeoutMs` is modeled and persisted, but both bulk and
  single-service starts use the supervisor-wide timeout.
- HTTP readiness accepts every status below 500, including authentication,
  authorization, and not-found responses.
- State writes and exit-info writes use direct `os.WriteFile`, so readers can
  observe a partial file if a writer is interrupted.
- `Stop` retains only the last error. `down` ignores that error and removes
  state anyway, which can discard the only ownership record for a process that
  failed to stop.
- `StopService` clears both PID and exit-info path. A subsequent status loses
  the previous run's diagnostic link.
- Restart intentionally recomputes `config.mutate` and `launch.plan` so raw
  plugin-computed environment values are not persisted. It does not rerun
  build, prepare, or validation.

### What was tricky to build

- “PID” has three meanings in the current implementation: the state PID is the
  wrapper, the ready-file payload is the child, and signaling must target the
  child process group through the wrapper. Treating them as interchangeable
  would hide the original signal bug and produce an unsafe redesign.
- Several error paths deliberately attempt cleanup and discard cleanup errors.
  This is acceptable only if durable state still describes what might remain;
  today the state is sometimes not yet written or is removed unconditionally.

### What warrants a second pair of eyes

- Decide whether HTTP readiness should default to a strict `200..399` range or
  require an explicitly configured expected-status set. The final design will
  choose one contract and justify it.
- Confirm whether the direct, no-wrapper branch of `Supervisor.startService`
  has a supported production consumer. It lacks exit-info persistence and
  should be removed if it is only a test convenience.
- Review the proposed use of Linux process start time for PID identity. A
  portable fallback will be necessary if devctl must support non-Linux hosts.

### What should be done in the future

- Make a run record durable before or atomically with process ownership.
- Assign every run a collision-resistant identifier and derive all artifact
  paths from it.
- Persist wrapper PID, child PID, child process-group ID, and a process
  identity token.
- Use atomic replace for state and exit records.
- Aggregate stop outcomes and never remove ownership records for processes
  that may still be alive.

### Code review instructions

- Read the “Current supervision and state architecture” section in the primary
  design guide next to `pkg/supervise/supervisor.go`.
- Run both scripts from the repository root. They should modify only temporary
  directories and print their evidence to stdout.
- Confirm task `ud53` is checked and the remaining CLI, log, TUI, and probe
  tasks remain open.

### Technical details

The process hierarchy after commit `39ba416` is:

```text
devctl up
  └─ wrapper process (persisted PID, process-group leader)
       └─ service child (separate process-group leader)
            └─ service descendants (normally inherit child group)
```

The lifecycle durability window is:

```text
start wrapper -> ready marker -> health wait -> Supervisor.Start returns
                                                  -> state.Save
```

Only the final operation makes the run discoverable by a later devctl process.

## Step 3: Audit logs, CLI contracts, TUI behavior, and historical intent

This step mapped all five things that current code calls a log, audited command
syntax and output, ran a real TUI under tmux, and compared the implementation
with prior devctl tickets. It separates useful capabilities from complexity
that accumulated because each surface grew independently.

### Prompt Context

**User prompt (verbatim):**

> Furthermore, before you start implementating, research about possible
> improvements you could make to devctl while you are at it, especially in
> terms of properly displaying and handling logs, ergonomic improvements to
> the CLI and TUI, things that could be removed and simplified that don't seem
> useful, etc...
>
> Review it all from the perspective of an actual heavy user like you are, with
> potentially more complex setups too.

**Assistant interpretation:** Evaluate real daily workflows and complex,
multi-service failure modes; do not treat the presence of a feature as proof
that its current interaction model is worth retaining.

**Inferred user intent:** Reduce the amount of devctl-specific behavior an
operator must remember while making failures easier to inspect and automation
safer.

### What I did

- Traced service stdout/stderr from wrapper creation through CLI and TUI
  readers.
- Traced devctl diagnostics through Logcopter/zerolog and plugin stderr into
  the runtime logger.
- Traced NDJSON protocol events through `runtime.Client`, the stream CLI, and
  the TUI stream runner.
- Audited the standalone `cmd/log-parse` and `pkg/logjs` subsystem and searched
  the repository for integration consumers.
- Counted 7,757 Go lines under `pkg/tui`, seven model files, and zero TUI test
  files.
- Read the root registration, repository flag layer, lifecycle commands,
  status, logs, plugin inspection, streams, and dynamic command registration.
- Ran `scripts/03-log-follow-lifecycle-probe.sh` against append, copy-truncate,
  and rename/recreate behavior.
- Ran `scripts/04-tui-state-event-probe.sh` in tmux with a 200 ms refresh and
  captured the Events view.
- Reviewed the implementation and task state of the prior TUI, streams,
  log-parser, Glazed migration, context-lifetime, state-event, and
  single-service tickets.
- Added current-state maps, gap tables, and consolidation constraints to the
  primary design guide.
- Checked tasks `1x08`, `zn2u`, `iwzu`, `8qz7`, and `wslm` only after the
  associated evidence was written.

### Why

Heavy-use reliability depends on consistency across surfaces. A CLI and TUI
that independently implement “restart” can each look correct in isolation
while producing different failure behavior. Likewise, a log viewer that works
for a quiet single service can silently lose output when a file rotates or
become unusable when ten services interleave.

### What worked

- The log lifecycle probe observed `append-visible`, proving its baseline
  setup worked, then observed neither `after-truncate` nor `after-rotate`.
- The TUI probe reproduced repeated state events without starting or stopping
  services. At a 200 ms refresh it reported eight events and five events per
  second after approximately two seconds.
- The existing screenshot
  `docs/screenshots/devctl-tui-dashboard.png` independently shows the same
  repetition at the default one-second refresh.
- Repository search found the JavaScript parser used by its standalone binary,
  tests, examples, and long-form help, but not by `devctl logs`, supervision,
  or the TUI.
- Historical task lists explain why features exist and expose metadata drift
  that prevents ticket status from serving as current truth.

### What didn't work

The first historical-ticket loop used `status` as a zsh variable:

```text
zsh:1: read-only variable: status
```

I renamed it to `ticket_status` and reran the inventory. No ticket was edited.

The first log lifecycle probe used a fresh empty `GOCACHE`. Compilation
consumed the available execution interval and the command exited with status
143 before printing assertions. I changed only the build-cache choice,
retaining a temporary binary and temporary repository. The rerun completed in
under five seconds and produced the expected baseline plus missing
post-lifecycle lines.

The first tmux command inside the filesystem sandbox failed:

```text
error connecting to /tmp/tmux-1000/default (Operation not permitted)
```

I reran the scoped tmux operations with approved access, captured the pane,
sent `q`, and verified the pane disappeared. A capture attempted immediately
after the quit raced with shutdown and returned `can't find pane`, which is
positive cleanup evidence rather than a TUI failure.

### What I learned

- The service log CLI follows one file descriptor and never checks inode or
  size. It misses replacement files and can stall after truncation.
- CLI tailing, `state.TailLines`, and TUI tailing are three implementations
  with different limits and semantics.
- Service output, operator diagnostics, protocol events, TUI events, and
  JavaScript-parsed records are distinct data planes. Presenting all of them
  as “logs” without source/run metadata causes ambiguity.
- `status` is a Glazed `WriterCommand` but manually emits a JSON object, so
  Glazed row processors and normal table/output selection are not actually
  used. Only two non-test command files implement `WriterCommand`.
- Lifecycle syntax is inconsistent: `start SERVICE`, `restart SERVICE`, and
  `stop-service SERVICE`, while logs uses `--service SERVICE`.
- The CLI and TUI duplicate orchestration. The TUI's `down` treats absent state
  as success while the CLI returns an error. The TUI single-service restart
  stops before resolving a fresh plan; the CLI resolves before stopping.
- The TUI action runner also duplicates the unsafe ignored-stop-error behavior.
- The TUI transforms every polled snapshot into an event even when the state
  has not changed.
- Root-model status is derived by parsing text prefixes such as
  `action failed:` and `action ok:` rather than consuming typed outcomes.
- Dashboard `x` signals a single wrapper PID directly, bypassing the
  supervisor's group-aware termination, timeout, state update, and structured
  result.
- Plugin capability introspection launches plugin processes independently of
  normal actions. This may have side effects and makes a display refresh an
  execution operation.
- The TUI has broad feature coverage but no automated tests under `pkg/tui`.

### What was tricky to build

- Protocol stream events must not be merged blindly with service logs. They
  can be high-volume telemetry and have their own stream lifecycle.
- The JavaScript parser is well implemented and documented in isolation. The
  simplification question is not whether that code is low quality; it is
  whether a 1,668-line independent product belongs inside devctl when the main
  operator log path does not use it.
- Old ticket headers often say `complete` while their body says `active` and
  tasks remain unchecked. Current source and executed behavior therefore had
  to outrank metadata.

### What warrants a second pair of eyes

- Confirm the product decision to remove the standalone JavaScript log parser
  from devctl's core repository if a downstream consumer inventory finds no
  external imports.
- Confirm that plugin capability and protocol-stream exploration can move out
  of the default TUI into explicit CLI/developer commands.
- Evaluate whether complex users need an optional “all events” diagnostic view
  after the default TUI is reduced to Overview, Logs, and Runs.

### What should be done in the future

- Build one run-aware log reader used by CLI and TUI.
- Emit typed lifecycle outcomes and let both frontends call the same
  application service.
- Convert snapshot/list commands to true Glazed rows; keep streaming records as
  stable NDJSON-capable rows.
- Consolidate command nouns and positional service selection.
- Retire stale tickets into a superseded index after this design is approved.

### Code review instructions

- Run `scripts/03-log-follow-lifecycle-probe.sh`; only `append-visible` should
  appear under current code.
- Run `scripts/04-tui-state-event-probe.sh`; verify repeated identical state
  events and that the tmux session exits.
- Compare `cmd/devctl/cmds/restart.go:53-74` with
  `pkg/tui/action_runner.go:426-462` to see the resolve/stop ordering
  divergence.
- Compare `cmd/devctl/cmds/down.go:21-37` with
  `pkg/tui/action_runner.go:264-355` to see absent-state divergence and common
  stop-error loss.

### Technical details

Current TUI event transport performs multiple JSON round trips inside one
process:

```text
producer
  -> domain struct
  -> JSON Envelope
  -> Watermill topic devctl.events
  -> unmarshal + type switch
  -> UI struct
  -> JSON Envelope
  -> Watermill topic devctl.ui.msgs
  -> unmarshal + type switch
  -> tea.Program.Send(typed message)
  -> RootModel type switch
```

The useful boundary is the final typed Bubble Tea message. The earlier
in-process JSON and broker layers do not provide persistence, cross-process
transport, replay, or independent deployment.

## Step 4: Compare mature operator interfaces

This step tested the emerging redesign against official documentation for
Process Compose, Docker Compose, Tilt, and systemd's journal. It looked for
stable interaction contracts rather than visual similarity.

### Prompt Context

**User prompt (verbatim):**

> research about possible improvements you could make to devctl while you are
> at it, especially in terms of properly displaying and handling logs,
> ergonomic improvements to the CLI and TUI, things that could be removed and
> simplified that don't seem useful, etc...

**Assistant interpretation:** Use established tools to identify expected
operator primitives and avoid inventing devctl-specific conventions without
evidence.

**Inferred user intent:** Produce a redesign that feels predictable to an
experienced developer while remaining appropriate for devctl's plugin-driven,
repository-local architecture.

### What I did

- Searched only official project and standards documentation for the technical
  comparison.
- Captured six pages with Defuddle under `sources/web/`.
- Added `sources/README.md` with URL provenance and the question each source
  answers.
- Compared:
  - Process Compose's process/log TUI, command palette, bounded log buffer,
    configurable follow/wrap controls, client API, health, and lifecycle
    policies;
  - Docker Compose's `logs [SERVICE...]` and `ps [SERVICE...]` contracts;
  - Tilt's multi-resource, source, level, since, tail, follow, and JSON Lines
    log filters;
  - journalctl's structured fields, time windows, follow modes, and cursor
    concepts.
- Added a comparison matrix, adopted patterns, rejected patterns, and design
  consequences to the primary guide.
- Checked task `6lar`.

### Why

The comparison prevents two opposite errors: retaining complex devctl
behavior merely because it exists, and copying a mature tool's architecture
without sharing its deployment constraints. Devctl needs the operator
contracts of these tools, but not necessarily a daemon, REST server, container
engine, or system journal.

### What worked

- Defuddle produced readable Markdown with normal line structure for all six
  successful captures.
- Docker Compose and Tilt independently converge on positional multi-resource
  selection plus follow, tail, time, and structured-output controls.
- Process Compose independently supports a compact process/log/action TUI
  rather than a primary view for every internal subsystem.
- Process Compose explicitly bounds its UI log buffer and warns about CPU
  cost, reinforcing the need for a defined memory/backpressure contract.
- Docker Compose `ps` demonstrates that a human table and JSON output can be
  two renderings of one row model, matching the intended Glazed design.

### What didn't work

Defuddle returned the same status for both official journalctl URLs:

```text
Error: Failed to fetch: 418 I'm a teapot
```

The attempted URLs were:

```text
https://www.freedesktop.org/software/systemd/man/latest/journalctl.html
https://www.freedesktop.org/software/systemd/man/255/journalctl.html
```

After two attempts I stopped that debugging path as required by the repository
guidelines. The official result had already been read through web search, and
the source register records that the local capture is absent.

The first research-interval commit gate also rejected five trailing spaces
preserved by three Defuddle captures:

```text
sources/web/01-process-compose-tui.md:74: trailing whitespace.
sources/web/01-process-compose-tui.md:120: trailing whitespace.
sources/web/01-process-compose-tui.md:177: trailing whitespace.
sources/web/02-process-compose-client.md:50: trailing whitespace.
sources/web/03-process-compose-lifecycle.md:389: trailing whitespace.
```

No commit was created. I removed only those trailing spaces, kept the captured
content otherwise unchanged, and reran the same staged-diff gate.

### What I learned

- Multi-service log selection is normally positional. Devctl's required
  `--service` flag is not supported by either Docker Compose or Tilt precedent.
- Time filtering and structured streaming output are baseline operator
  features, not advanced parser features.
- A command palette is a better home for infrequent actions than permanent
  primary TUI tabs.
- Process Compose separates server state from CLI/TUI clients, but devctl can
  gain shared semantics through an in-process application service and durable
  files without introducing a daemon.
- Lifecycle status needs more than alive/dead: starting, ready, unhealthy,
  stopping, exited, failed, and unknown are operator-relevant states.
- Explicit success-exit and restart policies are useful product features, but
  adding them now would expand scope before ownership durability is fixed.

### What was tricky to build

- Process Compose offers many attractive features. The relevant lesson is its
  coherent control/log model; replicas, namespaces, interactive PTYs,
  dependency graphs, remote REST clients, and configuration editing are not
  requirements for this ticket.
- journalctl's cursor semantics are valuable, but devctl cannot reuse journal
  cursors directly because service output is stored in ordinary files.
  Devctl needs its own run-local monotonically increasing sequence.

### What warrants a second pair of eyes

- Confirm the decision not to add a long-running devctl daemon or REST API in
  this redesign.
- Confirm that restart policies and service dependencies stay in a later
  ticket rather than entering the reliability MVP.
- Review the proposed default log selection: all services when no positional
  services are supplied.

### What should be done in the future

- Preserve a small list of adopted precedents in API tests so flags do not
  drift.
- Revisit restart policy only after run identity and journal durability ship.
- Consider a daemon only if independent clients or cross-terminal attachment
  becomes a proven workflow.

### Code review instructions

- Read `sources/README.md`, then spot-check each capture against its linked
  official URL.
- Verify that the comparison section distinguishes “adopt” from “do not
  adopt.”
- Confirm task `6lar` is checked and design/implementation tasks remain open.

### Technical details

The comparison dimensions were:

```text
resource selection
  + lifecycle state vocabulary
  + log time/stream/source filters
  + human vs machine output
  + follow cursor/backpressure behavior
  + CLI/TUI semantic sharing
  + action discoverability
  + process ownership model
```
