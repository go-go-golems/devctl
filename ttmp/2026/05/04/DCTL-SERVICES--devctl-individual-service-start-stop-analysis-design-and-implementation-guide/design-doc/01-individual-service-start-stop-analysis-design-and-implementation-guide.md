---
doc_type: design-doc
title: Individual Service Start/Stop: Analysis, Design, and Implementation Guide
status: active
intent: long-term
topics:
  - devctl
  - services
  - supervisor
  - restart
  - architecture
created_at: 2026-05-04
updated_at: 2026-05-04
---

# Individual Service Start/Stop: Analysis, Design, and Implementation Guide

**Ticket:** DCTL-SERVICES
**Date:** 2026-05-04
**Audience:** New intern joining the team. This document will teach you everything you need to understand the problem, why it's tricky, and how to solve it.

---

## Table of Contents

1. [Executive Summary](#executive-summary)
2. [Problem Statement](#problem-statement)
3. [Current Architecture: Service Lifecycle](#current-architecture-service-lifecycle)
   - [The Full Lifecycle: From Plugin to Process](#the-full-lifecycle-from-plugin-to-process)
   - [Supervisor: Atomic Start/Stop](#supervisor-atomic-startstop)
   - [State File: All-or-Nothing](#state-file-all-or-nothing)
   - [Pipeline: Service Provenance Is Lost](#pipeline-service-provenance-is-lost)
   - [TUI: UI Exists, Backend Missing](#tui-ui-exists-backend-missing)
   - [Wrapper Process Model](#wrapper-process-model)
4. [The Core Problem: Why Individual Service Control Is Hard](#the-core-problem-why-individual-service-control-is-hard)
   - [Problem 1: Lost Provenance](#problem-1-lost-provenance)
   - [Problem 2: Config Mutation Correlation](#problem-2-config-mutation-correlation)
   - [Problem 3: Atomic State](#problem-3-atomic-state)
   - [Problem 4: No Single-Service Supervisor Operations](#problem-4-no-single-service-supervisor-operations)
   - [Problem 5: Health Checks and Dependencies](#problem-5-health-checks-and-dependencies)
5. [Gap Analysis](#gap-analysis)
6. [Proposed Architecture](#proposed-architecture)
   - [Approach: Simple Restart with Stored Specs](#approach-simple-restart-with-stored-specs)
   - [ServiceSpec Storage in State](#servicespec-storage-in-state)
   - [Supervisor Single-Service Operations](#supervisor-single-service-operations)
   - [New CLI Commands](#new-cli-commands)
   - [TUI Action Runner Implementation](#tui-action-runner-implementation)
   - [State File Partial Updates](#state-file-partial-updates)
7. [API References and Pseudocode](#api-references-and-pseudocode)
   - [Supervisor Changes](#supervisor-changes)
   - [State Changes](#state-changes)
   - [CLI Command: devctl restart](#cli-command-devctl-restart)
   - [CLI Command: devctl stop](#cli-command-devctl-stop)
   - [TUI Action Runner: ActionStop Implementation](#tui-action-runner-actionstop-implementation)
8. [Diagrams](#diagrams)
9. [Implementation Phases](#implementation-phases)
10. [Test Strategy](#test-strategy)
11. [Risks, Alternatives, and Open Questions](#risks-alternatives-and-open-questions)
12. [File Reference Index](#file-reference-index)

---

## 1. Executive Summary

devctl manages development environments by orchestrating plugins that declare services to run. Today, the system treats service management as **atomic**: `devctl up` starts everything, `devctl down` stops everything. There is no way to restart just the web server without also restarting the Vite dev server, the API mock, and every other managed service.

This is a significant UX problem for daily development. When a service crashes or needs a configuration change, developers must tear down the entire environment and rebuild it. The TUI already has `[s] stop` and `[r] restart` keys per service, but the backend returns `"stop action is not implemented"`.

**Why is this hard?** The pipeline that produces services involves multiple plugins collaborating through a shared config. Plugin A mutates the config, plugin B sees the mutated config and declares services based on it. If you want to "restart" a service, you can't just re-run one plugin — its output depended on what other plugins did before it. This is the **config mutation correlation problem**, and it's the central challenge this document analyzes.

**Implementation update (2026-05-05):** the final implementation uses a safer **re-plan-and-respawn** strategy. devctl does **not** persist raw service environment variables in `state.json`. Instead, `devctl start <service>` and `devctl restart <service>` re-run the two planning phases (`config.mutate` and `launch.plan`) across the configured plugins, select the named service from the fresh launch plan, and then start that service. This preserves the plugin config-mutation correlation while avoiding plaintext secret persistence. It does not run `build.run`, `prepare.run`, or `validate.run`.

**Key design decisions:**

- Keep only non-secret service metadata in `state.json`; do not store raw `ServiceSpec.Env`.
- Re-run `config.mutate + launch.plan` when starting or restarting an individual service.
- Add `StopService()`, `StartService(spec)`, and `RestartService(spec)` to the supervisor.
- Add `devctl start <service>`, `devctl restart <service>`, and `devctl stop-service <service>` CLI commands.
- Implement the TUI's existing `ActionStop` handler and per-service `ActionRestart` path.
- Block `devctl start <service>` when that service is already running, to avoid duplicate unmanaged processes.

---

## 2. Problem Statement

### The user experience today

Imagine you're developing a full-stack web application with devctl managing three services:

1. **Vite dev server** — hot-reloading frontend on port 3000.
2. **Go API server** — backend on port 8080.
3. **PostgreSQL** — database on port 5432.

The Go server crashes. You want to restart just that one process. Here's what happens:

```bash
$ devctl down    # Stops ALL three services
$ devctl up      # Starts ALL three services, re-runs entire pipeline
```

This takes 15-30 seconds, kills your database connections, clears the Vite hot-reload state, and requires waiting for health checks on all services. Just to restart one process.

### What the user wants

```bash
$ devctl restart api-server    # Restart just the Go API server
$ devctl stop postgres         # Stop just the database
$ devctl status                # Show that api-server is running, postgres is stopped
```

Or in the TUI: navigate to a service, press `[s]` to stop it, press `[r]` to restart it.

### Why this matters

- **Developer velocity.** Restarting one process should take 2 seconds, not 30.
- **Debugging.** Developers need to stop individual services to test failure scenarios.
- **Resource management.** Some services (databases, caches) don't need to restart when you're only changing frontend code.
- **Crash recovery.** When a service crashes, the other services shouldn't be affected.

---

## 3. Current Architecture: Service Lifecycle

To understand why this is hard, you need to understand exactly how services get from "a plugin's idea" to "a running process." Let's trace the full lifecycle.

### The Full Lifecycle: From Plugin to Process

```
Step 1: Plugin declares services
┌─────────────────────────────────────────────────────────────────┐
│ Plugin A (backend)                                              │
│   config.mutate → adds {services.api.port: 8080}               │
│   launch.plan  → returns ServiceSpec{name: "api",              │
│                     command: ["go", "run", "./cmd/api"]}       │
├─────────────────────────────────────────────────────────────────┤
│ Plugin B (frontend)                                             │
│   config.mutate → adds {services.web.port: 3000}               │
│   launch.plan  → returns ServiceSpec{name: "web",              │
│                     command: ["npm", "run", "dev"]}            │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
Step 2: Pipeline merges services
┌─────────────────────────────────────────────────────────────────┐
│ LaunchPlan {                                                    │
│   Services: [                                                   │
│     {name: "api", command: ["go", "run", "./cmd/api"]},         │
│     {name: "web", command: ["npm", "run", "dev"]},              │
│   ]                                                             │
│ }                                                               │
│                                                                 │
│ ⚠️ At this point, we've LOST which plugin produced which        │
│    service. The LaunchPlan is just a flat list.                  │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
Step 3: Supervisor starts processes
┌─────────────────────────────────────────────────────────────────┐
│ for _, svc := range plan.Services:                              │
│   start process → get PID                                       │
│   create log files                                              │
│                                                                 │
│ State {                                                         │
│   Services: [                                                   │
│     {name: "api", pid: 12345, stdout: "...", stderr: "..."},    │
│     {name: "web", pid: 12346, stdout: "...", stderr: "..."},    │
│   ]                                                             │
│ }                                                               │
│                                                                 │
│ ⚠️ ServiceRecord stores PID and log paths, but NOT the          │
│    original ServiceSpec (command, env, cwd).                    │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
Step 4: State saved to disk
┌─────────────────────────────────────────────────────────────────┐
│ .devctl/state.json:                                             │
│ {                                                               │
│   "repo_root": "/path/to/repo",                                 │
│   "services": [                                                 │
│     {"name": "api", "pid": 12345, "command": [...], ...},       │
│     {"name": "web", "pid": 12346, "command": [...], ...}        │
│   ]                                                             │
│ }                                                               │
└─────────────────────────────────────────────────────────────────┘
```

The critical observation: between Step 1 and Step 2, the connection between "which plugin produced this service" is lost. Between Step 3 and Step 4, the original `ServiceSpec` (with env, cwd, health check config) is partially preserved but scattered across `ServiceRecord` fields.

### Supervisor: Atomic Start/Stop

**File:** `devctl/pkg/supervise/supervisor.go`

The `Supervisor` has exactly two operations:

```go
func (s *Supervisor) Start(ctx context.Context, plan engine.LaunchPlan) (*state.State, error)
func (s *Supervisor) Stop(ctx context.Context, st *state.State) error
```

- `Start()` iterates ALL services in the plan, starts each one, then checks health for all of them. If any health check fails, it stops everything and returns an error.
- `Stop()` iterates ALL services in the state and sends SIGTERM to each process group.

There is no `StopService(name)` or `StartService(spec)`. The supervisor treats the service set as immutable — you get exactly the services in the plan, all at once.

### State File: All-or-Nothing

**File:** `devctl/pkg/state/state.go`

The state file records the running services:

```go
type State struct {
    RepoRoot  string          `json:"repo_root"`
    CreatedAt time.Time       `json:"created_at"`
    Services  []ServiceRecord `json:"services"`
}

type ServiceRecord struct {
    Name      string            `json:"name"`
    PID       int               `json:"pid"`
    Command   []string          `json:"command"`     // ✅ preserved
    Cwd       string            `json:"cwd"`         // ✅ preserved
    Env       map[string]string `json:"env"`         // ⚠️ sanitized (secrets redacted)
    StdoutLog string            `json:"stdout_log"`
    StderrLog string            `json:"stderr_log"`
    ExitInfo  string            `json:"exit_info,omitempty"`
    StartedAt time.Time         `json:"started_at,omitempty"`
    
    // Health check config (preserved)
    HealthType    string `json:"health_type,omitempty"`
    HealthAddress string `json:"health_address,omitempty"`
    HealthURL     string `json:"health_url,omitempty"`
}
```

**Key observation:** `Command`, `Cwd`, and `Env` ARE preserved in the state file. This means we have enough information to restart a service without re-running the pipeline. However, `Env` is sanitized — secrets are replaced with `[REDACTED]`. This is a problem for restart (see Risks section).

### Pipeline: Service Provenance Is Lost

**File:** `devctl/pkg/engine/pipeline.go`

The `LaunchPlan()` method merges services from all plugins:

```go
func (p *Pipeline) LaunchPlan(ctx context.Context, cfg patch.Config) (LaunchPlan, error) {
    ordered := clientsInOrder(p.Clients)
    var merged LaunchPlan
    seen := map[string]int{}
    for _, c := range ordered {
        if !c.SupportsOp("launch.plan") {
            continue
        }
        var out LaunchPlan
        if err := c.Call(ctx, "launch.plan", map[string]any{"config": cfg}, &out); err != nil {
            return LaunchPlan{}, err
        }
        for _, svc := range out.Services {
            if idx, ok := seen[svc.Name]; ok {
                if p.Opts.Strict {
                    return LaunchPlan{}, errors.Errorf("service name collision: %s", svc.Name)
                }
                merged.Services[idx] = svc  // later plugin overwrites
                continue
            }
            seen[svc.Name] = len(merged.Services)
            merged.Services = append(merged.Services, svc)
        }
    }
    return merged, nil
}
```

After this function returns, the `LaunchPlan` is a flat list of `ServiceSpec` objects. There's no way to know which plugin produced which service. If two plugins contribute a service with the same name, the later one silently overwrites the earlier one (unless strict mode).

### TUI: UI Exists, Backend Missing

**File:** `devctl/pkg/tui/action_runner.go`

The TUI's action runner has a handler for `ActionStop`:

```go
case ActionStop:
    if req.Service == "" {
        err = errors.New("missing service for stop action")
        break
    }
    err = errors.New("stop action is not implemented")
```

And the TUI's service detail view (`service_model.go`) already sends stop/restart actions:

```go
// Press 's' in service view
case "s":
    return m, func() tea.Msg {
        return tui.ActionRequestMsg{Request: tui.ActionRequest{
            Kind: tui.ActionStop, Service: m.name,
        }}
    }

// Press 'r' in service view
case "r":
    return m, func() tea.Msg {
        return tui.ActionRequest{Kind: tui.ActionRestart, Service: m.name}
    }
```

The UI is fully wired. The backend just returns "not implemented."

### Wrapper Process Model

**File:** `devctl/cmd/devctl/cmds/wrap_service.go`

When `WrapperExe` is set (which it is in normal operation), services are started via a wrapper process:

```
devctl __wrap-service \
  --service api-server \
  --cwd /path/to/repo \
  --stdout-log .devctl/logs/api-server-20260504-100000.stdout.log \
  --stderr-log .devctl/logs/api-server-20260504-100000.stderr.log \
  --exit-info .devctl/logs/api-server-20260504-100000.exit.json \
  --ready-file .devctl/logs/api-server-20260504-100000.ready \
  --env KEY=VAL \
  -- \
  go run ./cmd/api
```

The wrapper:
- Sets up a new process group (`setpgid`).
- Forwards signals to the child process group.
- Records exit info (exit code, signal, stderr tail) on child termination.

This means we can't just send SIGTERM to the recorded PID — we need to send it to the process group. The `terminatePIDGroup` function in the supervisor handles this correctly by getting the PGID and killing `-pgid`.

---

## 4. The Core Problem: Why Individual Service Control Is Hard

This section is the heart of the document. Understanding these five problems is essential for evaluating any proposed solution.

### Problem 1: Lost Provenance

After the pipeline merges services from all plugins, there's no record of which plugin produced which service. The `LaunchPlan.Services` list is flat — just `ServiceSpec` objects with no `plugin_id` field.

**Why it matters:** If you want to "restart" a service by re-running its originating plugin, you can't — you don't know which plugin to ask. You'd have to re-run all plugins.

**File evidence:**
- `engine/types.go`: `ServiceSpec` has `Name`, `Command`, `Env`, `Health` — but no `PluginID` field.
- `engine/pipeline.go`: `LaunchPlan()` merges into a flat list with no plugin metadata.

### Problem 2: Config Mutation Correlation

The pipeline runs `MutateConfig()` as a sequential chain:

```
Plugin A: config.mutate → adds {services.api.port: 8080, env: "production"}
Plugin B: config.mutate → reads services.api.port, adds {proxy.target: "http://localhost:8080"}
```

Plugin B's output depends on what Plugin A wrote to the config. If you re-run only Plugin B's `launch.plan`, it gets a fresh (empty) config — not the one that was the result of Plugin A's mutation. Its behavior would be different.

**Why it matters:** You cannot correctly re-run a single plugin's `launch.plan` in isolation. The config it would see is different from the config it saw during the original `up`. This means "restart by re-running the pipeline" must either run the full pipeline or accept potentially stale service specs.

### Problem 3: Atomic State

The state file (`state.json`) is written once after all services start. `state.Save()` writes the entire file; `state.Load()` reads the entire file. There's no mechanism for:
- Updating a single service's PID (after restart).
- Removing a single service (after stop).
- Adding a single service (after ad-hoc start).

**Why it matters:** Restarting one service requires updating its PID in the state file. Without partial updates, you'd have to rewrite the entire state, which is a race condition if the TUI is reading it simultaneously.

### Problem 4: No Single-Service Supervisor Operations

The `Supervisor` struct has exactly two methods:
- `Start(plan LaunchPlan)` — starts ALL services.
- `Stop(state *State)` — stops ALL services.

There is no `StopService(name string)`, `StartService(spec ServiceSpec)`, or `RestartService(name string)`.

**Why it matters:** Even if you know which service to restart and have its spec, you need supervisor support to do it safely (process group management, log file setup, health check waiting).

### Problem 5: Health Checks and Dependencies

Services may have health checks (TCP or HTTP). When starting a service, the supervisor waits for it to become healthy. If service A depends on service B being healthy (e.g., the web server needs the API server), restarting B while A is running could temporarily break A.

Currently there's no dependency graph between services — they're started in parallel and health-checked independently. This is fine for initial startup but means restart could cause transient failures in dependent services.

**Why it matters:** Restarting a database might temporarily break the API server that's trying to connect to it. The system should at least be aware of this and communicate it to the user.

---

## 5. Gap Analysis

| Gap | Current Code | What's Needed |
|-----|-------------|---------------|
| No single-service stop | `Supervisor.Stop()` kills all | `Supervisor.StopService(name)` |
| No single-service start | `Supervisor.Start()` starts all | Re-run planning to resolve `ServiceSpec`, then `Supervisor.StartService(spec)` |
| No restart operation | TUI restart = down+up all | Re-run planning to resolve `ServiceSpec`, then `Supervisor.RestartService(spec)` |
| No CLI commands | Only `up`, `down` | `devctl start <name>`, `devctl restart <name>`, `devctl stop-service <name>` |
| ActionStop not implemented | Returns error in `action_runner.go` | Implement using supervisor single-service ops |
| State has no partial update | `Save()` rewrites entire file | Update one service record then save state |
| Secret env recovery | Raw env should not be persisted | Re-run `config.mutate + launch.plan` to recover effective env in memory |

---

## 6. Proposed Architecture

### Approach: Re-plan and Respawn

The implemented approach is:

1. **Stop only the target process** using `StopService`.
2. **Re-run planning** (`config.mutate + launch.plan`) across all configured plugins.
3. **Select the named service** from the fresh launch plan.
4. **Start only that service** from the resolved in-memory `ServiceSpec`.

This preserves cross-plugin config mutation correlation because the full planning pipeline is re-run. It also avoids persisting plaintext secrets because raw service env only exists in memory during planning and process launch.

**Important boundary:** `devctl start` and `devctl restart` do not run `build.run`, `prepare.run`, or `validate.run`. Plugins should treat `config.mutate` and `launch.plan` as idempotent planning phases.

### Service Metadata Storage in State

The `ServiceRecord` keeps non-secret metadata for inspection/backward compatibility. Raw env is intentionally not stored:

```go
type ServiceRecord struct {
    // ... existing fields ...
    
    // Non-secret metadata only; start/restart re-run planning for effective env.
    Spec *ServiceSpecRecord `json:"spec,omitempty"`
}

type ServiceSpecRecord struct {
    Name    string             `json:"name"`
    Cwd     string             `json:"cwd,omitempty"`
    Command []string           `json:"command"`
    Health  *HealthCheckRecord `json:"health,omitempty"`
}
```

`ServiceRecord.Env` remains sanitized for display. `ServiceSpecRecord` has no `Env` field.

### Supervisor Single-Service Operations

The `Supervisor` gains three new methods:

```go
// StopService stops a single service by name.
// Returns an error if the service is not found in the state.
func (s *Supervisor) StopService(ctx context.Context, st *state.State, name string) error

// StartService starts a single service from a freshly resolved ServiceSpec.
// Updates the state with the new PID and log paths.
func (s *Supervisor) StartService(ctx context.Context, st *state.State, spec engine.ServiceSpec) error

// RestartService stops and then starts a single service from a freshly resolved ServiceSpec.
func (s *Supervisor) RestartService(ctx context.Context, st *state.State, spec engine.ServiceSpec) error
```

Each method:
1. Finds the service by name in the state.
2. Performs the operation (stop/start/both).
3. Updates the service record in the state (new PID, new log paths, new started_at).
4. Saves the updated state file.

For `StartService`, it also waits for health checks if the spec defines them (same as the full `Start` does).

### New CLI Commands

**`devctl restart <service-name>`**

```bash
$ devctl restart api-server
# Stops api-server process
# Re-runs config.mutate + launch.plan to recover api-server's effective ServiceSpec
# Starts only api-server and waits for health check if configured
# Updates state.json with the new PID
```

**`devctl stop-service <service-name>`**

```bash
$ devctl stop-service postgres
# Stops postgres process
# Updates state.json (marks service as stopped, clears PID)
# Does NOT remove the service record (so it can be restarted later)
```

**`devctl start <service-name>`**

```bash
$ devctl start postgres
# Re-runs config.mutate + launch.plan to recover postgres's effective ServiceSpec
# Starts only postgres
# Fails if postgres is already running
```

### TUI Action Runner Implementation

**File to modify:** `devctl/pkg/tui/action_runner.go`

Replace the stub `ActionStop` handler:

```go
case ActionStop:
    if req.Service == "" {
        err = errors.New("missing service for stop action")
        break
    }
    err = runStopService(ctx, opts, bus.Publisher, runID, req.Service)
```

And implement `ActionRestart` as stop + start (not down + up):

```go
case ActionRestart:
    if req.Service == "" {
        // If no specific service, restart all (current behavior)
        if err2 := runDown(ctx, opts, bus.Publisher, runID); err2 != nil {
            err = err2
            break
        }
        err = runUp(ctx, opts, bus.Publisher, runID)
    } else {
        // Restart just the named service
        err = runRestartService(ctx, opts, bus.Publisher, runID, req.Service)
    }
```

### State File Partial Updates

Add helper functions for updating individual services:

```go
// UpdateService updates a single service record in the state file.
func UpdateService(repoRoot string, name string, update func(*ServiceRecord)) error {
    st, err := Load(repoRoot)
    if err != nil {
        return err
    }
    for i := range st.Services {
        if st.Services[i].Name == name {
            update(&st.Services[i])
            return Save(repoRoot, st)
        }
    }
    return fmt.Errorf("service %q not found", name)
}
```

This is simple but effective. The state file is small (typically < 10 services), so reading and rewriting it is fast. Race conditions with the TUI are handled by the filesystem's atomic write behavior.

---

## 7. API References and Pseudocode

### Supervisor Changes

**File to modify:** `devctl/pkg/supervise/supervisor.go`

```go
// StopService stops a single service by name.
// It sends SIGTERM to the process group, waits for it to exit,
// and then updates the state file with the cleared PID.
func (s *Supervisor) StopService(ctx context.Context, st *state.State, name string) error {
    var svc *state.ServiceRecord
    for i := range st.Services {
        if st.Services[i].Name == name {
            svc = &st.Services[i]
            break
        }
    }
    if svc == nil {
        return fmt.Errorf("service %q not found in state", name)
    }
    if svc.PID <= 0 || !state.ProcessAlive(svc.PID) {
        // Already stopped — just clear the PID.
        svc.PID = 0
        return state.Save(s.opts.RepoRoot, st)
    }

    if err := terminatePIDGroup(ctx, svc.PID, s.opts.ShutdownTimeout); err != nil {
        return fmt.Errorf("failed to stop service %q: %w", name, err)
    }
    
    // Clear the PID but keep the record (for potential restart).
    svc.PID = 0
    return state.Save(s.opts.RepoRoot, st)
}

// StartService starts a single service from a freshly resolved ServiceSpec.
// It refuses to start if the current tracked PID is still alive, to avoid
// duplicate unmanaged processes.
func (s *Supervisor) StartService(ctx context.Context, st *state.State, spec engine.ServiceSpec) error {
    name := spec.Name
    rec := findServiceRecord(st, name)
    if rec == nil {
        return fmt.Errorf("service %q not found in state", name)
    }
    if rec.PID > 0 && state.ProcessAlive(rec.PID) {
        return fmt.Errorf("service %q is already running (pid %d)", name, rec.PID)
    }

    newRec, err := s.startService(ctx, spec)
    if err != nil {
        return fmt.Errorf("failed to start service %q: %w", name, err)
    }

    // Wait for health check if configured.
    if spec.Health != nil {
        readyCtx, cancel := context.WithTimeout(ctx, s.opts.ReadyTimeout)
        err := waitReady(readyCtx, spec)
        cancel()
        if err != nil {
            _ = terminatePIDGroup(context.Background(), newRec.PID, s.opts.ShutdownTimeout)
            return fmt.Errorf("service %q health check failed: %w", name, err)
        }
    }

    // Update the state record with new process info and non-secret metadata.
    rec.PID = newRec.PID
    rec.Command = newRec.Command
    rec.Cwd = newRec.Cwd
    rec.Env = newRec.Env
    rec.StdoutLog = newRec.StdoutLog
    rec.StderrLog = newRec.StderrLog
    rec.ExitInfo = ""
    rec.StartedAt = newRec.StartedAt
    rec.HealthType = newRec.HealthType
    rec.HealthAddress = newRec.HealthAddress
    rec.HealthURL = newRec.HealthURL
    rec.Spec = newRec.Spec

    return state.Save(s.opts.RepoRoot, st)
}

// RestartService stops and then starts a single service from a freshly resolved ServiceSpec.
func (s *Supervisor) RestartService(ctx context.Context, st *state.State, spec engine.ServiceSpec) error {
    if err := s.StopService(ctx, st, spec.Name); err != nil {
        return fmt.Errorf("stop failed: %w", err)
    }
    if err := s.StartService(ctx, st, spec); err != nil {
        return fmt.Errorf("start failed: %w", err)
    }
    return nil
}
```

### State Changes

**File to modify:** `devctl/pkg/state/state.go`

Add the new types and update ServiceRecord:

```go
type ServiceRecord struct {
    Name      string            `json:"name"`
    PID       int               `json:"pid"`
    Command   []string          `json:"command"`
    Cwd       string            `json:"cwd"`
    Env       map[string]string `json:"env,omitempty"`          // sanitized for display
    StdoutLog string            `json:"stdout_log"`
    StderrLog string            `json:"stderr_log"`
    ExitInfo  string            `json:"exit_info,omitempty"`
    StartedAt time.Time         `json:"started_at,omitempty"`
    
    // Health check config
    HealthType    string `json:"health_type,omitempty"`
    HealthAddress string `json:"health_address,omitempty"`
    HealthURL     string `json:"health_url,omitempty"`
    
    // NEW: Original spec for restart.
    Spec *ServiceSpecRecord `json:"spec,omitempty"`
}

type ServiceSpecRecord struct {
    Name    string             `json:"name"`
    Cwd     string             `json:"cwd,omitempty"`
    Command []string           `json:"command"`
    Health  *HealthCheckRecord `json:"health,omitempty"`
}

type HealthCheckRecord struct {
    Type      string `json:"type"`
    Address   string `json:"address,omitempty"`
    URL       string `json:"url,omitempty"`
    TimeoutMs int64  `json:"timeout_ms,omitempty"`
}
```

When building the ServiceRecord, store only non-secret metadata:

```go
rec := state.ServiceRecord{
    // ... existing fields ...
    Env: state.SanitizeEnv(svc.Env),
    Spec: &state.ServiceSpecRecord{
        Name:    svc.Name,
        Cwd:     cwd,
        Command: svc.Command,
        Health:  healthCheckToRecord(svc.Health),
    },
}
```

The effective raw env is recovered later by re-running planning.

### CLI Command: devctl restart

**New file:** `devctl/cmd/devctl/cmds/restart.go`

```go
func newRestartCmd() *cobra.Command {
    var noHealthCheck bool
    
    cmd := &cobra.Command{
        Use:   "restart <service-name>",
        Short: "Restart a single supervised service",
        Args:  cobra.ExactArgs(1),
        RunE: func(cmd *cobra.Command, args []string) error {
            serviceName := args[0]
            opts, err := getRootOptions(cmd)
            if err != nil {
                return err
            }
            
            st, err := state.Load(opts.RepoRoot)
            if err != nil {
                return err
            }
            
            // Check that the service exists in the state.
            found := false
            for _, svc := range st.Services {
                if svc.Name == serviceName {
                    found = true
                    break
                }
            }
            if !found {
                return fmt.Errorf("service %q not found in state", serviceName)
            }
            
            ctx, cancel := context.WithTimeout(cmd.Context(), opts.Timeout)
            defer cancel()

            spec, err := servicecontrol.ResolveServiceSpec(ctx, opts, serviceName)
            if err != nil {
                return err
            }
            
            wrapperExe, _ := os.Executable()
            sup := supervise.New(supervise.Options{
                RepoRoot:        opts.RepoRoot,
                ShutdownTimeout: opts.Timeout,
                ReadyTimeout:    opts.Timeout,
                WrapperExe:      wrapperExe,
            })
            
            if err := sup.RestartService(ctx, st, spec); err != nil {
                return err
            }
            
            log.Info().Str("service", serviceName).Msg("restarted")
            _, _ = fmt.Fprintln(cmd.OutOrStdout(), "ok")
            return nil
        },
    }
    
    cmd.Flags().BoolVar(&noHealthCheck, "no-health-check", false, "Skip health check after restart")
    AddRepoFlags(cmd)
    return cmd
}
```

### CLI Command: devctl stop

**New file:** `devctl/cmd/devctl/cmds/stop_service.go`

```go
func newStopServiceCmd() *cobra.Command {
    cmd := &cobra.Command{
        Use:   "stop <service-name>",
        Short: "Stop a single supervised service (leaves others running)",
        Args:  cobra.ExactArgs(1),
        RunE: func(cmd *cobra.Command, args []string) error {
            serviceName := args[0]
            opts, err := getRootOptions(cmd)
            if err != nil {
                return err
            }
            
            st, err := state.Load(opts.RepoRoot)
            if err != nil {
                return err
            }
            
            wrapperExe, _ := os.Executable()
            sup := supervise.New(supervise.Options{
                RepoRoot:        opts.RepoRoot,
                ShutdownTimeout: opts.Timeout,
                WrapperExe:      wrapperExe,
            })
            
            ctx, cancel := context.WithTimeout(cmd.Context(), opts.Timeout)
            defer cancel()
            
            if err := sup.StopService(ctx, st, serviceName); err != nil {
                return err
            }
            
            log.Info().Str("service", serviceName).Msg("stopped")
            _, _ = fmt.Fprintln(cmd.OutOrStdout(), "ok")
            return nil
        },
    }
    
    AddRepoFlags(cmd)
    return cmd
}
```

### TUI Action Runner: ActionStop Implementation

**File to modify:** `devctl/pkg/tui/action_runner.go`

```go
func runStopService(ctx context.Context, opts RootOptions, pub message.Publisher, runID string, serviceName string) error {
    if opts.RepoRoot == "" {
        return errors.New("missing RepoRoot")
    }
    if opts.Timeout <= 0 {
        opts.Timeout = 30 * time.Second
    }
    if opts.DryRun {
        return nil
    }
    
    st, err := state.Load(opts.RepoRoot)
    if err != nil {
        return err
    }
    
    found := false
    for _, svc := range st.Services {
        if svc.Name == serviceName {
            found = true
            break
        }
    }
    if !found {
        return fmt.Errorf("service %q not found", serviceName)
    }
    
    wrapperExe, _ := os.Executable()
    sup := supervise.New(supervise.Options{
        RepoRoot:        opts.RepoRoot,
        ShutdownTimeout: opts.Timeout,
        WrapperExe:      wrapperExe,
    })
    
    stopStart := time.Now()
    _ = publishPipelinePhaseStarted(pub, PipelinePhaseStarted{
        RunID: runID, Phase: PipelinePhaseStopSupervise, At: stopStart,
    })
    
    if err := sup.StopService(ctx, st, serviceName); err != nil {
        _ = publishPipelinePhaseFinished(pub, PipelinePhaseFinished{
            RunID: runID, Phase: PipelinePhaseStopSupervise,
            At: time.Now(), Ok: false,
            DurationMs: time.Since(stopStart).Milliseconds(),
            Error: err.Error(),
        })
        return err
    }
    
    _ = publishPipelinePhaseFinished(pub, PipelinePhaseFinished{
        RunID: runID, Phase: PipelinePhaseStopSupervise,
        At: time.Now(), Ok: true,
        DurationMs: time.Since(stopStart).Milliseconds(),
    })
    return nil
}

func runRestartService(ctx context.Context, opts RootOptions, pub message.Publisher, runID string, serviceName string) error {
    if err := runStopService(ctx, opts, pub, runID, serviceName); err != nil {
        return err
    }
    // Re-load state (stop updated the file)
    st, err := state.Load(opts.RepoRoot)
    if err != nil {
        return err
    }
    
    wrapperExe, _ := os.Executable()
    sup := supervise.New(supervise.Options{
        RepoRoot:     opts.RepoRoot,
        ReadyTimeout: opts.Timeout,
        WrapperExe:   wrapperExe,
    })
    
    supStart := time.Now()
    _ = publishPipelinePhaseStarted(pub, PipelinePhaseStarted{
        RunID: runID, Phase: PipelinePhaseSupervise, At: supStart,
    })
    
    if err := sup.StartService(ctx, st, serviceName); err != nil {
        _ = publishPipelinePhaseFinished(pub, PipelinePhaseFinished{
            RunID: runID, Phase: PipelinePhaseSupervise,
            At: time.Now(), Ok: false,
            DurationMs: time.Since(supStart).Milliseconds(),
            Error: err.Error(),
        })
        return err
    }
    
    _ = publishPipelinePhaseFinished(pub, PipelinePhaseFinished{
        RunID: runID, Phase: PipelinePhaseSupervise,
        At: time.Now(), Ok: true,
        DurationMs: time.Since(supStart).Milliseconds(),
    })
    return nil
}
```

---

## 8. Diagrams

### 8.1 Current Flow: All-or-Nothing

```
devctl up                          devctl down
──────────                         ──────────
┌──────────────┐                   ┌──────────────┐
│ Load plugins  │                   │ Load state    │
│ A, B, C       │                   │ .json         │
└──────┬───────┘                   └──────┬───────┘
       │                                  │
       ▼                                  ▼
┌──────────────┐                   ┌──────────────┐
│ Pipeline:     │                   │ For each      │
│ mutate config │                   │ service:      │
│ build         │                   │  SIGTERM      │
│ prepare       │                   │  process group│
│ validate      │                   └──────┬───────┘
│ launch plan   │                          │
└──────┬───────┘                          ▼
       │                          ┌──────────────┐
       ▼                          │ Remove        │
┌──────────────┐                  │ state.json    │
│ LaunchPlan:   │                 └──────────────┘
│  [api, web,   │
│   db]         │
└──────┬───────┘
       │
       ▼
┌──────────────┐
│ Supervisor:   │
│ start api     │
│ start web     │
│ start db      │
│ health check  │
│ all           │
└──────┬───────┘
       │
       ▼
┌──────────────┐
│ state.json:   │
│ {api, web, db}│
└──────────────┘
```

### 8.2 Proposed Flow: Single-Service Restart

```
devctl restart api-server
──────────────────────────
┌──────────────────────────┐
│ Load state.json           │
│ {api: {pid:123,spec:{..}},│
│  web: {pid:124,spec:{..}}}│
└────────────┬─────────────┘
             │
             ▼
┌──────────────────────────┐
│ Find service "api"        │
│ Spec: {command: ["go",    │
│   "run", "./cmd/api"],    │
│   env: {...}, cwd: "..."} │
└────────────┬─────────────┘
             │
             ▼
┌──────────────────────────┐
│ StopService("api")        │
│  SIGTERM → pid 123        │
│  Wait for exit            │
│  Clear pid in state       │
└────────────┬─────────────┘
             │
             ▼
┌──────────────────────────┐
│ StartService("api")       │
│  startService(spec)       │
│  new pid: 456             │
│  new log files            │
│  health check             │
│  Update state.json        │
└────────────┬─────────────┘
             │
             ▼
┌──────────────────────────┐
│ state.json updated:       │
│ {api: {pid:456,           │
│   log:new-path,...},      │
│  web: {pid:124,...}}      │
└──────────────────────────┘
```

### 8.3 State File With Spec Storage

```
.devctl/state.json
┌─────────────────────────────────────────────────────┐
│ {                                                   │
│   "repo_root": "/path/to/repo",                     │
│   "created_at": "2026-05-04T10:00:00Z",             │
│   "services": [                                     │
│     {                                               │
│       "name": "api-server",                         │
│       "pid": 12345,                                 │
│       "command": ["go", "run", "./cmd/api"],        │
│       "cwd": "/path/to/repo",                       │
│       "env": {"LOG_LEVEL": "[REDACTED]"},  ← display│
│       "stdout_log": ".devctl/logs/api-...stdout.log",│
│       "stderr_log": ".devctl/logs/api-...stderr.log",│
│       "spec": {                            ← NEW    │
│         "name": "api-server",                       │
│         "command": ["go", "run", "./cmd/api"],      │
│         "env": {"LOG_LEVEL": "debug"},     ← real   │
│         "health": {                                 │
│           "type": "http",                           │
│           "url": "http://localhost:8080/health"     │
│         }                                           │
│       }                                             │
│     },                                              │
│     {                                               │
│       "name": "web",                                │
│       "pid": 12346,                                 │
│       "spec": {                                     │
│         "command": ["npm", "run", "dev"],           │
│         "env": {"PORT": "3000"}                     │
│       }                                             │
│     }                                               │
│   ]                                                 │
│ }                                                   │
└─────────────────────────────────────────────────────┘
```

### 8.4 Service Lifecycle States

```
                    ┌─────────┐
                    │ PLANNED │  ← spec stored in state, no process
                    │ (pid=0) │
                    └────┬────┘
                         │ StartService()
                         ▼
                    ┌─────────┐
              ┌─────│ RUNNING │←──────────────┐
              │     │ (pid>0) │               │
              │     └────┬────┘               │
              │          │ crash/stop          │ RestartService()
              │          ▼                    │ (stop + start)
              │     ┌─────────┐               │
              │     │ STOPPED │───────────────┘
              │     │ (pid=0) │
              │     └────┬────┘
              │          │ StartService()
              │          ▼
              │     ┌─────────┐
              └─────│ RUNNING │
                    └─────────┘

States:
  RUNNING: pid > 0, process alive
  STOPPED: pid = 0, spec stored in state
  DEAD:    pid > 0, process not alive (crashed)
```

---

## 9. Implementation Phases

### Phase 1: ServiceSpec Storage in State

**Goal:** Store the original `ServiceSpec` in the state file so it's available for restart.

**Files to modify:**

| File | Change |
|------|--------|
| `pkg/state/state.go` | Add `ServiceSpecRecord`, `HealthCheckRecord` types; add `Spec` field to `ServiceRecord` |
| `pkg/supervise/supervisor.go` | Populate `Spec` when creating `ServiceRecord` in `startService()` |
| `cmd/devctl/cmds/up.go` | Pass ServiceSpec through to supervisor for storage |

**Validation:**
```bash
# After Phase 1:
devctl up
cat .devctl/state.json | jq '.services[0].spec'
# Should show the full ServiceSpec with command, env, health

go test ./pkg/state/... -v
go test ./pkg/supervise/... -v
```

### Phase 2: Supervisor Single-Service Operations

**Goal:** Add `StopService()`, `StartService()`, `RestartService()` to the supervisor.

**Files to modify:**

| File | Change |
|------|--------|
| `pkg/supervise/supervisor.go` | Add `StopService()`, `StartService()`, `RestartService()` methods |
| `pkg/supervise/supervisor_test.go` | Unit tests for single-service operations |

**Validation:**
```bash
go test ./pkg/supervise/... -v -run TestStopService
go test ./pkg/supervise/... -v -run TestRestartService
```

### Phase 3: CLI Commands

**Goal:** Add `devctl restart <service>` and `devctl stop <service>` commands.

**Files to create:**

| File | Change |
|------|--------|
| `cmd/devctl/cmds/restart.go` | New file with `devctl restart` command |
| `cmd/devctl/cmds/stop_service.go` | New file with `devctl stop` command |
| `cmd/devctl/cmds/root.go` | Register new commands |

**Validation:**
```bash
# Start services
devctl up

# Restart a single service
devctl restart api-server
devctl status  # api-server has new PID

# Stop a single service
devctl stop web
devctl status  # web shows pid=0

# Clean up
devctl down
```

### Phase 4: TUI Action Runner Implementation

**Goal:** Implement the TUI's `ActionStop` and update `ActionRestart` for single services.

**Files to modify:**

| File | Change |
|------|--------|
| `pkg/tui/action_runner.go` | Implement `runStopService()`, `runRestartService()`; replace stub |

**Validation:**
```bash
# Start TUI
devctl tui

# Navigate to a service, press 's' to stop
# Verify the service stops but others remain running
# Press 'r' to restart
# Verify the service restarts with a new PID
```

### Phase 5: Integration Tests

**Goal:** End-to-end tests for individual service operations.

**Files to create:**

| File | Change |
|------|--------|
| `cmd/devctl/cmds/dev/smoketest/service_lifecycle.go` | Smoke tests for restart/stop |
| `pkg/supervise/supervisor_test.go` | Integration tests with real processes |

**Validation:**
```bash
go test ./... -v -count=1
```

---

## 10. Test Strategy

### Unit Tests

| Component | Test | What to verify |
|-----------|------|----------------|
| `state.ServiceRecord` | Spec round-trip | Spec stored in JSON → loaded back correctly |
| `state.ServiceRecord` | Backward compat | Old state.json without spec field loads successfully |
| `supervisor.StopService` | Happy path | Process is killed, PID cleared in state |
| `supervisor.StopService` | Not found | Error returned for unknown service name |
| `supervisor.StopService` | Already stopped | No error, PID remains 0 |
| `supervisor.StartService` | Happy path | New process started, health check passes, state updated |
| `supervisor.StartService` | No spec | Error returned if spec is nil |
| `supervisor.RestartService` | Happy path | Stop + start, new PID in state |

### Integration Tests

Use test apps from `testapps/`:

1. Start services using `http-echo` and `log-spewer`.
2. Restart one service, verify the other is unaffected.
3. Stop one service, verify it's gone from status but state file still has it.
4. Restart a stopped service, verify it comes back.

### Backward Compatibility Tests

1. Load old `state.json` (no `spec` field) → services load, but restart returns "no spec stored" error.
2. `devctl up` with old config → new state.json has spec field populated.

---

## 11. Risks, Alternatives, and Open Questions

### Risks

1. **Planning side effects during start/restart.** The implemented approach re-runs `config.mutate + launch.plan` for `devctl start` and `devctl restart`. Plugins should treat these phases as planning phases and avoid destructive side effects there. Side-effectful work belongs in `build.run`, `prepare.run`, service commands, or explicit plugin commands.

   **Mitigation:** Document the behavior clearly in CLI help and this design doc. This tradeoff avoids persisting raw env secrets and preserves cross-plugin config mutation correlation.

2. **Fresh specs may differ from initial `up`.** Because start/restart now re-run planning, a service can restart with a new command/env if `.devctl.yaml`, plugin code, or environment variables changed since the original `up`.

   **Mitigation:** This is usually desirable for developer workflows. If exact reproducibility is needed, that would require a future sealed-plan artifact with a separate secret-handling design.

3. **Race conditions with state file.** The TUI polls `state.json` for status updates. If a restart is in progress, the TUI might read a partially-written state file.

   **Mitigation:** `state.Save()` writes to a temp file and renames (atomic on most filesystems). Alternatively, add a write-and-rename approach to `Save()`.

4. **Wrapper process complexity.** The wrapper process model adds complexity to restart — we need to start a new wrapper, not just the child command.

   **Mitigation:** The `startService()` method already handles both wrapper and non-wrapper modes. `StartService()` reuses this logic.

### Alternatives Considered

**Alternative 1: Full pipeline re-execution for restart.**

Re-run `config.mutate → launch.plan` for the service's originating plugin, then replace just that service.

- **Pros:** Always gets the freshest service spec.
- **Cons:** Requires provenance tracking (which plugin produced which service). Can't correctly re-run one plugin in isolation due to config mutation correlation. Very complex.
- **Decision:** Partially adopted. The implementation re-runs the full planning phases (`config.mutate + launch.plan`) across all configured plugins, then selects the named service. It does not attempt to re-run only the originating plugin.

**Alternative 2: Service dependency graph.**

Build a DAG of service dependencies and use it to determine stop/start order.

- **Pros:** Handles cases where service A depends on service B.
- **Cons:** devctl currently has no dependency concept. Adding one is a major feature.
- **Decision:** Rejected for v1. Document that restart may cause transient failures in dependent services.

**Alternative 3: Just implement TUI stop (SIGTERM to PID).**

Skip storing specs — just allow killing individual processes from the TUI. No restart capability.

- **Pros:** Simplest possible implementation.
- **Cons:** Can't restart after stop. Services that crash can't be recovered.
- **Decision:** Rejected. Users need restart capability, not just stop.

### Open Questions

1. **Should stopped services show in `devctl status`?** When a service is stopped (pid=0, spec still in state), should `devctl status` show it as "stopped" or hide it?

   **Recommendation:** Show it with status "stopped" and the spec, so the user knows they can restart it.

2. **Should `devctl down` remove stopped services' specs?** Currently `devctl down` removes the entire state file. Should we preserve any information?

   **Recommendation:** No. `devctl down` removes everything, same as today.

3. **Should planning phases be allowed to mutate external state?** Individual start/restart now re-run `config.mutate + launch.plan`, so plugins with side effects in those phases will repeat those effects.

   **Recommendation:** Treat planning phases as pure/idempotent. If plugins need side effects, use `build.run`, `prepare.run`, service commands, or explicit commands.

---

## 12. File Reference Index

All file paths relative to workspace root `/home/manuel/workspaces/2026-05-04/devctl-multiple-profiles/`.

### Files to modify

| File | Purpose | Changes |
|------|---------|---------|
| `devctl/pkg/supervise/supervisor.go` | Service lifecycle | Add `StopService()`, `StartService(spec)`, `RestartService(spec)`; block duplicate starts |
| `devctl/pkg/servicecontrol/resolve.go` | Service planning | Re-run `config.mutate + launch.plan` and select a named service |
| `devctl/pkg/state/state.go` | State persistence | Add non-secret `ServiceSpecRecord`, `HealthCheckRecord`; add `Spec` field to `ServiceRecord` |
| `devctl/cmd/devctl/cmds/up.go` | Up command | Pass ServiceSpec for storage in state |
| `devctl/cmd/devctl/cmds/root.go` | Root command | Register `restart` and `stop` subcommands |
| `devctl/pkg/tui/action_runner.go` | TUI action handler | Implement `ActionStop` and per-service `ActionRestart` |

### Files to create

| File | Purpose |
|------|---------|
| `devctl/cmd/devctl/cmds/restart.go` | `devctl restart <service>` command |
| `devctl/cmd/devctl/cmds/stop_service.go` | `devctl stop <service>` command |

### Key reference files (read-only)

| File | What it teaches |
|------|-----------------|
| `devctl/pkg/supervise/supervisor.go` | How services are started/stopped (atomic model) |
| `devctl/pkg/state/state.go` | State file structure and persistence |
| `devctl/pkg/state/exit_info.go` | Exit info recording (used by wrapper) |
| `devctl/pkg/engine/pipeline.go` | How launch plans are merged (provenance loss) |
| `devctl/pkg/engine/types.go` | ServiceSpec, LaunchPlan types |
| `devctl/pkg/tui/action_runner.go` | TUI action dispatch (ActionStop stub, RestartAll) |
| `devctl/pkg/tui/actions.go` | Action types (ActionStop, ActionRestart, ActionDown) |
| `devctl/pkg/tui/models/service_model.go` | Service detail view with [s]top/[r]estart keys |
| `devctl/pkg/tui/models/dashboard_model.go` | Dashboard with service list and restart confirmation |
| `devctl/pkg/tui/models/root_model.go` | Root TUI model, action dispatch to publisher |
| `devctl/cmd/devctl/cmds/wrap_service.go` | Wrapper process model |
| `devctl/cmd/devctl/cmds/down.go` | Down command (stops all) |
| `devctl/cmd/devctl/cmds/status.go` | Status command (reads state, checks alive) |
| `devctl/cmd/devctl/cmds/logs.go` | Logs command (per-service log tailing) |
| `devctl/cmd/devctl/cmds/common.go` | Shared CLI flags and RepoContext |
| `devctl/pkg/config/config.go` | Config file model |
| `devctl/pkg/discovery/discovery.go` | Plugin discovery |
| `devctl/pkg/repository/repository.go` | Repository loading |
| `devctl/pkg/runtime/client.go` | Plugin client protocol |
| `devctl/pkg/runtime/factory.go` | Plugin process lifecycle |
| `devctl/pkg/protocol/types.go` | Protocol frame types |
| `devctl/pkg/patch/patch.go` | Config patching (mutation chain) |
