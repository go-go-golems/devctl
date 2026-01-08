---
Title: Pipeline View Stuck State Analysis
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
      Note: Main pipeline view model with phase state management
    - Path: devctl/pkg/tui/action_runner.go
      Note: Publishes pipeline phase events during up/down/restart
    - Path: devctl/pkg/tui/pipeline_events.go
      Note: Pipeline event type definitions
    - Path: devctl/pkg/tui/transform.go
      Note: Transforms domain events to UI events
    - Path: devctl/pkg/tui/forward.go
      Note: Forwards UI events to bubbletea Program
    - Path: devctl/pkg/tui/models/root_model.go
      Note: Routes pipeline messages to child models
ExternalSources: []
Summary: Analysis of pipeline view issues where phases show "running..." even after completion
LastUpdated: 2026-01-08T15:30:00-05:00
WhatFor: Document the bugs, architecture, and proposed fixes for pipeline view state issues
WhenToUse: Reference when debugging or fixing pipeline view state management
---

# Pipeline View Stuck State Analysis

## Executive Summary

The pipeline view in the devctl TUI has a bug where certain phases (notably `build`) show "running..." even after the pipeline has completed successfully. This analysis documents:

1. The observed symptoms
2. The pipeline view architecture
3. Root cause hypotheses
4. Reproduction steps
5. Recommended fixes

## Observed Symptoms

### Bug 1: Build Phase Stuck at "running..."

When running `devctl tui` and starting the system with `[u]`, the pipeline view shows:

```
╭─────────────────────────────────────────────────────────────────────╮
│Pipeline Progress                                                    │
│✓ Pipeline: up  OK                                                   │  ← Pipeline finished OK
│Run ID: 8c32aeb4-5ce9-4412-a734-5944107ae845                         │
│Started: 2026-01-08 15:21:08                                         │
╰─────────────────────────────────────────────────────────────────────╯
╭─────────────────────────────────────────────────────────────────────╮
│Phases                                                               │
│✓ mutate_config     ok                                               │
│▶ build             running...        ← BUG: Still shows running     │
│✓ prepare           ok                                               │
│✓ validate          ok                                               │
│✓ launch_plan       ok                                               │
│✓ supervise         ok (356ms)                                       │
│✓ state_save        ok                                               │
╰─────────────────────────────────────────────────────────────────────╯
```

**Key observations:**
- The pipeline header shows "OK" with a checkmark
- Total duration is shown (499ms)
- Phases AFTER build (prepare, validate, etc.) all show "ok"
- Only `build` is stuck at "running..."

### Bug 2: State Persists After System Down (Reported)

User reports that after stopping the system, the pipeline view still shows the last phase as "running". This needs further testing but is likely related to the same state management issue.

## Architecture Overview

### Event Flow

```
┌─────────────────┐     ┌──────────────────┐     ┌─────────────────┐
│ action_runner   │────▶│ TopicDevctlEvents│────▶│ transform.go    │
│ (runs pipeline) │     │ (Watermill bus)  │     │ (domain→UI)     │
└─────────────────┘     └──────────────────┘     └─────────────────┘
                                                          │
                                                          ▼
                        ┌──────────────────┐     ┌─────────────────┐
                        │ TopicUIMessages  │◀────│ publishUI()     │
                        │ (Watermill bus)  │     └─────────────────┘
                        └──────────────────┘
                                  │
                                  ▼
                        ┌─────────────────┐     ┌─────────────────┐
                        │ forward.go      │────▶│ tea.Program     │
                        │ (UI forwarder)  │     │ (bubbletea)     │
                        └─────────────────┘     └─────────────────┘
                                                          │
                                                          ▼
                        ┌─────────────────┐     ┌─────────────────┐
                        │ root_model.go   │────▶│ pipeline_model  │
                        │ (routes msgs)   │     │ (renders view)  │
                        └─────────────────┘     └─────────────────┘
```

### Key Files

| File | Purpose |
|------|---------|
| `action_runner.go` | Runs pipeline phases, publishes events |
| `pipeline_events.go` | Event type definitions |
| `transform.go` | Transforms domain events to UI events |
| `forward.go` | Forwards UI events to bubbletea |
| `root_model.go` | Routes messages to child models |
| `pipeline_model.go` | Manages pipeline view state |

### Pipeline Phase Events

For each phase, two events are published:

1. `PipelinePhaseStarted` - When phase begins
2. `PipelinePhaseFinished` - When phase ends (with ok/error status)

### Pipeline Model State

```go
type PipelineModel struct {
    phases     map[tui.PipelinePhase]*pipelinePhaseState
    phaseOrder []tui.PipelinePhase
    // ...
}

type pipelinePhaseState struct {
    startedAt  time.Time
    finishedAt time.Time
    ok         *bool      // nil = running, true = ok, false = failed
    durationMs int64
    errText    string
}
```

The view logic in `phaseIconAndStyle()`:

```go
func (m PipelineModel) phaseIconAndStyle(st *pipelinePhaseState, theme styles.Theme) (string, lipgloss.Style) {
    if st == nil {
        return styles.IconPending, theme.StatusPending  // pending
    }
    if st.ok == nil && !st.startedAt.IsZero() {
        return styles.IconRunning, theme.StatusRunning  // running...
    }
    if st.ok == nil {
        return styles.IconPending, theme.StatusPending  // pending
    }
    if *st.ok {
        return styles.IconSuccess, theme.StatusRunning  // ok
    }
    return styles.IconError, theme.StatusDead           // failed
}
```

## Root Cause Hypotheses

### Hypothesis 1: Message Ordering / Race Condition (Most Likely)

The Watermill message bus uses goroutines for message handling. Events may arrive out of order or some may be processed before others complete:

```go
// action_runner.go - Events published in sequence
_ = publishPipelineBuildResult(pub, PipelineBuildResult{...})
_ = publishPipelinePhaseFinished(pub, PipelinePhaseFinished{
    Phase: PipelinePhaseBuild,
    Ok:    true,
    ...
})
```

If `PipelinePhaseFinishedMsg` for build is not delivered or processed, the state remains `ok = nil` (running).

**Evidence for this hypothesis:**
- Phases AFTER build show "ok", so their `PhaseFinished` events were received
- The pipeline `RunFinished` event was received (shows "OK" in header)
- Only build is stuck, suggesting its specific `PhaseFinished` was lost

### Hypothesis 2: Empty Build Phase Not Publishing Correctly

When no plugins support `build.run`:
1. `p.Build()` returns empty `BuildResult{Steps: nil, Artifacts: {}}`
2. `publishPipelineBuildResult()` publishes with empty steps
3. `publishPipelinePhaseFinished()` should still publish

The demo plugin doesn't support `build.run`, so build result is empty. This shouldn't affect `PhaseFinished` but may be a factor.

### Hypothesis 3: Value Receiver Map Mutation Issue

The `phase()` helper uses a value receiver:

```go
func (m PipelineModel) phase(p tui.PipelinePhase) *pipelinePhaseState {
    if m.phases == nil {
        m.phases = map[tui.PipelinePhase]*pipelinePhaseState{}  // Lost if m.phases was nil!
    }
    st := m.phases[p]
    if st == nil {
        st = &pipelinePhaseState{}
        m.phases[p] = st
    }
    return st
}
```

If `m.phases` is nil when entering this function, the new map assignment is made to a copy and lost. However, `m.phases` is initialized in `PipelineRunStartedMsg`, so this may not be the issue unless timing is involved.

### Hypothesis 4: Goroutine/Threading Issue in Bubbletea

The `tea.Program.Send()` method is goroutine-safe, but rapid message sending might cause issues:

```go
// forward.go - Multiple p.Send() calls in quick succession
p.Send(PipelinePhaseFinishedMsg{Event: ev})
```

If messages are sent faster than bubbletea processes them, some might be dropped or processed out of order.

## Reproduction Steps

```bash
# 1. Navigate to demo repo
cd /tmp/devctl-demo-repo

# 2. Ensure clean state
rm -rf .devctl

# 3. Start TUI
/path/to/devctl tui --alt-screen=false

# 4. Press [u] to start system

# 5. Press [Tab] twice to go to Pipeline view

# 6. Observe: build phase shows "running..." even though pipeline completed
```

## Recommended Investigation Steps

### Step 1: Add Debug Logging

Add logging to trace message flow:

```go
// In forward.go
case UITypePipelinePhaseFinished:
    log.Debug().Str("phase", string(ev.Phase)).Bool("ok", ev.Ok).Msg("forwarding phase finished")
    p.Send(PipelinePhaseFinishedMsg{Event: ev})
```

### Step 2: Verify Event Publishing

Add logging to `action_runner.go`:

```go
err := publishPipelinePhaseFinished(pub, PipelinePhaseFinished{...})
if err != nil {
    log.Error().Err(err).Str("phase", "build").Msg("failed to publish phase finished")
}
```

### Step 3: Check Message Handler Errors

Watermill handlers swallow errors. Add explicit error logging:

```go
// In transform.go
if err := publishUI(UITypePipelinePhaseFinished, ev); err != nil {
    log.Error().Err(err).Msg("failed to publish UI phase finished")
    return err  // Don't swallow!
}
```

### Step 4: Test with Forced Delays

Add artificial delays to rule out race conditions:

```go
// In action_runner.go after publishPipelinePhaseFinished for build
time.Sleep(100 * time.Millisecond)
```

## Proposed Fixes

### Fix 1: Add Message Acknowledgment Verification

Ensure all pipeline events are acknowledged before proceeding:

```go
// Use a channel to confirm message was processed
done := make(chan struct{})
p.Send(PipelinePhaseFinishedMsgWithAck{Event: ev, Done: done})
<-done
```

### Fix 2: Change `phase()` to Pointer Receiver

```go
func (m *PipelineModel) phase(p tui.PipelinePhase) *pipelinePhaseState {
    // Now modifications to m.phases persist
}
```

**Note:** This requires changing the Update method signature which may have broader implications.

### Fix 3: Add Retry/Resync Mechanism

Periodically verify phase states match expected final state:

```go
// After PipelineRunFinishedMsg, verify all phases are marked complete
for _, phase := range m.phaseOrder {
    if st := m.phases[phase]; st != nil && st.ok == nil {
        // Phase still showing running but pipeline finished - fix it
        st.ok = &m.runFinished.Ok
    }
}
```

### Fix 4: Batch Phase Updates

Instead of individual messages per phase, send a batch update:

```go
type PipelinePhasesUpdateMsg struct {
    Phases map[PipelinePhase]PipelinePhaseFinished
}
```

## Information We Can Show in Pipeline View

The pipeline view already supports rich information:

| Data | Source | Status |
|------|--------|--------|
| Phase status (pending/running/ok/failed) | PhaseStarted/Finished | ✅ Implemented |
| Phase duration | PhaseFinished.DurationMs | ✅ Implemented |
| Build steps | PipelineBuildResult.Steps | ✅ Implemented |
| Prepare steps | PipelinePrepareResult.Steps | ✅ Implemented |
| Validation errors/warnings | PipelineValidateResult | ✅ Implemented |
| Config patches | PipelineConfigPatches | ✅ Implemented |
| Live output | PipelineLiveOutput | ✅ Implemented |
| Launch plan (services) | PipelineLaunchPlan | ✅ Implemented |
| Step progress (percent) | PipelineStepProgressMsg | ✅ Implemented |

### Missing/Underutilized Features

1. **No build steps displayed** - Because demo plugin doesn't support `build.run`, the "Build Steps" section never appears
2. **No prepare steps displayed** - Same reason
3. **Config patches not shown** - No plugins emit config patches in the demo

## Navigation Issues

The user mentioned navigation issues. Current keybindings:

| Key | Action |
|-----|--------|
| `b` | Focus build steps |
| `p` | Focus prepare steps |
| `v` | Focus validation |
| `o` | Toggle live output |
| `↑/↓` or `j/k` | Move cursor |
| `Enter` | Toggle details |

**Potential issues:**
1. If no build/prepare steps exist, focusing on them shows nothing
2. Navigation cursor state isn't clearly visible
3. No indication of what's focusable vs empty

## Conclusion

The primary bug is likely a **message delivery/ordering issue** where `PipelinePhaseFinishedMsg` for the build phase is not being received or processed by the pipeline model. 

**Recommended first step:** Add debug logging to trace the message flow and identify exactly where the message is lost.

**Quick workaround:** In `PipelineRunFinishedMsg` handler, force-complete any phases still showing "running":

```go
case tui.PipelineRunFinishedMsg:
    // ... existing code ...
    // Force-complete any stuck phases
    for _, phase := range m.phaseOrder {
        if st := m.phases[phase]; st != nil && st.ok == nil {
            ok := run.Ok
            st.ok = &ok
            st.finishedAt = run.At
        }
    }
    return m, nil
```

