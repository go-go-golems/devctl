# Tasks

## Milestone 0 — Foundations (messages + UI shell)

- [x] Decide where the TUI code lives (`devctl/pkg/tui` vs `devctl/cmd/devctl`) and lock Watermill topics (`devctl.events`, `devctl.ui.msgs`, optional `devctl.ui.actions`)
- [x] Define the message vocabulary: domain event envelope + core payloads (pipeline/service/state) and the UI `tea.Msg` types those map to
- [x] Add an in-memory Watermill bus lifecycle (router + pubsub + Run/Close; context-driven shutdown)
- [x] Add a transformer layer: `devctl.events` → `devctl.ui.msgs` (UIEnvelope + initial mapping set)
- [x] Add a program forwarder: `devctl.ui.msgs` → `tea.Program.Send(tea.Msg)`
- [x] Add the `devctl tui` command skeleton (enter/exit, global flags wired, help overlay)
- [x] Create Bubble Tea model skeleton (one model per file): root + dashboard + service + pipeline + plugins + event log + status bar
- [x] Implement EventLogModel as the first sink: show a readable stream of “what just happened”
- [x] Implement state watcher: read `.devctl/state.json`, compute liveness, publish `StateSnapshot` / `ServiceExitObserved`, and render the snapshot on the dashboard (including stale-state UX)

## Milestone 1 — Logs (service detail)

- [x] Implement service detail navigation and selection-driven service context (from dashboard)
- [x] Implement logs viewer model: stdout/stderr tabs, follow toggle, scrollback cap
- [x] Plumb log updates into the model (direct file tailing, or via Watermill messages if we want a single event path)
- [x] Add log search/filter UI (substring search + highlight; clear search)

## Milestone 2 — Actions + pipeline UX

- [x] Implement an orchestrator that runs up/down/restart in-process and publishes domain events for phases/steps/validation (no shelling out)
- [x] Add a confirmation modal for destructive actions (down/restart), and respect `--timeout`, `--dry-run`, `--strict`
- [x] Add a confirmation modal for destructive actions (kill selected service, for testing exit handling)
- [ ] Implement PipelineModel: phase progress, build/prepare step lists, last run summary (durations + status)
- [ ] Implement validation UX: errors/warnings table (code/message/details) and a clear “what to do next” affordance
- [ ] Implement cancellation + cleanup: cancel in-flight up, ensure plugin clients closed, keep UI responsive during long phases

## Milestone 3 — Plugins + richer event streams

- [ ] Implement plugins view: discovery + handshake capabilities, caching, manual refresh; publish a `PluginList` event so both dashboard and plugins view can consume it
- [ ] Optional: implement a plugin stream adapter (`runtime.Client.StartStream`) that turns `protocol.Event` into domain events and renders them in the EventLogModel

## Docs

- [ ] Document `devctl tui`: usage, keybindings, and the message architecture (bus → transform → forward → models)
- [ ] Troubleshooting doc: stale state, plugin stdout contamination, cancellations/timeouts

## Optional — Telemetry & polish

- [ ] Add health display: persist health spec or recompute plan on demand; define UX for unknown/unsupported health
- [ ] Add CPU/MEM sampling: decide dependency vs `/proc`; update UI columns accordingly
