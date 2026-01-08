---
Title: 'Pipeline TUI phase inspection: data model and sources'
Ticket: MO-014-IMPROVE-PIPELINE-TUI
Status: active
Topics:
    - devctl
    - tui
    - pipeline
    - ux
DocType: analysis
Intent: long-term
Owners: []
RelatedFiles:
    - Path: devctl/pkg/engine/pipeline.go
      Note: Merge logic that currently discards provenance
    - Path: devctl/pkg/supervise/supervisor.go
      Note: Supervise phase behavior and per-service start/health data
    - Path: devctl/pkg/tui/action_runner.go
      Note: Pipeline run orchestrator and current event publishers
    - Path: devctl/pkg/tui/models/pipeline_model.go
      Note: Current Pipeline view rendering and focus behavior
    - Path: devctl/pkg/tui/pipeline_events.go
      Note: Pipeline phases and current payload shapes
    - Path: devctl/pkg/tui/topics.go
      Note: Domain/UI event type registry (missing types for richer pipeline data)
    - Path: devctl/pkg/tui/transform.go
      Note: Domain→UI transformer (needs wiring for new pipeline payloads)
ExternalSources: []
Summary: ""
LastUpdated: 2026-01-08T02:26:24.382490034-05:00
WhatFor: ""
WhenToUse: ""
---


# Pipeline TUI Phase Inspection: Data Model and Sources

## 1. Problem Statement

devctl’s pipeline is the product: plugins compute config/build/prepare/validate/launch plans, and devctl executes supervision/state management. When something goes wrong (or when it goes right but unexpectedly), developers need to answer:

- What happened in each pipeline phase?
- Which plugin(s) were called, in what order, and with what results?
- What data was produced (config patches, step results, artifacts, service specs)?
- What was merged/overridden (and why) under `--strict` vs non-strict behavior?
- What side effects occurred (processes started/stopped, state saved/removed)?

The current Pipeline view in the TUI provides phase timing plus a small subset of results (build/prepare steps, validation issues, launch plan service names). It is not sufficient for debugging or for understanding “what devctl did” at a reasonable level of detail.

This document defines, for each pipeline phase, the information that should be shown when the phase is focused, and where that data can be sourced from in the codebase (including what must be instrumented because it is not currently available).

## 2. Scope and Non-Goals

### In scope

- A per-phase spec of what to show in the Pipeline view when focused.
- A concrete mapping from “desired UI fields” → “data source” (existing struct/function) or “gap” (needs instrumentation).
- A proposal for a structured “pipeline trace” model that captures provenance and merge decisions.

### Non-goals (for this analysis doc)

- Implementing the UI and instrumentation (this ticket will follow up with tasks once this doc is reviewed).
- Redesigning plugin protocol shapes beyond what is needed for inspection (we prefer to infer from existing op outputs where possible).
- TUI visual design work (this is about the data contract and where it comes from).

## 3. Existing Pipeline + TUI Plumbing (Current State)

This section summarizes the current code paths so the “where to get it from” mapping is precise.

### 3.1. Pipeline phases (canonical list)

Phases are enumerated in `devctl/pkg/tui/pipeline_events.go` as `tui.PipelinePhase`:

- Plugin-driven phases (protocol ops):
  - `mutate_config` (plugin op: `config.mutate`)
  - `build` (plugin op: `build.run`)
  - `prepare` (plugin op: `prepare.run`)
  - `validate` (plugin op: `validate.run`)
  - `launch_plan` (plugin op: `launch.plan`)
- devctl-internal phases:
  - `supervise` (spawn processes, wait health checks)
  - `state_save` (persist `.devctl/state.json`)
- teardown phases (used by `down`/`restart`):
  - `stop_supervise`
  - `remove_state`

### 3.2. Where phases run

The TUI action runner orchestrates pipeline runs in `devctl/pkg/tui/action_runner.go`:

- `runUp(...)` executes:
  - `engine.Pipeline.MutateConfig` → `engine.Pipeline.Build` → `engine.Pipeline.Prepare` → `engine.Pipeline.Validate` → `engine.Pipeline.LaunchPlan`
  - then `supervise.Supervisor.Start` (unless dry-run)
  - then `state.Save` (unless dry-run)
- `runDown(...)` executes:
  - `supervise.Supervisor.Stop` (if state exists)
  - `state.Remove`

### 3.3. What the pipeline engine returns (merged-only)

Engine result types live in:
- `devctl/pkg/engine/types.go` (service spec, health, build/prepare step result, artifacts)
- `devctl/pkg/engine/pipeline.go` (merge logic)

Important observation:

The engine merges per-plugin results into a single result (config, launch plan, build/prepare results, validate results), and the merge process currently discards *provenance* (which plugin produced which step/artifact/service/error) and discards *merge decisions* (what got overridden when `Strict=false`).

### 3.4. What the Pipeline TUI currently renders

The pipeline UI model is `devctl/pkg/tui/models/pipeline_model.go`.

It can render:
- phase list with status + durations (from `PipelinePhaseStarted/Finished`)
- build steps + prepare steps + artifacts count (from `PipelineBuildResult`/`PipelinePrepareResult`)
- validation errors and warnings with details (from `PipelineValidateResult`)
- launch plan service names (from `PipelineLaunchPlan`)

There are message structs for richer data (but they’re not wired end-to-end today):
- `PipelineConfigPatchesMsg` / `PipelineLiveOutputMsg` / `PipelineStepProgressMsg` in `devctl/pkg/tui/msgs.go`
- corresponding data structs in `devctl/pkg/tui/pipeline_events.go`

However:
- `devctl/pkg/tui/topics.go` does not define domain/ui types for these messages.
- `devctl/pkg/tui/transform.go` does not transform/publish them.
- `devctl/pkg/tui/action_runner.go` does not publish them.

So in practice, config patches and live output are not displayed during real pipeline runs.

## 4. Proposed “Pipeline Trace” Model (What We Need to Capture)

To make phase inspection genuinely useful, we need a data model that answers “what happened” at multiple levels of detail:

1. **Phase summary**: ok/failed, duration, counts (steps/services/errors), and major reasons.
2. **Per-plugin call provenance**: which plugin was called for which op, in what order, with what result.
3. **Merge decisions**: collisions and overrides (services, steps, artifacts, config keys) including strictness behavior.
4. **Side effects**: processes started/stopped, health checks performed, state file written/removed.

### 4.1. Minimal trace (can be implemented without protocol changes)

For each pipeline run (`RunID`), collect and publish:

- `RepoRoot`, `ActionKind` (`up|down|restart`), strict/dry-run flags (currently only implicit)
- For each phase:
  - started/finished times, duration, ok/error string
  - a phase-specific “result summary” payload (see section 5)
- For plugin-driven phases:
  - a list of “plugin op calls”:
    - `PluginID` (from `runtime.Client.Spec().ID`)
    - `Op` string (`config.mutate`, `build.run`, ...)
    - `StartedAt`, `FinishedAt`, `DurationMs`
    - `Ok` and error (including `runtime.OpError` fields if present)
    - output summary derived from decoded response output (e.g., patch keys, step list, service list)

This does not require changing plugins; it requires instrumenting the loops where devctl calls clients and where devctl merges results.

### 4.2. Rich trace (may require protocol additions)

Optional enhancements (high value, but can be staged later):

- Per-step live output and progress:
  - Requires either:
    - a protocol extension (build/prepare streaming events tied to steps), or
    - devctl executing steps itself (not current design).
- Detailed per-step failure reasons:
  - StepResult currently has no error message, no logs; add optional fields (or a “step details” map).
- Validation provenance:
  - `protocol.Error` does not include `plugin_id`; devctl can attach provenance by wrapping errors into `details.plugin_id` in the trace.

## 5. Phase-by-Phase Inspection Spec (What to Show + Where to Get It)

This is the core of the ticket. For each phase:
- what to show when focused (summary + details)
- existing source(s)
- gaps and exactly where instrumentation must be added

### 5.1. Phase: `mutate_config` (plugin op `config.mutate`)

#### Focus view: summary

- Phase status: ok/failed, duration.
- “Config changed?”: number of patch entries applied (set/unset count).
- Top-level key prefixes touched (e.g., `env.*`, `services.*`, `ports.*`) for quick scanning.

#### Focus view: details

- **Patch list (ordered by call order):**
  - plugin id (`runtime.Client.Spec().ID`)
  - patch operation (`set`/`unset`)
  - dotted key (e.g., `services.api.port`)
  - value (JSON preview, truncated)
- **Merge/collision report (only when key is set multiple times):**
  - key
  - first writer plugin → last writer plugin
  - strictness behavior:
    - in strict mode: error out at collision point
    - non-strict: show override occurred (“last wins”)
- **Resulting config preview (optional, gated):**
  - show only relevant subtrees (filter by prefix)
  - redact secrets (see cross-cutting section)

#### Where to get it (current)

- The current phase exists only as timing:
  - publisher: `runUp` in `devctl/pkg/tui/action_runner.go`
  - phase state: `PipelinePhaseStarted/Finished` in `devctl/pkg/tui/pipeline_events.go`
- The actual mutation logic:
  - `engine.Pipeline.MutateConfig` in `devctl/pkg/engine/pipeline.go`
  - config patch application: `patch.Apply` in `devctl/pkg/patch/patch.go`

#### Gaps / instrumentation needed

- `engine.Pipeline.MutateConfig` discards the patch output per plugin; it only returns the final config.
- `patch.Apply` does not report “collision” information; it blindly writes keys.

**Proposed instrumentation points:**

- Change `engine.Pipeline.MutateConfig` to optionally return a trace:
  - per plugin call: returned patch, list of keys set/unset
  - keep an ordered list (call order matters)
- Track collisions by maintaining a `map[key]firstWriter` and reporting re-writes.
- Publish a new domain+UI event:
  - `pipeline.config.patches` (already modeled as `PipelineConfigPatches` in `devctl/pkg/tui/pipeline_events.go`)
  - Add types in `devctl/pkg/tui/topics.go` and wiring in `devctl/pkg/tui/transform.go`
  - Publish from `runUp` after `MutateConfig` completes (or incrementally after each plugin).

### 5.2. Phase: `build` (plugin op `build.run`)

#### Focus view: summary

- Phase status: ok/failed, duration.
- Steps:
  - number of steps requested vs returned
  - number ok vs failed
- Artifacts:
  - count and top N keys

#### Focus view: details

- **Step list (with provenance):**
  - plugin id
  - step name
  - ok/failed
  - duration
  - if available: error message, links to log output
- **Artifacts table (with provenance):**
  - plugin id
  - artifact key
  - artifact value (path/url), normalized to abs path if relevant
  - collisions (key overwritten by later plugin when strict=false)
- **Live output pane (if supported):**
  - stream: stdout/stderr
  - source: step name (and plugin id)
  - line

#### Where to get it (current)

- Engine result:
  - `engine.Pipeline.Build` returns `engine.BuildResult` in `devctl/pkg/engine/pipeline.go`
  - `engine.BuildResult.Steps` and `.Artifacts` in `devctl/pkg/engine/types.go`
- Action runner publishes summary:
  - `publishPipelineBuildResult` in `devctl/pkg/tui/action_runner.go`
  - event type `PipelineBuildResult` in `devctl/pkg/tui/pipeline_events.go`

#### Gaps / instrumentation needed

- Current UI receives only merged step results and artifacts, without plugin provenance.
- No live output publishing (even though types exist).

**Proposed instrumentation points:**

- In `engine.Pipeline.Build`, track per-plugin results before merge:
  - `plugin_id` via `runtime.Client.Spec().ID`
  - step collision decisions
  - artifact collision decisions
- Publish an enriched build result event:
  - either extend `PipelineBuildResult` to include provenance fields, or add `PipelineBuildResultV2`.
- Add domain/ui types and forwarding for live output if we implement it:
  - `pipeline.build.live_output` → `PipelineLiveOutputMsg`
  - `pipeline.build.step_progress` → `PipelineStepProgressMsg`

### 5.3. Phase: `prepare` (plugin op `prepare.run`)

Same shape as `build`:

#### Focus view: summary
- status, duration
- steps ok/failed counts
- artifacts count

#### Focus view: details
- per-plugin step list + collisions
- per-plugin artifacts + collisions
- optional live output/progress

#### Where to get it (current)
- `engine.Pipeline.Prepare` in `devctl/pkg/engine/pipeline.go`
- published via `publishPipelinePrepareResult` in `devctl/pkg/tui/action_runner.go`

#### Gaps / instrumentation needed
- identical to `build` (provenance, collisions, optional output/progress)

### 5.4. Phase: `validate` (plugin op `validate.run`)

#### Focus view: summary

- status, duration
- valid vs invalid
- error count, warning count

#### Focus view: details

- **Issue list**:
  - kind (error/warn)
  - code
  - message
  - plugin id (provenance)
  - details (JSON pretty print, truncated)
- **Suggested remediations (optional)**
  - if `details` includes known keys, present a human hint (but keep raw details visible)

#### Where to get it (current)

- Engine result:
  - `engine.Pipeline.Validate` merges `engine.ValidateResult` in `devctl/pkg/engine/pipeline.go`
  - error shape: `protocol.Error` in `devctl/pkg/protocol/types.go`
- Action runner publishes:
  - `PipelineValidateResult` in `devctl/pkg/tui/pipeline_events.go`
  - publisher in `devctl/pkg/tui/action_runner.go`
- UI already renders issues with details:
  - `renderStyledValidation` in `devctl/pkg/tui/models/pipeline_model.go`

#### Gaps / instrumentation needed

- No plugin provenance is preserved: `protocol.Error` has no plugin id, and merge logic does not annotate errors.

**Proposed instrumentation points:**

- In `engine.Pipeline.Validate`, when collecting errors/warnings from each client:
  - annotate each `protocol.Error` by injecting `details.plugin_id = <id>` if not already present
  - optionally inject `details.op = validate.run`
- Alternatively, publish a per-plugin validate result event, and have the UI group by plugin.

### 5.5. Phase: `launch_plan` (plugin op `launch.plan`)

#### Focus view: summary

- status, duration
- service count
- collision count (services overwritten) and strictness mode

#### Focus view: details

- **Services table (full spec):**
  - service name
  - plugin id (provenance)
  - cwd (resolved absolute path)
  - command (argv array)
  - env: count + selected keys (with redaction rules)
  - health check (type + address/url + timeout)
- **Collision report:**
  - service name
  - old provider plugin → new provider plugin
  - strictness behavior (“error” vs “last wins”)

#### Where to get it (current)

- Engine merge:
  - `engine.Pipeline.LaunchPlan` in `devctl/pkg/engine/pipeline.go`
  - service spec: `engine.ServiceSpec` in `devctl/pkg/engine/types.go`
- Action runner currently publishes only service names:
  - `PipelineLaunchPlan` in `devctl/pkg/tui/pipeline_events.go`
  - from `runUp` in `devctl/pkg/tui/action_runner.go`

#### Gaps / instrumentation needed

- Only service names are published today; the UI can’t show cwd/command/env/health.
- Merge logic discards provenance and collision decisions.

**Proposed instrumentation points:**

- Extend the published event to include full `[]engine.ServiceSpec` (or a TUI-safe copy).
- In `engine.Pipeline.LaunchPlan`, record provenance:
  - `service_name → plugin_id`
  - override list when `Strict=false`
- Publish a `PipelineLaunchPlan` payload that includes:
  - `Services []ServiceSpecWithProvenance`
  - `Collisions []ServiceCollision`

### 5.6. Phase: `supervise` (devctl internal)

#### Focus view: summary

- status, duration
- services started count
- health checks: count and failures (if any)

#### Focus view: details

- **Per-service start details** (should match `state.ServiceRecord`):
  - service name
  - pid
  - started_at
  - cwd (resolved)
  - command
  - stdout/stderr log paths
  - wrapper vs direct mode (presence of `ExitInfo` indicates wrapper mode)
  - health config (type/address/url)
- **Health check status**:
  - waited or skipped
  - total ready time per service
  - last error text if timed out

#### Where to get it (current)

- Process starting and health checks:
  - `supervise.Supervisor.Start` and `startService` and `waitReady` in `devctl/pkg/supervise/supervisor.go`
- Persisted runtime state:
  - `state.State` and `state.ServiceRecord` in `devctl/pkg/state/state.go`
- Action runner:
  - publishes only phase timing (no per-service details) in `devctl/pkg/tui/action_runner.go`

#### Gaps / instrumentation needed

- We don’t publish any supervise result payload; only timing.
- Health check progress is invisible; only the final ok/fail.

**Proposed instrumentation points:**

- After `sup.Start(...)` succeeds in `runUp`, publish a `PipelineSuperviseResult` event:
  - `Services []state.ServiceRecord` (or a projection)
  - optionally: per-service “ready waited” and “ready duration”
- To capture health-check progress, instrument `waitReady` (or a wrapper around it) to publish attempt events (bounded).

### 5.7. Phase: `state_save` (devctl internal)

#### Focus view: summary

- status, duration
- state file path
- number of services saved

#### Focus view: details

- file size, last modified time
- a compact listing of services saved (name, pid)

#### Where to get it (current)

- write occurs in `state.Save` in `devctl/pkg/state/state.go`
- orchestrated from `runUp` in `devctl/pkg/tui/action_runner.go`

#### Gaps / instrumentation needed

- No payload is published; only timing.

**Proposed instrumentation points:**

- After saving, `os.Stat(state.StatePath(repoRoot))` to get size/mtime.
- Publish a `PipelineStateSaveResult` event.

### 5.8. Phase: `stop_supervise` (devctl internal, teardown)

#### Focus view: summary

- status, duration
- services attempted to stop
- failures count (if any)

#### Focus view: details

- per-service stop result:
  - pid, pgid (if available), termination strategy, timeout
  - whether process was alive at start/end
  - error if stop timed out or failed

#### Where to get it (current)

- `runDown` orchestrates:
  - `state.Load` + `supervise.Supervisor.Stop` in `devctl/pkg/tui/action_runner.go`
- `terminatePIDGroup` in `devctl/pkg/supervise/supervisor.go` is where the actual signal/timeout loop occurs.

#### Gaps / instrumentation needed

- Today, stop is a single boolean “ok” phase with no detail.

**Proposed instrumentation points:**

- Extend `Supervisor.Stop` (or wrap it) to return structured per-service stop results and publish them.

### 5.9. Phase: `remove_state` (devctl internal, teardown)

#### Focus view: summary

- status, duration
- state file path
- whether state existed

#### Focus view: details

- what files were removed (currently only `.devctl/state.json`)
- note that logs are not removed (so “down” does not delete log history)

#### Where to get it (current)

- `state.Remove` in `devctl/pkg/state/state.go` (removes only state.json)
- orchestrated by `runDown` in `devctl/pkg/tui/action_runner.go`

#### Gaps / instrumentation needed

- No details beyond phase ok/error.

**Proposed instrumentation points:**

- Stat before removal to show file size and “was present”.
- Publish a `PipelineRemoveStateResult` event.

## 6. Cross-Cutting UI/UX Requirements

### 6.1. Navigation and focusing

The Pipeline view should allow focusing *any* phase, not only build/prepare/validation.

Current keys in `devctl/pkg/tui/models/pipeline_model.go` only support:
- `b` build, `p` prepare, `v` validation, `o` output

Proposed:
- Cursor through phases list (up/down) and press enter to focus that phase.
- Phase-specific detail panes appear based on focused phase.

### 6.2. Provenance is mandatory

Without plugin provenance, the Pipeline view cannot answer “who did this?”.

When provenance is not available from protocol payloads, devctl should attach it while collecting results (it knows which client produced which output).

### 6.3. Size limits and truncation

Some data is unbounded:
- config patches can be large
- env maps can contain secrets and large values
- live output can be infinite

All “detail panes” must:
- cap line counts
- cap value sizes (preview + expand)
- support filtering by prefix / substring

### 6.4. Redaction / secrets

The UI must treat env/config values as sensitive by default:
- redact keys matching patterns (`*_TOKEN`, `*_PASSWORD`, `*_SECRET`, etc.)
- optionally allow reveal with an explicit action

### 6.5. Persisting pipeline traces

Currently pipeline info is only in-memory/UI-time.

We likely want a persisted “last run trace” for:
- post-mortem after closing TUI
- bug reports

Candidate location:
- `.devctl/pipeline/<run_id>.json` or `.devctl/pipeline_last.json`

This doc does not mandate persistence, but the data model should be serializable.

## 7. Incremental Implementation Strategy (Suggested)

To avoid a large refactor, implement in layers:

1. Wire the existing-but-unused pipeline messages end-to-end:
   - add domain/ui topic types for config patches/live output/step progress
   - implement publishers (even if coarse at first)
2. Add `launch_plan` details beyond service names.
3. Add provenance + collision reporting for mutate_config and launch_plan.
4. Add supervise/state_save/stop_supervise/remove_state result payloads.
5. Improve build/prepare provenance and artifact collisions.

## 8. Appendix: Key Symbols (Index)

- Phase definitions: `tui.PipelinePhase` in `devctl/pkg/tui/pipeline_events.go`
- Pipeline orchestrator: `runUp`, `runDown` in `devctl/pkg/tui/action_runner.go`
- Engine merges:
  - `engine.Pipeline.MutateConfig` / `Build` / `Prepare` / `Validate` / `LaunchPlan` in `devctl/pkg/engine/pipeline.go`
  - result types in `devctl/pkg/engine/types.go`
- Supervision:
  - `supervise.Supervisor.Start` / `Stop` in `devctl/pkg/supervise/supervisor.go`
  - state types in `devctl/pkg/state/state.go`
- TUI pipeline view: `PipelineModel` in `devctl/pkg/tui/models/pipeline_model.go`
- Event wiring:
  - topic strings in `devctl/pkg/tui/topics.go`
  - domain → UI mapping in `devctl/pkg/tui/transform.go`
