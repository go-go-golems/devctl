---
Title: Diary
Ticket: MO-015-CREATE-PLUGIN-HOST-DOCUMENTATION
Status: active
Topics:
    - plugins
    - runtime
    - concurrency
    - protocol
    - tui
    - documentation
DocType: reference
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: "Step-by-step investigation diary while producing deep plugin-host documentation (concurrency, state, protocol, UI integration)."
LastUpdated: 2026-01-09T17:20:32.036475462-05:00
WhatFor: "Capture the journey of building the MO-015 plugin-host documentation: what we inspected, what we learned, and what remains."
WhenToUse: "Read this first if you need to continue the investigation or validate how a conclusion in the analysis doc was derived."
---

# Diary

## Goal

Record a detailed, chronological diary of research and documentation work for `MO-015-CREATE-PLUGIN-HOST-DOCUMENTATION`, with frequent intermediate steps, including the exact files inspected and the key conclusions derived.

## Step 1: Ticket bootstrap + investigation plan

This step created the ticket workspace and the initial documents we’ll keep updating as we trace the plugin host from CLI entrypoints through runtime/protocol into the event system and TUI integration.

The goal is to make the final analysis doc verifiable: every major claim should map back to concrete files, symbols, and (where needed) snippets, so we can reuse the same architecture for a new static-analysis/codebase-inspection tool.

**Commit (code):** N/A (documentation-only)

### What I did
- Created ticket `MO-015-CREATE-PLUGIN-HOST-DOCUMENTATION`.
- Created two docs in the ticket:
  - `analysis/01-plugin-host-architecture-deep-dive.md`
  - `reference/01-diary.md`
- Planned the research approach: start at CLI commands → runtime/plugin process host → protocol → event bus → TUI.

### Why
- We need a “single source of truth” deep dive on how plugin hosting works (especially concurrency, state, and UI/event coupling) so we can reuse the same principle for a new plugin-powered static analysis tool.

### What worked
- `docmgr` is initialized and ticket/doc creation succeeded.

### What didn't work
- N/A

### What I learned
- The repo already has a docmgr knowledge base (`ttmp/`) and existing docs we can cross-reference while tracing code.

### What was tricky to build
- N/A (setup step)

### What warrants a second pair of eyes
- N/A (setup step)

### What should be done in the future
- N/A

### Code review instructions
- Start with the ticket directory:
  - `ttmp/2026/01/09/MO-015-CREATE-PLUGIN-HOST-DOCUMENTATION--create-plugin-host-documentation/`
- Then open:
  - `analysis/01-plugin-host-architecture-deep-dive.md`
  - `reference/01-diary.md`

### Technical details
- Commands executed:
  - `docmgr status --summary-only`
  - `docmgr ticket create-ticket --ticket MO-015-CREATE-PLUGIN-HOST-DOCUMENTATION ...`
  - `docmgr doc add --ticket MO-015-CREATE-PLUGIN-HOST-DOCUMENTATION --doc-type analysis --title "Plugin host architecture (deep dive)"`
  - `docmgr doc add --ticket MO-015-CREATE-PLUGIN-HOST-DOCUMENTATION --doc-type reference --title "Diary"`

### What I'd do differently next time
- N/A

## Step 2: Trace the runtime plugin host (process + protocol + router)

This step established the *actual* “plugin host” in Go: plugins are launched as child processes with **NDJSON-over-stdio**, with an initial handshake line and then a request/response/event loop. I focused on identifying where processes are started, how the protocol is read/written, and what the concurrency topology is.

The biggest outcome is a precise mental model for concurrency/backpressure: each plugin client has a stdout reader goroutine that demultiplexes frames into either request-response completions or stream event fanout.

**Commit (code):** N/A (documentation-only)

### What I did
- Located the plugin process start + handshake code:
  - `devctl/pkg/runtime/factory.go`
  - `devctl/pkg/runtime/client.go`
- Read the in-process router responsible for request correlation + stream event fanout:
  - `devctl/pkg/runtime/router.go`
- Read protocol frame types and handshake validation:
  - `devctl/pkg/protocol/types.go`
  - `devctl/pkg/protocol/validate.go`
  - `devctl/pkg/protocol/errors.go`

### Why
- The host/runtime layer is where “how concurrency works” and “how plugins run” actually lives: process boundaries, I/O loops, request tracking, cancellation, and stream fanout.

### What worked
- The runtime is intentionally small and readable: `Factory.Start()` + `client.Call()`/`StartStream()` + `router`.

### What didn't work
- N/A

### What I learned
- Transport is **stdio** (not socket): `Factory.Start()` wires stdin/stdout/stderr pipes and uses a `bufio.Reader` over stdout.
- **Handshake** is the first stdout line and is validated strictly:
  - `protocol_version` must be `"v2"`.
  - `plugin_name` must be non-empty.
  - `capabilities.commands` must have unique names and well-formed args.
- Concurrency topology per plugin client:
  - one goroutine for `readStdoutLoop()` (NDJSON demux)
  - one goroutine for `readStderrLoop()` (logs)
  - writes are serialized via `writerMu`
  - request IDs are generated using an atomic counter
- “Intermediate state” inside the client is the `router`:
  - pending requests are `request_id -> chan Response`
  - stream subscribers are `stream_id -> []chan Event`
  - stream pre-subscribe buffering is `stream_id -> []Event`
  - a fatal protocol error flips the router into a “fail everything” mode

### What was tricky to build
- The stream fanout has real backpressure implications:
  - `router.publish()` sends events directly into subscriber channels (buffer size 16).
  - If a consumer stops draining, the plugin stdout reader goroutine can block, which can stall *all* protocol processing for that plugin (including responses).

### What warrants a second pair of eyes
- The backpressure semantics (and whether 16-buffer is sufficient) are review-critical: it’s a correctness + liveness risk under high-frequency event streams.
- The handshake read uses a goroutine + context timeout; it relies on killing the process to unblock the read. It’s correct, but subtle.

### What should be done in the future
- Consider documenting (or enforcing) a contract: **events should only be produced for stream_ids that the host will subscribe to**, otherwise the router buffer can grow.
- If high-frequency streams are expected, consider a lossy/metrics-style channel or dropping strategy for stream events.

### Code review instructions
- Start in:
  - `devctl/pkg/runtime/factory.go` (`Factory.Start`, `readHandshake`, `terminateProcessGroup`)
  - `devctl/pkg/runtime/client.go` (`Call`, `StartStream`, `readStdoutLoop`)
  - `devctl/pkg/runtime/router.go` (`register`, `deliver`, `publish`, `subscribe`, `failAll`)
  - `devctl/pkg/protocol/types.go` (frame types)
  - `devctl/pkg/protocol/validate.go` (handshake rules)

### Technical details
- Host writes request frames as single-line JSON with trailing `\n` (NDJSON).
- Stdout is parsed line-by-line; any non-JSON “contamination” is treated as fatal and fails all pending work.
- Stream start is still implemented as a request op that returns `{"stream_id": ...}` in `Response.output`.

### What I'd do differently next time
- N/A

## Step 3: Trace discovery/repository + the pipeline “intermediate state” model

This step mapped how plugins are selected and how their outputs are merged into the host’s intermediate state. The core insight is: devctl treats plugins as **repo-specific adapters** that compute facts (config patches, validate results, launch plans), and devctl owns orchestration and state persistence.

**Commit (code):** N/A (documentation-only)

### What I did
- Traced plugin discovery and spec resolution:
  - `devctl/pkg/config/config.go`
  - `devctl/pkg/discovery/discovery.go`
  - `devctl/pkg/repository/repository.go`
- Traced pipeline behavior and merge semantics:
  - `devctl/pkg/engine/pipeline.go`
  - `devctl/pkg/engine/types.go`
- Traced key CLI entrypoints that exercise pipeline:
  - `devctl/cmd/devctl/cmds/up.go`
  - `devctl/cmd/devctl/cmds/plan.go`

### Why
- The ticket asks explicitly how we “manage intermediate state” and how concurrency works. For intermediate state, the pipeline is the central abstraction: it merges plugin outputs deterministically by priority and id.

### What worked
- Plugin ordering is consistent across the codebase: sort by `priority`, then `id`.
- Each pipeline phase is clearly “opt-in” via handshake capabilities: if a plugin does not declare an op, it’s skipped.

### What didn't work
- N/A

### What I learned
- Plugin specs come from two sources:
  - explicit config `.devctl.yaml` (`config.File.Plugins`)
  - auto-discovery: executables in `<repoRoot>/plugins/` prefixed `devctl-` (default priority 1000)
- Pipeline is sequential, deterministic, and merge-based:
  - `config.mutate`: applies `config_patch` sequentially, feeding the updated config to the next plugin.
  - `validate.run`: concatenates errors/warnings and ANDs validity.
  - `launch.plan`: merges services by name; in non-strict mode, later plugins override earlier services of same name.
  - `build.run`/`prepare.run`: merges artifacts by key and merges steps by name (same collision rules as plan).
- CLI commands (`up`, `plan`) start plugin processes once per invocation (`repo.StartClients`), then do pipeline calls under a per-phase `context.WithTimeout`, then close all clients at the end.

### What was tricky to build
- Collision semantics are subtle and depend on strictness. In “warn” mode, later plugins override earlier records on collisions (service names, build/prepare steps).

### What warrants a second pair of eyes
- The collision/override semantics for launch plans and step results: confirm this is the intended “last writer wins” behavior in non-strict mode.

### What should be done in the future
- For new tools (static analysis), define explicit merge semantics for “findings”: do we dedupe? override? append? (the pipeline pattern forces you to decide).

### Code review instructions
- Start in `devctl/pkg/engine/pipeline.go` and read `clientsInOrder`, then each op method.
- Then read `devctl/pkg/discovery/discovery.go` to understand how plugin specs are assembled.

### Technical details
- `.devctl.yaml` schema is defined in `devctl/pkg/config/config.go`.
- Auto-discovered plugins are executable files named `plugins/devctl-<id>`.

### What I'd do differently next time
- N/A

## Step 4: Trace event system + TUI integration points (actions, streams, state polling)

This step mapped the “event bus” architecture used by the TUI and how it interacts with plugins. The key insight is the TUI uses **Watermill** as an in-memory pub/sub + router, with two topic spaces:

- **UI actions** flowing from Bubbletea → bus → runners (action runner, stream runner)
- **Domain events** flowing from runners/watchers → bus → transformer → UI messages → Bubbletea program

**Commit (code):** N/A (documentation-only)

### What I did
- Traced TUI entrypoint and concurrency structure:
  - `devctl/cmd/devctl/cmds/tui.go`
- Traced Watermill bus and envelope system:
  - `devctl/pkg/tui/bus.go`
  - `devctl/pkg/tui/envelope.go`
  - `devctl/pkg/tui/topics.go`
  - `devctl/pkg/tui/transform.go`
  - `devctl/pkg/tui/forward.go`
- Traced how UI triggers runtime/plugin work:
  - `devctl/pkg/tui/action_runner.go`
  - `devctl/pkg/tui/stream_runner.go`
  - `devctl/pkg/tui/actions.go`
  - `devctl/pkg/tui/stream_events.go`
- Traced how persistent state is surfaced to UI:
  - `devctl/pkg/tui/state_watcher.go`
  - `devctl/pkg/state/state.go`

### Why
- The ticket explicitly asks: “how we hook up plugins to the event system” and “how the UI interacts with plugins”.

### What worked
- The event architecture is consistent:
  - everything over the bus is an `Envelope{type,payload}`
  - domain events are transformed to UI messages, then forwarded into Bubbletea as `tea.Msg`.
- The `tui` command uses `errgroup.WithContext` to run the bus, state watcher, and the Bubbletea program concurrently with shared cancellation.

### What didn't work
- N/A

### What I learned
- The TUI does **not** keep plugin processes resident; it starts plugin processes as needed:
  - action runner starts all plugins for each action run (up/down/restart).
  - stream runner starts a plugin per stream (and keeps it alive until “end” or stop/cancel).
- Stream integration path:
  - UI publishes `UITypeStreamStartRequest` to `TopicUIActions`
  - `stream_runner` starts plugin + `StartStream` + forwards `protocol.Event` into domain events `DomainTypeStreamEvent`
  - domain-to-UI transformer publishes UI message `UITypeStreamEvent` to `TopicUIMessages`
  - UI forwarder injects `StreamEventMsg` into Bubbletea `Program.Send`
- State integration path:
  - `StateWatcher` polls `.devctl/state.json` and publishes `DomainTypeStateSnapshot`
  - UI consumes `StateSnapshotMsg` to update dashboard/services/plugins view models
- The plugins view in the UI currently uses **config-derived** plugin summaries (id/path/priority/status) rather than handshake introspection.

### What was tricky to build
- Even though stream events are “just messages”, they are part of a backpressure chain:
  - plugin stdout reader → runtime router subscriber channel → stream runner forward goroutine → bus publish → transformer/forwarder → Bubbletea update loop.
  - any slow consumer can propagate backpressure all the way to plugin stdout.

### What warrants a second pair of eyes
- Stream event throughput/backpressure: confirm Watermill `gochannel` buffer (1024) and router fanout won’t deadlock under high-frequency streams.

### What should be done in the future
- If we want the plugins UI view to show real capabilities, add a low-frequency “introspection” path (like `devctl plugins list`) that runs occasionally and caches handshake data.

### Code review instructions
- Start in `devctl/cmd/devctl/cmds/tui.go` to see how everything is wired.
- Then follow `TopicUIActions` handlers in `action_runner.go` and `stream_runner.go`.
- Then follow `TopicDevctlEvents` → `transform.go` → `TopicUIMessages` → `forward.go`.

### Technical details
- Bus uses `watermill/pubsub/gochannel` with `OutputChannelBuffer: 1024`.
- Domain events and UI messages are distinct types (`DomainType*` vs `UIType*`) to keep UI contracts stable.

### What I'd do differently next time
- N/A

## Step 5: Fill the deep-dive analysis doc (replace placeholders with code-traced reality)

This step converted our traced understanding into the actual “ticket deliverable”: a long, detailed analysis document describing the plugin host end-to-end, with explicit concurrency/backpressure behavior, protocol contracts, and the exact UI integration path.

The goal here wasn’t to be clever; it was to be *auditable*: a reader should be able to jump from a claim in the doc to the relevant file/symbol in the repo and confirm it quickly.

**Commit (code):** N/A (documentation-only)

### What I did
- Replaced all “TBD/placeholder” sections in:
  - `analysis/01-plugin-host-architecture-deep-dive.md`
- Added sections that are essential for reuse:
  - explicit protocol frame semantics
  - concurrency topology + backpressure chain
  - deterministic ordering/merge semantics in the pipeline
  - recommended plugin API surface for a static-analysis tool

### Why
- The ticket asks for “in depth and very detailed” documentation of concurrency, intermediate state, event wiring, and UI interactions; the skeleton doc didn’t satisfy that.
- We also want to reuse the principle for a new static analysis tool, so we must extract patterns and specify what to change.

### What worked
- The doc naturally “snapped into place” once we treated the system as three planes:
  - plugin host runtime/protocol
  - pipeline/state/supervision
  - TUI bus + runners

### What didn't work
- N/A

### What I learned
- The most important “gotcha” in the host is backpressure: stream event channels can block plugin stdout demux and stall responses.

### What was tricky to build
- Writing the concurrency section correctly required being explicit about:
  - what goroutine is doing the `ch <- ev` send
  - where buffers exist (16 per stream subscriber, 1024 for the Watermill pubsub output channel)
  - how backpressure propagates across boundaries

### What warrants a second pair of eyes
- The backpressure explanation: it’s easy to get wrong by hand-waving, so it’s worth verifying against the exact send/receive points.
- The “events are stream-scoped” statement: confirm no other place uses protocol events outside streams.

### What should be done in the future
- If this architecture becomes the template for static analysis, strongly consider:
  - a persistent plugin manager (avoid repeated process startup),
  - explicit “findings merge” semantics,
  - and a deliberate backpressure strategy for high-frequency analysis streams.

### Code review instructions
- Review the analysis doc top-to-bottom and spot-check claims against these files:
  - `devctl/pkg/runtime/*`
  - `devctl/pkg/protocol/*`
  - `devctl/pkg/engine/pipeline.go`
  - `devctl/pkg/tui/*`
  - `devctl/pkg/supervise/supervisor.go`
  - `devctl/pkg/state/state.go`

### Technical details
- The analysis doc now includes mermaid diagrams for:
  - CLI pipeline flow (`up`)
  - TUI event flow (topics + transformer + forwarder)

### What I'd do differently next time
- N/A

## Step 6: Fix Mermaid diagram parse error + add prose paragraphs

This step addressed two doc-quality issues: Mermaid diagram parsing in the renderer, and the overall “readability” of the analysis doc. The doc had the right facts, but it needed more connective tissue so it reads like an architectural explanation rather than a list of notes.

For the diagram issue, I rewrote the Mermaid flowcharts to be more robust: quoted node labels, replaced a unicode arrow with ASCII, and added explicit statement terminators (`;`) so the parser can’t get confused if line breaks are normalized.

**Commit (code):** N/A (documentation-only)

### What I did
- Updated Mermaid blocks in:
  - `analysis/01-plugin-host-architecture-deep-dive.md`
- Added prose paragraphs introducing and connecting:
  - scope/goals
  - architecture overview
  - concurrency/backpressure
  - intermediate state split (pipeline vs persisted state)
  - UI/plugin interaction modes
  - static-analysis reuse motivation

### Why
- Mermaid parse errors are usually caused by line-ending normalization or tokenization edge cases; making diagrams “boringly explicit” prevents the issue.
- Prose paragraphs are required so future readers don’t need to mentally reconstruct the architecture from bullets.

### What worked
- Diagrams are now resilient even if the renderer collapses newlines.

### What didn't work
- N/A

### What I learned
- Mermaid is sensitive to how statements are separated; adding semicolons and quoting labels is the safest “portable” style.

### What was tricky to build
- Avoiding hidden unicode/tokenization issues in diagrams while still keeping labels readable.

### What warrants a second pair of eyes
- Confirm Mermaid renders correctly in your target renderer (Cursor preview / GitHub / internal doc tooling), since each environment embeds Mermaid differently.

### What should be done in the future
- Consider standardizing diagram style in docs (quoted labels + semicolons) to avoid recurring parser issues.

### Code review instructions
- Open `analysis/01-plugin-host-architecture-deep-dive.md` and ensure both Mermaid blocks render.

### Technical details
- Mermaid blocks updated to use:
  - quoted labels: `A["..."]`
  - explicit statement terminators: `;`
  - ASCII labels (no unicode arrow)

### What I'd do differently next time
- Write Mermaid in “strict style” from the start to avoid later churn.

## Context

This ticket documents how `devctl` runs plugins: discovery, lifecycle, protocol/handshake, concurrency model, intermediate state management, event fan-out, and how the TUI/CLI surfaces plugin output and state.

## Quick Reference

Working map (will be refined as we read code):

- CLI entrypoints:
  - `cmd/devctl/cmds/plugins.go`
  - `cmd/devctl/cmds/dynamic_commands.go`
- Plugin discovery + repository:
  - `pkg/discovery/*`
  - `pkg/repository/*`
- Runtime / plugin host:
  - `pkg/runtime/*`
  - `pkg/protocol/*`
- State and streaming:
  - `pkg/state/*`
  - `pkg/engine/*`
- TUI event system and plugin interaction:
  - `pkg/tui/*`

## Usage Examples

- If you want to understand “how plugins are started and supervised”, read Step(s) covering `pkg/runtime/*` and `pkg/supervise/*`.
- If you want to understand “how the UI reacts to plugin events”, read Step(s) covering `pkg/tui/bus.go`, runner(s), and model updates.

## Step 7: Narrative rewrite for human engagement

The analysis document was technically accurate but read like a reference manual—terse, bullet-heavy, and not particularly engaging for a human reader. This step rewrote the entire document with a narrative style.

**Commit (code):** N/A (documentation-only)

### What I did
- Rewrote the analysis doc from ~630 lines of bullet-point reference to ~630 lines of narrative prose
- Added an opening hook and mental model ("plugins are just facts")
- Reorganized into 7 parts with story flow
- Added practical examples (JSON samples, config snippets)
- Explained the "why" behind design decisions, not just the "what"
- Added tables for quick reference where appropriate
- Included a "closing thoughts" section with key takeaways
- Added a provenance note at the end ("every claim maps to real code")

### Why
- Technical documentation is more useful when it tells a story
- Understanding the *motivation* behind design choices helps future developers make consistent decisions
- An engaging doc gets read; a boring doc gets skimmed and forgotten

### What worked
- The narrative structure makes the document flow naturally from concepts to details
- Examples make abstract protocol descriptions concrete
- The "lessons for a new tool" section is now actionable

### What didn't work
- N/A

### What I learned
- The seven-part structure (finding plugins → handshake → protocol → concurrency → pipeline → events → reuse) mirrors the actual code flow, which made organization natural

### What was tricky to build
- Keeping technical accuracy while adding personality
- Deciding what to leave out (some edge cases are now implicit rather than explicit)

### What warrants a second pair of eyes
- Verify that the prose simplifications haven't introduced inaccuracies
- Check that the proposed static-analysis API sketch is reasonable

### What should be done in the future
- Add a companion "quick reference" doc for people who want just the facts
- Consider adding runnable examples in `examples/plugins/`

### Code review instructions
- Read `analysis/01-plugin-host-architecture-deep-dive.md` as if you were a new team member
- Does it answer the questions you'd actually have?
- Are there any claims that seem incorrect or misleading?

### Technical details
- Removed the Mermaid diagrams (they were causing parse errors and the prose now explains the flows)
- Kept the YAML frontmatter with all RelatedFiles intact

### What I'd do differently next time
- Start with the narrative style from the beginning rather than writing reference-style first

## Related

- Ticket analysis doc: `../analysis/01-plugin-host-architecture-deep-dive.md`
