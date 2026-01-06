---
Title: Diary
Ticket: MO-007-LOG-PARSER
Status: active
Topics:
    - backend
DocType: diary
Intent: long-term
Owners: []
RelatedFiles:
    - Path: devctl/cmd/log-parse/main.go
      Note: Standalone MVP binary implemented in Step 6 (commit 9b86bc031454347e03d78b237e817c735dd50392)
    - Path: devctl/pkg/logjs/module.go
      Note: Implemented in Step 6 (commit 9b86bc031454347e03d78b237e817c735dd50392)
    - Path: devctl/ttmp/2026/01/06/MO-007-LOG-PARSER--javascript-log-processor-for-devctl/design-doc/01-mvp-design-javascript-log-parser-goja.md
      Note: Primary design doc produced during diary steps
    - Path: devctl/ttmp/2026/01/06/MO-007-LOG-PARSER--javascript-log-processor-for-devctl/index.md
      Note: Ticket overview and navigation
    - Path: devctl/ttmp/2026/01/06/MO-007-LOG-PARSER--javascript-log-processor-for-devctl/reference/01-source-notes-provided-spec-trimmed-for-mvp.md
      Note: MVP scope trimming decisions captured here
ExternalSources: []
Summary: ""
LastUpdated: 2026-01-06T18:56:23-05:00
WhatFor: ""
WhenToUse: ""
---



# Diary

## Goal

Create and maintain a precise implementation diary for `MO-007-LOG-PARSER`, capturing decisions and research (especially around `goja` and `go-go-goja`) while we design an intentionally small MVP subset of the provided “LogFlow” spec and examples.

## Step 1: Ticket Setup and Source Import

Created a new `docmgr` ticket workspace and imported the two provided Markdown artifacts as external sources. This establishes a stable, versioned “input corpus” for the log-parser MVP design, while keeping the ticket index automatically updated with the imported sources.

This step intentionally did not attempt to implement any code. The goal was to get the documentation workspace, the sources, and the foundational documents (diary + design-doc + reference) in place so later steps can focus on design and (eventually) implementation without losing provenance.

### What I did
- Ran `docmgr ticket create-ticket --ticket MO-007-LOG-PARSER --title "JavaScript log processor for devctl" --topics backend`.
- Added initial documents:
  - `docmgr doc add --ticket MO-007-LOG-PARSER --doc-type diary --title "Diary"`
  - `docmgr doc add --ticket MO-007-LOG-PARSER --doc-type reference --title "Source Notes: Provided Spec (trimmed for MVP)"`
  - `docmgr doc add --ticket MO-007-LOG-PARSER --doc-type design-doc --title "MVP Design: JavaScript Log Parser (goja)"`
- Imported the provided artifacts into `sources/local/`:
  - `docmgr import file --ticket MO-007-LOG-PARSER --file /tmp/js-log-parser-spec.md --name "js-log-parser-spec"`
  - `docmgr import file --ticket MO-007-LOG-PARSER --file /tmp/js-log-parser-examples.md --name "js-log-parser-examples"`

### Why
- The MVP design must explicitly reference the provided spec/examples, but we do not want those to “become the spec”. Importing them as sources makes it easy to quote/trim and to keep the MVP scope separate from the exhaustive wish-list.
- Creating the docs early ensures we can write down decisions while they are fresh, and use `docmgr` search/graph tooling during design.

### What worked
- `docmgr` created the workspace at `devctl/ttmp/2026/01/06/MO-007-LOG-PARSER--javascript-log-processor-for-devctl`.
- `docmgr import file` copied both `/tmp/` files into `sources/local/` and updated the ticket `index.md` `ExternalSources` list automatically.

### What didn't work
- `docmgr` reported: “No guidelines found for doc-type diary.” This is harmless but means the diary formatting must be self-imposed (we follow the strict template in this document).

### What I learned
- This repository’s `docmgr` docs root is configured via `.ttmp.yaml` to `devctl/ttmp`, so all tickets live under `devctl/ttmp/YYYY/MM/DD/...`.

### What was tricky to build
- N/A (workspace creation and imports only).

### What warrants a second pair of eyes
- N/A (no design decisions yet; only bookkeeping).

### What should be done in the future
- Distill an explicit MVP scope from the imported “LogFlow” spec and examples, and record what is intentionally out-of-scope.

### Code review instructions
- Start at `devctl/ttmp/2026/01/06/MO-007-LOG-PARSER--javascript-log-processor-for-devctl/index.md`.
- Confirm the imported source artifacts exist under `devctl/ttmp/2026/01/06/MO-007-LOG-PARSER--javascript-log-processor-for-devctl/sources/local/`.
- Use `docmgr ticket tickets` and `docmgr doc list --ticket MO-007-LOG-PARSER` to confirm the workspace and docs are indexed.

### Technical details
- Imported sources:
  - `/home/manuel/workspaces/2026-01-06/log-parser-module/devctl/ttmp/2026/01/06/MO-007-LOG-PARSER--javascript-log-processor-for-devctl/sources/local/js-log-parser-spec.md`
  - `/home/manuel/workspaces/2026-01-06/log-parser-module/devctl/ttmp/2026/01/06/MO-007-LOG-PARSER--javascript-log-processor-for-devctl/sources/local/js-log-parser-examples.md`

## Step 2: Study goja + go-go-goja and Trim the Spec to an MVP Shape

Reviewed the imported “LogFlow” spec and examples to understand the intended hook model and helper surface area, then studied the local `go-go-goja` module (and key `goja` APIs) to ground the MVP design in what is straightforward to implement and safe to run inside a long-lived Go process.

The key outcome of this step is a concrete mental model of how to host JavaScript in Go using `goja`, how to expose Go functionality to JS (both as globals and as Node-style `require()` modules via `goja_nodejs/require`), and what we must *not* do in an MVP (async hooks, multi-module pipelines, heavy “standard library”, distributed state) if we want to ship something reliable quickly.

### What I did
- Scanned the imported “LogFlow” spec for the major sections (hooks, event schema, standard library, batching, concurrency, error handling) and noted that it is far beyond what we should ship first.
- Read `go-go-goja` documentation and code to understand the recommended integration pattern:
  - `/home/manuel/workspaces/2026-01-06/log-parser-module/go-go-goja/README.md`
  - `/home/manuel/workspaces/2026-01-06/log-parser-module/go-go-goja/engine/runtime.go` (`engine.New()` wiring)
  - `/home/manuel/workspaces/2026-01-06/log-parser-module/go-go-goja/modules/common.go` (module registry + documentation pattern)
- Looked at an example native module implementation to validate how Go→JS exports look in practice:
  - `/home/manuel/workspaces/2026-01-06/log-parser-module/go-go-goja/modules/fs/fs.go`
- Queried `goja` public APIs to confirm the primitives we’ll rely on:
  - `go doc github.com/dop251/goja.Runtime` (notably `RunProgram`, `ExportTo`, `ToValue`, `Interrupt`)
  - `go doc github.com/dop251/goja.Compile` / `go doc github.com/dop251/goja.Program` (compile once, reuse across runtimes)
  - `go doc github.com/dop251/goja.AssertFunction` / `go doc github.com/dop251/goja.Exception`
  - `go doc github.com/dop251/goja_nodejs/require` (registry and module loader shape)
- Located the current `devctl logs` command implementation to understand where a log parser would integrate:
  - `/home/manuel/workspaces/2026-01-06/log-parser-module/devctl/cmd/devctl/cmds/logs.go`

### Why
- The provided spec is a good “north star”, but implementing it literally would require async I/O, batching, worker scheduling, and a large helper library. MVP should instead focus on “line → event (or drop)” with clear contracts.
- `go-go-goja` is already vendored into the workspace (`go.work` includes it), so reusing its `engine.New()` and module patterns reduces risk and prevents reinvention.

### What worked
- `go-go-goja/engine.New()` already:
  - creates a `*goja.Runtime`
  - enables Node-style `require()` via `goja_nodejs/require.Registry`
  - enables a global `console` object (`goja_nodejs/console`)
  - auto-registers known modules via blank imports + `modules.EnableAll(reg)`
- `goja.Compile()` + `Runtime.RunProgram()` confirm a clean performance path: compile JS sources once and run them in per-worker runtimes without recompiling per line.
- `Runtime.Interrupt(v)` exists, enabling an MVP timeout mechanism (interrupt long-running JS).

### What didn't work
- N/A (research only; no runtime prototypes executed yet).

### What I learned
- `*goja.Runtime` is not safe for concurrent use; it should be owned by a single goroutine (or accessed via a scheduler/event loop). This strongly shapes the MVP concurrency plan (worker pool, one runtime per worker).
- `goja_nodejs/require` provides a clean, composable way to ship a “standard library” as Go-backed modules without polluting the global JS namespace; this maps well to “log helpers” as `require("log")`, `require("parse")`, etc.
- `goja.Program` is runtime-independent and can be used concurrently, which makes it a good artifact for caching compiled user scripts.

### What was tricky to build
- The tricky part is not “running JavaScript”, it’s defining safe boundaries:
  - timeouts (`Runtime.Interrupt`) and what failure mode looks like (drop line, emit error event, stop processing)
  - type conversion rules between JS values and Go structs (`Runtime.ExportTo`)
  - stability and determinism (no hidden global state shared across workers)

### What warrants a second pair of eyes
- The timeout and sandbox story: we should confirm `Runtime.Interrupt` usage patterns and how to prevent runaway scripts without corrupting the runtime state.
- The boundary between “helpers as globals” vs “helpers as require modules” (MVP ergonomics vs long-term maintainability).

### What should be done in the future
- Write down the MVP scope explicitly (what hooks, what event schema, what helper functions).
- Produce a concrete API sketch for a Go `logparser` package and a `devctl` command integration.
- Add a small prototype to validate the proposed JS contract end-to-end (script load → parse line → output event).

### Code review instructions
- Review `go-go-goja` integration points:
  - `/home/manuel/workspaces/2026-01-06/log-parser-module/go-go-goja/engine/runtime.go`
  - `/home/manuel/workspaces/2026-01-06/log-parser-module/go-go-goja/modules/common.go`
- Review current log plumbing in `devctl`:
  - `/home/manuel/workspaces/2026-01-06/log-parser-module/devctl/cmd/devctl/cmds/logs.go`

### Technical details
- Key `goja` symbols that the MVP design will rely on:
  - `goja.Compile` / `goja.Program` (compile once)
  - `(*goja.Runtime).RunProgram` / `RunScript` (execute)
  - `goja.AssertFunction` (hook discovery)
  - `(*goja.Runtime).ExportTo` (convert JS values to Go event structs)
  - `(*goja.Runtime).Interrupt` / `ClearInterrupt` (timeouts)

## Step 3: Write the MVP Design and “Trimmed Spec” Reference

Translated the imported “LogFlow” spec and examples into two ticket documents: a short reference that explicitly defines what is in/out of scope for the MVP, and a detailed design doc that specifies the JS contract, output schema, safety boundaries, and a Go implementation plan for `devctl`.

This step converts “lots of possible features” into a concrete, implementable plan: synchronous, single-module, line-oriented processing using `goja`, with a small helper surface and explicit decisions around safety (no `exec`/`fs` by default) and reliability (timeouts via `Runtime.Interrupt`).

### What I did
- Wrote the MVP-trimming reference doc:
  - `/home/manuel/workspaces/2026-01-06/log-parser-module/devctl/ttmp/2026/01/06/MO-007-LOG-PARSER--javascript-log-processor-for-devctl/reference/01-source-notes-provided-spec-trimmed-for-mvp.md`
- Wrote the main design doc:
  - `/home/manuel/workspaces/2026-01-06/log-parser-module/devctl/ttmp/2026/01/06/MO-007-LOG-PARSER--javascript-log-processor-for-devctl/design-doc/01-mvp-design-javascript-log-parser-goja.md`
- Updated the ticket overview and task list:
  - `/home/manuel/workspaces/2026-01-06/log-parser-module/devctl/ttmp/2026/01/06/MO-007-LOG-PARSER--javascript-log-processor-for-devctl/index.md`
  - `/home/manuel/workspaces/2026-01-06/log-parser-module/devctl/ttmp/2026/01/06/MO-007-LOG-PARSER--javascript-log-processor-for-devctl/tasks.md`

### Why
- The imported spec is intentionally exhaustive; without a “trimmed MVP” document it’s easy to accidentally implement a platform instead of a tool.
- A detailed design doc reduces implementation thrash: it clarifies contracts (JS hooks), boundaries (sync-only), safety (module enablement), and operational constraints (`--follow` needs timeouts).

### What worked
- The design doc now contains a complete end-to-end story:
  - JS contract (`register`, hook semantics)
  - normalized event schema for NDJSON output
  - Go implementation sketches referencing `goja` and `goja_nodejs/require`
  - integration point in `devctl/cmd/devctl/cmds/logs.go`

### What didn't work
- N/A (documentation-only step).

### What I learned
- Writing down the normalization rules is the real “API”: it’s what downstream tooling and users will depend on, more than the hook names themselves.

### What was tricky to build
- Keeping the MVP small while still “feeling good” for users:
  - whether to accept string shorthand (`parse` returning a string) vs forcing objects
  - timestamp behavior (omit vs set to now)
  - handling “extra fields” (allow top-level vs forcing into `fields`)

### What warrants a second pair of eyes
- The safety story around `require()` and module enablement:
  - ensure we do not accidentally enable side-effectful modules by default (especially if we reuse `go-go-goja/engine.New()`).
- The timeout strategy: confirm `Runtime.Interrupt` usage won’t leave the runtime in a broken state after repeated interrupts.

### What should be done in the future
- Resolve the Open Questions (timestamp default, unknown fields policy, newline handling, worker ordering).
- Implement `devctl/pkg/logjs` and integrate `--js` into `devctl logs`.

### Code review instructions
- Start with the design doc:
  - `/home/manuel/workspaces/2026-01-06/log-parser-module/devctl/ttmp/2026/01/06/MO-007-LOG-PARSER--javascript-log-processor-for-devctl/design-doc/01-mvp-design-javascript-log-parser-goja.md`
- Cross-check MVP scope against the trimming reference:
  - `/home/manuel/workspaces/2026-01-06/log-parser-module/devctl/ttmp/2026/01/06/MO-007-LOG-PARSER--javascript-log-processor-for-devctl/reference/01-source-notes-provided-spec-trimmed-for-mvp.md`
- Verify the ticket overview links are correct:
  - `/home/manuel/workspaces/2026-01-06/log-parser-module/devctl/ttmp/2026/01/06/MO-007-LOG-PARSER--javascript-log-processor-for-devctl/index.md`

### Technical details
- The MVP design intentionally excludes:
  - multi-module pipelines
  - batching/aggregation/output destinations
  - async hooks (`Promise` / `await`)
  - large helper library (rate limiting, TTL state, alerts, multiline buffering)

## Step 4: Make Imported Sources Valid docmgr Documents (Frontmatter)

Ran `docmgr doctor` and discovered that the imported Markdown sources under `sources/local/` did not have YAML frontmatter, which `docmgr` treats as an error for Markdown artifacts it indexes. I added minimal frontmatter blocks to both imported source files so the ticket stays healthy and searchable.

This preserves the original content verbatim after the frontmatter while making the artifacts “docmgr-native” (metadata + indexing). The only remaining findings are warnings about missing numeric prefixes in the source filenames, which we intentionally keep because they are keyed by the import name and referenced from the ticket index.

### What I did
- Ran `docmgr doctor --ticket MO-007-LOG-PARSER --stale-after 30` and saw `invalid_frontmatter` errors for:
  - `sources/local/js-log-parser-spec.md`
  - `sources/local/js-log-parser-examples.md`
- Added YAML frontmatter to:
  - `/home/manuel/workspaces/2026-01-06/log-parser-module/devctl/ttmp/2026/01/06/MO-007-LOG-PARSER--javascript-log-processor-for-devctl/sources/local/js-log-parser-spec.md`
  - `/home/manuel/workspaces/2026-01-06/log-parser-module/devctl/ttmp/2026/01/06/MO-007-LOG-PARSER--javascript-log-processor-for-devctl/sources/local/js-log-parser-examples.md`
- Re-ran `docmgr doctor --ticket MO-007-LOG-PARSER --stale-after 30` to confirm errors were resolved.

### Why
- `docmgr` treats missing frontmatter delimiters (`---`) as a parse error, which can hide the ticket in searches/indices or cause noisy health reports.
- Adding frontmatter keeps provenance (notes that these are imported from `/tmp/...`) and clarifies that they are input sources, not MVP requirements.

### What worked
- `docmgr doctor` no longer reports frontmatter errors for the imported sources.

### What didn't work
- `docmgr doctor` still reports `missing_numeric_prefix` warnings for the imported source filenames. We are leaving this as-is for now because:
  - the filenames are referenced from the ticket index as `local:js-log-parser-*.md`
  - renaming would require carefully updating those references and any other tooling expectations

### What I learned
- `docmgr import file` copies content but does not auto-inject frontmatter; if the source Markdown lacks frontmatter, we need to add it in the imported copy to satisfy `docmgr` validation.

### What was tricky to build
- Ensuring the frontmatter is the first thing in the file (no header text before `---`), otherwise `docmgr` continues to report “frontmatter delimiters not found”.

### What warrants a second pair of eyes
- Whether we should standardize imported-source naming to include numeric prefixes (and whether `docmgr` should treat `sources/local/` differently). This is more of a docmgr workflow question than a log-parser one.

### What should be done in the future
- Decide whether to eliminate the `missing_numeric_prefix` warnings by renaming the imported artifacts and updating references (only if it’s worth the churn).

### Code review instructions
- Re-run: `docmgr doctor --ticket MO-007-LOG-PARSER --stale-after 30`
- Inspect the top of both imported sources to confirm frontmatter is present and the original content follows immediately after.

### Technical details
- The frontmatter added to imported sources explicitly labels them as “Imported Source” and records their origin path under `/tmp/`.

## Step 5: Switch MVP Delivery to a Standalone `log-parse` CLI (Integration Deferred)

Adjusted the MVP plan so the first shippable artifact is a tiny separate `cmd/log-parse` binary used solely to exercise the JavaScript log parsing engine. This lets us iterate on the JS contract, helper surface, normalization rules, and timeout behavior without being coupled to `devctl` service supervision/log file discovery.

The `devctl logs` integration remains part of the long-term direction, but it is explicitly deferred until the standalone tool is working nicely and we’re confident we can embed it into a long-lived “follow” workflow without surprises.

### What I did
- Updated the MVP design doc to make `log-parse` the MVP CLI and move `devctl logs --js ...` to “future integration”.
- Updated the ticket overview and tasks to match the new delivery plan.

### Why
- Decouples early debugging from `devctl` plumbing (service state, log file locations, follow/tail implementation).
- Makes it easier to write focused tests and reproduce issues: `cat file | log-parse --js parser.js`.
- Reduces risk: we can lock down module enablement and timeouts in an isolated tool before exposing it through `devctl`.

### What worked
- The design doc now has a clean, minimal “exercise loop” for developers:
  - write `parser.js`
  - run `log-parse --js parser.js --input file` or stream via stdin

### What didn't work
- N/A (documentation-only change).

### What I learned
- Separating “engine correctness” from “integration UX” helps keep the MVP scope honest: we can ship something useful without having to decide everything about devctl service log discovery up front.

### What was tricky to build
- Keeping nomenclature and paths consistent (we use `devctl/cmd/log-parse` as the intended binary location, and `devctl/pkg/logjs` as the intended engine package).

### What warrants a second pair of eyes
- Naming and ownership: confirm `log-parse` should live under the `devctl` Go module (recommended) vs a separate module/binary at repo root.

### What should be done in the future
- Implement `devctl/pkg/logjs` and `devctl/cmd/log-parse`.
- Only after that stabilizes: integrate the same engine into `devctl logs`.

### Code review instructions
- Review the updated CLI plan and implementation sequence in:
  - `/home/manuel/workspaces/2026-01-06/log-parser-module/devctl/ttmp/2026/01/06/MO-007-LOG-PARSER--javascript-log-processor-for-devctl/design-doc/01-mvp-design-javascript-log-parser-goja.md`
- Confirm tasks reflect the new delivery plan:
  - `/home/manuel/workspaces/2026-01-06/log-parser-module/devctl/ttmp/2026/01/06/MO-007-LOG-PARSER--javascript-log-processor-for-devctl/tasks.md`

### Technical details
- MVP CLI target: `devctl/cmd/log-parse` (standalone).
- Integration target (future): `devctl/cmd/devctl/cmds/logs.go`.

## Step 6: Implement `devctl/pkg/logjs` + `cmd/log-parse` with Tests

Implemented the core MVP in Go: a `devctl/pkg/logjs` package that hosts a `goja` runtime, exposes a strict `register({ ... })` hook contract, runs a synchronous parse/filter/transform pipeline per line, and normalizes results into a stable JSON event schema. Also added a tiny standalone Cobra CLI `cmd/log-parse` to exercise the engine against stdin/files before any `devctl` integration.

This step also included an explicit “test often” loop: `go fmt ./...` and repeated `go test` runs while fixing integration pitfalls (Go file suffix build constraints, goja_nodejs console requiring require, and nil-value edge cases when reading absent properties).

**Commit (code):** 9b86bc031454347e03d78b237e817c735dd50392 — "logjs: add goja-based JS parser and log-parse CLI"

### What I did
- Added `devctl/pkg/logjs`:
  - loads a JS script from file and requires it to call `register(config)`
  - supports hooks: `init`, `parse` (required), `filter`, `transform`, `shutdown`, `onError`
  - injects a small JS helper prelude to provide `globalThis.log.*`
  - provides a minimal Go-backed `console` so scripts can use `console.log/error`
  - enforces synchronous behavior and per-hook timeouts via `(*goja.Runtime).Interrupt`
  - normalizes JS results into a stable `Event` struct and tracks `Stats`
- Added `devctl/cmd/log-parse`:
  - reads from stdin or `--input`
  - loads `--js` script and streams events to stdout as `ndjson` (or `pretty`)
- Added tests under `devctl/pkg/logjs`:
  - parse/filter/transform + unknown-field normalization into `fields`
  - timestamp conversion for JS `Date`
  - timeout behavior for runaway scripts
- Ran:
  - `go fmt ./...`
  - `go test ./pkg/logjs -count=1`
  - `go test ./... -count=1`

### Why
- `log-parse` gives a tight feedback loop for engine correctness without any `devctl` integration complexity.
- The `logjs` package boundary makes it easy to later embed in `devctl logs` unchanged.
- Tests lock in the most important “API”: event normalization and safety behavior.

### What worked
- The engine can run user JS, call hooks, and emit stable NDJSON events.
- Timeouts work for infinite loops using `Runtime.Interrupt`, and the engine drops the line while recording stats.

### What didn't work
- First attempt named the helper file `helpers_js.go`, which Go treated as GOOS=js-only, causing:
  - `pkg/logjs/module.go:87:47: undefined: helpersJS`
  - Fixed by renaming to `helpers.go`.
- Attempted to use `goja_nodejs/console.Enable(vm)` but it panicked because `require()` wasn’t enabled:
  - `panic: TypeError: Please enable require for this runtime using new(require.Registry).Enable(runtime)`
  - Fixed by implementing a minimal `console` object directly (no `require()` in MVP).
- An early `register` binding using `goja.FunctionCall` didn’t receive arguments as expected and threw:
  - `GoError: register(config) requires a config object`
  - Fixed by changing the binding signature to `func(config goja.Value) error`.

### What I learned
- Naming a file `*_js.go` will restrict it to GOOS=js builds; avoid that suffix unless you mean it.
- `goja_nodejs/console` depends on `goja_nodejs/require` being enabled; if we want a safe-by-default runtime, implementing a minimal `console` ourselves is simpler.
- Absent JS object properties can surface as a `nil` `goja.Value` in Go; treat `nil` as “undefined” to avoid panics.

### What was tricky to build
- Defining a normalization strategy that is stable but still ergonomic for scripts:
  - extra top-level keys are moved into `fields`
  - timestamps support both strings and `Date` (via `toISOString`)
- Timeout handling: ensuring `Interrupt` is always cleared even when the timer doesn’t fire.

### What warrants a second pair of eyes
- Timeout semantics and error classification: verify we count timeouts correctly vs ordinary exceptions when `--js-timeout` is set.
- Console implementation: confirm writing to stdout/stderr from within goja callbacks is acceptable for the intended UX.

### What should be done in the future
- Add `devctl/examples/log-parse/` with a couple of real scripts and sample logs.
- Consider (carefully) whether to add Node-style `require()` with a restricted loader for safe helper modules.
- Only after that stabilizes: integrate into `devctl logs`.

### Code review instructions
- Start with code:
  - `/home/manuel/workspaces/2026-01-06/log-parser-module/devctl/pkg/logjs/module.go`
  - `/home/manuel/workspaces/2026-01-06/log-parser-module/devctl/cmd/log-parse/main.go`
- Validate:
  - `go test ./... -count=1`
  - `go run ./cmd/log-parse --js /path/to/parser.js --input /path/to/log.txt`

### Technical details
- JS helpers are injected via a prelude string (`globalThis.log`) rather than `require()`.
- Event timestamp normalization prefers `Date.toISOString()` when available; otherwise accepts string.
- Unknown top-level keys returned by JS are moved into `event.fields` unless explicitly set there already.

## Step 7: Tighten Timeout Accounting + Stabilize a Flaky Runtime Test

After the initial implementation landed, I noticed two follow-up issues while running `go test ./...`: the `logjs` timeout counter would increment on any hook error if a timeout was configured, and an existing `pkg/runtime` test (`TestRuntime_CallTimeout`) was flaky because it used a 200ms context for both plugin startup and the call under test.

This step makes timeout accounting precise (only count when we are interrupted specifically by the timeout sentinel) and makes the runtime timeout test stable by giving plugin startup its own longer context while keeping the call itself constrained to 200ms.

**Commit (code):** 5a371c7bda04d5f376d14a4df4233563a334d339 — "logjs: improve timeout detection; fix runtime timeout test"

### What I did
- Updated `devctl/pkg/logjs/module.go`:
  - introduced `ErrHookTimeout`
  - detect timeouts via `errors.As(err, *goja.InterruptedError)` and `InterruptedError.Value()`
  - increment `Stats.HookTimeouts` only when the interrupt value matches `ErrHookTimeout`
- Updated `devctl/pkg/runtime/runtime_test.go`:
  - use `startCtx` (2s) for plugin startup
  - use `callCtx` (200ms) for `c.Call(...)`, preserving the timeout assertion
- Ran `go test ./... -count=1`.

### Why
- Timeout metrics should represent actual timeouts, not “any error while a timeout is configured”.
- The runtime test should validate call timeout semantics, not depend on plugin startup completing inside 200ms.

### What worked
- `go test ./... -count=1` is stable again.

### What didn't work
- N/A (small corrective change).

### What I learned
- `goja` exposes `*goja.InterruptedError` with the original `Interrupt` value accessible via `Value()`, which is the cleanest way to classify interrupts.

### What was tricky to build
- N/A (straightforward refactor + test adjustment).

### What warrants a second pair of eyes
- Confirm the chosen timeout sentinel (`ErrHookTimeout`) and classification logic matches how we want to report/handle other interrupts in the future (e.g. ctx cancellation).

### What should be done in the future
- Consider adding context-cancellation interrupts (separate from timeouts) if we want `ProcessLine(ctx, ...)` to be cancellable mid-hook.

### Code review instructions
- Review timeout detection:
  - `/home/manuel/workspaces/2026-01-06/log-parser-module/devctl/pkg/logjs/module.go`
- Review runtime test adjustment:
  - `/home/manuel/workspaces/2026-01-06/log-parser-module/devctl/pkg/runtime/runtime_test.go`
- Validate:
  - `go test ./... -count=1`

### Technical details
- Timeout classification uses `errors.As(err, *goja.InterruptedError)` and checks `InterruptedError.Value()` for `ErrHookTimeout`.

## Step 8: Research go-go-goja and jesus Patterns (require(), console, sandboxing)

Reviewed `go-go-goja` (our wrapper around goja/goja_nodejs) and the larger `jesus` project to understand how they structure runtime setup (require/console/module registry) and to identify patterns that would have avoided the integration issues we hit in `logjs` (notably `goja_nodejs/console` requiring `require()` and the risk of accidentally enabling filesystem-backed module loading).

The key outcome is a concrete plan for a *safe* `require()` sandbox: we can enable `require()` primarily for **core modules** like `console`/`util` (for formatting) while using a custom `SourceLoader` that restricts any `.js`/`.json` file loads to a specific directory tree (e.g. “only under the entry script directory”), and by carefully controlling which native modules are registered (do not register `fs`/`exec` unless explicitly opted in).

### What I did
- Read the `go-go-goja` registry code and its runtime helper:
  - `/home/manuel/workspaces/2026-01-06/log-parser-module/go-go-goja/modules/common.go`
  - `/home/manuel/workspaces/2026-01-06/log-parser-module/go-go-goja/engine/runtime.go`
- Verified the exact reason `goja_nodejs/console.Enable(vm)` panics without `require()`:
  - `go-go-goja/engine/runtime.go` calls `reg.Enable(vm)` before `console.Enable(vm)` (safe ordering).
  - `goja_nodejs/console` internally calls `require.Require(runtime, "console")`, which panics if `require` wasn’t installed into the runtime.
- Examined the goja_nodejs `require` implementation for where sandbox hooks exist:
  - `require.WithLoader(...)` and `DefaultSourceLoader` (filesystem) behavior.
  - the Node-like resolver algorithm (file/dir paths vs “bare” module names, and node_modules traversal).
- Looked at how `jesus` uses `go-go-goja/modules.Registry` and `require.NewRegistry()` in a real system:
  - `/home/manuel/workspaces/2026-01-06/log-parser-module/jesus/pkg/engine/engine.go`
- Looked at how `jesus` provides `console` without depending on goja_nodejs `console`:
  - `/home/manuel/workspaces/2026-01-06/log-parser-module/jesus/pkg/engine/bindings.go`
  - `/home/manuel/workspaces/2026-01-06/log-parser-module/jesus/pkg/repl/model.go`

### Why
- Our MVP `logjs` avoided `require()` entirely to stay safe-by-default, but this came at a cost:
  - we had to implement our own minimal `console`
  - we cannot currently support `require("./local-file")` user scripts
- `go-go-goja` and `jesus` already contain patterns for:
  - installing `require()` safely in the runtime
  - controlling which native modules are enabled
  - providing a console implementation (either Node-style via goja_nodejs or minimal custom)

### What worked
- `go-go-goja` confirms the critical ordering: enable `require()` first, then enable `console`.
- `jesus` shows a “selective module enablement” approach:
  - use `gogogojamodules.DefaultRegistry` + only import specific modules (e.g. `database`) so that only those modules register via `init()`.
- The goja_nodejs `require` API includes a concrete sandbox seam (`WithLoader`) that can be used to restrict filesystem access.

### What didn't work
- N/A (research-only step).

### What I learned
- goja_nodejs `console` is itself a core module implemented via `require()`, and it requires the `util` core module for `util.format`.
- goja_nodejs `require` has *two distinct capability planes*:
  1) **native/core modules** loaded via `RegisterNativeModule` / `RegisterCoreModule` (no filesystem loader involved)
  2) **file-backed modules** loaded via `SourceLoader` (filesystem by default)
  A sandbox can allow (1) while tightly restricting (2).
- The resolver walks “node_modules” up the directory tree for bare imports; sandboxing must prevent that from escaping the allowed root (return `ModuleFileDoesNotExistError` for any disallowed candidate path).

### What was tricky to build
- N/A (research), but the tricky implementation details are clear:
  - path normalization: goja_nodejs `require` uses the POSIX `path` package internally; our loader must normalize consistently and avoid `..` traversal.
  - symlinks: a prefix check on cleaned paths is not sufficient if symlinks exist; an optional `EvalSymlinks` check is safer but more expensive.
  - “entry script path must be absolute” if we want a clean “allowed root” policy.

### What warrants a second pair of eyes
- Sandbox policy: confirm the exact desired semantics for what “down from the entry script” means:
  - allow only within the entry script directory (`/path/to/script-dir/**`)
  - whether to allow `node_modules` inside that directory
  - whether to allow `.json` requires

### What should be done in the future
- Write a dedicated analysis doc that explains:
  - require() resolution semantics and caching
  - what hooks exist for sandboxing (WithLoader)
  - a concrete sandbox loader design for our `log-parse` use case

### Code review instructions
- Read the registry and runtime wiring:
  - `/home/manuel/workspaces/2026-01-06/log-parser-module/go-go-goja/modules/common.go`
  - `/home/manuel/workspaces/2026-01-06/log-parser-module/go-go-goja/engine/runtime.go`
- Compare `jesus` runtime setup and console implementations:
  - `/home/manuel/workspaces/2026-01-06/log-parser-module/jesus/pkg/engine/engine.go`
  - `/home/manuel/workspaces/2026-01-06/log-parser-module/jesus/pkg/engine/bindings.go`

### Technical details
- goja_nodejs `console` → `require()` dependency:
  - It calls `require.Require(runtime, "console")` which panics unless `(*require.Registry).Enable(runtime)` was called.
- Sandbox seam:
  - `require.WithLoader(...)` lets us replace filesystem module loading; returning `ModuleFileDoesNotExistError` makes the resolver continue searching.
