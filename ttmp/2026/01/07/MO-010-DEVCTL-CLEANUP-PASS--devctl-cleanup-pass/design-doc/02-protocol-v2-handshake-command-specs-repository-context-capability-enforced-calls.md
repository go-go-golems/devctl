---
Title: 'Protocol v2: handshake command specs + Repository context + capability-enforced calls'
Ticket: MO-010-DEVCTL-CLEANUP-PASS
Status: active
Topics:
    - backend
    - tui
    - refactor
    - ui-components
DocType: design-doc
Intent: long-term
Owners: []
RelatedFiles:
    - Path: devctl/cmd/devctl/cmds/dynamic_commands.go
      Note: |-
        Today calls commands.list; v2 design removes commands.list and reads command specs from handshake
        Remove commands.list; use handshake commands
    - Path: devctl/cmd/devctl/main.go
      Note: Runs dynamic discovery before all commands; a Repository bootstrap is the natural place to centralize discovery
    - Path: devctl/pkg/doc/topics/devctl-plugin-authoring.md
      Note: |-
        Authoring contract for plugins; must be updated for protocol v2 and command specs in handshake
        Update docs for protocol v2 and command specs in handshake
    - Path: devctl/pkg/engine/pipeline.go
      Note: |-
        Main consumer of runtime clients; needs updated request context construction via Repository/Meta
        Pipeline call sites will use Repository Meta for request ctx
    - Path: devctl/pkg/protocol/types.go
      Note: |-
        Handshake and capabilities schema; needs v2 changes to carry command specs directly
        Handshake and Capabilities schema to change in v2
    - Path: devctl/pkg/protocol/validate.go
      Note: Handshake validation rules (v2 plan may extend validation)
    - Path: devctl/pkg/runtime/client.go
      Note: |-
        Call/StartStream request building and SupportsOp behavior; Call should fail fast if op unsupported
        Client.Call should fail fast if op unsupported
    - Path: devctl/pkg/runtime/meta.go
      Note: |-
        RequestMeta used to construct protocol.RequestContext (repo_root/cwd/dry_run) without context.Value
        Replaces runtime/context.go helpers
    - Path: devctl/pkg/runtime/factory.go
      Note: Reads handshake and constructs runtime clients; must support v2 handshake and repo-bound meta
ExternalSources: []
Summary: 'Breaking (no-compat) design: Protocol v2 moves command specs into handshake capabilities, adds an explicit Repository container for repo/plugin context, and makes Client.Call fail fast for unsupported ops.'
LastUpdated: 2026-01-07T15:29:24.024119884-05:00
WhatFor: 'Eliminate startup stalls and context bugs by: removing commands.list, making repo/plugin context explicit, and making unsupported ops a local fast error.'
WhenToUse: When we are ready to intentionally break plugin protocol for a simpler, safer long-term architecture.
---


# Protocol v2: handshake command specs + Repository context + capability-enforced calls

## Executive Summary

This design is an intentionally breaking (“no backwards compatibility”) cleanup pass over devctl’s plugin interaction backend:

1) **Protocol v2**: eliminate `commands.list` and move full command specs into the handshake capabilities. Command wiring becomes “read handshake once”, not “call discovery op per plugin”.
2) **Repository container**: create and initialize a `Repository` (or `Workspace`) struct that owns repo-root, loaded config, discovered plugin specs, and request metadata used for all plugin calls. This removes `context.Value` usage for repo-root/cwd/dry-run.
3) **Capability-enforced calls**: make `runtime.Client.Call` fail fast with `E_UNSUPPORTED` if the op is not declared in handshake `capabilities.ops`. This prevents “forgot to gate” hangs without needing a separate invocation helper wrapper.

Together, these changes aim to reduce startup stalls, centralize correctness-critical bootstrapping, and make plugin calls safer by default.

## Problem Statement

### A) `commands.list` is fragile and expensive

Dynamic command discovery today:
- starts each plugin,
- calls `commands.list` (3 seconds timeout) for each plugin,
- uses `context.Background()` (drops `repo_root` from `request.ctx`),
- and is run before *every* command because `main.go` calls `AddDynamicPluginCommands` pre-Execute.

This produces:
- unnecessary “startup tax” (timeouts multiplied by N plugins),
- inability to run unrelated commands if a plugin blocks,
- and fatal interactions with internal commands like `__wrap-service` (wrapper delays cause supervisor ready-file timeouts).

### B) Repo/plugin context is “ambient” and easy to drop

Today, request metadata is pulled from `context.Context` values (`runtime/context.go`). This is fragile because:
- using `context.Background()` silently drops repo_root/cwd/dry-run,
- the presence/absence of metadata is invisible at the call site,
- and fixing it requires chasing down every place a context is constructed.

### C) Capability checks are not enforced at the runtime boundary

`runtime.Client.Call` will send any op string, even if the plugin never declared it in handshake capabilities.
If the plugin ignores unknown ops, devctl waits until a deadline expires.

This is a long-term correctness risk: every new call site is a chance to reintroduce hangs.

## Proposed Solution

The solution has three coordinated changes.

### 1) Protocol v2: command specs in handshake capabilities (remove `commands.list`)

#### 1.1 Change: handshake carries structured command specs

In v2, the handshake becomes the authoritative source for dynamic CLI command wiring.

Today:
- handshake `capabilities.commands` is `[]string` (names only),
- `commands.list` returns the structured list devctl needs (`name`, `help`, optional `args_spec`).

In v2:
- remove `commands.list` entirely,
- change `capabilities.commands` to carry a structured list.

Proposed schema (conceptual):

```go
type CommandSpec struct {
  Name     string       `json:"name"`
  Help     string       `json:"help,omitempty"`
  ArgsSpec []CommandArg `json:"args_spec,omitempty"`
}

type CommandArg struct {
  Name string `json:"name"`
  Type string `json:"type"`
}

type Capabilities struct {
  Ops      []string      `json:"ops,omitempty"`
  Streams  []string      `json:"streams,omitempty"`
  Commands []CommandSpec `json:"commands,omitempty"` // v2 breaking change
}
```

#### 1.2 Execution: keep `command.run` (for now)

This proposal only removes discovery (`commands.list`), not execution.

- `command.run` remains the operation that runs a command by name with argv + config.
- A later cleanup could remove `command.run` too by making each command its own op (e.g. `command.db-reset`), but that is optional.

#### 1.3 Devctl behavior in `dynamic_commands.go`

Dynamic command wiring becomes:
1) discover plugin specs,
2) start plugin process,
3) read handshake,
4) collect `capabilities.commands`,
5) close plugin,
6) register cobra commands.

This removes:
- one extra request per plugin,
- per-plugin “discovery op” timeouts,
- and the failure mode where a plugin never responds to `commands.list`.

### 2) Repository container: one discovery pass, explicit request metadata

Introduce a `Repository` (or `Workspace`) object that centralizes:
- repo root and config path
- loaded config
- discovered plugin specs
- index by plugin ID
- request metadata (meta) used to build `protocol.RequestContext`

Conceptual structure:

```go
type RequestMeta struct {
  RepoRoot string
  Cwd      string
  DryRun   bool
}

type Repository struct {
  Root     string
  Config   *config.Config
  Specs    []runtime.PluginSpec
  SpecByID map[string]runtime.PluginSpec
  Meta     RequestMeta
}
```

Bootstrap responsibilities (single pass per process):
- parse repo-root/config once
- load config once
- discover plugin specs once
- build `SpecByID` once
- set `Meta.RepoRoot` once

Dynamic commands benefit directly:
- provider lookup no longer needs “re-discover specs” inside the generated command `RunE` path.
- request metadata is explicit and cannot be lost via `context.Background()`.

### 3) Capability-enforced runtime calls: unsupported ops fail fast

Change runtime behavior so capability checks are not “optional” at call sites.

Rule:
- If an op is not listed in handshake `capabilities.ops`, then `Client.Call(ctx, op, ...)` returns `E_UNSUPPORTED` without writing to stdin.

This removes the “forgot to gate” failure class without requiring a separate invocation helper wrapper.

#### Do we still need a dedicated invocation helper layer?

Probably not.

- There is no dedicated invocation helper layer in the codebase today; nothing calls it.
- If `Client.Call` enforces capabilities, then “capability check + call” wrappers are redundant.
- Timeout policy can remain explicit via `context.WithTimeout`, and can be centralized in Repository helpers if desired (without changing runtime.Client).

## Design Decisions

### Decision 1: No backwards compatibility

Rationale:
- The cleanest model requires changing the handshake schema and deleting `commands.list`.
- Supporting both v1 and v2 would add branching in protocol, runtime, and docs.

### Decision 2: `commands` in handshake is authoritative

Rationale:
- Eliminates the “discovery op” class of timeouts/hangs.
- Keeps command wiring deterministic and as-fast-as-handshake.

### Decision 3: Repository is the central bootstrap and context carrier

Rationale:
- Avoids duplicated discovery logic.
- Makes request metadata explicit instead of ambient context values.
- Provides a single place to cache discovery results and enforce consistent context.

### Decision 4: `Client.Call` fails for unsupported ops

Rationale:
- Robust by default: new call sites cannot forget gating.
- Makes “unsupported” fast and local.

## Alternatives Considered

### Alternative A: Keep `commands.list` but gate it

This improves behavior but keeps the extra protocol surface and the startup call. It does not address “request ctx missing repo_root” unless all call sites are also fixed.

### Alternative B: Keep context.Value request metadata

This avoids an API shift but keeps the same fragility: repo_root can still be dropped silently.

### Alternative C: Keep permissive Call and add wrappers only

Wrappers are easy to bypass; runtime enforcement is more robust.

## Implementation Plan

This plan is intentionally “no-compat”: it updates the protocol and devctl in lockstep and then updates all in-repo example/test plugins to match.

### Phase 0: Doc + design cleanup (remove invocation helper layer)

- Remove all mentions of a dedicated invocation helper/function from MO-010 docs (design + reference). The v2 approach is: runtime enforces capabilities; call sites set deadlines explicitly.
- Ensure no Go code introduces a new invocation helper API as part of this cleanup pass.

### Phase 1: Protocol v2 handshake types + validation

- Update `devctl/pkg/protocol/types.go`:
  - add `ProtocolV2`
  - change `Capabilities.Commands` from `[]string` to structured `[]CommandSpec`
  - add shared `CommandSpec` / `CommandArg` types in `protocol`
- Update `devctl/pkg/protocol/validate.go`:
  - accept `protocol_version: "v2"`
  - add basic validation for `capabilities.commands` (no empty names; unique names; args fields are well-formed)

### Phase 2: Runtime capability enforcement (no call-site foot-guns)

- Update `devctl/pkg/runtime/client.go`:
  - `Call`: if `op` not in handshake `capabilities.ops`, return a fast local error with code `E_UNSUPPORTED` (no stdin write)
  - `StartStream`: same behavior (either gate on `ops` or on `streams`+`ops`, but be consistent and document it)
  - (Optional but recommended) introduce a typed error for protocol-level failures so call sites can detect `E_UNSUPPORTED` without string parsing

### Phase 3: Repository container + explicit request metadata (remove context.Value)

- Introduce `Repository` (or `Workspace`) type:
  - owns repo root, loaded config, discovered plugin specs, index by plugin id
  - owns explicit request meta (`repo_root`, `cwd`, `dry_run`)
- Replace `runtime/context.go` and `requestContextFrom(ctx)`-via-context-values:
  - request deadlines still come from `ctx.Deadline()`
  - repo/cwd/dry-run come from Repository.Meta (or client-local meta captured at start time)
- Update CLI + TUI bootstraps to build/use Repository:
  - CLI: `devctl/cmd/devctl/main.go` + `devctl/cmd/devctl/cmds/*`
  - TUI: `devctl/pkg/tui/action_runner.go` (and anywhere else starting runtime clients)

### Phase 4: Dynamic commands from handshake (delete `commands.list`)

- Update `devctl/cmd/devctl/cmds/dynamic_commands.go`:
  - start plugin, read handshake, read `capabilities.commands`, close plugin
  - register cobra commands from handshake command specs
  - remove the `commands.list` call and its 3s per-plugin timeout entirely
  - remove re-discovery inside generated `RunE`; use Repository’s cached `SpecByID` for provider lookup

### Phase 5: Update plugin authoring docs, examples, and fixtures (in repo)

- Update docs:
  - `devctl/pkg/doc/topics/devctl-plugin-authoring.md` (rename to protocol v2; update handshake schema; remove `commands.list`; document structured `capabilities.commands`)
  - `devctl/docs/plugin-authoring.md` (same)
- Update in-repo example + test plugins to v2:
  - `devctl/examples/plugins/python-minimal/plugin.py`
  - `devctl/testdata/plugins/*/plugin.py` (protocol_version + capability shapes)
  - convert “command plugin” to advertise commands via handshake and stop implementing `commands.list`

### Phase 6: Tests + acceptance

- Update Go tests that assume protocol v1 handshakes.
- Add/adjust tests to cover:
  - handshake v2 parsing
  - runtime `Call` fast-failing unsupported ops (no timeout wait)
  - dynamic command wiring does not call `commands.list`
- Acceptance criteria:
  - `go test ./...` passes
  - `devctl plugins list` prints v2 handshake and structured command specs
  - `devctl` startup no longer issues `commands.list` requests (confirmed by fixture / test plugin)
  - no docs mention a dedicated invocation helper/function

## Open Questions

1) Should command execution remain `command.run`, or should each command be a distinct op?
2) Should the Repository own long-lived plugin processes (pool), or remain “start per run” as today?
3) How strict should handshake validation be for command specs (unique names, non-empty)?
4) How should devctl handle command name collisions across plugins (keep-first vs strict error)?

## References

- Baseline capability design (v1): `devctl/ttmp/2026/01/07/MO-010-DEVCTL-CLEANUP-PASS--devctl-cleanup-pass/design-doc/01-safe-plugin-invocation-and-capability-enforcement-long-term-pattern.md`
- Current dynamic commands implementation: `devctl/cmd/devctl/cmds/dynamic_commands.go`
- Protocol schemas/validation: `devctl/pkg/protocol/types.go`, `devctl/pkg/protocol/validate.go`
