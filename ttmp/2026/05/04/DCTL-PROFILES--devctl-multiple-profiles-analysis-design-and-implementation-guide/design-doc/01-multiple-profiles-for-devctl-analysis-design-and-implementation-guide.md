---
doc_type: design-doc
title: Multiple Profiles for devctl: Analysis, Design, and Implementation Guide
status: active
intent: long-term
topics:
  - devctl
  - profiles
  - architecture
  - pinocchio
  - plugins
created_at: 2026-05-04
updated_at: 2026-05-04
---

# Multiple Profiles for devctl: Analysis, Design, and Implementation Guide

**Ticket:** DCTL-PROFILES
**Date:** 2026-05-04
**Audience:** New intern joining the team — expected to be unfamiliar with devctl, pinocchio, or the go-go-golems ecosystem. This document will teach you everything you need.

---

## Table of Contents

1. [Executive Summary](#executive-summary)
2. [Problem Statement](#problem-statement)
3. [Current-State Architecture: devctl](#current-state-architecture-devctl)
   - [Overview: What devctl Does](#overview-what-devctl-does)
   - [Configuration Model](#configuration-model)
   - [Plugin Discovery](#plugin-discovery)
   - [Plugin Lifecycle](#plugin-lifecycle)
   - [Pipeline Operations](#pipeline-operations)
   - [NDJSON Protocol v2](#ndjson-protocol-v2)
   - [CLI Command Structure](#cli-command-structure)
   - [State Management](#state-management)
   - [Limitations of the Current Model](#limitations-of-the-current-model)
4. [Reference Architecture: Pinocchio Profiles](#reference-architecture-pinocchio-profiles)
   - [Overview: Why Pinocchio Matters Here](#overview-why-pinocchio-matters-here)
   - [Config Document Model](#config-document-model)
   - [Profile Types: Inline vs Registry](#profile-types-inline-vs-registry)
   - [Stack Composition and Merging](#stack-composition-and-merging)
   - [Profile Resolution Chain](#profile-resolution-chain)
   - [Per-Request Profile Selection (web-chat)](#per-request-profile-selection-web-chat)
   - [Lessons for devctl](#lessons-for-devctl)
5. [Gap Analysis](#gap-analysis)
6. [Proposed Architecture](#proposed-architecture)
   - [Profile Config Model](#profile-config-model)
   - [Local Override File](#local-override-file)
   - [Profile Selection Flow](#profile-selection-flow)
   - [Plugin Filtering by Profile](#plugin-filtering-by-profile)
   - [Profile-Aware Pipeline](#profile-aware-pipeline)
   - [CLI Changes](#cli-changes)
   - [State File Changes](#state-file-changes)
   - [Dynamic Commands and Profiles](#dynamic-commands-and-profiles)
7. [API References and Pseudocode](#api-references-and-pseudocode)
   - [Config File Schema](#config-file-schema)
   - [Profile Resolver Pseudocode](#profile-resolver-pseudocode)
   - [Repository Loader Changes](#repository-loader-changes)
   - [Pipeline Changes](#pipeline-changes)
8. [Diagrams](#diagrams)
9. [Implementation Phases](#implementation-phases)
10. [Test Strategy](#test-strategy)
11. [Risks, Alternatives, and Open Questions](#risks-alternatives-and-open-questions)
12. [File Reference Index](#file-reference-index)

---

## 1. Executive Summary

devctl is a dev-environment orchestrator that loads plugins from `.devctl.yaml`, starts them as child processes speaking a JSON-over-stdio protocol, and orchestrates them through a pipeline (config mutation → build → prepare → validate → launch plan → supervise). Today, every plugin listed in the config runs on every `devctl up` — there is no way to say "run only the production services" or "start in developer mode with hot-reload."

This document proposes adding **profiles** to devctl. A profile is a named configuration that selects a subset of plugins (and optionally overrides plugin args/env). The user selects a profile via `--profile production`, a `profile.active` field in `.devctl.yaml`, or a local `.devctl.override.yaml` that is stacked on top of the project config. If no profile is specified, all plugins load as before (backward compatible).

The design draws heavily from pinocchio's profile system — a mature implementation with stack composition, layered config merging, and per-request profile selection. We extract the relevant patterns (named profiles, filtering, active selection, and local override stacking) while keeping devctl's profile model simpler: no stack inheritance, no SQLite-backed stores, no external registries, no per-request switching.

**Implementation status (2026-05-05):** The v1 profile system described here has been implemented through config loading, repository filtering, CLI plumbing, state recording, profile commands, and tests. The implementation intentionally keeps the scope to inline profiles plus the optional local `.devctl.override.yaml` stack.

**Key design decisions:**

- Profiles are defined inline in `.devctl.yaml` under a `profiles:` block.
- A local `.devctl.override.yaml`, if present, is loaded after `.devctl.yaml` and can add personal profiles or adjust existing profile fields without changing the shared project config.
- Each profile specifies `plugins:` (a list of plugin IDs to activate) and optional `env:` overrides.
- A `profile.active` field or `--profile` flag selects the active profile; `--profile` remains the highest-precedence selection mechanism.
- A profile named `default` is allowed, but it is not implicit. It is used only when explicitly selected with `profile.active: default` or `--profile default`.
- When no profile is active, all top-level plugins load exactly as they do today (backward compatible).
- The plugin `priority` and `id` system already in devctl is reused for filtering.
- State files (`state.json`) record which profile was active so `devctl down` can clean up correctly.

---

## 2. Problem Statement

### The core problem

devctl loads and runs **every plugin** listed in `.devctl.yaml` every time you run `devctl up`. There is no mechanism to:

1. **Switch between modes.** A developer might want a "development" profile (with hot-reload, verbose logging, mock services) versus a "production" profile (with optimized builds, real credentials, minimal logging).
2. **Select service subsets.** A microservice repository might have 10 services, but a developer only wants to run 3 of them for their current task.
3. **Override plugin configuration per-mode.** The same plugin might need different arguments or environment variables depending on whether you're in dev or prod mode.

### Why this matters now

As repositories grow and more plugins are added, the "all-or-nothing" model becomes painful:

- `devctl up` takes longer because every plugin starts.
- Developers waste resources running services they don't need.
- There's no way to test production-like configurations without manually editing `.devctl.yaml`.
- Teams can't share standardized profiles (e.g., "frontend-only", "backend-only", "full-stack").

### What success looks like

A developer can:

```bash
# Run only the services needed for frontend work
devctl up --profile frontend

# Switch to full-stack mode
devctl up --profile full-stack

# List available profiles
devctl profiles list

# See which profile is active
devctl profiles active
```

And in the shared `.devctl.yaml`:

```yaml
profile:
  active: development

profiles:
  development:
    display_name: Development
    description: Hot-reload, verbose logging, mock services
    plugins:
      - web-server
      - mock-api
    env:
      LOG_LEVEL: debug
  production:
    display_name: Production
    description: Optimized builds, real services
    plugins:
      - web-server
      - api-server
      - database
      - cache
    env:
      LOG_LEVEL: warn

plugins:
  - id: web-server
    path: ./plugins/devctl-web-server
  - id: mock-api
    path: ./plugins/devctl-mock-api
  - id: api-server
    path: ./plugins/devctl-api-server
  - id: database
    path: ./plugins/devctl-database
  - id: cache
    path: ./plugins/devctl-cache
```

A developer can then keep personal choices in `.devctl.override.yaml` without editing the shared file:

```yaml
profile:
  active: manuel-debug

profiles:
  manuel-debug:
    display_name: Manuel Debug
    description: Backend plus local tracing settings
    plugins:
      - api-server
      - database
    env:
      LOG_LEVEL: trace
      DEVCTL_TRACE: "1"

  development:
    env:
      LOG_LEVEL: trace
```

The override file is intentionally local and simple. It is not a new profile registry. It is just a second YAML document loaded after `.devctl.yaml` with deterministic merge rules.

---

## 3. Current-State Architecture: devctl

This section teaches you everything about how devctl works today. Read it carefully — you'll need this mental model to understand where profiles fit.

### Overview: What devctl Does

devctl is a CLI tool that manages a development environment for a repository. Its job is to:

1. **Load plugins** — external programs that speak a defined JSON protocol over stdin/stdout.
2. **Orchestrate them through a pipeline** — each plugin can mutate config, validate, build, prepare, and declare services to run.
3. **Launch services** — start the declared services as supervised child processes.
4. **Supervise** — monitor processes, capture logs, allow status queries.

The key insight is that devctl itself knows almost nothing about your specific project. All the domain logic lives in **plugins**. devctl is a thin orchestrator that:
- Starts plugin processes.
- Sends them JSON requests.
- Collects and merges their responses.
- Launches the resulting services.

### Configuration Model

**File:** `devctl/pkg/config/config.go`

The configuration is a single YAML file called `.devctl.yaml` in the repository root. Its structure is minimal:

```go
type File struct {
    Plugins    []Plugin `yaml:"plugins"`
    Strictness string   `yaml:"strictness,omitempty"` // "warn" | "error"
}

type Plugin struct {
    ID       string            `yaml:"id"`
    Path     string            `yaml:"path"`
    Args     []string          `yaml:"args,omitempty"`
    Priority int               `yaml:"priority,omitempty"`
    WorkDir  string            `yaml:"workdir,omitempty"`
    Env      map[string]string `yaml:"env,omitempty"`
}
```

A concrete example:

```yaml
# .devctl.yaml
strictness: warn

plugins:
  - id: backend
    path: ./plugins/devctl-backend
    priority: 10
  - id: frontend
    path: ./plugins/devctl-frontend
    env:
      PORT: "3000"
```

**Key points:**
- The `id` field is required and must be unique. It's used for ordering, collision detection, and dynamic commands.
- The `path` field can be absolute, relative to repo root, or just a binary name (no path separator).
- `priority` defaults to 0; lower numbers run first in the pipeline.
- `strictness` controls whether name collisions across plugins are errors or warnings.

**Loading path:** `config.LoadOptional(path)` returns an empty `File{}` if the file doesn't exist (not an error). `config.DefaultPath(repoRoot)` returns `<repoRoot>/.devctl.yaml`.

### Plugin Discovery

**File:** `devctl/pkg/discovery/discovery.go`

Plugin discovery converts the config file into a list of `PluginSpec` structs (the runtime representation). There are two sources:

1. **Explicit plugins** from `.devctl.yaml` `plugins:` list.
2. **Auto-discovered plugins** from a `plugins/` directory in the repo root.

For auto-discovery, any executable file in `<repoRoot>/plugins/` whose name starts with `devctl-` is automatically registered as a plugin. The ID is derived by stripping the `devctl-` prefix. Auto-discovered plugins get priority 1000 (so they run after explicit ones).

```go
func Discover(cfg *config.File, opts Options) ([]runtime.PluginSpec, error)
```

The function:
- Iterates `cfg.Plugins`, converting each `config.Plugin` to `runtime.PluginSpec`.
- Validates: each plugin must have a non-empty `id` and `path`; no duplicate IDs.
- Scans `plugins/` directory for executables matching `devctl-*`.
- Deduplicates: explicit plugins take precedence over auto-discovered ones with the same ID.
- Sorts the final list by priority (ascending), then by ID (alphabetical) for stable ordering.

### Plugin Lifecycle

**File:** `devctl/pkg/runtime/factory.go`

The `Factory.Start()` method manages the full plugin lifecycle:

1. **Start the process:** `exec.CommandContext(ctx, spec.Path, spec.Args...)` with the plugin's working directory and merged environment.
2. **Capture stdio:** stdin pipe (for writing JSON frames), stdout pipe (for reading), stderr pipe (for logging).
3. **Read handshake:** Wait up to `HandshakeTimeout` (default 2s) for the first line on stdout. This must be a valid JSON handshake frame.
4. **Create client:** Wrap the process, handshake, and pipes into a `runtime.Client`.
5. **Start background goroutines:** One for reading stdout (dispatching responses and events), one for reading stderr (logging).

```go
type Factory struct {
    opts FactoryOptions  // HandshakeTimeout, ShutdownTimeout
}

func (f *Factory) Start(ctx context.Context, spec PluginSpec, opts StartOptions) (Client, error)
```

**Shutdown:** The client's `Close()` method sends SIGTERM to the process group, waits up to `ShutdownTimeout`, then SIGKILL if needed.

### Pipeline Operations

**File:** `devctl/pkg/engine/pipeline.go`

The `Pipeline` is the heart of devctl's orchestration. It takes an ordered list of `Client` objects and calls operations on them in sequence:

```
┌──────────┐   ┌──────────┐   ┌──────────┐
│ Plugin A │   │ Plugin B │   │ Plugin C │
│ prio: 10 │──▶│ prio: 20 │──▶│ prio: 30 │
└──────────┘   └──────────┘   └──────────┘
```

The pipeline supports these operations:

| Operation        | Method          | Description                                              |
|-----------------|-----------------|----------------------------------------------------------|
| Config mutation  | `MutateConfig`  | Each plugin patches a shared config map                  |
| Build            | `Build`         | Each plugin runs build steps (compile, bundle, etc.)     |
| Prepare          | `Prepare`       | Each plugin runs prepare steps (generate config, etc.)   |
| Validate         | `Validate`      | Each plugin validates the config; returns errors/warnings |
| Launch plan      | `LaunchPlan`    | Each plugin declares services to run                     |

**Key behavior:**
- Plugins that don't support an operation are skipped (checked via `client.SupportsOp(op)`).
- For `MutateConfig`, each plugin receives the config as modified by all previous plugins (sequential mutation chain).
- For `LaunchPlan`, services from all plugins are merged. If two plugins declare a service with the same name, the later one wins (unless strict mode).
- For `Build` and `Prepare`, steps and artifacts are merged similarly.

### NDJSON Protocol v2

**File:** `devctl/pkg/protocol/types.go`

Plugins communicate with devctl over stdin/stdout using newline-delimited JSON (NDJSON). There are four frame types:

```
┌─────────────────────────────────────────────────┐
│ Frame Types                                      │
├────────────┬────────────────────────────────────┤
│ handshake  │ Plugin → devctl (first line)       │
│ request    │ devctl → Plugin                    │
│ response   │ Plugin → devctl                    │
│ event      │ Plugin → devctl (streamed)         │
└────────────┴────────────────────────────────────┘
```

**Handshake** (sent by plugin on startup):
```json
{
  "type": "handshake",
  "protocol_version": "v2",
  "plugin_name": "my-plugin",
  "capabilities": {
    "ops": ["config.mutate", "launch.plan", "command.run"],
    "streams": ["logs.stream"],
    "commands": [{"name": "hello", "help": "Say hello"}]
  }
}
```

**Request** (sent by devctl):
```json
{
  "type": "request",
  "request_id": "my-plugin-1",
  "op": "launch.plan",
  "ctx": {"repo_root": "/path/to/repo", "dry_run": false},
  "input": {"config": {}}
}
```

**Response** (sent by plugin):
```json
{
  "type": "response",
  "request_id": "my-plugin-1",
  "ok": true,
  "output": {"services": [...]}
}
```

**Key rules:**
- `protocol_version` must be `"v2"`.
- `capabilities.ops` declares which operations the plugin supports.
- `capabilities.commands` declares dynamic CLI commands the plugin provides.
- Any non-JSON output on stdout is a protocol violation and kills the plugin.
- Stderr is captured and logged but does not affect the protocol.

### CLI Command Structure

**File:** `devctl/cmd/devctl/main.go`, `devctl/cmd/devctl/cmds/root.go`

The CLI is built with Cobra. The main commands are:

```
devctl
├── up          # Start the dev environment (full pipeline + supervise)
├── down        # Stop the dev environment
├── status      # Show running services
├── logs        # Tail service logs
├── stream      # Stream events from plugins
├── tui         # Interactive terminal UI
├── plan        # Show launch plan without starting
├── plugins     # Plugin discovery and inspection
│   └── list    # List configured plugins and capabilities
├── <dynamic>   # Commands discovered from plugin handshakes
└── dev         # Development and testing commands
    └── smoketest  # Smoke tests
```

**Common flags** (defined in `cmds/common.go`):
- `--repo-root` — Repository root directory (defaults to cwd).
- `--config` — Path to config file (defaults to `.devctl.yaml` under repo-root).
- `--strict` — Treat merge collisions as errors.
- `--dry-run` — Don't perform destructive side effects.
- `--timeout` — Default timeout for plugin operations (default 30s).

**Important: dynamic commands.** At startup, `AddDynamicPluginCommands()` starts every plugin, reads its handshake, and registers any declared commands as Cobra subcommands. This means the CLI's command tree depends on which plugins are configured.

### State Management

**File:** `devctl/pkg/state/state.go`

When `devctl up` completes, it writes a state file to `.devctl/state.json`:

```json
{
  "repo_root": "/path/to/repo",
  "created_at": "2026-05-04T10:00:00Z",
  "services": [
    {
      "name": "web-server",
      "pid": 12345,
      "command": ["bash", "-lc", "echo demo && sleep 3600"],
      "stdout_log": ".devctl/logs/web-server.stdout.log",
      "stderr_log": ".devctl/logs/web-server.stderr.log"
    }
  ]
}
```

`devctl down` reads this file and sends SIGTERM to each process. `devctl status` reads it and checks if processes are alive (using `kill(pid, 0)` and checking for zombie state).

**Key point for profiles:** The state file currently does not record which profile was active. After profiles are added, it must record the profile name so that `devctl down` can verify the correct services are being stopped.

### Limitations of the Current Model

Now that you understand the architecture, here are the specific gaps that profiles would fill:

1. **No filtering mechanism.** `discovery.Discover()` returns ALL plugins. There's no way to load only a subset.
2. **No mode concept.** Every `devctl up` runs the same set of plugins in the same way.
3. **No per-mode overrides.** Plugin `env` and `args` are static in the config.
4. **State is profile-blind.** The state file doesn't record context about why services were started.
5. **Dynamic commands are all-or-nothing.** Every plugin's commands appear in the CLI regardless of whether they're relevant to the current workflow.
6. **No way to share configurations.** Teams can't define named configurations that encode standard setups.


---

## 4. Reference Architecture: Pinocchio Profiles

Pinocchio is an LLM application runner in the same go-go-golems ecosystem. It has a sophisticated profile system that lets users define multiple AI engine configurations (model, temperature, tools, prompts) and switch between them at runtime. We study it here to extract reusable patterns for devctl.

### Overview: Why Pinocchio Matters Here

Pinocchio solves a similar problem: "one tool, many configurations." Its profile system is production-quality and handles:

- **Named profiles** with display names and descriptions.
- **Stack composition** — profiles can inherit from other profiles in a directed acyclic graph.
- **Layered config merging** — multiple config files (system, user, repo, CWD, explicit) are merged with deterministic override rules.
- **External registries** — profiles can be stored in SQLite databases or YAML files separate from the main config.
- **Per-request selection** — the web-chat server resolves profiles from cookies, query params, or request body.

For devctl, we don't need all of this complexity. But the core patterns (named profiles, selection, merging) are directly applicable.

### Config Document Model

**File:** `pinocchio/pkg/configdoc/types.go`

Pinocchio's configuration is a layered YAML document called a `Document`:

```go
type Document struct {
    App      AppBlock                  `yaml:"app"`
    Profile  ProfileBlock              `yaml:"profile"`
    Profiles map[string]*InlineProfile `yaml:"profiles"`
}
```

A concrete `.pinocchio.yml` file:

```yaml
app:
  repositories:
    - ~/.pinocchio/prompts

profile:
  active: gpt-4-fast
  registries:
    - ~/.pinocchio/profiles.db

profiles:
  gpt-4-fast:
    display_name: GPT-4 Fast
    description: Quick responses, moderate quality
    inference_settings:
      engine: openai
      model: gpt-4-turbo
      temperature: 0.7
  gpt-4-quality:
    display_name: GPT-4 Quality
    description: Best quality, slower responses
    stack:
      - profile_slug: gpt-4-fast
    inference_settings:
      temperature: 0.2
```

The three top-level sections are:
- `app` — Application-level settings (repository paths).
- `profile` — Profile selection settings (which profile is active, where to find external registries).
- `profiles` — Inline profile definitions (defined directly in this file).

### Profile Types: Inline vs Registry

Pinocchio supports two sources of profiles:

**Inline profiles** are defined directly in the config document under `profiles:`. They're simple and good for project-local configurations.

**Registry profiles** come from external stores — SQLite databases or YAML files referenced by `profile.registries`. They support versioning, provenance tracking, and can be shared across projects.

The `geppetto/pkg/engineprofiles/` package provides the registry infrastructure:

```go
// A registry is a named collection of profiles
type EngineProfileRegistry struct {
    Slug                     RegistrySlug
    DefaultEngineProfileSlug EngineProfileSlug
    Profiles                 map[EngineProfileSlug]*EngineProfile
}

// A store provides persistence (in-memory, SQLite, or YAML file)
type EngineProfileStore interface {
    EngineProfileStoreReader
    EngineProfileStoreWriter
}
```

For devctl, we should start with inline profiles only. External registries add complexity that isn't justified yet.

### Stack Composition and Merging

**File:** `geppetto/pkg/engineprofiles/stack_resolver.go`, `stack_merge.go`

Pinocchio's most powerful feature is **stack composition**. A profile can declare a `stack` of base profiles that it layers on top of:

```yaml
profiles:
  base:
    inference_settings:
      engine: openai
      temperature: 0.5
  fast:
    stack:
      - profile_slug: base
    inference_settings:
      model: gpt-4-turbo
      temperature: 0.7   # overrides base temperature
  quality:
    stack:
      - profile_slug: base
    inference_settings:
      model: gpt-4
      temperature: 0.2
```

The stack resolver performs a depth-first traversal to produce a base→leaf ordering:

```
fast resolves to: [base, fast]
quality resolves to: [base, quality]
```

The merge function then applies each layer in order, with later values overriding earlier ones. It handles:
- Map merging (recursive, not replace).
- Scalar overriding (last wins).
- Cycle detection (with a configurable max depth).

**For devctl:** Stack composition is powerful but adds significant complexity. We should start without it. If needed later, the geppetto `stack_resolver` and `stack_merge` patterns can be adapted.

### Profile Resolution Chain

**File:** `pinocchio/pkg/cmds/profilebootstrap/profile_selection.go`

Pinocchio resolves the active profile through a multi-step chain:

```
1. Load config files (system → user → XDG → git-root → CWD → explicit)
2. Merge documents (later files override earlier ones)
3. Extract profile.active from the effective document
4. CLI --profile flag overrides profile.active
5. Build the profile registry (inline + external stores)
6. Resolve the profile (expand stack, merge layers)
7. Return the effective settings
```

The config file layers (from `pinocchioConfigPlanBuilder`):

```go
glazedconfig.SystemAppConfig("pinocchio")       // /etc/pinocchio/config
glazedconfig.HomeAppConfig("pinocchio")          // ~/.config/pinocchio/config
glazedconfig.XDGAppConfig("pinocchio")           // $XDG_CONFIG_HOME/pinocchio/config
glazedconfig.GitRootFile(".pinocchio.yml")       // <repo-root>/.pinocchio.yml
glazedconfig.WorkingDirFile(".pinocchio.yml")    // ./.pinocchio.yml
glazedconfig.ExplicitFile(explicit)              // --config <path>
```

Each layer can set `profile.active`, define inline profiles, or reference external registries. Later layers override earlier ones.

**For devctl:** We should support a simpler chain: `.devctl.yaml` is the single source of truth. No layered config files initially.

### Per-Request Profile Selection (web-chat)

**File:** `pinocchio/cmd/web-chat/profiles/resolver.go`

The web-chat server resolves profiles per HTTP request. The resolution priority:

1. **Path parameter** — `/api/chat/profiles/{slug}`
2. **Request body** — `{"profile": "gpt-4-fast"}`
3. **Query parameter** — `?profile=gpt-4-fast`
4. **Cookie** — `chat_profile=default/gpt-4-fast`

This allows users to switch profiles in the web UI without restarting the server.

**For devctl:** Per-request switching doesn't apply. devctl selects the profile at CLI invocation time (`devctl up --profile production`) and it stays fixed for the lifetime of that `up` session.

### Lessons for devctl

Studying pinocchio's profile system yields these key takeaways for devctl:

| Pattern from Pinocchio | Applicability to devctl |
|------------------------|------------------------|
| Named profiles with display names/descriptions | ✅ Yes — helps `devctl profiles list` be informative |
| `profile.active` field in config | ✅ Yes — simple, discoverable, no flag needed for default |
| `--profile` CLI flag override | ✅ Yes — essential for ad-hoc switching |
| Inline profiles in config file | ✅ Yes — start simple, avoid external stores |
| Stack composition (inheritance) | ❌ Skip for v1 — adds complexity, unclear benefit for plugin selection |
| External registries (SQLite, YAML) | ❌ Skip for v1 — devctl repos typically have one config file |
| Per-request profile switching | ❌ Not applicable — devctl is CLI-only, not a server |
| Layered config files (system → user → repo) | ❌ Skip for v1 — devctl uses a single `.devctl.yaml` |
| Config document merge with presence tracking | ⚠️ Maybe — useful if we later support override files |


---

## 5. Gap Analysis

This section maps each limitation of the current devctl architecture to the specific code changes needed.

### Gap 1: No plugin filtering

**Current:** `discovery.Discover()` returns all plugins from config + auto-discovery. `repository.Load()` passes all specs to `StartClients()`. There is no filter step.

**Needed:** A filter step between discovery and client-starting that selects only the plugins listed in the active profile.

**Files affected:**
- `pkg/discovery/discovery.go` — add a profile-aware filter (or add filtering in the repository layer).
- `pkg/repository/repository.go` — accept profile selection, filter specs before starting clients.

### Gap 2: No profile data model

**Current:** `config.File` has `Plugins` and `Strictness`. No profile fields.

**Needed:** A `Profiles` map and `Profile` selection block in the config model.

**Files affected:**
- `pkg/config/config.go` — add `Profile` and `Profiles` fields to `File`.

### Gap 3: No profile selection mechanism

**Current:** CLI flags include `--repo-root`, `--config`, `--strict`, `--dry-run`, `--timeout`. No `--profile` flag.

**Needed:** A `--profile` flag on all commands that load plugins. A `profile.active` config field as the default.

**Files affected:**
- `cmd/devctl/cmds/common.go` — add `Profile` to `RepoSettings` and `RepoContext`.
- `cmd/devctl/cmds/common.go` — add `--profile` to `AddRepoFlags()`.
- `cmd/devctl/cmds/up.go` — pass profile selection through to repository loader.

### Gap 4: Profile-blind state

**Current:** `state.State` records `RepoRoot`, `CreatedAt`, and `Services`. No profile information.

**Needed:** Record the active profile name in the state file.

**Files affected:**
- `pkg/state/state.go` — add `Profile string` to `State`.

### Gap 5: Dynamic commands are all-or-nothing

**Current:** `AddDynamicPluginCommands()` starts ALL plugins and registers ALL their commands. There's no way to only show commands relevant to a profile.

**Needed:** Profile-aware command discovery — only start plugins in the active profile.

**Files affected:**
- `cmd/devctl/cmds/dynamic_commands.go` — accept profile selection, filter plugins.

### Gap 6: No profile management commands

**Current:** No `devctl profiles` command exists.

**Needed:** `devctl profiles list` and `devctl profiles active` subcommands.

**Files affected:**
- `cmd/devctl/cmds/root.go` — add a `profiles` command group.
- New file `cmd/devctl/cmds/profiles.go`.

---

## 6. Proposed Architecture

### Profile Config Model

The shared `.devctl.yaml` file gains two new top-level sections:

```yaml
# New: profile selection
profile:
  active: development    # default profile (optional)

# New: profile definitions
profiles:
  development:
    display_name: Development Mode
    description: Hot-reload, verbose logging, mock services
    plugins:
      - web-server
      - mock-api
    env:
      LOG_LEVEL: debug
      NODE_ENV: development

  production:
    display_name: Production Mode
    description: Optimized builds, real services, minimal logging
    plugins:
      - web-server
      - api-server
      - database
      - cache
    env:
      LOG_LEVEL: warn
      NODE_ENV: production

  frontend:
    display_name: Frontend Only
    description: Just the web server and mock API
    plugins:
      - web-server
      - mock-api

# Existing: plugin definitions (unchanged)
plugins:
  - id: web-server
    path: ./plugins/devctl-web-server
    priority: 10
  - id: mock-api
    path: ./plugins/devctl-mock-api
    priority: 20
  - id: api-server
    path: ./plugins/devctl-api-server
    priority: 30
  - id: database
    path: ./plugins/devctl-database
    priority: 40
  - id: cache
    path: ./plugins/devctl-cache
    priority: 50
```

**Go model changes to `config.File`:**

```go
type File struct {
    Profile    ProfileBlock            `yaml:"profile,omitempty"`
    Profiles   map[string]*Profile     `yaml:"profiles,omitempty"`
    Plugins    []Plugin                `yaml:"plugins"`
    Strictness string                  `yaml:"strictness,omitempty"`
}

type ProfileBlock struct {
    Active string `yaml:"active,omitempty"`
}

type Profile struct {
    DisplayName string            `yaml:"display_name,omitempty"`
    Description string            `yaml:"description,omitempty"`
    Plugins     []string          `yaml:"plugins"`            // list of plugin IDs
    Env         map[string]string `yaml:"env,omitempty"`      // env overrides for all plugins
}
```

**Backward compatibility:** If `profile.active` is empty and no `--profile` flag is given, the system loads ALL top-level plugins (current behavior). If a profile is specified but a plugin in that profile isn't in the config, it's an error.

**The `default` profile name:** `default` is an ordinary profile name, not a magic fallback. This is deliberate. A project can define `profiles.default` and select it explicitly:

```yaml
profile:
  active: default

profiles:
  default:
    display_name: Default Development Profile
    plugins:
      - web-server
      - api-server
```

A user can also request it explicitly:

```bash
devctl up --profile default
```

But if a config defines `profiles.default` and does not set `profile.active`, then `devctl up` with no `--profile` still loads the top-level `plugins:` list. This preserves old repositories and avoids turning the mere presence of a profile named `default` into a behavior change.

### Local Override File

`devctl` should also support a local `.devctl.override.yaml` file in the repository root. This file is optional. If present, it is loaded after `.devctl.yaml` and merged into the base config before profile resolution.

The purpose of the override file is narrow:

- Let an individual developer add local profiles without editing the shared project config.
- Let an individual developer adjust profile defaults, such as `profile.active` or profile-level `env`.
- Let an individual developer add or tweak local plugin definitions when a local profile needs them.

It is not a general config-source system. There are no SQLite stores, user registries, remote registries, or multi-directory search chains in v1. The load order is exactly:

```text
.devctl.yaml
  -> .devctl.override.yaml, if present
  -> --profile flag, if present
```

A local override can define a personal profile:

```yaml
# .devctl.override.yaml
profile:
  active: frontend-local

profiles:
  frontend-local:
    display_name: Frontend Local
    description: Frontend with a locally running API stub
    plugins:
      - web-server
      - mock-api
    env:
      API_BASE_URL: http://localhost:4100
      LOG_LEVEL: debug
```

It can also adjust an existing shared profile without restating the entire profile:

```yaml
# .devctl.override.yaml
profiles:
  development:
    env:
      LOG_LEVEL: trace
      DEVCTL_TRACE: "1"
```

The merge rules should be deterministic and easy to explain:

| Field shape | Merge rule |
|---|---|
| Scalars, such as `profile.active` and `strictness` | Override value wins when non-empty. |
| Maps, such as `profiles` and `env` | Keys are merged; override keys win. |
| Profile records | Merged field-by-field so local overrides can adjust `env` without restating `plugins`. |
| Profile `plugins` list | Replaced when the override provides a non-empty list. |
| Top-level `plugins` list | Merged by plugin `id`; override entries with an existing `id` patch that plugin, and new IDs are appended. |
| Plugin `args` list | Replaced when the override provides a non-empty list. |
| Plugin `env` map | Merged; override keys win. |

The `plugins` merge-by-ID rule is what allows a local profile to reference a local plugin. For example:

```yaml
# .devctl.override.yaml
profiles:
  tracing:
    plugins:
      - api-server
      - jaeger-local

plugins:
  - id: jaeger-local
    path: ./plugins/devctl-jaeger-local
    priority: 90
```

The override file should normally be gitignored. The implementation should not require it to be ignored, because some teams may intentionally commit an override file for a local template, but the expected developer workflow is local and personal.

### Profile Selection Flow

The profile selection follows this priority (highest wins):

```text
1. --profile <name> CLI flag                    (explicit user choice)
2. profile.active in .devctl.override.yaml      (local developer default)
3. profile.active in .devctl.yaml               (project default)
4. (no profile)                                 (load all top-level plugins)
```

This priority falls out of the config merge order: `.devctl.override.yaml` is merged over `.devctl.yaml`, and then the CLI flag is applied as the final explicit override.

The selection happens early, before plugin discovery is finalized:

```
User runs: devctl up --profile production

    ┌─────────────────┐
    │  Parse CLI flags │
    │  --profile=prod  │
    └────────┬────────┘
             │
    ┌────────▼────────┐
    │  Load base config │
    │  .devctl.yaml     │
    └────────┬────────┘
             │
    ┌────────▼──────────────┐
    │  Merge local override  │
    │  .devctl.override.yaml │
    │  if present            │
    └────────┬──────────────┘
             │
    ┌────────▼────────┐
    │  Resolve profile │
    │  flag > override │
    │  > base active   │
    │  → "production"  │
    └────────┬────────┘
             │
    ┌────────▼────────┐
    │  Discover all    │
    │  plugins         │
    └────────┬────────┘
             │
    ┌────────▼────────┐
    │  Filter to       │
    │  profile plugins │
    │  [web,api,db,cache]│
    └────────┬────────┘
             │
    ┌────────▼────────┐
    │  Start filtered  │
    │  clients         │
    └────────┬────────┘
             │
    ┌────────▼────────┐
    │  Run pipeline    │
    │  → supervise     │
    └────────┬────────┘
```

### Plugin Filtering by Profile

When a profile is active, the system filters the discovered plugins:

```go
// In repository.go or a new profile resolver

func FilterPluginsForProfile(specs []runtime.PluginSpec, profile *config.Profile, profileEnv map[string]string) []runtime.PluginSpec {
    if profile == nil {
        return specs  // no profile → all plugins
    }
    
    // Build a set of allowed plugin IDs
    allowed := map[string]bool{}
    for _, id := range profile.Plugins {
        allowed[id] = true
    }
    
    var filtered []runtime.PluginSpec
    for _, spec := range specs {
        if !allowed[spec.ID] {
            continue
        }
        // Merge profile-level env overrides into the spec
        merged := spec
        if len(profileEnv) > 0 {
            merged.Env = mergeEnvMaps(spec.Env, profileEnv)
        }
        filtered = append(filtered, merged)
    }
    return filtered
}
```

### Profile-Aware Pipeline

The pipeline itself (`engine.Pipeline`) doesn't need to change. It already operates on a list of `Client` objects. The filtering happens before clients are started, so the pipeline receives only the relevant clients.

This is a key design principle: **profiles are a selection/filtering concern, not a pipeline concern.** The pipeline does the same thing regardless of which plugins are loaded.

### CLI Changes

**New flag: `--profile`**

Added to the `repo` section in `common.go`:

```go
// In AddRepoFlags()
fields.New(
    "profile",
    fields.TypeString,
    fields.WithDefault(""),
    fields.WithHelp("Active profile name (overrides profile.active in config)"),
)
```

**New command: `devctl profiles`**

```
devctl profiles
├── list      # List available profiles with display names and descriptions
└── active    # Show which profile is currently active
```

**Modified commands:** `up`, `down`, `status`, `plan`, `plugins list`, `tui`, `logs`, `stream` — all gain profile awareness through the shared `RepoContext`.

### State File Changes

The `State` struct gains a `Profile` field:

```go
type State struct {
    RepoRoot  string          `json:"repo_root"`
    Profile   string          `json:"profile,omitempty"`  // NEW
    CreatedAt time.Time       `json:"created_at"`
    Services  []ServiceRecord `json:"services"`
}
```

When `devctl up` writes state, it records the active profile. `devctl down` reads it for verification. `devctl status` displays it.

### Dynamic Commands and Profiles

Dynamic command discovery (`AddDynamicPluginCommands`) currently starts ALL plugins to read their handshakes. With profiles, it should only start plugins from the active profile:

```go
func AddDynamicPluginCommands(root *cobra.Command, args []string) error {
    // ... parse repo args ...
    // NEW: resolve profile
    profile, err := resolveActiveProfile(repoRoot, cfgPath, parsedProfileFlag)
    
    // Load repository with profile filtering
    repo, err := repository.Load(repository.Options{
        RepoRoot:   repoRoot,
        ConfigPath: cfgPath,
        Profile:    profile,   // NEW
    })
    
    // Only start filtered plugins for command discovery
    // ... rest is the same ...
}
```


---

## 7. API References and Pseudocode

This section provides concrete code sketches for each component. These are not final implementations but show the exact interfaces and data flow.

### Config File Schema

**File to modify:** `devctl/pkg/config/config.go`

The same schema is used for both `.devctl.yaml` and `.devctl.override.yaml`. The override file is not a separate format; it is a partial config document that is merged over the base document.

```go
package config

// File represents the parsed devctl YAML configuration.
// It is used for both .devctl.yaml and .devctl.override.yaml.
type File struct {
    // Profile selects the active profile by name.
    // This can be overridden by the --profile CLI flag.
    Profile    ProfileBlock        `yaml:"profile,omitempty"`
    
    // Profiles defines named configurations, each selecting a subset of plugins.
    // The key is the profile slug (lowercase, hyphenated).
    Profiles   map[string]*Profile `yaml:"profiles,omitempty"`
    
    // Plugins lists all available plugins.
    // When a profile is active, only plugins listed in that profile are loaded.
    Plugins    []Plugin            `yaml:"plugins"`
    
    // Strictness controls error handling for name collisions.
    // "warn" (default) or "error".
    Strictness string              `yaml:"strictness,omitempty"`
}

// ProfileBlock configures which profile is active.
type ProfileBlock struct {
    // Active is the name of the default profile.
    // Overridden by the --profile CLI flag.
    Active string `yaml:"active,omitempty"`
}

// Profile defines a named configuration that selects a subset of plugins.
type Profile struct {
    // DisplayName is a human-readable name shown in profile listings.
    DisplayName string            `yaml:"display_name,omitempty"`
    
    // Description explains what this profile is for.
    Description string            `yaml:"description,omitempty"`
    
    // Plugins is the list of plugin IDs to activate.
    // Each ID must match a plugin in the top-level plugins list.
    // Order does not matter here; plugin ordering is controlled by priority.
    Plugins     []string          `yaml:"plugins"`
    
    // Env provides environment variable overrides applied to ALL plugins
    // when this profile is active. Plugin-level env takes precedence.
    Env         map[string]string `yaml:"env,omitempty"`
}

// Plugin remains unchanged from current implementation.
type Plugin struct {
    ID       string            `yaml:"id"`
    Path     string            `yaml:"path"`
    Args     []string          `yaml:"args,omitempty"`
    Priority int               `yaml:"priority,omitempty"`
    WorkDir  string            `yaml:"workdir,omitempty"`
    Env      map[string]string `yaml:"env,omitempty"`
}

// Merge overlays override onto f and returns a new File.
// It is used to apply .devctl.override.yaml over .devctl.yaml.
func (f *File) Merge(override *File) *File {
    // Scalars: override non-empty values win.
    // Maps: deep merge; override keys win.
    // Profiles: merge profile records field-by-field.
    // Plugins: merge by plugin ID.
}

// LoadStacked loads .devctl.yaml and then .devctl.override.yaml if present.
func LoadStacked(basePath string, overridePath string) (*File, error) {
    base, err := LoadOptional(basePath)
    if err != nil {
        return nil, err
    }

    override, err := LoadOptional(overridePath)
    if err != nil {
        return nil, err
    }

    return base.Merge(override), nil
}

// ResolveProfile determines the active profile name.
// Priority after config merge: explicitFlag > merged profile.active > "" (no profile).
func (f *File) ResolveProfile(explicitFlag string) string {
    if explicitFlag != "" {
        return explicitFlag
    }
    return f.Profile.Active
}

// GetProfile returns the named profile, or nil if it doesn't exist.
func (f *File) GetProfile(name string) *Profile {
    if name == "" || f.Profiles == nil {
        return nil
    }
    return f.Profiles[name]
}

// ValidateProfile checks that all plugins referenced by a profile exist.
func (f *File) ValidateProfile(name string) error {
    p := f.GetProfile(name)
    if p == nil {
        return fmt.Errorf("profile %q not found", name)
    }
    
    known := map[string]bool{}
    for _, plugin := range f.Plugins {
        known[plugin.ID] = true
    }
    
    for _, pluginID := range p.Plugins {
        if !known[pluginID] {
            return fmt.Errorf("profile %q references unknown plugin %q", name, pluginID)
        }
    }
    return nil
}
```

### Profile Resolver Pseudocode

The profile resolver sits between config loading and plugin discovery. It determines which plugins to activate.

```go
package profile

// ResolveResult contains the outcome of profile resolution.
type ResolveResult struct {
    // ProfileName is the resolved profile name (may be empty for "all plugins").
    ProfileName string
    
    // Profile is the resolved profile config (nil if no profile active).
    Profile *config.Profile
    
    // ActivePluginIDs is the set of plugin IDs to load.
    // If nil/empty, all plugins should be loaded (backward compatible).
    ActivePluginIDs map[string]bool
}

// Resolve determines the active profile and which plugins to load.
func Resolve(cfg *config.File, explicitFlag string) (*ResolveResult, error) {
    name := cfg.ResolveProfile(explicitFlag)
    
    // No profile selected → load every top-level plugin (backward compatible).
    // A profile named "default" is not selected implicitly; it must be chosen
    // through --profile default or profile.active: default.
    if name == "" {
        return &ResolveResult{
            ProfileName:     "",
            Profile:         nil,
            ActivePluginIDs: nil, // nil means "all"
        }, nil
    }
    
    profile := cfg.GetProfile(name)
    if profile == nil {
        return nil, fmt.Errorf("profile %q not found in config", name)
    }
    
    // Validate that all referenced plugins exist.
    if err := cfg.ValidateProfile(name); err != nil {
        return nil, err
    }
    
    // Build the set of active plugin IDs.
    activeIDs := make(map[string]bool, len(profile.Plugins))
    for _, id := range profile.Plugins {
        activeIDs[id] = true
    }
    
    return &ResolveResult{
        ProfileName:     name,
        Profile:         profile,
        ActivePluginIDs: activeIDs,
    }, nil
}
```

### Repository Loader Changes

**File to modify:** `devctl/pkg/repository/repository.go`

The repository loader gains a `ProfileName` field, loads the optional local override, and filters specs after discovery.

```go
// Options now includes profile selection and optional override path.
type Options struct {
    RepoRoot       string
    ConfigPath     string
    OverridePath   string   // optional; defaults to <repo>/.devctl.override.yaml
    Cwd            string
    DryRun         bool
    ProfileName    string   // explicit --profile flag value
}

type Repository struct {
    Root         string
    Config       *config.File
    Specs        []runtime.PluginSpec
    SpecByID     map[string]runtime.PluginSpec
    Request      runtime.RequestMeta
    ConfigAbs    string
    OverrideAbs  string           // NEW: resolved override path, if present
    ProfileName  string           // NEW: resolved profile name
    Profile      *config.Profile  // NEW: resolved profile (nil = all)
}

func Load(opts Options) (*Repository, error) {
    // ... existing repo-root and config-path resolution ...

    overridePath := opts.OverridePath
    if overridePath == "" {
        overridePath = filepath.Join(root, ".devctl.override.yaml")
    }

    cfg, err := config.LoadStacked(cfgPath, overridePath)
    if err != nil {
        return nil, err
    }
    
    // Discover all plugins from the merged config.
    specs, err := discovery.Discover(cfg, discovery.Options{RepoRoot: root})
    if err != nil {
        return nil, err
    }
    
    // NEW: Resolve profile and filter plugins.
    profileName := cfg.ResolveProfile(opts.ProfileName)
    var profile *config.Profile
    if profileName != "" {
        profile = cfg.GetProfile(profileName)
        if profile == nil {
            return nil, fmt.Errorf("profile %q not found", profileName)
        }
        if err := cfg.ValidateProfile(profileName); err != nil {
            return nil, err
        }
        specs = filterSpecs(specs, profile)
    }
    
    // ... rest unchanged ...
}

// filterSpecs keeps only plugins listed in the profile
// and merges profile-level env overrides.
func filterSpecs(specs []runtime.PluginSpec, profile *config.Profile) []runtime.PluginSpec {
    allowed := make(map[string]bool, len(profile.Plugins))
    for _, id := range profile.Plugins {
        allowed[id] = true
    }
    
    filtered := make([]runtime.PluginSpec, 0, len(profile.Plugins))
    for _, spec := range specs {
        if !allowed[spec.ID] {
            continue
        }
        // Merge profile env on top of plugin env.
        merged := spec
        if len(profile.Env) > 0 {
            merged.Env = mergeEnvMaps(spec.Env, profile.Env)
        }
        filtered = append(filtered, merged)
    }
    return filtered
}

// mergeEnvMaps returns a new map with base values overridden by overlay.
func mergeEnvMaps(base, overlay map[string]string) map[string]string {
    result := make(map[string]string, len(base)+len(overlay))
    for k, v := range base {
        result[k] = v
    }
    for k, v := range overlay {
        result[k] = v  // overlay wins
    }
    return result
}
```

### Pipeline Changes

**The pipeline does NOT change.** It receives a filtered list of clients and operates on them exactly as before. This is intentional — profiles are a selection concern, not an orchestration concern.

The only pipeline-adjacent change is in `up.go`, where the profile name is passed through to the state file:

```go
// In up.go, when saving state:
st, err := sup.Start(ctx, plan)
// ... 
state := &state.State{
    RepoRoot:  opts.RepoRoot,
    Profile:   profileName,  // NEW
    CreatedAt: time.Now().UTC(),
    Services:  st.Services,
}
if err := state.Save(opts.RepoRoot, state); err != nil { ... }
```

### Common CLI Layer Changes

**File to modify:** `devctl/cmd/devctl/cmds/common.go`

```go
type RepoSettings struct {
    RepoRoot string `glazed:"repo-root"`
    Config   string `glazed:"config"`
    Strict   bool   `glazed:"strict"`
    DryRun   bool   `glazed:"dry-run"`
    Timeout  string `glazed:"timeout"`
    Profile  string `glazed:"profile"`  // NEW
}

type RepoContext struct {
    RepoRoot   string
    ConfigPath string
    Cwd        string
    Strict     bool
    DryRun     bool
    Timeout    time.Duration
    Profile    string  // NEW
}

// In getRepoLayer(), add the profile field:
fields.New(
    "profile",
    fields.TypeString,
    fields.WithDefault(""),
    fields.WithHelp("Active profile name (overrides profile.active in config)"),
),
```


---

## 8. Diagrams

### 8.1 Current Architecture (No Profiles)

```
.devctl.yaml                devctl CLI
┌─────────────────┐        ┌──────────────────────────────────────────────┐
│ plugins:        │        │                                              │
│   - id: A       │───────▶│  config.Load()                               │
│   - id: B       │        │       │                                      │
│   - id: C       │        │       ▼                                      │
└─────────────────┘        │  discovery.Discover()                        │
                           │       │                                      │
      plugins/             │       ▼  [A, B, C]                          │
      ┌───────────┐        │  repository.Load()                           │
      │devctl-D   │───────▶│       │                                      │
      └───────────┘        │       ▼                                      │
                           │  factory.Start(A) → Client A                 │
                           │  factory.Start(B) → Client B                 │
                           │  factory.Start(C) → Client C                 │
                           │  factory.Start(D) → Client D                 │
                           │       │                                      │
                           │       ▼                                      │
                           │  Pipeline{Clients: [A, B, C, D]}             │
                           │    .MutateConfig() → config                  │
                           │    .Build() → build result                   │
                           │    .Validate() → validation result           │
                           │    .LaunchPlan() → services                  │
                           │       │                                      │
                           │       ▼                                      │
                           │  Supervisor.Start(services)                  │
                           │  state.Save(.devctl/state.json)              │
                           └──────────────────────────────────────────────┘
```

### 8.2 Proposed Architecture (With Profiles)

```
.devctl.yaml                devctl CLI
┌─────────────────────┐    ┌──────────────────────────────────────────────┐
│ profile:            │    │                                              │
│   active: dev       │    │  config.Load()                               │
│ profiles:           │    │       │                                      │
│   dev:              │    │       ▼                                      │
│     plugins: [A,B]  │    │  profile.Resolve(profileName="dev")          │
│   prod:             │    │       │                                      │
│     plugins: [A,C,D]│    │       ▼  ActivePluginIDs = {A, B}           │
│ plugins:            │    │                                              │
│   - id: A           │───▶│  discovery.Discover() → [A, B, C, D]        │
│   - id: B           │    │       │                                      │
│   - id: C           │    │       ▼  filterSpecs(allowed={A, B})         │
│   - id: D           │    │       │                                      │
└─────────────────────┘    │       ▼  [A, B]                             │
                           │                                              │
                           │  repository.Load(opts, profile="dev")        │
                           │       │                                      │
                           │       ▼                                      │
                           │  factory.Start(A) → Client A                 │
                           │  factory.Start(B) → Client B                 │
                           │       │                                      │
                           │       ▼                                      │
                           │  Pipeline{Clients: [A, B]}                   │
                           │    .MutateConfig() → config                  │
                           │    .Build() → build result                   │
                           │    .Validate() → validation result           │
                           │    .LaunchPlan() → services                  │
                           │       │                                      │
                           │       ▼                                      │
                           │  Supervisor.Start(services)                  │
                           │  state.Save(.devctl/state.json,              │
                           │    profile="dev")                            │
                           └──────────────────────────────────────────────┘
```

### 8.3 Profile Resolution Decision Tree

```
                    ┌──────────────────┐
                    │  --profile flag? │
                    └───────┬──────────┘
                            │
               ┌────────────┴────────────┐
               │                         │
           Yes  │                         │  No
               ▼                         ▼
    ┌─────────────────┐       ┌──────────────────┐
    │ Use flag value   │       │ profile.active   │
    │ as profile name  │       │ in .devctl.yaml? │
    └────────┬────────┘       └─────────┬────────┘
             │                          │
             │              ┌───────────┴───────────┐
             │              │                       │
             │          Yes │                       │  No
             │              ▼                       ▼
             │    ┌─────────────────┐    ┌─────────────────┐
             │    │ Use active value │    │ No profile      │
             │    │ as profile name  │    │ Load ALL plugins│
             │    └────────┬────────┘    └─────────────────┘
             │             │
             └──────┬──────┘
                    │
                    ▼
         ┌──────────────────┐
         │ Profile exists   │
         │ in profiles map? │
         └───────┬──────────┘
                 │
        ┌────────┴────────┐
        │                 │
    Yes │                 │  No
        ▼                 ▼
  ┌───────────┐    ┌───────────┐
  │ Filter    │    │ ERROR:    │
  │ plugins   │    │ profile   │
  │ to match  │    │ not found │
  └───────────┘    └───────────┘
```

### 8.4 State File With Profile

```
.devctl/state.json
┌───────────────────────────────────────┐
│ {                                     │
│   "repo_root": "/path/to/repo",       │
│   "profile": "development",  ← NEW   │
│   "created_at": "2026-05-04T...",     │
│   "services": [                       │
│     {                                 │
│       "name": "web-server",           │
│       "pid": 12345,                   │
│       "command": [...]                │
│     },                                │
│     {                                 │
│       "name": "mock-api",             │
│       "pid": 12346,                   │
│       "command": [...]                │
│     }                                 │
│   ]                                   │
│ }                                     │
└───────────────────────────────────────┘
```

---

## 9. Implementation Phases

### Phase 1: Config Model, Override Merge, and Profile Resolution

**Goal:** Add profile types to config, implement local override stacking, implement resolution, maintain backward compatibility.

**Files to create/modify:**

| File | Change |
|------|--------|
| `pkg/config/config.go` | Add `ProfileBlock`, `Profile` types; add `Merge()`, `LoadStacked()`, `ResolveProfile()`, `GetProfile()`, `ValidateProfile()` methods |
| `pkg/config/config_test.go` | New tests for profile resolution, override merge behavior, validation, backward compatibility |

**Validation:**
- `config.LoadOptional()` parses `.devctl.yaml` with new fields.
- `config.LoadStacked()` returns the base config unchanged when `.devctl.override.yaml` is absent.
- `config.LoadStacked()` merges local `profiles` entries from `.devctl.override.yaml` over `.devctl.yaml`.
- A local override can add a new profile without restating shared profiles.
- A local override can adjust `profiles.development.env.LOG_LEVEL` without restating `profiles.development.plugins`.
- A local override can set `profile.active`, and that value wins over the base file.
- `ResolveProfile("")` returns the merged `profile.active`.
- `ResolveProfile("production")` returns `"production"`.
- `ResolveProfile("")` with empty config returns `""` (all top-level plugins mode).
- Defining `profiles.default` alone does not activate it; `ResolveProfile("")` still returns `""` unless `profile.active: default` is set in the merged config.
- `ResolveProfile("default")` returns `"default"` and validates against `profiles.default` when present.
- `ValidateProfile("nonexistent")` returns an error.
- `ValidateProfile("dev")` with valid plugin IDs succeeds.
- `ValidateProfile("dev")` with unknown plugin ID returns an error.

```bash
# After Phase 1:
go test ./pkg/config/... -v
```

### Phase 2: Plugin Filtering in Repository

**Goal:** Make `repository.Load()` load the stacked config, accept a profile name, and filter plugins.

**Files to modify:**

| File | Change |
|------|--------|
| `pkg/repository/repository.go` | Add `ProfileName` and optional `OverridePath` to `Options`; call `config.LoadStacked()`; add filtering after discovery |
| `pkg/repository/repository_test.go` | New tests for filtering and local override behavior |

**Validation:**
- `Load(opts)` with no profile returns all plugins.
- `Load(opts)` with profile name filters to that profile's plugins.
- `Load(opts)` sees profiles defined only in `.devctl.override.yaml`.
- `Load(opts)` uses `profile.active` from `.devctl.override.yaml` when no `--profile` flag is present.
- Profile env overrides are merged correctly.
- Plugin ordering (by priority) is preserved after filtering.

```bash
go test ./pkg/repository/... -v
```

### Phase 3: CLI Flag and Command Plumbing

**Goal:** Add `--profile` flag to all relevant commands, thread it through `RepoContext`.

**Files to modify:**

| File | Change |
|------|--------|
| `cmd/devctl/cmds/common.go` | Add `Profile` to `RepoSettings` and `RepoContext`; add flag to `AddRepoFlags()` |
| `cmd/devctl/cmds/up.go` | Pass profile to `repository.Load()`; save profile in state |
| `cmd/devctl/cmds/down.go` | Read profile from state (for display) |
| `cmd/devctl/cmds/status.go` | Display active profile |
| `cmd/devctl/cmds/plan.go` | Pass profile to `repository.Load()` |
| `cmd/devctl/cmds/plugins.go` | Pass profile to `repository.Load()` |
| `cmd/devctl/cmds/tui.go` | Pass profile to `repository.Load()` |
| `cmd/devctl/cmds/dynamic_commands.go` | Pass profile to `repository.Load()` |

**Validation:**
```bash
# --profile flag is recognized
devctl up --profile development --dry-run
devctl plugins list --profile development

# --profile with nonexistent profile gives clear error
devctl up --profile nonexistent --dry-run
# Expected: error: profile "nonexistent" not found in config
```

### Phase 4: State File Changes

**Goal:** Record profile in state file; display in status/down.

**Files to modify:**

| File | Change |
|------|--------|
| `pkg/state/state.go` | Add `Profile string` to `State` struct |
| `cmd/devctl/cmds/up.go` | Set `Profile` when saving state |
| `cmd/devctl/cmds/down.go` | Display profile when reading state |
| `cmd/devctl/cmds/status.go` | Display profile in status output |

**Validation:**
```bash
# Start with a profile
devctl up --profile development
cat .devctl/state.json | jq .profile
# Expected: "development"

devctl status
# Expected output includes profile name

# Clean up
devctl down
```

### Phase 5: Profile Management Commands

**Goal:** Add `devctl profiles list` and `devctl profiles active`.

**Files to create:**

| File | Change |
|------|--------|
| `cmd/devctl/cmds/profiles.go` | New file with `profiles list` and `profiles active` commands |

**`devctl profiles list` output example:**

```
PROFILE         DISPLAY NAME        DESCRIPTION                          PLUGINS
development     Development Mode    Hot-reload, verbose logging          web-server, mock-api
production      Production Mode     Optimized builds, real services      web-server, api-server, database, cache
frontend        Frontend Only       Just the web server and mock API     web-server, mock-api
```

**`devctl profiles active` output example:**

```
development
```

**Validation:**
```bash
devctl profiles list
devctl profiles active
devctl profiles active --profile production
# Expected: production
```

### Phase 6: Integration Tests and Documentation

**Goal:** End-to-end tests with a multi-profile `.devctl.yaml`.

**Files to create:**

| File | Change |
|------|--------|
| `testdata/profiles/` | Test fixtures with profile-aware config |
| `cmd/devctl/cmds/profiles_test.go` | Unit tests for profile commands |
| Integration test script | End-to-end test of profile selection |

**Validation:**
```bash
go test ./... -v -count=1
```


---

## 10. Test Strategy

### Unit Tests

| Component | Test File | What to test |
|-----------|-----------|--------------|
| `config.File` | `pkg/config/config_test.go` | Parse profiles, merge `.devctl.override.yaml`, resolve profile name, validate profile references, backward compat (config without profiles) |
| `repository.Load` | `pkg/repository/repository_test.go` | Filtering with profile, filtering without profile, override-defined profiles, env merging, plugin ID validation |
| `state.State` | `pkg/state/state_test.go` | Round-trip with profile field, backward compat loading old state files without profile |

### Integration Tests

Use the existing test plugin infrastructure in `testdata/plugins/`:

1. Create a test `.devctl.yaml` with multiple profiles.
2. Run `devctl up --profile <name> --dry-run` and verify only the correct plugins are loaded.
3. Run `devctl profiles list` and verify output.
4. Run `devctl down` after `up` with a profile and verify clean shutdown.

### Smoke Tests

Extend the existing smoke test framework in `cmd/devctl/cmds/dev/smoketest/`:

```bash
# New smoke test: profile selection
devctl smoketest profiles --config testdata/profiles/.devctl.yaml
```

### Backward Compatibility Tests

Critical: these ensure existing repos without profiles continue to work unchanged:

1. `.devctl.yaml` with no `profile` or `profiles` section → all plugins load.
2. `.devctl.yaml` with empty `profile.active` → all top-level plugins load.
3. `.devctl.yaml` with `profiles.default` but no `profile.active` → all top-level plugins load; `default` is not implicit.
4. `.devctl.yaml` with `profile.active: default` and `profiles.default` → only the default profile's plugins load.
5. Missing `.devctl.override.yaml` → no error and base config behavior is unchanged.
6. `.devctl.override.yaml` defining only a new profile → base plugins and shared profiles remain available.
7. Old `state.json` without `profile` field → loads successfully, profile defaults to empty string.

---

## 11. Risks, Alternatives, and Open Questions

### Risks

1. **Breaking backward compatibility.** If the profile filtering is too aggressive, existing repos without profiles might break.
   - **Mitigation:** No profile = all plugins. This is the default and must never change.
   
2. **Profile validation misses.** If a profile references a plugin ID that doesn't exist (typo, plugin not installed), the user gets a confusing error.
   - **Mitigation:** `ValidateProfile()` runs early and returns a clear error message with the offending ID.

3. **Dynamic command discovery slowdown.** Starting only profile-filtered plugins for command discovery could miss commands the user expects.
   - **Mitigation:** Dynamic commands from all plugins could still be registered, but only the filtered plugins would actually run when invoked. However, this means starting unneeded plugins. Open question: which approach is better?

4. **Profile env override confusion.** If a plugin defines `env: {PORT: "3000"}` and a profile defines `env: {PORT: "8080"}`, the profile wins. This might surprise users who expect plugin-level env to always take precedence.
   - **Mitigation:** Document the override order clearly. Consider adding a `plugin_env_overrides` field in profiles for per-plugin env (see open questions).

5. **Override merge confusion.** A local `.devctl.override.yaml` can change `profile.active`, patch profiles, and patch plugins. Users may be confused about whether a list merges or replaces.
   - **Mitigation:** Keep the merge rules small and documented: maps merge, profile records merge by field, top-level plugins merge by `id`, and lists such as `profile.plugins` and `plugin.args` replace when provided.

6. **Accidental team-specific override commits.** A developer may accidentally commit a personal `.devctl.override.yaml`.
   - **Mitigation:** Recommend adding `.devctl.override.yaml` to `.gitignore`, but do not hard-code this assumption. Some teams may choose to commit a template override intentionally.

### Alternatives Considered

**Alternative 1: Separate config files per profile.**

Instead of one `.devctl.yaml` with a `profiles:` block, have multiple files: `.devctl.development.yaml`, `.devctl.production.yaml`.

- **Pros:** Simpler parsing, each file is self-contained.
- **Cons:** Plugin definitions are duplicated across files, harder to keep in sync, more files to manage.
- **Decision:** Rejected. Single file with profiles section is more maintainable.

**Alternative 2: Reuse geppetto `engineprofiles` library.**

Import the full geppetto profile stack resolution, registries, and stores.

- **Pros:** Battle-tested, supports stack composition, external stores.
- **Cons:** Heavy dependency on AI-specific types (`InferenceSettings`); massive over-engineering for devctl's needs; adds a dependency between devctl and geppetto.
- **Decision:** Rejected for v1. Could revisit if devctl needs stack composition later.

**Alternative 3: Profile-only config files (no inline profiles).**

Store profiles in separate `.devctl-profiles/` directory as individual YAML files.

- **Pros:** Profiles can be versioned independently, easy to share.
- **Cons:** More complex loading, two sources of truth.
- **Decision:** Rejected for v1. Inline profiles are simpler. Could add external profile files as a v2 feature.

**Alternative 4: Full pinocchio-style source stack.**

Support user-level config files, SQLite stores, registries, named stacks, and explicit source resolution.

- **Pros:** Powerful and already familiar from pinocchio.
- **Cons:** More machinery than devctl needs for local profile customization; introduces source-order complexity before there is a concrete need.
- **Decision:** Rejected for v1. Implement only `.devctl.yaml` plus local `.devctl.override.yaml`.

### Open Questions

1. **Should profiles support per-plugin env overrides?** Currently the design has profile-level `env:` that applies to all plugins. Should we add per-plugin overrides?

   ```yaml
   profiles:
     development:
       plugins: [web-server, api-server]
       env:
         LOG_LEVEL: debug
       plugin_env:          # per-plugin overrides
         web-server:
           PORT: "3000"
         api-server:
           PORT: "8080"
   ```

   **Recommendation:** Start with profile-level env only. Add per-plugin env if users ask for it.

2. **Should `devctl up` without a profile fail when profiles are defined?** If a `.devctl.yaml` defines profiles, should `devctl up` (no `--profile`) use `profile.active` as the default? If `profile.active` is also empty, should it fall back to all plugins or fail?

   **Recommendation:** Fall back to all plugins (backward compatible). Print a warning if profiles are defined but none is selected: "Warning: profiles defined but no profile selected. All plugins will be loaded."

3. **Should profiles be able to override plugin `args` and `workdir`?** Currently only `env` overrides are supported. Should we also allow overriding `args` and `workdir` per profile?

   **Recommendation:** Not in v1. If needed, add `plugin_overrides` in a future phase.

4. **How should `devctl down` handle profile mismatches?** If `devctl up` was run with profile "development" and `devctl down` is run with profile "production", what happens?

   **Recommendation:** `devctl down` always reads the profile from the state file, not from the CLI. The CLI profile flag is only used for `up`, `plan`, `plugins list`, and dynamic commands.

5. **Should there be a `--all-profiles` flag?** A way to say "load all plugins regardless of what profiles are defined."

   **Recommendation:** Not needed. Simply omitting `--profile` and `profile.active` already means "load all."

6. **Should devctl create or update `.gitignore` automatically for `.devctl.override.yaml`?** The expected workflow is local, so ignoring it is usually right.

   **Recommendation:** Do not mutate `.gitignore` automatically in v1. Document the recommendation and maybe have `devctl profiles init-override` add the ignore entry in a future command.

---

## 12. File Reference Index

All file paths are relative to the workspace root `/home/manuel/workspaces/2026-05-04/devctl-multiple-profiles/`.

### devctl files to modify

| File | Purpose | Changes |
|------|---------|---------|
| `devctl/pkg/config/config.go` | Config file model | Add `ProfileBlock`, `Profile` types; add config merge, override loading, and resolution/validation methods |
| `devctl/pkg/repository/repository.go` | Repository loading | Load stacked `.devctl.yaml` + `.devctl.override.yaml`; add profile filtering after discovery |
| `devctl/pkg/state/state.go` | State persistence | Add `Profile` field to `State` struct |
| `devctl/cmd/devctl/cmds/common.go` | Shared CLI flags | Add `Profile` field and `--profile` flag |
| `devctl/cmd/devctl/cmds/up.go` | Up command | Pass profile to repository; save in state |
| `devctl/cmd/devctl/cmds/down.go` | Down command | Display profile from state |
| `devctl/cmd/devctl/cmds/status.go` | Status command | Display profile from state |
| `devctl/cmd/devctl/cmds/plan.go` | Plan command | Pass profile to repository |
| `devctl/cmd/devctl/cmds/plugins.go` | Plugins list command | Pass profile to repository |
| `devctl/cmd/devctl/cmds/tui.go` | TUI command | Pass profile to repository |
| `devctl/cmd/devctl/cmds/dynamic_commands.go` | Dynamic commands | Pass profile to repository for filtering |
| `devctl/cmd/devctl/cmds/root.go` | Root command | Add `profiles` command group |

### devctl files to create

| File | Purpose |
|------|---------|
| `devctl/cmd/devctl/cmds/profiles.go` | `devctl profiles list` and `devctl profiles active` commands |
| `devctl/pkg/config/config_test.go` | Unit tests for profile config model |
| `devctl/pkg/repository/repository_test.go` | Unit tests for profile filtering |
| `devctl/testdata/profiles/` | Test fixtures for profile-aware configs |

### Key reference files (read-only, for understanding)

| File | What it teaches |
|------|-----------------|
| `devctl/pkg/discovery/discovery.go` | How plugins are discovered from config and `plugins/` directory |
| `devctl/pkg/engine/pipeline.go` | How the pipeline orchestrates ordered plugin calls |
| `devctl/pkg/engine/types.go` | Pipeline data types (ServiceSpec, LaunchPlan, etc.) |
| `devctl/pkg/runtime/client.go` | NDJSON protocol v2 client implementation |
| `devctl/pkg/runtime/factory.go` | Plugin process lifecycle management |
| `devctl/pkg/runtime/meta.go` | `RequestMeta` struct passed to plugins |
| `devctl/pkg/protocol/types.go` | Protocol frame types (Handshake, Request, Response, Event) |
| `devctl/pkg/protocol/validate.go` | Handshake validation rules |
| `devctl/pkg/patch/patch.go` | Config patching (set/unset dotted keys) |
| `devctl/cmd/devctl/main.go` | CLI entry point and dynamic command registration |
| `devctl/examples/plugins/bash-minimal/plugin.sh` | Minimal bash plugin example |
| `devctl/examples/plugins/python-minimal/plugin.py` | Minimal python plugin example |

### Pinocchio reference files (patterns to learn from)

| File | What it teaches |
|------|-----------------|
| `pinocchio/pkg/configdoc/types.go` | Document model with profiles map |
| `pinocchio/pkg/configdoc/profiles.go` | Inline profiles → registry conversion |
| `pinocchio/pkg/configdoc/merge.go` | Layered document merging |
| `pinocchio/pkg/configdoc/load.go` | Config loading and validation |
| `pinocchio/pkg/cmds/profilebootstrap/profile_selection.go` | Profile resolution chain |
| `pinocchio/cmd/web-chat/profiles/resolver.go` | Per-request profile resolution |
| `pinocchio/cmd/web-chat/profiles/api.go` | Profile HTTP API handlers |
| `pinocchio/cmd/web-chat/profiles/types.go` | Profile API types |
| `geppetto/pkg/engineprofiles/types.go` | Core profile types (EngineProfile, Registry) |
| `geppetto/pkg/engineprofiles/stack_resolver.go` | Stack composition with DFS and cycle detection |
| `geppetto/pkg/engineprofiles/stack_merge.go` | Deterministic layer merging |
| `geppetto/pkg/engineprofiles/registry.go` | Registry interface (read/query operations) |
| `geppetto/pkg/engineprofiles/store.go` | Store abstraction (in-memory, SQLite, YAML) |
