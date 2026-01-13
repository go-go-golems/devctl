---
Title: Streams TUI Context Lifetime Analysis
Ticket: STREAMS-TUI
Status: active
Topics:
    - devctl
    - tui
    - streams
DocType: analysis
Intent: long-term
Owners: []
RelatedFiles:
    - Path: devctl/cmd/devctl/cmds/tui.go
      Note: TUI root context
    - Path: devctl/pkg/runtime/factory.go
      Note: exec.CommandContext ties plugin lifetime to context.
    - Path: devctl/pkg/tui/action_runner.go
      Note: Action runner uses message context (similar lifetime concern).
    - Path: devctl/pkg/tui/bus.go
      Note: Watermill router run context and shutdown behavior.
    - Path: devctl/pkg/tui/state_watcher.go
      Note: Background loop correctly scoped to TUI context.
    - Path: devctl/pkg/tui/stream_runner.go
      Note: Stream context creation and plugin lifecycle wiring.
ExternalSources: []
Summary: Traces context lifetimes across the TUI, Watermill bus, and stream runner; identifies scope mismatches; and evaluates fixes that tie stream work to the TUI lifecycle.
LastUpdated: 2026-01-08T14:29:36-05:00
WhatFor: Reference for correcting stream/action context usage so TUI shutdown cleanly cancels background work.
WhenToUse: Use when diagnosing stream shutdown leaks or refactoring TUI lifecycle/context wiring.
---


# Streams TUI Context Lifetime Analysis

## Goal

Understand how contexts flow through the TUI and identify where stream operations outlive the TUI. Confirm whether the current behavior matches the intended lifecycle, and outline solutions that bind stream activity to the TUI runtime.

## System Overview (Context Lifetimes)

The TUI has three primary runtime pieces:

1) Bubbletea program (UI loop)
2) Watermill in-memory bus (message routing)
3) Background services (state watcher, stream/action runners)

The main TUI command wires them together:

```
cmd.Context() -> ctx (cancelable)
  |- errgroup.WithContext(ctx) -> egCtx
     |- bus.Run(egCtx)
     |- watcher.Run(egCtx)
     |- program.Run() -> cancel() after return
```

Key observations:

- `cmd.Context()` is the logical TUI lifetime. It is canceled when the command exits.
- `program.Run()` is not currently tied to `ctx`; it returns when the user quits, and then calls `cancel()`.
- The bus and watcher are tied to `egCtx`, which is derived from `ctx`.
- Stream/action runners are registered on the bus, but they are not given the TUI context directly.

## Where Contexts Come From Today

### Bubbletea

- The TUI creates the Bubbletea program in `devctl/cmd/devctl/cmds/tui.go`.
- There is no `tea.WithContext(...)` option used, so the program's internal context is not tied to `ctx`.
- The TUI cancels `ctx` *after* `program.Run()` returns.

### Watermill Messages

- UI requests are published with `message.NewMessage(...)`, which defaults to `context.Background()`.
- Watermill enriches message contexts with handler metadata, but does not cancel them when the handler returns.
- Result: `msg.Context()` is effectively background unless a publisher explicitly sets it.

### Stream Runner

`devctl/pkg/tui/stream_runner.go` currently uses `context.Background()` for:

- `runtime.Factory.Start(...)` (spawns plugin processes via `exec.CommandContext`).
- `streamCtx := context.WithCancel(context.Background())` for forwarding stream events.

This means stream processes and forwarding goroutines are not tied to the TUI lifetime.

### Action Runner (Similar Pattern)

`devctl/pkg/tui/action_runner.go` takes `ctx := msg.Context()` and uses it to run `runUp`/`runDown` with `context.WithTimeout(...)`. Because the message context is background, actions are not canceled when the TUI exits.

## What Happens on TUI Exit Today

When the user quits the TUI:

- `program.Run()` returns, `cancel()` is called.
- The errgroup context is canceled, so the bus router and state watcher shut down.
- Stream forwarding goroutines are *not* canceled because their context is background.
- Plugin processes started for streams are *not* canceled because they were started with background context.

Potential side effects:

- Long-running stream goroutines may continue publishing to the bus after the router has stopped.
- The gochannel publisher may block if its output buffer fills (default 1024), causing goroutine leaks.
- Plugin processes can continue running even though the TUI has exited.

## Is the Assessment Correct?

Yes, with an important nuance.

- The original hypothesis ("msg.Context() is canceled when the handler returns") is **not** supported by Watermill's implementation. The default message context is background, and the router only enriches it with values.
- However, the *lifecycle mismatch is real*: using background contexts means stream work outlives the TUI, which is contrary to the goal of a clean exit.
- Therefore, the correct fix is not "background instead of message context," but "TUI-lifetime context instead of background or message context."

## Similar Occurrences of the Same Issue

From an audit of TUI context usage (`rg -n "msg.Context\(\)" devctl/pkg/tui` and `rg -n "context\.Background\(\)" devctl/pkg/tui`):

1) `devctl/pkg/tui/stream_runner.go`
   - Uses background for plugin start and stream lifetime.
   - Root cause of streams outliving the TUI.

2) `devctl/pkg/tui/action_runner.go`
   - Uses `msg.Context()` as the base for action execution.
   - Since message contexts are background, long actions continue after TUI exit.

3) `devctl/pkg/tui/bus.go` / `devctl/pkg/tui/state_watcher.go`
   - Correctly accept `ctx` and stop on TUI cancellation.
   - These are already aligned with the desired lifetime.

## Solution Space

### Option A (Recommended): Pass TUI Context Into Stream Runner

**Idea:** Register the stream runner with a TUI-scoped context, and derive stream contexts from it.

- Change registration to accept a context:
  - `RegisterUIStreamRunner(ctx, bus, opts)`
- Store `ctx` on `streamManager` (e.g., `tuiCtx`).
- Use `streamCtx, cancel := context.WithCancel(tuiCtx)` for each stream.
- Use `tuiCtx` or `streamCtx` when calling `factory.Start` so that plugin processes are killed when the TUI exits.

Notes:

- `client.Close(...)` should still use a *fresh* timeout context (e.g., `context.WithTimeout(context.Background(), ...)`) so cleanup can happen even if the TUI context is already canceled.

#### Addendum: Option A Notes Expanded

The note above applies to every place where TUI-triggered work is long-lived (streams, actions, watchers, and any future background loops). The core rule is: **derive the work context from the TUI lifetime, but do not reuse the canceled TUI context for shutdown/cleanup**.

Concretely:

- **Start/Run context:** Use the TUI-scoped context (or a child of it) for starting processes, streams, and long-running goroutines. This ensures they are *automatically* canceled when the UI exits.
- **Cleanup context:** Use a fresh timeout context (`context.WithTimeout(context.Background(), ...)`) for `Close`/`Shutdown` calls. If you reuse the already-canceled TUI context for cleanup, the shutdown path may short-circuit and leak resources.
- **Uniform application:** Apply this pattern to **all** similar sites, not just the stream runner. Any operation that currently uses `context.Background()` or `msg.Context()` for long-lived work should instead be rooted in the TUI context.

This is the difference between \"work lifetime\" (tied to the UI) and \"shutdown lifetime\" (bounded cleanup even after cancellation). The addendum is a lifecycle invariant that should hold for every UI-initiated background operation.

### Option B: Explicit Manager Shutdown

**Idea:** Keep current stream contexts but add a TUI shutdown hook that calls `streamManager.StopAll()`.

- Create a `Close()`/`StopAll()` method that cancels all active streams and closes clients.
- Call it from `tui.go` after `program.Run()` returns or when `egCtx` is canceled.

This works but is more error-prone because it relies on explicit shutdown calls. Option A is more structural and safer.

### Option C: Publish UI Requests with TUI Context

**Idea:** Use `message.NewMessageWithContext(tuiCtx, ...)` when publishing UI requests, so `msg.Context()` inherits the TUI lifetime.

- This would affect both action and stream runners without changing their signatures.
- However, it couples all message handling to the UI lifetime and still relies on message context semantics.

This can be complementary but is less explicit than Option A.

## Suggested End-State (Lifecycle-Invariant Design)

- Stream and action runners receive a TUI-scoped context at registration.
- Per-stream work derives from that context and is canceled when the TUI exits.
- Long-running plugin processes are started with that context so they are cleaned up on TUI shutdown.
- Cleanup uses background + timeout contexts so shutdown completes even after cancellation.

## Bubbletea Integration Notes

Bubbletea supports `tea.WithContext(...)` (v1.3.10) to tie program cancellation to an external context. This can be used to ensure that canceling the TUI context stops the UI loop, not just the bus and watchers. This is adjacent to the stream issue, but it makes the lifecycle model more consistent.

## Validation Checklist

- Start a stream and quit the TUI; verify plugin process terminates.
- Ensure no goroutines are blocked on bus publishing after TUI exit.
- Confirm that user-initiated stop still results in a clean `StreamEnded` event.
- Validate that action runner operations are canceled when TUI exits (if aligned).

## Open Questions

- Should stream shutdown on TUI exit publish a final `StreamEnded` event, or silently stop?
- Do we want any streams to survive TUI exit (e.g., handoff to CLI), or is a hard stop always correct?
- Should action runner also be tied to the TUI context, or should actions complete even after exit?
