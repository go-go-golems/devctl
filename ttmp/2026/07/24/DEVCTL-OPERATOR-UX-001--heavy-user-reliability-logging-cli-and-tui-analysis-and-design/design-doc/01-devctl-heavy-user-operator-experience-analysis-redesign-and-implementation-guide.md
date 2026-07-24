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

## References

- [Investigation Diary](../reference/01-investigation-diary.md)
- [Ticket Tasks](../tasks.md)
