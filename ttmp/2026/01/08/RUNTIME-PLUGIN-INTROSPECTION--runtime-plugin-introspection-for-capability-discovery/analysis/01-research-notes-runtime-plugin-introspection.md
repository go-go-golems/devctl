---
Title: Research Notes - Runtime Plugin Introspection
Ticket: RUNTIME-PLUGIN-INTROSPECTION
Status: active
Topics:
    - devctl
    - plugins
    - introspection
DocType: analysis
Intent: long-term
Owners: []
RelatedFiles:
    - Path: devctl/pkg/doc/topics/devctl-plugin-authoring.md
      Note: handshake contract and capability docs
    - Path: devctl/pkg/doc/topics/devctl-user-guide.md
      Note: handshake example
    - Path: devctl/pkg/protocol/types.go
      Note: handshake capabilities schema
    - Path: devctl/pkg/runtime/factory.go
      Note: handshake read + timeouts
    - Path: devctl/pkg/runtime/runtime_test.go
      Note: handshake noise and timeout behavior
    - Path: devctl/pkg/tui/models/plugin_model.go
      Note: capability rendering + stream indicator
    - Path: devctl/pkg/tui/state_events.go
      Note: PluginSummary fields for ops/streams
    - Path: devctl/pkg/tui/state_watcher.go
      Note: reads plugin config; candidate site for introspection
ExternalSources: []
Summary: Research findings and implementation outline for runtime plugin introspection in devctl.
LastUpdated: 2026-01-08T13:30:00-05:00
WhatFor: Capture how plugin handshakes work today, constraints, and a concrete research/implementation path.
WhenToUse: Use when designing or implementing plugin capability discovery in the TUI.
---


# Research Notes: Runtime Plugin Introspection

## Scope and objective

The goal is to surface plugin capabilities (ops, streams, commands) in the devctl TUI without requiring a running pipeline. The research focuses on the existing runtime/handshake behavior, constraints around plugin startup, and the safest integration points for introspection. The output is an actionable outline that complements the research plan with concrete observations and recommended next steps.

## Current behavior (observed)

### Static discovery vs runtime handshake

- `repository.Load()` reads `.devctl.yaml` and returns `[]runtime.PluginSpec` without capabilities.
- `runtime.Factory.Start()` launches the plugin process and blocks until the handshake is read from stdout (`readHandshake`).
- `runtime.Client.Handshake()` exposes `Capabilities{Ops, Streams, Commands}` only after the plugin is running.

This means the TUI `StateWatcher` currently has no path to capability data without starting plugins.

### TUI surfaces

- `PluginSummary` includes `Ops` and `Streams` but they are never populated.
- `PluginModel` already renders stream indicators and ops/streams lists if present.
- `Dashboard` renders a plugin summary which could also display capability hints.

### Protocol and handshake constraints

- Handshake must be the first stdout frame, otherwise start fails (`TestRuntime_NoiseBeforeHandshakeFailsStart`).
- Handshake timeout defaults to 2s (`FactoryOptions.HandshakeTimeout`).
- Stderr is allowed, but stdout contamination after handshake breaks calls (`TestRuntime_NoiseAfterHandshakeFailsCall`).
- Plugin authoring docs specify that handshake is the first frame and includes `capabilities.ops`, `capabilities.streams`, and `capabilities.commands`.

These constraints imply introspection must be careful about process lifecycle, stdout cleanliness, and timeouts.

## Integration constraints and implications

1) **Starting a plugin has side effects**
   - Some plugins may mutate state or spawn subprocesses on startup.
   - An immediate `Start()` + `Close()` might still trigger these side effects.

2) **Handshake latency is user-facing**
   - With the default 2s timeout, slow-starting plugins risk noisy errors in the UI.
   - If introspection happens in the background, timeouts should degrade gracefully.

3) **No protocol handshake cache exists**
   - Capabilities are ephemeral and re-read only during operations (up/plan/stream).
   - This is the core gap for TUI capability display.

4) **Commands are part of capabilities**
   - Dynamic commands are advertised via handshake and are not discoverable statically.
   - Introspection should include commands to keep CLI and TUI in sync.

## Candidate approaches (re-evaluated)

### A) StateWatcher background introspection (lowest friction)

- Spawn a background goroutine that starts each plugin, reads handshake, and closes immediately.
- Cache results in memory and update `StateSnapshot` with ops/streams.
- UI updates once data becomes available.

Pros:
- Smallest changeset.
- Reuses existing runtime start and handshake validation.

Cons:
- Plugins may have side effects on startup.
- Some plugins may exceed handshake timeout.

Pseudocode sketch:

```go
func (w *StateWatcher) runIntrospection(ctx context.Context) {
  repo, _ := repository.Load(...)
  factory := runtime.NewFactory(runtime.FactoryOptions{HandshakeTimeout: 2 * time.Second})
  for _, spec := range repo.Specs {
    c, err := factory.Start(ctx, spec, runtime.StartOptions{Meta: repo.Request})
    if err != nil { recordErr(spec, err); continue }
    hs := c.Handshake()
    cache(spec.ID, hs)
    _ = c.Close(ctx)
  }
  publishSnapshotWithCaps()
}
```

### B) Repository LoadWithIntrospection (cleaner API, blocking)

- Extend repository loading to include optional introspection.
- Useful for non-TUI contexts that can afford blocking introspection.

Pros:
- Centralizes capability data with specs.

Cons:
- Needs careful use to avoid blocking UI startup.

### C) Capability cache on disk (performance)

- Store handshake data in `.devctl/plugin-cache.json` with timestamps and spec hashes.
- Use cache for instant UI display, then refresh in the background.

Pros:
- Fast startup even with many plugins.

Cons:
- Stale data, cache invalidation complexity.

### D) Protocol-level introspection op (explicit semantics)

- Add a dedicated introspection request in the protocol.
- This avoids a full plugin lifecycle but requires protocol and plugin changes.

Pros:
- Clear separation of concerns.

Cons:
- Highest coordination cost; not suitable for MVP.

## Recommended research outline (concrete)

### 1) Validate Start+Close viability

- Run `Factory.Start()` followed by immediate `Close()` on representative plugins.
- Observe whether plugins perform side effects or leave stray processes.
- Record handshake and shutdown timings.

### 2) Measure handshake latency

- Instrument timing around `readHandshake` to capture elapsed time per plugin.
- Compare against default 2s timeout; identify if a longer timeout is necessary.

### 3) Identify safe introspection boundaries

- Verify whether plugins emit extra stdout noise on start or shutdown.
- Confirm that stderr-only logs do not interfere.

### 4) Prototype StateWatcher introspection

- Implement a branch of `StateWatcher` that performs background introspection.
- Confirm UI updates correctly when caps map is populated.
- Confirm `PluginSummary.Ops/Streams` reach `PluginModel` and stream indicator shows.

### 5) Decide on caching

- If startup latency is too high or plugins are slow, design a small cache layer.
- Key cache by plugin path + args + mtime to reduce staleness.

## Suggested implementation checkpoints

- Add `StateWatcher.introspectPlugins()` with `sync.Once` guard.
- Add `StateWatcher.pluginCaps map[string]protocol.Handshake` and mutex.
- Populate `PluginSummary` from cached handshakes in `readPlugins()`.
- Publish a new `StateSnapshot` immediately after introspection completes.

## Risks and mitigations

- **Plugin side effects on start**: document plugin guidance (avoid side effects before handshake), add warning in UI on introspection failures.
- **Handshake timeout**: allow a per-introspection timeout override (e.g., 5s) to reduce spurious failures.
- **Noisy stdout**: detect and surface contamination errors with file/line hints.
- **Resource usage**: limit introspection concurrency (one at a time) to avoid CPU spikes.

## TUI integration notes

- `PluginModel` already supports stream indicator and capability lists; data wiring is the missing link.
- `Dashboard` plugin summary should include the presence/absence of streams if available.
- A soft UI status like "introspecting..." could reduce confusion while waiting for results.

## References (code + docs)

- `devctl/pkg/tui/state_watcher.go` (plugin read path)
- `devctl/pkg/tui/state_events.go` (`PluginSummary` fields)
- `devctl/pkg/runtime/factory.go` (handshake start + timeouts)
- `devctl/pkg/runtime/runtime_test.go` (handshake noise + timeout behavior)
- `devctl/pkg/tui/models/plugin_model.go` (capability display)
- `devctl/pkg/protocol/types.go` (handshake capabilities)
- `devctl/pkg/doc/topics/devctl-plugin-authoring.md` (handshake + capabilities contract)
- `devctl/pkg/doc/topics/devctl-user-guide.md` (handshake example)

## Clarifications, constraints updates, and design plan

### Why StateWatcher does not have handshake data today

- `Factory.Start()` reads the handshake only when plugins are explicitly started for an operation (via `repository.StartClients()`).
- `StateWatcher` never starts plugins; it only reads config in `readPlugins()` and thus has no runtime handshake data to attach to `PluginSummary`.
- `repository.Load()` is a static loader (config + discovery). It does not start plugins or cache handshakes. It is invoked in multiple contexts (CLI commands, TUI actions), not a single global bootstrap that persists across the app.

In short: handshakes exist only in the short-lived runtime clients created for specific operations, and they are not stored in the repository or state snapshot.

### Updated constraints (from ticket owner)

- **Handshake-only startup is considered side-effect-free**; any side effects are plugin author responsibility.
- **Slow or hanging plugin start should be visible**: show status to the user rather than hiding it behind timeouts or background-only flows.
- **Capabilities should be cached on startup** with a clear, explicit refresh option.

### Design plan (aligned to updated constraints)

#### Approach overview

Use a startup introspection pass to populate a capabilities cache that feeds `StateWatcher` snapshots. Provide an explicit refresh path to re-run introspection on demand. Defer any persistent on-disk cache to a later phase (future work).

This combines the simplicity of Avenue A (StateWatcher background introspection) with the optional API cleanliness of Avenue B (repository introspection mode), but keeps the runtime behavior visible to users.

#### Proposed behavior

1) **Startup introspection**
   - On TUI startup, trigger a repository introspection pass that starts each plugin, reads handshake, and closes the client.
   - Store results in an in-memory cache keyed by plugin ID (and possibly spec hash).
   - Publish a `StateSnapshot` update once introspection completes (or partially completes).
   - If a plugin hangs or is slow, surface “introspecting…” and show elapsed time.

2) **Explicit refresh**
   - Add a UI action or CLI command to re-run introspection (e.g., `[r] refresh` in Plugins view).
   - On refresh, re-run the same Start+Close cycle and update the cache.

3) **Error handling**
   - Preserve per-plugin error state (timeout, invalid handshake, start error).
   - Render in the Plugins view as status text and keep the rest of the list usable.

#### Data flow sketch

```
TUI startup -> repo introspect -> Factory.Start() -> handshake -> cache
  -> StateWatcher snapshot -> PluginSummary.Ops/Streams/Commands
  -> PluginModel renders stream indicator + capability lists
```

#### Recommended API shape (minimal)

Option A (StateWatcher-owned introspection):

```go
type PluginIntrospection struct {
  Handshake protocol.Handshake
  Err       error
  StartedAt time.Time
  FinishedAt time.Time
}

type StateWatcher struct {
  // ...
  pluginCaps map[string]PluginIntrospection
}
```

Option B (Repository introspection helper):

```go
func (r *Repository) Introspect(ctx context.Context, factory *runtime.Factory) map[string]PluginIntrospection
```

StateWatcher can call this on startup and on refresh, without embedding runtime logic directly.

#### Cache scope

- **Now**: in-memory cache only; persists during TUI session.
- **Later**: optional disk cache (Avenue C) for faster startup across runs.

### Implementation notes and UI affordances

- Show per-plugin introspection status: `introspecting`, `ok`, `error: <short reason>`.
- Keep a “last refreshed” timestamp in the snapshot to aid user trust.
- Avoid hiding slow plugins; show elapsed time and allow cancel/refresh.

### Open questions for follow-up

- Should the explicit refresh live in TUI only, CLI only, or both?
- Should a hung plugin be cancelable without restarting the whole TUI?
- Do we need a configurable handshake timeout for introspection separate from runtime operations?

## UX addendum: introspection visibility, control, and trust

This addendum focuses on how users experience runtime introspection in the TUI. The goal is to make introspection explicit, diagnosable, and empowering, not something silently hidden in the background.

### Core UX principles

- **Visibility**: plugin introspection is real work and should be visible, not hidden.
- **Control**: users can explicitly refresh and re-run introspection on demand.
- **Trust**: show timestamps, elapsed time, and errors so users understand the state of their toolchain.
- **Graceful partial success**: a broken plugin should not block discovery for others.

### Where introspection appears in the TUI

1) **Plugins view (primary surface)**
   - Each plugin row shows a status badge: `introspecting`, `ok`, `error`, or `stale`.
   - If introspection is in progress, show elapsed time (e.g., `introspecting (3.4s)`).
   - When a plugin is selected, show detailed status lines in the expanded card.

2) **Dashboard plugin summary (secondary surface)**
   - Show a lightweight indicator if introspection is still running, e.g., `Plugins: 4 ok, 1 error, 1 introspecting`.
   - Keep this passive; do not spam global event log unless errors occur.

3) **Global status / help hints (tertiary)**
   - When the TUI first starts, a short hint appears in the Plugins view: `Introspecting plugin capabilities... [r] refresh`.
   - Once done, the hint disappears.

### Key user actions and behaviors

#### Refresh behavior

- **Action**: `[r] refresh` in Plugins view.
- **Result**: re-runs introspection for all plugins, resets status to `introspecting`.
- **UX detail**: show a spinner/elapsed time per plugin; keep the list interactive.

#### Error detail

- **Action**: `[e] error details` when a plugin shows error.
- **Result**: opens a small details panel showing the error category (timeout, invalid handshake, stdout contamination), plus the last 3 lines of stderr if available.
- **UX detail**: errors should show actionable hints (e.g., “Handshake not first frame on stdout”).

### Status language (consistent and actionable)

Suggested status labels:
- `introspecting` — plugin start in progress, show elapsed time.
- `ok` — handshake received, capabilities cached, show last refreshed time.
- `error` — handshake failed; show short reason inline, longer reason in details panel.
- `stale` — capability cache exists but a refresh is pending or requested.

### Capability presentation behavior

When handshake data is available:
- **Ops**: show as comma list; if more than N (e.g., 6), show summary `+3 more` and allow expand.
- **Streams**: display stream indicator badge and list in expanded view.
- **Commands**: list command names (from handshake) with hint that CLI will auto-wire them.

When handshake data is missing:
- **Ops/Streams**: display `(unknown)` instead of `(none)`.
- **Commands**: display `(unknown)` to avoid implying no support.
- Show a hint: “Not introspected yet. Press [r] to refresh.”

### Error categories and mapping

Map error types to clear labels so users can diagnose quickly:

- **Handshake timeout**: “Handshake timeout (plugin did not respond).”
- **Invalid handshake**: “Invalid handshake JSON.”
- **Unexpected stdout**: “Handshake must be first stdout frame.”
- **Start failure**: “Failed to start plugin process.”

If the error is recoverable, the UI should suggest `Press [r] to retry` in the details panel.

### State transitions

```
unknown -> introspecting -> ok
unknown -> introspecting -> error
ok -> refresh requested -> introspecting -> ok/error
error -> refresh requested -> introspecting -> ok/error
```

### Telemetry and observability (optional)

- Log introspection duration per plugin (debug level).
- Emit an event if a plugin repeatedly fails introspection (e.g., 3 consecutive failures).

### Edge cases to handle explicitly

- **Plugin hangs forever**: show elapsed time and provide a cancel action.
- **Plugin binary missing**: display `error: path not found` with the resolved path.
- **Plugin produces stdout noise**: present actionable guidance from docs.
- **Plugins with zero capabilities**: treat as valid and display `(none)` after successful handshake.

### UX acceptance criteria (addendum)

- Users can see, at a glance, which plugins are introspecting, ok, or failing.
- Users can refresh capabilities without restarting devctl.
- Missing handshake data is labeled as “unknown” (not “none”).
- Every error state has a brief, actionable explanation.
- Long-running introspection is visible with elapsed time and optional cancel.
