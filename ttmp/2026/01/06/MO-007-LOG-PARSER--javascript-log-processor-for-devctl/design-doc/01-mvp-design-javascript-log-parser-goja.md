---
Title: 'MVP Design: JavaScript Log Parser (goja)'
Ticket: MO-007-LOG-PARSER
Status: active
Topics:
    - backend
DocType: design-doc
Intent: long-term
Owners: []
RelatedFiles:
    - Path: devctl/cmd/devctl/cmds/logs.go
      Note: Existing devctl logs command; future integration point once standalone log-parse is stable
    - Path: devctl/cmd/log-parse/main.go
      Note: Standalone CLI for exercising the JS log parser engine
    - Path: devctl/pkg/logjs/module.go
      Note: Core goja module loader
    - Path: devctl/pkg/logjs/module_test.go
      Note: Unit tests locking in MVP normalization + timeout behavior
    - Path: go-go-goja/engine/runtime.go
      Note: Reference implementation of goja runtime + require + console wiring
    - Path: go-go-goja/modules/common.go
      Note: Native module registry pattern for goja_nodejs/require modules
    - Path: go.work
      Note: Workspace shows devctl can import go-go-goja
ExternalSources:
    - local:js-log-parser-spec.md
    - local:js-log-parser-examples.md
Summary: A small, synchronous, single-module JS log parser for devctl implemented with goja, reusing go-go-goja patterns where beneficial.
LastUpdated: 2026-01-06T18:46:13-05:00
WhatFor: Define an implementable MVP for JS-based log parsing with clear contracts, safety boundaries, and a Go implementation plan (standalone CLI first; devctl integration later).
WhenToUse: Use when implementing the MVP, reviewing API/UX choices, or extending scope beyond the MVP.
---



# MVP Design: JavaScript Log Parser (goja)

## Executive Summary

We want a JavaScript-based log processor that lets developers quickly parse and reshape raw log lines into structured events without inventing a new DSL. The imported “LogFlow” spec/examples describe a full platform; this design deliberately defines a much smaller MVP that we can ship reliably.

The MVP is:

- **Single-module**: one JS module (one `register({ ... })`) per run.
- **Synchronous**: all hooks are sync (no `Promise`, no async I/O).
- **Line-oriented**: input is a stream of lines; output is NDJSON of normalized events (0 or 1 event per line).
- **Small helper surface**: a tiny built-in helper module (`log.*`) for common parsing tasks (JSON, logfmt, regex capture, field access).
- **Safe-by-default runtime**: `goja` embedded in Go; no filesystem/exec/network helpers unless explicitly enabled.
- **Standalone first**: ship a tiny separate `cmd/log-parse` binary to exercise the JS parsing engine in isolation; integrate with `devctl` later.

Implementation uses:

- `github.com/dop251/goja` for the JS runtime and type bridging.
- A small embedded JS “prelude” to provide `globalThis.log` helpers (pure JS, no I/O).
- A minimal built-in `console` implementation (Go-backed) so scripts can use `console.log/error` without enabling Node-style `require()`.

Note: MVP intentionally does **not** enable Node-style `require()` because it can imply filesystem-backed module loading unless carefully sandboxed, and `goja_nodejs/console` requires `require()` to be enabled.

## Problem Statement

`devctl` already has a `logs` command (`devctl/cmd/devctl/cmds/logs.go`) that can dump or follow a service log file. However, raw logs are hard to sift through when you want:

- “only ERROR lines, but include parsed fields”
- “extract trace_id and print as JSON”
- “reformat heterogeneous logs into a single schema”
- “drop chatty lines, normalize timestamps/levels”

A log processor should:

- be easy to experiment with (edit JS file, rerun)
- have a stable and small contract
- be fast enough for `--follow`
- fail safely (no crashes due to script errors; clearly reported failures)

## Proposed Solution

### High-level architecture

```
                     ┌───────────────────────────┐
                     │ log-parse (new)            │
                     │ - reads stdin/file         │
                     │ - loads JS parser module   │
                     └─────────────┬─────────────┘
                                   │ lines
                                   ▼
                     ┌───────────────────────────┐
                     │ JS Parser Engine (new)     │
                     │ - goja runtime(s)          │
                     │ - loads user script        │
                     │ - calls hooks per line     │
                     └─────────────┬─────────────┘
                                   │ events (0..1 per line)
                                   ▼
                     ┌───────────────────────────┐
                     │ Output (MVP)               │
                     │ - NDJSON to stdout         │
                     │ - or pretty printing       │
                     └───────────────────────────┘
```

### JS module contract (MVP)

The user writes a JS file that calls a provided global:

```js
register({
  name: "my-parser",
  init(ctx) {},
  parse(line, ctx) { return null; },
  filter(event, ctx) { return true; },
  transform(event, ctx) { return event; },
  shutdown(ctx) {},
  onError(err, payload, ctx) {},
});
```

#### Required vs optional

- `name`: required string (used for diagnostics/telemetry).
- `parse(line, ctx)`: required.
- `filter`, `transform`, `init`, `shutdown`, `onError`: optional.

#### Sync-only semantics

All hooks must return synchronously. If a hook returns a Promise-like value, MVP treats it as an error (“async hooks not supported”) and calls `onError`.

This keeps the runtime simple and avoids the need for `goja_nodejs/eventloop` and a full event-loop integration.

### Hook semantics and return values

#### `parse(line, ctx) => EventLike | null`

Input:
- `line`: the raw log line (without trailing newline if we normalize; see Open Questions).
- `ctx`: the parse context.

Return:
- `null` / `undefined`: drop line.
- object: interpreted as an event.
- string: shorthand for `{ message: <string> }` (optional convenience; see Design Decisions).

#### `filter(event, ctx) => boolean`

Return `false` to drop the event. If `filter` throws, the event is dropped and `onError` is called (if defined).

#### `transform(event, ctx) => EventLike | null`

Return:
- object: new/modified event.
- `null` / `undefined`: drop.

### Context objects

The MVP provides a single context shape and reuses it across hooks; a `ctx.hook` field tells which hook is currently running (useful for `onError`).

```ts
type HookName = "init" | "parse" | "filter" | "transform" | "shutdown" | "onError";

type Context = {
  hook: HookName,
  source: string,        // service name or log path label
  lineNumber: number,    // monotonically increasing per input stream
  now: Date,             // snapshot of "now" for this invocation
  state: Record<string, any>, // per-runtime mutable state (shared across lines)
};
```

Notes:
- `state` is the MVP replacement for the spec’s “cache/state/ttl primitives”. It is in-memory only.
- If we add a worker pool, `state` is per worker runtime (not shared across workers).

### Event schema and normalization

The imported spec proposes a LogEvent with many fields. MVP defines a *normalized* event (what we output) and accepts a looser *EventLike* shape from JS.

#### Normalized event (what devctl emits)

```json
{
  "timestamp": "2026-01-06T18:00:00Z",
  "level": "INFO",
  "message": "something happened",
  "fields": {},
  "tags": [],
  "source": "service-name",
  "raw": "original line",
  "lineNumber": 42
}
```

#### Normalization rules (MVP)

Given `EventLike` returned from JS:

1. Start with defaults:
   - `fields = {}`
   - `tags = []`
   - `source = ctx.source`
   - `raw = original line`
   - `lineNumber = ctx.lineNumber`
2. If JS returns a string, treat as `{ message: <string> }`.
3. `message`:
   - if missing, set to `raw`.
4. `level`:
   - if missing, set to `"INFO"`.
5. `timestamp`:
   - if JS returns a `Date`, convert using `toISOString()` (stable string).
   - if JS returns a string, keep it as-is (no parsing in MVP).
   - if missing, omit it (do not set to `now`).
6. `fields`:
   - if returned `fields` is an object, shallow-merge it.
   - any extra top-level keys returned by JS (e.g. `trace_id`) are moved into `fields` unless already present there.

### Built-in helper module: `log`

Instead of the full “standard library” in the imported spec, MVP provides a tiny helper surface aimed at the common “parse a line” cases.

The helper surface is exposed as a global `log` object. MVP does not enable `require("log")`.

#### Helper API sketch (MVP)

```ts
type LogHelpers = {
  parseJSON(line: string): object | null;
  parseLogfmt(line: string): Record<string,string> | null;
  namedCapture(line: string, re: RegExp): Record<string,string> | null;
  extract(line: string, re: RegExp, group: number): string | null;
  field(obj: any, path: string): any; // dot-notation getter
};
```

Guiding principles:
- Keep it tiny and easy to implement in Go.
- Make failure return `null` rather than throwing (unless input is invalid type).
- Keep helpers pure and deterministic (no I/O).

### CLI: `log-parse` (MVP UX)

The MVP ships as a tiny separate binary so we can iterate quickly without touching `devctl` service supervision, log discovery, or interactive UX.

Example usage:

```bash
# Parse a file into NDJSON
log-parse --js ./parser.js --input ./app.log

# Parse stdin into NDJSON (exercise streaming behavior)
tail -f ./app.log | log-parse --js ./parser.js --source app
```

#### Proposed flags

- `--js <path>`: path to JS parser file (required).
- `--input <path>`: input file path; if omitted, read from stdin.
- `--source <label>`: source label stamped into events (defaults to input filename or `stdin`).
- `--format ndjson|pretty`: output format (default `ndjson`).
- `--js-timeout <duration>`: per-hook timeout (e.g. `50ms`, `200ms`), implemented via `Runtime.Interrupt`.
- `--workers <n>`: number of JS runtimes/workers (default `1`).
- `--unsafe-modules <list>`: opt-in enablement of dangerous modules (e.g. `fs`, `exec`), default empty.

### Future integration with devctl (post-MVP)

Once `log-parse` is working nicely and we are confident in runtime safety + normalization stability, we can integrate with `devctl logs` as an optional processing mode.

## Design Decisions

### 1) Keep the MVP hook set minimal

MVP hooks are `init/parse/filter/transform/shutdown/onError`. This matches the “spirit” of the imported spec while avoiding the large complexity of `aggregate` and `output` stages.

Rationale:
- Most developer log processing needs are solved by parse/filter/transform.
- Aggregation/output/destinations are a separate problem domain and often require async I/O.

### 2) Single module per run

The imported spec supports multiple modules and composition. MVP explicitly supports only one `register(...)` call.

Rationale:
- It drastically reduces configuration and lifecycle complexity.
- It avoids having to define a module composition order and event schema compatibility rules.

### 3) Synchronous hooks only (no Promises) in MVP

Rationale:
- `goja` can create Promises, but without a properly integrated event loop (e.g. `goja_nodejs/eventloop`) it’s easy to create “hung” runs or subtle concurrency bugs.
- Synchronous parsing is enough for the first release; async becomes relevant when we add destinations or external lookups.

### 4) Runtime safety: no `exec`/`fs` by default

`go-go-goja/engine.New()` enables all registered modules (currently including `exec`, `fs`, and `database`). For a log parser, this is an attractive nuisance.

MVP should:
- enable only `console` and our pure helper modules by default
- require explicit opt-in for any side-effectful module

Rationale:
- Even for a local dev tool, “JS script can run arbitrary commands” is a footgun.
- The MVP is about parsing; I/O and orchestration can be added later behind explicit flags.

### 5) Type bridging strategy: normalize at the boundary

We should treat JS output as untrusted “EventLike” and normalize it in Go into a strictly defined `Event` struct that the rest of `devctl` can rely on.

Rationale:
- Keeps the output stable even if scripts vary.
- Simplifies testing (golden NDJSON output).

### 6) Timeouts are required for `--follow`

We must prevent a user script from wedging a streaming log run. MVP should support per-hook deadlines via `(*goja.Runtime).Interrupt(v)`.

Rationale:
- A single infinite loop in JS should not freeze `tail -f ... | log-parse ...` or future `devctl logs --follow`.

### 7) Default ordering over throughput

Default `--workers=1` preserves log order. If we add multi-worker mode, we must either:
- accept reordering, or
- implement an ordering buffer keyed by line number.

MVP chooses ordering and simplicity.

## Go Implementation Plan (API sketches)

This section sketches the Go packages and key types. Names are suggestions; the goal is to make the design concrete enough that implementation work can start without re-litigating contracts.

### Package layout proposal

- `devctl/pkg/logjs` (new): goja integration, script loading, hook execution, helper module(s)
- `devctl/cmd/log-parse` (new): standalone CLI for exercising `devctl/pkg/logjs`
- (future) `devctl/cmd/devctl/cmds/logs.go`: integrate `--js` processing into existing `logs` command

### Core Go types (sketch)

```go
// devctl/pkg/logjs/types.go
package logjs

type Event struct {
	Timestamp  *time.Time              `json:"timestamp,omitempty"`
	Level      string                 `json:"level"`
	Message    string                 `json:"message"`
	Fields     map[string]any         `json:"fields"`
	Tags       []string               `json:"tags"`
	Source     string                 `json:"source"`
	Raw        string                 `json:"raw"`
	LineNumber int64                  `json:"lineNumber"`
}

type Context struct {
	Hook       string                 `json:"hook"`
	Source     string                 `json:"source"`
	LineNumber int64                  `json:"lineNumber"`
	Now        time.Time              `json:"-"`
	State      map[string]any         `json:"-"`
}
```

### Script/module loading (sketch)

```go
// devctl/pkg/logjs/loader.go
type Options struct {
	Timeout       time.Duration
	UnsafeModules []string
}

type Module struct {
	Name      string
	HasInit   bool
	HasFilter bool
	HasXform  bool
	HasOnErr  bool

	// One goja runtime per Module instance; not safe for concurrent use.
	vm  *goja.Runtime
	req *require.RequireModule

	// Hook callables extracted from registered config.
	initFn      goja.Callable
	parseFn     goja.Callable
	filterFn    goja.Callable
	transformFn goja.Callable
	shutdownFn  goja.Callable
	onErrorFn   goja.Callable

	state map[string]any
}

func LoadFromFile(ctx context.Context, scriptPath string, opts Options) (*Module, error)
```

Implementation outline:

1. Create `vm := goja.New()`.
2. Enable `console` (`goja_nodejs/console.Enable(vm)`).
3. Configure `require` registry and enable only allowed modules:
   - always: `log` helper module
   - optionally: `fs`, `exec`, etc if opt-in (either via our own module list or by reusing `go-go-goja/modules` selectively)
4. Install a global `register` Go function:
   - it receives one argument: the config object
   - it validates required fields (`name`, `parse`)
   - it stores the config object in Go for later hook extraction
5. Execute the user script:
   - either via `req.Require(scriptPath)` (Node semantics) or via `goja.Compile + vm.RunProgram`
6. Extract hook functions from the config object:
   - `goja.AssertFunction(config.Get("parse"))`
   - optional: `filter`, `transform`, etc.

### Hook invocation (sketch)

```go
func (m *Module) ProcessLine(ctx context.Context, line string, source string, lineNumber int64) (*Event, error)
```

Pseudocode:

```text
ctxObj := {
  hook: "parse",
  source,
  lineNumber,
  now: new Date(),
  state: <shared object>
}

eventLike := callHookWithTimeout(parseFn, [line, ctxObj])
if eventLike is null -> return nil

event := normalize(eventLike, source, line, lineNumber)

if filterFn exists:
  ctxObj.hook = "filter"
  keep := callHookWithTimeout(filterFn, [event, ctxObj])
  if !keep -> return nil

if transformFn exists:
  ctxObj.hook = "transform"
  eventLike2 := callHookWithTimeout(transformFn, [event, ctxObj])
  if null -> return nil
  event = normalize(eventLike2, source, line, lineNumber)

return event
```

### Timeouts with `Runtime.Interrupt`

The key constraint: all JS execution happens on the goroutine owning the runtime. A timeout must interrupt that goroutine, not “run JS elsewhere”.

Sketch:

```go
func (m *Module) callWithTimeout(ctx context.Context, hook string, fn goja.Callable, args ...goja.Value) (goja.Value, error) {
	timeout := m.opts.Timeout
	if timeout <= 0 {
		return m.call(hook, fn, args...)
	}

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	done := make(chan struct{})
	go func() {
		select {
		case <-timer.C:
			m.vm.Interrupt(errors.New("js hook timeout"))
		case <-ctx.Done():
			m.vm.Interrupt(ctx.Err())
		case <-done:
		}
	}()

	val, err := m.call(hook, fn, args...)
	close(done)
	m.vm.ClearInterrupt()
	return val, err
}
```

The actual `m.call` should use `m.vm.Try(...)` to capture JS exceptions:

- JS exceptions become `*goja.Exception` (via `Try`), which should be passed to `onError` (if present) and then treated as “drop event” (MVP default).

### Helper module implementation approach

We have two viable approaches:

1) Implement helpers as a global `log` object set via `vm.Set("log", ...)`.
2) Implement helpers as a `require("log")` module using the same loader signature as `go-go-goja/modules` (`func(*goja.Runtime, *goja.Object)`).

MVP can do both:
- set `globalThis.log = require("log")` during runtime initialization
- keep `require` available for future expansion

This matches the ergonomic needs of small scripts (global `log`) while keeping the long-term packaging story clean (module-based).

## Alternatives Considered

### 1) Implement a YAML/DSL log parser instead of JS

Rejected for MVP:
- DSLs become their own product; users immediately want “just one more feature”.
- JS gives power users what they want and is easy to prototype.

### 2) Use `jq` / `yq` / shell pipelines

Rejected as a replacement:
- Great for structured JSON logs, but not for mixed text logs.
- Harder to manage multi-step parsing and reuse across services.

### 3) Use a different embeddable language (Starlark, Lua)

Rejected for now:
- JS is already familiar to most developers.
- The workspace already includes `go-go-goja` and goja patterns we can reuse.

### 4) Use Node/V8 instead of goja

Rejected for MVP:
- Operational complexity (shipping Node, native V8 bindings, heavier dependency chain).
- goja is pure Go and simpler to embed in a Go CLI.

## Implementation Plan

1. Add tasks and refine MVP decisions (timestamp handling, top-level fields policy).
2. Implement `devctl/pkg/logjs` runtime + `register` + helper module(s).
3. Implement the standalone CLI `devctl/cmd/log-parse`:
   - read from stdin or `--input`
   - stream lines through `logjs.Module.ProcessLine`
   - emit events in `ndjson` or `pretty`
4. Add tests:
   - golden inputs/outputs for a few scripts (JSON parse, regex parse)
   - timeout test (infinite loop in JS)
5. (future) Integrate with `devctl logs` and service log discovery.
6. Add documentation/examples under `devctl/examples/` (optional for MVP).

## Open Questions

1) **Ordering vs workers**:
- If we add `--workers>1`, do we accept out-of-order output or buffer/reorder?

2) **Module loading**:
- Do we require `register(...)` or allow `module.exports = { ... }` too?
- MVP should start with one to avoid ambiguity.

## References

- Source notes (MVP trimming): `reference/01-source-notes-provided-spec-trimmed-for-mvp.md`
- Imported upstream artifacts:
  - `sources/local/js-log-parser-spec.md`
  - `sources/local/js-log-parser-examples.md`
- go-go-goja patterns and code:
  - `/home/manuel/workspaces/2026-01-06/log-parser-module/go-go-goja/engine/runtime.go` (`engine.New`)
  - `/home/manuel/workspaces/2026-01-06/log-parser-module/go-go-goja/modules/common.go` (`modules.NativeModule`)
- goja/goja_nodejs key APIs:
  - `goja.Compile`, `goja.Program`, `goja.Runtime.RunProgram`, `goja.Runtime.ExportTo`, `goja.Runtime.Interrupt`
  - `goja_nodejs/require.Registry`, `require.ModuleLoader`
- devctl integration point:
  - `/home/manuel/workspaces/2026-01-06/log-parser-module/devctl/cmd/devctl/cmds/logs.go` (future)
