---
Title: Diary
Ticket: MO-018-PIPELINE-VIEW-STUCK-STATE
Status: active
Topics:
    - devctl
    - tui
    - pipeline
DocType: reference
Intent: long-term
Owners: []
RelatedFiles:
    - Path: devctl/pkg/tui/models/pipeline_model.go
      Note: Main pipeline view model with phase state management
    - Path: devctl/pkg/tui/action_runner.go
      Note: Publishes pipeline phase events during up/down/restart
    - Path: devctl/pkg/tui/pipeline_events.go
      Note: Pipeline event type definitions
    - Path: devctl/pkg/tui/transform.go
      Note: Transforms domain events to UI events
    - Path: devctl/pkg/tui/forward.go
      Note: Forwards UI events to bubbletea Program
ExternalSources: []
Summary: Step-by-step investigation of pipeline view stuck state issues.
LastUpdated: 2026-01-08T15:45:00-05:00
WhatFor: Track investigation and fix progress for pipeline view bugs
WhenToUse: Reference when debugging or fixing pipeline view state management
---

# Diary

## Goal

Investigate and fix the pipeline view bugs where phases show "running..." even after completion, and navigation/state issues.

## Step 1: Initial Investigation and Reproduction

Analyzed the pipeline view architecture and successfully reproduced the "stuck running" bug. The build phase shows "running..." even after the entire pipeline completes successfully.

### What I did
- Read all pipeline-related source files:
  - `pipeline_model.go` (view logic)
  - `action_runner.go` (event publishing)
  - `pipeline_events.go` (event types)
  - `transform.go` (domain→UI transform)
  - `forward.go` (UI→bubbletea forwarding)
  - `root_model.go` (message routing)
- Set up test environment at `/tmp/devctl-demo-repo`
- Ran TUI in tmux session for automated testing
- Started system with `[u]` key
- Navigated to Pipeline view

### Why
- User reported phases getting stuck and navigation issues
- Need to understand the full event flow before fixing

### What worked
- Successfully reproduced the bug:
  - Pipeline header shows "OK"
  - Total duration shows 499ms
  - Phases AFTER build (prepare, validate, etc.) all show "ok"
  - Only `build` phase is stuck at "running..."

### What didn't work
- N/A (investigation phase)

### What I learned
1. **Event flow is complex**: Events go through 4 hops (action_runner → bus → transform → forward → bubbletea → root_model → pipeline_model)
2. **The bug is phase-specific**: Only build is stuck, not all phases
3. **Order matters**: Phases after build show "ok", so events are generally flowing

### What was tricky to build
- Setting up automated TUI testing with tmux

### What warrants a second pair of eyes
- The Watermill message bus uses goroutines; message ordering may not be guaranteed

### What should be done in the future
- Add debug logging to trace message flow
- Consider end-to-end integration tests for pipeline view

### Code review instructions
- Read `analysis/01-pipeline-view-stuck-state-analysis.md` for full architecture documentation
- Start with `action_runner.go` runUp() function to see event publishing

### Technical details

**Observed output:**
```
╭─────────────────────────────────────────────────────────────────────╮
│Phases                                                               │
│✓ mutate_config     ok                                               │
│▶ build             running...        ← BUG                          │
│✓ prepare           ok                                               │
│✓ validate          ok                                               │
│✓ launch_plan       ok                                               │
│✓ supervise         ok (356ms)                                       │
│✓ state_save        ok                                               │
╰─────────────────────────────────────────────────────────────────────╯
```

**Event publishing order in action_runner.go:**
```go
// Line 441-453
_ = publishPipelineBuildResult(pub, PipelineBuildResult{...})
_ = publishPipelinePhaseFinished(pub, PipelinePhaseFinished{
    RunID:      runID,
    Phase:      PipelinePhaseBuild,
    At:         time.Now(),
    Ok:         true,
    DurationMs: time.Since(buildStart).Milliseconds(),
})
```

Events are published in sequence, but the PhaseFinished for build is not being processed by the UI.

---

## Step 2: Root Cause Hypotheses

Documented multiple hypotheses for the bug and created a detailed analysis document.

### What I did
- Created comprehensive analysis document at `analysis/01-pipeline-view-stuck-state-analysis.md`
- Documented 4 hypotheses with varying likelihood
- Created task list for investigation steps
- Proposed both short-term workaround and long-term fixes

### Root Cause Hypotheses

**Hypothesis 1: Message Ordering / Race Condition (Most Likely)**
- Watermill uses goroutines for message handling
- Events may arrive out of order or be dropped
- Evidence: Phases after build show "ok", but build doesn't

**Hypothesis 2: Empty Build Phase**
- Demo plugin doesn't support `build.run`
- Build result has empty Steps
- May affect event processing somehow

**Hypothesis 3: Value Receiver Map Mutation**
- `phase()` helper uses value receiver
- If `m.phases` is nil, new map assignment is lost
- Unlikely since map is initialized on RunStarted

**Hypothesis 4: Bubbletea Message Queue**
- Rapid p.Send() calls might cause issues
- Some messages might be dropped

### Quick Workaround Proposed
```go
// In PipelineRunFinishedMsg handler
for _, phase := range m.phaseOrder {
    if st := m.phases[phase]; st != nil && st.ok == nil {
        ok := run.Ok
        st.ok = &ok
        st.finishedAt = run.At
    }
}
```

### What warrants a second pair of eyes
- The Watermill message ordering guarantees (or lack thereof)
- The bubbletea concurrent Send() behavior

### What should be done in the future
- Add debug logging as first step
- Consider message batching for phase updates

---

## Next Steps

1. [ ] Add debug logging to trace pipeline message flow
2. [ ] Identify exactly where PipelinePhaseFinished for build is lost
3. [ ] Implement fix (either message-level or workaround)
4. [ ] Test fix with demo repo
5. [ ] Verify down action clears pipeline state correctly
