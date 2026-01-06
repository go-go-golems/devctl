---
Title: Diary
Ticket: MO-006-DEVCTL-TUI
Status: active
Topics:
    - backend
    - ui-components
DocType: diary
Intent: long-term
Owners: []
RelatedFiles:
    - Path: devctl/ttmp/2026/01/06/MO-006-DEVCTL-TUI--create-a-devctl-tui/index.md
      Note: Ticket overview updated by docmgr import
    - Path: devctl/ttmp/2026/01/06/MO-006-DEVCTL-TUI--create-a-devctl-tui/sources/local/01-devctl-tui-layout.md
      Note: Input layout mockups imported from /tmp/devctl-tui.md
ExternalSources: []
Summary: ""
LastUpdated: 2026-01-06T15:24:44.538361047-05:00
WhatFor: ""
WhenToUse: ""
---




# Diary

## Goal

Capture the work and decisions involved in designing (and later implementing) a `devctl` TUI, including doc imports, design iterations, and task breakdown.

## Session

Session: `019b94f6-bdd4-7c12-8ac3-d6554e018c62`

## Step 1: Create ticket workspace and import layout source

Set up a new `docmgr` ticket workspace for the `devctl` TUI work and imported the provided `/tmp/devctl-tui.md` as a tracked source artifact under the ticket. This establishes a stable “layout baseline” (ASCII mock screens) that the subsequent implementation design can reference.

I also created the ticket diary doc early so that subsequent design iterations and mapping decisions are recorded incrementally rather than as one big end-of-session dump.

### What I did
- Ran `docmgr ticket create-ticket --ticket MO-006-DEVCTL-TUI --title "Create a devctl TUI" --topics backend,ui-components`
- Imported the provided layout Markdown via `docmgr import file --ticket MO-006-DEVCTL-TUI --file /tmp/devctl-tui.md --name "devctl-tui-layout"`
- Added a diary doc via `docmgr doc add --ticket MO-006-DEVCTL-TUI --doc-type diary --title "Diary"`

### Why
- Keep all TUI work (design, analysis, tasks, sources) in one ticket workspace with consistent metadata.
- Preserve `/tmp/devctl-tui.md` as an immutable input artifact, separate from the design docs we’ll edit.

### What worked
- `docmgr import file` placed the layout doc at `.../sources/local/devctl-tui-layout.md` (now `.../sources/local/01-devctl-tui-layout.md`) and updated the ticket `index.md`.

### What didn't work
- Typo while listing the ticket directory: `ls -λα ...` failed with `ls: invalid option -- 'á'`.

### What I learned
- `docmgr import` is separate from `docmgr doc`; the correct command is `docmgr import file ...` (not `docmgr doc import`).

### What was tricky to build
- N/A (no code/design changes yet; only workspace setup and source import).

### What warrants a second pair of eyes
- N/A.

### What should be done in the future
- Add a dedicated design doc that “canonizes” the imported ASCII layout and links back to the imported source file.

### Code review instructions
- Start at `devctl/ttmp/2026/01/06/MO-006-DEVCTL-TUI--create-a-devctl-tui/index.md` and the imported source in `.../sources/local/01-devctl-tui-layout.md`.
- Validate by running `docmgr ticket list --ticket MO-006-DEVCTL-TUI` and `docmgr doc list --ticket MO-006-DEVCTL-TUI`.

### Technical details
- Imported source location: `devctl/ttmp/2026/01/06/MO-006-DEVCTL-TUI--create-a-devctl-tui/sources/local/01-devctl-tui-layout.md`

## Step 2: Draft the TUI design document (layout + incremental milestones)

Wrote the first-pass design document describing the TUI’s screens, keybindings, data sources, and a milestone-based implementation plan. The key constraint baked into the design is that the TUI should provide value even before we add new persisted state (CPU/MEM, health specs, event streams).

This step intentionally “anchors” the scope around what exists today in `devctl` (state.json + logs + pipeline methods) and treats everything else as optional follow-on work.

### What I did
- Created and drafted `design-doc/01-devctl-tui-layout-and-implementation-design.md`
- Linked the design back to the imported ASCII baseline (`sources/local/01-devctl-tui-layout.md`)

### Why
- Capture a single canonical view of the intended UX and incremental delivery plan before starting deeper code mapping.

### What worked
- The design doc guidelines (executive summary → problem → proposed solution → decisions → alternatives → plan) were a good scaffold for keeping the doc readable while still detailed.

### What didn't work
- N/A.

### What I learned
- The current persisted state (`devctl/pkg/state`) is intentionally minimal, so any “rich dashboard” fields from the mockups (CPU/MEM/health) must be phased in, not assumed.

### What was tricky to build
- Designing a “screen parity” plan with the ASCII mockups while being honest about missing data sources, without overcommitting to invasive changes up front.

### What warrants a second pair of eyes
- The boundary between “must-have MVP” and “optional enhancements” (especially around health polling and persisted launch plans) could be debated; it will influence how invasive the first implementation needs to be.

### What should be done in the future
- Write an explicit “code mapping” analysis doc to ground the design in current `devctl/pkg/*` APIs and identify the smallest seams we need for a first implementation.

### Code review instructions
- Review `devctl/ttmp/2026/01/06/MO-006-DEVCTL-TUI--create-a-devctl-tui/design-doc/01-devctl-tui-layout-and-implementation-design.md` for correctness and scope.
- Cross-check the “Data sources” section against `devctl/pkg/state`, `devctl/cmd/devctl/cmds/logs.go`, and `devctl/cmd/devctl/cmds/up.go`.

### Technical details
- Design doc path: `devctl/ttmp/2026/01/06/MO-006-DEVCTL-TUI--create-a-devctl-tui/design-doc/01-devctl-tui-layout-and-implementation-design.md`

## Step 3: Write the code-mapping analysis document (design → existing `pkg/*`)

Produced a mapping document that ties each proposed UX feature to the existing `devctl` packages and the current cobra command implementations. The main outcome is clarity about what we can ship without touching persisted state (dashboard + logs) versus what needs explicit new seams (health, CPU/MEM, real event streams).

This step also identifies the likely “cleanest” integration approach: implement `devctl tui` in-process by reusing `devctl/pkg/*` directly (as `cmds/up.go` already does), rather than shelling out and parsing output.

### What I did
- Created and drafted `working-note/01-devctl-tui-code-mapping-and-integration-analysis.md`
- Enumerated package-level reuse points and identified missing data sources relative to the ASCII mockups

### Why
- Avoid designing a TUI that implicitly requires invasive refactors (state schema changes, daemonization) before we have an MVP.

### What worked
- The codebase already has clear separations (pipeline vs supervision vs persisted state), making it straightforward to reuse logic in a new TUI entry point.

### What didn't work
- N/A.

### What I learned
- `runtime.Client` has stream support (`StartStream`) and event routing, but the current CLI never exercises it; event timelines will require explicit work beyond the TUI layer.

### What was tricky to build
- Keeping the mapping actionable (specific files/symbols) while not prematurely deciding exact package layouts or forcing an early refactor of `cmds/*`.

### What warrants a second pair of eyes
- The proposed “seams” (shared helpers in `pkg/` vs duplicating `cmds/*` logic) will affect long-term maintainability; it’s worth reviewing before implementation begins.

### What should be done in the future
- Update the design doc to mark which fields/interactions are MVP vs optional enhancements, based on the mapping.

### Code review instructions
- Read `devctl/ttmp/2026/01/06/MO-006-DEVCTL-TUI--create-a-devctl-tui/working-note/01-devctl-tui-code-mapping-and-integration-analysis.md` top-to-bottom.
- Cross-check the referenced packages: `devctl/pkg/state`, `devctl/pkg/engine`, `devctl/pkg/runtime`, `devctl/pkg/discovery`, `devctl/pkg/supervise`, and `devctl/cmd/devctl/cmds/{up,down,logs,plugins}.go`.

### Technical details
- Analysis doc path: `devctl/ttmp/2026/01/06/MO-006-DEVCTL-TUI--create-a-devctl-tui/working-note/01-devctl-tui-code-mapping-and-integration-analysis.md`

## Step 4: Revise the design doc based on code mapping realities

Updated the design document to explicitly separate MVP fields (available today from state.json + logs) from optional enhancements (health polling, CPU/MEM, plugin stream events). This makes the milestone plan implementable without needing early schema changes or new dependencies.

The revision also adds a clear “stale state” policy for the dashboard: if the state file exists but processes are dead, the UI should say so and make cleanup trivial.

### What I did
- Updated the design doc’s “Data sources” section with explicit MVP vs optional field lists
- Added an MVP policy for stale state detection and remediation

### Why
- Prevent scope creep and reduce the risk that the first TUI iteration becomes blocked on non-essential telemetry.

### What worked
- The mapping doc made it obvious which fields are truly available vs assumed from the mockups.

### What didn't work
- N/A.

### What I learned
- The simplest “correct” dashboard is largely a renderer over `state.State` + file-follow; everything else is a deliberate product choice.

### What was tricky to build
- Writing requirements that are simultaneously ambitious (match the mockups conceptually) and honest (don’t promise fields we can’t derive).

### What warrants a second pair of eyes
- The proposed stale-state behavior (especially whether it should auto-suggest `down` vs simply warn) affects safety/UX; worth reviewing.

### What should be done in the future
- Translate milestones into tasks with acceptance criteria and clear “done” definitions.

### Code review instructions
- Review `devctl/ttmp/2026/01/06/MO-006-DEVCTL-TUI--create-a-devctl-tui/design-doc/01-devctl-tui-layout-and-implementation-design.md` for MVP/optional clarity and consistency with the analysis doc.

### Technical details
- Revised design doc: `devctl/ttmp/2026/01/06/MO-006-DEVCTL-TUI--create-a-devctl-tui/design-doc/01-devctl-tui-layout-and-implementation-design.md`

## Step 5: Create an incremental task breakdown

Converted the milestone plan into a concrete task list in `tasks.md`, covering an MVP dashboard/logs experience first, then actions and pipeline UX, and finally optional enrichment work (health, CPU/MEM, plugin stream events). The tasks are ordered so that each step produces user-visible value and can be reviewed independently.

### What I did
- Added milestone-oriented tasks to `tasks.md` via `docmgr task add`

### Why
- Provide an actionable path from “design intent” to implementation, with clear checkpoints.

### What worked
- `docmgr task add` cleanly appends tasks and keeps `tasks.md` as the single source of truth for checklist tracking.

### What didn't work
- N/A.

### What I learned
- Keeping “optional” milestones explicitly labeled avoids accidental scope creep during implementation.

### What was tricky to build
- Picking task granularity that’s neither too coarse (hard to review) nor too fine (too many micro-tasks).

### What warrants a second pair of eyes
- The ordering of M2/M3 tasks (actions vs pipeline UI) can be debated depending on which pain points are most urgent for the team.

### What should be done in the future
- As soon as an actual TUI library is chosen, refine the MVP tasks to include the specific view components (lists, viewports) we’ll rely on.

### Code review instructions
- Review `devctl/ttmp/2026/01/06/MO-006-DEVCTL-TUI--create-a-devctl-tui/tasks.md` for completeness and sequencing.

### Technical details
- Task list file: `devctl/ttmp/2026/01/06/MO-006-DEVCTL-TUI--create-a-devctl-tui/tasks.md`

## Step 6: Doc hygiene (frontmatter validation + doctor)

Validated frontmatter for the docs created in this ticket and ran `docmgr doctor` to catch workspace hygiene issues. The imported layout file under `sources/` initially triggered a doctor error because it lacked YAML frontmatter, so I added minimal frontmatter to treat it as a reference artifact (while preserving the imported content).

### What I did
- Ran `docmgr validate frontmatter` on the ticket docs (index, design-doc, working-note, diary)
- Ran `docmgr doctor --ticket MO-006-DEVCTL-TUI`
- Added YAML frontmatter to `sources/local/devctl-tui-layout.md` (now `sources/local/01-devctl-tui-layout.md`) so `docmgr doctor` recognizes it as a valid Markdown doc

### Why
- Keep the ticket workspace clean and avoid future “frontmatter parse” failures when searching/validating docs.

### What worked
- After adding frontmatter, `docmgr doctor` no longer reports an error for the imported layout file (only a non-blocking numeric-prefix warning).

### What didn't work
- `docmgr validate frontmatter --doc ...` initially failed when I passed a path that already included the docs root (`devctl/ttmp/...`), resulting in a doubled path like `.../devctl/ttmp/devctl/ttmp/...`. Using the docs-root-relative path (e.g., `2026/01/06/...`) worked.

### What I learned
- For `docmgr validate frontmatter`, prefer docs-root-relative paths (under `devctl/ttmp/`) to avoid path resolution surprises.

### What was tricky to build
- N/A (hygiene step).

### What warrants a second pair of eyes
- N/A (we later renamed the imported source to `sources/local/01-devctl-tui-layout.md` to satisfy the numeric-prefix policy).

### What should be done in the future
- If the numeric-prefix warning becomes noisy, consider renaming the imported layout file and updating references.

### Code review instructions
- Run `docmgr doctor --ticket MO-006-DEVCTL-TUI` and confirm there are no errors.
- Spot-check that the imported layout content begins immediately after frontmatter and remains intact.

### Technical details
- Imported layout file: `devctl/ttmp/2026/01/06/MO-006-DEVCTL-TUI--create-a-devctl-tui/sources/local/01-devctl-tui-layout.md`

## Step 7: Create a dedicated “layout baseline” design doc from the imported mockups

To match the ticket intent (“import the layout as a design doc”), I added a dedicated design-doc that excerpts the key ASCII baseline screens (dashboard, service detail, startup/pipeline, validation error state) and links back to the full imported source. This makes the layout easily discoverable via `docmgr doc list` and keeps it alongside the other design docs.

### What I did
- Created `design-doc/02-devctl-tui-layout-ascii-baseline.md` and populated it with curated baseline screens
- Linked it to the full imported baseline in `sources/local/01-devctl-tui-layout.md`

### Why
- Make the imported layout discoverable as a first-class design doc in the ticket workspace, while still retaining the original imported file under `sources/`.

### What worked
- The curated “excerpt” doc stays readable while still grounding the UX in concrete screen layouts.

### What didn't work
- N/A.

### What I learned
- `docmgr import file` is best treated as “source ingestion”; creating a proper design-doc on top gives better discoverability and aligns with ticket workflows.

### What was tricky to build
- Balancing completeness (include enough screens to be useful) vs keeping the baseline doc from becoming unwieldy.

### What warrants a second pair of eyes
- Whether we should excerpt additional baseline screens (plugin list, multi-service event/log stream) into the design-doc, or keep them only in the imported source.

### What should be done in the future
- If the team prefers a single authoritative layout doc, consider moving more of the baseline screens into `design-doc/02-...` and treating `sources/` strictly as provenance.

### Code review instructions
- Review `devctl/ttmp/2026/01/06/MO-006-DEVCTL-TUI--create-a-devctl-tui/design-doc/02-devctl-tui-layout-ascii-baseline.md` for fidelity to the baseline and readability.

### Technical details
- Layout baseline design doc: `devctl/ttmp/2026/01/06/MO-006-DEVCTL-TUI--create-a-devctl-tui/design-doc/02-devctl-tui-layout-ascii-baseline.md`

## Step 8: Improve ticket index discoverability

Updated the ticket `index.md` to include a short summary and direct links to the core documents (implementation design, ASCII baseline, code mapping analysis, diary). This makes the ticket easier to navigate for reviewers and future implementation work.

### What I did
- Updated `index.md` summary and added links under “Key Links”

### Why
- Reduce friction when jumping between layout baseline, implementation design, analysis mapping, and tasks.

### What worked
- The index now acts as a true landing page for the ticket’s documentation set.

### What didn't work
- N/A.

### What I learned
- N/A.

### What was tricky to build
- N/A.

### What warrants a second pair of eyes
- N/A.

### What should be done in the future
- N/A.

### Code review instructions
- Open `devctl/ttmp/2026/01/06/MO-006-DEVCTL-TUI--create-a-devctl-tui/index.md` and verify the links resolve.

### Technical details
- Ticket index: `devctl/ttmp/2026/01/06/MO-006-DEVCTL-TUI--create-a-devctl-tui/index.md`

## Step 9: Refine the Watermill/Bubble Tea architecture notes and update tasks

Reworked the code-mapping working note so it reads more like a narrative design rationale, while still keeping the concrete pieces (event vocabulary, message schemas, pseudocode, and diagrams). The goal is to make the Watermill→Bubble Tea coupling feel “obvious” to a reviewer who hasn’t looked at `bobatea`/`pinocchio` recently, and to clarify why we want a bus/transform/forward layer rather than ad-hoc goroutines poking the UI.

I also updated `tasks.md` to reflect this message-driven decomposition: the early milestones now explicitly call out domain events vs UI messages, the transformer/forwarder pipeline, and the model-per-file structure.

### What I did
- Revised the prose and structure of `working-note/01-devctl-tui-code-mapping-and-integration-analysis.md` (more narrative + clearer “mental model”)
- Updated `tasks.md` to include the Watermill message plumbing steps and model composition tasks
- Verified that `docmgr task list` still parses the updated `tasks.md`

### Why
- The initial version captured the right shape, but it was too “notesy”; reviewers benefit from a more written-out explanation.
- The tasks needed to match the refined architecture so implementation can proceed incrementally without re-planning.

### What worked
- The bobatea/pinocchio patterns remain a strong template: Watermill as the concurrency boundary, Bubble Tea as the single-threaded renderer.
- Restructuring tasks into “messages + shell first” makes a clean MVP path: you can render event log + state snapshots before implementing actions.

### What didn't work
- N/A.

### What I learned
- `docmgr task list` is tolerant of headings/sections as long as checkboxes are present; this allows us to keep tasks readable while still machine-listable.

### What was tricky to build
- Getting the right balance between engaging prose and the “do not lose the API shapes” requirement—too much narrative makes it vague, too many bullets makes it sterile.

### What warrants a second pair of eyes
- The proposed message taxonomy/topic names and the exact boundary between “domain events” and “UI messages” are the kind of thing that benefit from early agreement.

### What should be done in the future
- Once a concrete TUI package layout is chosen (`devctl/pkg/tui` vs elsewhere), sync the working note’s file layout section to match the actual directory structure.

### Code review instructions
- Read `devctl/ttmp/2026/01/06/MO-006-DEVCTL-TUI--create-a-devctl-tui/working-note/01-devctl-tui-code-mapping-and-integration-analysis.md` focusing on the “pattern reuse” and “event routing” sections.
- Review `devctl/ttmp/2026/01/06/MO-006-DEVCTL-TUI--create-a-devctl-tui/tasks.md` and confirm the milestone ordering matches how you’d want to implement the system.

### Technical details
- Working note: `devctl/ttmp/2026/01/06/MO-006-DEVCTL-TUI--create-a-devctl-tui/working-note/01-devctl-tui-code-mapping-and-integration-analysis.md`
- Tasks file: `devctl/ttmp/2026/01/06/MO-006-DEVCTL-TUI--create-a-devctl-tui/tasks.md`

## Step 10: Track ticket move to `devctl/ttmp` and repair doc relationships

Moved the ticket workspace under `devctl/ttmp/2026/01/06/...` and updated `docmgr` configuration so the ticket continues to be discoverable via `docmgr ticket tickets` and `docmgr doc list`. After the move, several documents still referenced the old `moments/ttmp/...` paths in frontmatter `RelatedFiles` and in prose, which caused `docmgr doctor` warnings and made clickable paths confusing.

This step re-aligns the documentation to the new root: `RelatedFiles` now points at the moved doc paths under `devctl/ttmp/...`, and the diary/changelog text references were updated so reviewers land in the right place.

### What I did
- Confirmed `docmgr` root is now `devctl/ttmp` (via `.ttmp.yaml` and `docmgr status`)
- Updated `RelatedFiles` entries (via `docmgr doc relate --remove-files` + `--file-note`) so they point to `devctl/ttmp/...`
- Updated lingering `moments/ttmp/...` references in `diary/01-diary.md` and `changelog.md`
- Re-ran `docmgr validate frontmatter` and `docmgr doctor --ticket MO-006-DEVCTL-TUI`

### Why
- Keep doc navigation reliable after reorganizing where ticket workspaces live.
- Ensure `docmgr` tooling (doctor/graph) stays actionable instead of warning-noisy.

### What worked
- `docmgr` correctly discovers the ticket in the new root once `.ttmp.yaml` is updated.
- Repairing `RelatedFiles` removes the “missing_related_file” warnings.

### What didn't work
- N/A.

### What I learned
- `docmgr doctor` is a good “smoke test” after moves/renames because it catches stale `RelatedFiles` pointers immediately.

### What was tricky to build
- Avoiding half-updated pointers: some references live in YAML frontmatter (`RelatedFiles`), others live in prose (diary/changelog).

### What warrants a second pair of eyes
- N/A (numeric-prefix warning resolved by renaming the imported source to `sources/local/01-devctl-tui-layout.md`).

### What should be done in the future
- If we decide to rename the imported source to satisfy numeric-prefix policy, do it with a deliberate link/relationship update (or via a docmgr-supported move/renumber workflow).

### Code review instructions
- Run `docmgr doctor --ticket MO-006-DEVCTL-TUI` and confirm there are no errors.
- Open `devctl/ttmp/2026/01/06/MO-006-DEVCTL-TUI--create-a-devctl-tui/index.md` and confirm the linked docs open correctly.

### Technical details
- Ticket workspace: `devctl/ttmp/2026/01/06/MO-006-DEVCTL-TUI--create-a-devctl-tui`

## Step 11: Implement Milestone 0 skeleton (bus → transform → forward → models)

Implemented the first executable slice of the architecture described in the working note: a Watermill in-memory bus, a domain→UI transformer, and a UI forwarder that injects typed messages into a Bubble Tea program. On the UI side, I added a minimal root model composed of a dashboard and an event log; on the backend side, a state watcher publishes periodic snapshots from `.devctl/state.json`.

This is intentionally “boring” UI: the main goal is to prove the event plumbing and model composition, and to create a safe place to iterate on richer views (services table, pipeline view, actions) without re-threading concurrency each time.

### What I did
- Added `devctl/pkg/tui` (topics, envelopes, bus, transformer, forwarder, state watcher, message types)
- Added `devctl/pkg/tui/models` (root/dashboard/event log models; one model per file)
- Added a new CLI entry: `devctl tui` (`devctl/cmd/devctl/cmds/tui.go`) and registered it in `devctl/cmd/devctl/cmds/root.go`
- Added dependencies to `devctl/go.mod` for Watermill + Bubble Tea and ran `go mod tidy`
- Validated formatting and tests:
  - `cd devctl && go fmt ./...`
  - `cd devctl && go test ./...`

### Why
- Establish the “event spine” early so later features (pipeline progress, validation UX, plugin stream events) become incremental handlers/messages rather than ad-hoc goroutines poking UI state.

### What worked
- The basic plumbing works end-to-end: watcher publishes domain envelopes → transformer emits UI envelopes → forwarder calls `Program.Send` → models update.
- `go test ./...` stayed green after adding the new packages/command.

### What didn't work
- I initially tried to format with `gofmt -w ./...` and got: `stat ./...: no such file or directory` (because `gofmt` expects file paths; `go fmt ./...` is the right tool here).

### What I learned
- Starting with an “event log” view is a great way to debug the system: you can observe what the backend thinks is happening before worrying about polished widgets.

### What was tricky to build
- Coordinating lifecycles: the router, watcher, and Bubble Tea program all need a single cancellation point so we don’t leave goroutines running after exit.

### What warrants a second pair of eyes
- Whether the Watermill std logger is acceptable for a TUI command, or whether we should swap to a quiet logger to avoid accidental terminal noise during UI rendering.

### What should be done in the future
- Upgrade the dashboard from “plain text” to a real selectable services table, but keep state updates flowing through `tea.Msg`.
- Expand the transformer to emit richer UI messages (service rows, pipeline phase updates) rather than only a snapshot + generic event lines.

### Code review instructions
- Start at `devctl/cmd/devctl/cmds/tui.go` for the wiring and lifecycle.
- Then review the event spine: `devctl/pkg/tui/bus.go`, `devctl/pkg/tui/transform.go`, `devctl/pkg/tui/forward.go`, `devctl/pkg/tui/state_watcher.go`.
- Validate with `cd devctl && go test ./...` and run the UI against a repo with `.devctl/state.json`: `cd devctl && go run ./cmd/devctl tui --repo-root /path/to/repo`.

### Technical details
- New package root: `devctl/pkg/tui`

## Step 12: Run the TUI in tmux against a realistic fixture + write a test playbook

Ran `devctl tui` in a real `tmux` session and captured the output to verify that the UI renders the state snapshot as expected (running status + service list). To make this repeatable, I wrote a playbook that generates a “sensible” temporary repo-root using the existing `testdata/plugins/e2e` fixture: it builds two small service binaries, writes a `.devctl.yaml`, runs `devctl up` to persist state, and then launches the TUI against that repo-root.

This exposed one important UX issue: Watermill’s default std logger printed router lifecycle logs into the terminal, polluting the TUI output. Switching the TUI bus to `watermill.NopLogger{}` removes that noise and makes the captured output look like an actual UI.

### What I did
- Used the existing E2E fixture plugin (`devctl/testdata/plugins/e2e/plugin.py`) to create a temporary repo-root with two supervised services and persisted `.devctl/state.json`
- Ran `devctl tui` inside tmux and captured the screen via `tmux capture-pane`
- Wrote a repeatable playbook: `playbook/01-playbook-testing-devctl-tui-in-tmux.md`
- Changed the Watermill bus logger to `watermill.NopLogger{}` so the TUI isn’t polluted by router logs

### Why
- We need a small, deterministic environment to validate the UI without depending on Moments or other large repos.
- A playbook makes it easy to reproduce UI regressions and capture output for review.

### What worked
- The TUI shows `System: Running` and lists the supervised services (PIDs + alive status) when pointed at a repo-root with a valid `.devctl/state.json`.
- The tmux capture is a good “snapshot test” of the UI at this stage.

### What didn't work
- Before switching to `watermill.NopLogger{}`, the captured output included Watermill router logs (e.g., `[watermill] ... Adding handler ...`), which is unacceptable for a TUI.

### What I learned
- Even “harmless” background logs are very visible in a TUI context; the bus/logger choice matters early.

### What was tricky to build
- Cleaning up correctly when testing: if the temp repo-root is deleted before running `devctl down`, the supervisor state is lost and processes can be left running. The playbook orders cleanup as: `down` → kill tmux session → delete repo-root.

### What warrants a second pair of eyes
- Whether we want a configurable logger mode (silent by default, verbose with a flag) for debugging bus routing issues.

### What should be done in the future
- Consider adding a small “fixtures” helper command (`devctl fixtures create`) instead of a shell snippet, if we find ourselves repeating this workflow often.

### Code review instructions
- Run the playbook in `playbook/01-playbook-testing-devctl-tui-in-tmux.md` and confirm the captured output contains only UI text (no router logs).
- Optionally, attach to tmux and press `tab` to switch to the event view, then `q` to exit.

### Technical details
- Playbook: `devctl/ttmp/2026/01/06/MO-006-DEVCTL-TUI--create-a-devctl-tui/playbook/01-playbook-testing-devctl-tui-in-tmux.md`

## Step 13: Enable alternate screen for `devctl tui`

Enabled Bubble Tea’s alternate screen mode for the TUI so the interface runs in a clean screen buffer and, on exit, returns you to your previous terminal contents. This makes the TUI feel much more “app-like” and also helps keep incidental output from mixing with the UI.

Note: alternate screen is not a substitute for silencing background logs; anything written to stdout/stderr while the UI is running can still corrupt the UI. We keep the Watermill router logger disabled as the primary fix for that, and the alternate screen as an additional UX improvement.

### What I did
- Updated `devctl tui` to use Bubble Tea alternate screen by default (`--alt-screen=true`)
- Updated the tmux test playbook’s exit criteria to include alternate screen behavior

### Why
- Improve UX: users expect TUIs to take over the screen and restore the terminal afterwards.
- Reduce “visual noise” when running the UI in tmux and capturing output.

### What worked
- The TUI now runs in an alternate screen buffer by default and restores the previous screen on quit.

### What didn't work
- N/A.

### What I learned
- Alternate screen improves presentation, but log hygiene is still required; any stray logs can still corrupt the UI while it’s active.

### What was tricky to build
- N/A.

### What warrants a second pair of eyes
- Whether `--alt-screen` should default to true (current) or be opt-in for users who prefer inline TUIs.

### What should be done in the future
- Consider adding a “debug mode” that enables Watermill logs to a file (not stdout) while keeping the UI clean.

### Code review instructions
- Run `cd devctl && go run ./cmd/devctl tui --repo-root /path/to/repo` and confirm quitting restores the original terminal screen.
- Run with `--alt-screen=false` and confirm it behaves as before (no alternate screen).

## Step 14: Final doc hygiene + commit the initial TUI skeleton

With the core docs and the Milestone 0 “event spine” in place, I did a quick pass of doc hygiene and then committed the whole slice (code + ticket docs). The main goal here was to get to a clean, shareable baseline where `docmgr doctor` passes and `go test ./...` is green, so we can build the richer UI milestones on top without worrying about missing references or half-staged changes.

This step also resolves the lingering docmgr warning about the imported layout source by renaming it to include a numeric prefix and updating the links/frontmatter that reference it.

**Commit (code):** 2e22243 — "devctl: add tui skeleton + MO-006 ticket docs"

### What I did
- Renamed the imported source to satisfy numeric-prefix policy: `sources/local/devctl-tui-layout.md` → `sources/local/01-devctl-tui-layout.md`
- Updated ticket docs (index/design-doc/diary/changelog) to reference the renamed file
- Verified hygiene and tests:
  - `docmgr doctor --ticket MO-006-DEVCTL-TUI`
  - `cd devctl && go test ./...`
- Committed everything (code + docs) in the `devctl/` git repo

### Why
- Keep the ticket workspace warning-free so it stays easy to search and maintain.
- Land a stable “M0 baseline” commit that others can review and iterate on.

### What worked
- `docmgr doctor` now passes with no findings after the rename + reference updates.
- The TUI skeleton + docs are now in a single commit, making review and rollback straightforward.

### What didn't work
- N/A.

### What I learned
- It’s worth enforcing doc hygiene early; otherwise small warnings accumulate and make doc-centric workflows feel unreliable.

### What was tricky to build
- Avoiding “historical drift” in the diary: the source really was imported under the old name, but we also want all clickable paths to match the current filesystem.

### What warrants a second pair of eyes
- Whether we want a stronger convention for `sources/` naming (e.g., always `01-...` for the primary imported artifact) so imported docs don’t need a follow-up rename.

### What should be done in the future
- If we import additional mockups or artifacts, decide up front whether they should live under `sources/local/NN-*` to avoid docmgr warnings.

### Code review instructions
- Start with `devctl/cmd/devctl/cmds/tui.go` and then the event spine in `devctl/pkg/tui/*`.
- Confirm doc hygiene with `docmgr doctor --ticket MO-006-DEVCTL-TUI`.
- Validate behavior with the playbook in `devctl/ttmp/2026/01/06/MO-006-DEVCTL-TUI--create-a-devctl-tui/playbook/01-playbook-testing-devctl-tui-in-tmux.md`.

### Technical details
- Imported baseline (renamed): `devctl/ttmp/2026/01/06/MO-006-DEVCTL-TUI--create-a-devctl-tui/sources/local/01-devctl-tui-layout.md`

## Step 15: Implement logs + in-TUI actions (Milestone 1 and start of Milestone 2)

This step turns the “plumbing demo” into something you can actually use during a dev loop: the dashboard is now interactive, you can drill into a selected service, and you get a real log viewer with follow, scroll, and quick filtering. The event log also grew into a proper debugging surface (filter/clear), so it’s easier to see what the system thinks is happening without attaching a debugger.

I also introduced the first “real action” path inside the TUI: up/down/restart requests are emitted from the UI, handled via a dedicated Watermill topic (`devctl.ui.actions`), executed in-process using existing `pkg/*` code, and then echoed back into the UI as action log events. To keep the UI readable while these operations run, the TUI command now disables zerolog output by default (with an escape hatch flag).

**Commit (code):** e2e407b — "devctl tui: logs view + in-process actions"

### What I did
- Dashboard improvements:
  - Added selection (`↑/↓`) and navigation to service detail (`enter` / `l`)
  - Added a confirmable “kill selected service” action (`x` then `y`) to exercise exit detection
- Service detail logs (first usable log viewer):
  - stdout/stderr tabs (`tab`)
  - follow toggle (`f`)
  - filter (`/` then enter; `ctrl+l` clears)
  - scroll via Bubble Tea viewport
- Event log UX:
  - filter (`/`), clear filter (`ctrl+l`), and clear events (`c`)
- Actions via Watermill:
  - Defined `devctl.ui.actions` topic + `tui.action.request` envelope
  - Wired dashboard actions: `u` up, `d` down (confirm), `r` restart (confirm)
  - Implemented an in-process action runner that reuses `pkg/config`, `pkg/discovery`, `pkg/runtime`, `pkg/engine`, `pkg/supervise`, `pkg/state`
  - Published `action.log` domain events so actions show up as UI event lines
- Log hygiene:
  - Disabled zerolog output while `devctl tui` runs by default; added `--debug-logs` escape hatch
- Validation:
  - `cd devctl && go test ./...`
  - Re-ran the tmux playbook patterns to confirm logs render and actions show up in the event view

### Why
- The fastest path to a “real” devctl TUI is to make logs + state navigation solid first; everything else (pipeline UX, plugin streams) is easier once those basics are reliable.
- Routing UI actions through Watermill keeps the concurrency model simple: the UI emits requests, the runner emits domain events, and models only ever react to messages.

### What worked
- Logs are readable and controllable (tab/follow/filter/scroll) without leaving the TUI.
- The `up/down/restart` path works end-to-end and produces visible action events in the event log.
- Silencing zerolog prevents background library logs from corrupting the terminal UI.

### What didn't work
- Adding `bubbles/textinput` initially failed at runtime tooling level with a missing `go.sum` entry (`github.com/atotto/clipboard`); running `go get github.com/charmbracelet/bubbles/textinput@...` and `go mod tidy` fixed it.
- With viewport “high performance rendering” enabled, tmux capture output was sometimes blank; switching it off makes `tmux capture-pane` reflect what we expect.

### What I learned
- For this repo’s workflow, tmux capture is part of the “test surface”, so rendering modes that look fine interactively can still be problematic.
- It’s valuable to keep “action logs” as domain events: they’re debuggable, searchable, and don’t require special UI plumbing beyond the existing transformer.

### What was tricky to build
- Avoiding subtle state bugs: the service lookup originally returned a pointer to a loop-local copy of the service record, which can silently produce wrong data; returning a pointer to the slice element fixes it.
- Keeping the UI clean while running operations that touch runtime/supervise code paths that normally log to stdout/stderr.

### What warrants a second pair of eyes
- The in-process “up” path currently mirrors the CLI default behavior (build + prepare + validate + launch); we should confirm this matches how people expect `u` to behave (especially for repos where build/prepare are expensive).
- Whether we want a more structured action progress stream (phase start/end) rather than only “start/ok/failed” log lines.

### What should be done in the future
- Add PipelineModel progress messages (phase/step events) and a proper validation table view.
- Consider an opt-in “skip build/prepare/validate” set of keys or toggles for faster iteration during development.

### Code review instructions
- Start at `devctl/cmd/devctl/cmds/tui.go` for wiring (`--debug-logs`, action runner registration).
- Review the action runner in `devctl/pkg/tui/action_runner.go` and message types in `devctl/pkg/tui/actions.go`.
- Review the log viewer in `devctl/pkg/tui/models/service_model.go`.
- Validate with:
  - `cd devctl && go test ./...`
  - `docmgr doctor --ticket MO-006-DEVCTL-TUI`
  - The tmux playbook in `devctl/ttmp/2026/01/06/MO-006-DEVCTL-TUI--create-a-devctl-tui/playbook/01-playbook-testing-devctl-tui-in-tmux.md`

### Technical details
- UI actions topic: `devctl.ui.actions` (`devctl/pkg/tui/topics.go`)
- Action request envelope: `{type:\"tui.action.request\", payload:{kind,at}}`

## Step 16: Make restart failures obvious (CLI prompt + TUI status line)

After trying a “kill → restart” loop in the TUI, it was too easy to end up in a confusing state: `up` would be requested while `state.json` still existed, the operation would fail, and the failure was only visible if you happened to be watching the event log. The CLI had the same underlying sharp edge (`devctl up` errors if state exists), which made it hard to tell whether you should run `down`, use `--force`, or just clean up a stale state.

This step tightens the ergonomics in two places: the TUI now keeps a small persistent status line for action results (so “action failed …” is hard to miss), and the CLI `devctl up` prompts interactively when state exists. If the state looks stale (no PIDs alive), it offers to remove it and continue; otherwise it offers a restart (down then up). Non-interactive runs keep the old behavior and require `--force`.

**Commit (code):** 8677065 — "devctl up: prompt on existing state; tui: clearer errors"

### What I did
- TUI:
  - Added a persistent `Status:` line for action failures/successes
  - Changed `u` on the dashboard to prompt for restart if state already exists
- CLI:
  - Updated `devctl up` to prompt when state exists (TTY only), using:
    - “remove stale state and continue?” if all PIDs are dead
    - “restart (down then up)?” if any PID is alive
  - Printed the prompt on stderr so it still shows up if stdout is redirected
- Ran `cd devctl && go test ./...`

### Why
- A failed restart is a “high-salience” event in the dev loop; hiding it in a scrolling event tab is a poor default.
- Prompting on existing state is a safer default than forcing users to remember `down`/`--force`, especially when state can become stale after crashes.

### What worked
- The TUI now makes action failures/successes visible immediately without leaving the dashboard/service views.
- In an interactive terminal, `devctl up` now guides the user to the right next action instead of just erroring.

### What didn't work
- When I tried to test the CLI prompt by piping input (`printf "y\\n" | devctl up ...`), it intentionally did not prompt because stdin is not a TTY; it returned the old error. This is expected, and `--force` remains the automation-friendly path.

### What I learned
- “Interactive prompt only when stdin is a TTY” is the right compromise: it avoids hangs in CI/scripts while improving the human CLI.

### What was tricky to build
- Keeping the behavior consistent across three entry points:
  - CLI `up` (interactive prompt)
  - TUI `u` (restart prompt)
  - TUI `d/r` (already-confirmed destructive actions)

### What warrants a second pair of eyes
- Whether we should add an explicit `--yes` flag for `devctl up` to accept the prompt in scripted environments without needing `--force`.

### What should be done in the future
- Consider surfacing richer action errors (e.g., validation failure details) in the TUI beyond the single-line status/event log.

### Code review instructions
- Review `devctl/cmd/devctl/cmds/up.go` for the prompt logic and stale-vs-running distinction.
- Review `devctl/pkg/tui/models/root_model.go` and `devctl/pkg/tui/models/dashboard_model.go` for the new UX behavior.
- Validate with `cd devctl && go test ./...` and by running `devctl up` twice in an interactive terminal to see the prompt.

### Technical details
- TTY detection is based on checking `os.ModeCharDevice` of stdin (TTY-only prompts).
