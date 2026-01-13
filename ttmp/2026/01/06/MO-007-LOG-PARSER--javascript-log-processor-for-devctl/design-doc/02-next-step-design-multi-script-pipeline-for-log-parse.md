---
Title: 'Next Step Design: Multi-Script Pipeline for log-parse'
Ticket: MO-007-LOG-PARSER
Status: active
Topics:
    - backend
DocType: design-doc
Intent: long-term
Owners: []
RelatedFiles:
    - Path: devctl/cmd/log-parse/main.go
      Note: CLI will grow module list and validate/print-pipeline modes
    - Path: devctl/examples/log-parse
      Note: Example scripts; will expand to pipeline directory
    - Path: devctl/pkg/logjs/module.go
      Note: Current MVP; will be refactored into multi-module Pipeline
ExternalSources: []
Summary: Evolve log-parse from single-script to a multi-module fan-out runner where each self-contained module emits a tagged derived stream.
LastUpdated: 2026-01-06T21:24:45-05:00
WhatFor: Make log-parse more consequential by supporting many self-contained scripts that each emit a tagged derived event stream, without introducing JS-level code sharing (require) yet.
WhenToUse: Use when implementing the next iteration of log-parse/logjs or reviewing pipeline semantics and safety boundaries.
---


# Next Step Design: Multi-Script Pipeline for log-parse

## Executive Summary

The MVP `log-parse` supports a single JavaScript file that registers one module via `register({ ... })`. The next meaningful step is to support **many scripts** and a deterministic **fan-out runner**:

- **Many modules**: load multiple `register({ ... })` modules and run them on the same input stream.
- **Tagged outputs**: each module emits its own “derived” event stream, tagged with a module-defined tag (default: module name). The user can merge streams downstream or keep them separate.
- **Better ergonomics**: `--module` repeated flags, `--modules-dir`, and a config file mode.
- **Safer by default**: keep filesystem/exec capabilities off by default; `require()` remains off in this ticket. Shared reuse across parsers is deferred to a sandboxed `require()` design tracked in `MO-008-REQUIRE-SANDBOX`.
- **Operational clarity**: add `--validate` and `--print-pipeline` modes, and per-module stats.

This design keeps the runtime model simple and safe:

- one `goja.Runtime` per module (still `--workers=1` initially)
- each script is loaded into its own runtime; it calls `register(...)` exactly once
- the fan-out runner is implemented in Go (no JS-level module composition yet)

## Problem Statement

The MVP is useful for one-off experiments, but real log workflows quickly need:

1) **Many derived views of the same log stream** (errors, metrics, security, correlation IDs).
2) **Module isolation**: each script should be self-contained and not depend on shared globals (until we add sandboxed `require()`).
3) **Stable tagging**: downstream tools can group/merge results by tag.
4) **Confidence tools**: validate scripts before running; print what’s loaded; show per-module error counts and drops.

Without multi-script support, users either:

- jam everything into one large script (hard to maintain), or
- try to build their own “include” mechanism (which is basically `require()`).

We want the “consequential” value first: many scripts loaded by Go and executed independently, producing tagged outputs. Shared code reuse comes later via sandboxed `require()`.

## Proposed Solution

### Execution model: fan-out modules producing tagged streams

Instead of a “shared pipeline” where one module’s output becomes the next module’s input, we run modules independently on the same input stream. Each module may parse and then filter/transform its own derived event(s).

This keeps each `register()` self-contained and avoids cross-module coupling.

```
input lines
   │
   ▼
┌──────────────────────────────────────────────────────────┐
│ Module A (tag=a): parse/filter/transform => event|null    │
├──────────────────────────────────────────────────────────┤
│ Module B (tag=b): parse/filter/transform => event|null    │
├──────────────────────────────────────────────────────────┤
│ Module C (tag=c): parse/filter/transform => event|null    │
└──────────────────────────────────────────────────────────┘
   │
   ▼
output events (0..N per input line), each tagged
```

This matches the common case you described:
- the input stream is usually a single format
- multiple modules exist because they emit different *views* / *derived events* (e.g. “errors”, “metrics”, “security”)
- downstream merging is user-controlled (group by tag, or merge selectively)

### Multi-script loading UX

Add to `log-parse`:

- `--module <path>` (repeatable): load scripts in the order provided.
- `--modules-dir <dir>` (repeatable): load all `*.js` under dir (non-recursive MVP; recursive future), in lexicographic order.
- `--config <path>`: YAML/JSON config describing pipeline (optional; recommended once stable).

Examples:

```bash
# multiple self-contained derived-stream modules (explicit order for determinism)
log-parse \
  --module ./modules/errors.js \
  --module ./modules/metrics.js \
  --module ./modules/security.js \
  --input ./mixed.log

# load every script under a directory (lexicographic)
log-parse --modules-dir ./pipeline/ --input ./mixed.log
```

### Module contract updates (JS)

We keep the same `register({ ... })` contract, but add an explicit `tag` field for the “derived stream tag”:

```js
register({
  name: "errors",
  tag: "errors",       // optional; defaults to name

  parse(line, ctx) { /* ... */ },
});
```

Rules:

- `name` is required and must be unique within a run.
- `tag` is optional; if absent we set `tag = name`.
- `parse` is required for this multi-module iteration (every module is a parser/producer).
- `filter` and `transform` (optional) apply only to the derived event(s) produced by that module’s `parse`.

### Self-contained module semantics (important)

The operational goal is **one input stream, many derived tagged streams**:

- Each module receives the same input line (and context) and decides whether it recognizes it.
- Each module returns `null` when the line is “not for it”, and emits an event when it is.
- Modules do not depend on other modules’ output. There is no ordering dependency between modules for correctness.
- Code reuse across modules is intentionally deferred to sandboxed `require()` (ticket `MO-008-REQUIRE-SANDBOX`).

This fits the “one format, many outputs” usage:

- Input stream: usually a single format (e.g. devctl logs from one service).
- Output streams: different tags for different consumers (alerts vs metrics vs audit).
- Merging: downstream tools decide whether to merge or keep separate (`jq`, vector, fluent-bit, etc.).

### Tag injection rules (output schema)

For every emitted event from module `M` with `tag`:

- ensure `event.tags` contains `tag` (append if missing)
- set `event.fields._tag = tag` (unless already set)
- set `event.fields._module = M.name` (unless already set)

This yields a robust downstream grouping key even if the user script doesn’t set `tags` at all.

### Validation and introspection

Add commands/flags to make this safe and debuggable:

- `log-parse validate --module ...`:
  - load/compile scripts
  - verify exactly one `register(...)` call per script
  - verify unique names
  - verify that at least one module provides `parse`
  - print a pipeline summary (which hooks exist)
- `--print-pipeline`:
  - print module list + stage classification + execution order
- `--stats`:
  - print per-module `Stats` and global totals on exit (to stderr)

### Go implementation approach (incremental, minimal refactor)

Current MVP has `logjs.Module` representing one script + one module.
For “fan-out multi modules”, we introduce:

```go
// devctl/pkg/logjs/fanout.go (new)
type Fanout struct {
	Modules []*Module // existing logjs.Module, one per script
	Options FanoutOptions
}

type FanoutOptions struct {
	HookTimeout time.Duration
}
```

Key runtime decision:

-- **One runtime per module** (recommended for this iteration).

Rationale:
- modules are intended to be self-contained
- separate runtimes avoid global collisions between scripts
- it keeps the implementation closer to the existing MVP (`LoadFromFile` already builds a complete runnable module)

Downside:
- slightly more per-line overhead (N modules × N parse calls)

Mitigations:
- compile user scripts once per module (already done via `goja.Compile`)
- keep `--workers=1` initially; optimize later if needed

### Pseudocode: fan-out execution

```text
for each input line:
  for each module M:
    ev = M.ProcessLine(line, source, lineNumber)
    if ev != nil:
      injectTag(ev, M.tag, M.name)
      emit ev
```

### “Further things that make sense” (next step backlog)

Once multi-script is in place, the next high-leverage additions are:

1) **Config file** (`log-parse.yaml`) for repeatability:
   - list of modules (paths/directories)
   - timeout, output format
   - optional per-module enable/disable
   - optional tag overrides

2) **Module discovery and packaging**:
   - `--modules-dir` recursive
   - glob patterns
   - `log-parse init` to scaffold a pipeline directory

3) **Structured error output / dead-letter**:
   - `--errors ndjson` to emit error records as NDJSON (to stderr or a file)
   - include module name, hook name, source line number, raw line

4) **Per-module debug toggles**:
   - `--trace module=foo` to show inputs/outputs for that module (bounded sampling)

5) **Sandboxed require()** (tracked separately):
   - enable `require("./helpers")` safely (see `MO-008-REQUIRE-SANDBOX`)

## Design Decisions

### 1) Compose scripts in Go, not in JS

We intentionally avoid JS-to-JS module loading for now (no `require()`), and instead load scripts from Go and compose via deterministic pipeline rules.

Rationale:
- simpler security story
- simpler runtime behavior (no module cache surprises)
- fewer decisions required about filesystem access and path traversal

### 2) One runtime per module (fan-out)

Update: for this iteration we use **one runtime per module** (fan-out), not one shared runtime.

Rationale:
- keeps each `register()` self-contained (no shared globals)
- reduces accidental coupling between modules
- makes it easier to later sandbox `require()` per module

### 3) Per-module state objects

Rationale:
- scripts can safely keep counters/caches without stepping on each other
- we avoid global state collisions and encourage small modules

## Alternatives Considered

### 1) Require every script to provide the full hook set

Rejected:
- makes small “stage-only” scripts awkward
- encourages copy/paste and bloat

### 2) One goja runtime for all scripts

Deferred:
- feasible, but it increases the risk of global collisions (scripts accidentally share globals)
- it also invites “implicit coupling” (scripts can call each other’s globals), which we want to avoid until require() is sandboxed

### 3) Enable require() and let scripts compose themselves

Deferred:
- feasible, but needs sandboxing and explicit policy (tracked in `MO-008-REQUIRE-SANDBOX`)

## Implementation Plan

1. Extend CLI to accept repeated `--module` and `--modules-dir`.
2. Refactor `devctl/pkg/logjs`:
   - add a `Fanout` runner that runs multiple `Module`s per line and injects tags
3. Implement `validate`/`--print-pipeline`.
4. Add tests:
   - tag injection
   - per-module state isolation
5. Add example pipeline directory under `devctl/examples/log-parse/pipeline/`.

## Open Questions

1) Should we allow parse modules to emit multiple events per line (array return) in this phase?
2) Should tag be only in JS metadata, or also settable via CLI/config?
3) How do we want to represent “module version” and compatibility in the future?

## References

- MVP design: `design-doc/01-mvp-design-javascript-log-parser-goja.md`
- MVP engine: `/home/manuel/workspaces/2026-01-06/log-parser-module/devctl/pkg/logjs/module.go`
- MVP CLI: `/home/manuel/workspaces/2026-01-06/log-parser-module/devctl/cmd/log-parse/main.go`
- Require sandbox follow-up: `../../MO-008-REQUIRE-SANDBOX--sandbox-goja-nodejs-require-for-log-parse/design-doc/02-analysis-sandboxing-goja-nodejs-require.md`
