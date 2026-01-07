---
Title: Service launching and wrapper mechanics (devctl __wrap-service)
Ticket: MO-009-TUI-COMPLETE-FEATURES
Status: active
Topics:
    - backend
    - ui-components
DocType: reference
Intent: long-term
Owners: []
RelatedFiles:
    - Path: devctl/cmd/devctl/cmds/up.go
      Note: Wires WrapperExe via os.Executable and calls supervise.Start
    - Path: devctl/cmd/devctl/cmds/wrap_service.go
      Note: Internal wrapper that starts child
    - Path: devctl/pkg/runtime/factory.go
      Note: Process-group setup for plugin processes (SysProcAttr)
    - Path: devctl/pkg/state/state.go
      Note: State/log paths; ServiceRecord schema; ProcessAlive/zombie detection
    - Path: devctl/pkg/supervise/supervisor.go
      Note: Service start/stop logic; wrapper invocation; terminatePIDGroup
ExternalSources: []
Summary: Textbook-style reference for how devctl launches services via __wrap-service wrapper, including SysProcAttr, process groups, signals, ready files, logs, and exit info
LastUpdated: 2026-01-07T00:01:35.789984932-05:00
WhatFor: Provide an exhaustive guide for contributors debugging supervise/wrapper behavior and process lifecycle semantics
WhenToUse: When modifying supervisor, wrapper, signal handling, or investigating service start/stop issues
---


# Service launching and wrapper mechanics (devctl __wrap-service)

## Goal

Explain, in exhaustive detail, how `devctl` launches “services” (child processes) and manages their lifecycle using:
- a supervisor (`pkg/supervise/supervisor.go`)
- an internal wrapper command (`cmd/devctl/cmds/wrap_service.go` → `devctl __wrap-service`)
- Unix process groups and `syscall.SysProcAttr` (`Setpgid`, `Pgid`)
- log/exit-info files under `.devctl/`

This is intended to be a “textbook chapter” that lets a new contributor reason about:
- which PID represents what (wrapper vs child)
- how signals are routed
- why process groups are used
- where “ready” fits in
- how logs and exit metadata are recorded
- why subtle startup ordering matters (e.g., wrapper overhead vs supervisor timeouts)

## Context

`devctl` is a dev environment orchestrator. “Launching services” means: given a launch plan (service name + command + cwd + env + optional health checks), start multiple long-lived processes, persist enough metadata to observe/control them, and stop them reliably.

This guide focuses on the **supervise** layer that actually starts and stops processes. It does not attempt to document the plugin pipeline (config mutation/build/validate/launch planning) except insofar as it produces a `LaunchPlan` consumed by the supervisor.

### Key terms

- **Service**: A process the supervisor should start and keep running. Identified by a `Name` and a `Command` slice.
- **Wrapper**: A small process whose job is to start the real service process, capture exit information, write readiness metadata, and forward signals.
- **Child process**: The actual service executable (e.g., `bin/http-echo`) started by the wrapper.
- **Process group (PGID)**: A Unix concept for grouping processes so a signal can be delivered to *the whole group* via `kill(-pgid, sig)`.
- **Session**: A Unix concept above process groups; not directly manipulated here, but relevant for terminal signal propagation.
- **Ready file**: A file that the wrapper writes once it successfully started the child process (contains child PID as text).
- **Exit info**: A JSON file written by the wrapper when the child exits (exit code, signal, timestamps, stderr tail, etc.).

### Source files (canonical)

- Supervisor: `devctl/pkg/supervise/supervisor.go`
- Wrapper command: `devctl/cmd/devctl/cmds/wrap_service.go`
- `devctl up` entry point that wires wrapper exe: `devctl/cmd/devctl/cmds/up.go`
- State/log paths: `devctl/pkg/state/state.go`
- Process alive checks: `devctl/pkg/state/state.go` (`ProcessAlive`, zombie detection)

## Quick Reference

### The two launch modes (wrapper vs direct)

`devctl/pkg/supervise/supervisor.go` supports two modes:

1) **Direct mode** (no wrapper): `Options.WrapperExe == ""`
   - supervisor starts service directly via `exec.CommandContext(ctx, svc.Command[0], svc.Command[1:]...)`
   - logs are written directly by the supervisor to `.devctl/logs/*.stdout.log` / `.stderr.log`
   - supervisor records `ServiceRecord.PID` as the **child PID**
   - no structured exit info file unless you add one

2) **Wrapper mode** (preferred for TUI/user workflows): `Options.WrapperExe != ""`
   - supervisor starts `WrapperExe __wrap-service ... -- <service command...>`
   - wrapper starts the actual service, writes ready file + exit info
   - supervisor records `ServiceRecord.PID` as the **wrapper PID**
   - supervisor waits briefly for “ready file exists” to confirm wrapper progressed

### Where files go on disk

Paths are derived from `devctl/pkg/state/state.go`:

- State directory: `<repoRoot>/.devctl/`
- Logs directory: `<repoRoot>/.devctl/logs/`
- State file: `<repoRoot>/.devctl/state.json`

Within `.devctl/logs/`, each service launch uses a timestamp:
- `<service>-<ts>.stdout.log`
- `<service>-<ts>.stderr.log`
- `<service>-<ts>.exit.json` (wrapper mode)
- `<service>-<ts>.ready` (wrapper mode)

### The wrapper CLI surface

`devctl __wrap-service` is a hidden/internal command:

```
devctl __wrap-service \
  --service <name> \
  --cwd <dir> \
  --stdout-log <path> \
  --stderr-log <path> \
  --exit-info <path> \
  --ready-file <path> \
  --env KEY=VAL --env KEY2=VAL2 ... \
  -- <service-exe> <args...>
```

Notes:
- args after `--` are the *real* service command.
- `--env` is repeatable; it is merged with `os.Environ()` inside the wrapper.
- the wrapper writes `--ready-file` after `child.Start()` and writes `--exit-info` on exit (or on start failure).

### How signals are meant to work (process-group model)

In wrapper mode, the intended model is:

1) Supervisor starts wrapper in a new process group:
   - `cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}`
   - On Linux, this causes wrapper PID = wrapper PGID (new group leader).

2) Wrapper starts child and puts it into the wrapper’s process group:
   - `pgid := os.Getpid()` (wrapper pid)
   - `child.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pgid: pgid}`

3) Wrapper forwards signals it receives to its process group:
   - `signal.Notify(... SIGTERM, SIGINT, SIGHUP)`
   - handler does `syscall.Kill(-pgid, sig)`

4) Supervisor stops “a service” by sending SIGTERM/SIGKILL to the wrapper’s process group:
   - `terminatePIDGroup(ctx, wrapperPID, timeout)` does:
     - pgid := Getpgid(wrapperPID)
     - Kill(-pgid, SIGTERM), wait, then Kill(-pgid, SIGKILL) if needed

So “stop the wrapper group” stops *wrapper + child(ren)* together.

### PID map: who is who?

| Concept | PID stored in state? | Where | Why |
|--------|------------------------|-------|-----|
| Wrapper PID | Yes (wrapper mode) | `state.ServiceRecord.PID` | Used as the handle for stopping via process group |
| Child PID | Not in state | written to `*.ready` file | Used only as a “wrapper progressed” marker (today); could be used later for proc stats, etc. |

Important: the TUI and other code that interpret `ServiceRecord.PID` need to remember it might be the wrapper PID (not the real service PID).

## Usage Examples

### Example 1: Launch via `devctl up` (normal user flow)

At the top-level CLI:
```bash
devctl --repo-root /path/to/repo up
```

Internally:
1) build a launch plan via plugins (outside the scope of this doc)
2) instantiate `supervise.Supervisor` with:
   - `RepoRoot`
   - `WrapperExe = os.Executable()` (path to the devctl binary)
3) start each plan service via wrapper mode
4) save `state.json` with wrapper PIDs and log/exit file paths

### Example 2: Run the wrapper manually (debugging)

This helps isolate whether:
- supervisor can start wrapper
- wrapper can start child
- ready file is written
- exit info is written

```bash
REPO_ROOT=/tmp/some-repo
LOGDIR="$REPO_ROOT/.devctl/logs"
mkdir -p "$LOGDIR"

devctl __wrap-service \
  --service backend \
  --cwd "$REPO_ROOT" \
  --stdout-log "$LOGDIR/backend.stdout.log" \
  --stderr-log "$LOGDIR/backend.stderr.log" \
  --exit-info "$LOGDIR/backend.exit.json" \
  --ready-file "$LOGDIR/backend.ready" \
  --env ENV=development \
  -- /path/to/bin/http-echo --port 8080
```

### Example 3: Stop semantics (process groups)

When you stop via `devctl down`, the supervisor loads `state.json`, reads each `ServiceRecord.PID`, and calls `terminatePIDGroup`:

Pseudocode:
```go
pgid, err := syscall.Getpgid(pid)
if err == nil {
  syscall.Kill(-pgid, SIGTERM) // note the negative pid => group
} else {
  syscall.Kill(pid, SIGTERM)
}
```

So if `pid` is the wrapper PID and it is also the process group leader, both wrapper and child should get SIGTERM.

## The full story (textbook-style)

This section is intentionally long and explicit. It repeats some information from the Quick Reference, but adds the “why” and the exact ordering constraints.

### 1) `LaunchPlan` becomes a series of `startService` calls

The supervisor API:

- `supervise.New(opts Options) *Supervisor`
- `(*Supervisor).Start(ctx, plan) (*state.State, error)`
- `(*Supervisor).Stop(ctx, st) error`

`Start`:
1) ensures `.devctl/logs/` exists
2) iterates `plan.Services`:
   - calls `startService` for each service spec
   - appends returned `state.ServiceRecord` to in-memory state
3) iterates `plan.Services` again and runs health checks (`waitReady`) for those that define them

Key separation:
- “ready file exists” is not the same as “health check succeeded”.
- ready file means the wrapper started the child process.
- health check means the child process is actually listening/responding.

### 2) How the supervisor derives working directory and log paths

For each service:

- base working directory is `Options.RepoRoot`
- if `svc.Cwd` is set:
  - absolute: use it directly
  - relative: join with repo root

Then it produces timestamped log paths:

```go
ts := time.Now().Format("20060102-150405")
stdoutPath := filepath.Join(state.LogsDir(repoRoot), svc.Name+"-"+ts+".stdout.log")
stderrPath := filepath.Join(state.LogsDir(repoRoot), svc.Name+"-"+ts+".stderr.log")
exitInfoPath := filepath.Join(state.LogsDir(repoRoot), svc.Name+"-"+ts+".exit.json")
readyPath := filepath.Join(state.LogsDir(repoRoot), svc.Name+"-"+ts+".ready")
```

This design has two important consequences:
1) Multiple runs generate new log files (no overwrite).
2) Tools that “tail logs” need to discover the latest file by timestamp or track it from state.

### 3) Direct mode (no wrapper)

Direct mode exists mainly as a fallback:
- used in some smoke tests and potentially environments where wrapper isn’t available

In direct mode the supervisor:
1) opens stdout/stderr log files
2) starts the service directly (the service is the child)
3) sets `cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}`
4) returns a `ServiceRecord` whose `PID` is the child PID

Why `Setpgid: true` even in direct mode?
- It isolates each service into its own process group, so `terminatePIDGroup` can kill the whole subtree if the service forks.

### 4) Wrapper mode (why it exists)

Wrapper mode exists because “start child and then later collect exit info” is surprisingly hard to do cleanly from the supervisor once you mix in:
- log files that should exist even if the service fails early
- structured exit metadata (exit code, signal, stderr tail)
- signal forwarding for a process tree
- TUI-driven stop/restart actions that should behave predictably

Instead of asking the supervisor to do everything, `devctl` runs a small wrapper process for each service.

Wrapper mode splits responsibilities:

- Supervisor:
  - chooses paths (`stdoutLog`, `stderrLog`, `exitInfoPath`, `readyPath`)
  - starts wrapper in a new process group
  - tracks wrapper PID for stop semantics

- Wrapper:
  - opens stdout/stderr logs itself
  - starts the *real service* child process
  - writes ready file after successful `Start()`
  - waits for child exit and writes exit info JSON
  - forwards signals to the process group (wrapper + child)

### 5) `__wrap-service` is a command inside the same devctl binary

The wrapper is not a separate binary. It is implemented as a hidden Cobra command:
- `cmd/devctl/cmds/wrap_service.go`
- registered in `cmd/devctl/cmds/root.go` (`AddCommands`)

So `WrapperExe` is just “path to devctl binary” and the supervisor runs:
```bash
<devctl-binary> __wrap-service --service ... -- --real-command ...
```

Why do this?
- avoids packaging a second helper binary
- guarantees wrapper and supervisor versions match
- keeps wrapper “private” (hidden CLI)

### 6) Process groups, `SysProcAttr`, and why negative PIDs matter

Unix background:

- Every process has:
  - PID (process id)
  - PGID (process group id)
  - SID (session id)

- `kill(pid, sig)` sends a signal to one process
- `kill(-pgid, sig)` sends a signal to the entire process group

Go API:
- `exec.Cmd.SysProcAttr` controls low-level OS process attributes at exec time.
- On Unix, `syscall.SysProcAttr{Setpgid: true}` means: create a new process group for the child process (or ensure it is in a group distinct from the parent).
- `Pgid: <id>` means: put the child into an existing process group with that id.

In devctl:

Supervisor launching wrapper:
```go
cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
```

Effect (typical Linux):
- wrapper becomes a new process group leader
- wrapper PGID == wrapper PID

Wrapper launching child:
```go
pgid := os.Getpid() // wrapper pid, also wrapper pgid
child.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pgid: pgid}
```

Effect:
- child joins wrapper’s process group
- now wrapper and child receive signals sent to -pgid

Stop logic:
`terminatePIDGroup(ctx, pid)` does:
1) `pgid, err := syscall.Getpgid(pid)`
2) if ok: `syscall.Kill(-pgid, SIGTERM)` else `syscall.Kill(pid, SIGTERM)`
3) wait loop using `state.ProcessAlive(pid)` (PID is wrapper PID in wrapper mode)
4) escalate to SIGKILL if needed

Important nuance: the wait loop checks whether the **pid** is alive, not whether the group is empty. That works if:
- wrapper exits when child exits, which it does because wrapper does `child.Wait()` and then returns.

### 7) Signal forwarding inside the wrapper

The wrapper registers:
```go
signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)
go func() {
  for s := range sigCh {
    _ = syscall.Kill(-pgid, s.(syscall.Signal))
  }
}()
```

Interpretation:
- Any time wrapper gets SIGTERM/SIGINT/SIGHUP, it re-sends that signal to the whole process group.
- Because wrapper is in the group, the wrapper will also receive the forwarded signal.
  - That’s okay: repeated SIGTERM is benign, and wrapper will typically be in the process of shutting down.

Why do this at all if the supervisor already kills the process group?

1) It makes wrapper behavior correct even if signals are delivered to wrapper directly (e.g., by a user, or a parent supervisor).
2) It ensures the child receives signals even if only wrapper is targeted.
3) It keeps “kill the group” semantics consistent even if the child forks and creates subchildren within the same group.

### 8) “Ready” is a file, not an IPC protocol

The wrapper writes ready file immediately after `child.Start()`:
```go
os.WriteFile(readyFile, []byte(fmt.Sprintf("%d\n", child.Process.Pid)), 0644)
```

The supervisor does not parse the PID; it only checks for file existence:
```go
if _, err := os.Stat(readyPath); err == nil { break }
```

The ready file is used as:
- a cheap “wrapper got past child.Start()” marker
- a guard against silent wrapper startup failure

It is *not* a health check. Health is separate (`waitReady`).

### 9) Why wrapper startup timing matters

Because the supervisor uses a short “ready file exists” deadline (currently hard-coded to 2 seconds), anything that delays wrapper execution before it reaches `child.Start()` can cause:
- ready file never appears
- supervisor kills wrapper group
- user sees `wrapper did not report child start`

This is a general design constraint:
- wrapper startup must be “minimal” and deterministic
- avoid doing unrelated work in devctl process initialization that could delay `__wrap-service`

### 10) Exit info (why it’s useful)

When the child exits, wrapper writes a JSON file containing:
- service name
- child PID
- started_at / exited_at
- exit code or signal
- error string (e.g., `signal: terminated`)
- stderr tail (last N lines) to help diagnose failures

This gives UI and CLI commands structured data to present:
- “short-lived exited code=0”
- “backend exited by signal=terminated”
- “stderr last lines: …”

## Related

- Existing bug analysis: `devctl/ttmp/2026/01/06/MO-009-TUI-COMPLETE-FEATURES--complete-tui-features-per-mo-006-design/analysis/03-devctl-wrapper-startup-failure-in-comprehensive-fixture.md`

## Related

<!-- Link to related documents or resources -->
