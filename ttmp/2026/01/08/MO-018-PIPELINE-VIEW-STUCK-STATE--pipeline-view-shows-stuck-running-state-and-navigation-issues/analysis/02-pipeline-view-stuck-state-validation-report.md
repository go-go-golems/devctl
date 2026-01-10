---
Title: Pipeline View Stuck State Validation Report
Ticket: MO-018-PIPELINE-VIEW-STUCK-STATE
Status: active
Topics:
    - devctl
    - tui
    - pipeline
DocType: analysis
Intent: long-term
Owners: []
RelatedFiles:
    - Path: devctl/pkg/tui/models/pipeline_model.go
      Note: Phase state transitions and reset behavior
    - Path: devctl/pkg/tui/action_runner.go
      Note: Publishing order for pipeline events
    - Path: devctl/pkg/tui/transform.go
      Note: Domain to UI translation and ack behavior
    - Path: devctl/pkg/tui/forward.go
      Note: UI forwarder concurrency with Program.Send
    - Path: devctl/pkg/tui/bus.go
      Note: In-memory Watermill bus setup
    - Path: devctl/cmd/devctl/cmds/tui.go
      Note: Bus and program startup sequencing
ExternalSources:
    - Path: /home/manuel/go/pkg/mod/github.com/!three!dots!labs/watermill@v1.5.1/pubsub/gochannel/pubsub.go
      Note: GoChannel publishes concurrently; ordering not guaranteed
    - Path: /home/manuel/go/pkg/mod/github.com/!three!dots!labs/watermill@v1.5.1/message/router.go
      Note: Router handles messages in goroutines
    - Path: /home/manuel/go/pkg/mod/github.com/charmbracelet/bubbletea@v1.3.10/tea.go
      Note: Program.Send enqueues to channel; no drop but unordered across goroutines
Summary: Validation of the pipeline view stuck-state analysis with ordering/concurrency findings and risk assessment.
LastUpdated: 2026-01-08T15:33:02-05:00
WhatFor: Validate analysis hypotheses and flag corrections before implementation.
WhenToUse: Use before implementing fixes to the pipeline view stuck-running state.
---

# Pipeline View Stuck State Validation Report

## Scope and method

This report validates the existing analysis against the current source and upstream library behavior. It is a static review only; no runtime reproduction or tests were performed.

## Validation summary

- The architecture and event flow documented in the analysis are accurate.
- The highest-confidence root cause remains message ordering, but the precise mechanism is more specific than stated: `PipelinePhaseStarted` can arrive after `PipelinePhaseFinished` and overwrite the completed state, leaving a phase stuck as "running...".
- Hypotheses about empty build steps, value-receiver map mutation, or Bubbletea dropping messages are weak or unsupported by the current code paths.
- There is an additional, low-probability risk of event loss if messages are published before subscribers are running (non-persistent GoChannel).

## Hypothesis validation

### Hypothesis 1: Message ordering / race condition

**Validated, with a concrete failure mode.**

Watermill routes each message to handlers in its own goroutine, and GoChannel publishes concurrently per subscriber. There is no guarantee of per-topic ordering through the chain of `action_runner -> transform -> forward -> bubbletea`.

In `PipelineModel.Update`, the `PipelinePhaseStartedMsg` handler resets `ok` and `finishedAt`. If a `PipelinePhaseFinishedMsg` is processed first and the start arrives later, the phase regresses to "running..." indefinitely. This aligns with the observed symptom (single phase stuck, later phases OK).

**Concrete mechanism:**
- `PipelinePhaseFinishedMsg` processed first: sets `ok = true`, `finishedAt`.
- `PipelinePhaseStartedMsg` processed later: clears `ok`, `finishedAt`.
- UI shows "running..." forever unless another finish arrives.

### Hypothesis 2: Empty build phase not publishing correctly

**Not supported.**

`runUp` publishes `PipelinePhaseFinished` for the build phase unconditionally, regardless of build steps or artifacts. Empty build steps should not suppress the finish event. This hypothesis does not explain a build-only regression unless the finish message itself is dropped or reordered (which is already covered by Hypothesis 1).

### Hypothesis 3: Value receiver map mutation issue

**Unlikely / not causal.**

`PipelineModel.phase()` uses a value receiver, but `m.phases` is initialized during `PipelineRunStartedMsg` and also in `NewPipelineModel`. Additionally, `PipelinePhaseStarted/Finished` short-circuit when `runStarted` is nil, so `phase()` is not called when the map is nil. This is not a credible cause of the stuck running phase.

### Hypothesis 4: Bubbletea message queue drops

**Not supported, but ordering can still be affected.**

`Program.Send` enqueues to a channel and does not drop messages; however, calls occur from concurrent goroutines in the Watermill handlers. This means ordering is not guaranteed even though delivery is reliable. The bug is more about ordering than loss at the Bubbletea layer.

## Additional risks and clarifications

- **Out-of-order start/finish is enough to reproduce the symptom.** The pipeline model has no guard against a start event arriving after a finish event for the same phase.
- **Non-persistent bus startup window.** The GoChannel pub/sub is non-persistent; messages published before subscribers attach are discarded. This is a low-likelihood edge case but could explain sporadic missing events right after startup.
- **Run-finished does not reconcile phase state.** `PipelineRunFinishedMsg` doesn't validate or close any "running" phases, so any ordering glitch will persist in the UI.

## Suggested updates to the original analysis

- Add a concrete ordering failure mode: "PhaseFinished arrives before PhaseStarted; PhaseStarted overwrites finished state."
- Reclassify the "empty build phase" and "value receiver map mutation" hypotheses as low likelihood.
- Clarify that Bubbletea does not drop messages, but ordering is not guaranteed across goroutines.

## Recommended next investigation steps

1. Add logging that records the phase name, event type, and timestamps at each hop (domain publish, UI publish, forwarder, pipeline model update) to confirm ordering in the wild.
2. Add a defensive check in `PipelinePhaseStartedMsg` handling to avoid clearing a completed phase unless the started event is newer than the finished timestamp.
3. Consider running the pipeline model with a deterministic ordering guard for testing to see if the stuck phase disappears.

