---
Title: devctl Heavy-User Operator Experience Analysis, Redesign, and Implementation Guide
Ticket: DEVCTL-OPERATOR-UX-001
Status: active
Topics:
    - devctl
    - tui
    - architecture
    - supervisor
    - cli
    - refactor
DocType: design-doc
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: "Intern-ready analysis and redesign of devctl's process supervision, logging, CLI, and TUI operator architecture."
LastUpdated: 2026-07-24T13:22:12.13927521-04:00
WhatFor: "Provide the technical basis and phased implementation contract for making devctl reliable, observable, ergonomic, and maintainable."
WhenToUse: "Read before implementing changes to devctl service lifecycle, state, logs, commands, or terminal UI."
---

# devctl Heavy-User Operator Experience Analysis, Redesign, and Implementation Guide

## Executive Summary

This document is under active evidence gathering. Its final form will teach a
new engineer how devctl works today, identify concrete operator and maintenance
failures, compare alternative designs, and specify a phased redesign without
requiring the implementer to make unresolved architectural decisions.

## Problem Statement

Devctl has grown from a process launcher into a plugin pipeline, supervisor,
state store, log viewer, command framework, and multi-view terminal
application. The central research question is whether those parts form a
coherent daily operator tool and, where they do not, which parts should be
repaired, consolidated, or removed.

## Research Status

- Repository and ticket baseline: complete.
- Supervision and state architecture: complete.
- Logging, CLI, and TUI architecture: in progress.
- Comparative operator research: pending.
- Design decisions and implementation roadmap: pending.

## Audience and reading contract

This guide is written for an intern who knows Go but has not worked on devctl,
process supervision, terminal applications, or the NDJSON plugin protocol.
Statements use four evidence labels:

- **Observed:** directly established by current source or a stored probe.
- **Measured:** counted or timed by a reproducible ticket-local script.
- **Inferred:** a consequence of observed behavior that has not yet been
  reproduced as a failure.
- **Proposed:** a future contract; it does not describe current behavior.

The distinction matters because this ticket is a redesign specification, not a
claim that proposed behavior already exists.

## System boundary

Devctl is a repository-local development-environment orchestrator. Its job is
larger than launching a command. A normal `up` invocation:

1. Loads `.devctl.yaml` and resolves a profile.
2. starts configured protocol plugins;
3. asks those plugins to mutate configuration;
4. optionally runs build, prepare, and validation phases;
5. asks plugins for a merged launch plan;
6. creates a wrapper and service process for every planned service;
7. waits for configured health checks;
8. persists repository-local runtime state.

The code is divided into these principal areas:

| Area | Responsibility | Primary files |
|---|---|---|
| CLI composition | Cobra/Glazed commands, flags, output | `cmd/devctl/cmds/*.go` |
| Repository and profiles | Config loading and plugin selection | `pkg/repository/*`, `pkg/config/*` |
| Plugin runtime | NDJSON process startup and request transport | `pkg/runtime/*` |
| Pipeline | Phase ordering and merged results | `pkg/engine/*` |
| Supervision | Wrapper startup, health waits, termination | `pkg/supervise/supervisor.go` |
| Runtime records | State, log paths, exit metadata | `pkg/state/*` |
| Terminal UI | Views, polling, actions, event transforms | `pkg/tui/*` |
| Log parsing | JavaScript parser host and `log-parse` binary | `pkg/logjs/*`, `cmd/log-parse/*` |

The pipeline and supervisor are deliberately separate. Plugins describe what
should run; the supervisor owns the resulting local processes. This separation
is useful and should remain, but the handoff must become a durable run
transaction rather than a returned in-memory value.

## Current supervision and state architecture

### Startup sequence

**Observed.** `cmd/devctl/cmds/up.go:80-181` loads the repository, starts plugin
clients, runs configuration mutation, build, prepare, validation, and launch
planning. In non-dry-run mode, `up.go:183-197` constructs the supervisor,
starts the launch plan, assigns the profile, and then saves state.

`pkg/supervise/supervisor.go:41-77` performs startup in two sequential loops.
The first loop starts every service. The second waits for health checks.
Consequently all planned processes can exist before the first health wait
finishes.

```text
                         plugin subprocesses
                                │
config → build → prepare → validate → launch plan
                                      │
                                      ▼
                         start wrapper for service 1
                         start wrapper for service 2
                         start wrapper for service n
                                      │
                                      ▼
                         health check service 1..n
                                      │
                                      ▼
                         return in-memory State
                                      │
                                      ▼
                           write .devctl/state.json
```

**Inferred reliability defect.** The durable state write occurs after wrapper
startup and every health check. If the devctl process is killed in that
interval, a later invocation has no state record from which to discover or
stop the wrappers. Cleanup inside `Supervisor.Start` covers returned startup
and readiness errors, but it cannot cover abrupt loss of the devctl process.

### Wrapper and process-group model

Production startup passes the current devctl executable as `WrapperExe`
(`up.go:183-188`). `supervisor.go:264-301` starts the hidden
`__wrap-service` command as a new process-group leader and waits up to two
seconds for a ready file. The state record stores the wrapper PID.

The wrapper opens separate stdout and stderr files, starts the service child,
and records its termination in an exit-info JSON file
(`cmd/devctl/cmds/wrap_service.go:54-166`). Commit `39ba416` corrected the
critical process-group topology:

```text
wrapper PID = wrapper PGID        state.json owns this PID
    │
    ├── receives SIGTERM/SIGINT/SIGHUP
    │
    └── child PID = child PGID    ready file and exit.json name this PID
            │
            └── descendants normally inherit child PGID
```

The wrapper forwards a received signal to `-childPGID`
(`wrap_service.go:107-112`). Previously, wrapper and child shared a group, so
forwarding to the group signaled the wrapper again and could recursively
amplify signals. `cmd/devctl/cmds/wrap_service_test.go` is the regression
evidence for the new isolation.

The wrapper is useful because it survives the initiating CLI, waits for the
child, and can persist exit metadata. It should remain an explicit process
supervision component. Its API, however, should be internal Go data rather
than an unvalidated collection of loosely related file paths and flags.

### Artifact identity and readiness

**Observed.** `supervisor.go:209-213` creates stdout, stderr, exit, and ready
paths from the service name and a wall-clock timestamp with one-second
resolution. The wrapper writes the child PID into the ready file but ignores
directory and file-write errors (`wrap_service.go:114-117`). The supervisor
only tests whether the path exists; it does not read the PID or verify that
the marker belongs to the wrapper it just started (`supervisor.go:292-301`).

This creates three related hazards:

- two starts in one second can append to the same logs and reuse the same exit
  and ready paths;
- a stale ready path can satisfy a later startup;
- a ready-file write failure appears to the user as a generic two-second
  timeout rather than the underlying filesystem error.

The future model needs a unique `run_id`, created before process startup. A
suitable value is a UUIDv7 or ULID because it is collision resistant and
sortable without relying on filename timestamps.

### State representation

`pkg/state/state.go:22-49` stores one repository state containing a profile and
a slice of service records. Each service record includes:

- service name and wrapper PID;
- command and working directory;
- sanitized environment values;
- stdout, stderr, and exit-info paths;
- start time and health-check fields;
- a non-secret copy of the planned service specification.

Raw plugin-computed environment variables are intentionally not persisted.
`pkg/servicecontrol/resolve.go:24-99` recomputes configuration mutation and the
launch plan for a single-service start or restart. This secret-minimization
property is correct and must be preserved.

**Observed.** `state.Save` marshals JSON and writes the final path directly
(`state.go:88-103`). `WriteExitInfo` does the same
(`pkg/state/exit_info.go:26-40`). Neither uses a same-directory temporary file,
`fsync`, rename, or a schema version.

**Observed.** Liveness is a zombie test followed by `kill(pid, 0)`
(`state.go:117-151`). The state has no process-start identity. If the wrapper
exits and the kernel reuses its PID, devctl may classify and signal an
unrelated process.

### Health semantics

`engine.HealthCheck` exposes `Type`, `Address`, `URL`, and `TimeoutMs`
(`pkg/engine/types.go:13-18`). The persisted record retains that timeout.
Startup does not use it: both bulk and single-service readiness wrap
`waitReady` in the global supervisor timeout (`supervisor.go:64-75` and
`155-163`).

TCP readiness attempts a 200 ms connection every 200 ms. HTTP readiness uses
a 500 ms client timeout and succeeds for every response from 200 through 499
(`supervisor.go:361-401`). The latter can declare `/wrong-path` or an
unauthorized endpoint ready. No health result, attempt count, last error, or
transition time is persisted for `status` or the TUI.

### Stop, start, and restart semantics

Bulk `Stop` iterates services in state order and returns only the last error
(`supervisor.go:80-94`). `down` ignores that result and removes `state.json`
(`cmd/devctl/cmds/down.go:21-37`). Therefore a process that resisted
termination can remain alive after devctl discards its ownership record.

Single-service stop terminates the wrapper group, sets its PID to zero, clears
the exit-info path, and saves state (`supervisor.go:96-123`). Clearing the
path removes the previous run's diagnostic association. A failed
single-service health check terminates the new wrapper but does not add the
failed attempt to state (`supervisor.go:150-178`).

Restart resolves a fresh service specification before stopping the current
run (`cmd/devctl/cmds/restart.go:50-75`). It executes configuration mutation
and launch planning but intentionally omits build, prepare, and validation.
That policy is documented in the command, but it means `up` and `restart` do
not prove the same prerequisites.

## Supervision findings and severity

| ID | Finding | Evidence | Operator consequence | Severity |
|---|---|---|---|---|
| SUP-01 | State is written after all starts and health waits | `up.go:183-197`, `supervisor.go:41-77` | Abrupt interruption can orphan an environment | Critical |
| SUP-02 | State PID has no process identity token | `state.go:29-49,117-151` | PID reuse can target an unrelated process | Critical |
| SUP-03 | `down` removes state after ignoring stop errors | `down.go:21-37` | Unstopped processes become unmanaged | Critical |
| SUP-04 | Run artifact names collide within one second | `supervisor.go:209-213` | Logs and readiness from separate runs can mix | High |
| SUP-05 | Ready marker errors are ignored and content is unvalidated | `wrap_service.go:114-117`, `supervisor.go:292-301` | False readiness or opaque timeout | High |
| SUP-06 | State and exit JSON writes are not atomic | `state.go:88-103`, `exit_info.go:26-40` | Concurrent readers can observe corrupt JSON | High |
| SUP-07 | Per-service health timeout is ignored | `types.go:13-18`, `supervisor.go:64-75` | Configuration contract is misleading | Medium |
| SUP-08 | Any HTTP response below 500 is ready | `supervisor.go:381-401` | Wrong or unauthorized endpoints pass | Medium |
| SUP-09 | Stop reports only the last failure | `supervisor.go:80-94` | Multi-service cleanup is not diagnosable | Medium |
| SUP-10 | Stopped/failed runs lose diagnostic linkage | `supervisor.go:120-122,150-178` | Status cannot explain the previous attempt | Medium |

Severity expresses potential impact, not implementation order. The future
roadmap must address ownership durability before adding richer presentation.

## Current CLI contract probe

`scripts/02-cli-contract-probe.sh` builds the current binary with isolated
temporary build and repository directories. On 2026-07-24 it established:

| Invocation | Exit | Current output contract |
|---|---:|---|
| `devctl status` without state | 0 | JSON object with `exists:false` |
| `devctl logs` without `--service` | 1 | Error printed twice around rendered help |
| `devctl down` without state | 1 | Missing-file error printed twice around rendered help |
| `devctl plan` without config | 0 | Warning on stderr and `{}` on stdout |
| `devctl profiles list` without config | 0 | Empty human table |

These are observations. The CLI analysis later in this document will define a
consistent rule for absence, invalid configuration, human output, structured
output, and error rendering.

## The five current log and event planes

Devctl currently has five related but semantically different streams. A new
engineer must not combine them until each record has explicit provenance.

### Service stdout and stderr

The wrapper writes the child process's byte streams to two append-only files
whose paths are stored in the service record. These bytes are unstructured:
they may contain text, ANSI control sequences, JSON, carriage-return progress
updates, or binary fragments. Separate files destroy the original temporal
ordering between stdout and stderr.

`devctl logs` selects one current service record and one stream
(`cmd/devctl/cmds/logs.go:16-80`). Its capabilities are:

- exactly one service selected by the required `--service` flag;
- stdout by default or stderr with `--stderr`;
- last 50 lines by default, all bytes with `--tail 0`;
- optional polling follow mode.

It has no all-service mode, combined stream, run selection, source prefixes,
timestamps, `--since`, structured output, ANSI policy, rotation handling, or
backpressure contract.

### Devctl and plugin diagnostic logging

Devctl uses generated Logcopter areas with zerolog. Root flags select level,
format, file, stdout mirroring, caller, and area overrides. These diagnostics
describe devctl itself and should normally remain on stderr so stdout is safe
for data.

Plugin stdout is reserved for NDJSON protocol frames. Any non-JSON stdout line
fails all outstanding calls as protocol contamination
(`pkg/runtime/client.go:216-263`). Plugin stderr is read line by line and
re-emitted as an info-level devctl log with a `plugin` field
(`client.go:266-284`). The original plugin severity and timestamp are not
preserved unless encoded inside the text.

This separation is correct:

```text
plugin stdout  -> machine protocol; contamination is fatal
plugin stderr  -> operator diagnostic; never parsed as protocol
service output -> application-owned bytes persisted per run
```

### Protocol stream events

The v2 protocol exposes `Event{StreamID, Event, Level, Message, Fields, Ok}` and
capabilities for operations, streams, and commands
(`pkg/protocol/types.go:19-77`). `devctl stream start` selects a provider,
starts a stream, and prints either human lines or raw event JSON
(`cmd/devctl/cmds/stream.go:28-123`).

Protocol streams are not service file logs. They can represent telemetry,
progress, sensor data, or a plugin-defined feed. They need bounded buffering
and explicit start/end ownership. The current TUI correctly avoids echoing
every stream event into its global event log
(`pkg/tui/transform.go:161-167`), but it gives streams an entire primary view.

### Ephemeral TUI events

The TUI creates its own in-memory events for snapshot polling, action text,
pipeline phases, service-exit observations, and stream lifecycle. They are not
persisted and disappear when the TUI exits. A state snapshot is transformed
into `state: loaded`, `state: missing`, or `state: error` on every refresh
(`pkg/tui/transform.go:43-65`), even if nothing changed.

`scripts/04-tui-state-event-probe.sh` observed five identical
`state: missing` events per second with a 200 ms refresh. The committed
dashboard screenshot shows the same issue at a one-second refresh. This is
polling noise presented as event history.

### JavaScript parsed-log events

`pkg/logjs` embeds goja and supports module registration, parsing, filtering,
transformation, timeouts, normalization, errors, multiline state, and
multi-module fan-out. `cmd/log-parse` is a standalone stream processor. The
subsystem contains 1,668 Go lines plus examples and a long help page.

Repository search found no integration from `devctl logs`, the supervisor, or
the TUI. Historical ticket `MO-007-LOG-PARSER` explicitly called devctl
integration future work; `MO-016-LOGPARSER-DEVCTL-INTEGRATION` still has nine
integration tasks open.

**Design disposition:** the operator redesign must not integrate this
subsystem. Before implementation, search known downstream repositories for
imports of `github.com/go-go-golems/devctl/pkg/logjs`. If there are none,
remove `cmd/log-parse`, `pkg/logjs`, its examples, and its devctl help entry in
one explicit change. Git history is the archive. Do not add a compatibility
command or adapter. If external consumers exist, stop that removal and open a
separate extraction ticket; do not let it block the core logging work.

## Log-reader implementation audit

There are three tail implementations:

| Reader | Algorithm | Limit | Follow behavior |
|---|---|---:|---|
| CLI `readTailLines` | scans entire file with 1 MiB token cap | line count | separate `followFile` |
| `state.TailLines` | reads at most final byte window | caller supplied, usually 2 MiB | none |
| TUI `readTailLines` | reads final byte window and returns offset | 2 MiB default | polls both files |

The CLI follow loop opens the file once, seeks to the end, and retries EOF
every 200 ms (`cmd/devctl/cmds/logs.go:82-108`). It never compares file
identity or detects a shorter size. `scripts/03-log-follow-lifecycle-probe.sh`
established:

```text
expected_appended_line=append-visible
expected_after_truncate=after-truncate
expected_after_rotation=after-rotate
captured_stdout_begin
append-visible
captured_stdout_end
```

The baseline append proves the follower was active. Missing later lines prove
that current behavior cannot follow common copy-truncate or rename/recreate
lifecycles.

The TUI implementation is more robust to truncation because it resets an
offset when size decreases (`pkg/tui/models/service_model.go:778-840`), but it
reopens and reads each stream every 250 ms and still has no inode identity. It
duplicates the bounded-tail algorithm at `service_model.go:872-922`.

**Required consolidation:** implement one package, tentatively
`pkg/runlog`, with snapshot and follow APIs. CLI, status diagnostics, and TUI
must consume it. No frontend may open service log files directly.

## CLI architecture and ergonomic audit

### Registration and framework mixture

`cmd/devctl/cmds/root.go:8-27` registers 17 top-level commands, including the
hidden wrapper and hidden developer group. Most are hand-written Cobra
commands. Only status and plugin list declare Glazed `WriterCommand`
interfaces, and both manually marshal composite JSON instead of emitting
Glazed rows. Consequently the code has Glazed schema parsing without the
consistent table/JSON/YAML/CSV output contract that motivates Glazed.

The repository section is a useful shared unit. It defines repo root, config,
profile, strictness, dry-run, and timeout in one schema
(`cmd/devctl/cmds/common.go:18-197`). That section should remain and be used by
every public command. The current `--timeout` help calls it a plugin-operation
timeout even though lifecycle commands also use it for readiness and shutdown.

### Syntax inconsistencies

| Intent | Current syntax | Problem |
|---|---|---|
| Start one service | `devctl start SERVICE` | clear positional form |
| Restart one service | `devctl restart SERVICE` | clear positional form |
| Stop one service | `devctl stop-service SERVICE` | noun is embedded in verb |
| Read one service | `devctl logs --service SERVICE` | same noun becomes required flag |
| Stop all | `devctl down` | separate environment-level vocabulary |
| Observe plugin stream | `devctl stream start --op OP` | start is a subgroup verb |

The redesign uses:

```text
devctl up [SERVICE...]
devctl down [SERVICE...]
devctl restart [SERVICE...]
devctl logs [SERVICE...]
```

With no service arguments, `up` and `down` operate on the environment and
`logs` selects all current services. With arguments, they operate on the named
set. Remove public `start`, `stop-service`, and their old spellings rather than
keeping aliases. This is an intentional breaking cleanup and requires the
release note to say so.

### Output inconsistencies

The isolated CLI probe recorded:

- missing state is successful JSON for `status` but an error for `down`;
- missing config is a warning plus successful `{}` for `plan`;
- an empty profile list is a successful table;
- errors are printed twice when Cobra receives returned errors and renders the
  rich help wrapper;
- lifecycle success is the word `ok`, while list/snapshot commands emit JSON
  or tables.

The future contract is:

| Condition | Human mode | Machine mode | Exit |
|---|---|---|---:|
| No state for `status` | one “stopped” row | one environment row | 0 |
| No state for `down` | “already stopped” outcome | one no-op outcome row | 0 |
| No configuration for planning/up | concise error with config path | error on stderr, no data rows | 2 |
| Empty list | headings/no rows | empty stream/array per Glazed formatter | 0 |
| Usage error | one error plus focused usage | same stderr | 2 |
| Runtime failure | one contextual error | same stderr | 1 |
| Partial multi-service failure | result row per service plus summary | result row per service | 1 |

Never print a warning and an empty object to claim successful planning. Never
print errors twice. Stdout is data; diagnostics stay on stderr.

### Glazed command contract

Snapshot, list, and lifecycle-result commands must implement
`RunIntoGlazeProcessor` and emit one row per environment, service, plugin,
phase, or outcome. Their command descriptions include:

- `settings.NewGlazedSchema()` for output selection;
- `cli.NewCommandSettingsSection()` for schema/debug inspection;
- the shared repository section;
- `cmds.WithArguments` for variadic `SERVICE...` using
  `fields.TypeStringList` and `fields.WithIsArgument(true)`.

Streaming logs use the same command-description and settings machinery, but
emit a stable record shape:

```go
type LogRecord struct {
    Sequence uint64
    Time     time.Time
    RunID    string
    Service  string
    Stream   string // stdout | stderr | system
    Text     string
    Partial  bool
}
```

Human default renders compact prefixed text. `--output json` emits one JSON
object per record and flushes after each row. A streaming command must honor
context cancellation promptly. Environment-variable flag sources must not be
introduced silently; the repository already uses explicit flags/config and
the project instructions require notification before environment reads.

### Dynamic command discovery

`AddDynamicPluginCommands` starts configured plugin processes during root
construction for unknown top-level verbs (`cmd/devctl/cmds/dynamic_commands.go:
22-139`). This makes command discovery depend on executing repository code and
can add startup latency or side effects before normal argument validation.

**Design disposition:** remove automatic top-level injection. Expose plugin
commands as:

```text
devctl plugin command list
devctl plugin command run PLUGIN COMMAND -- ARGS...
```

Discovery then occurs only under an explicit plugin namespace. Do not retain
top-level compatibility aliases.

## TUI architecture audit

### Measured size and views

The current `pkg/tui` contains 7,757 Go lines, seven model files, and no
`*_test.go` files. The root exposes six views:

```text
Dashboard -> Events -> Pipeline -> Plugins -> Streams -> Dashboard
                    \
                     Service detail (entered from Dashboard)
```

The largest files are:

| File | Lines | Responsibility |
|---|---:|---|
| `models/service_model.go` | 923 | service detail, health/env/exit, dual log tailer |
| `models/dashboard_model.go` | 821 | services, events, plugins, streams, actions |
| `action_runner.go` | 749 | duplicate lifecycle orchestration and phase events |
| `models/pipeline_model.go` | 738 | phase/step/config/live-output presentation |
| `models/root_model.go` | 600 | routing, global state, view navigation |
| `models/eventlog_model.go` | 599 | ephemeral filters, queue, stats, viewport |
| `models/streams_model.go` | 463 | protocol-stream creation and inspection |
| `state_watcher.go` | 450 | state, liveness, stats, health, plugin introspection |
| `stream_runner.go` | 388 | plugin process and stream lifecycle |
| `models/plugin_model.go` | 354 | capability inspection |

Large files are not defects by themselves. Here they correspond to duplicated
I/O and orchestration responsibilities inside presentation models.

### Event transport

The TUI uses an in-memory Watermill broker, JSON envelopes, a domain-to-UI
transformer, another JSON envelope, a UI topic, and a forwarder before calling
`tea.Program.Send`:

```text
StateWatcher / ActionRunner / StreamRunner
          │ typed domain value
          ▼
     JSON Envelope
          ▼
 Watermill: devctl.events
          ▼
 unmarshal + transform
          ▼
     JSON UI Envelope
          ▼
 Watermill: devctl.ui.msgs
          ▼
 unmarshal + switch
          ▼
 typed tea.Msg -> RootModel
```

This process is entirely in-memory. It provides no persistence, replay,
cross-process boundary, or independent scaling. It adds two serialization
failure points and several type switches before reaching Bubble Tea, whose
native API already accepts typed messages.

**Design disposition:** remove Watermill and JSON envelopes from the TUI.
Application services send typed immutable events to a small sink interface:

```go
type EventSink interface {
    Send(context.Context, OperatorEvent) error
}

type TeaSink struct {
    Program interface{ Send(tea.Msg) }
}

func (s TeaSink) Send(ctx context.Context, event OperatorEvent) error {
    select {
    case <-ctx.Done():
        return ctx.Err()
    default:
        s.Program.Send(EventMsg{Event: event})
        return nil
    }
}
```

Persistence, if needed, belongs in the run journal, not an in-memory message
broker.

### Polling and health divergence

`StateWatcher` publishes a complete snapshot on every interval and performs
process stats and health checks (`pkg/tui/state_watcher.go:168-245`). Its HTTP
health rule treats status 400 and above as unhealthy
(`state_watcher.go:409-423`), while startup accepts every status below 500.
Thus the same endpoint can be “ready” during `up` and “unhealthy” in the TUI.

Plugin introspection starts and stops each configured plugin in a background
loop (`state_watcher.go:106-159`). A display refresh can therefore execute
plugin code. Capability inspection should be a cached result of explicit
planning/doctor operations, not a recurring TUI side effect.

The future watcher compares a revision/hash and emits:

- a snapshot message when a model first attaches;
- a state-changed message only when durable state changes;
- health-transition messages only when status changes;
- resource samples on a separately throttled interval without adding event
  history.

### Duplicate control plane

`pkg/tui/action_runner.go` reimplements up, down, stop, and restart instead of
calling the CLI's underlying application operations. Concrete divergence
already exists:

- CLI `down` errors when state is absent; TUI `runDown` succeeds.
- CLI restart resolves the new plan before stopping; TUI restart stops first,
  then resolves. A resolution failure leaves the service stopped.
- Both down paths ignore supervisor stop errors and remove state.
- Phase definitions and actual phase behavior are manually synchronized.

The dashboard also has a separate `x` action that calls
`syscall.Kill(pid, SIGTERM)` (`models/dashboard_model.go:124-140`). It does not
target the child group, wait, escalate, persist an outcome, or update state.

**Design disposition:** introduce one `pkg/operator` application service used
by both CLI and TUI:

```go
type Controller interface {
    Up(ctx context.Context, request UpRequest, sink EventSink) (OperationResult, error)
    Down(ctx context.Context, request DownRequest, sink EventSink) (OperationResult, error)
    Restart(ctx context.Context, request RestartRequest, sink EventSink) (OperationResult, error)
    Snapshot(ctx context.Context, request SnapshotRequest) (Snapshot, error)
    FollowLogs(ctx context.Context, request FollowRequest, sink LogSink) error
}
```

The CLI adapts operation results to Glazed rows. The TUI adapts typed events to
Bubble Tea messages. Models never signal processes, start plugins, read state
files, or tail logs.

### Text-derived state

`RootModel` sets its status line by checking prefixes such as
`action failed:`, `action ok:`, and `sent SIGTERM`
(`pkg/tui/models/root_model.go:176-191`). Changing prose can silently change
behavior.

Use a typed outcome:

```go
type OperationFinished struct {
    OperationID string
    Kind        OperationKind
    Status      OperationStatus // succeeded | failed | canceled | partial
    Error       *OperatorError
    FinishedAt  time.Time
}
```

Text is rendered at the edge and is never parsed back into state.

### Reduced information architecture

The redesigned default TUI has three views:

1. **Overview** — environment state, service table, health, current operation,
   and the last important transition.
2. **Logs** — combined or selected services/streams, follow/pause/search,
   timestamps, and run selector.
3. **Runs** — current and recent lifecycle operations, phases, validation
   issues, durations, and failure details.

The current Events view is folded into Runs as typed transitions and into Logs
for system records. Pipeline becomes Runs. Plugin capability inspection and
protocol-stream experimentation remain explicit CLI developer workflows and
are removed from the primary TUI. The service-detail page becomes a side panel
or filtered Overview/Logs state rather than a fourth navigation system.

This reduction prioritizes daily questions:

- What is running?
- What failed?
- What is it printing?
- What operation is devctl performing?
- What do I need to do next?

It does not remove plugin or stream protocol capability; it removes their
permanent position in the operator's main navigation.

## Historical ticket reconciliation

Prior tickets are design evidence, not authoritative completion records.
Several index headers say `complete` while their body says `active`; task lists
also disagree with both.

| Ticket | Recorded state | Current interpretation |
|---|---|---|
| `MO-006-DEVCTL-TUI` | header complete; body active; 27 open task markers | foundational intent implemented, completion metadata unreliable |
| `MO-007-LOG-PARSER` | header complete; one future integration task open | standalone parser complete; devctl integration absent by design |
| `MO-008-IMPROVE-TUI-LOOKS` | header complete; placeholder task open | visual work exists; ticket cannot prove acceptance |
| `MO-009-TUI-COMPLETE-FEATURES` | active; 58 checked, 31 open | feature expansion explains current breadth; not a stable target |
| `MO-010-DEVCTL-CLEANUP-PASS` | header complete; manual checks recorded | useful historical cleanup, not a current architecture audit |
| `MO-011-IMPLEMENT-STREAMS` | header complete; optional stop semantics open | CLI/TUI streams exist; lifecycle remains one-client-per-stream |
| `MO-012-PORT-CMDS-TO-GLAZED` | active; most public verbs still open | migration is partial; current hybrid contract confirms it |
| `MO-014-IMPROVE-PIPELINE-TUI` | active; placeholder only | no actionable source of truth |
| `MO-016-LOGPARSER-DEVCTL-INTEGRATION` | active; nine implementation tasks open | integration was designed but never implemented |
| `MO-017-TUI-CONTEXT-LIFETIME-SCOPING` | header complete; shutdown validation open | important context fixes landed; verification incomplete |
| `MO-018-PIPELINE-VIEW-STUCK-STATE` | active; fix tasks open | evidence against relying on current pipeline view state |
| `MO-018-STATE-EVENT-TRACE` | active; repeated state-event fix open | directly reproduced by this ticket |
| `STREAMS-TUI` | active; most feature tasks checked | current view exists; does not establish default-TUI value |
| `DCTL-SERVICES` | active; implementation checked; upload open | individual lifecycle exists; current safety gaps supersede design |

After approval, update those tickets with a short “superseded in whole or part
by DEVCTL-OPERATOR-UX-001” note. Do not rewrite their historical task lists.
Create implementation tasks only in the new execution ticket so two trackers
cannot claim ownership of the same refactor.

## Current-state gap summary

| Area | Keep | Consolidate or correct | Remove from core path |
|---|---|---|---|
| Plugin pipeline | config mutation, phases, launch plan | typed operation events | automatic top-level dynamic command injection |
| Wrapper | detached ownership and exit capture | durable run identity/state transaction | direct no-wrapper production branch if unowned |
| State | non-secret planned metadata | atomic versioned run records | destructive clearing of diagnostic links |
| Logs | persisted service output | one run-aware reader and record schema | three frontend-specific readers |
| CLI | shared repo Glazed section | nouns, exit codes, rows, error rendering | old `start`/`stop-service` aliases |
| TUI | Bubble Tea and focused operator views | shared controller and typed messages | Watermill/JSON chain, duplicate actions, kill shortcut |
| Plugin streams | explicit protocol capability | bounded CLI lifecycle | default primary TUI view |
| Log parser | independent functionality if separately owned | external consumer gate | devctl integration and repository ownership |

## Comparative research

The comparison uses official documentation captured under `sources/web/`.
Features are evidence of established contracts, not a mandate to reproduce
another product.

### Process Compose

[Process Compose's TUI](https://f1bonacc1.github.io/process-compose/tui/)
concentrates process status, logs, start/stop/restart, a dependency graph, and
a command palette in one operator surface. It defines:

- a bounded configurable log buffer and an explicit CPU-cost warning;
- follow, full-screen, and wrap toggles;
- configurable shortcuts;
- activity and silence indicators for unfocused processes;
- a searchable command palette for less frequent actions.

Its [client documentation](https://f1bonacc1.github.io/client/) exposes the
same process operations to CLI and remote TUI clients through an OpenAPI
server. Its
[lifecycle documentation](https://f1bonacc1.github.io/process-compose/launcher/)
models readiness, restart policies, dependencies, disabled processes, and
additional successful exit codes.

**Adopt:**

- one coherent status/log/action TUI;
- bounded log memory and explicit follow/wrap state;
- command palette for secondary actions;
- typed lifecycle states and exit classification;
- one control API shared by every frontend.

**Do not adopt in this project:**

- a local REST server or permanent daemon;
- configuration editing from the TUI;
- replicas, namespaces, PTY forwarding, dependency graphs, and automatic
  restart policies in the reliability MVP.

The shared `pkg/operator.Controller` supplies semantic unification without a
network server. A daemon should require separate evidence that independent
clients must attach to a surviving controller.

### Docker Compose

[Docker Compose logs](https://docs.docker.com/reference/cli/docker/compose/logs/)
uses `logs [SERVICE...]` and supports follow, instance selection, ANSI/prefix
control, since/until, tail, and timestamps.
[Docker Compose ps](https://docs.docker.com/reference/cli/docker/compose/ps/)
uses the same optional service positionals and offers running/all selection,
status filters, human tables, JSON, and non-truncated output.

**Adopt:**

- optional positional service lists;
- source prefixes by default for combined output;
- follow, tail, since, until, timestamps, and ANSI controls;
- one row model with human and machine renderings;
- explicit inclusion of stopped/historical runs.

**Do not adopt:**

- container-specific replica indices, port columns, image fields, and orphan
  container semantics.

### Tilt

[Tilt logs](https://docs.tilt.dev/cli/tilt_logs.html) accepts positional
resources and provides follow, JSON Lines, severity, time, source, and tail
filters. Its source distinction between build and runtime output is relevant
to devctl's pipeline-versus-service distinction.

**Adopt:**

- JSON Lines for long-running structured output;
- source filtering;
- a relative-duration `--since` form;
- consistent resource positionals.

Devctl's source vocabulary is `service`, `pipeline`, `plugin`, and `system`.
Protocol telemetry remains under `stream`, not the default logs feed.

### journalctl

The official
[journalctl manual](https://www.freedesktop.org/software/systemd/man/latest/journalctl.html)
defines structured field filtering, since/until windows, bounded recent
records, follow mode, multiple JSON formats, and opaque cursors. Devctl does
not use journald, but two concepts matter:

- every record needs structured provenance separate from rendered text;
- follow resumption needs an opaque stable cursor, not a byte offset exposed
  as public API.

Devctl's cursor is `(run_id, sequence)`. The initial implementation may derive
records from files, but the public API does not expose inode or byte offset.

### Comparison matrix

| Capability | Current devctl | Process Compose | Docker Compose | Tilt | Proposed devctl |
|---|---|---|---|---|---|
| Optional multi-service logs | no | TUI selection | yes | yes | yes |
| Combined source prefixes | no | yes | yes | resource aware | yes, default |
| Follow | one file | yes | yes | yes | multi-source |
| Since/until | no | not central | yes | since | both |
| Structured streaming | no | API | no Compose JSON flag documented | JSON Lines | Glazed JSON records |
| Stable resume cursor | no | server-owned | engine-owned | server-owned | run ID + sequence |
| Shared CLI/TUI control | duplicated | server/client | daemon API | server/client | in-process Controller |
| Bounded TUI logs | line cap only | explicit | n/a | server-owned | explicit records/bytes |
| Action palette | no | yes | n/a | web UI actions | yes |
| Default TUI primary views | six + detail | process/log centric | n/a | resource/log centric | three |

### Research conclusions

The redesign is conservative in architecture and conventional in interface:

1. Do not add a daemon.
2. Do add one application control service shared by CLI and TUI.
3. Treat logs as structured, run-scoped records at the API boundary.
4. Use positional services and conventional filters.
5. Keep machine output as a first-class streaming contract.
6. Reduce the default TUI to daily operator questions.
7. Defer scheduling features until durable process ownership is correct.

These conclusions validate the earlier code-based direction without requiring
devctl to become a clone of any comparison tool.

## References

- [Investigation Diary](../reference/01-investigation-diary.md)
- [Ticket Tasks](../tasks.md)
