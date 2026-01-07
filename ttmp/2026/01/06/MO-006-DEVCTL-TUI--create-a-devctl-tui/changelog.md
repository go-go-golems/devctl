# Changelog

## 2026-01-06

- Initial workspace created


## 2026-01-06

Initialize ticket workspace; import devctl TUI layout source; add diary doc.

### Related Files

- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/ttmp/2026/01/06/MO-006-DEVCTL-TUI--create-a-devctl-tui/diary/01-diary.md — Start diary for design iterations
- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/ttmp/2026/01/06/MO-006-DEVCTL-TUI--create-a-devctl-tui/sources/local/01-devctl-tui-layout.md — Imported ASCII layout baseline


## 2026-01-06

Draft TUI design doc with views, keybindings, and milestone-based implementation plan.

### Related Files

- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/ttmp/2026/01/06/MO-006-DEVCTL-TUI--create-a-devctl-tui/design-doc/01-devctl-tui-layout-and-implementation-design.md — Primary design baseline
- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/ttmp/2026/01/06/MO-006-DEVCTL-TUI--create-a-devctl-tui/sources/local/01-devctl-tui-layout.md — Referenced ASCII baseline


## 2026-01-06

Add code-mapping analysis doc tying TUI design to existing devctl packages and identifying MVP vs optional data sources.

### Related Files

- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/ttmp/2026/01/06/MO-006-DEVCTL-TUI--create-a-devctl-tui/working-note/01-devctl-tui-code-mapping-and-integration-analysis.md — Design-to-code mapping and integration seams


## 2026-01-06

Revise design doc to distinguish MVP vs optional fields and define stale-state dashboard behavior.

### Related Files

- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/ttmp/2026/01/06/MO-006-DEVCTL-TUI--create-a-devctl-tui/design-doc/01-devctl-tui-layout-and-implementation-design.md — Updated MVP/optional field definitions and stale-state policy


## 2026-01-06

Populate tasks.md with milestone-based implementation tasks for the devctl TUI (M0–M3 plus optional M4/M5).

### Related Files

- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/ttmp/2026/01/06/MO-006-DEVCTL-TUI--create-a-devctl-tui/tasks.md — Concrete incremental task list


## 2026-01-06

Doc hygiene: validate frontmatter; add frontmatter to imported layout source so doctor passes without errors.

### Related Files

- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/ttmp/2026/01/06/MO-006-DEVCTL-TUI--create-a-devctl-tui/sources/local/01-devctl-tui-layout.md — Add frontmatter so docmgr can parse/validate imported artifact


## 2026-01-06

Add a dedicated ASCII layout baseline design doc (excerpted screens) and link it to the imported source.

### Related Files

- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/ttmp/2026/01/06/MO-006-DEVCTL-TUI--create-a-devctl-tui/design-doc/02-devctl-tui-layout-ascii-baseline.md — Baseline layout spec (design-doc)
- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/ttmp/2026/01/06/MO-006-DEVCTL-TUI--create-a-devctl-tui/sources/local/01-devctl-tui-layout.md — Imported source backing the baseline


## 2026-01-06

Update ticket index summary and add key links to design/analysis/diary docs.

### Related Files

- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/ttmp/2026/01/06/MO-006-DEVCTL-TUI--create-a-devctl-tui/index.md — Improve discoverability of core docs


## 2026-01-06

Expand code-mapping working note with Watermill→Bubble Tea event architecture (based on bobatea/pinocchio), model/file layout sketches, and routing diagrams.

### Related Files

- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/ttmp/2026/01/06/MO-006-DEVCTL-TUI--create-a-devctl-tui/working-note/01-devctl-tui-code-mapping-and-integration-analysis.md — Add Watermill integration + Bubble Tea model composition and event routing


## 2026-01-06

Refine working note prose for Watermill→Bubble Tea architecture; update tasks.md to reflect message-based decomposition.

### Related Files

- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/ttmp/2026/01/06/MO-006-DEVCTL-TUI--create-a-devctl-tui/tasks.md — Decompose work into domain events
- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/ttmp/2026/01/06/MO-006-DEVCTL-TUI--create-a-devctl-tui/working-note/01-devctl-tui-code-mapping-and-integration-analysis.md — More engaging narrative + clearer mental model

## 2026-01-06

Ticket moved under devctl/ttmp; update docmgr root and repair RelatedFiles/path references.

### Related Files

- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/ttmp/2026/01/06/MO-006-DEVCTL-TUI--create-a-devctl-tui/diary/01-diary.md — Record ticket move and relationship repairs
- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/ttmp/2026/01/06/MO-006-DEVCTL-TUI--create-a-devctl-tui/index.md — Fix RelatedFiles to point at moved docs


## 2026-01-06

Implement M0 TUI skeleton: Watermill bus + domain→UI transformer + UI forwarder; add devctl tui command; add state watcher and minimal dashboard/eventlog models.

### Related Files

- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/cmd/devctl/cmds/tui.go — New devctl tui command wiring bus/watcher/program
- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/pkg/tui/bus.go — In-memory Watermill router/pubsub used as event spine
- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/pkg/tui/forward.go — UI envelope → Program.Send forwarder
- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/pkg/tui/models/root_model.go — Root model composing dashboard + event log
- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/pkg/tui/state_watcher.go — Publishes state snapshots from .devctl/state.json
- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/pkg/tui/transform.go — Domain→UI envelope transformer


## 2026-01-06

Add tmux testing playbook for devctl tui and silence Watermill logs in the TUI bus.

### Related Files

- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/pkg/tui/bus.go — Use watermill.NopLogger to avoid polluting the TUI
- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/ttmp/2026/01/06/MO-006-DEVCTL-TUI--create-a-devctl-tui/playbook/01-playbook-testing-devctl-tui-in-tmux.md — Repeatable procedure for fixture repo + tmux capture


## 2026-01-06

Enable Bubble Tea alternate screen for devctl tui (default on; --alt-screen toggle).

### Related Files

- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/cmd/devctl/cmds/tui.go — Enable tea.WithAltScreen by default
- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/ttmp/2026/01/06/MO-006-DEVCTL-TUI--create-a-devctl-tui/playbook/01-playbook-testing-devctl-tui-in-tmux.md — Document alt screen expectation


## 2026-01-06

Diary: annotate work session id 019b94f6-bdd4-7c12-8ac3-d6554e018c62.

### Related Files

- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/ttmp/2026/01/06/MO-006-DEVCTL-TUI--create-a-devctl-tui/diary/01-diary.md — Record session id for all work so far

## 2026-01-07

Surface exit diagnostics in the TUI: show a compact `dead (exit=...)` hint on the dashboard and render exit code/signal + a small stderr tail excerpt in the service view (commit bd34996).

### Related Files

- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/pkg/tui/models/dashboard_model.go — Read `*.exit.json` and append dashboard hint for dead services
- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/pkg/tui/models/service_model.go — Render exit details + stderr tail excerpt in the service view
- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/pkg/tui/models/root_model.go — Adjust child sizing so the status line doesn’t break layouts

## 2026-01-07

Expand the ticket’s implementation task breakdown with message-level slices for pipeline/validation/cancellation, plus a small “failure UX polish” milestone (commit b3c648a).

### Related Files

- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/ttmp/2026/01/06/MO-006-DEVCTL-TUI--create-a-devctl-tui/tasks.md — Break down next milestones into message-driven slices

## 2026-01-07

Add a Pipeline view to the TUI and publish structured “pipeline progress” events from the in-TUI action runner so the UI can show phase timings, build/prepare step results, validation summaries, and launch plan basics (commit 97bd82d).

### Related Files

- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/pkg/tui/action_runner.go — Publish pipeline run/phase/result events during up/down/restart
- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/pkg/tui/transform.go — Transform pipeline domain events into UI messages
- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/pkg/tui/forward.go — Forward pipeline UI messages into Bubble Tea
- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/pkg/tui/models/pipeline_model.go — Render pipeline progress and last-run summary
- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/pkg/tui/models/root_model.go — Add `pipeline` view and tab cycling

## 2026-01-06

Finalize M0 baseline: fix doc hygiene (numeric prefix) and land initial TUI skeleton (commit 2e22243).

### Related Files

- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/cmd/devctl/cmds/tui.go — devctl tui wiring + alternate screen option
- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/pkg/tui/bus.go — M0 Watermill bus + NopLogger to keep UI clean
- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/ttmp/2026/01/06/MO-006-DEVCTL-TUI--create-a-devctl-tui/sources/local/01-devctl-tui-layout.md — Renamed imported baseline to satisfy numeric-prefix policy


## 2026-01-06

Implement Milestone 1 log viewer + add in-TUI actions (up/down/restart) via Watermill UI actions topic (commit e2e407b).

### Related Files

- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/cmd/devctl/cmds/tui.go — Wire action runner + silence zerolog by default
- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/pkg/tui/action_runner.go — Run up/down/restart in-process and publish action.log events
- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/pkg/tui/models/service_model.go — Service log viewer (stdout/stderr
- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/ttmp/2026/01/06/MO-006-DEVCTL-TUI--create-a-devctl-tui/playbook/01-playbook-testing-devctl-tui-in-tmux.md — Update playbook for logs/actions keys


## 2026-01-06

Improve restart ergonomics: devctl up prompts when state exists (stale vs running), and TUI shows a persistent status line + prompts restart on u when state exists (commit 8677065).

### Related Files

- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/cmd/devctl/cmds/up.go — Add interactive confirmation when state exists (TTY only)
- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/pkg/tui/models/dashboard_model.go — Prompt restart on u when state exists
- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/pkg/tui/models/root_model.go — Persistent Status line for action failures/success


## 2026-01-06

Add persistent exit diagnostics: supervise wrapper writes per-service exit JSON (exit code + stderr tail) and devctl status surfaces it (commit 23cacc9).

### Related Files

- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/cmd/devctl/cmds/status.go — Include exit info + tail in status output
- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/pkg/state/exit_info.go — ExitInfo schema and read/write
- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/pkg/supervise/supervisor.go — Launch via wrapper when WrapperExe set
