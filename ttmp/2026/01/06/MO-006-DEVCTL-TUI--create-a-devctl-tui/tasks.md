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
- [x] Surface exit diagnostics (exit code/signal + stderr tail) for dead services (dashboard row + service view)

## Milestone 2 — Actions + pipeline UX

- [x] Implement an orchestrator that runs up/down/restart in-process and publishes domain events for phases/steps/validation (no shelling out)
- [x] Add a confirmation modal for destructive actions (down/restart), and respect `--timeout`, `--dry-run`, `--strict`
- [x] Add a confirmation modal for destructive actions (kill selected service, for testing exit handling)
- [ ] Implement PipelineModel: phase progress + steps list + last run summary (durations + status)
  - [ ] Define UI message types for pipeline lifecycle:
    - `PipelineRunStarted{RunID, RepoRoot, At}`
    - `PipelinePhaseStarted{RunID, Phase, At}`
    - `PipelinePhaseFinished{RunID, Phase, Status, Duration, Err?}`
    - `PipelineBuildStep{RunID, StepIndex, Name, Status, Duration, Err?}`
    - `PipelinePrepareStep{RunID, StepIndex, Name, Status, Duration, Err?}`
  - [ ] Extend the action runner to publish those messages at natural boundaries (before/after each engine call and per-step).
  - [ ] Render a stable “pipeline timeline” header: current phase, last completed phase, total duration so far.
  - [ ] Render build/prepare step tables with selection + a right-side details pane (or a bottom details section).
  - [ ] Add a “last run result” summary that survives view switching until the next run.
- [ ] Implement validation UX: errors/warnings table + “what to do next”
  - [ ] Define `ValidationResultMsg{RunID, Valid, Errors[], Warnings[]}` with a compact `ValidationIssue` struct:
    - `Code string`
    - `Message string`
    - `Source string` (plugin id / op, if known)
    - `Details map[string]any` (kept small; JSON-ish)
  - [ ] Add a validation view/pane that:
    - shows errors and warnings in separate lists (or a filterable combined list),
    - supports selection + details,
    - provides an explicit next action (e.g., “Fix config and press r to restart”).
  - [ ] Ensure the dashboard status line and event log include a crisp validation summary (e.g. `validate: 3 errors, 1 warning`).
- [ ] Implement cancellation + cleanup for in-flight actions
  - [ ] Add a UI-level “cancel current action” keybinding (e.g. `ctrl+k` or `esc` in modal) that cancels the action context.
  - [ ] Ensure cancellation is propagated through pipeline, runtime plugin clients, and supervisor start/stop.
  - [ ] Publish explicit cancellation events so the UI can reset state cleanly (`ActionCanceled{Kind, RunID}`).

## Milestone 2.1 — Failure UX polish (fast feedback)

- [ ] Improve exit diagnostics ergonomics in the TUI
  - [ ] Cache `*.exit.json` reads (avoid re-parsing on every snapshot tick if unchanged).
  - [ ] Add an expand/collapse toggle for the stderr excerpt in the service view (default collapsed to ~8 lines).
  - [ ] Enrich the event log entry for service exits to include `exit=…` / `sig=…` when known.
- [ ] Add a `--tail-lines` flag to `devctl tui` to control:
  - [ ] initial log viewport tail size
  - [ ] exit stderr excerpt size
  - [ ] `status`-like fallback when exit info is missing

## Milestone 3 — Plugins + richer event streams

- [ ] Implement plugins view: discovery + handshake capabilities, caching, manual refresh; publish a `PluginList` event so both dashboard and plugins view can consume it
- [ ] Implement a plugin “capabilities summary” for the dashboard header (ops present: mutate/validate/build/prepare/plan)
- [ ] Optional: implement a plugin stream adapter (`runtime.Client.StartStream`) that turns `protocol.Event` into domain events and renders them in the EventLogModel
  - [ ] Define a minimal mapping for common plugin stream events (log-ish text, progress, warnings)
  - [ ] Ensure stream shutdown is tied to action lifecycle (no orphan goroutines)

## Docs

- [ ] Document `devctl tui`: usage, keybindings, and the message architecture (bus → transform → forward → models)
- [ ] Troubleshooting doc: stale state, plugin stdout contamination, cancellations/timeouts
- [ ] Update the tmux playbook with a “failure reproduction” section (dead service, stale state, restart prompt)

## Optional — Telemetry & polish

- [ ] Add health display: persist health spec or recompute plan on demand; define UX for unknown/unsupported health
- [ ] Add CPU/MEM sampling: decide dependency vs `/proc`; update UI columns accordingly
