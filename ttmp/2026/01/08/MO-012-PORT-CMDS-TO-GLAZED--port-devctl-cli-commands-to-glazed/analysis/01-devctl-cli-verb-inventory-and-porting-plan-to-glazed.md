---
Title: devctl CLI verb inventory and porting plan to Glazed
Ticket: MO-012-PORT-CMDS-TO-GLAZED
Status: active
Topics:
    - devctl
    - glazed
    - cli
    - refactor
    - docs
DocType: analysis
Intent: long-term
Owners: []
RelatedFiles:
    - Path: ../../../../../../../glazed/cmd/glaze/main.go
      Note: Reference for Glazed root init + help system
    - Path: ../../../../../../../glazed/pkg/doc/topics/01-help-system.md
      Note: Reference for help system integration
    - Path: ../../../../../../../glazed/pkg/doc/tutorials/05-build-first-command.md
      Note: Reference for BuildCobraCommand patterns
    - Path: ../../../../../../../glazed/pkg/doc/tutorials/custom-layer.md
      Note: Reference for custom layer design
    - Path: cmd/devctl/cmds/common.go
      Note: Root flags to port into a custom Glazed layer
    - Path: cmd/devctl/cmds/logs.go
      Note: logs flags and follow semantics
    - Path: cmd/devctl/cmds/root.go
      Note: Canonical list of devctl Cobra commands
    - Path: cmd/devctl/cmds/status.go
      Note: status JSON output + tail-lines
    - Path: cmd/devctl/cmds/stream.go
      Note: stream start flags and JSON input handling
    - Path: cmd/devctl/cmds/up.go
      Note: up flags and pipeline execution
ExternalSources: []
Summary: ""
LastUpdated: 2026-01-08T00:28:58.716960441-05:00
WhatFor: ""
WhenToUse: ""
---


# devctl → Glazed port: CLI verb inventory and plan

This document inventories every devctl CLI verb and flag, then proposes a concrete Glazed-based port plan. The goal is not to rewrite behavior, but to change *how the CLI is structured*: move from ad-hoc Cobra flag parsing to Glazed parameter definitions and reusable layers (especially for repo-root/config/timeouts), while integrating the Glazed help system so docs are queryable and consistent.

## 1. Scope and non-goals

This section defines what “port to Glazed” means in practice, so the implementation stays focused and reviewable.

**In scope:**

- Use Glazed’s command model (`cli.BuildCobraCommand`) rather than hand-written Cobra `RunE` blocks.
- Replace ad-hoc root flag parsing with a custom Glazed layer for `repo-root` (and related flags).
- Initialize the Glazed help system in the devctl root command, similar to `glazed/cmd/glaze/main.go`.
- Preserve the existing devctl UX contracts (flags, defaults, output formats, error messages where reasonable).

**Out of scope (for this ticket):**

- Redesigning devctl’s underlying engine/runtime/protocol.
- Changing the output shapes for JSON commands (unless necessary for Glazed integration).
- Rewriting the TUI internals (BubbleTea models, bus) beyond adapting the command wrapper.

## 2. Relevant Glazed references (implementation patterns)

These are the upstream “how to do it the Glazed way” sources that should guide the port.

- Root command + help system setup: `glazed/cmd/glaze/main.go`
- Build a command via `cli.BuildCobraCommand`: `glazed/pkg/doc/tutorials/05-build-first-command.md`
- Create reusable custom layers: `glazed/pkg/doc/tutorials/custom-layer.md`
- Help system concepts and Cobra integration: `glazed/pkg/doc/topics/01-help-system.md`

## 3. Current devctl CLI inventory

This section lists the current verbs and the flags they define. It is the canonical contract for porting work.

### 3.1. Root command and global flags

devctl defines global flags in `cmd/devctl/cmds/common.go`:

- `--repo-root string` (default: current directory; normalized to absolute path)
- `--config string` (default: `.devctl.yaml` under repo-root; relative paths resolved under repo-root)
- `--strict bool` (treat merge collisions as errors)
- `--dry-run bool` (best-effort “no side effects”)
- `--timeout duration` (default timeout for plugin operations; must be > 0)

devctl also includes Glazed’s logging flags via `logging.AddLoggingLayerToRootCommand` in `cmd/devctl/main.go`.

### 3.2. User-facing commands (“verbs”)

Core workflow:

- `devctl up`
  - Flags: `--force`, `--skip-validate`, `--skip-build`, `--skip-prepare`, `--build-step` (repeatable), `--prepare-step` (repeatable)
- `devctl down`
  - Flags: none (uses global flags)
- `devctl status`
  - Flags: `--tail-lines int` (default 25)
- `devctl logs`
  - Flags: `--service string` (required), `--stderr`, `--follow`
- `devctl plan`
  - Flags: none (uses global flags)
- `devctl plugins list`
  - Flags: none (uses global flags)
- `devctl stream start`
  - Flags: `--plugin string`, `--op string` (required), `--input-json string`, `--input-file string`, `--start-timeout duration`, `--json`
- `devctl tui`
  - Flags: `--refresh duration`, `--alt-screen bool`, `--debug-logs bool`

### 3.3. Developer/testing commands

These are valuable during development, but are not necessarily “product UX”:

- These are dev-only commands, moved under a `dev` group:
  - `devctl dev smoketest` (root)
    - Flags: `--plugin string`, `--timeout duration`
  - `devctl dev smoketest supervise`
    - Flags: `--timeout duration`
  - `devctl dev smoketest e2e`
    - Flags: `--timeout duration`
  - `devctl dev smoketest logs`
    - Flags: `--timeout duration`
  - `devctl dev smoketest failures`
    - Flags: `--timeout duration`

### 3.4. Internal commands

- `devctl __wrap-service` (hidden)
  - Flags: `--service`, `--cwd`, `--stdout-log`, `--stderr-log`, `--exit-info`, `--ready-file`, `--env` (repeatable), `--tail-lines`

This command is invoked by `devctl up` internally via the supervisor wrapper.

## 4. Porting architecture: how devctl should look “as a Glazed CLI”

Glazed gives us three big building blocks: help system, reusable parameter layers, and command structs with typed settings. A successful port should make these the default patterns for every verb.

### 4.1. Root command initialization

Devctl’s root should follow the same structure as `glazed/cmd/glaze/main.go`:

1. Create Cobra root
2. Add Glazed logging layer
3. Create help system + load docs
4. Setup Cobra root help integration
5. Register each command by building Cobra commands from Glazed command structs

Key reference snippet (from `glazed/cmd/glaze/main.go`):

```go
helpSystem := help.NewHelpSystem()
err := doc.AddDocToHelpSystem(helpSystem)
cobra.CheckErr(err)
help_cmd.SetupCobraRootCommand(helpSystem, rootCmd)
```

For devctl, the docs source should include:

- devctl docs: `devctl/pkg/doc/topics` (and any future examples/tutorials)
- optional: selected Glazed docs if they are useful to ship with devctl (decision point)

### 4.2. The custom `repo-root` layer (required)

Devctl currently “manually” implements root flag normalization (absolute repo root, config resolution under repo root, timeout validation). In Glazed, this should be a dedicated reusable layer.

**Proposed layer:**

- Layer slug: `repo` (or `devctl-repo`; decision point)
- Settings struct: `RepoSettings`
  - `RepoRoot string` (`glazed.parameter:"repo-root"`)
  - `Config string` (`glazed.parameter:"config"`)
  - `Strict bool` (`glazed.parameter:"strict"`)
  - `DryRun bool` (`glazed.parameter:"dry-run"`)
  - `Timeout time.Duration` (`glazed.parameter:"timeout"`)

**Normalization/validation responsibilities (centralize here):**

- Default repo root to current directory if empty.
- Convert repo root to an absolute path.
- Default config to `config.DefaultPath(repoRoot)` if empty.
- Resolve relative config paths under repo root.
- Enforce `timeout > 0`.

This should replace the logic in `cmd/devctl/cmds/common.go` (`getRootOptions` + `requestMetaFromRootOptions`) over time.

### 4.3. Logging layer (reuse; already in use)

Devctl already uses Glazed’s logging layer integration pattern. Porting should preserve:

- `logging.AddLoggingLayerToRootCommand(rootCmd, "devctl")`
- `PersistentPreRunE: logging.InitLoggerFromCobra`

### 4.4. Command types: how to map devctl behaviors into Glazed interfaces

Most devctl commands are “imperative orchestration” and emit either:

- a JSON document (`plan`, `status`, `plugins list`, smoketests)
- a short “ok” (`up`, `down`)
- streamed text (`logs --follow`, `stream start`)
- interactive BubbleTea UI (`tui`)

That suggests using:

- `cmds.WriterCommand` (or `cmds.BareCommand`) for commands that write plain output
- optionally `cmds.GlazeCommand` for commands that want structured rows + output formats (not required for current devctl UX)

This ticket should decide a consistent approach:

- Keep devctl’s outputs exactly the same (mostly text/JSON) → favor `WriterCommand`.
- Add Glazed output formatting (`--output json/yaml/table`) as an enhancement → out of scope for now.

## 5. Per-command porting plan (flags → Glazed layers/parameters)

Each subsection describes how to represent the command’s flags in Glazed terms: which layer(s), which parameter definitions, which settings structs, and how to preserve defaults and validation.

### 5.1. Root flags (applies to most commands)

**Current flags (Cobra persistent):**

- `--repo-root string`
- `--config string`
- `--strict bool`
- `--dry-run bool`
- `--timeout duration`

**Glazed port:**

- Implement the `RepoSettings` custom layer (Section 4.2).
- Include this layer in every devctl Glazed command by default.
- Prefer accessing values via `parsedLayers.InitializeStruct("repo", &RepoSettings{})`.

### 5.2. `up`

**Current flags:**

- Local:
  - `--force bool`
  - `--skip-validate bool`
  - `--skip-build bool`
  - `--skip-prepare bool`
  - `--build-step []string` (repeatable)
  - `--prepare-step []string` (repeatable)
- Global: repo layer + logging

**Glazed port:**

- Command settings struct `UpSettings`:
  - `Force bool` → `force`
  - `SkipValidate bool` → `skip-validate`
  - `SkipBuild bool` → `skip-build`
  - `SkipPrepare bool` → `skip-prepare`
  - `BuildSteps []string` → `build-step`
  - `PrepareSteps []string` → `prepare-step`
- Layers:
  - `RepoSettings` layer
  - Glazed command settings layer (optional) for debugging output
- Behavioral notes:
  - Preserve interactive prompt behavior when state exists and stdin is a TTY.
  - Keep stdout output contract: `ok` on success; JSON for `--dry-run`.

### 5.3. `down`

**Current flags:** none (uses global flags).

**Glazed port:**

- No local parameters.
- Layers:
  - `RepoSettings` layer
- Output:
  - Preserve `ok` on success and state-missing errors.

### 5.4. `status`

**Current flags:**

- Local: `--tail-lines int` (default 25)

**Glazed port:**

- `StatusSettings`:
  - `TailLines int` → `tail-lines` (default 25)
- Layers:
  - `RepoSettings` layer
- Output:
  - Preserve JSON output shape.

### 5.5. `logs`

**Current flags:**

- Local:
  - `--service string` (required)
  - `--stderr bool`
  - `--follow bool`

**Glazed port:**

- `LogsSettings`:
  - `Service string` (required)
  - `Stderr bool`
  - `Follow bool`
- Layers:
  - `RepoSettings` layer
- Output:
  - Preserve raw log output behavior (text stream), and follow semantics.

### 5.6. `plan`

**Current flags:** none (uses global flags).

**Glazed port:**

- No local parameters.
- Layers:
  - `RepoSettings` layer
- Output:
  - Preserve `{}` output when no plugins are configured, and warning log.

### 5.7. `plugins list`

**Current flags:** none (uses global flags).

**Glazed port:**

- No local parameters (keep `plugins list` as subcommand).
- Layers:
  - `RepoSettings` layer
- Output:
  - Preserve JSON output shape (including handshake capability fields).

### 5.8. `stream start`

**Current flags:**

- Local:
  - `--plugin string` (plugin id)
  - `--op string` (required)
  - `--input-json string`
  - `--input-file string`
  - `--start-timeout duration` (default 2s)
  - `--json bool` (raw protocol JSON lines)

**Glazed port:**

- `StreamStartSettings`:
  - `PluginID string` (optional)
  - `Op string` (required)
  - `InputJSON string` (mutually exclusive with input-file)
  - `InputFile string` (mutually exclusive with input-json)
  - `StartTimeout time.Duration` (default 2s)
  - `JSON bool` (raw output)
- Layers:
  - `RepoSettings` layer
- Validation:
  - enforce the mutual exclusivity (`input-json` vs `input-file`) in a `Validate()` method.

### 5.9. `tui`

**Current flags:**

- Local:
  - `--refresh duration` (default 1s)
  - `--alt-screen bool` (default true)
  - `--debug-logs bool` (default false)

**Glazed port:**

- `TUISettings`:
  - `Refresh time.Duration`
  - `AltScreen bool`
  - `DebugLogs bool`
- Layers:
  - `RepoSettings` layer
- Output:
  - No structured output; wraps BubbleTea run loop.

### 5.10. Dev smoketests (`dev smoketest ...`)

**Current/target shape:**

- These are dev-only commands, moved under a `dev` group:
  - `dev smoketest` (root): `--plugin`, `--timeout`
  - `dev smoketest supervise`: `--timeout`
  - `dev smoketest e2e`: `--timeout`
  - `dev smoketest logs`: `--timeout`
  - `dev smoketest failures`: `--timeout`

**Glazed port:**

- Keep these Cobra-only (hidden under `dev`) initially, to keep the Glazed port focused on user-facing commands.
- If we port these later, keep them as Writer commands that emit JSON or `ok`, and use dedicated settings structs per command (these commands run on temp dirs and do not use `.devctl.yaml`).

### 5.11. Internal: `__wrap-service`

**Current flags:** see `cmd/devctl/cmds/wrap_service.go`.

**Glazed port:**

- This is not a “user” command; it should remain Cobra-only or a hidden Glazed command.
- Critical requirement: it must not be affected by global CLI startup logic (dynamic command discovery, slow help loading).

## 6. Implementation strategy and sequencing

This section proposes an order that keeps the port reviewable and reduces the chance of shipping regressions.

1. Add help system setup to devctl root (mirroring `glaze`).
2. Implement the `RepoSettings` custom layer + tests for normalization.
3. Port one simple command end-to-end (suggested: `status`), validating the layer wiring.
4. Port remaining core workflow commands (`plan`, `plugins list`, `logs`, `down`, `up`).
5. Port `tui` and `stream start`.
6. Keep smoketests as dev-only commands under `dev` (Cobra-only) and update docs/CI call sites accordingly.

## 7. Risks and “how we could miss this again”

This section captures the class of bug we recently saw: global startup behavior accidentally affecting commands that do not need plugins.

- Dynamic command discovery should not run for built-in verbs (`status`, etc.).
- Plugin handshakes must be newline-delimited JSON (NDJSON). A plugin printing `\\n` instead of an actual newline will hang until handshake timeout.
- In the Glazed port, avoid side effects during command registration; do work in `Run`/`RunInto...`, not during root command construction.

## 8. Review checklist (for each ported command)

- Flags and defaults match the existing Cobra command.
- Help output includes the same examples and behavior.
- Errors are actionable and preserve exit codes.
- No plugin processes are started for commands that don’t need plugins.
- Docs are loaded into help system and queryable.
