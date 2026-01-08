---
Title: Diary
Ticket: MO-014-IMPROVE-PIPELINE-TUI
Status: active
Topics:
    - devctl
    - tui
    - pipeline
    - ux
DocType: reference
Intent: long-term
Owners: []
RelatedFiles:
    - Path: devctl/pkg/tui/action_runner.go
      Note: Where to instrument additional phase payloads
    - Path: devctl/pkg/tui/models/pipeline_model.go
      Note: Current limitations and focus shortcuts
    - Path: devctl/ttmp/2026/01/08/MO-014-IMPROVE-PIPELINE-TUI--improve-pipeline-tui-phase-inspection/analysis/01-pipeline-tui-phase-inspection-data-model-and-sources.md
      Note: Primary analysis output for MO-014
ExternalSources: []
Summary: ""
LastUpdated: 2026-01-08T02:26:24.461508061-05:00
WhatFor: ""
WhenToUse: ""
---


# Diary

## Goal

Keep a precise research log for MO-014: what code was inspected, what the current TUI pipeline view can and cannot show, and which concrete data sources (symbols/files) can be used to improve phase inspection.

## Context

MO-014 is an analysis-first ticket. The goal is to define a clear “inspection contract” for the Pipeline TUI: for each pipeline phase, what information should be displayed when focused, and where that information comes from (or what instrumentation must be added).

## Step 1: Create Ticket Workspace + Identify the Core Code Paths

This step sets up the ticket workspace and finds the concrete entrypoints that define “the pipeline” for TUI runs. devctl already has a Pipeline view and a pipeline-events data model; the key question is why the view feels unhelpful and what data is missing at the source.

The high-level discovery is that the TUI pipeline run is orchestrated by `runUp`/`runDown` in `devctl/pkg/tui/action_runner.go`, which publishes only a subset of available pipeline event types. Several richer message structs exist (config patches, live output, step progress), but they are not currently wired from domain events → UI messages.

### What I did
- Created the MO-014 ticket workspace:
  - `docmgr ticket create-ticket --ticket MO-014-IMPROVE-PIPELINE-TUI ...`
- Created the analysis doc and this diary doc:
  - `docmgr doc add --ticket MO-014-IMPROVE-PIPELINE-TUI --doc-type analysis --title "Pipeline TUI phase inspection: data model and sources"`
  - `docmgr doc add --ticket MO-014-IMPROVE-PIPELINE-TUI --doc-type reference --title "Diary"`
- Located the pipeline phase enumeration and event payload types:
  - `devctl/pkg/tui/pipeline_events.go`
- Located the pipeline run orchestrator (publisher of pipeline events):
  - `devctl/pkg/tui/action_runner.go`

### Why
- Without a map of “who produces pipeline events”, it’s easy to mistakenly assume the UI model is the problem. For this ticket, the contract must start at the producer layer.

### What worked
- The codebase already has a phase model (`tui.PipelinePhase`) and event payloads for build/prepare/validate/launch-plan.

### What didn't work
- N/A.

### What I learned
- The current pipeline UI is limited primarily by what the action runner publishes, not by rendering logic alone.

### What was tricky to build
- N/A (setup and inspection).

### What warrants a second pair of eyes
- Whether we should treat the pipeline trace model as a first-class persisted artifact under `.devctl/` (not just UI-time events).

### What should be done in the future
- N/A (subsequent steps in this diary cover the deeper mapping).

### Code review instructions
- Start with:
  - `devctl/pkg/tui/action_runner.go`
  - `devctl/pkg/tui/pipeline_events.go`

### Technical details
- The canonical phase list includes both plugin ops and internal phases:
  - `mutate_config`, `build`, `prepare`, `validate`, `launch_plan`, `supervise`, `state_save`, `stop_supervise`, `remove_state`

## Step 2: Map Current Pipeline TUI Data to Producers (and Identify Gaps)

This step traces what the TUI Pipeline view *can currently show* back to the exact event publishers and payload shapes, then enumerates the missing pieces needed for meaningful phase inspection.

The key gap is that the pipeline UI model can render config patches and live output, but the bus topics and transformer don’t define/publish those event types. So the view lacks detail not because it can’t render, but because the data never arrives.

### What I did
- Read the pipeline UI model:
  - `devctl/pkg/tui/models/pipeline_model.go`
- Read the domain → UI transformer:
  - `devctl/pkg/tui/transform.go`
- Read the topic type registry:
  - `devctl/pkg/tui/topics.go`
- Confirmed which pipeline event types are published by the action runner:
  - `devctl/pkg/tui/action_runner.go`

### Why
- The ticket’s analysis doc needs to be explicit about “where to get data from”, and for TUI that means both the producer and the transform/forward layers.

### What worked
- The UI model already supports:
  - phase timeline
  - build/prepare step lists
  - validation error/warning details
  - launch plan service names

### What didn't work
- Config patches, live output, and step progress are defined as message types (`devctl/pkg/tui/msgs.go`) but are not:
  - defined as domain types in `devctl/pkg/tui/topics.go`
  - transformed in `devctl/pkg/tui/transform.go`
  - published in `devctl/pkg/tui/action_runner.go`

### What I learned
- Fixing the pipeline TUI meaningfully will likely require both:
  - publishing richer data for internal phases (`supervise`, `state_save`, teardown), and
  - instrumenting plugin phases to keep provenance (which plugin produced which config key/step/artifact/service).

### What was tricky to build
- Distinguishing “engine returns merged results” from “UI needs per-plugin provenance”: the merge step destroys information unless we explicitly record it during the loop.

### What warrants a second pair of eyes
- Whether we should evolve existing payload structs (breaking changes) or introduce `V2` payloads for backward compatibility in the bus.

### What should be done in the future
- Define a “pipeline trace” data model that captures per-plugin call provenance and merge decisions, and publish it in bounded chunks (phase-focused payloads).

### Code review instructions
- Start at the source:
  - `devctl/pkg/tui/action_runner.go`
- Then check wiring:
  - `devctl/pkg/tui/topics.go`
  - `devctl/pkg/tui/transform.go`
- Finally, see what the UI can render:
  - `devctl/pkg/tui/models/pipeline_model.go`

### Technical details
- Current published payloads: `PipelineRunStarted`, `PipelineRunFinished`, `PipelinePhaseStarted`, `PipelinePhaseFinished`, `PipelineBuildResult`, `PipelinePrepareResult`, `PipelineValidateResult`, `PipelineLaunchPlan`.

## Step 3: Inventory Engine and Supervisor Data That Could Be Surfaced Per Phase

This step identifies the “ground truth” data sources for each phase: engine pipeline outputs for plugin phases and supervisor/state outputs for internal phases. The goal is to ensure the analysis doc doesn’t make vague UX claims; it should point to concrete structs and functions that already contain (or can cheaply produce) the needed inspection fields.

### What I did
- Read engine pipeline and types:
  - `devctl/pkg/engine/pipeline.go`
  - `devctl/pkg/engine/types.go`
- Read patch application logic:
  - `devctl/pkg/patch/patch.go`
- Read supervisor and state types:
  - `devctl/pkg/supervise/supervisor.go`
  - `devctl/pkg/state/state.go`
- Checked fixture plugins for what outputs exist today (steps/artifacts/services):
  - `devctl/testdata/plugins/e2e/plugin.py`

### Why
- The UI can only be as good as the data we can produce. If a field doesn’t exist in any output today, the ticket must clearly state that we need instrumentation or protocol changes.

### What worked
- The engine and supervisor already have most of the “raw facts” we’d want to show:
  - build/prepare step name/ok/duration/artifacts
  - launch plan service specs (name/cwd/command/env/health)
  - supervisor produces per-service PID/log paths/health config in `state.ServiceRecord`

### What didn't work
- Provenance and merge decisions are lost in the engine merge:
  - build/prepare merges steps and artifacts but does not retain which plugin produced them
  - launch plan merge does not retain which plugin provided each service (or which overrides happened)
  - mutate_config returns only the final config and discards per-plugin patches

### What I learned
- Meaningful “phase inspection” requires a trace layer that lives alongside the existing engine merge logic; otherwise we can’t explain collisions/overrides in non-strict mode.

### What was tricky to build
- `patch.Apply` performs silent overwrites; to explain config changes we must track write history outside the patch package (or extend it).

### What warrants a second pair of eyes
- Whether to implement trace capture in:
  - `engine.Pipeline.*` methods (centralized, reusable), or
  - the action runner (closest to UI, but more duplicated).

### What should be done in the future
- N/A (this step is pure research).

### Code review instructions
- Focus on merge points:
  - `engine.Pipeline.MutateConfig` / `Build` / `Prepare` / `Validate` / `LaunchPlan` in `devctl/pkg/engine/pipeline.go`
  - `Supervisor.Start` in `devctl/pkg/supervise/supervisor.go`

### Technical details
- Per-plugin call order is determined by `clientsInOrder` in `devctl/pkg/engine/pipeline.go` (priority then ID).

## Related

- Analysis doc for this ticket:
  - `devctl/ttmp/2026/01/08/MO-014-IMPROVE-PIPELINE-TUI--improve-pipeline-tui-phase-inspection/analysis/01-pipeline-tui-phase-inspection-data-model-and-sources.md`
