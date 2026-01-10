---
Title: Diary
Ticket: MO-017-TUI-CONTEXT-LIFETIME-SCOPING
Status: active
Topics:
    - devctl
    - tui
DocType: reference
Intent: long-term
Owners: []
RelatedFiles:
    - Path: devctl/cmd/devctl/cmds/tui.go
      Note: |-
        Passes TUI context into runner registrations.
        Adds tea.WithContext to bind Bubbletea lifetime to TUI context.
    - Path: devctl/pkg/tui/action_runner.go
      Note: Action execution now derives from TUI context instead of message context.
    - Path: devctl/pkg/tui/stream_runner.go
      Note: Stream lifetime now derives from TUI context; cleanup uses bounded close helper.
ExternalSources: []
Summary: Implementation diary for scoping TUI context lifetimes across background work.
LastUpdated: 2026-01-08T15:08:13-05:00
WhatFor: Track context-lifetime refactors and validation steps for MO-017.
WhenToUse: Reference when reviewing or extending TUI context wiring changes.
---



# Diary

## Goal

Ensure all TUI-initiated long-lived work derives from the TUI context while cleanup uses bounded, fresh contexts.

## Step 1: Scope TUI Context in Stream and Action Runners

Refactored the stream and action runners to take a TUI-scoped context at registration, then derived all long-lived operations from that context. This aligns stream lifetimes and action phases with the UI lifecycle instead of message or background contexts.

Added a bounded cleanup helper for stream client shutdown so cancellation of the TUI context does not prevent cleanup. This sets the baseline behavior for the MO-017 fix before deciding on Bubbletea context propagation or message context wiring.

**Commit (code):** 1cfee17 — "Fix: scope TUI background work to UI context"

### What I did
- Updated `RegisterUIActionRunner` to accept a TUI context and use it for action execution
- Updated `RegisterUIStreamRunner` to accept a TUI context and use it for plugin start and stream lifetime
- Added `closeClient` helper with a fresh timeout context in `stream_runner.go`
- Updated `tui.go` to pass the TUI context into the runner registrations
- Checked tasks 2–4 in MO-017

### Why
- Stream and action work must be canceled when the UI exits to avoid leaks and orphaned plugin processes
- Cleanup should still run after TUI cancellation, so it needs its own bounded context

### What worked
- Stream/action lifetimes are now tied to the TUI context rather than message or background contexts
- Cleanup paths now use a timeout context, avoiding early exits on canceled parent contexts

### What didn't work
- N/A

### What I learned
- Keeping a separate cleanup context is essential when the parent context is expected to be canceled

### What was tricky to build
- Balancing stream lifetime (tied to TUI) with cleanup lifetime (fresh timeout context)

### What warrants a second pair of eyes
- Confirm stream cancellation semantics still publish correct `StreamEnded` events
- Verify no unintended changes in plugin shutdown ordering when using the new context wiring

### What should be done in the future
- Decide on Bubbletea `tea.WithContext` and UI message context propagation
- Validate the new behavior with manual TUI runs

### Code review instructions
- Start in `devctl/pkg/tui/stream_runner.go` and `devctl/pkg/tui/action_runner.go`
- Verify the call sites in `devctl/cmd/devctl/cmds/tui.go`

### Technical details
- Stream runner now uses a TUI-scoped context for `Factory.Start` and stream lifetimes
- Cleanup uses `context.WithTimeout(context.Background(), 2*time.Second)` in `closeClient`

---

## Step 2: Tie Bubbletea Program to TUI Context

Enabled `tea.WithContext` on the TUI program so canceling the TUI context cleanly stops the UI loop. This implements the agreed decision to bind the Bubbletea lifecycle to the same context already used by the bus and background runners.

Skipped message context propagation as requested, since explicit runner context injection already establishes the desired lifetimes.

**Commit (code):** 1cfee17 — "Fix: scope TUI background work to UI context"

### What I did
- Added `tea.WithContext(ctx)` to the Bubbletea program options in `tui.go`
- Marked task 5 as complete based on the decision (WithContext yes, message context no)

### Why
- TUI cancellation should end the UI loop directly, not just the bus/watchers
- Message context propagation is unnecessary when runner lifetimes are already explicit

### What worked
- Program context is now tied to the TUI lifecycle context

### What didn't work
- N/A

### What I learned
- Bubbletea supports a straightforward context hook that matches the lifecycle invariant

### What was tricky to build
- N/A

### What warrants a second pair of eyes
- Confirm that canceling the parent command context shuts down the UI as expected

### What should be done in the future
- Validate behavior in a manual TUI session (task 6)

### Code review instructions
- Review `devctl/cmd/devctl/cmds/tui.go` for `tea.WithContext(ctx)`

### Technical details
- `tea.WithContext(ctx)` is added alongside input/output options to tie program lifetime to the TUI context

### Validation steps (next)
- Run `devctl tui`, start a stream, then exit the TUI; confirm the plugin process terminates
- Start an action (e.g., `ActionUp`), exit the TUI mid-run; confirm action cancels and no goroutines block on publish
- Confirm `StreamEnded` shows a clear cancellation/stopped status when exiting during a stream
