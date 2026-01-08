---
Title: Diary
Ticket: MO-011-IMPLEMENT-STREAMS
Status: active
Topics:
    - streams
    - tui
    - plugins
DocType: reference
Intent: long-term
Owners: []
RelatedFiles:
    - Path: devctl/pkg/runtime/client.go
      Note: |-
        StartStream implementation and current capability gating behavior (ops vs streams).
        StartStream capability check nuance discovered during investigation
    - Path: devctl/pkg/runtime/router.go
      Note: |-
        Stream event routing, buffering, and end-of-stream channel closure logic.
        Router buffering explains why early events are not lost
    - Path: devctl/pkg/tui/action_runner.go
      Note: |-
        Current TUI event pipeline (actions + pipeline phases) where a future stream runner would plug in.
        Current action/event pipeline that will need a stream runner sibling
    - Path: devctl/ttmp/2026/01/07/MO-011-IMPLEMENT-STREAMS--implement-streams/analysis/01-streams-codebase-analysis-and-tui-integration.md
      Note: |-
        Main deliverable analysis document for MO-011.
        Main MO-011 analysis deliverable
ExternalSources: []
Summary: 'Work log for MO-011 stream analysis: ticket creation, stream-related code inventory, and synthesis into a TUI integration plan.'
LastUpdated: 2026-01-07T16:20:08.05684409-05:00
WhatFor: Capture the investigation trail and key discoveries about devctl stream plumbing (protocol/runtime) and the gaps to integrate streams into the current TUI architecture.
WhenToUse: When continuing MO-011 implementation work, reviewing why particular files were identified as integration points, or validating that analysis assumptions match code.
---


# Diary

## Goal

Record the step-by-step investigation and documentation work for MO-011, including what was searched/read, what conclusions were drawn about streams, and how to validate/review the resulting analysis.

## Step 1: Create ticket + inventory stream plumbing

This step created the MO-011 ticket workspace and then did a codebase-wide inventory of “stream” concepts, focusing on devctl’s plugin protocol streams (`event` frames keyed by `stream_id`) rather than the TUI’s stdout/stderr log switching. The output is a textbook-style analysis that ties together protocol docs, runtime implementation, fixtures, and the current TUI event bus so implementation can proceed with fewer surprises.

The key outcome was confirming that streams are already implemented and tested in `devctl/pkg/runtime`, but there is effectively no production integration yet: the CLI and TUI follow service logs by tailing files, and the TUI bus does not carry stream events. The analysis also surfaced a correctness/robustness foot-gun: `StartStream` currently allows “streams-only” capability declarations, which would hang against an existing fixture plugin that advertises a stream but never responds.

### What I did
- Created ticket workspace and initial docs via `docmgr`.
- Searched for stream-related code paths and fixtures with `rg`, then read the relevant Go and plugin files with `sed`.
- Cross-referenced prior ticket docs (MO-005/MO-006/MO-009/MO-010) to understand intended stream usage and expected TUI integration.
- Wrote the textbook analysis document `analysis/01-streams-codebase-analysis-and-tui-integration.md`.

### Why
- Streams are a partially implemented feature with multiple similarly named “streams” concepts; mapping the ground truth prevents implementing the wrong thing.
- TUI integration requires touching multiple layers (bus types, transformer, forwarder, models); doing a call-graph style inventory first reduces churn.
- Existing fixture plugins intentionally misbehave (streams advertised but no responses); the analysis needs to bake in hang-prevention constraints.

### What worked
- `docmgr ticket create-ticket` and `docmgr doc add` produced a consistent ticket workspace under `devctl/ttmp/2026/01/07/`.
- `rg` surfaced the critical stream implementation files quickly (`runtime/client.go`, `runtime/router.go`, runtime tests, and plugin authoring docs).
- Prior docs (especially MO-010’s runtime client reference and MO-005’s logs.follow schema) provided concrete protocol shapes to anchor the analysis.

### What didn't work
- `rg -n "PipelineLiveOutput|StepProgress|ConfigPatches" devctl/pkg/tui/action_runner.go` returned no matches (confirming the current action runner does not emit live pipeline output/progress/config patch events).

### What I learned
- `runtime.Client.StartStream` exists and is tested, but there are no production call sites using it today; the “streams feature” is currently dormant outside tests/docs.
- The current TUI is event-driven (Watermill → transformer → forwarder → Bubble Tea), but it has no stream event types; adding streams implies adding new domain/UI envelopes and a dedicated runner.
- Stream capability semantics matter: a fixture plugin advertises `capabilities.streams` without implementing any ops or responses, so treating “streams list” as an invocation permission will recreate timeout/hang failure modes.

### What was tricky to build
- Terminology collision: “stream” refers both to protocol streams (`event` frames) and to local stdout/stderr log files in the Service view. The analysis had to disambiguate these to avoid misleading integration guidance.
- Design drift: MO-006’s TUI layout docs describe a richer topic-based bus (e.g., `cmd.logs.follow` → `service.logs.line`), while the current TUI implementation uses a different envelope scheme and tails log files directly.
- Capability semantics drift: docs and fixtures sometimes treat `capabilities.streams` as a declaration of stream-producing ops, but the runtime currently treats it as an allowlist for starting a stream (which is dangerous with misbehaving fixtures).

### What warrants a second pair of eyes
- The proposed capability semantics (treat `ops` as authoritative for stream-start requests) should be sanity-checked against the intended protocol contract and existing fixtures/docs.
- The “best first UI surface” for streams (Service view vs new Streams view vs Events view) is a product decision; the analysis presents options but implementation should confirm the UX direction.

### What should be done in the future
- Implement MO-011: add a stream runner + bus plumbing + UI surface, and validate against both “good” streaming fixtures and “bad” streams-advertising fixtures.

### Code review instructions
- Start with `devctl/ttmp/2026/01/07/MO-011-IMPLEMENT-STREAMS--implement-streams/analysis/01-streams-codebase-analysis-and-tui-integration.md`.
- Validate key claims by spot-checking:
  - `devctl/pkg/runtime/client.go` (`StartStream` capability check and response parsing),
  - `devctl/pkg/runtime/router.go` (buffering + `event=end` behavior),
  - `devctl/pkg/tui/transform.go` / `devctl/pkg/tui/forward.go` (no stream message types today),
  - fixture script `devctl/ttmp/2026/01/06/MO-009-TUI-COMPLETE-FEATURES--complete-tui-features-per-mo-006-design/scripts/setup-comprehensive-fixture.sh` (streams advertised without response behavior).

### Technical details
- Commands run (representative):
  - `docmgr ticket create-ticket --ticket MO-011-IMPLEMENT-STREAMS --title "Implement streams" --topics streams,tui,plugins`
  - `docmgr doc add --ticket MO-011-IMPLEMENT-STREAMS --doc-type analysis --title "Streams: codebase analysis and TUI integration"`
  - `rg -n "\\bstream(s|ing)?\\b" -S devctl moments pinocchio glazed`
  - `sed -n '1,260p' devctl/pkg/runtime/client.go`
  - `sed -n '1,260p' devctl/pkg/runtime/router.go`

## Step 2: Make stream-start capability gating fail fast

This step implemented the key robustness decision from the analysis/design docs: starting a stream is still invoking a request op, so it must be gated by `handshake.capabilities.ops`. The goal was to prevent the “streams-only advertised, never responds” hang class before we add any production call sites (TUI runner or CLI).

The concrete outcome is that `runtime.Client.StartStream` now short-circuits with `E_UNSUPPORTED` unless the op is present in `capabilities.ops`, and a new unit test asserts the behavior is truly “fails fast” (not “wait for deadline then fail”) against a plugin that ignores unknown ops.

**Commit (code):** a2013d4 — "runtime: gate StartStream on ops"

### What I did
- Updated `StartStream` to require `op ∈ handshake.capabilities.ops` (treat `capabilities.streams` as informational only for invocation).
- Added `TestRuntime_StartStreamUnsupportedFailsFast` mirroring the existing “call unsupported fails fast” test.
- Ran `gofmt` and `go test ./...` in `devctl/`.

### Why
- The repo already contains (and future plugins will likely contain) “streams advertised but not actually implemented” cases; waiting for timeouts would make the TUI/CLI feel hung.
- Fixing this in the runtime makes future call sites simpler and harder to get wrong.

### What worked
- The new test reliably asserts `errors.Is(err, ErrUnsupported)` and `!errors.Is(err, context.DeadlineExceeded)` for `StartStream` on an unsupported op.
- Existing stream tests (`TestRuntime_Stream`, `TestRuntime_StreamClosesOnClientClose`) still pass.

### What didn't work
- N/A

### What I learned
- `ignore-unknown` is a good “misbehaving plugin” fixture for proving fail-fast behavior because it consumes stdin but never responds to unknown ops.

### What was tricky to build
- Ensuring the test proves “fast” failure required using a short context deadline and explicitly asserting it did not trip the deadline error path.

### What warrants a second pair of eyes
- Confirm the intended semantics: should `capabilities.streams` ever be considered authoritative for invocation, or should it remain purely UI/metadata? This change commits to “metadata only”.

### What should be done in the future
- Update any future stream-start call sites (TUI runner, CLI) to rely on `SupportsOp`/runtime gating and keep `capabilities.streams` as presentation/informational unless a stricter rule is adopted.

### Code review instructions
- Review `devctl/pkg/runtime/client.go` and focus on `StartStream`’s capability check.
- Review `devctl/pkg/runtime/runtime_test.go` and focus on `TestRuntime_StartStreamUnsupportedFailsFast`.
- Validate: `cd devctl && go test ./... -count=1`.

### Technical details
- Commands run:
  - `gofmt -w devctl/pkg/runtime/client.go devctl/pkg/runtime/runtime_test.go`
  - `cd devctl && go test ./...`

## Step 3: Add telemetry + negative stream fixtures (tests prove “no hangs”)

This step added two new plugin fixtures under `devctl/testdata/plugins/` so stream behavior can be validated without touching any real repo plugins. One fixture is “happy path telemetry” (a deterministic finite stream), and the other is a deliberately misbehaving plugin that advertises `capabilities.streams` but not `capabilities.ops` and never responds—exactly the kind of thing that would previously cause a hang in stream-start code paths.

The key outcome is a concrete safety net: runtime tests now cover both “telemetry stream works” and “streams-only advertisement does not enable invocation,” ensuring that future UIStreamRunner/CLI work can rely on fail-fast behavior rather than timeouts.

**Commit (code):** 25819fd — "runtime: add telemetry and negative stream fixtures"

### What I did
- Added `devctl/testdata/plugins/telemetry/plugin.py` implementing `telemetry.stream` and emitting 3 deterministic `metric` events then `end`.
- Added `devctl/testdata/plugins/streams-only-never-respond/plugin.py` that advertises `streams:["telemetry.stream"]`, declares `ops:[]`, and never responds.
- Added runtime tests:
  - `TestRuntime_TelemetryStreamFixture`
  - `TestRuntime_StartStreamIgnoresStreamsCapabilityForInvocation`
- Ran `go test ./...` to confirm no regressions.

### Why
- The TUI runner and CLI will depend on stream semantics; having fixtures makes both development and regression testing straightforward.
- The negative fixture encodes the real-world failure mode we care about: “stream capability advertised” must not imply “safe to invoke”.

### What worked
- The telemetry fixture is deterministic and yields a stable assertion (`[0,1,2]` counter values) without relying on timing beyond a tiny sleep.
- The negative fixture test demonstrates that `StartStream("telemetry.stream")` fails with `E_UNSUPPORTED` even though the op is listed in `capabilities.streams`.

### What didn't work
- N/A

### What I learned
- Having the plugin end on its own (finite count) is much easier to test than relying on client-close-driven termination.

### What was tricky to build
- `protocol.Event.Fields` values arrive as `float64` when encoded/decoded through JSON, so the test needs to handle numeric types carefully.

### What warrants a second pair of eyes
- Confirm that the telemetry fixture event schema (`fields.name/value/unit`) is a good canonical pattern for future telemetry/metrics streams in real plugins.

### What should be done in the future
- Use these fixtures as the baseline validation path for `devctl stream start` and `UIStreamRunner` (happy path + hang-proofing).

### Code review instructions
- Review the fixtures:
  - `devctl/testdata/plugins/telemetry/plugin.py`
  - `devctl/testdata/plugins/streams-only-never-respond/plugin.py`
- Review tests in `devctl/pkg/runtime/runtime_test.go` for both positive and negative behaviors.
- Validate: `cd devctl && go test ./... -count=1`.

### Technical details
- Commands run:
  - `cd devctl && go test ./...`

## Step 7: Implement `devctl stream start` CLI (stream debugging harness)

This step added a dedicated CLI surface for streams so we can start a stream op and observe `protocol.Event` frames without the TUI. This is both a real feature (for power users / scripting) and a practical debugging tool for plugin authors: it makes it easy to test “does my stream start quickly, does it emit events, does it end cleanly?” without needing to wire a full UI.

The command follows the same safety rules as the TUI: it selects a plugin provider, gates on `SupportsOp(op)` before attempting to start the stream, and prints events until the stream ends (or the user interrupts).

**Commit (code):** 12a85fd — "cmds: add devctl stream start"

### What I did
- Added `devctl stream start` in `devctl/cmd/devctl/cmds/stream.go`:
  - `--op` required, `--plugin` optional (defaults to first plugin supporting op by priority)
  - `--input-json` / `--input-file` for request input
  - `--start-timeout` for the initial `StartStream` request/response (getting `stream_id`)
  - `--json` for raw `protocol.Event` JSONL output
- Wired the command into the CLI in `devctl/cmd/devctl/cmds/root.go`.
- Ran `go test ./...` in `devctl/`.

### Why
- The easiest way to validate streaming behavior is a minimal CLI that prints raw events.
- It reduces “TUI required” coupling and helps debug protocol issues (E_UNSUPPORTED vs hang vs stdout contamination).

### What worked
- Provider selection uses handshake gating (`SupportsOp`) so it won’t hang against “streams-only” fixtures.
- Output supports both human and JSONL modes.

### What didn't work
- I initially attempted `git commit -m "cmds: add \`devctl stream start\`"` and zsh treated the backticks as command substitution, causing:
  - `zsh:1: command not found: devctl`
  - and an incomplete commit message.
  I fixed this by amending the commit message with safe quoting.

### What I learned
- Avoid backticks in shell-quoted commit messages under zsh; use plain text or single quotes.

### What was tricky to build
- Choosing timeouts: we need a short start timeout (get `stream_id`) but the stream itself should usually run until `end`/interrupt, not until the global `--timeout`.

### What warrants a second pair of eyes
- CLI UX: whether `--start-timeout` is the right knob name and whether we should also support an optional overall max duration flag.

### What should be done in the future
- Add a docs/playbook snippet that shows running `devctl stream start` against the telemetry fixture/config for quick validation.

### Code review instructions
- Review `devctl/cmd/devctl/cmds/stream.go` for provider selection and output formatting.
- Review `devctl/cmd/devctl/cmds/root.go` for command registration.
- Validate: `cd devctl && go test ./... -count=1`.

### Technical details
- Commands run:
  - `cd devctl && go test ./...`

## Step 6: Add a Streams view to the TUI (start/stop + render stream events)

This step added an actual UI surface for streams so the newly implemented `UIStreamRunner` can be exercised from within the TUI. The Streams view provides a minimal but functional workflow: start a stream by pasting JSON (op/plugin_id/input), watch events arrive, stop the stream, and clear per-stream event history.

The main outcome is that streams are now end-to-end usable inside the TUI process: the view publishes `tui.stream.start` and `tui.stream.stop` requests (via RootModel), the runner starts the plugin stream and emits `stream.*` domain events, the transformer/forwarder delivers those as `tui.stream.*` messages, and the Streams view renders them.

**Commit (code):** bbe7e27 — "tui: add Streams view"

### What I did
- Added `StreamsModel` in `devctl/pkg/tui/models/streams_model.go`:
  - `n` opens a JSON prompt for `{op, plugin_id?, input?}` and publishes a stream start request
  - `j/k` selects a stream, `↑/↓` scrolls within the event viewport
  - `x` stops the selected stream, `c` clears its event buffer
- Integrated Streams view into `RootModel`:
  - new `ViewStreams` and tab-cycle `plugins → streams → dashboard`
  - routes `StreamStartedMsg` / `StreamEventMsg` / `StreamEndedMsg` into `StreamsModel` even when not active
  - added help + footer keybinds for Streams
- Ran `go test ./...` in `devctl/`.

### Why
- Without a Streams view, the runner and message plumbing were “headless” and harder to validate.
- The JSON-based “new stream” prompt is a low-friction way to support arbitrary stream ops without building complex forms yet.

### What worked
- Streams view compiles cleanly and uses the same Bubble Tea patterns as Events/Service (textinput + viewport).
- Stream event spam stays contained to the Streams view; the global Events log only gets “started/ended” entries.

### What didn't work
- N/A

### What I learned
- Using JSON as the initial “command language” avoids premature UX decisions while still making streams debuggable and flexible.

### What was tricky to build
- Coordinating keybindings so stream selection and viewport scrolling don’t fight: selection uses `j/k`, scrolling uses `↑/↓`.

### What warrants a second pair of eyes
- UI ergonomics: whether `j/k` vs `↑/↓` is the right split for selection vs scrolling.
- Whether the Streams view should eventually support multiple panes (stream list + event list) more cleanly.

### What should be done in the future
- Add a small “start telemetry stream” shortcut that autofills JSON for common ops (telemetry/logs.follow) once we pick first-class ops.
- Add batching/coalescing in the runner or model if telemetry events prove too high-frequency.

### Code review instructions
- Review `devctl/pkg/tui/models/streams_model.go` for the UI surface and message emission.
- Review `devctl/pkg/tui/models/root_model.go` for view integration and message routing.
- Validate: `cd devctl && go test ./... -count=1` and run `devctl tui` manually.

### Technical details
- Commands run:
  - `cd devctl && go test ./...`

## Step 5: Implement `UIStreamRunner` (central stream lifecycle management in the TUI process)

This step implemented the first production subsystem that actually calls `runtime.Client.StartStream`: `UIStreamRunner`. It follows the same architectural pattern as `UIActionRunner`, but for long-lived stream lifecycles instead of short-lived pipeline actions. The runner centralizes start/stop, chooses a plugin client, publishes `stream.*` domain events, and ensures plugin processes are cleaned up when streams end or are stopped.

The key outcome is that the TUI now has a correct “backend” for streams: if the UI publishes a `tui.stream.start` request, the runner will start a plugin, initiate the stream (with a short start timeout), forward events into the bus, and publish a terminal `stream.ended` event. This unblocks building a Streams UI view next.

**Commit (code):** e0db4d5 — "tui: add UIStreamRunner"

### What I did
- Added `RegisterUIStreamRunner` in `devctl/pkg/tui/stream_runner.go`:
  - consumes `tui.stream.start` / `tui.stream.stop` on `TopicUIActions`
  - loads repo config via `repository.Load`
  - starts plugin clients via `runtime.Factory`
  - gates stream-start on `SupportsOp(op)` and then calls `StartStream` with a 2s start timeout
  - publishes domain events: `stream.started`, `stream.event`, `stream.ended`
  - stop semantics: close the per-stream client (v1 = one client per stream)
- Wired the runner into the TUI startup alongside the action runner in `devctl/cmd/devctl/cmds/tui.go`.
- Ran `go test ./...` in `devctl/`.

### Why
- Streams require long-lived goroutines and resource cleanup; concentrating that in a single runner avoids model-level leaks and “forgot to stop” bugs.
- `UIStreamRunner` makes stream usage testable and debuggable independently of any specific UI view.

### What worked
- The runner builds on the existing envelope/bus architecture cleanly and compiles without changes to unrelated packages.
- Per-stream “one client per stream” makes stop semantics reliable without needing protocol-level `stream.stop` yet.

### What didn't work
- N/A

### What I learned
- Picking a plugin “by op support” (when plugin_id isn’t provided) requires starting plugins to read handshakes; doing that per stream is acceptable for v1 but will motivate future caching/stop semantics.

### What was tricky to build
- Avoiding duplicate/multiple `stream.ended` emissions required centralizing ended emission in the event-forwarding goroutine.
- Telemetry flood risks are real; this runner intentionally does not try to mirror events into the global Events log (transformer logs only start/end).

### What warrants a second pair of eyes
- The “one client per stream” choice is pragmatic but potentially heavy; review whether we should pursue a protocol stop op sooner.
- Confirm the selection policy when `plugin_id` is omitted (first plugin supporting op by priority/id) is acceptable.

### What should be done in the future
- Implement a Streams UI view that can publish `tui.stream.start` requests and render `tui.stream.*` messages.

### Code review instructions
- Review `devctl/pkg/tui/stream_runner.go` end-to-end (it owns lifecycle, selection, and publishing).
- Review `devctl/cmd/devctl/cmds/tui.go` to confirm the runner is wired in.
- Validate: `cd devctl && go test ./... -count=1`.

### Technical details
- Commands run:
  - `cd devctl && go test ./...`

## Step 4: Add TUI stream message plumbing (topics + transformer + forwarder)

This step introduced the basic “wiring harness” needed for streams to exist as first-class events inside the TUI process, without actually starting any streams yet. The focus was on adding stable message types and ensuring the existing Watermill pipeline can carry stream lifecycle events from a future runner into Bubble Tea.

The practical outcome is that we can now publish a `tui.stream.start` request (from the UI) and receive `tui.stream.*` messages in Bubble Tea, once a `UIStreamRunner` starts emitting the corresponding domain events. This keeps stream management centralized and prevents models from calling `StartStream` directly.

**Commit (code):** 472593f — "tui: add stream message plumbing"

### What I did
- Added stream topic constants:
  - domain: `stream.started`, `stream.event`, `stream.ended`
  - ui: `tui.stream.start`, `tui.stream.stop`, `tui.stream.started`, `tui.stream.event`, `tui.stream.ended`
- Added stream event/request structs in `devctl/pkg/tui/stream_events.go`.
- Added `PublishStreamStart/PublishStreamStop` helpers (mirroring `PublishAction`) in `devctl/pkg/tui/stream_actions.go`.
- Extended:
  - `devctl/pkg/tui/transform.go` to map domain stream events to UI messages (and log only start/end to avoid telemetry spam),
  - `devctl/pkg/tui/forward.go` to forward stream UI messages into Bubble Tea,
  - `devctl/pkg/tui/models/root_model.go` to accept `StreamStartRequestMsg`/`StreamStopRequestMsg` and publish to the bus.
- Wired publish functions into `devctl tui` startup via `devctl/cmd/devctl/cmds/tui.go`.
- Ran `gofmt` and `go test ./...`.

### Why
- The TUI already has a clean separation: side-effectful subsystems publish domain events, transformer maps to UI envelopes, forwarder injects into Bubble Tea. Streams should reuse this pipeline.
- By adding message plumbing first, we can implement `UIStreamRunner` next with less churn and clearer responsibilities.

### What worked
- The new plumbing compiles cleanly and all tests pass.
- Transformer avoids flooding the global Events log by not echoing every `stream.event` as a text log line.

### What didn't work
- N/A

### What I learned
- RootModel already has a nice “request message → publish → append EventLogEntry” pattern for actions; streams fit the same structure cleanly.

### What was tricky to build
- Making stream events visible without accidentally creating an unbounded “event log spam” path required being deliberate in `transform.go` about what becomes a log line vs a typed message.

### What warrants a second pair of eyes
- Confirm the chosen “log only start/end” policy in `devctl/pkg/tui/transform.go` is acceptable for early debugging; we may want a debug flag that also logs individual stream events.

### What should be done in the future
- Implement `UIStreamRunner` to actually start plugin streams and publish `stream.*` domain events.

### Code review instructions
- Review `devctl/pkg/tui/topics.go`, `devctl/pkg/tui/stream_events.go`, and `devctl/pkg/tui/msgs.go` for the new message surface.
- Review `devctl/pkg/tui/transform.go` and `devctl/pkg/tui/forward.go` for correct mapping/forwarding behavior.
- Review `devctl/pkg/tui/models/root_model.go` for the publish-on-request pattern.

### Technical details
- Commands run:
  - `cd devctl && go test ./...`
