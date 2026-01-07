---
Title: Devctl wrapper startup failure in comprehensive fixture
Ticket: MO-009-TUI-COMPLETE-FEATURES
Status: active
Topics:
    - backend
    - ui-components
DocType: analysis
Intent: long-term
Owners: []
RelatedFiles:
    - Path: devctl/cmd/devctl/cmds/dynamic_commands.go
      Note: Calls commands.list with 3s timeout for each plugin
    - Path: devctl/cmd/devctl/cmds/wrap_service.go
      Note: Wrapper command that writes ready file after child.Start
    - Path: devctl/cmd/devctl/main.go
      Note: Runs dynamic plugin command discovery before executing commands
    - Path: devctl/pkg/protocol/types.go
      Note: Handshake capabilities define supported ops
    - Path: devctl/pkg/runtime/client.go
      Note: SupportsOp available to gate commands.list calls
    - Path: devctl/pkg/supervise/supervisor.go
      Note: Starts wrapper and waits 2s for ready file
    - Path: devctl/ttmp/2026/01/06/MO-009-TUI-COMPLETE-FEATURES--complete-tui-features-per-mo-006-design/scripts/setup-comprehensive-fixture.sh
      Note: Defines fixture plugins including logger.py which blocks commands.list
ExternalSources: []
Summary: Investigate why devctl up fails with 'wrapper did not report child start' under the comprehensive fixture; identify root cause and fixes
LastUpdated: 2026-01-06T23:56:50.825268996-05:00
WhatFor: Root-cause analysis of wrapper/ready-file startup failure triggered by dynamic plugin command discovery
WhenToUse: When fixing supervise wrapper startup and comprehensive fixture execution
---


# Devctl wrapper startup failure in comprehensive fixture

## TL;DR

The error `wrapper did not report child start` during `devctl up` in the comprehensive fixture is primarily a **startup ordering/timeout bug**:

- The supervisor expects the wrapper process to create a ready file within **2 seconds** (`devctl/pkg/supervise/supervisor.go`).
- The wrapper is the `devctl` binary itself running the hidden cobra command `__wrap-service` (`devctl/cmd/devctl/cmds/wrap_service.go`).
- **Before Cobra executes any command**, `devctl` always runs dynamic plugin command discovery (`devctl/cmd/devctl/main.go` → `devctl/cmd/devctl/cmds/dynamic_commands.go`).
- In the comprehensive fixture, at least one plugin (`plugins/logger.py`) never responds to `commands.list`, so dynamic command discovery waits for its **3s timeout**.
- This delays wrapper execution beyond the supervisor’s **2s ready-file deadline**, so the supervisor kills the wrapper and reports `wrapper did not report child start`.

This is *not* a “service binary missing” problem; the service binaries run fine when executed directly. It’s a wrapper initialization delay caused by dynamic plugin discovery.

## Background: the comprehensive fixture setup

The fixture repo is produced by:
- `devctl/ttmp/2026/01/06/MO-009-TUI-COMPLETE-FEATURES--complete-tui-features-per-mo-006-design/scripts/setup-comprehensive-fixture.sh`

It creates a repo root like `/tmp/devctl-comprehensive-XXXXXX` containing:
- `bin/http-echo`, `bin/log-spewer`, `bin/crash-after` (Go binaries)
- `plugins/comprehensive.py`, `plugins/logger.py`, `plugins/metrics.py`
- `.devctl.yaml` configuring the plugins

Example working directory used during investigation:
- `/tmp/devctl-comprehensive-eB3u5j`

## Symptom and reproduction

### Symptom

Running `devctl up` against the fixture fails during supervise:
```
Error: wrapper did not report child start
```

### Repro

From the `devctl` repo:
```bash
REPO_ROOT=/tmp/devctl-comprehensive-eB3u5j
go run ./cmd/devctl --repo-root "$REPO_ROOT" up
```

The pipeline executes plugin phases (build/prepare/validate/launch.plan), then fails right after “service started pid=…”.

Important detail: that PID is the **wrapper PID**, not the service’s PID.

## Code path walkthrough (with file references)

### Phase A: `devctl up` creates a launch plan, then calls supervisor

`devctl/cmd/devctl/cmds/up.go`:

Pseudocode:
```go
cfg := config.LoadOptional(...)
specs := discovery.Discover(cfg, RepoRoot)
clients := runtime.Factory.Start(...) // start plugins

plan := engine.Pipeline{clients}.LaunchPlan(...)

wrapperExe := os.Executable()
sup := supervise.New(Options{RepoRoot, ReadyTimeout: opts.Timeout, WrapperExe: wrapperExe})
st := sup.Start(ctx, plan) // <- failure happens here
```

### Phase B: supervisor starts each service via a wrapper process

`devctl/pkg/supervise/supervisor.go` → `startService(...)`

It builds wrapper args (notably `--stdout-log`, `--stderr-log`, `--exit-info`, `--ready-file`) and runs:
```go
cmd := exec.Command(s.opts.WrapperExe, "__wrap-service", ..., "--", <service command...>)
cmd.Dir = s.opts.RepoRoot
cmd.Env = os.Environ()
cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
cmd.Start()
```

Then it waits for the ready file:
```go
deadline := time.Now().Add(2 * time.Second) // hard-coded
for {
  if os.Stat(readyPath) == nil { break }
  if time.Now().After(deadline) {
    terminatePIDGroup(..., wrapperPID)
    return errors.New("wrapper did not report child start")
  }
  time.Sleep(10 * time.Millisecond)
}
```

Two critical properties of this design:

1) The “ready-file wait” is **hard-coded to 2 seconds**, independent of `Options.ReadyTimeout`.
2) The supervisor **does not capture** the wrapper’s stdout/stderr, so wrapper failures before it opens the per-service log files are invisible unless we instrument separately.

### Phase C: wrapper is a hidden cobra command within `devctl`

`devctl/cmd/devctl/cmds/wrap_service.go`

This command is responsible for:
- creating log dirs/files (stdout/stderr)
- starting the real service process (`exec.Command(args[0], args[1:]...)`)
- writing the ready file **immediately after `child.Start()`**
- waiting for child exit and writing exit info JSON

Pseudocode:
```go
mkdir log dirs
open stdout/stderr logs
child := exec.Command(serviceExe, args...)
child.Dir = cwd
child.Env = mergeEnv(os.Environ(), parseEnvPairs(...))
child.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pgid: os.Getpid()}

err := child.Start()
if err != nil {
  state.WriteExitInfo(exitInfoPath, {...Error: err...})
  return err
}

write readyFile(childPID)
waitErr := child.Wait()
write exitInfo(...)
```

Given this, if `__wrap-service` is actually reached, you should expect **at least**:
- `stdoutLog` + `stderrLog` files created (even if empty)
- `exitInfoPath` created if child start fails
- `readyFile` created on successful child start

During the failing `devctl up`, none of those per-service files appeared, which is a strong indicator that the wrapper never reached this code within the 2s window (or died before opening files).

## The real culprit: dynamic plugin command discovery runs before every command

`devctl/cmd/devctl/main.go` always does:
```go
cmds.AddDynamicPluginCommands(rootCmd, os.Args)
rootCmd.Execute()
```

`devctl/cmd/devctl/cmds/dynamic_commands.go`:

High-level flow:
```go
repoRoot, cfgPath := parseRepoArgs(os.Args)
cfg := config.LoadOptional(cfgPath)
specs := discovery.Discover(cfg, repoRoot)

for spec in specs {
  client := runtime.Factory.Start(spec) // starts plugin process
  client.Call("commands.list", ..., timeout=3s) // ALWAYS CALLED
  client.Close()
  // build cobra commands from response
}
```

Key issue: `commands.list` is called **unconditionally**, regardless of whether the plugin supports it.

### Why the comprehensive fixture triggers this

From `setup-comprehensive-fixture.sh`, the fixture config includes:

- `comprehensive` plugin: responds to unknown ops with `E_UNSUPPORTED` quickly
- `metrics` plugin: responds to any request with `ok: True` quickly
- `logger` plugin: reads stdin but **never responds** to requests:
  - file: `/tmp/devctl-comprehensive-eB3u5j/plugins/logger.py`
  - loop: `for line in sys.stdin: pass`

Therefore:

- `AddDynamicPluginCommands` starts `logger` via `python3 ... logger.py`
- it sends a `commands.list` request
- the plugin ignores it
- `client.Call()` blocks until the **3 second** timeout expires

This delay happens *inside the wrapper process* before Cobra dispatch to `__wrap-service`.

## Why this produces `wrapper did not report child start`

Putting the timeouts together:

- `dynamic_commands.go`: `commands.list` timeout = **3 seconds**
- `supervisor.go`: wrapper “ready-file deadline” = **2 seconds**

So the wrapper can spend up to ~3 seconds in dynamic command discovery, never reaching `__wrap-service` quickly enough.

Supervisor behavior:
1) Start wrapper
2) Wait 2 seconds for ready file
3) Kill wrapper process group
4) Return error `wrapper did not report child start`

This is exactly the observed behavior:
- supervisor prints “service started pid=…” (wrapper PID)
- then exits with `wrapper did not report child start`
- no per-service log files appear because wrapper never got to `wrap_service.go`’s “open stdout/stderr logs” stage

## Supporting experiments and observations

### 1) Service binaries are executable and run fine

Direct execution works:
```bash
/tmp/devctl-comprehensive-eB3u5j/bin/http-echo --port 18080
```

So the failure is not due to missing +x bits or invalid binaries.

### 2) `__wrap-service` works when run from the devctl repo (no fixture plugins loaded)

When invoked from the `devctl/` directory (where `.devctl.yaml` likely doesn’t exist or contains no blocking plugin), `__wrap-service` is fast enough and writes a ready file.

This is consistent with dynamic command discovery being the differentiator: when the wrapper is executed with `cmd.Dir` set to the fixture root (as supervisor does), it discovers fixture plugins and pays the 3s timeout.

### 3) Wrapper-as-supervisor starts with `cmd.Dir = repoRoot`

This detail matters: `startService` sets `cmd.Dir = s.opts.RepoRoot`, which points at the fixture root containing `.devctl.yaml`, so the wrapper’s early dynamic command discovery sees the plugins and blocks.

## Secondary/adjacent issues worth noting

### A) Supervisor’s wrapper-ready wait uses a hard-coded 2 seconds

`devctl/pkg/supervise/supervisor.go` uses:
```go
deadline := time.Now().Add(2 * time.Second)
```

Even though `Options` includes `ReadyTimeout`, it is used for health checks only, not wrapper readiness.

This makes the system very sensitive to any wrapper startup overhead (plugins, slow disk, cold caches).

### B) Wrapper startup is “non-minimal” and does extra work unrelated to supervising a service

Even if the `commands.list` issue is fixed, any future global startup logic added to `main.go` will also impact the wrapper unless explicitly excluded.

## Recommendations / fixes (ranked)

### 1) Skip dynamic plugin command discovery for internal wrapper invocations (best immediate fix)

In `devctl/cmd/devctl/main.go`, skip `AddDynamicPluginCommands` for `__wrap-service`:

Pseudocode:
```go
if len(os.Args) > 1 && os.Args[1] == "__wrap-service" {
  // do not start plugins / dynamic commands
} else {
  cmds.AddDynamicPluginCommands(...)
}
```

This ensures the wrapper reaches `wrap_service.go` quickly and makes startup time predictable.

### 2) In `AddDynamicPluginCommands`, only call `commands.list` if the plugin declares support (robust fix)

The runtime client exposes `SupportsOp(op string)` based on handshake `Capabilities.Ops`:
- `devctl/pkg/runtime/client.go:67` (`SupportsOp`)

So dynamic discovery should:
1) Start plugin
2) If `!client.SupportsOp("commands.list")`, skip calling it (close plugin).

This avoids waiting 3s on plugins like `logger.py` that don’t implement `commands.list`.

### 3) Make supervisor’s wrapper-ready timeout configurable (paper-cut fix)

Replace hard-coded 2s with something derived from options:
```go
deadline := time.Now().Add(s.opts.ReadyTimeout) // or a separate WrapperReadyTimeout
```

This reduces spurious failures but doesn’t address the underlying “wrapper does too much work” problem.

### 4) Fixture/plugin-side workaround (not preferred, but useful for test stability)

Update `plugins/logger.py` to respond with `E_UNSUPPORTED` for unknown ops so `commands.list` doesn’t hang.

This only helps the fixture and doesn’t fix real-world plugins that might behave similarly.

## “Where to start” for code review / future debugging

- `devctl/cmd/devctl/main.go`: unconditional `AddDynamicPluginCommands` call
- `devctl/cmd/devctl/cmds/dynamic_commands.go`: unconditional `commands.list` call with 3s timeout
- `devctl/pkg/runtime/client.go`: `SupportsOp()` exists and should be used by dynamic discovery
- `devctl/pkg/supervise/supervisor.go`: wrapper invocation + hard-coded 2s ready wait
- `devctl/cmd/devctl/cmds/wrap_service.go`: ready file write happens after `child.Start()`
- `devctl/ttmp/.../scripts/setup-comprehensive-fixture.sh`: fixture plugins, especially `plugins/logger.py`

## Open questions / “leave no stone unturned” checks

If fixing the above doesn’t fully resolve the issue, check:

1) Does the wrapper process in `startService` ever reach Cobra dispatch?
   - Add temporary logging around `AddDynamicPluginCommands` and inside `newWrapServiceCmd().RunE`.
2) Are we accidentally loading `.devctl.yaml` during wrapper startup when we shouldn’t?
   - Confirm wrapper `cmd.Dir` and `--repo-root` parsing behavior (`parseRepoArgs`).
3) Are there multiple `.devctl.yaml` sources being considered (e.g., `--config`)?
4) Is the wrapper killed by the supervisor before it can open log files (likely)?
   - Confirm by increasing wrapper-ready timeout and observing that logs then appear.
