---
Title: Diary
Ticket: RUNTIME-PLUGIN-INTROSPECTION
Status: active
Topics:
    - devctl
    - plugins
    - introspection
DocType: reference
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: ""
LastUpdated: 2026-01-08T13:35:00.000000000-05:00
WhatFor: ""
WhenToUse: ""
---

# Diary

## Goal

Capture the research work for runtime plugin introspection, including the code paths reviewed, constraints discovered, and the research outline for next steps.

## Step 1: Establish ticket docs and research targets

I created a dedicated research notes document and a new diary for the RUNTIME-PLUGIN-INTROSPECTION ticket. This set up a clear place to capture findings and align them with the existing research plan.

I also scanned the ticket structure and existing research plan to make sure the new notes build on it rather than duplicating or diverging from the intended outline.

### What I did
- Created the analysis doc for research notes and the ticket diary.
- Reviewed the existing research plan to align scope and terminology.

### Why
- Keep research artifacts and decision trail organized for this ticket.

### What worked
- Ticket workspace already included a research plan with clear avenues to investigate.

### What didn't work
- N/A

### What I learned
- The ticket already identifies a concrete path (StateWatcher + background introspection) for a low-risk MVP.

### What was tricky to build
- N/A

### What warrants a second pair of eyes
- N/A

### What should be done in the future
- N/A

### Code review instructions
- Where to start: `devctl/ttmp/2026/01/08/RUNTIME-PLUGIN-INTROSPECTION--runtime-plugin-introspection-for-capability-discovery/design/01-research-plan-runtime-plugin-introspection.md`.
- How to validate: N/A

### Technical details
- Target files noted in the research plan include `devctl/pkg/tui/state_watcher.go` and `devctl/pkg/runtime/factory.go`.

## Step 2: Inspect runtime handshake and TUI integration paths

I reviewed the runtime factory, client, and tests to understand how handshakes are read, what the timeout behavior is, and how stdout noise affects startup. I also examined TUI models to confirm that capability data is already rendered if present.

This clarifies that capability introspection is a data-wiring problem more than a UI design problem, and that the safest approach should respect handshake strictness and avoid stdout contamination.

### What I did
- Read `devctl/pkg/runtime/factory.go` and `devctl/pkg/runtime/runtime_test.go` for handshake behavior.
- Read `devctl/pkg/tui/state_watcher.go`, `devctl/pkg/tui/state_events.go`, and `devctl/pkg/tui/models/plugin_model.go` for data flow to the UI.
- Reviewed plugin authoring and user guide docs for handshake contract examples.

### Why
- Capture the actual runtime constraints before proposing an introspection mechanism.

### What worked
- Tests explicitly cover handshake noise and show the expected failure modes.

### What didn't work
- N/A

### What I learned
- Handshake must be the first stdout frame and is enforced by `readHandshake`, which implies introspection must be careful about stdout noise and timeouts.

### What was tricky to build
- N/A

### What warrants a second pair of eyes
- Confirm that an immediate Start+Close introspection does not introduce side effects for current plugins.

### What should be done in the future
- N/A

### Code review instructions
- Where to start: `devctl/pkg/runtime/factory.go`, `devctl/pkg/runtime/runtime_test.go`.
- How to validate: N/A

### Technical details
- Default handshake timeout is 2s; stdout contamination after handshake breaks calls.

## Step 3: Draft research notes and concrete outline

I wrote the research notes document summarizing the observed architecture, constraints, and a concrete research outline with recommended experiments. The outline includes a focus on validating Start+Close introspection, measuring handshake latencies, and prototyping a StateWatcher-based approach.

This consolidates the research plan into a set of concrete, testable next steps that can be used to guide implementation.

### What I did
- Authored research notes with constraints, approach comparisons, and suggested experiments.

### Why
- Provide an actionable research outline grounded in the current codebase.

### What worked
- The existing plugin model already renders capabilities, so the primary gap is data collection.

### What didn't work
- N/A

### What I learned
- Background introspection is the least invasive starting point, but it requires careful timeout and side-effect handling.

### What was tricky to build
- N/A

### What warrants a second pair of eyes
- Review the proposed risk mitigations (side effects, stdout noise) for completeness.

### What should be done in the future
- N/A

### Code review instructions
- Where to start: `devctl/ttmp/2026/01/08/RUNTIME-PLUGIN-INTROSPECTION--runtime-plugin-introspection-for-capability-discovery/analysis/01-research-notes-runtime-plugin-introspection.md`.
- How to validate: N/A

### Technical details
- Research outline includes handshake timing instrumentation and StateWatcher background introspection.

## Step 4: Add UX addendum for introspection behavior

I expanded the research notes with a detailed UX addendum that makes introspection visible and controllable in the TUI. The addendum specifies status states, refresh flows, error details, and how capabilities should be presented when data is missing or stale.

This keeps the design aligned with the updated constraints while giving implementers a concrete UX contract that is feature-level and behavior-focused.

### What I did
- Added the introspection UX addendum to the research notes document.
- Cleaned up formatting artifacts so the design plan reads cleanly.

### Why
- Provide explicit, implementable UX behaviors for the introspection feature.

### What worked
- The plugin model already supports capability rendering, so the UX addendum can focus on state and controls.

### What didn't work
- N/A

### What I learned
- Clear UX status language reduces ambiguity around slow or failing plugins.

### What was tricky to build
- N/A

### What warrants a second pair of eyes
- Validate that the proposed error taxonomy aligns with actual runtime failure modes.

### What should be done in the future
- N/A

### Code review instructions
- Where to start: `devctl/ttmp/2026/01/08/RUNTIME-PLUGIN-INTROSPECTION--runtime-plugin-introspection-for-capability-discovery/analysis/01-research-notes-runtime-plugin-introspection.md`.
- How to validate: N/A

### Technical details
- UX addendum covers refresh actions, status labels, error panels, and acceptance criteria.

## Step 5: Implement startup introspection and refresh plumbing

I implemented the initial runtime introspection flow: the StateWatcher now runs a startup pass that starts each plugin, captures its handshake, and caches capabilities in memory. I also wired a global refresh action from the Plugins view so users can re-run introspection without restarting the TUI.

On the UI side, the Plugins view now shows capability status (unknown/introspecting/ok/error) and renders ops/streams/commands only when the handshake is available, keeping the experience consistent with the new UX addendum.

### What I did
- Added in-memory introspection cache and background loop to StateWatcher.
- Added a global refresh keybinding (`r`) in the Plugins view and RootModel plumbing.
- Extended PluginSummary/PluginModel to include capability status, commands, and protocol information.

### Why
- Provide visible introspection progress and explicit refresh control, per the updated constraints.

### What worked
- The existing plugin view rendering made it straightforward to surface capability status and lists.

### What didn't work
- N/A

### What I learned
- A simple channel-based refresh trigger keeps the TUI responsive while introspection runs.

### What was tricky to build
- Ensuring the UI renders “unknown” instead of “none” before handshakes are available.

### What warrants a second pair of eyes
- Review concurrency assumptions around the introspection cache and snapshot timing.

### What should be done in the future
- N/A

### Code review instructions
- Where to start: `devctl/pkg/tui/state_watcher.go`, `devctl/pkg/tui/models/plugin_model.go`, `devctl/pkg/tui/models/root_model.go`.
- How to validate: run the TUI and verify the Plugins view updates after startup introspection and refresh.

### Technical details
- Introspection uses `Factory.Start()` + `Handshake()` + immediate `Close()` per plugin.
- Refresh is a global action (`r`) with no per-plugin refresh.
