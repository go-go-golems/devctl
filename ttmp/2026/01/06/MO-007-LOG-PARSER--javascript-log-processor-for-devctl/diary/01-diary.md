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
      Note: |-
        Standalone MVP binary implemented in Step 6 (commit 9b86bc031454347e03d78b237e817c735dd50392)
        CLI now loads many modules
        Step 14 adds --errors NDJSON output and handles multi-event returns
    - Path: devctl/examples/log-parse/README.md
      Note: |-
        Updated example commands to use --module
        Step 15 documents modules-dir demo
    - Path: devctl/examples/log-parse/modules/01-errors.js
      Note: Step 15 many-module fan-out example (errors)
    - Path: devctl/examples/log-parse/parser-regex.js
      Note: Fixed example to avoid goja RegExp named capture groups so scripts remain runnable
    - Path: devctl/examples/log-parse/sample-fanout-json-lines.txt
      Note: Step 15 sample input for many-module demo
    - Path: devctl/pkg/logjs/fanout.go
      Note: |-
        Fan-out runner that executes multiple modules per input line and injects tags
        Step 14 aggregates module error records and emits multiple events per line
    - Path: devctl/pkg/logjs/fanout_test.go
      Note: Tests for tagging
    - Path: devctl/pkg/logjs/helpers.go
      Note: |-
        Step 14 expands stdlib helper surface (parseKeyValue/capture/getPath/addTag/toNumber/etc.)
        Step 15 adds log.createMultilineBuffer helper
    - Path: devctl/pkg/logjs/module.go
      Note: |-
        Implemented in Step 6 (commit 9b86bc031454347e03d78b237e817c735dd50392)
        Add module tag support + metadata needed for pipeline introspection
        Step 14 adds multi-event returns
    - Path: devctl/pkg/logjs/module_test.go
      Note: |-
        Step 14 adds tests for array returns and error records
        Step 15 adds multiline buffer test
    - Path: devctl/pkg/logjs/types.go
      Note: |-
        Add ModuleInfo type used by CLI pipeline printing
        Step 14 adds ErrorRecord type
    - Path: devctl/ttmp/2026/01/06/MO-007-LOG-PARSER--javascript-log-processor-for-devctl/design-doc/01-mvp-design-javascript-log-parser-goja.md
      Note: |-
        Primary design doc produced during diary steps
        Updated MVP CLI flag docs to --module
    - Path: devctl/ttmp/2026/01/06/MO-007-LOG-PARSER--javascript-log-processor-for-devctl/design-doc/02-next-step-design-multi-script-pipeline-for-log-parse.md
      Note: |-
        Drafted in Step 10 to define multi-script pipeline evolution
        Updated design to emphasize self-contained modules emitting tagged derived streams
    - Path: devctl/ttmp/2026/01/06/MO-007-LOG-PARSER--javascript-log-processor-for-devctl/design-doc/03-roadmap-design-from-fan-out-log-parse-to-logflow-ish-system.md
      Note: Step 12 produced this roadmap design
    - Path: devctl/ttmp/2026/01/06/MO-007-LOG-PARSER--javascript-log-processor-for-devctl/index.md
      Note: Ticket overview and navigation
    - Path: devctl/ttmp/2026/01/06/MO-007-LOG-PARSER--javascript-log-processor-for-devctl/playbook/01-playbook-log-parse-mvp-testing.md
      Note: Updated testing playbook commands to use --module
    - Path: devctl/ttmp/2026/01/06/MO-007-LOG-PARSER--javascript-log-processor-for-devctl/reference/01-source-notes-provided-spec-trimmed-for-mvp.md
      Note: MVP scope trimming decisions captured here
    - Path: devctl/ttmp/2026/01/06/MO-007-LOG-PARSER--javascript-log-processor-for-devctl/scripts/02-run-examples.sh
      Note: Updated to use --module flag
    - Path: devctl/ttmp/2026/01/06/MO-007-LOG-PARSER--javascript-log-processor-for-devctl/scripts/03-timeout-demo.sh
      Note: Updated to use --module flag
    - Path: devctl/ttmp/2026/01/06/MO-007-LOG-PARSER--javascript-log-processor-for-devctl/scripts/04-run-fanout-modules-dir-demo.sh
      Note: Step 15 runnable demo script
    - Path: devctl/ttmp/2026/01/06/MO-007-LOG-PARSER--javascript-log-processor-for-devctl/scripts/05-validate-fanout-modules-dir.sh
      Note: Step 15 validation script
    - Path: devctl/ttmp/2026/01/06/MO-007-LOG-PARSER--javascript-log-processor-for-devctl/sources/local/js-log-parser-spec.md
      Note: Step 12 input spec studied for roadmap
    - Path: devctl/ttmp/2026/01/06/MO-007-LOG-PARSER--javascript-log-processor-for-devctl/tasks.md
      Note: |-
        Added tasks for multi-module build/test scripts
        Step 12 new tasks added
ExternalSources: []
Summary: ""
LastUpdated: 2026-01-06T19:27:06-05:00
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

## Step 9: Make log-parse Output Unbuffered (Stream Results Immediately)

Observed that `log-parse` appeared to “only print parsed lines once stdin closes”. The core issue was output buffering: the CLI wrapped stdout in a `bufio.Writer` and only flushed at process exit, so NDJSON output would sit in the buffer during long-running streams (e.g. `tail -f ... | log-parse ...`).

This step removes that output buffering so each emitted event is written directly to `cmd.OutOrStdout()` as it’s produced. Input is still line-oriented (we require newline-delimited input for streaming), but output is now immediate for each event.

### What I did
- Updated `/home/manuel/workspaces/2026-01-06/log-parser-module/devctl/cmd/log-parse/main.go`:
  - removed `bufio.NewWriter(cmd.OutOrStdout())`
  - write NDJSON via `json.NewEncoder(cmd.OutOrStdout()).Encode(...)` directly
  - write pretty JSON directly to `cmd.OutOrStdout()`
- Ran `go fmt ./cmd/log-parse` and `go test ./... -count=1`.

### Why
- For a streaming CLI, “buffer until exit” defeats the purpose of `--follow`/piped usage.
- Keeping buffering out of the program makes behavior consistent regardless of where stdout points (terminal, pipe, file).

### What worked
- `go test ./... -count=1` still passes.
- Interactive piping should now print each event as soon as a newline-delimited input line arrives and is processed.

### What didn't work
- N/A (small behavioral fix).

### What I learned
- For NDJSON streaming tools, it’s best to avoid adding an extra user-space buffered writer unless you flush on every record (which defeats most performance benefits anyway).

### What was tricky to build
- N/A.

### What warrants a second pair of eyes
- Confirm expected UX for non-newline-terminated input: `log-parse` remains line-based and won’t process partial lines until a newline arrives.

### What should be done in the future
- Consider adding an explicit `--flush`/`--buffered-output` flag if we ever want to trade throughput for latency in a controlled way (default should remain streaming-friendly).

### Code review instructions
- Inspect the change:
  - `/home/manuel/workspaces/2026-01-06/log-parser-module/devctl/cmd/log-parse/main.go`
- Validate:
  - `cd devctl && go test ./... -count=1`
  - `echo '{\"msg\":\"hi\"}' | go run ./cmd/log-parse --js /tmp/parser.js`

### Technical details
- Output now writes directly to the underlying writer returned by Cobra (`cmd.OutOrStdout()`), so each `Encode` call results in immediate writes without waiting for a buffered flush.

## Step 10: Design the Next “Consequence Jump” (Multi-Script Pipeline)

After stabilizing the MVP, I drafted the next design step to make `log-parse` materially more useful: loading **many scripts** and composing them as a deterministic pipeline. This is the point where the tool becomes something you can realistically keep in a repo and evolve: multiple parsers for multiple formats, shared filter/transform stages, validation/introspection, and per-module stats.

This design intentionally does *not* enable JavaScript `require()` yet; composition happens in Go by loading multiple scripts and executing their hooks in order. The separate `MO-008-REQUIRE-SANDBOX` ticket captures how we can safely enable `require()` later.

### What I did
- Wrote a new design document that defines:
  - multi-script loading UX (`--module`, `--modules-dir`, optional `--config`)
  - parse mode (`first` vs `all`)
  - pipeline semantics and ordering
  - validation and introspection (`validate`, `--print-pipeline`, `--stats`)
  - Go API sketches (`Pipeline`, `LoadedModule`) and pseudocode execution model
- Related the design doc to the current implementation and examples.

### Why
- Real-world log streams are mixed-format and need multiple parsers.
- Users want reusable, small scripts for enrichment/filtering across many services.
- Deterministic composition and validation tools reduce “it works on my machine” script drift.

### What worked
- The resulting document provides a concrete path from MVP to a multi-module system without changing the core “safe-by-default” posture.

### What didn't work
- N/A (design-only step).

### What I learned
- The simplest “multi-script” approach that preserves performance is “one runtime per worker, load all scripts into that runtime, store callables, and run hooks in Go”.

### What was tricky to build
- Getting unambiguous semantics when multiple parse hooks exist (hence an explicit `--parse-mode`).

### What warrants a second pair of eyes
- Stage ordering semantics and whether `order` should live in JS metadata vs config-only.
- Whether we should allow parse modules to emit multiple events per line (array return) in the first multi-module iteration.

### What should be done in the future
- Implement the `Pipeline` refactor in `devctl/pkg/logjs` and extend `log-parse` flags accordingly.

### Code review instructions
- Review the design doc:
  - `/home/manuel/workspaces/2026-01-06/log-parser-module/devctl/ttmp/2026/01/06/MO-007-LOG-PARSER--javascript-log-processor-for-devctl/design-doc/02-next-step-design-multi-script-pipeline-for-log-parse.md`

### Technical details
- The design explicitly defers `require()` to `MO-008-REQUIRE-SANDBOX` to avoid accidentally expanding filesystem capabilities.

## Step 11: Rethink Multi-Module Semantics (Self-Contained Tagged Streams) + Fix goja RegExp Example

Reworked the “next step” design to match the intended operator model: each `register()` module is **self-contained**, runs against the same input stream, and emits its own **tagged derived event stream**. This removes accidental coupling implied by a stage/pipeline mental model and makes it explicit that downstream merging is up to the user.

While validating the ticket scripts, the regex example failed under goja due to unsupported RegExp features. I fixed the example to avoid named capture groups and verified the scripts and Go tests pass again.

**Commit (code):** N/A (this workspace has no `.git` directory, so I could not produce a commit hash)

### What I did
- Updated the multi-module design doc to clarify fan-out semantics (“one input stream, many tagged outputs”) and to make “one runtime per module” the explicit default for isolation.
- Added `docmgr` tasks for multi-module build/test scripts.
- Ran the existing ticket scripts:
  - `bash devctl/ttmp/2026/01/06/MO-007-LOG-PARSER--javascript-log-processor-for-devctl/scripts/02-run-examples.sh`
  - `bash devctl/ttmp/2026/01/06/MO-007-LOG-PARSER--javascript-log-processor-for-devctl/scripts/01-go-test.sh`
- Fixed `devctl/examples/log-parse/parser-regex.js`:
  - removed JS named capture groups (`(?<name>...)`), which goja’s RegExp does not support
  - used positional capture groups via `.exec()` instead

### Why
- The real usage is typically “one log format in, many derived views out”; enforcing self-contained modules + explicit tags makes the system easier to reason about and reduces hidden coupling.
- Keeping modules isolated (no shared runtime) aligns with the “no `require()` yet” posture and reduces global namespace collisions.
- The examples must remain runnable to preserve a fast feedback loop for developers.

### What worked
- After updating the regex example, `scripts/02-run-examples.sh` completes successfully again.
- `go test ./... -count=1` still passes via `scripts/01-go-test.sh`.

### What didn't work
- `scripts/02-run-examples.sh` initially failed with:
  - `Error: compile script: SyntaxError: Unmatched ')' at examples/log-parse/parser-regex.js:7:15`
  - Root cause: unsupported RegExp features in goja (named capture groups and related syntax).

### What I learned
- goja’s RegExp implementation is compatible with a large subset of JS regexes, but it does not support named capture groups; examples should avoid these features unless we document the limitation prominently.
- “Pipeline” language easily suggests stage chaining; describing the design as “fan-out tagged derived streams” better matches what we want users to build.

### What was tricky to build
- Keeping the document precise without prematurely committing to `require()` semantics: the design has to clearly explain code reuse as a future capability, not a current coupling mechanism.

### What warrants a second pair of eyes
- The tag injection rules and schema defaults in the design doc: verify that `event.tags` + `event.fields._tag/_module` are the right long-term knobs (namespacing, collisions, downstream ergonomics).

### What should be done in the future
- Implement the multi-module fan-out runner and CLI flags described in the updated design doc.
- Add new ticket scripts (tasks 12/13) for multi-module demo + validation once the feature exists.

### Code review instructions
- Read the updated design doc:
  - `/home/manuel/workspaces/2026-01-06/log-parser-module/devctl/ttmp/2026/01/06/MO-007-LOG-PARSER--javascript-log-processor-for-devctl/design-doc/02-next-step-design-multi-script-pipeline-for-log-parse.md`
- Validate examples and unit tests:
  - `bash devctl/ttmp/2026/01/06/MO-007-LOG-PARSER--javascript-log-processor-for-devctl/scripts/02-run-examples.sh`
  - `bash devctl/ttmp/2026/01/06/MO-007-LOG-PARSER--javascript-log-processor-for-devctl/scripts/01-go-test.sh`

### Technical details
- The regex example now uses:
  - `^(\w+)\s+\[([^\]]+)\]\s+(.*)$` with positional groups `m[1..3]`
- New tasks were added to `tasks.md` to track the multi-module build/test script work.

## Step 12: Translate the “Full” Spec into a Fan-Out Roadmap (Tasks + Design)

Read the imported upstream spec and turned it into a concrete, incremental roadmap that stays aligned with our “fan-out tagged derived streams” model. The goal of this step is to keep momentum and avoid “spec paralysis”: we explicitly choose which spec features to build next, how they map to our runner architecture, and which features stay deferred behind safety boundaries (notably sandboxed `require()`).

This step produced two outputs: a new set of ticket tasks that break down the next implementation phases, and a new design doc that explains how the LogFlow-ish features (stdlib, multiline, error records, future aggregation) fit into the fan-out module model.

**Commit (code):** N/A (this workspace has no `.git` directory, so I could not produce a commit hash)

### What I did
- Reviewed the full imported spec:
  - `/home/manuel/workspaces/2026-01-06/log-parser-module/devctl/ttmp/2026/01/06/MO-007-LOG-PARSER--javascript-log-processor-for-devctl/sources/local/js-log-parser-spec.md`
- Added a new batch of docmgr tasks to move from MVP → multi-module fan-out → “LogFlow-ish” features:
  - `docmgr task add --ticket MO-007-LOG-PARSER --text "..."`
- Created a new roadmap design doc that aligns the spec to the fan-out plan:
  - `/home/manuel/workspaces/2026-01-06/log-parser-module/devctl/ttmp/2026/01/06/MO-007-LOG-PARSER--javascript-log-processor-for-devctl/design-doc/03-roadmap-design-from-fan-out-log-parse-to-logflow-ish-system.md`

### Why
- The spec is intentionally over-scoped; without a roadmap it’s easy to oscillate between “implement everything” and “do nothing”.
- We need a shared understanding of how future features (stdlib, multiline, aggregation, outputs) fit into the fan-out semantics already captured in the next-step design doc.
- Task granularity matters: it lets us implement, test, and validate in small slices while keeping `docmgr` bookkeeping accurate.

### What worked
- The spec decomposes cleanly into phases that match our current architecture:
  - multi-module fan-out foundation first
  - then multi-event returns and error records
  - then stdlib v1 helpers
  - then multiline buffering
- The new roadmap design doc makes “alignment to fan-out” explicit, instead of silently drifting back into stage chaining.

### What didn't work
- N/A (documentation + planning only).

### What I learned
- The largest “value per feature” in the upstream spec is **stdlib ergonomics** (helpers) and **multiline buffering**; advanced outputs and async hooks are bigger boundary crossings and should stay deferred.
- The spec’s `aggregate`/`output` stages don’t map 1:1 to fan-out; they need a deliberate reinterpretation (per-module aggregate events or Go-level sinks).

### What was tricky to build
- Writing down a “full system” path without prematurely committing to unsafe capabilities (filesystem/network/require); this required explicitly separating “what we can do safely now” from “what needs sandbox policy”.

### What warrants a second pair of eyes
- The stdlib v1 surface area: confirm the chosen helper set is the right “minimum useful” and that we aren’t locking ourselves into awkward namespacing (`log.*` vs globals).
- The proposed “dead-letter” error record shape: ensure it’s sufficient for debugging while not leaking sensitive data by default.

### What should be done in the future
- Start implementing tasks 14–17 (fan-out runner + CLI loading/validation/introspection), then add tests (task 24) before expanding helper surface area.

### Code review instructions
- Review the roadmap design doc:
  - `/home/manuel/workspaces/2026-01-06/log-parser-module/devctl/ttmp/2026/01/06/MO-007-LOG-PARSER--javascript-log-processor-for-devctl/design-doc/03-roadmap-design-from-fan-out-log-parse-to-logflow-ish-system.md`
- Compare it against the alignment target:
  - `/home/manuel/workspaces/2026-01-06/log-parser-module/devctl/ttmp/2026/01/06/MO-007-LOG-PARSER--javascript-log-processor-for-devctl/design-doc/02-next-step-design-multi-script-pipeline-for-log-parse.md`
- Confirm the task breakdown:
  - `docmgr task list --ticket MO-007-LOG-PARSER`

### Technical details
- Roadmap highlights (next-phase building blocks):
  - multi-module loader (`--module`, `--modules-dir`) + `validate`
  - fan-out runner that emits tagged derived events
  - stdlib v1 helpers as safe globals/namespaced object (no `require()` yet)
  - multiline buffering as a per-module helper with single-worker semantics

## Step 13: Implement Fan-Out Multi-Module Runner + CLI Loading/Validation + Tests

Implemented the first “consequence jump” after the MVP: `log-parse` now loads **many self-contained modules** and runs them in **fan-out** on the same input stream, emitting 0..N tagged derived events per input line. This makes the tool immediately useful for real workflows where you want multiple “views” of the same log stream (errors/metrics/security) without forcing everything into one large JS script.

This step also added “confidence tooling” (`validate`, `--print-pipeline`, `--stats`) and multi-module tests (tagging, state isolation, and error isolation). As part of this change, the CLI flag changed from `--js` to `--module`; docs and ticket scripts were updated so the fast feedback loop stays intact.

**Commit (code):** N/A (this workspace has no `.git` directory, so I could not produce a commit hash)

### What I did
- Implemented a multi-module fan-out runner:
  - Added `devctl/pkg/logjs/fanout.go` with `LoadFanoutFromFiles` and `(*Fanout).ProcessLine`.
  - Added tagging injection for each emitted event (`event.tags`, `event.fields._tag`, `event.fields._module`).
- Extended the module metadata:
  - `register({ tag })` support (defaults to `name`): `devctl/pkg/logjs/module.go`
  - `Module.Info()` and `Module.ScriptPath()` for pipeline introspection.
- Updated `log-parse` CLI:
  - `--module` (repeatable), `--modules-dir` (repeatable; non-recursive), deterministic directory load order
  - `validate` subcommand
  - `--print-pipeline` and `--stats`
- Added multi-module tests:
  - `devctl/pkg/logjs/fanout_test.go`
- Updated examples + ticket scripts to use `--module`:
  - `devctl/examples/log-parse/README.md`
  - `devctl/ttmp/2026/01/06/MO-007-LOG-PARSER--javascript-log-processor-for-devctl/scripts/02-run-examples.sh`
  - `devctl/ttmp/2026/01/06/MO-007-LOG-PARSER--javascript-log-processor-for-devctl/scripts/03-timeout-demo.sh`
  - `devctl/ttmp/2026/01/06/MO-007-LOG-PARSER--javascript-log-processor-for-devctl/playbook/01-playbook-log-parse-mvp-testing.md`
- Checked off tasks: 8, 9, 10, 14, 15, 16, 17, 24.
- Validation runs:
  - `cd devctl && gofmt -w ./cmd/log-parse/main.go ./pkg/logjs/module.go ./pkg/logjs/fanout.go ./pkg/logjs/types.go`
  - `cd devctl && go test ./... -count=1`
  - `bash devctl/ttmp/2026/01/06/MO-007-LOG-PARSER--javascript-log-processor-for-devctl/scripts/02-run-examples.sh`
  - `bash devctl/ttmp/2026/01/06/MO-007-LOG-PARSER--javascript-log-processor-for-devctl/scripts/03-timeout-demo.sh`

### Why
- Fan-out multi-module execution is the simplest way to get real “pipeline-like” power without introducing JS-level code sharing or stage-coupling.
- Tagging every derived output stream makes downstream routing and merging trivial.
- Validation/introspection reduces “silent failure” modes and makes the tool approachable for teams.

### What worked
- Multi-module fan-out works end-to-end (multiple scripts, multiple outputs per line).
- `validate` prints a concise module summary and catches duplicate names.
- Tests cover the most important correctness properties for this iteration: tag injection, per-module state isolation, per-module error isolation.

### What didn't work
- The initial upload to reMarkable failed because PDFs already existed:
  - `Error: entry already exists (use --force to recreate, --content-only to replace content)`
  - Fixed by rerunning with `--force`.

### What I learned
- `goja.Object.Get("missing")` can yield a nil `goja.Value`; checking “nullish” should include nil (we used `isNullish` when reading optional `tag`).
- It’s worth wiring `validate` early: it provides an immediate safety net before we expand the stdlib surface.

### What was tricky to build
- Designing tagging injection so it’s robust and non-invasive:
  - don’t clobber user-provided `fields._tag/_module`
  - avoid duplicate tags in `event.tags`
- Evolving CLI ergonomics without a compatibility layer: switching from `--js` to `--module` required updating every script/playbook quickly so the project stays runnable.

### What warrants a second pair of eyes
- The “fatal error” vs “recoverable per-module hook error” boundary in `Fanout.ProcessLine`: currently `Module.ProcessLine` swallows hook errors and returns `(nil, nil)`; confirm this is the right long-term contract for fan-out.
- Tag key naming (`_tag`, `_module`) and whether they should be reserved/namespaced more strongly.

### What should be done in the future
- Implement task 18: allow `parse`/`transform` to return arrays (0..N events), which will matter for multiline and “explode” semantics.
- Implement task 19: optional dead-letter/error stream to NDJSON, so failures are observable without parsing stderr.

### Code review instructions
- Start with:
  - `/home/manuel/workspaces/2026-01-06/log-parser-module/devctl/pkg/logjs/fanout.go`
  - `/home/manuel/workspaces/2026-01-06/log-parser-module/devctl/cmd/log-parse/main.go`
  - `/home/manuel/workspaces/2026-01-06/log-parser-module/devctl/pkg/logjs/fanout_test.go`
- Validate:
  - `cd devctl && go test ./... -count=1`
  - `cd devctl && go run ./cmd/log-parse validate --module examples/log-parse/parser-json.js`
  - `bash devctl/ttmp/2026/01/06/MO-007-LOG-PARSER--javascript-log-processor-for-devctl/scripts/02-run-examples.sh`

### Technical details
- `log-parse` now emits 0..N events per input line (fan-out across modules), while each module still emits 0..1 event per line for now; arrays are deferred to task 18.
- Tag injection rules are implemented in `injectTag` (fanout) rather than in `Module.normalizeEvent`, so the module remains unaware of runner composition.

## Step 14: Implement Multi-Event Returns + Error Stream + Stdlib Helpers (Tasks 18–22)

Expanded the engine so a module’s `parse` (and `transform`) can return **arrays** of event-like values, enabling 0..N derived events per line and setting the stage for multiline buffering and “explode” transforms. In the same pass, we added a structured error record type and wired an optional **dead-letter/error stream** from the CLI so hook failures are observable as NDJSON instead of only via stderr text.

This step also grew the “stdlib v1” helpers surface toward the upstream spec: parsing helpers (`parseKeyValue`, `capture`), event/path helpers (`getPath`/`hasPath`, tag helpers), and time/numeric helpers (`parseTimestamp`, `toNumber`). `parseTimestamp` is implemented in Go (best-effort parsing via `dateparse`) and exposed as `log.parseTimestamp` for predictable behavior across environments.

**Commit (code):** N/A (this workspace has no `.git` directory, so I could not produce a commit hash)

### What I did
- Engine: allow `parse`/`transform` to return arrays:
  - Updated `devctl/pkg/logjs/module.go` to normalize `null|string|object|array` into 0..N events, and to treat errors per-event (continue processing remaining candidates).
  - Updated fan-out and CLI call sites for the new multi-return signature.
- Engine: structured error records:
  - Added `logjs.ErrorRecord` in `devctl/pkg/logjs/types.go`.
  - `Module.ProcessLine` now returns `(events, errors, err)` and records hook failures as `ErrorRecord` (including timeout classification).
  - `Fanout.ProcessLine` aggregates module error records.
- CLI: dead-letter/error stream:
  - Added `--errors <path|stderr|->` to `log-parse` to emit `ErrorRecord` as NDJSON.
  - Guarded against mixing NDJSON errors with `--print-pipeline`/`--stats` on stderr.
- Stdlib v1 helpers:
  - Updated `devctl/pkg/logjs/helpers.go` to add:
    - parsing: `log.parseKeyValue`, `log.capture`
    - path: `log.getPath`, `log.hasPath` (and `log.field` delegates to `getPath`)
    - tags: `log.addTag`, `log.removeTag`, `log.hasTag`
    - numeric: `log.toNumber`
    - time: `log.parseTimestamp` (fallback in JS, overridden by Go)
  - Injected Go-backed `log.parseTimestamp` via `injectGoHelpers` in `devctl/pkg/logjs/module.go` (uses `github.com/araddon/dateparse`).
- Tests:
  - Updated existing tests for new return signatures.
  - Added coverage for parse/transform returning arrays, error records, and `log.parseTimestamp`.
- Validation:
  - `cd devctl && go test ./... -count=1`
  - `bash devctl/ttmp/2026/01/06/MO-007-LOG-PARSER--javascript-log-processor-for-devctl/scripts/01-go-test.sh`
  - `bash devctl/ttmp/2026/01/06/MO-007-LOG-PARSER--javascript-log-processor-for-devctl/scripts/02-run-examples.sh`

### Why
- Arrays are a prerequisite for multiline buffering and for derived streams where one input yields multiple outputs.
- A structured error stream is essential once we have many modules: errors must be routable/observable without scraping logs.
- Stdlib helpers (parsing + path access + time/numeric) are the highest-leverage parts of the upstream spec for day-to-day usability.

### What worked
- Multi-event returns work for both `parse` and `transform`.
- Hook errors produce `ErrorRecord` while allowing other modules/events to continue processing.
- `log.parseTimestamp` reliably converts many time strings into a JS `Date` that normalizes to ISO via our existing timestamp normalization.

### What didn't work
- N/A (no new blockers encountered; existing tests and scripts were updated as needed).

### What I learned
- goja’s behavior around “missing properties” can yield nil `goja.Value`; optional fields should be checked with a helper like `isNullish` rather than only `goja.IsUndefined`.
- A Go-backed `parseTimestamp` is worth it: relying on JS `Date` parsing would make behavior platform-dependent and weaker for non-ISO formats.

### What was tricky to build
- Keeping stats behavior reasonable when “one line → many events”:
  - `LinesProcessed` stays line-based
  - `EventsEmitted` becomes per output event
  - `LinesDropped` now more accurately means “candidate events dropped” (parse/filter/transform yielding null/false), which is fine but worth remembering.

### What warrants a second pair of eyes
- The semantics of `LinesDropped` vs “event candidates dropped”: confirm we want to keep this counter as-is or add a new counter for per-event drops once multiline/aggregation arrives.
- Error record privacy: `RawLine` is currently included when available; decide whether default CLI behavior should redact by default for sensitive log streams.

### What should be done in the future
- Implement tasks 23 (multiline buffer helper) and 25/26 (many-module examples + scripts) now that arrays + errors + stdlib helpers are in place.

### Code review instructions
- Start with:
  - `/home/manuel/workspaces/2026-01-06/log-parser-module/devctl/pkg/logjs/module.go`
  - `/home/manuel/workspaces/2026-01-06/log-parser-module/devctl/pkg/logjs/helpers.go`
  - `/home/manuel/workspaces/2026-01-06/log-parser-module/devctl/pkg/logjs/fanout.go`
  - `/home/manuel/workspaces/2026-01-06/log-parser-module/devctl/cmd/log-parse/main.go`
- Validate:
  - `cd devctl && go test ./... -count=1`
  - `cd devctl && printf '%s\n' 'x' | go run ./cmd/log-parse --module examples/log-parse/parser-json.js --errors stderr >/dev/null`

### Technical details
- Supported event-like returns from `parse`/`transform`:
  - `null`/`undefined` → no events
  - `string` → `{ message: string }`
  - `object` → one event-like object
  - `array` → flattened list of the above (nulls skipped)
- `log.parseTimestamp`:
  - JS fallback uses `new Date(value)` and returns null if invalid.
  - Go override uses `dateparse.ParseAny` (and optionally Go layouts if a formats array is provided).

## Step 15: Add Multiline Buffer Helper + Many-Module Example Suite (Tasks 23, 12/13, 25/26)

Implemented the first usable version of multiline log handling via a `log.createMultilineBuffer(...)` helper, and added a concrete “many modules” example suite that demonstrates the fan-out model with tagged derived streams. This step turns the system into something you can actually experiment with as a developer: run a directory of modules against sample input, inspect pipeline/stats, and iterate without rewriting one giant script.

This step intentionally keeps multiline semantics deterministic and single-threaded: there’s no background timer, and flushing by timeout happens only when the next line arrives. That keeps behavior predictable under goja and is sufficient for the initial “devctl logs streaming” use case.

**Commit (code):** N/A (this workspace has no `.git` directory, so I could not produce a commit hash)

### What I did
- Implemented `log.createMultilineBuffer` in the stdlib prelude:
  - `/home/manuel/workspaces/2026-01-06/log-parser-module/devctl/pkg/logjs/helpers.go`
- Added unit test coverage for the multiline buffer behavior (match=after, negate=true):
  - `/home/manuel/workspaces/2026-01-06/log-parser-module/devctl/pkg/logjs/module_test.go`
- Added a “many modules” example directory + sample input:
  - `/home/manuel/workspaces/2026-01-06/log-parser-module/devctl/examples/log-parse/modules/`
  - `/home/manuel/workspaces/2026-01-06/log-parser-module/devctl/examples/log-parse/sample-fanout-json-lines.txt`
  - `/home/manuel/workspaces/2026-01-06/log-parser-module/devctl/examples/log-parse/README.md`
- Added ticket scripts to exercise/validate the many-module setup:
  - `/home/manuel/workspaces/2026-01-06/log-parser-module/devctl/ttmp/2026/01/06/MO-007-LOG-PARSER--javascript-log-processor-for-devctl/scripts/04-run-fanout-modules-dir-demo.sh`
  - `/home/manuel/workspaces/2026-01-06/log-parser-module/devctl/ttmp/2026/01/06/MO-007-LOG-PARSER--javascript-log-processor-for-devctl/scripts/05-validate-fanout-modules-dir.sh`
- Ran:
  - `cd devctl && go test ./... -count=1`
  - `bash devctl/ttmp/2026/01/06/MO-007-LOG-PARSER--javascript-log-processor-for-devctl/scripts/02-run-examples.sh`
  - `bash devctl/ttmp/2026/01/06/MO-007-LOG-PARSER--javascript-log-processor-for-devctl/scripts/03-timeout-demo.sh`
  - `bash devctl/ttmp/2026/01/06/MO-007-LOG-PARSER--javascript-log-processor-for-devctl/scripts/04-run-fanout-modules-dir-demo.sh`
  - `bash devctl/ttmp/2026/01/06/MO-007-LOG-PARSER--javascript-log-processor-for-devctl/scripts/05-validate-fanout-modules-dir.sh`
- Checked off tasks: 23, 25, 26, and also completed earlier “multi-module scripts” tasks 12/13.

### Why
- Multiline buffering is one of the most valuable “real log” features from the upstream spec, and it depends on task 18 (multi-event returns) which we already implemented.
- A many-module example suite is essential for confidence and onboarding: it makes the fan-out model tangible and testable.
- Ticket scripts preserve a fast, repeatable validation loop for future refactors (stdlib expansion, multiline refinements, registry/require work).

### What worked
- The multiline helper supports the most important initial mode for stack traces: `match="after"` with `negate=true` and a continuation-pattern regex.
- The many-module demo produces multiple outputs per line with stable tagging and prints pipeline/stats cleanly.

### What didn't work
- I accidentally ran `gofmt` on `devctl/examples/log-parse/README.md` and got:
  - `./examples/log-parse/README.md:1:1: illegal character U+0023 '#'`
  - Fixed by only running `gofmt` on `.go` files.

### What I learned
- When embedding JS/regex in Go raw strings, it’s easy to accidentally over-escape (`\\s` vs `\s`), which changes regex meaning; tests are critical here.
- “Timeout” in multiline helpers is tricky without a background goroutine; making it “flush-on-next-line” keeps implementation safe and deterministic for now.

### What was tricky to build
- Choosing semantics that are useful without over-promising:
  - We support `match="after"` now (the dominant real-world case).
  - We document that flushing is driven by incoming lines (no background timer).

### What warrants a second pair of eyes
- Multiline semantics in `createMultilineBuffer`: confirm the chosen interpretation matches how you expect to model stack traces and “start line vs continuation line” detection.
- Whether we need an explicit EOF flush mechanism soon for file-oriented parsing (last buffered record).

### What should be done in the future
- Consider adding an explicit end-of-input flush path (CLI-level or hook-level) so the final buffered multiline record isn’t lost when parsing finite files.
- Implement the next helper families from the spec as needed (rate limiting, remember/recall, fingerprint, CIDR helpers), but keep safety boundaries intact.

### Code review instructions
- Review the multiline helper:
  - `/home/manuel/workspaces/2026-01-06/log-parser-module/devctl/pkg/logjs/helpers.go`
- Review examples and scripts:
  - `/home/manuel/workspaces/2026-01-06/log-parser-module/devctl/examples/log-parse/modules/01-errors.js`
  - `/home/manuel/workspaces/2026-01-06/log-parser-module/devctl/examples/log-parse/README.md`
  - `/home/manuel/workspaces/2026-01-06/log-parser-module/devctl/ttmp/2026/01/06/MO-007-LOG-PARSER--javascript-log-processor-for-devctl/scripts/04-run-fanout-modules-dir-demo.sh`
- Validate:
  - `bash devctl/ttmp/2026/01/06/MO-007-LOG-PARSER--javascript-log-processor-for-devctl/scripts/01-go-test.sh`
  - `bash devctl/ttmp/2026/01/06/MO-007-LOG-PARSER--javascript-log-processor-for-devctl/scripts/04-run-fanout-modules-dir-demo.sh`

### Technical details
- `log.createMultilineBuffer` returns `{ add(line), flush() }`:
  - `add(line)` returns `null` while accumulating and returns a joined `"\n"` string when it decides a record is complete.
  - `timeout` parsing supports `ms|s|m|h` and only triggers on the next `add(...)` call (no background timer).

## Step 16: Investigate Long-Term Documentation Candidates for Log-Parse

Reviewed MO-007 and related ticket docs to identify what should migrate out of `ttmp/` into durable documentation. The goal was to inventory long-term design/playbook material and outline missing docs needed to help teams adopt and integrate the log-parser feature.

This step produced a new analysis document that lists migration candidates (design docs, playbook, sandboxing analysis) and outlines a focused set of future docs based on the feature surface documented in the diary and `git log --stat`.

**Commit (code):** N/A (documentation analysis only)

### What I did
- Listed ticket docs for MO-007 and MO-008 with `docmgr doc list`.
- Reviewed MO-007 design docs, playbook, and diary for long-term doc candidates.
- Reviewed MO-008 sandboxing design doc for security guidance to preserve.
- Checked `git log --stat -- cmd/log-parse pkg/logjs` to confirm the implemented feature surface.
- Created `analysis/01-long-term-documentation-investigation.md` and related key sources.

### Why
- Long-term docs in `ttmp/` are hard to discover; migrating them to `docs/` improves onboarding and future maintenance.
- The log-parser feature has grown beyond MVP; the current doc set lacks user-facing references for CLI usage and integration.

### What worked
- docmgr listing made it clear which docs already declare long-term intent.
- The diary and git history provided a reliable map of current functionality to backfill into documentation.

### What didn't work
- N/A

### What I learned
- The most valuable existing docs to migrate are the MVP design, fan-out design, roadmap, testing playbook, and require() sandboxing analysis.
- The current documentation gap is primarily user-facing: CLI reference, JS API contract, helpers, schemas, and integration guidance.

### What was tricky to build
- Distinguishing “long-term architectural guidance” from ticket-local artifacts (diary, source imports) so we don’t over-migrate.

### What warrants a second pair of eyes
- Confirm the proposed migration targets and directory layout for long-term docs (`docs/log-parser/` vs `docs/architecture/log-parser/`).
- Validate that the proposed new docs list aligns with current team onboarding and devctl integration priorities.

### What should be done in the future
- Execute the migration of the identified docs into `docs/` and update references in the repo.
- Draft the highest-priority missing docs: CLI reference, JS module API, event/error schema.

### Code review instructions
- Review the analysis doc:
  - `/home/manuel/workspaces/2026-01-06/log-parser-module/devctl/ttmp/2026/01/06/MO-007-LOG-PARSER--javascript-log-processor-for-devctl/analysis/01-long-term-documentation-investigation.md`
- Spot-check the referenced sources:
  - `/home/manuel/workspaces/2026-01-06/log-parser-module/devctl/ttmp/2026/01/06/MO-007-LOG-PARSER--javascript-log-processor-for-devctl/design-doc/01-mvp-design-javascript-log-parser-goja.md`
  - `/home/manuel/workspaces/2026-01-06/log-parser-module/devctl/ttmp/2026/01/06/MO-008-REQUIRE-SANDBOX--sandbox-goja-nodejs-require-for-log-parse/design-doc/02-analysis-sandboxing-goja-nodejs-require.md`

### Technical details
- Investigated docs under:
  - `ttmp/2026/01/06/MO-007-LOG-PARSER--javascript-log-processor-for-devctl/`
  - `ttmp/2026/01/06/MO-008-REQUIRE-SANDBOX--sandbox-goja-nodejs-require-for-log-parse/`
- Git history inspected:
  - `git log --stat -- cmd/log-parse pkg/logjs`
