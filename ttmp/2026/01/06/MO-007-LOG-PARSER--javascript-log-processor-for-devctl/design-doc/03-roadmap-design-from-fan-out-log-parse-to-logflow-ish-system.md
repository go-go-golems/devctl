---
Title: 'Roadmap Design: From Fan-Out log-parse to LogFlow-ish System'
Ticket: MO-007-LOG-PARSER
Status: active
Topics:
    - backend
DocType: design-doc
Intent: long-term
Owners: []
RelatedFiles:
    - Path: devctl/cmd/log-parse/main.go
      Note: CLI will gain --module/--modules-dir/validate/print-pipeline/stats per roadmap
    - Path: devctl/pkg/logjs/module.go
      Note: Current MVP module contract and context; roadmap proposes fan-out runner + stdlib expansions on top
    - Path: devctl/ttmp/2026/01/06/MO-007-LOG-PARSER--javascript-log-processor-for-devctl/design-doc/02-next-step-design-multi-script-pipeline-for-log-parse.md
      Note: Alignment target for fan-out semantics
    - Path: devctl/ttmp/2026/01/06/MO-007-LOG-PARSER--javascript-log-processor-for-devctl/sources/local/js-log-parser-spec.md
      Note: Upstream spec being decomposed into phased roadmap
    - Path: devctl/ttmp/2026/01/06/MO-007-LOG-PARSER--javascript-log-processor-for-devctl/tasks.md
      Note: Task breakdown for roadmap phases
ExternalSources: []
Summary: ""
LastUpdated: 2026-01-06T22:08:44.688569922-05:00
WhatFor: ""
WhenToUse: ""
---


# Roadmap Design: From Fan-Out log-parse to LogFlow-ish System

## Executive Summary

The imported “LogFlow” spec (`sources/local/js-log-parser-spec.md`) describes a full-featured JavaScript log processing system (stdlib helpers, state primitives, multiline buffering, aggregation windows, outputs, error handling, multi-worker execution).

Our current `log-parse`/`logjs` MVP intentionally implements a **small** subset: a single JS file, a single registered module, and synchronous `parse` + optional `filter`/`transform`, emitting normalized events to stdout.

This document proposes a pragmatic path from the MVP to a more “LogFlow-ish” system while staying aligned with the already-agreed next-step design:

- Keep the **fan-out** model from `design-doc/02-next-step-design-multi-script-pipeline-for-log-parse.md`: **one input stream → many self-contained modules → many tagged derived streams**.
- Build “full spec” features **around** that model (stdlib, multiline, aggregation shape, dead-letter, config), without introducing JS-to-JS code sharing (`require()`) yet.
- Keep safety boundaries explicit: *no filesystem/network/exec access by default*; sandboxed `require()` remains a follow-up (`MO-008-REQUIRE-SANDBOX`).

## Problem Statement

We have two forces to reconcile:

1) The spec is intentionally ambitious (a full pipeline system with rich helpers and state primitives).
2) The direction we chose for the next iteration is intentionally minimal and safe: load many scripts and run them independently on the same input, producing **tagged derived outputs**.

If we “implement the spec” naively, we risk:

- drifting into a stage/pipeline system with implicit coupling between modules
- enabling dangerous capabilities (filesystem/network via `require()` or helper modules)
- adding async/Promise semantics that don’t map cleanly to goja
- building a large surface area before the multi-module runtime is stable

We need a plan that:

- remains aligned with the fan-out, self-contained module model
- selects a **useful** subset of the spec in clear phases
- provides a durable API structure for later expansion (including sandboxed `require()`)

### Scope and Non-Goals (for the next phases)

**In-scope, near-term (after MVP):**
- Multi-module fan-out execution (load many scripts; run all modules per line; tag derived outputs).
- Introspection tooling (`validate`, `--print-pipeline`, per-module stats).
- “Stdlib v1” covering high-value helpers from the spec:
  - parsing helpers (`parseJSON`, `parseLogfmt`, simple `parseKeyValue`, regex capture helper)
  - event/tag helpers (`field`/`getPath`, `addTag`/`hasTag`)
  - basic time/numeric helpers (`parseTimestamp`, `toNumber`)
- Multiline buffering (per-module, single-worker) via `createMultilineBuffer`.
- Error isolation per module and an optional dead-letter stream.

**Out-of-scope for now (but designed for):**
- Sandboxed JS `require()` (tracked in `MO-008-REQUIRE-SANDBOX`).
- Network outputs (webhooks, databases, etc.) and retry/backoff.
- Multi-worker shared state (“remember/recall shared across workers”).
- Async hooks / Promise-based user code.
- Full aggregation windows (we design the shape, implement later once fan-out is stable).

## Proposed Solution

### 1) Keep the “fan-out tagged derived streams” execution model

This is the core alignment requirement with `design-doc/02-next-step-design-multi-script-pipeline-for-log-parse.md`.

Instead of chaining stages, we run each module independently:

```
                   same input line
                         │
                         ▼
┌───────────────┐  ┌───────────────┐  ┌───────────────┐
│ Module:errors │  │ Module:metrics│  │ Module:audit  │
│ parse/filter/ │  │ parse/filter/ │  │ parse/filter/ │
│ transform     │  │ transform     │  │ transform     │
└───────┬───────┘  └───────┬───────┘  └───────┬───────┘
        │                  │                  │
        ▼                  ▼                  ▼
  tagged derived      tagged derived      tagged derived
  event stream        event stream        event stream
```

Operationally, this matches typical “one log format, many derived views” workflows:
- one service’s logs in a single format
- multiple modules for different derived outputs (errors, perf, security, correlation IDs)
- downstream merges are user-controlled (by tag)

### 2) Extend the module contract to support 0..N derived events per line

We extend `parse`/`transform` returns to allow:
- `null` / `undefined`: emit nothing
- `string`: shorthand for `{ message: string }` (already supported)
- `object`: a single event-like object (already supported)
- `array`: a list of event-like objects/strings/nulls (new)

This unlocks:
- multiline buffering (a flush emits one event after many input lines)
- “explode” operations (one line yields multiple derived records)

### 3) Add “stdlib v1” as Go-injected helpers (no require yet)

The spec’s biggest usability claim is “batteries included” (`parseLogfmt`, `field`, `parseTimestamp`, etc.). We add a minimal standard library surface area injected into each module runtime as safe globals (pure/bounded; no filesystem/network).

### 4) Add multiline buffering as a per-module helper

Implement `createMultilineBuffer` with clear **single-worker** semantics and per-module lifetime (usually created in `init` and stored in module state/cache).

### 5) Improve observability: error isolation + optional dead-letter output

Add an optional machine-readable error stream (`--errors`) that emits structured error records, in addition to invoking `onError`.

## Detailed Design

### A. Module contract (JS) — aligned with fan-out

```js
register({
  name: "errors",
  tag: "errors", // optional; default == name

  // Required (fan-out producer): return null | event | [events]
  parse(line, ctx) {},

  // Optional: applies only to this module’s derived events
  filter(event, ctx) { return true; },
  transform(event, ctx) { return event; },

  init(ctx) {},
  shutdown(ctx) {},
  onError(err, payload, ctx) {},
});
```

In this phase, every module is a producer (must provide `parse`). “Stage-only modules” can be added later, but they complicate semantics (they need input events, not raw lines).

### B. Context object — reconcile MVP with spec ergonomics

Today `devctl/pkg/logjs/module.go` provides `ctx.state` as a mutable object. The spec uses `context.cache.set/get`.

Proposal:
- Keep `ctx.state` (existing behavior).
- Add `ctx.cache` as a JS `Map` (or Map-like wrapper) so spec-style patterns work:

```js
ctx.cache.set("headers", headers);
const headers = ctx.cache.get("headers");
```

This is per-module, per-runtime state. Multi-worker shared state remains out-of-scope for now.

### C. Output schema and tagging rules

We keep the normalized event schema (`devctl/pkg/logjs/types.go`) and enforce tagging on every derived event:

For module `M` with `name` and `tag`:
- Ensure `event.tags` contains `tag`
- Ensure `event.fields._tag == tag` unless already set
- Ensure `event.fields._module == M.name` unless already set

### D. Stdlib v1 design — where helpers live

We want the spec’s helpers, but without `require()` yet. Options:
1) globals (`parseJSON(...)`)
2) namespaced object (`log.parseJSON(...)`) (recommended)
3) sandboxed `require()` modules (later; MO-008)

For this phase we recommend (2):

```js
log.parseJSON(line)
log.parseLogfmt(line)
log.parseKeyValue(line, " ", "=")
log.capture(line, /.../)
log.getPath(obj, "a.b.c")
log.field(event, "fields.trace_id")
log.addTag(event, "errors")
log.hasTag(event, "errors")
log.parseTimestamp(value)
log.toNumber(value)
log.createMultilineBuffer({ ... })
```

Implementation direction:
- extend the existing helper injection in `devctl/pkg/logjs/helpers.go` (JS prelude + Go-bound functions).

### E. Multi-module loading and validation (CLI)

Loading:
- `--module <path>` repeated: load in CLI flag order
- `--modules-dir <dir>`: load `*.js` lexicographically (non-recursive first)

Validation:
- compile each script
- run it in an isolated runtime with a `register()` hook
- ensure exactly one register call per script
- require unique module names
- verify `parse` exists
- print a module summary (name, tag, hooks present, source path)

### F. API sketches (Go)

Today:
- `devctl/pkg/logjs/module.go` defines `type Module` with `LoadFromFile`, `ProcessLine`, `Close`

Add:

```go
// devctl/pkg/logjs/fanout.go
type Fanout struct {
	Modules []*Module
}

func LoadModules(paths []string, opts Options) ([]*Module, error)
func (f *Fanout) ProcessLine(ctx context.Context, line, source string, lineNumber int64) ([]*Event, error)
func (f *Fanout) Close(ctx context.Context) error
```

Error record sketch (optional stream):

```go
type ErrorRecord struct {
	Module    string  `json:"module"`
	Tag       string  `json:"tag"`
	Hook      string  `json:"hook"`
	Source    string  `json:"source"`
	Line      int64   `json:"lineNumber"`
	Timeout   bool    `json:"timeout"`
	Message   string  `json:"message"`
	RawLine   *string `json:"rawLine,omitempty"`
}
```

### G. Where “full spec” features fit in the fan-out model

The spec includes `aggregate(events, context)` and `output(event, context)` which assume a pipeline after transform.

In a fan-out system there are two clean interpretations:

1) **Per-module aggregation**: module consumes its own derived events and periodically emits “aggregate events” (still as NDJSON) tagged e.g. `metrics.p95`.
2) **Go-level sinks**: Go (not JS) handles outputs/destinations from config; JS remains for parse/derive/enrich.

Recommendation for now:
- Keep JS side-effect free (no network/FS).
- Treat “output” as emitting additional events; if/when we add sinks, do it in Go config, not in JS.

## Design Decisions

### 1) Fan-out over stage chaining

Fan-out matches the desired user experience: each module is self-contained and produces a tagged derived stream. Stage chaining encourages shared globals and ordering dependencies.

### 2) No JS-level require() yet

Code reuse is important, but enabling filesystem module loading is a security boundary. Keep it separate in `MO-008-REQUIRE-SANDBOX`.

### 3) Stdlib as safe helpers

We build the spec’s helper surface incrementally with priorities:
- high utility primitives first
- deterministic behavior
- bounded memory usage

### 4) Single-worker first for multiline + determinism

Multiline buffering depends on strict ordering. We keep it single-worker until semantics are stable.

## Alternatives Considered

### A) One shared goja runtime for all scripts

Pros: less overhead; shared helpers.
Cons: scripts can share globals and accidentally couple; weaker isolation boundary.

### B) Stage/pipeline chaining (LogFlow architecture as written)

Pros: matches the upstream spec diagram directly.
Cons: conflicts with “each module emits its own tagged stream”; encourages brittle ordering dependencies.

### C) Implement stdlib purely in JS

Pros: simplest to ship.
Cons: harder to bound memory/TTL and keep fast/stable; harder to test at the Go boundary.

## Implementation Plan

This aligns with tasks in `tasks.md` (IDs 14–26) and complements the open “next step” tasks (IDs 8–10).

### Phase 1 — multi-module fan-out foundation
- Implement `logjs` fan-out runner and tag injection (task 14).
- Extend `cmd/log-parse` with `--module` / `--modules-dir` and deterministic load order (task 15).
- Add `validate`, `--print-pipeline`, and per-module `--stats` (tasks 16–17).
- Add multi-module unit tests (task 24).

### Phase 2 — multi-event returns + error observability
- Allow `parse`/`transform` to return arrays (task 18).
- Add optional dead-letter output stream (`--errors`) (task 19).

### Phase 3 — stdlib v1
- Implement parsing helpers (task 20).
- Implement event/tag helpers (task 21).
- Implement time + numeric helpers (task 22).

### Phase 4 — multiline v1
- Implement `createMultilineBuffer` (task 23).

### Phase 5 — examples + scripts
- Add a “many modules” example directory (task 25).
- Add scripts to run demo + validation loops (task 26; plus earlier tasks 12/13).

## Open Questions

1) Should we accept “stage-only modules” (filter/transform without parse) in the fan-out model, or keep “parse required” as an invariant for simplicity?
2) What is the canonical tag key for downstream routing: `event.tags[]` vs `event.fields._tag` (or both)?
3) How should multiline buffers behave at EOF: do we auto-flush on shutdown, and should that emit an event even if incomplete?
4) When we add multi-worker later, what state is shared vs per-worker? (Spec says remember/recall shared; likely a separate ticket.)

## References

- Upstream spec (design input): `sources/local/js-log-parser-spec.md`
- Fan-out next-step design (alignment target): `design-doc/02-next-step-design-multi-script-pipeline-for-log-parse.md`
- MVP design (current behavior + contracts): `design-doc/01-mvp-design-javascript-log-parser-goja.md`
- MVP engine: `/home/manuel/workspaces/2026-01-06/log-parser-module/devctl/pkg/logjs/module.go`
- Require sandbox follow-up (deferred): `../../MO-008-REQUIRE-SANDBOX--sandbox-goja-nodejs-require-for-log-parse/design-doc/02-analysis-sandboxing-goja-nodejs-require.md`
