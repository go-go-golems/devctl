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
RelatedFiles:
    - Path: repo://cmd/devctl/cmds/logs.go
      Note: Current CLI log reader and follower
    - Path: repo://cmd/devctl/cmds/status.go
      Note: Current Glazed-adjacent status output
    - Path: repo://cmd/devctl/cmds/wrap_service.go
      Note: Current wrapper, signal forwarding, and exit capture
    - Path: repo://pkg/logjs/module.go
      Note: Standalone parser evaluated for removal
    - Path: repo://pkg/state/state.go
      Note: Current persistent state and PID liveness model
    - Path: repo://pkg/supervise/supervisor.go
      Note: Current lifecycle, health, and process-group implementation
    - Path: repo://pkg/tui/action_runner.go
      Note: Duplicated TUI lifecycle control plane
    - Path: repo://pkg/tui/models/root_model.go
      Note: Current six-view routing and text-derived status
    - Path: repo://pkg/tui/models/service_model.go
      Note: Current TUI file-tail implementation
    - Path: repo://pkg/tui/state_watcher.go
      Note: Current snapshot, health, stats, and plugin polling
    - Path: repo://pkg/tui/transform.go
      Note: Current domain-to-UI transform and repeated state events
ExternalSources:
    - https://f1bonacc1.github.io/process-compose/tui/
    - https://f1bonacc1.github.io/client/
    - https://f1bonacc1.github.io/process-compose/launcher/
    - https://docs.docker.com/reference/cli/docker/compose/logs/
    - https://docs.docker.com/reference/cli/docker/compose/ps/
    - https://docs.tilt.dev/cli/tilt_logs.html
    - https://www.freedesktop.org/software/systemd/man/latest/journalctl.html
Summary: Intern-ready analysis and redesign of devctl's process supervision, logging, CLI, and TUI operator architecture.
LastUpdated: 2026-07-24T13:49:58.732286898-04:00
WhatFor: Provide the technical basis and phased implementation contract for making devctl reliable, observable, ergonomic, and maintainable.
WhenToUse: Read before implementing changes to devctl service lifecycle, state, logs, commands, or terminal UI.
---



# devctl Heavy-User Operator Experience Analysis, Redesign, and Implementation Guide

## Executive Summary

Devctl's current plugin pipeline is a sound foundation, and the focused
process-group correction at commit `39ba416` fixes a real recursive-signal
defect. The wider operator system is not yet reliable enough for heavy daily
use. Process ownership becomes durable only after every service has started
and passed readiness; PID reuse is not detected; `down` discards stop errors
and removes ownership state; log artifacts can collide within one second; and
health rules differ between startup and the TUI.

The presentation layers multiply those risks. CLI lifecycle syntax and output
contracts are inconsistent. Three independent file-tail implementations have
different behavior. The CLI follower loses output after truncation or
rotation. The 7,757-line TUI contains no TUI tests, duplicates the lifecycle
controller, differs from CLI restart ordering, turns every poll into an event,
derives status by parsing prose, and can signal a PID directly.

The proposed redesign retains the plugin pipeline, wrapper, Bubble Tea, and
Glazed. It introduces:

- a versioned run record created before process startup;
- atomic state and handshake files under a unique run directory;
- PID plus process-start identity checks;
- one controller shared by CLI and TUI;
- one sequenced run-log journal and reader;
- conventional `up/down/restart/logs [SERVICE...]` commands with Glazed rows;
- a three-view Overview, Logs, and Runs TUI using typed messages.

It removes duplicate TUI orchestration, Watermill/JSON message plumbing,
direct process signaling from models, frontend-specific log readers,
automatic top-level plugin command injection, old lifecycle spellings, and,
after a downstream-consumer gate, the unintegrated standalone JavaScript log
parser. It deliberately does not add a daemon, REST API, scheduler, restart
policy, state compatibility layer, or automatic repair.

The implementation is divided into seven dependency-ordered phases with exact
schemas, APIs, pseudocode, file changes, tests, commit boundaries, and
acceptance gates. A new intern should be able to execute the work without
choosing core architecture.

## Problem Statement

Devctl has grown from a process launcher into a plugin pipeline, supervisor,
state store, log viewer, command framework, and multi-view terminal
application. The central research question is whether those parts form a
coherent daily operator tool and, where they do not, which parts should be
repaired, consolidated, or removed.

## Research Status

- Repository and ticket baseline: complete.
- Supervision and state architecture: complete.
- Logging, CLI, and TUI architecture: complete.
- Comparative operator research: complete.
- Design decisions and implementation roadmap: complete.

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

**Required consolidation:** implement `pkg/runlog` with snapshot and follow
APIs. CLI, status diagnostics, and TUI
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

## Target architecture

The future system has one control plane, one durable run model, and two thin
frontends.

```text
                         ┌─────────────────────┐
                         │ Plugin pipeline     │
                         │ config/plan/phases  │
                         └──────────┬──────────┘
                                    │ ServiceSpec[]
                                    ▼
┌──────────────┐           ┌─────────────────────┐          ┌──────────────┐
│ Glazed CLI   │──────────▶│ pkg/operator        │◀─────────│ Bubble Tea   │
│ rows/errors  │ requests  │ Controller          │ events   │ TUI          │
└──────────────┘           └──────┬────────┬─────┘          └──────────────┘
                                  │        │
                         lifecycle│        │log query/follow
                                  ▼        ▼
                         ┌────────────┐  ┌────────────┐
                         │supervise   │  │runlog      │
                         │wrapper     │  │journal     │
                         └─────┬──────┘  └──────┬─────┘
                               │                │
                               ▼                ▼
                         ┌────────────────────────────┐
                         │ .devctl/runs/<run_id>/     │
                         │ owner/ready/log/exit JSON  │
                         └──────────────┬─────────────┘
                                        │ indexed by
                                        ▼
                              .devctl/state.json v2
```

The controller is an application service, not a network service. It owns
validation, locking, planning, state transitions, and structured outcomes.
The supervisor owns OS processes. `runlog` owns service output. The CLI and TUI
own rendering and interaction only.

### Proposed package layout

```text
pkg/
  operator/
    controller.go          public application operations
    events.go              typed operation events
    errors.go              stable error codes and context
    requests.go            validated request types
    snapshot.go            frontend-neutral read model
  runstate/
    schema.go              versioned environment/run records
    store.go               atomic reads/writes and repository lock
    identity_linux.go      Linux process identity
    identity_other.go      explicit unsupported/fallback behavior
    reconcile.go           durable-artifact reconciliation
  runlog/
    record.go              LogRecord and Cursor
    writer.go              stdout/stderr capture and sequenced journal
    reader.go              snapshot/query/follow
    follow.go              lifecycle and cancellation
  supervise/
    supervisor.go          start/stop primitives
    wrapper.go             wrapper request and durable handshake
  tui/
    app.go                 Bubble Tea wiring
    model.go               global navigation and typed updates
    overview.go
    logs.go
    runs.go
cmd/devctl/cmds/
  up.go
  down.go
  restart.go
  status.go
  logs.go
  doctor.go
  tui.go
  plugin/                  explicit inspection/command namespace
  stream/                  explicit protocol stream namespace
```

Do not create a second `go.mod`. Do not add old-command aliases, state
migrators, or adapter packages.

## Durable run model

### Directory layout

Every attempt gets a unique run directory before a process starts:

```text
.devctl/
  lock
  state.json
  runs/
    01J3...ULID/
      run.json
      owner.json
      ready.json
      logs.jsonl
      stdout.log
      stderr.log
      exit.json
```

`state.json` is the environment index. `run.json` is the durable attempt
record. The wrapper is the sole writer of `owner.json`, `ready.json`,
`logs.jsonl`, raw output, and `exit.json`. The controller is the sole writer of
`state.json` and `run.json`. This single-writer rule avoids multi-process JSON
updates.

Use ULID or UUIDv7 library support already acceptable to the repository. The
run ID is generated once by the controller. It is never inferred from a
timestamp or path.

### Environment state schema

```go
const StateSchemaVersion = 2

type EnvironmentState struct {
    Version    int                    `json:"version"`
    RepoRoot   string                 `json:"repo_root"`
    Profile    string                 `json:"profile,omitempty"`
    Revision   uint64                 `json:"revision"`
    CreatedAt  time.Time              `json:"created_at"`
    UpdatedAt  time.Time              `json:"updated_at"`
    Services   map[string]ServiceSlot `json:"services"`
}

type ServiceSlot struct {
    Name         string `json:"name"`
    CurrentRunID string `json:"current_run_id,omitempty"`
    LastRunID    string `json:"last_run_id,omitempty"`
    Desired      string `json:"desired"` // running | stopped
}
```

`Revision` increments on every successful state mutation. Maps are serialized
deterministically for reviewable fixtures; frontend snapshots sort service
names. `CurrentRunID` identifies the attempt expected to own a process.
`LastRunID` retains diagnostics when desired state becomes stopped.

### Run record schema

```go
type RunPhase string

const (
    RunPlanned  RunPhase = "planned"
    RunStarting RunPhase = "starting"
    RunReady    RunPhase = "ready"
    RunStopping RunPhase = "stopping"
    RunExited   RunPhase = "exited"
    RunFailed   RunPhase = "failed"
    RunUnknown  RunPhase = "unknown"
)

type RunRecord struct {
    Version       int               `json:"version"`
    RunID         string            `json:"run_id"`
    Service       string            `json:"service"`
    Phase         RunPhase          `json:"phase"`
    Spec          ServiceSpecRecord `json:"spec"`
    CreatedAt     time.Time         `json:"created_at"`
    UpdatedAt     time.Time         `json:"updated_at"`
    Wrapper       *ProcessIdentity  `json:"wrapper,omitempty"`
    Child         *ProcessIdentity  `json:"child,omitempty"`
    ChildPGID     int               `json:"child_pgid,omitempty"`
    Health        *HealthResult     `json:"health,omitempty"`
    Exit          *ExitSummary      `json:"exit,omitempty"`
    ArtifactDir   string            `json:"artifact_dir"`
    LastError     *OperatorError    `json:"last_error,omitempty"`
}

type ProcessIdentity struct {
    PID        int    `json:"pid"`
    StartToken string `json:"start_token"`
}
```

`StartToken` is the Linux `/proc/<pid>/stat` start-time field combined with
boot ID. A PID matches only when both PID and token match. On an unsupported
platform, process mutation must return `E_PROCESS_IDENTITY_UNSUPPORTED` until a
platform implementation exists; it must not silently fall back to PID-only
signaling.

The run record never stores raw environment values. `ServiceSpecRecord`
contains command, working directory, health configuration, and environment key
names or redacted values according to the existing sanitization policy.

### Wrapper-owned handshake files

```go
type OwnerRecord struct {
    Version   int             `json:"version"`
    RunID     string          `json:"run_id"`
    Service   string          `json:"service"`
    Wrapper   ProcessIdentity `json:"wrapper"`
    WrittenAt time.Time       `json:"written_at"`
}

type ReadyRecord struct {
    Version   int             `json:"version"`
    RunID     string          `json:"run_id"`
    Service   string          `json:"service"`
    Wrapper   ProcessIdentity `json:"wrapper"`
    Child     ProcessIdentity `json:"child"`
    ChildPGID int             `json:"child_pgid"`
    WrittenAt time.Time       `json:"written_at"`
}
```

The wrapper writes `owner.json` atomically before starting the child. It writes
`ready.json` atomically only after the child starts and its process identity is
read. The controller validates version, run ID, service, wrapper identity,
child identity, and process group. File existence alone is never readiness.

### Atomic write algorithm

Every replaceable JSON artifact uses a same-directory temporary file:

```text
function atomicWriteJSON(path, value):
    bytes = marshal(value) + newline
    temp = createExclusive(path.directory, "." + path.base + ".tmp-*", 0600)
    writeAll(temp, bytes)
    fsync(temp)
    close(temp)
    rename(temp.name, path)
    fsync(path.directory)
```

Use `github.com/pkg/errors` to wrap every filesystem operation with artifact
and run context. Cleanup of a known temporary path is best effort. Unit tests
must inject failures at write, sync, rename, and directory-sync boundaries.

### Repository mutation lock

All lifecycle mutations take an exclusive advisory lock on `.devctl/lock`.
Read-only snapshots do not take the lock because JSON replacement is atomic.
The lock metadata includes PID, process identity, operation ID, command, and
acquisition time for diagnostics.

```go
type Locker interface {
    WithExclusive(
        ctx context.Context,
        metadata LockMetadata,
        fn func(context.Context) error,
    ) error
}
```

Lock acquisition honors context cancellation and returns
`E_OPERATION_BUSY` with current lock metadata. Do not “steal” a lock based only
on age. A `doctor` check may classify an owner identity as dead and recommend
removing the stale lock, but mutation remains explicit.

## Lifecycle state machine

The state machine is per run. Desired service state is maintained separately
in the environment index.

```text
                    wrapper/child start
   planned ───────────────▶ starting ───────────────▶ ready
      │                         │                       │
      │ setup failure           │ start/health failure │ stop requested
      ▼                         ▼                       ▼
    failed ◀──────────────── failed                 stopping
                                                        │
                                      graceful/forced exit
                                                        ▼
                                                     exited

Any phase ── identity/artifact contradiction ──▶ unknown
```

`failed` means devctl has authoritative evidence of a failed attempt and no
matching live child. `unknown` means devctl cannot prove ownership or
termination. Unknown is never treated as stopped.

### Start transaction

```text
function startService(ctx, spec):
    validate spec and health contract
    acquire repository mutation lock
    reconcile durable artifacts
    reject a matching live current run

    runID = newRunID()
    create run directory (0700)
    atomically write run.json phase=planned
    atomically update state slot:
        desired=running, current_run_id=runID

    start wrapper with one request file/path
    atomically update run.json phase=starting

    wait for validated owner.json
    wait for validated ready.json
    wait for health using per-service timeout

    atomically update run.json phase=ready with identities and health
    emit typed ServiceReady event
```

The pre-start state record closes the current orphan window. If the controller
dies, reconciliation finds the current run directory. If the wrapper started,
its `owner.json` identifies it. If no owner exists, the attempt is failed and
safe to clear.

The wrapper should receive a single `--request PATH` argument. The request is a
versioned JSON object containing run ID, service, cwd, command, environment,
artifact directory, and log settings. The request file is mode `0600` and is
removed by the wrapper after loading. This reduces hidden command complexity
and prevents environment values from appearing in process arguments.

### Health contract

```go
type HealthCheck struct {
    Type             string        `json:"type"` // tcp | http
    Address          string        `json:"address,omitempty"`
    URL              string        `json:"url,omitempty"`
    Timeout          time.Duration `json:"timeout"`
    Interval         time.Duration `json:"interval"`
    ExpectedStatuses []int         `json:"expected_statuses,omitempty"`
}
```

Rules:

- `Timeout` is per service and required after defaulting.
- TCP succeeds on a completed connection.
- HTTP defaults to status `200..399`.
- `ExpectedStatuses`, when non-empty, replaces the default range.
- Every attempt records time, duration, and a bounded last error.
- Startup and observation call the same health evaluator.
- An HTTP body is closed; response content is not retained.

### Stop transaction

```text
function stopServices(ctx, names):
    acquire repository mutation lock
    reconcile durable artifacts
    for each selected service in deterministic order:
        mark desired=stopped
        if no current run:
            record no-op result
            continue
        validate wrapper and child identities
        mark run phase=stopping
        signal wrapper; wrapper forwards to child PGID
        wait shutdown timeout
        if still matching: SIGKILL child PGID
        wait and read exit.json
        if matching process remains:
            mark run unknown
            retain current_run_id
            record failure
        else:
            mark run exited
            clear current_run_id
            retain last_run_id
    persist every result
    return all service outcomes and aggregate error
```

Do not remove `state.json` during normal `down`. A stopped environment is
useful durable state. Retention cleanup is a separate operation and never
changes process ownership.

### Restart transaction

Restart resolves and validates the new plan before stopping any current
service. For environment restart, build/prepare/validate policy must be the
same as `up`; for selected-service restart, resolve the plan and validate the
selected specifications before the first stop.

```text
resolve -> build/prepare/validate as policy requires -> acquire lock
        -> stop selected services -> start selected services
```

If start fails after a successful stop, the operation returns partial failure
and preserves the stopped run plus the failed new run. It never rewrites
history to look atomic.

### Reconciliation

Reconciliation runs before every mutation and in `doctor`:

1. load versioned environment state;
2. for every current run, load run/owner/ready/exit artifacts;
3. verify run IDs and process identities;
4. advance a stale `starting` run to ready only if handshake and health prove
   it;
5. advance a run to exited only if exit data exists and identities are no
   longer alive;
6. mark contradictions unknown;
7. scan run directories referenced by no state slot and report them as
   unindexed attempts; do not signal automatically.

Reconciliation is deterministic and returns a structured report. It does not
guess ownership from command strings.

## Target log architecture

### Capture requirements

The wrapper owns child stdout and stderr pipes. It starts one goroutine per
pipe with `errgroup.WithContext`, frames records without `bufio.Scanner`, and
submits them to one sequencer. The sequencer is the only writer of
`logs.jsonl`, so record order and sequence values are deterministic within a
run.

Raw output files remain useful for exact bytes and external tools:

```text
child stdout ─▶ raw stdout writer ─┐
                                  ├─▶ line framer ─▶ sequencer ─▶ logs.jsonl
child stderr ─▶ raw stderr writer ─┘
```

Each pipe reader writes the bytes it received to the matching raw file before
framing. The record journal represents observed read order, not an assertion
about nanosecond ordering inside the child process.

Do not use a scanner token limit. Implement a byte-oriented framer with:

- newline delimiter support;
- CRLF normalization only in the `Text` field, never in raw bytes;
- a default 1 MiB record-text limit;
- `Partial=true` chunks for longer logical lines;
- a final partial record at EOF;
- ANSI bytes retained in `RawText` or the raw file and optionally stripped in
  rendered text.

### Record and cursor API

```go
type SourceKind string
type StreamKind string

const (
    SourceService  SourceKind = "service"
    SourcePipeline SourceKind = "pipeline"
    SourcePlugin   SourceKind = "plugin"
    SourceSystem   SourceKind = "system"

    StreamStdout StreamKind = "stdout"
    StreamStderr StreamKind = "stderr"
    StreamEvent  StreamKind = "event"
)

type LogRecord struct {
    Version   int            `json:"version"`
    RunID     string         `json:"run_id"`
    Sequence  uint64         `json:"sequence"`
    Time      time.Time      `json:"time"`
    Source    SourceKind     `json:"source"`
    Service   string         `json:"service,omitempty"`
    Stream    StreamKind     `json:"stream"`
    Level     string         `json:"level,omitempty"`
    Text      string         `json:"text"`
    Partial   bool           `json:"partial,omitempty"`
    Fields    map[string]any `json:"fields,omitempty"`
}

type Cursor struct {
    RunID    string `json:"run_id"`
    Sequence uint64 `json:"sequence"`
}
```

The journal contains one JSON object plus newline per record. A reader ignores
only an unterminated final line after a crash and reports
`E_LOG_TRAILING_PARTIAL` as a diagnostic. It rejects an invalid terminated
record with run ID and byte offset.

### Query and follow API

```go
type Query struct {
    RunIDs    []string
    Services  []string
    Sources   []SourceKind
    Streams   []StreamKind
    Levels    []string
    Since     *time.Time
    Until     *time.Time
    Tail      int
    Contains  string
}

type FollowRequest struct {
    Query  Query
    After  map[string]Cursor // one cursor per selected run
}

type Reader interface {
    Query(ctx context.Context, query Query) ([]LogRecord, error)
    Follow(ctx context.Context, request FollowRequest, sink LogSink) error
}

type LogSink interface {
    Add(ctx context.Context, record LogRecord) error
}
```

Query ordering is `(Time, RunID, Sequence)`. Ties use run ID then sequence, so
output is stable. `Tail` applies per service/run first and the combined set is
then merged. This prevents a noisy service from consuming every tail slot.

Follow behavior:

- opens each selected immutable run journal;
- resumes after the supplied sequence;
- detects file replacement by identity and returns corruption rather than
  silently switching;
- polls with a context-aware ticker in the first implementation;
- flushes every record to the sink;
- exits when every selected run has a terminal record;
- continues across restarts only when `FollowCurrent=true` is explicitly
  requested in a later API extension.

Active run files are not rotated. Retention applies only after a run is
terminal. This removes the current rotation problem rather than making every
reader emulate a general-purpose rotating-file tailer.

### Backpressure

The wrapper must not deadlock a child because a TUI is slow. Frontends read
from disk; they are not on the capture path. Within the wrapper:

- stdout and stderr readers have bounded channels;
- the sequencer drains continuously to local files;
- filesystem write failure terminates the child and writes the best possible
  exit error because continuing without logs violates the supervision
  contract;
- no record is dropped silently;
- journal flush policy is configurable by record count/time but `exit.json`
  is written only after all pipe readers and journal sync complete.

The TUI maintains a bounded presentation buffer, defaulting to 2,000 records
and 8 MiB, whichever is reached first. It drops oldest displayed records and
shows a dropped-display count. Disk history remains queryable.

### Retention

Retention is not part of `down`. A later `devctl prune` command may delete
terminal runs by age/count after proving that:

- the run is not current;
- no matching process identity is alive;
- the run is terminal;
- the user-selected retention rule matches.

The MVP only reports disk usage in `doctor`; it does not auto-delete.

## Operator controller API

### Requests

```go
type Selection struct {
    Services []string
}

type UpRequest struct {
    RepoRoot string
    Profile  string
    Select   Selection
    Policy   PipelinePolicy
}

type DownRequest struct {
    RepoRoot string
    Select   Selection
}

type RestartRequest struct {
    RepoRoot string
    Profile  string
    Select   Selection
    Policy   PipelinePolicy
}

type SnapshotRequest struct {
    RepoRoot      string
    IncludeRuns   bool
    IncludeHealth bool
}
```

Empty selection means all configured services for `up` and all current
services for `down`/`restart`. Unknown explicit services are errors before
mutation.

### Typed events

```go
type OperatorEvent struct {
    Version     int
    OperationID string
    At          time.Time
    Kind        EventKind
    Phase       string
    Service     string
    Status      string
    Message     string
    Error       *OperatorError
    Fields      map[string]any
}

type EventSink interface {
    Send(context.Context, OperatorEvent) error
}
```

`Message` is display text. `Kind`, `Phase`, `Status`, and `Error.Code` are the
control contract. Frontends may render `Message` but never parse it.

Required event kinds:

```text
operation.started
operation.finished
phase.started
phase.finished
service.planned
service.starting
service.ready
service.unhealthy
service.stopping
service.exited
service.failed
service.unknown
diagnostic
```

### Results and errors

```go
type ServiceOutcome struct {
    Service  string
    RunID    string
    Before   RunPhase
    After    RunPhase
    Changed  bool
    Exit     *ExitSummary
    Error    *OperatorError
}

type OperationResult struct {
    OperationID string
    Kind        string
    StartedAt   time.Time
    FinishedAt  time.Time
    Status      string // succeeded | failed | partial | canceled
    Outcomes    []ServiceOutcome
}

type OperatorError struct {
    Code      string         `json:"code"`
    Message   string         `json:"message"`
    Operation string         `json:"operation,omitempty"`
    Service   string         `json:"service,omitempty"`
    RunID     string         `json:"run_id,omitempty"`
    Path      string         `json:"path,omitempty"`
    Details   map[string]any `json:"details,omitempty"`
}
```

Stable initial error codes:

```text
E_USAGE
E_CONFIG_MISSING
E_CONFIG_INVALID
E_STATE_VERSION
E_STATE_CORRUPT
E_OPERATION_BUSY
E_SERVICE_UNKNOWN
E_SERVICE_ALREADY_RUNNING
E_PROCESS_IDENTITY_UNSUPPORTED
E_PROCESS_IDENTITY_MISMATCH
E_WRAPPER_START
E_WRAPPER_HANDSHAKE
E_HEALTH_TIMEOUT
E_STOP_FAILED
E_PARTIAL_FAILURE
E_LOG_CORRUPT
E_CANCELED
```

Wrap implementation causes with `github.com/pkg/errors`, but serialize only
the stable operator error and safe context. Do not place raw environment
values in errors, events, state, or logs.

### Controller interface

```go
type Controller interface {
    Up(
        ctx context.Context,
        request UpRequest,
        sink EventSink,
    ) (OperationResult, error)
    Down(
        ctx context.Context,
        request DownRequest,
        sink EventSink,
    ) (OperationResult, error)
    Restart(
        ctx context.Context,
        request RestartRequest,
        sink EventSink,
    ) (OperationResult, error)
    Snapshot(
        ctx context.Context,
        request SnapshotRequest,
    ) (Snapshot, error)
    Logs() runlog.Reader
    Doctor(
        ctx context.Context,
        request DoctorRequest,
    ) (DoctorReport, error)
}
```

Every implementation includes:

```go
var _ operator.Controller = (*controller)(nil)
```

Long-running goroutines use `errgroup`. Every blocking operation accepts a
context. Cleanup that must outlive cancellation uses a new bounded context and
records cleanup failure.

## CLI specification

### Command tree

```text
devctl
  up [SERVICE...]
  down [SERVICE...]
  restart [SERVICE...]
  status [SERVICE...]
  logs [SERVICE...]
  doctor
  plan
  build
  prepare
  validate
  profiles ...
  plugin
    list
    command list
    command run PLUGIN COMMAND -- ARGS...
  stream
    start ...
  tui
  dev ...                         hidden
  __wrap-service --request PATH   hidden
```

Remove `start`, `stop-service`, automatic top-level plugin commands, and the
standalone log parser from the core command tree according to the consumer
gate. No aliases remain.

### Logs flags

```text
devctl logs [SERVICE...]
  -f, --follow
  -n, --tail N                  default 100 per service/run; -1 means all
      --since TIME_OR_DURATION
      --until TIME_OR_DURATION
      --source service|pipeline|plugin|system   repeatable
      --stream stdout|stderr|event              repeatable
      --level LEVEL                           repeatable
      --contains TEXT
      --run RUN_ID                            repeatable
      --timestamps                            default true in combined mode
      --no-prefix
      --ansi auto|always|never
      --output text|json|yaml|csv|...
```

Rules:

- no service arguments selects all services;
- text prefixes are enabled whenever more than one service or stream is
  selected;
- `--output json` is JSON Lines during follow;
- `--until` and `--follow` are mutually exclusive;
- `--tail -1` means all, `0` means no historical records before follow;
- time accepts RFC3339/RFC3339Nano or a Go duration relative to now;
- unknown services and runs are usage errors before reading;
- cancellation exits zero for an interactive Ctrl-C after records were
  emitted, but an already-canceled input context returns `E_CANCELED`.

### Status rows

One row per selected service:

```text
service
desired
phase
run_id
wrapper_pid
child_pid
health
started_at
uptime
exit_code
signal
last_error_code
stdout_path
stderr_path
```

Human default selects a concise field subset. Glazed `--fields` can expose the
rest. With no state, status emits one environment summary row indicating
`stopped`; it does not fabricate a service row.

### Doctor

`devctl doctor` is read-only by default and checks:

- configuration parse and selected profile;
- plugin executables and handshake, only when `--plugins` is supplied;
- state and run schema versions;
- atomic-artifact parse validity;
- current-run index consistency;
- wrapper and child process identities;
- process groups;
- health configuration;
- log journal continuity and trailing partial record;
- disk use and count of terminal runs;
- stale/unindexed run directories and lock metadata.

It emits one Glazed row per check:

```text
check, scope, status, code, summary, path, service, run_id, remediation
```

No check repairs state or signals a process. Repair commands require a
separate design because automatic repair is destructive.

### Exit codes and rendering

The root sets `SilenceErrors=true` and `SilenceUsage=true`. One centralized
handler renders an error once. Usage is shown only for `E_USAGE`. Exit codes:

```text
0  success or explicit no-op
1  runtime failure or partial service failure
2  usage/configuration/schema incompatibility
130 user interruption before a normal streaming shutdown
```

Glazed output and logging are initialized once at the root. Public commands do
not call `json.MarshalIndent` directly for result output.

## TUI specification

### Global layout

```text
┌ devctl  PROFILE  ENVIRONMENT-STATUS ───────────── operation/status ┐
│ [1] Overview   [2] Logs   [3] Runs                  [:] Commands   │
├────────────────────────────────────────────────────────────────────┤
│                                                                    │
│                         active view                                │
│                                                                    │
├────────────────────────────────────────────────────────────────────┤
│ contextual keys                                      [?] [q]      │
└────────────────────────────────────────────────────────────────────┘
```

Number keys switch directly. `Tab` cycles. `Esc` closes a modal or returns to
the parent state. `q` quits only when no confirmation modal is active.

### Overview

```text
Services
NAME       DESIRED   STATE      HEALTH      PID      AGE       LAST ERROR
api        running   ready      healthy     1234     08m       -
worker     running   failed     unknown     -        12s       E_EXIT
web        stopped   exited     -           -        04m       -

Selected: worker
Run: 01J...   exit=1   finished 12s ago
Last error: process exited before readiness
[enter] logs  [r] restart  [d] down  [u] up
```

The service table is the primary selection surface. Health and resource
sampling update fields but do not create event-history lines. Destructive
actions show a confirmation with exact service names.

### Logs

```text
Filters: services=api,worker  streams=stdout,stderr  follow=on  dropped=0
13:44:01.120 api    stdout  listening on :8080
13:44:01.381 worker stderr  connection refused
13:44:02.009 api    stdout  GET /health 200

[/] search  [s] services  [o] streams  [f] follow  [w] wrap  [p] pause
```

The view consumes `runlog.Reader`; it does not open files. Pausing stops
viewport advancement, not disk capture. Search filters already buffered
records. The header always shows run selection and whether history was
truncated for display.

### Runs

```text
Operation restart  op=01J...  partial  3.42s
✓ resolve plan             120ms
✓ validate                  48ms
✓ stop api                 410ms
✗ start api               2.84s  E_HEALTH_TIMEOUT

Service api / run 01J...
Health: timeout after 2.5s; last error: GET http://... connection refused
[enter] details  [l] related logs  [c] copy error
```

Runs presents typed phases and outcomes from the shared controller. It does not
infer completion from strings. The current operation is in memory; completed
service attempts come from durable run records.

### Command palette

The palette contains actions that do not deserve permanent navigation:

```text
Start selected services
Stop selected services
Restart selected services
Open logs for selected service
Run doctor
Refresh snapshot
Inspect plugins (opens/prints CLI command guidance)
Start protocol stream (opens/prints CLI command guidance)
```

Plugin inspection and arbitrary stream construction do not run inside the MVP
TUI. The palette entry may display the exact CLI command.

### TUI update architecture

```go
type SnapshotMsg struct{ Snapshot operator.Snapshot }
type EventMsg struct{ Event operator.OperatorEvent }
type LogMsg struct{ Record runlog.LogRecord }
type OperationDoneMsg struct {
    Result operator.OperationResult
    Err    error
}
```

Bubble Tea commands call the controller and send typed messages. A background
snapshot watcher compares revisions before sending. No Watermill dependency,
JSON envelope, prefix parser, direct syscall, file tailer, or plugin
introspection remains under `pkg/tui`.

## Security and privacy requirements

Devctl executes repository-controlled plugins and services. The redesign does
not make those inputs trusted.

- Plugin stdout remains protocol-only. Diagnostics remain on stderr.
- Service commands are executed directly from `[]string`; do not add implicit
  shell evaluation.
- Raw environment values may exist only in the mode-0600 wrapper request and
  child environment. The wrapper removes the request after decoding.
- State, events, error details, diagnostic rows, and run records contain
  sanitized environment data only.
- Run directories are mode `0700`; files containing output or process metadata
  are `0600`.
- Service names are validated identifiers and never used unescaped as path
  components.
- Run IDs are generated, never accepted as arbitrary relative paths.
- All artifact paths are joined beneath a validated `.devctl/runs` root and
  checked against traversal.
- Log rendering treats ANSI as untrusted. `auto` preserves color only to a TTY
  and strips control sequences that can alter the terminal outside normal SGR
  color/style codes.
- TUI copy/export actions are explicit. No log content is sent over a network.
- `doctor` reports paths and identifiers but not secret values.

Add gosec coverage for command execution and path construction. Comments
justifying `#nosec G204` must state that the command comes from the validated
repository plan.

## Decisions

### D1: Keep the wrapper

**Decision:** Keep a detached wrapper per service run.

**Reason:** It survives the initiating CLI, forwards signals to the child
group, drains logs, and writes exit metadata. Removing it would require a
daemon or would lose durable exit observation.

### D2: Durable files, not a daemon

**Decision:** Use atomic versioned files and a repository lock.

**Reason:** Current workflows are repository-local and do not prove a need for
independent remote clients. A daemon increases lifecycle, security, port, and
upgrade complexity.

### D3: One shared controller

**Decision:** CLI and TUI call `pkg/operator.Controller`.

**Reason:** Current duplicate lifecycle logic already diverges. A controller
provides one semantic boundary without network transport.

### D4: Run-scoped sequenced log journal

**Decision:** The wrapper writes raw stream files plus `logs.jsonl`.

**Reason:** Raw files preserve exact bytes. Structured records provide source,
time, sequence, cursors, combined display, and stable machine output.

### D5: Breaking CLI cleanup

**Decision:** Replace `start`/`stop-service` with service arguments on
`up`/`down`, make logs positional, and remove automatic top-level plugin
commands.

**Reason:** One vocabulary is easier to learn and script. The project
explicitly does not require backward-compatibility adapters.

### D6: Three-view TUI

**Decision:** Ship Overview, Logs, and Runs.

**Reason:** These views answer daily operator questions. Plugin and stream
views expose implementation/developer surfaces and remain available through
explicit CLI commands.

### D7: Remove standalone log parsing from devctl

**Decision:** Remove the Goja parser subsystem if the one-time downstream
consumer check is empty; otherwise extract it under a separate ticket.

**Reason:** It is a separate 1,668-line product with no current operator-path
integration. Integrating it would delay reliable raw logs and enlarge the
failure surface.

### D8: No state compatibility layer

**Decision:** State schema v2 refuses unversioned/v1 state.

**Reason:** `.devctl` state is ephemeral process ownership data. Guessing or
migrating live ownership is unsafe. Release instructions require users to stop
the old environment with the old binary before upgrading. No adapter or
automatic migration is added.

## Implementation guide

The phases are ordered by safety dependency. An intern should not start with
TUI rendering because every view depends on the durable model and controller.
Each phase ends in a commit that independently builds and passes its gate.

### Phase 0: Baseline and destructive-change gate

Goal: establish test fixtures and confirm removal scope before changing public
behavior.

Tasks:

1. Run and archive:
   - `go test ./...`
   - `go build ./...`
   - `make lint`
   - the four ticket probe scripts.
2. Add fixture helpers that create repository-local temporary `.devctl`
   directories. Tests must never use a developer's actual `.devctl`.
3. Search known go-go-golems repositories and `go.work` files for:
   - imports of `devctl/pkg/logjs`;
   - invocations of `log-parse`;
   - invocations of `devctl start`, `stop-service`, or
     `logs --service`.
4. Record results in the implementation ticket.
5. If external logjs consumers exist, open the extraction ticket and exclude
   parser deletion from this project. Do not write a compatibility adapter.

Acceptance:

- baseline commands and probe outputs are committed to the implementation
  diary;
- no production behavior changes;
- removal gate has an explicit pass/extract outcome.

### Phase 1: Versioned atomic run state

Goal: introduce `pkg/runstate` without changing process startup.

Files:

- add `pkg/runstate/schema.go`;
- add `pkg/runstate/store.go`;
- add `pkg/runstate/lock.go`;
- add `pkg/runstate/identity_linux.go`;
- add `pkg/runstate/identity_other.go`;
- add table-driven tests and golden JSON fixtures.

Implementation order:

1. Define schema constants and types.
2. Implement path validation and run-directory creation.
3. Implement atomic JSON write/read.
4. Implement revision-checked environment update:

   ```go
   func (s *Store) Update(
       ctx context.Context,
       expectedRevision uint64,
       fn func(*EnvironmentState) error,
   ) error
   ```

5. Implement the advisory lock with context cancellation.
6. Implement Linux boot ID plus `/proc/<pid>/stat` start token parsing. Parse
   the command name by locating the final `)` as current zombie code does.
7. Implement process identity comparison without signaling.

Tests:

- JSON round trip and deterministic fixture;
- interrupted write leaves old file valid;
- rename failure preserves old file;
- concurrent readers observe only old or new revision;
- stale expected revision returns conflict;
- lock contention cancels promptly;
- PID with wrong token is not a match;
- process stat names containing spaces and `)` parse correctly;
- unsupported platform does not claim PID-only safety.

Acceptance:

- no direct `os.WriteFile` remains for mutable state/exit JSON;
- `go test ./pkg/runstate/... -race` passes;
- repository paths cannot escape `.devctl`.

Commit: `feat(runstate): add versioned atomic run records`

### Phase 2: Wrapper request and durable handshake

Goal: close the startup ownership gap while retaining current log files.

Files:

- add `pkg/supervise/wrapper_request.go`;
- refactor `cmd/devctl/cmds/wrap_service.go`;
- refactor `pkg/supervise/supervisor.go`;
- extend wrapper integration tests.

Implementation order:

1. Define versioned `WrapperRequest`, `OwnerRecord`, and `ReadyRecord`.
2. Make the hidden command accept only `--request PATH`.
3. Load and validate the request, then remove it.
4. Atomically write `owner.json`.
5. start the child in a separate group;
6. acquire child identity and atomically write `ready.json`;
7. preserve the fixed signal-forwarding topology;
8. make supervisor validate full handshake records.
9. Pre-create planned state and run records before wrapper start.
10. Record failed start attempts rather than deleting them.

Tests:

- owner record exists before child fixture reports execution;
- request file is removed and mode is 0600;
- wrong run ID, service, wrapper PID/token, child PID/token, or PGID is rejected;
- missing owner and missing ready yield distinct error codes;
- controller crash simulation leaves a reconcilable run;
- child and grandchild receive one termination signal;
- wrapper does not signal itself recursively;
- failed child start produces terminal exit/failed data.

Use tmux only for manual long-running validation. Kill test web servers with
`lsof-who -p $PORT -k`.

Acceptance:

- no window exists where a started child lacks a pre-created run reference;
- ready-file existence is not used as proof;
- wrapper regression test from `39ba416` remains green.

Commit: `feat(supervise): persist wrapper ownership handshake`

### Phase 3: Shared lifecycle controller

Goal: make one application service authoritative for CLI and future TUI.

Files:

- add `pkg/operator/*`;
- move pipeline/lifecycle policy out of `up.go` and `action_runner.go`;
- add reconciliation;
- leave old frontends temporarily calling the controller in the same commit
  series, not through adapters.

Implementation order:

1. Define requests, events, results, stable errors, and interfaces.
2. Implement `Snapshot` and `Doctor` read paths first.
3. Implement reconciliation.
4. Implement `Up` under the mutation lock.
5. Implement `Down` with per-service outcomes and retained state.
6. Implement `Restart` with resolve/validate-before-stop ordering.
7. Make all event emissions typed and best-effort only where losing a
   presentation event cannot alter lifecycle safety.
8. Delete duplicated lifecycle functions from `pkg/tui/action_runner.go` when
   the TUI switches; do not leave a second implementation.

Tests:

- no-state down is a successful no-op;
- unknown explicit selection fails before mutation;
- multi-service stop returns every outcome;
- one failed stop retains its current run ID and yields partial status;
- restart resolve failure leaves current process running;
- restart start failure retains stopped and failed run history;
- cancellation at every phase leaves a reconcilable state;
- health startup and snapshot share status rules;
- raw environment values do not appear in artifacts/events/errors;
- race test covers simultaneous snapshot and mutation.

Acceptance:

- all public lifecycle behavior can be exercised through a fake `EventSink`;
- no lifecycle policy remains in Cobra or Bubble Tea packages;
- stop errors are never discarded.

Commit: `feat(operator): centralize lifecycle operations`

### Phase 4: Sequenced run logs

Goal: implement `pkg/runlog` and make the wrapper the only capture writer.

Files:

- add `pkg/runlog/record.go`, `writer.go`, `framer.go`, `reader.go`,
  `follow.go`;
- update wrapper;
- add high-volume and lifecycle tests.

Implementation order:

1. Implement the byte framer independently with fuzz tests.
2. Implement the sequencer and single-write JSONL encoder.
3. fan in stdout/stderr using `errgroup`;
4. write exact raw bytes plus structured records;
5. sync the journal before exit metadata;
6. implement query filters and stable merge;
7. implement context-aware follow and cursor resume.

Tests:

- stdout/stderr records have unique increasing sequence numbers;
- CRLF, no-final-newline, empty lines, invalid UTF-8, ANSI, and >1 MiB lines;
- final partial record after simulated crash is ignored and diagnosed;
- terminated invalid JSON record is a hard error;
- tail is applied per run/service;
- time/source/stream/level/text filters compose;
- cursor resume emits no duplicate;
- follow cancellation completes within 250 ms;
- 100,000-record fixture remains within documented memory bounds;
- slow reader does not block service capture because it reads disk separately;
- disk-full/write failure terminates the child and records failure if possible.

Acceptance:

- ticket log lifecycle probe is replaced by package tests that pass for append
  and immutable active run files;
- CLI and TUI no longer need separate tail implementations after their phases.

Commit: `feat(runlog): add run-scoped structured log journal`

### Phase 5: Glazed CLI replacement

Goal: ship the new command and output contract as a deliberate breaking
change.

Files:

- refactor `cmd/devctl/cmds/{up,down,restart,status,logs}.go`;
- add `doctor.go`;
- reorganize explicit `plugin/` and `stream/` groups;
- update root error handling and embedded help.

For each command:

1. create a struct embedding `*cmds.CommandDescription`;
2. create a settings struct with `glazed` tags;
3. compose repository, Glazed output, and command-settings sections;
4. define service positionals with `TypeStringList`;
5. decode with `values.Values`;
6. call `operator.Controller`;
7. emit Glazed rows;
8. document examples and failure behavior.

Delete:

- `newStartServiceCmd`;
- `newStopServiceCmd`;
- automatic dynamic top-level command registration;
- CLI-local `readTailLines`, `followFile`, and raw JSON marshaling;
- log-parser subsystem if Phase 0 gate passed.

Tests:

- golden `--help` command tree;
- human and JSON output for every no-state/success/partial/error case;
- stdout contains only data;
- each error appears once;
- services are valid before mutation;
- logs flag conflict matrix;
- JSON follow produces one object per line and flushes;
- exit codes match the specification;
- built-in command help never starts a plugin;
- root initialization includes logging and embedded Glazed help.

Acceptance:

- `rg 'stop-service|logs --service'` finds only release notes/historical
  tickets;
- every snapshot/list/outcome public command uses Glazed processors;
- old command spellings fail as unknown commands, with no aliases.

Commit: `feat(cli): unify lifecycle and log commands`

### Phase 6: TUI replacement

Goal: replace rather than incrementally adapt the six-view TUI.

Build a new typed model alongside current code only within the feature branch.
Delete the old path before merge.

Implementation order:

1. add pure Overview model/update/view tests;
2. add pure Logs model with bounded buffer tests;
3. add pure Runs model with typed phase tests;
4. add Root model and direct navigation;
5. wire controller actions through `tea.Cmd`;
6. wire revision-aware snapshots and runlog follow;
7. add confirmations and command palette;
8. delete Watermill bus, envelopes, transformer, forwarder, action runner,
   stream runner/view, plugin view, and model-local file/syscall code.

Tests:

- every update branch is table tested;
- snapshot with same revision emits no history event;
- health sample changes fields without adding run history;
- partial operation renders service-specific failure;
- status never depends on message text;
- paused logs remain bounded and report display drops;
- filter/follow/wrap state survives view switches;
- confirmation names exact target services;
- quitting cancels watchers and actions with no goroutine leak;
- 80x24, 120x30, and narrow-terminal golden views;
- keyboard navigation and modal precedence;
- no TUI test starts plugins or signals real PIDs.

Manual tmux matrix:

```text
no config
configured but stopped
two healthy services
one startup health failure
one service exits non-zero
partial stop failure fixture
high-volume mixed stdout/stderr
long line and ANSI output
restart while Logs view follows old run
Ctrl-C during up and restart
```

Acceptance:

- three primary views only;
- no JSON serialization inside TUI message routing;
- no `syscall.Kill`, `os.Open` for logs, or `state.Save` under `pkg/tui`;
- `go test ./pkg/tui/... -race` has meaningful coverage.

Commit: `feat(tui): replace operator interface with typed three-view model`

### Phase 7: Cleanup, documentation, and release

Tasks:

1. Remove dead dependencies, including Watermill if no non-TUI consumer
   remains.
2. Run `go mod tidy`.
3. Update README, embedded help, architecture docs, and screenshot playbook.
4. Add an upgrade note:
   - stop environments with the old devctl before upgrade;
   - v2 refuses old state;
   - list removed commands and replacements;
   - explain run retention and disk paths.
5. Add superseded notes to old tickets without rewriting their history.
6. Run the full verification matrix.

Acceptance:

- `go build ./...`
- `go test ./...`
- `go test -race ./pkg/operator/... ./pkg/runstate/... ./pkg/runlog/... ./pkg/tui/...`
- `make lint`
- `make gosec` if available in the repository Makefile;
- all smoke tests and tmux matrix pass;
- no old path remains reachable.

Commits should separate mechanical deletion/documentation from behavior where
review would otherwise be obscured.

## File-by-file review map

| Current file/area | Future action |
|---|---|
| `pkg/supervise/supervisor.go` | retain OS primitives; remove state policy and duplicated log tail assumptions |
| `cmd/devctl/cmds/wrap_service.go` | reduce to versioned request loader and wrapper runner |
| `pkg/state/state.go` | replace with `pkg/runstate`; delete PID-only liveness |
| `pkg/state/exit_info.go` | move versioned exit schema under runstate/supervise |
| `pkg/state/tail.go` | delete after runlog consumers migrate |
| `cmd/devctl/cmds/logs.go` | replace with Glazed query/follow adapter |
| `cmd/devctl/cmds/status.go` | emit Glazed service rows from controller snapshot |
| `cmd/devctl/cmds/up.go` | retain command settings only; move operation into controller |
| `cmd/devctl/cmds/down.go` | retain command settings only; remove state deletion |
| `cmd/devctl/cmds/restart.go` | retain command settings only; call controller |
| `cmd/devctl/cmds/start_service.go` | delete |
| `cmd/devctl/cmds/stop_service.go` | delete |
| `cmd/devctl/cmds/dynamic_commands.go` | replace automatic injection with explicit plugin group |
| `pkg/tui/action_runner.go` | delete after controller wiring |
| `pkg/tui/bus.go`, `envelope.go`, `transform.go`, `forward.go` | delete |
| `pkg/tui/state_watcher.go` | replace with revision-aware controller snapshot watcher |
| `pkg/tui/models/service_model.go` | replace file tail/detail screen with Logs selection state |
| `pkg/tui/models/eventlog_model.go` | fold typed transitions into Runs/System Logs |
| `pkg/tui/models/pipeline_model.go` | replace with Runs model |
| `pkg/tui/models/plugin_model.go` | delete from TUI |
| `pkg/tui/models/streams_model.go`, `stream_runner.go` | delete from TUI; retain explicit stream CLI |
| `pkg/logjs`, `cmd/log-parse`, examples/help | remove when consumer gate passes |

## Intern onboarding sequence

Before editing:

1. Read this document through “Lifecycle state machine.”
2. Run all ticket scripts and compare output with the diary.
3. Read the wrapper regression test and draw the wrapper/child process groups.
4. Trace one `up` request from Cobra through plugins into the current
   supervisor.
5. Trace one TUI restart and identify where it differs from CLI restart.
6. Explain why PID alone is not process identity.
7. Explain why state must be durable before process ownership.
8. Explain why frontend log readers must not own capture semantics.

The mentor should approve those explanations before Phase 1. The intern is not
expected to choose schemas, command names, or TUI views; those are decisions in
this guide.

## Verification matrix

| Requirement | Unit | Integration | Manual |
|---|---|---|---|
| Atomic state | injected filesystem failures | crash/reload fixture | inspect artifacts |
| Process identity | parser/token tests | live short process/PID mismatch | `doctor` output |
| Wrapper groups | signal fixture | child+grandchild test | tmux stop |
| Startup durability | state transition tests | kill controller at each boundary | reconcile with `doctor` |
| Health consistency | evaluator table | HTTP/TCP fixtures | Overview status |
| Multi-service stop | outcome aggregation | one resistant fixture | partial result UX |
| Structured logs | framer/query tests | mixed stream service | Logs view |
| Cursor resume | reader tests | cancel/restart follower | CLI JSON Lines |
| Glazed output | row/golden tests | subprocess exit codes | shell pipelines |
| Typed TUI | model tests | fake controller | tmux matrix |
| Secret handling | redaction tests | artifact scan | inspect doctor/error |
| Shutdown cleanup | context tests | goroutine/process leak check | quit during operation |

No single green smoke test proves the redesign. Completion requires every row
appropriate to the changed phase.

## Out of scope

The following are explicitly deferred:

- a devctl daemon, REST API, or remote attach;
- production deployment supervision;
- automatic restart policies;
- service dependency scheduling and replicas;
- PTY/interactive child attachment;
- log parsing DSLs or JavaScript transforms;
- automatic log retention deletion;
- state v1 migration or command compatibility aliases;
- TUI configuration editing;
- automatic repair of unknown process ownership.

Deferral prevents reliability work from becoming a general scheduler rewrite.

## Definition of done

The future implementation ticket is complete only when:

- a run record exists before every possible live child;
- process mutation validates PID plus start identity;
- stop failures retain ownership and are visible per service;
- state and handshake JSON use atomic replacement;
- startup and observation share one health contract;
- CLI and TUI call one controller;
- all service logs use one run-aware reader API;
- multi-service logs support structured streaming and time/source filters;
- the CLI has one lifecycle vocabulary and one error renderer;
- the TUI has Overview, Logs, and Runs with typed updates;
- duplicate TUI orchestration, Watermill envelopes, direct kill, and direct
  file tailing are deleted;
- removal gates have been resolved without compatibility adapters;
- full build, test, race, lint, security, smoke, and tmux gates pass;
- help, release notes, and ticket supersession records match shipped behavior.

Anything less is partial implementation, even if individual commands appear to
work.

## References

- [Investigation Diary](../reference/01-investigation-diary.md)
- [Ticket Tasks](../tasks.md)
- [External Source Register](../sources/README.md)
- [Architecture Inventory Probe](../scripts/01-architecture-inventory.sh)
- [CLI Contract Probe](../scripts/02-cli-contract-probe.sh)
- [Log Follow Lifecycle Probe](../scripts/03-log-follow-lifecycle-probe.sh)
- [TUI State Event Probe](../scripts/04-tui-state-event-probe.sh)
