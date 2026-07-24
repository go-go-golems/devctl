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
    - Path: repo://cmd/devctl/cmds/wrap_service.go
      Note: Request-consuming wrapper and child ownership writer added in Phase 2 milestone
    - Path: repo://pkg/runstate/lock.go
      Note: Repository mutation lock contract and diagnostic metadata added in Phase 1
    - Path: repo://pkg/runstate/schema.go
      Note: Versioned environment run and process identity schemas added in Phase 1
    - Path: repo://pkg/runstate/store.go
      Note: Atomic revision-checked environment and run store added in Phase 1
    - Path: repo://pkg/supervise/supervisor.go
      Note: Planned-run creation and live handshake validation added in Phase 2 milestone
    - Path: repo://pkg/supervise/wrapper_request.go
      Note: Versioned request and ownership handshake contract added in Phase 2 milestone
ExternalSources: []
Summary: Chronological evidence, experiments, failures, and decisions for the devctl operator-experience research ticket.
LastUpdated: 2026-07-24T14:33:16.81503865-04:00
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

## Step 5: Convert evidence into a decision-complete implementation contract

This step wrote the target architecture and the phased guide a new intern will
follow. It resolves schemas, state transitions, package boundaries, log
capture, public commands, TUI views, removals, security rules, tests, and
commit gates.

### Prompt Context

**User prompt (verbatim):**

> Create a detailed analysis / design / implementation guide that is for a new
> intern, explaining all the parts of the system needed to understand what it
> is, with prose paragraphs and bullet point sand pseudocode and diagrams and
> api references and file references. It should be very clear and technical.

**Assistant interpretation:** The document must teach current behavior and
also be sufficiently specific that implementation does not require the intern
to make unreviewed product or architecture decisions.

**Inferred user intent:** Make the next implementation session systematic,
reviewable, and safe instead of another sequence of local fixes.

### What I did

- Replaced the provisional executive summary with measured findings and the
  complete recommendation.
- Defined the target `operator`, `runstate`, `runlog`, `supervise`, CLI, and
  TUI boundaries.
- Specified versioned environment, run, owner, ready, exit, log-record, cursor,
  request, event, result, and error APIs.
- Designed the pre-start durable transaction that closes the current orphan
  window.
- Specified atomic JSON replacement, repository locking, Linux process
  identity, lifecycle phases, reconciliation, health, stop, and restart.
- Designed wrapper-owned raw logs plus a sequenced structured journal.
- Specified exact public CLI commands, log flags, status rows, doctor checks,
  output rules, and exit codes.
- Reduced the TUI to Overview, Logs, and Runs and supplied ASCII wireframes and
  typed Bubble Tea messages.
- Recorded eight explicit design decisions, including removals and the absence
  of compatibility layers.
- Wrote seven implementation phases with file lists, order, tests, acceptance
  gates, and suggested commit messages.
- Added a file-by-file review map, onboarding sequence, verification matrix,
  out-of-scope list, and definition of done.
- Applied the Glazed command-authoring conventions: command descriptions,
  typed settings, shared sections, variadic positional arguments, processors,
  centralized logging/help, and stdout-as-data.
- Checked tasks `k3kw` and `1v87`.

### Why

The evidence identifies interacting failures. Fixing only log rendering would
leave process ownership unsafe; rewriting only the TUI would preserve
duplicated lifecycle semantics. The dependency-ordered design starts with
durability, then unifies control, then builds logs and frontends.

### What worked

- Current source and external precedents converged on one shared controller,
  positional service selection, structured streaming logs, and a smaller TUI.
- A per-run directory provides a stable namespace for state, handshake, logs,
  and exit data without timestamp collisions.
- Wrapper-owned `owner.json` written before child start makes an interrupted
  controller reconcilable without introducing a daemon.
- PID plus process-start token makes signaling decisions explicit and testable.
- Keeping raw stream bytes alongside structured records preserves debugging
  fidelity while providing stable frontend APIs.
- The implementation phases allow meaningful commits instead of one
  unreviewable replacement.

### What didn't work

No tool or edit failed during the design-writing step. Earlier failures and
their corrections remain in the steps where they occurred.

### What I learned

- The state index and per-run record serve different purposes. The index
  answers desired/current ownership; the run record preserves an attempt.
- A wrapper can remain a small durable supervision agent without becoming a
  central daemon.
- Active-run rotation is unnecessary when every attempt has its own immutable
  directory. Retention can operate only on terminal runs.
- A TUI does not need a message broker to be typed and asynchronous; Bubble
  Tea messages plus controller events are sufficient.
- Compatibility migration is especially dangerous for live process ownership.
  Explicit refusal is safer than guessing.

### What was tricky to build

- The startup gap cannot be closed merely by saving state immediately after
  `cmd.Start`: the controller can die between those operations. Pre-creating
  the run reference and having the wrapper persist its own identity before
  starting the child closes both sides of the boundary.
- Combining stdout and stderr requires defining what ordering means. The
  journal records wrapper-observed sequencing and does not claim to reproduce
  simultaneous child writes at a finer resolution.
- Stop is not an all-or-nothing operation. The API must return per-service
  outcomes and preserve unknown ownership instead of hiding partial failure.

### What warrants a second pair of eyes

- Review state/run schemas before implementation because they are the most
  expensive contract to change after shipping.
- Review the decision to fail closed on non-Linux process identity until a
  supported implementation exists.
- Validate that holding the repository mutation lock through startup health
  waits is acceptable; it intentionally serializes lifecycle changes while
  allowing read-only snapshots.
- Confirm the one-time downstream logjs consumer gate and breaking CLI release
  plan.

### What should be done in the future

- Create a separate implementation ticket after this design is accepted.
- Preserve this ticket as the architectural source and write implementation
  deviations as explicit decisions in the execution diary.
- Do not begin Phase 6 TUI work before Phases 1–5 satisfy their acceptance
  gates.

### Code review instructions

- Start at the design Executive Summary and ensure every claim has a detailed
  evidence section.
- Review “Durable run model” and “Lifecycle state machine” before frontend
  sections.
- Verify every removed surface has an explicit replacement or an explicit
  out-of-scope disposition.
- Compare the Definition of Done with the seven phase acceptance gates.
- Confirm tasks `k3kw` and `1v87` are checked while delivery task `eoh0`
  remains open.

### Technical details

The dependency order is:

```text
atomic run state
  -> durable wrapper handshake
    -> shared lifecycle controller
      -> sequenced run logs
        -> Glazed CLI
          -> typed three-view TUI
            -> cleanup, docs, release
```

Reordering frontend work before the controller would recreate the duplicated
semantics this design removes.

## Step 6: Validate, relate, and deliver the design

This step attached authoritative source relations, validated the ticket and Go
repository, rendered the primary guide, and uploaded it to reMarkable.

### Prompt Context

**User prompt (verbatim):**

> Store in the ticket and the nupload to remarkable.

**Assistant interpretation:** Keep the complete guide and audit trail in the
devctl docmgr ticket, then deliver the primary guide as an annotation-friendly
PDF with a confirmed remote destination.

**Inferred user intent:** Make the document available both as versioned project
documentation and as a readable review artifact.

### What I did

- Used `docmgr doc relate` to attach eleven current source files to the primary
  guide with specific notes.
- Used `docmgr meta update` to attach seven authoritative external URLs.
- Ran `docmgr doctor --ticket DEVCTL-OPERATOR-UX-001`; every check passed.
- Verified all tasks except the final delivery/closure task were checked.
- Ran all four ticket-local probes during the investigation.
- Ran `go test ./...`; every package passed.
- Ran a reMarkable dry run with editor layout and ToC depth 2.
- Rendered and uploaded the primary guide as:

  ```text
  /ai/2026/07/24/DEVCTL-OPERATOR-UX-001/
    DEVCTL-OPERATOR-UX-001 - devctl Heavy-User Operator Redesign.pdf
  ```

- Received the confirmed uploader result:

  ```text
  OK: uploaded DEVCTL-OPERATOR-UX-001 - devctl Heavy-User Operator Redesign.pdf -> /ai/2026/07/24/DEVCTL-OPERATOR-UX-001
  ```

### Why

Relations make the source basis discoverable through docmgr. A dry run checks
input, name, layout, and destination without a remote mutation. The real
uploader result is stronger delivery evidence than assuming a locally rendered
PDF reached the device.

### What worked

- Docmgr resolved the repository-local `devctl/ttmp` root.
- Doctor reported `All checks passed`.
- The guide's related-file frontmatter was normalized to `repo://` paths.
- The reMarkable renderer handled the 12,000-plus-word document, code blocks,
  ASCII diagrams, and tables without reporting an error.
- The upload completed in approximately fourteen seconds.
- Current Go tests passed, including wrapper, supervisor, runtime, state, and
  logjs packages.

### What didn't work

I first queried a nonexistent command:

```text
docmgr source --help
Error: unknown command "source" for "docmgr"
```

The current CLI manages local captures through `docmgr import file` and
frontmatter through `docmgr meta update`. The captures already existed in the
ticket, so I used `meta update` for external URL registration and did not
duplicate the files through import.

After closing the ticket, the first final commit gate found:

```text
changelog.md:79: new blank line at EOF.
```

The close operation had succeeded and all tasks remained checked. No commit
was created. I removed only the generated trailing blank line and reran
`git diff --check` before the closing commit.

### What I learned

- `docmgr doctor` accepts ticket-local source registers and direct web captures
  without requiring an import manifest.
- Related-file updates also refresh document metadata timestamps.
- The reMarkable bundle command can render a single explicit Markdown file,
  which avoids bundling the diary and raw source captures into the reader
  document.

### What was tricky to build

- The delivery artifact needed only the polished guide. Bundling the full
  ticket directory would have included thousands of lines of raw upstream
  documentation and diluted the review document.
- Closure metadata must follow confirmed upload, not precede it, so the final
  task is not checked on intent alone.

### What warrants a second pair of eyes

- Open the uploaded PDF on reMarkable and spot-check table wrapping and long
  code blocks; the renderer reported success, but device typography is a
  visual review.
- Review the process-identity and breaking-command decisions before creating
  the implementation ticket.

### What should be done in the future

- Use the proposed Phase 0 downstream-consumer gate before deleting logjs or
  old CLI spellings.
- Create the implementation ticket only after design review comments are
  incorporated.

### Code review instructions

- Run `docmgr doctor --ticket DEVCTL-OPERATOR-UX-001`.
- Run `go test ./...`.
- Inspect the guide's `RelatedFiles` and `ExternalSources` frontmatter.
- Confirm the reMarkable document name and folder against the uploader result.

### Technical details

Validation evidence at delivery:

```text
docmgr doctor: all checks passed
Go packages: go test ./... passed
probe scripts: 01, 02, 03, and 04 executed
reMarkable dry run: passed
reMarkable upload: confirmed
```

## Step 7: Preserve and harden top-level plugin commands

This step corrects the design disposition after product-owner review. Automatic
top-level plugin command injection is a desired devctl capability. The design
must preserve its native `devctl COMMAND` ergonomics while removing the eager,
ambiguous discovery behavior of the current implementation.

### Prompt Context

**User prompt (verbatim):**

> I want to keep the feature, but make it more robust.

**Assistant interpretation:** Revise the completed operator design so dynamic
root commands remain a first-class contract, then specify deterministic
cataloging, collision handling, lifecycle boundaries, diagnostics, and tests.

### What I did

- Re-read `cmd/devctl/cmds/dynamic_commands.go`, its tests, and the protocol
  `CommandSpec` and handshake validation types.
- Replaced every recommendation to remove dynamic root commands.
- Specified a `pkg/plugincatalog` boundary, cache fingerprints, static
  declarations, explicit refresh and inspection commands, reserved names,
  deterministic conflict errors, runtime handshake verification, and
  single-provider execution.
- Extended the CLI tree, architectural decision, implementation phase,
  verification requirements, and file-by-file map.
- Uploaded the revised primary guide to
  `/ai/2026/07/24/DEVCTL-OPERATOR-UX-001` using the default renderer and a
  separate document name so the earlier PDF and any annotations were not
  overwritten.

### Why

The earlier recommendation conflated two separate concerns. Native
repository-specific root commands are useful operator ergonomics. Starting
every plugin while resolving an unknown word is the implementation weakness.
The corrected design retains the former and replaces the latter.

### What warrants a second pair of eyes

- Decide the exact `.devctl.yaml` syntax for optional static command
  declarations before implementation.
- Decide whether a missing catalog should produce an instruction to refresh or
  whether an explicitly enabled `discovery: eager` mode is necessary. The
  design defaults to no implicit execution during typo resolution.
- Confirm whether valid plugin exit codes should pass through exactly or map
  into devctl's global exit-code taxonomy.

## Step 8: Reopen the ticket and establish the implementation baseline

Implementation began by converting the seven design phases into tracked
ticket tasks and capturing an authoritative pre-change baseline. No production
behavior changed in this step. A repository-local test fixture helper now
ensures new state, supervisor, CLI, and TUI tests cannot accidentally use a
developer's real `.devctl` directory.

The destructive-change gate found no external Go imports of `pkg/logjs`, so
the standalone parser can be removed without an extraction package or
compatibility adapter. It also found 42 legacy CLI references outside devctl;
those are a downstream migration inventory rather than a reason to preserve
the old command spellings.

### Prompt Context

**User prompt (verbatim):**

> implement DEVCTL-OPERATOR-UX-001. Keep a detailed frequent diary as you work, commit at appropriate intervals.

**Assistant interpretation:** Execute every implementation phase in the
approved operator redesign, update this diary throughout the work, and create
reviewable commits at phase boundaries.

**Inferred user intent:** Move devctl from an analysis artifact to a reliable
daily operator tool while preserving enough implementation evidence for
review, debugging, and intern handoff.

**Commit (baseline):** `82244d4` — `test(devctl): establish operator redesign baseline`

### What I did

- Changed the ticket index status from `complete` to `active`.
- Added eight tasks covering Phase 0 through Phase 7.
- Added `scripts/05-phase0-baseline.sh` to archive tests, build, lint, and all
  four existing operator probes.
- Added `scripts/06-phase0-consumer-gate.sh` to search sibling go-go-golems
  repositories while excluding historical devctl ticket files and the devctl
  repository itself.
- Archived every command result under `sources/local/phase0`.
- Added `internal/testrepo.Repository`, which uses `testing.TB.TempDir`,
  pre-creates a mode-0700 `.devctl`, and writes mode-0600 test configuration.
- Ran focused tests for the new helper and the existing CLI, supervisor, and
  state packages.

### Why

- The design requires observable before/after evidence for logging and TUI
  behavior.
- Removing `logjs` is permitted only after proving there are no downstream Go
  consumers.
- Future crash, lock, and lifecycle tests need one safe repository fixture
  contract rather than repeated ad hoc temporary-directory setup.

### What worked

- With normal Git cache and tmux access, `go test ./...`, `go build ./...`,
  `make lint`, and all four probes passed.
- The existing log probe reproduced loss after truncation and
  rename-and-create rotation: only `append-visible` reached stdout.
- The tmux capture reproduced repeated identical `state: missing` events at
  the 200 ms polling interval.
- The corrected external search found zero `pkg/logjs` imports and 42 legacy
  CLI references.
- Focused tests passed for `internal/testrepo`, `cmd/devctl/cmds`,
  `pkg/supervise`, and `pkg/state`.

### What didn't work

The first baseline ran inside the restricted filesystem sandbox. The exact
build error was:

```text
error obtaining VCS status: exit status 128
        Use -buildvcs=false to disable VCS stamping.
```

The first lint loader ended with:

```text
level=error msg="Running error: context loading failed: no go files to analyze: running `go mod tidy` may solve the problem"
make: *** [Makefile:17: lint] Error 5
```

The first TUI probe failed before launch:

```text
error connecting to /tmp/tmux-1000/default (Operation not permitted)
```

The first focused test attempt also hit the restricted Go cache:

```text
open /home/manuel/.cache/go-build/f7/f7e402a4335d411cb754764ae210c43725f0d6459f67b586d637ef8f8f994488-d: read-only file system
```

These were environment restrictions rather than software failures. The same
baseline script and focused tests passed with access to Git worktree metadata,
the Go cache, and tmux. The initially recorded failure files were overwritten
by the authoritative successful run; the errors remain recorded here.

The consumer script initially used `--glob '!devctl/**'`, which did not exclude
the repository when the search root was absolute. It reported three internal
imports and 118 mixed matches. Changing the exclusion to
`--glob '!**/go-go-golems/devctl/**'` produced the intended external-only
result.

### What I learned

- Baseline commands that invoke Go VCS stamping or golangci-lint need access to
  the linked worktree's Git metadata.
- The current TUI emits observed polling state as event history even when the
  semantic state is unchanged.
- The parser subsystem is internally documented but has no external Go
  consumer in the searched workspace.
- Active sibling projects still teach the old log and individual-service
  syntax, so Phase 7 must publish a concrete migration inventory.

### What was tricky to build

The downstream search needed to distinguish current external consumers from
devctl's own source, generated help, examples, and historical ticket files.
The output is therefore split by concern, excludes all `ttmp` trees, and
preserves active documentation/script hits instead of collapsing them into a
single count.

The baseline also needed to preserve genuine command results without
misclassifying sandbox restrictions as a red repository. The same script was
run twice; only the unrestricted result is the authoritative summary, while
the first errors are preserved verbatim in this diary.

### What warrants a second pair of eyes

- Review the 42-line downstream migration inventory before Phase 7 and decide
  which sibling repositories should receive follow-up commits outside this
  devctl worktree.
- Confirm that no private or unmounted workspace imports `pkg/logjs`; the
  evidence covers `/home/manuel/code/wesen/go-go-golems`.
- Check whether the test fixture should gain fixture builders for services and
  run records as those schemas land, rather than placing those helpers in
  individual package tests.

### What should be done in the future

- Use `internal/testrepo.New(t)` for every new test that writes `.devctl`
  artifacts.
- Remove the standalone parser during Phase 5 because the Phase 0 consumer
  gate passed.
- Re-run the baseline and behavioral probes after Phases 5 and 6.

### Code review instructions

- Start with `scripts/05-phase0-baseline.sh` and
  `scripts/06-phase0-consumer-gate.sh`.
- Inspect `sources/local/phase0/summary.tsv` and
  `consumer-gate-summary.tsv`.
- Review `internal/testrepo/repository.go` for isolation and file modes.
- Run `go test ./internal/testrepo ./cmd/devctl/cmds ./pkg/supervise ./pkg/state`.

### Technical details

Authoritative baseline:

```text
go-test        0
go-build       0
make-lint      0
probe-01       0
probe-02       0
probe-03       0
probe-04       0
```

Consumer gate:

```text
external_logjs_import_lines    0
external_legacy_cli_lines      42
go_work_files                  1
```

## Step 9: Implement versioned atomic run state and process identity

Phase 1 established the durable storage and ownership primitives required by
every later lifecycle change. The new `pkg/runstate` package owns v2
environment indexes, versioned run records, UUIDv7 attempt IDs, private run
directories, atomic JSON replacement, optimistic revisions, a repository
mutation lock, and platform-specific process identities.

The existing state and exit-info writers now use the same atomic JSON
primitive. Process startup behavior remains unchanged in this phase, but
mutable JSON can no longer be exposed as a partial truncate-and-rewrite
document.

### Prompt Context

**User prompt (verbatim):** (same as Step 8)

**Assistant interpretation:** Implement Phase 1 exactly as the approved
dependency foundation, prove filesystem and concurrency invariants, and
commit it independently before wrapper changes.

**Inferred user intent:** Make later supervision and UI work rest on durable,
testable ownership facts instead of PID-only and non-atomic state.

**Commit (code):** `49513bf` — `feat(runstate): add versioned atomic run records`

### What I did

- Added the v2 `EnvironmentState`, `ServiceSlot`, `RunRecord`,
  `ProcessIdentity`, health, exit, error, and redacted service-spec schemas.
- Added UUIDv7 run ID generation and strict identifier validation.
- Added `Store.CreateEnvironment`, `LoadEnvironment`, revision-checked
  `Update`, `CreateRun`, `LoadRun`, and `UpdateRun`.
- Added mode-0700 `.devctl/runs/<run-id>` directories and mode-0600 JSON
  artifacts.
- Added same-directory temporary writes with full write, file sync, close,
  rename, and directory sync.
- Added injected write, short-write, file-sync, rename, and directory-sync
  tests.
- Added a context-aware advisory repository lock with persisted owner PID,
  process identity, operation ID, command, and acquisition time.
- Added Linux boot ID plus `/proc/<pid>/stat` field-22 process identities.
- Added an explicit unsupported-platform implementation that never claims
  PID-only safety.
- Migrated legacy `state.Save` and `WriteExitInfo` to atomic mode-0600 writes.
- Added deterministic golden JSON, atomic concurrent-reader tests, revision
  conflicts, path escape rejection, lock contention, and process-token tests.

### Why

- A controller cannot safely reconcile or mutate a process using a reusable
  PID alone.
- State readers must always observe the complete old or complete new
  document.
- A repository-wide mutation lock and optimistic state revision solve
  different races and are both required.
- Run directories must exist before wrapper startup in Phase 2, which requires
  validated run IDs and artifact paths first.

### What worked

- `go test -race ./pkg/runstate/... -count=1` passed.
- The full uncached `go test ./... -count=1` suite passed.
- `make lint` passed with zero issues.
- The pre-commit hook independently reran lint and the full suite successfully.
- A Windows cross-build of `pkg/runstate` passed, proving the explicit
  unsupported identity/lock path compiles without Linux APIs.
- `rg 'os\.WriteFile' pkg/state pkg/runstate --glob '*.go'` now finds only an
  old-state fixture write in `pkg/state/state_test.go`.

### What didn't work

The first focused test command inside the restricted sandbox could not read
the configured Go cache:

```text
open /home/manuel/.cache/go-build/39/391a4965a71a7e4405d85c28500dafd60bcba016b53ae7cf0d3672a1aa299ffe-d: read-only file system
```

It was rerun with normal cache access.

The first formatted test run then found a missing closing brace in the
concurrent-reader goroutine:

```text
pkg/runstate/store_test.go:106:4: expected ';', found '('
pkg/runstate/store_test.go:122:2: expression in go must be function call
pkg/runstate/store_test.go:124:6: expected '(', found TestCreateAndUpdateRun
pkg/runstate/store_test.go:156:6: expected '(', found TestPathsRejectEscapesAndInvalidIdentifiers
pkg/runstate/store_test.go:172:3: expected '}', found 'EOF'
```

The goroutine, loop, and select scopes were corrected, formatted, and the same
focused test passed on the second software attempt.

### What I learned

- Go's JSON encoder already sorts string map keys, so the environment map has
  deterministic fixture output without an ordered-map dependency.
- Linux process stat parsing must locate the final `)` before counting from
  field 3; splitting the whole line cannot support valid command names.
- A directory-sync error occurs after the rename and must be reported even
  though the new file is already visible. The test asserts that exact
  durability ambiguity.
- Repository locks and state revisions cannot substitute for one another:
  flock serializes lifecycle operations, while revisions reject stale
  in-memory snapshots.

### What was tricky to build

Atomic replacement has different failure semantics before and after rename.
Write, short-write, file-sync, and rename failures preserve the old document
and remove the temporary file. Directory-sync failure is later: the
replacement is visible, but its survival across a crash is not proven, so the
operation returns an error while readers see the new revision.

The lock file is both a kernel lock target and a diagnostic artifact. Metadata
is written only after flock succeeds, and a contender reads it only after its
nonblocking lock attempt fails. Cancellation returns both
`ErrOperationBusy` and the context cause, allowing callers to classify the
operation while still showing the owner.

### What warrants a second pair of eyes

- Review whether controller-facing errors should wrap the runstate sentinel
  errors directly or translate them once into operator error codes.
- Confirm UUIDv7 is the desired stable run ID over ULID; both satisfy the
  approved design, and UUIDv7 support was already available through the
  repository dependency graph.
- Review directory-sync behavior on non-Linux Unix filesystems before claiming
  identical crash guarantees there.

### What should be done in the future

- Phase 2 must pre-create `RunRecord` and the environment slot before wrapper
  execution.
- The future controller must acquire `RepositoryLocker` around every
  lifecycle mutation rather than embedding lock acquisition inside `Store`.
- `doctor` must expose lock owner identity and stale-owner diagnostics without
  automatically stealing a lock.

### Code review instructions

- Start with `pkg/runstate/schema.go`, then read `store.go`, `atomic.go`,
  `lock_unix.go`, and `identity_linux.go`.
- Inspect `atomic_test.go` for the old/new visibility boundary.
- Inspect `lock_test.go` for cancellation and owner reporting.
- Run `go test -race ./pkg/runstate/... -count=1`.
- Run `go test ./... -count=1` and `make lint`.

### Technical details

The single-writer boundary after this phase is:

```text
controller (future) -> state.json, run.json via runstate.Store
wrapper (future)    -> owner.json, ready.json, exit.json via atomic JSON
lock holder         -> .devctl/lock through its locked file descriptor
```

State mutation:

```text
read + validate revision N
clone
apply mutation
set revision N+1 and updated_at
write temp -> fsync -> rename -> fsync directory
```

## Step 10: Replace wrapper flags with a durable ownership handshake

The first Phase 2 milestone replaced the hidden wrapper's command, environment,
log, and readiness flags with one versioned, mode-0600 request file. The
wrapper deletes that secret-bearing request before process startup, writes its
own process identity to `owner.json`, starts the child in a distinct process
group, and writes a validated child identity and PGID to `ready.json`.

The production supervisor now creates a UUIDv7 run directory and planned
`run.json` before launching the wrapper. It validates both handshake records
against live Linux process identities and the kernel-reported child process
group before returning a service record. This is a milestone rather than the
end of Phase 2: the v2 environment slot must still be persisted before launch,
and failed-attempt reconciliation must be completed with Phase 3 controller
ownership.

### Prompt Context

**User prompt (verbatim):** (same as Step 8)

**Assistant interpretation:** Implement the wrapper half of Phase 2 without
weakening the signal fix, prove it through the real self-executing devctl
binary, and commit the milestone before changing lifecycle ownership.

**Inferred user intent:** Ensure every supervised process has durable,
cryptographically unambiguous ownership evidence that later commands can
reconcile safely.

**Commit (code):** `352c815` — `feat(supervise): add durable wrapper handshake`

### What I did

- Added the versioned `WrapperRequest`, `OwnerRecord`, and `ReadyRecord`
  contracts in `pkg/supervise/wrapper_request.go`.
- Derived all wrapper artifact paths from the validated repository/run
  directory instead of accepting independent arbitrary paths.
- Changed `__wrap-service` to accept only `--request PATH`.
- Made the wrapper remove the request before opening logs or starting the
  child.
- Wrote `owner.json` atomically before `exec.Cmd.Start`.
- Captured the live child process start token and actual PGID, required the
  child to lead its own group, and wrote `ready.json` atomically.
- Preserved wrapper-to-child `SIGTERM`, `SIGINT`, and `SIGHUP` forwarding while
  giving the signal goroutine an explicit shutdown path.
- Recorded setup, child-start, and handshake failures in atomic `exit.json`
  files.
- Changed production wrapper launches to pass no raw environment or command
  values in process arguments.
- Added transitional run ID, wrapper token, child identity, and child PGID
  fields to the existing service record so current frontends retain the
  ownership facts until the v2 index migration.
- Added a real-binary supervisor integration test plus request privacy,
  path-binding, missing-owner, missing-ready, tampered identity, and tampered
  PGID tests.

### Why

- Independent wrapper flags allowed paths and identity fields to disagree.
- The old ready file proved only that some PID text had been written; it did
  not bind the child to the run, service, wrapper, boot, start time, or process
  group.
- Raw service environment values in wrapper process arguments were visible to
  local process inspection.
- The wrapper must establish ownership before a child can execute so a
  controller crash always leaves reconcilable evidence.

### What worked

- The existing SIGHUP regression test still proves the wrapper itself is not
  killed while the child receives the forwarded hangup.
- The new integration test builds the real `devctl` binary, launches it as its
  own wrapper, and verifies `run.json`, consumed `request.json`, `owner.json`,
  `ready.json`, live identities, child PGID, redacted environment, and clean
  shutdown.
- `go test -race ./pkg/supervise ./cmd/devctl/cmds -count=1` passed.
- The pre-commit lint and complete Go suite passed.
- Tampering with run ID, child PGID, or child start token causes handshake
  validation to fail.
- Missing owner and missing ready records return distinct sentinel errors
  joined with the context deadline.

### What didn't work

N/A. The request/handshake implementation and its focused, integration, race,
lint, and full-suite gates passed on their first software execution.

### What I learned

- Validating `ChildPGID == ChildPID` in JSON is insufficient; the supervisor
  must also query `getpgid(childPID)` and compare the kernel result.
- The request file is the only wrapper artifact allowed to contain raw
  environment values, so failure before wrapper consumption needs explicit
  parent-side cleanup.
- Owner and ready timeouts represent different recovery states and must remain
  separately classifiable.
- A real built devctl executable is necessary to test self-wrapping; a Go test
  binary does not route arbitrary `__wrap-service` arguments through Cobra.

### What was tricky to build

The wrapper and child deliberately occupy different process groups. The parent
launches the wrapper as a group leader, the wrapper confirms its own group,
and the child starts as another group leader. Stop signals target the wrapper
group; the wrapper forwards the same signal to the child group. Handshake
validation therefore must not assume the wrapper and child share a PGID.

Request consumption also has a narrow ordering contract: load and validate,
remove the mode-0600 secret file, establish wrapper identity, atomically write
owner, and only then start the child. Moving any of those operations changes
the crash-recovery evidence.

### What warrants a second pair of eyes

- Review the behavior when `owner.json` exists but opening a raw log fails;
  the wrapper records `exit.json`, and reconciliation must classify that as a
  failed attempt with no child.
- Confirm the two-second wrapper-handshake timeout should become an explicit
  supervisor option instead of remaining a focused internal bound.
- Review whether signal-forwarding errors should be represented as system log
  records once `pkg/runlog` exists.

### What should be done in the future

- Complete Phase 2 by creating the v2 environment index and current-run slots
  before any wrapper launch.
- Reconcile `exit.json` into terminal run phases and never overwrite the v2
  environment index with the transitional legacy state.
- Move lifecycle policy and failure aggregation into `pkg/operator`; the
  supervisor should retain only process primitives.

### Code review instructions

- Start with `pkg/supervise/wrapper_request.go` and
  `cmd/devctl/cmds/wrap_service.go`.
- Follow `Supervisor.startService` from `CreateRun` through
  `waitWrapperHandshake`.
- Inspect `supervisor_wrapper_test.go` for the real-binary evidence.
- Run `go test -race ./pkg/supervise ./cmd/devctl/cmds -count=1`.

### Technical details

Wrapper launch and evidence order:

```text
controller/supervisor:
  create run directory
  write run.json(planned)
  write request.json(0600)
  write run.json(starting)
  exec devctl __wrap-service --request request.json

wrapper:
  load + validate request
  remove request
  write owner.json(wrapper PID + boot/start token)
  start child in child-led process group
  write ready.json(wrapper + child identities + actual PGID)
  wait and forward signals
  write exit.json
```

## Step 11 — Establish the typed operator lifecycle boundary

### Prompt

(same as Step 8)

### What I did

- Added typed request, result, event, error, planner, and controller contracts
  in `pkg/operator`.
- Added the initial `pkg/runlog` contracts so the controller owns a stable log
  reader boundary before any CLI or TUI migration.
- Split supervisor process primitives into prepared-start, health-completion,
  and prepared-stop methods.
- Implemented `up`, `down`, `restart`, `snapshot`, and initial `doctor`
  controller operations.
- Made `up` create every planned run record and atomically index every current
  run in the version-2 environment document before starting the first wrapper.
- Made `down` retain the environment document and run history, clearing a
  current-run pointer only after termination is proven.
- Changed `restart` to resolve and validate the complete launch plan before
  acquiring the lifecycle lock, then perform stop and start under one
  repository lock and one operation ID.
- Added focused controller tests with an observing fake supervisor. The fake
  loads `state.json` and `run.json` at the start boundary, proving a wrapper
  cannot begin before its run is durably indexed.

### Why

The existing commands combined planning, rendering, state mutation, process
control, and output policy. A typed controller is necessary so Cobra and
Bubble Tea can become consumers of one lifecycle implementation rather than
independent implementations.

Restart cannot be implemented as a public `down` call followed by a public
`up` call. That composition releases the lock, creates two operation
identities, and permits another process to mutate the repository between stop
and start.

### What worked

- `go test ./pkg/operator ./pkg/supervise ./pkg/runstate` passes.
- The tests prove:
  - a run and its current-run slot exist before wrapper start;
  - an unknown explicit selection fails before state mutation;
  - `down` with no state is a successful no-op;
  - restart performs planning before the first stop;
  - restart reports one operation and executes both phases under its internal
    locked callback.

### What didn't work

The first restart test panicked in `errors.Is`. `selectStateServices` returns a
`*OperatorError`; assigning its nil pointer to a previously declared `error`
interface produced a non-nil interface containing a nil pointer. Restart then
called `Unwrap` on that nil receiver.

I changed the selection result to a distinct `selectionErr` pointer and return
it only after an explicit non-nil check. The focused suite passed afterward.

### What I learned

Typed errors improve CLI stability only when pointer-valued errors do not
cross interface boundaries as typed nils. Selection helpers should retain
their concrete pointer type until the non-nil branch.

The state-before-process invariant is easiest to prove at the supervisor
boundary. A test that inspects files after `up` completes would not detect an
incorrect transient ordering.

### What was tricky to build

Restart needs both pre-lock and in-lock validation. Planning and selection must
finish before any process is stopped, while current-run ownership can only be
trusted after acquiring the repository lock. The current implementation
therefore resolves immutable launch intent first and performs state-dependent
stop/start decisions inside one exclusive callback.

### What warrants a second pair of eyes

- Review whether a partial restart should start services whose old runs were
  proven stopped when another selected service could not be stopped. The
  current safe policy aborts every start after any unproven termination.
- Confirm whether restart results should retain separate stop and start
  outcomes for the same service or introduce an explicit phase field.
- Review failure-code classification between wrapper start, handshake, and
  health failures before the public CLI exposes stable exit mappings.

### What should be done in the future

- Add reconciliation before every mutating operation.
- Classify exit artifacts, identity mismatch, missing evidence, and orphaned
  runs without guessing.
- Add race tests for concurrent snapshot reads and locked lifecycle mutations.
- Replace direct lifecycle logic in Cobra and Bubble Tea only after these
  controller invariants are complete.

### Code review instructions

- Start at `pkg/operator/controller.go`.
- Inspect `upLocked` for the create-run, index-state, start-wrapper ordering.
- Inspect `Restart` for one lock and one operation ID.
- Run `go test ./pkg/operator ./pkg/supervise ./pkg/runstate`.

### Technical details

```text
restart request
  -> resolve plugins/config/build/prepare/validate/launch plan
  -> validate explicit selection
  -> acquire repository lifecycle lock
     -> load and validate current run ownership
     -> stop selected current runs
     -> abort if any termination is unproven
     -> create replacement run records
     -> atomically index all replacement run IDs
     -> start wrappers and complete health checks
  -> aggregate deterministic per-service outcomes
```

## Step 12 — Reconcile crash evidence and prove partial stop semantics

### Prompt

(same as Step 8)

### What I did

- Added conservative reconciliation in `pkg/operator/reconcile.go`.
- Made reconciliation run under the repository lock before `up`, `down`, and
  each locked phase of `restart`.
- Made `doctor` acquire the lifecycle lock, reconcile, and include the
  structured reconciliation report in its result.
- Added Linux process-group inspection beside PID/start-token inspection, with
  an explicit unsupported-platform result on non-Linux builds.
- Reconciled:
  - planned runs with no owner into authoritative failed attempts;
  - valid owner-only evidence into `starting`;
  - valid live owner/ready evidence into `ready` when no health gate exists;
  - health-gated live handshakes into `starting` pending health evaluation;
  - exit artifacts with absent owned processes into `failed` or `exited`;
  - identity, handshake, PGID, artifact, and liveness contradictions into
    `unknown`.
- Cleared current-run slots only for terminal runs whose lack of live
  ownership was proven.
- Scanned unreferenced run directories and reported them without signaling or
  deleting anything.
- Extended the observing supervisor test double to launch a real isolated
  process group and emit valid owner/ready evidence.
- Added reconciliation tests and a two-service partial-stop test.

### Why

Pre-creating a run closes the startup indexing gap but deliberately creates
intermediate states after controller interruption. A later command needs to
interpret those states from durable evidence before deciding whether a new
process may start or an old process may be signaled.

An unindexed run directory is not ownership proof. Automatic signaling based
on directory names or command strings would reintroduce PID-reuse and
cross-process hazards.

### What worked

- `go test ./pkg/operator ./pkg/runstate ./pkg/supervise` passed.
- `go test -race ./pkg/operator ./pkg/runstate ./pkg/supervise` passed.
- A planned run with no owner becomes failed and moves from
  `current_run_id` to `last_run_id`.
- A valid exit artifact with absent identities becomes an exited run with its
  exit code retained.
- An unindexed run remains untouched and appears in the report.
- In a two-service stop, one injected failure produces a partial operation,
  retains that service's current run and desired-running state, and still
  clears the successfully stopped service.

### What didn't work

N/A. The focused and race suites passed on the first execution of this
reconciliation slice.

### What I learned

The fake supervisor must produce the same durable ownership contract as the
real supervisor. Marking a fake run `ready` without owner, ready, and live
identity evidence correctly becomes a reconciliation contradiction.

Reconciliation must distinguish a recovered live handshake with a health
contract from a ready service. Durable process ownership proves the child is
owned; it does not prove application health.

### What was tricky to build

Exit evidence and live-process evidence can contradict each other. The
reconciler must not prefer the exit file merely because it is terminal-looking:
an exit artifact plus a still-matching process becomes `unknown`.

The process group in `ready.json` must be checked against the kernel, not only
against the child PID encoded in JSON. Otherwise a tampered artifact could
cause a later stop to address the wrong group.

### What warrants a second pair of eyes

- Review whether `doctor` should always reconcile mutably or gain a distinct
  report-only mode before public CLI migration.
- Review the classification of a wrapper that disappears after writing only
  `owner.json`; the current implementation preserves it as `unknown`.
- Confirm that health-gated recovered handshakes should be evaluated
  synchronously by the next mutation or exclusively by an explicit
  reconciliation/doctor policy.

### What should be done in the future

- Add cancellation-boundary tests around state indexing and wrapper start.
- Classify start, handshake, and health errors with distinct stable codes.
- Ensure failed health starts terminate their owned process before the run is
  considered authoritatively failed.
- Move Cobra and Bubble Tea onto the controller, then delete their lifecycle
  policy.

### Code review instructions

- Start with `pkg/operator/reconcile.go`.
- Check every branch that returns `terminal=true`; those are the only branches
  allowed to clear a current-run slot.
- Inspect the partial-stop test in `pkg/operator/controller_test.go`.
- Run both focused normal and race commands listed above.

### Technical details

```text
current run
  ├─ no owner, planned, no stored identity
  │    -> failed (start never observed) -> clear current
  ├─ owner live, ready absent
  │    -> starting
  ├─ owner + ready live, PGID matches kernel
  │    ├─ no health contract -> ready
  │    └─ health contract    -> starting (health not yet proven)
  ├─ exit exists, all owned identities absent
  │    -> failed/exited -> clear current
  └─ artifact or identity contradiction
       -> unknown -> retain current, never guess or signal
```

## Step 13 — Make unhealthy and canceled starts ownership-safe

### Prompt

(same as Step 8)

### What I did

- Split wrapper start, wrapper handshake, and health failures into stable
  operator error classifications.
- Emitted an explicit `service.unhealthy` event before final service failure.
- On health failure, stopped the owned wrapper/child process group with a
  bounded cleanup context.
- Preserved the failed health result and exit evidence after cleanup by
  returning the run to the authoritative `failed` phase only after termination
  succeeded.
- Stopped launching subsequent services when the caller context is canceled.
- Added tests for failed-health cleanup and cancellation during the first
  service start of a two-service operation.

### Why

`failed` is defined as authoritative failure evidence with no matching live
child. Marking a health-gated run failed while leaving its child running would
make state unsafe and cause later reconciliation to discover a contradiction.

Cancellation must stop new work but cannot erase already committed intent.
Because all selected run records and slots are created before wrapper launch,
the safe result is durable planned attempts that reconciliation can classify.

### What worked

- `go test ./pkg/operator ./pkg/supervise ./pkg/runstate` passed.
- `go test -race ./pkg/operator` passed.
- The full pre-commit test suite and lint passed.
- A health failure now yields `E_HEALTH_TIMEOUT`, an absent owned process, a
  retained unhealthy health result, and a failed run.
- Cancellation during the first start prevents the second start attempt.
- Both pre-indexed planned runs are later reconciled to failed history and
  removed from current slots.

### What didn't work

N/A. The implementation and tests passed on their first execution.

### What I learned

Lifecycle phase and liveness must be updated as one semantic operation even
when they require several durable writes. The safe intermediate state is a
current run with ownership evidence, not a terminal label applied early.

Pre-indexing all selected runs makes cancellation recovery deterministic. It
also means cancellation tests must verify the unattempted runs, not merely
assert that a second process was not launched.

### What was tricky to build

`StopPreparedService` correctly records `exited`, while startup semantics need
the terminal attempt to remain `failed`. The controller therefore performs
bounded stop first, retains the exit summary written by stop, and changes only
the phase back to failed after proven termination.

### What warrants a second pair of eyes

- Review whether cleanup failure after a health timeout should expose
  `E_STOP_FAILED` as the primary code or retain `E_HEALTH_TIMEOUT` with joined
  causal details. The current result retains the health code and leaves the
  supervisor's unknown state intact.
- Confirm whether cancellation should emit a final operation event even when
  the canceled context prevents the event sink from accepting it.

### What should be done in the future

- Add event-sink coverage for exact ordering and best-effort sink failures.
- Add raw-secret absence assertions at the controller result/artifact layer.
- Complete sequenced log capture before migrating user-facing logs commands.

### Code review instructions

- Inspect the start loop in `pkg/operator/controller.go`.
- Follow the health error through cleanup, run reload, and outcome creation.
- Inspect the cancellation test to confirm both indexed attempts reconcile.
- Run the focused normal and race commands above.

### Technical details

```text
health failure
  -> persist unhealthy result
  -> emit service.unhealthy
  -> bounded owned-process stop
  -> persist exit evidence
  -> mark attempt failed (only after absence is proven)
  -> return E_HEALTH_TIMEOUT outcome

caller cancellation during start
  -> retain all pre-indexed planned runs
  -> stop launching additional wrappers
  -> return canceled operation
  -> next reconciliation fails unowned planned attempts safely
```

## Step 14 — Add the run-scoped structured log journal

### Prompt

(same as Step 8)

### What I did

- Implemented a byte-oriented framer with newline handling, CRLF text
  normalization, final EOF fragments, and bounded partial chunks for logical
  lines over 1 MiB.
- Implemented a bounded two-stream capture pipeline:
  - exact bytes are written to the matching raw stdout/stderr file;
  - framed records fan into one sequencer;
  - the sequencer alone assigns run-local sequence numbers and writes
    `logs.jsonl`;
  - every complete record is flushed for live readers;
  - raw files and the journal are synced before capture completion.
- Changed the wrapper from direct `child.Stdout`/`child.Stderr` files to
  `StdoutPipe`/`StderrPipe` capture.
- Made first-stream capture failure cancel capture, close both child pipes, and
  return promptly even if the peer stream is blocked.
- Made the wrapper kill the child process group when capture fails and wait for
  capture completion before writing `exit.json`.
- Implemented a file-backed reader with:
  - composable run/service/source/stream/level/time/text filters;
  - tail applied independently per run before stable merge;
  - `(time, run ID, sequence)` ordering;
  - terminated-record corruption errors with byte offsets;
  - ignored unterminated crash fragments with
    `E_LOG_TRAILING_PARTIAL`;
  - cursor-based follow without duplicate emission;
  - active-file replacement detection;
  - context-aware polling and terminal-run completion.
- Extended the real-binary supervisor test to prove journal content and exact
  raw bytes.
- Added a Linux `/dev/full` wrapper integration test and a 100,000-record
  bounded-pipeline test.

### Why

Separate raw files cannot reconstruct stdout/stderr ordering and forced the
CLI and TUI to implement independent tail logic. A per-run journal gives both
frontends one immutable identity, one cursor domain, and deterministic record
ordering.

Capture cannot depend on frontend speed. Frontends read the journal from disk;
the child-to-disk path has a small bounded channel and no TUI callback.

### What worked

- `go test ./pkg/runlog ./pkg/supervise ./cmd/devctl/cmds` passed.
- `go test -race ./pkg/runlog` passed.
- The complete pre-commit suite and lint passed.
- The real wrapper produced two records with unique sequences across stdout
  and stderr while preserving `out\r\n` exactly in the raw stdout file and
  exposing normalized `out` in the journal.
- A blocked peer pipe no longer delays the first raw/journal write error.
- `/dev/full` caused prompt wrapper failure, killed the child, and produced an
  exit record naming log capture failure.
- 100,000 records completed with the in-process frame queue fixed at 64.
- Follow cancellation completed within 250 ms, and cursor resume emitted no
  duplicate.

### What didn't work

The initial writer waited for `errgroup.Wait()` before returning a stream
failure. If stdout hit disk-full while stderr remained open and silent,
capture could not return to the wrapper, so the wrapper could not kill the
child that would close stderr.

I added a first-error channel, canceled the group, and closed every pipe reader
before waiting for reader goroutines. A regression test uses one failing raw
writer and one blocking `io.Pipe`; it now returns within 250 ms.

### What I learned

Flushing only at process exit makes a correct journal useless for `logs
--follow`. Complete JSON records must become visible during the active run.
The current MVP flushes every record and reserves batched count/time tuning
for later profiling.

`errgroup` cancellation alone does not interrupt an arbitrary blocking
`Read`. The owner of pipe readers must close them when another capture
component fails.

### What was tricky to build

`child.Wait`, pipe EOF, journal sync, and exit metadata have a strict order.
The wrapper waits for both process completion and capture completion, but if
capture fails first it kills the child group before waiting. Handshake-failure
cleanup also waits for capture after killing the child.

An active reader may see a crash fragment or the short interval between an
append and its newline. The reader ignores only an unterminated final line; a
terminated invalid line is hard corruption.

### What warrants a second pair of eyes

- Review the per-record flush cost under sustained multi-megabyte output and
  choose a bounded count/time flush policy without sacrificing live latency.
- Review whether `unknown` runs should terminate follow or require explicit
  caller cancellation. The current reader treats only failed and exited as
  terminal.
- Confirm whether query diagnostics should remain a non-nil error returned
  alongside valid records or move to an explicit diagnostic collection.

### What should be done in the future

- Wire the file reader into the Glazed `logs` command and TUI.
- Add CLI presentation rules for trailing-partial diagnostics.
- Add optional ANSI stripping only at rendering time; raw bytes and journal
  text currently retain ANSI content.
- Measure query allocation for very large retained histories and add streaming
  iteration if needed.

### Code review instructions

- Start with `pkg/runlog/writer.go`, then `framer.go`.
- Follow `runWrappedService` from pipe creation through capture/process select
  and exit writing.
- Inspect `reader.go` and `follow.go` for corruption and cursor behavior.
- Run the focused, race, disk-full, and full suite commands above.

### Technical details

```text
child stdout -> pipe -> exact stdout.log -> framer --\
                                                        -> sequencer -> logs.jsonl
child stderr -> pipe -> exact stderr.log -> framer --/

first capture error
  -> cancel capture
  -> close both pipe readers
  -> wrapper kills child process group
  -> wait child + capture
  -> write exit.json with capture failure
```

## Step 15 — Replace CLI lifecycle policy and bound dynamic command injection

### Prompt

(same as Step 8)

### What I did

- Deleted the old Cobra-local `up`, `down`, and single-service `restart`
  implementations and replaced them with one Glazed lifecycle command type
  calling `operator.Controller`.
- Added positional service lists to `up`, `down`, `restart`, `status`, and
  `logs`.
- Deleted `start-service` and `stop-service` without aliases.
- Replaced status JSON marshaling with one Glazed row per durable service.
- Replaced CLI-local raw-file tailing with `runlog.Reader` query/follow,
  cursors, composable filters, time parsing, and ANSI presentation policy.
- Added a Glazed read-only `doctor` command and expanded controller diagnostics
  to cover:
  - configuration/profile validation;
  - opt-in plugin handshakes;
  - state/run schema reads;
  - PID/start-token and PGID checks;
  - health configuration;
  - journal continuity and trailing crash fragments;
  - unindexed run directories;
  - artifact disk usage.
- Proved `doctor` does not reconcile or mutate a planned run.
- Added static plugin command declarations under plugin configuration.
- Added `pkg/plugincatalog` with:
  - schema-versioned mode-0600 cache;
  - profile/config/plugin/executable fingerprinting;
  - deterministic command entries and conflicts;
  - reserved root-name validation;
  - profile isolation;
  - static declarations that require no discovery process;
  - explicit handshake discovery in `plugins refresh`.
- Changed automatic top-level injection to read only a validated cache or
  trusted static declarations.
- Made a dynamic invocation start only the recorded provider and reject
  handshake drift with `PLUGIN_CATALOG_STALE`.
- Replaced `plugins list` with configuration-only rows and added
  `plugins commands` and `plugins refresh`.
- Removed the standalone `cmd/log-parse`, `pkg/logjs`, its examples, and its
  embedded help topic after the Phase 0 consumer gate found no production
  imports.
- Added real-binary CLI contract tests.

### Why

Cobra and the old TUI cannot remain alternate lifecycle implementations. The
CLI must decode presentation settings, call the controller, and render typed
results.

Automatic root command injection is useful, but executing every configured
plugin while handling an unknown word makes typo parsing a repository-code
execution surface. A fingerprinted cache preserves native root commands while
making help, completion, built-ins, and typos side-effect bounded.

### What worked

- The full pre-commit test suite and lint passed.
- Real-binary tests prove:
  - no-state status emits valid structured JSON with empty stderr;
  - `stop-service` is an unknown command;
  - root help in a configured plugin repository creates no provider-start
    marker;
  - a typo with no cache creates no provider-start marker and explains
    `plugins refresh`;
  - after refresh, one dynamic command invocation starts exactly one provider.
- Catalog tests prove:
  - fingerprints are stable across plugin ordering;
  - plugin/plugin collisions list providers deterministically;
  - reserved names are rejected;
  - a cache generated for one profile is stale for another.
- `go run ./cmd/devctl --help`, `logs --help`, `up --help`, and no-state JSON
  status all succeeded.
- The logs-specific Glazed section preserves the required `--stream` filter
  flag by removing Glazed's conflicting output-stream flag only for that
  command.

### What didn't work

The first logs command construction failed because Glazed already defines an
output flag named `--stream`. Renaming the public log filter would violate the
CLI contract. I constructed a command-specific Glazed section, removed only
the output formatter's `stream` definition, and retained every other output
processor flag.

The old dynamic-command tests initially failed because they expected implicit
handshake discovery during root construction. I changed them to invoke
explicit catalog refresh and added separate cache-missing/static-declaration
tests.

Runtime drift validation initially treated nil and empty `ArgsSpec` slices as
different through `reflect.DeepEqual`. JSON handshake decoding can produce an
empty slice for semantically empty metadata. I replaced reflection with an
element-wise semantic comparison.

### What I learned

Glazed command construction clones and injects its output schema. A command
that intentionally reserves an output flag name must supply a prebuilt Glazed
section; hiding the duplicate flag happens too late to prevent registration
collision.

Root command construction can load and fingerprint repository configuration
without executing repository plugins. The execution boundary is now explicit:
refresh starts discovery providers; a selected dynamic command starts one
provider; other parsing starts zero.

### What was tricky to build

The catalog fingerprint must be stable across configuration ordering while
changing for profile selection, executable identity, arguments, priority,
working directory, environment, static declarations, schema version, or
protocol version. The implementation sorts selected plugin specs before
hashing canonical JSON material.

Dynamic execution must verify the runtime handshake before running
`config.mutate` or `command.run`. It compares provider identity, operation
support, and the complete recorded command set, then uses a bounded close
context.

### What warrants a second pair of eyes

- Review whether static command declarations should require a handshake during
  `plugins refresh`; the current preferred static path avoids discovery and
  verifies at actual execution.
- Review the current Glazed streaming JSON behavior against the strict JSON
  Lines requirement for `logs --follow`.
- Review root error rendering because Glazed's builder invokes
  `cobra.CheckErr` internally for command execution errors.
- Review whether `doctor --plugins` should attribute plugin IDs in a dedicated
  field instead of the generic service column.

### What should be done in the future

- Complete centralized exit-code/error rendering and exact help goldens.
- Add `plugins inspect` and provider-qualified `plugins run` if they remain in
  MVP scope.
- Update user/help documentation and remove old command strings outside
  historical tickets.
- Replace the legacy TUI so no second lifecycle implementation remains.

### Code review instructions

- Start with `cmd/devctl/cmds/lifecycle.go`, `logs.go`, and `doctor.go`.
- Inspect `pkg/plugincatalog/catalog.go` and the root injection path in
  `dynamic_commands.go`.
- Inspect `cli_contract_test.go` for real-process side-effect assertions.
- Verify removed commands with
  `rg 'stop-service|start-service|logs --service' --glob '!ttmp/**'`.
- Run `go test ./...` and `make lint`.

### Technical details

```text
root construction
  -> parse bootstrap repo/config/profile flags
  -> built-in/help/completion? return without catalog or plugin
  -> load repository config (no plugin execution)
  -> load and fingerprint-check cache
     -> selected dynamic name? register one provider command
     -> typo? normal error / refresh guidance, zero providers

plugins refresh
  -> use trusted static declarations where present
  -> start only providers requiring handshake discovery
  -> reject reserved and ambiguous names
  -> atomically persist mode-0600 catalog

dynamic invocation
  -> start recorded provider only
  -> verify handshake against catalog
  -> config.mutate
  -> command.run
  -> bounded provider shutdown
```

## Step 16 — Replace process-exiting command adapters and centralize failures

I replaced Glazed's stock Cobra adapter for the lifecycle, status, logs,
doctor, and plugin table commands. The stock adapter calls `cobra.CheckErr`
inside the generated `RunE` closure. That function prints and terminates the
process immediately, so the root command cannot assign the documented exit
code, suppress duplicate diagnostics, or finish a partially emitted table.
The local adapter now uses Glazed's public Cobra parser and table processor
APIs and returns every parse, execution, and close error to the root.

The first adapter test reported `unknown flag: --repo-root`. I had cloned the
command description with `Clone(false)`, which copies metadata but creates an
empty schema. Changing that call to `Clone(true)` preserved every command
section before adding the Glazed output section. The focused command and
operator tests then passed.

I removed every `cobra.CheckErr` call from the executable root. One renderer
now prints exactly one diagnostic and maps failures to the operator contract:

- `0` for success;
- `1` for runtime and partial-operation failures;
- `2` for usage, configuration, schema, and catalog bootstrap failures; and
- `130` for canceled or interrupted operations.

Cobra's untyped parser errors are normalized to `E_USAGE`, while typed
`OperatorError` values retain their stable code and message. The executable
now runs under `signal.NotifyContext`, so Ctrl-C reaches commands as context
cancellation before the root maps it to exit 130.

Real-binary contract tests now prove that a removed command exits 2, contains
one copy of the Cobra message, and carries the `E_USAGE` code. They also prove
that a dynamic-command cache miss exits 2 without starting the provider.
Unit tests cover runtime, usage, and cancellation classification plus the
single-line renderer.

### Commands and results

```text
go test ./cmd/devctl/cmds ./pkg/operator
  PASS

go test ./cmd/devctl/... ./pkg/operator
  first run: compile failure because the test helper name `exitCode` collided
  with an existing package helper

go test ./cmd/devctl/... ./pkg/operator
  PASS after renaming the test-only helper to `processExitCode`
```

### Remaining robustness work for dynamic injection

The user explicitly chose to retain automatic top-level command injection.
The implementation must therefore continue to treat it as a durable public
feature, not a transitional compatibility path. Remaining work includes
provider-qualified inspection/execution, exact collision/help-order
contracts, stale-cache runtime cases, and documentation of the refresh and
execution trust boundaries.

## Step 17 — Make catalog collisions inspectable and executable

I audited the automatic command-injection path as a retained public feature.
The first material gap was in conflict handling. Catalog generation detected
ambiguous names, but refresh refused to persist the catalog. That made the
documented provider-qualified escape hatch impossible for commands discovered
from handshakes: neither inspection nor explicit execution could recover the
provider metadata after the conflict.

Refresh now persists a structurally valid conflict catalog atomically and
returns `ErrCatalogConflict` after the write. Normal top-level injection still
refuses the ambiguous name. The catalog remains available to:

- `devctl plugins commands`, which displays every contender;
- `devctl plugins inspect PLUGIN`, which emits Glazed rows without starting a
  provider; and
- `devctl plugins run PLUGIN COMMAND -- ARGS...`, which selects the requested
  provider explicitly and forwards the remaining argument bytes unchanged.

I extended the local Glazed adapter with two small optional interfaces for
commands that need Cobra positional validation. `plugins inspect` therefore
keeps the exact positional syntax from the design while still using Glazed
for JSON, YAML, CSV, table, and template output.

Catalog validation now checks conflict entries instead of trusting them
merely because the conflict map is nonempty. It rejects:

- a map key that differs from the embedded command name;
- missing providers or mismatched fingerprints;
- invalid names;
- empty conflict lists; and
- duplicate or nondeterministically ordered providers.

Runtime handshake verification now includes commands held in the conflict
map. This is necessary for provider-qualified execution: a provider's complete
advertised command set must match the cached set even when the selected name
is ambiguous globally.

The real-process CLI test creates two plugins that both advertise `echo`.
Refresh starts both and reports the collision. Inspection reads the saved
catalog and starts neither. Provider-qualified execution then starts `alpha`
exactly once and leaves `beta` untouched.

Finally, dynamic command failures now return a typed
`PluginCommandExitError`. The root preserves valid plugin exit codes 1–125,
while out-of-range values collapse to the general runtime exit 1.

### Commands and results

```text
go test ./pkg/plugincatalog ./cmd/devctl/cmds
  PASS

go test ./cmd/devctl/... ./pkg/plugincatalog
  first run: compile failure because main_test.go lacked the cmds import

go test ./cmd/devctl/... ./pkg/plugincatalog
  PASS after adding the import
```

### Review focus

- Confirm that saving a validated conflict catalog while returning a conflict
  error is the desired operator contract for `plugins refresh`.
- Review `catalogEntry` and `validateRuntimeCatalog` together; the former
  selects one contender, while the latter still verifies that provider's
  complete command handshake.
- Inspect the real-binary marker assertions in `cli_contract_test.go`; these
  are the side-effect boundary for inspection and qualified execution.

## Step 18 — Enforce JSON Lines for followed logs

The `logs` command owns `--stream` as a repeatable stdout/stderr/event filter.
Glazed also normally owns that spelling as an output-processor switch. The
command-specific output section already removed Glazed's duplicate flag, but
that also meant `logs --follow --output json` retained JSON array framing. An
array cannot be consumed safely until it closes and is invalid when a
long-lived follow is interrupted.

I added an optional post-parse hook to the local Glazed adapter. `LogsCommand`
uses it to force Glazed's `output-as-objects` setting only when both conditions
are true:

```text
follow == true AND output == "json"
```

Glazed's JSON row formatter then writes one complete JSON object per added
row, followed by a newline. Each `processor.AddRow` reaches the configured
writer immediately, so there is no table-close dependency and no devctl-side
buffer to flush. Non-follow JSON queries retain their normal JSON array
contract.

Focused tests construct parsed Glazed values directly and prove both halves of
the contract: follow JSON is promoted to individual objects, while non-follow
JSON remains an array.

```text
go test ./cmd/devctl/cmds
  PASS
```

## Step 22 — Audit the remaining protocol stream renderer and root help

The direct-output search is now empty for normal finite public commands. The
only public exception is `stream start`, which transports an open-ended
protocol event channel. A mechanical Glazed conversion would be incorrect:
the default table formatter is table-oriented and cannot emit ordinary table
rows incrementally. Forcing Glazed's streaming switch makes JSON/CSV-style
row formatters viable, but makes the default human table format unsupported.
Buffering until the stream ends would violate the operator requirement.

This needs an explicit streaming presentation boundary:

```text
protocol.Event channel
  -> canonical event row
  -> streaming JSONL/CSV formatter when machine output selected
  -> incremental human event renderer for default output
```

The human renderer and machine formatter must share the canonical row and
filter semantics, but they do not need to pretend that a terminal event feed
is a finite table. I left the existing behavior intact pending this focused
implementation instead of silently changing the default format or buffering.

I also captured the authoritative root help after the command migrations:

```text
go run ./cmd/devctl --help
```

The tree contains the intended lifecycle, plan/phase, profile, plugin,
stream, and TUI groups and omits removed lifecycle spellings. The output is
rendered by the embedded Glazed help system with terminal-width wrapping, so
the golden test should normalize presentation whitespace while comparing the
exact ordered command names and descriptions.

## Step 23 — Implement canonical incremental stream rows and strict JSONL

I replaced `stream start`'s separate human and raw-JSON loops with one
`StreamCommand` that emits canonical rows. The first row identifies the
stream; every later row carries one protocol event:

```text
stream row:
  kind=stream, plugin_id, op, stream_id

event row:
  kind=event, plugin_id, op, stream_id,
  event, level, message, fields, ok
```

The command now has two presentation paths over those same rows:

- default table output selects an incremental human processor that preserves
  the concise event-feed format; and
- machine output uses the ordinary Glazed processor and its row
  transformations.

The adapter gained an optional processor-builder hook so genuinely unbounded
commands can select an incremental renderer without weakening finite command
behavior. Context cancellation is checked while waiting for every event, and
all rejected/non-selected clients use bounded shutdown contexts.

During verification I found that Glazed's built-in individual-row JSON
formatter pretty-prints objects over multiple lines. My earlier Step 18 claim
that setting `output-as-objects` alone produced strict JSON Lines was
incorrect. I introduced a compact `jsonLinesFormatter` instead. It is attached
as a Glazed row middleware after all standard select/rename/filter
middlewares, so each processed row is encoded by exactly one `Encoder.Encode`
call. Both followed logs and protocol streams use it.

The first end-to-end human test appeared to emit JSON because the phase-test
helper had been changed to append `--output json` to every invocation and was
reused by the stream test. Per the debugging limit I stopped that pass. On the
fresh pass I separated `runPhaseCommand` from the general stream runner.

Tests now prove:

- two formatter rows produce exactly two independently parseable physical
  lines;
- a real stream plugin produces a four-line JSONL feed (header, two logs,
  end); and
- the same plugin with default output produces the expected incremental human
  feed.

```text
go test ./cmd/devctl/cmds -run \
  'TestStreamCommand|TestJSONLines|TestLogs|TestPhaseCommands'
  PASS
```

## Step 24 — Add exact help-tree and runtime-drift proof

I added the two remaining focused Phase 5 proof gates.

The normalized public help golden recursively records every non-hidden command
name and short description, including plugin diagnostics, profile commands,
the stream command, and Cobra's completion shells. The first run showed that
the expected tree omitted those generated completion children; I added them
because they are genuinely part of the public help surface.

The runtime catalog test supplies a client whose provider identity and
`command.run` capability still match but whose command metadata changed. The
validator returns `PLUGIN_CATALOG_STALE`, proving that a superficially valid
provider cannot change help/schema metadata between refresh and execution.

```text
go test ./cmd/devctl/cmds -run \
  'TestPublicHelp|TestValidateRuntimeCatalog'
  first run: golden omitted completion shell children
  second run: PASS
```

## Step 25 — Replace the legacy TUI with typed Overview, Logs, and Runs

I deleted the legacy TUI implementation rather than adapting it. The removed
tree included the Watermill bus, JSON envelopes and transformer, action and
stream runners, polling watcher, six primary/detail models, plugin
introspection, direct process kill action, model-local log tailers, and the
old widget/style packages.

The replacement is 989 lines including tests and has six source files:

```text
messages.go   typed Bubble Tea messages
model.go      root navigation, controller commands, snapshot/log scheduling
overview.go   durable service snapshot selection and rendering
logs.go       bounded structured-record buffer and filter state
runs.go       typed lifecycle operations, events, and service outcomes
model_test.go pure model, controller-boundary, cancellation, and size tests
```

The new command constructs one `operator.Controller` and injects it into the
model. TUI code does not start plugins, plan services, signal processes, read
state files, or open logs. Up/down/restart commands call only the controller.
Controller events cross an in-process typed channel as `EventMsg`; completion
arrives as `OperationDoneMsg`. No prose is parsed into state.

Snapshot polling uses `Controller.Snapshot`. Durable revision changes update
the Overview, while same-revision snapshots still update sampled health
fields without creating Runs history. Log consumption uses
`Controller.Logs().Follow` with `(run_id, sequence)` cursors. A single-flight
guard prevents a snapshot tick or repeated navigation from starting duplicate
followers. Every follower and operation channel selects on the command
context, so quitting cancels blocked work.

The three models implement the MVP interactions:

- `1`, `2`, `3`, and `Tab` navigate exactly three primary views;
- Overview selects services and confirms up/down/restart with exact names;
- Logs retains service, stream, search, follow, pause, and wrap state across
  view switches;
- the log buffer is bounded by both records and bytes and reports display
  drops while paused;
- Runs renders typed operation statuses, phases/events, service outcomes, and
  stable error codes;
- confirmation, palette, search, and help modals take precedence over quit or
  global navigation.

The first size test expected the literal inactive label while the active label
was uppercased. I changed the active marker to `> [1] Overview <`, preserving
the exact label and accessibility of the navigation text.

Verification:

```text
go test ./pkg/tui/... ./cmd/devctl/cmds
  PASS

go test -race ./pkg/tui/...
  PASS

rg 'encoding/json|json\.|syscall|os\.Open|state\.Save|watermill' pkg/tui
  no matches
```

### Manual tmux smoke

I launched the no-state TUI at 100x28:

```text
go run ./cmd/devctl tui \
  --repo-root /tmp/devctl-tui-empty \
  --alt-screen=false \
  --refresh 200ms
```

The captured pane showed the three-view navigation, revision zero, stopped
environment, and no-state start guidance without logging corruption. A
batched key sequence did not quit because the final key was not delivered
reliably by tmux; an explicit `send-keys q` closed the session, and
`tmux has-session` confirmed it no longer existed.

### Review focus

- `model.go` intentionally uses one-record `runlog.Follow` commands so Bubble
  Tea remains the sole owner of model mutation.
- The current palette provides operator guidance for doctor/plugin workflows;
  richer selection can be added without restoring permanent plugin/stream
  views.
- Human rendering is deliberately plain and responsive; styling can be
  layered onto these pure models after behavior is stable.

## Step 20 — Migrate profile inspection to structured rows

I replaced the `profiles list` tabwriter and the special `profiles active`
strings with one Glazed command implementation. Both subcommands now share
the repository settings section and all output processor options.

The row contracts are:

```text
profiles list:
  profile, active, display_name, description, plugins

profiles active:
  profile, configured
```

The first focused test run failed because the old tests explicitly required
`(none)` and an asterisk embedded in human table text. Those are presentation
details from the renderer being removed. I updated the tests to request JSON
and assert typed fields, ordering, override merging, and active resolution.

```text
go test ./cmd/devctl/cmds -run 'TestProfiles|TestBuiltCLIContracts'
  first run: old presentation assertions failed
  second run: PASS with structured contract assertions
```

## Step 21 — Migrate plan and phase outcomes to Glazed

I removed the remaining direct JSON marshaling from `plan`, `build`,
`prepare`, and `validate`. Build, prepare, and validate now share one
`PhaseCommand`, one repository settings decoder, and one bounded plugin
pipeline setup. Their rows retain the existing nested payloads:

```text
build:    config, build
prepare:  config, prepare
validate: config, validate
plan:     config, plan, plugin_count, service_count
```

Validation still emits its result before returning an error when
`valid=false`. This relies on the local Glazed adapter closing the processor
even when command execution fails, preserving useful stdout and a single
stderr diagnostic.

The first focused phase test run received human tables because the old tests
implicitly depended on each command always emitting JSON. I updated the test
harness to request `--output json` and assert the single structured row. This
is the intended public contract: the default is human-readable, while
automation selects an explicit formatter.

The plan no-plugin case previously printed the exceptional literal `{}`. It
now emits the same stable row schema with an empty service list and zero
counts.

```text
go test ./cmd/devctl/cmds -run TestPhaseCommands
  first run: old implicit-JSON assertions failed
  second run: PASS

go test ./cmd/devctl/cmds
  PASS
```

## Step 19 — Extend process-level CLI boundary coverage

I extended the real executable contract rather than relying only on command
unit tests. It now proves:

- `logs --follow --until 1m` exits 2, carries `E_USAGE`, and prints the
  conflict exactly once; and
- shell completion reads no dynamic catalog and starts no configured plugin,
  matching the existing root-help and typo side-effect assertions.

```text
go test ./cmd/devctl/cmds
  PASS
```

## Step 26 — Remove dependencies owned by the retired TUI

After replacing the legacy TUI, I ran `go mod tidy` and reviewed the complete
module diff. Watermill and Goja were direct dependencies of the deleted UI
implementation and are no longer in either `go.mod` or `go.sum`. Bubble
components and Lip Gloss remain as transitive dependencies because the new
Bubble Tea UI and Glazed still require them. `google/uuid` and `x/sys` moved
from indirect to direct dependencies because production packages import them
directly.

The first test command could not read the host Go build cache under the
workspace sandbox:

```text
open /home/manuel/.cache/go-build/...: read-only file system
```

I reran the same test suite with permission to use the existing host cache;
this changed no repository files and the suite passed:

```text
go test ./...
  PASS
```

This cleanup does not remove or weaken automatic top-level dynamic command
injection. That feature is now owned by the typed plugin catalog and command
builder rather than by any of the retired TUI dependencies.
