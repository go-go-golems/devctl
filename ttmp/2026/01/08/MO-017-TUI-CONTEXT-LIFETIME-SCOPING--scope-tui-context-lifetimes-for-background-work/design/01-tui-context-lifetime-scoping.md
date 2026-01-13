---
Title: TUI Context Lifetime Scoping
Ticket: MO-017-TUI-CONTEXT-LIFETIME-SCOPING
Status: active
Topics:
    - devctl
    - tui
DocType: design
Intent: long-term
Owners: []
RelatedFiles:
    - Path: devctl/cmd/devctl/cmds/tui.go
      Note: TUI root context and runner registration call sites.
    - Path: devctl/pkg/runtime/factory.go
      Note: exec.CommandContext defines plugin process lifetime.
    - Path: devctl/pkg/tui/action_runner.go
      Note: Action runner context base and timeouts.
    - Path: devctl/pkg/tui/stream_actions.go
      Note: Publish API for stream start/stop messages.
    - Path: devctl/pkg/tui/stream_runner.go
      Note: Stream and plugin lifecycle context usage.
ExternalSources: []
Summary: Define and apply a TUI-scoped context pattern for long-lived background work so UI exit cancels streams, actions, and future runners cleanly.
LastUpdated: 2026-01-08T14:48:22.151203936-05:00
WhatFor: Guide refactor of TUI context wiring across runners, stream actions, and Bubbletea so background work shuts down with the UI.
WhenToUse: Use when implementing the MO-017 fix or adding new TUI background workflows.
---



# TUI Context Lifetime Scoping

## Goal

Make all TUI-initiated long-lived work (streams, actions, watchers, and future background loops) derive from the TUI lifecycle context so the UI can exit cleanly without leaks or orphaned processes.

## Problem Statement

Some TUI components currently run with `context.Background()` or a message context that is effectively background. This lets plugin processes and goroutines continue after the UI exits, which can leak resources, block publishers, and leave subprocesses running.

## Design Principles

1) **Work lifetime matches TUI lifetime.** If the work was initiated by the UI and should end with the UI, its context should be derived from the TUI context.
2) **Cleanup is bounded and independent.** Shutdown calls should use a fresh timeout context so cleanup can occur even after the TUI context is canceled.
3) **Apply the pattern uniformly.** The same approach must cover every place we start long-lived work, not just the stream runner.

## Current Behavior (Summary)

- Stream runner uses `context.Background()` for plugin start and stream forwarding.
- Action runner uses `msg.Context()`, which is background by default.
- Bubbletea program is not explicitly tied to the TUI context.
- State watcher is already correctly bound to the TUI context.

## Proposed Design

### 1) Introduce a TUI-Scoped Context and Pass It to Runners

Update registration signatures to accept an explicit TUI context:

- `RegisterUIActionRunner(ctx context.Context, bus *Bus, opts RootOptions)`
- `RegisterUIStreamRunner(ctx context.Context, bus *Bus, opts RootOptions)`

Store the context on the manager/runner struct (e.g., `tuiCtx`) and use it to derive all long-lived work.

### 2) Derive Work Contexts from the TUI Context

For long-running work:

- Use `context.WithCancel(tuiCtx)` for stream lifetimes.
- Use `context.WithTimeout(tuiCtx, ...)` for action steps.
- Use `tuiCtx` (or a child) for `runtime.Factory.Start(...)` so plugin processes are terminated when the UI exits.

### 3) Use Fresh Cleanup Contexts

For cleanup calls (e.g., `client.Close`, `supervisor.Stop`, etc):

- Use `context.WithTimeout(context.Background(), ...)` so cleanup proceeds even if the TUI context has already been canceled.

### 4) Optional: Propagate Context Through Message Publishing

Optionally publish UI requests with `message.NewMessageWithContext(tuiCtx, ...)` so any handler that still relies on `msg.Context()` gets the correct lifetime. This is additive, not a substitute for explicit TUI context injection.

### 5) Bubbletea Integration

Add `tea.WithContext(tuiCtx)` when constructing the program so canceling the TUI context can terminate the UI loop if needed. This makes the lifecycle model consistent across UI and background tasks.

## Scope (Apply Pattern Everywhere)

The fix must apply to all long-lived operations triggered by the TUI, not just streams:

- Stream runner (start, event forwarders, plugin processes)
- Action runner (runUp/runDown and any action-phase work)
- Any future long-running UI commands (pipelines, log tailing, etc.)

## API and Call Site Changes

- `devctl/cmd/devctl/cmds/tui.go` passes `ctx` into `RegisterUIActionRunner` and `RegisterUIStreamRunner`.
- `devctl/pkg/tui/stream_runner.go` stores `tuiCtx` and derives stream contexts from it.
- `devctl/pkg/tui/action_runner.go` uses `tuiCtx` as the base context for actions (instead of `msg.Context()`).
- Optional: `PublishAction`, `PublishStreamStart`, `PublishStreamStop` can use `message.NewMessageWithContext`.

## Risks

- If cleanup uses the TUI context, shutdown can silently fail after cancellation. This is why cleanup must use a fresh timeout context.
- Some actions may be expected to finish even after UI exit; confirm desired semantics before enforcing cancellation.
- If the TUI context is canceled too aggressively, it may interrupt in-flight user workflows.

## Testing and Validation

- Start a stream, quit the TUI, confirm plugin process terminates.
- Start a long-running action, quit the TUI, confirm action cancels and no goroutines block on publish.
- Confirm stream stop requests still emit `StreamEnded` with appropriate status.
- Confirm bus and watcher still stop on TUI exit.

## Open Questions

- Should action runner be hard-canceled on UI exit, or should it optionally continue until completion?
- Should stream shutdown on UI exit emit a final `StreamEnded` event or exit silently?
- Do we want to standardize a helper (e.g., `tui.WithShutdownTimeout`) for cleanup contexts?

## Tasks (Tracking)

- Audit all TUI code paths for long-lived work and background contexts.
- Refactor action/stream runners to accept and use a TUI-scoped context.
- Add cleanup timeout contexts where needed.
- Optionally publish UI messages with the TUI context.
- Validate behavior with manual TUI flows.
