---
Title: Diary
Ticket: MO-010-DEVCTL-CLEANUP-PASS
Status: active
Topics:
    - backend
    - tui
    - refactor
    - ui-components
DocType: reference
Intent: long-term
Owners: []
RelatedFiles:
    - Path: cmd/devctl/cmds/dynamic_commands.go
      Note: |-
        Unconditional commands.list and command.run calls that can stall startup
        Skip discovery for __wrap-service to prevent wrapper readiness failures
    - Path: cmd/devctl/cmds/dynamic_commands_test.go
      Note: Unit test for wrapper discovery skip
    - Path: cmd/devctl/cmds/wrap_service.go
      Note: |-
        Wrapper command implementation that starts child
        Setpgid to allow safe process-group wiring when invoked directly
    - Path: cmd/devctl/main.go
      Note: Shows dynamic plugin command discovery runs before every Cobra command (including __wrap-service)
    - Path: pkg/supervise/supervisor.go
      Note: Supervisor start/stop
    - Path: ttmp/2026/01/06/MO-009-TUI-COMPLETE-FEATURES--complete-tui-features-per-mo-006-design/analysis/03-devctl-wrapper-startup-failure-in-comprehensive-fixture.md
      Note: Prior root-cause analysis of wrapper startup timing failure
    - Path: ttmp/2026/01/07/MO-010-DEVCTL-CLEANUP-PASS--devctl-cleanup-pass/design-doc/01-safe-plugin-invocation-and-capability-enforcement-long-term-pattern.md
      Note: Design doc produced from MO-009 review and codebase audit
    - Path: ttmp/2026/01/07/MO-010-DEVCTL-CLEANUP-PASS--devctl-cleanup-pass/reference/02-runtime-client-plugin-protocol-ops-streams-and-commands.md
      Note: New textbook reference on runtime client and plugin interaction
ExternalSources: []
Summary: ""
LastUpdated: 2026-01-07T13:47:34.626648784-05:00
WhatFor: ""
WhenToUse: ""
---





# Diary

## Goal

Capture the investigation + documentation work for MO-010: how `devctl` supervises services (startup/stop/readiness/logging), what supervision-related events exist, how the TUI consumes them, and how the comprehensive fixture fails during `devctl up`.

## Step 1: Source Review and System Map

This step built a concrete mental model of “service supervision” as implemented today, including the wrapper process, state/log files, and the TUI’s polling/event pipeline. The intent was to gather enough precise, code-referenced facts to write an exhaustive analysis document (and to know where a fixture failure would likely originate).

The most important outcome was identifying the startup ordering constraint: `devctl/cmd/devctl/main.go` always executes dynamic plugin command discovery before Cobra runs *any* subcommand, including the hidden wrapper command `__wrap-service`. That means a slow/blocked plugin can delay wrapper execution long enough for `supervise.Supervisor`’s wrapper “ready file” deadline to expire, producing the `wrapper did not report child start` failure.

**Commit (code):** N/A

### What I did
- Read MO-009 docs that already map the wrapper start failure and capability checking (`devctl/ttmp/2026/01/06/MO-009-TUI-COMPLETE-FEATURES--complete-tui-features-per-mo-006-design/analysis/05-capability-checking-and-safe-plugin-invocation-ops-commands-streams.md`, `devctl/ttmp/2026/01/06/MO-009-TUI-COMPLETE-FEATURES--complete-tui-features-per-mo-006-design/analysis/03-devctl-wrapper-startup-failure-in-comprehensive-fixture.md`, `devctl/ttmp/2026/01/06/MO-009-TUI-COMPLETE-FEATURES--complete-tui-features-per-mo-006-design/reference/02-service-launching-and-wrapper-mechanics-devctl-wrap-service.md`).
- Located the primary supervision code paths and tests (`devctl/pkg/supervise/supervisor.go`, `devctl/cmd/devctl/cmds/wrap_service.go`, `devctl/pkg/supervise/supervisor_test.go`).
- Located the primary UI integration points (`devctl/pkg/tui/action_runner.go`, `devctl/pkg/tui/state_watcher.go`, `devctl/pkg/tui/transform.go`, `devctl/pkg/tui/models/root_model.go`, `devctl/pkg/tui/models/dashboard_model.go`, `devctl/pkg/tui/models/service_model.go`).
- Captured the relevant dynamic-command startup behavior (`devctl/cmd/devctl/main.go`, `devctl/cmd/devctl/cmds/dynamic_commands.go`).

### Why
- The MO-010 analysis doc needs to be “textbook style” and code-referential: a reader should be able to jump from prose → symbol/file and verify behavior.
- A bug report is only useful if it precisely names the ordering/timeouts and demonstrates repro.

### What worked
- `docmgr doc search` was effective for quickly locating prior analyses/diaries that already discussed the wrapper and supervision.
- `rg`/`sed` across `devctl/` made it straightforward to map: pipeline → supervise → state file → state watcher → domain events → UI messages → BubbleTea models.

### What didn't work
- N/A (no execution repro attempted yet in this step).

### What I learned
- Service supervision “events” are not emitted directly by `supervise.Supervisor`; instead, the TUI gets supervision state via `state.json` polling (`StateWatcher`) and an inferred exit event when a previously-alive PID becomes not-alive.
- Wrapper mode introduces an extra PID indirection: `state.ServiceRecord.PID` stores the wrapper PID (used as the kill handle), while the real child PID only appears in the `*.ready` file and `*.exit.json`.
- The TUI also has a separate, direct “kill” path (`syscall.Kill(pid, SIGTERM)` from the dashboard) that bypasses the supervisor’s process-group termination semantics.

### What was tricky to build
- There are effectively two control planes: CLI commands (`devctl up/down/status/logs`) and the TUI “actions bus” pipeline. Both ultimately use `pkg/supervise`, but they differ in how they surface progress/errors (stdout vs event log + pipeline events).
- The wrapper being implemented as a Cobra subcommand inside the same binary means *any* global CLI initialization (including dynamic plugin command discovery) runs before the wrapper can even open service log files.

### What warrants a second pair of eyes
- Process-group correctness: wrapper and supervisor both rely on `Setpgid`/`Pgid` and group-kill via negative PID; subtle OS differences or zombie detection (`/proc/$pid/stat`) could cause false “alive”/“dead”.
- UI action semantics: `ActionStop` and per-service `ActionRestart` exist in message schemas but are not handled in `RegisterUIActionRunner` today; the dashboard kill path bypasses `terminatePIDGroup`.
- Timeout layering: health readiness uses `Options.ReadyTimeout`, but wrapper readiness uses a hard-coded 2s deadline; dynamic command discovery uses a 3s per-plugin timeout.

### What should be done in the future
- N/A (this ticket is currently documentation + bug reporting; improvements will be enumerated in the analysis doc’s “code review” section).

### Code review instructions
- Start at `devctl/pkg/supervise/supervisor.go` (`Start`, `startService`, `terminatePIDGroup`) and `devctl/cmd/devctl/cmds/wrap_service.go` (`__wrap-service` implementation).
- Then read `devctl/cmd/devctl/main.go` and `devctl/cmd/devctl/cmds/dynamic_commands.go` to understand why the wrapper is delayed.
- For UI integration, follow `devctl/pkg/tui/state_watcher.go` → `devctl/pkg/tui/transform.go` → `devctl/pkg/tui/models/root_model.go`.

### Technical details
- Commands executed (selection):
  - `docmgr doc list --ticket MO-009-TUI-COMPLETE-FEATURES`
  - `docmgr doc search --query "supervise"`
  - `rg -n "type Supervisor|startService|terminatePIDGroup|__wrap-service" devctl -S`
  - `sed -n '1,240p' devctl/pkg/supervise/supervisor.go`

## Step 2: MO-010 Ticket and Documents Created

This step created a new documentation workspace for MO-010 so the supervision analysis, a bug report, and an ongoing diary can live in one place. Keeping these artifacts in `devctl/ttmp` makes it easy to link to exact code symbols and preserve reproduction logs.

It also set up a dedicated bug report doc so that if the comprehensive fixture failure reproduces, we can record the exact command line, stdout/stderr, repo fixture path, and relevant `.devctl/logs/*` artifacts without mixing that into the architecture write-up.

**Commit (code):** N/A

### What I did
- Created the ticket workspace and initial documents:
  - `docmgr ticket create-ticket --ticket MO-010-DEVCTL-CLEANUP-PASS --title "devctl cleanup pass" --topics backend,tui,refactor,ui-components`
  - `docmgr doc add --ticket MO-010-DEVCTL-CLEANUP-PASS --doc-type reference --title "Diary"`
  - `docmgr doc add --ticket MO-010-DEVCTL-CLEANUP-PASS --doc-type analysis --title "Service supervision: architecture, events, and UI integration"`
  - `docmgr doc add --ticket MO-010-DEVCTL-CLEANUP-PASS --doc-type analysis --title "Comprehensive fixture: devctl up failure (bug report)"`

### Why
- The user request is explicitly to create a new ticket and produce exhaustive documentation (plus a bug report if repro succeeds).

### What worked
- `docmgr` created a consistent ticket layout (`index.md`, `tasks.md`, `changelog.md`) and stable doc paths for future linking.

### What didn't work
- N/A.

### What I learned
- Ticket topics in this workspace are constrained by the current vocabulary; I used existing slugs (`backend`, `tui`, `refactor`, `ui-components`) to avoid adding new vocabulary entries mid-task.

### What was tricky to build
- N/A (pure doc scaffolding).

### What warrants a second pair of eyes
- N/A.

### What should be done in the future
- Add file relations once the analysis doc and bug report are populated, so `RelatedFiles` stays tight and meaningful.

### Code review instructions
- N/A (no code changes in this step).

### Technical details
- Created paths:
  - `devctl/ttmp/2026/01/07/MO-010-DEVCTL-CLEANUP-PASS--devctl-cleanup-pass/reference/01-diary.md`
  - `devctl/ttmp/2026/01/07/MO-010-DEVCTL-CLEANUP-PASS--devctl-cleanup-pass/analysis/01-service-supervision-architecture-events-and-ui-integration.md`
  - `devctl/ttmp/2026/01/07/MO-010-DEVCTL-CLEANUP-PASS--devctl-cleanup-pass/analysis/02-comprehensive-fixture-devctl-up-failure-bug-report.md`

## Step 3: Wrote Service Supervision Analysis (and Fixed Frontmatter)

This step produced the main “textbook style” analysis document for service supervision, including the process model (PIDs/PGIDs), filesystem artifacts, startup/stop sequences, readiness semantics, and the TUI’s observation/control event flow. The intent was to make the system reviewable: a reader can trace any claim to a file/symbol and understand where UX behavior comes from.

While relating files, `docmgr` rejected the analysis doc due to invalid `ExternalSources` frontmatter shape (it expects a list of strings, not a list of `{Path,Note}` objects). I removed the structured `ExternalSources` block and kept cross-document references inside the body text instead, so doc tooling remains happy.

**Commit (code):** N/A

### What I did
- Authored `devctl/ttmp/2026/01/07/MO-010-DEVCTL-CLEANUP-PASS--devctl-cleanup-pass/analysis/01-service-supervision-architecture-events-and-ui-integration.md` with:
  - conceptual model + invariants,
  - API reference snippets,
  - two Mermaid sequence diagrams (wrapper-mode start; TUI observation/control planes),
  - a second “cleanup pass” section identifying risks and improvement targets.
- Fixed frontmatter schema error by changing `ExternalSources` back to `[]`.
- Related core implementation files to the analysis doc using:
  - `docmgr doc relate --doc devctl/ttmp/.../analysis/01-service-supervision-architecture-events-and-ui-integration.md --file-note ...`

### Why
- The user request explicitly asked for an exhaustive, code-referenced “textbook style” description of supervision and UI interaction.
- Keeping frontmatter schema-valid ensures `docmgr` tooling (search/relate/validate) remains usable for future work.

### What worked
- A single analysis doc can explain both CLI supervision and TUI supervision UX once the “state.json is the UI API” model is made explicit.
- `docmgr` frontmatter validation caught a real schema mismatch early (before related-files metadata drifted).

### What didn't work
- `docmgr doc relate` initially failed with:
  - `document has invalid frontmatter ... cannot unmarshal !!map into string`

### What I learned
- `ExternalSources` is schema-validated differently than `RelatedFiles` in this workspace; it should remain a simple list (or be left empty), while `RelatedFiles` supports `{Path,Note}` objects.

### What was tricky to build
- Documenting supervision required crossing multiple packages (`supervise`, `state`, `tui`) and correctly describing which layer emits which “event” (most are inferred/published outside the supervisor).

### What warrants a second pair of eyes
- The “code review / cleanup pass” section calls out real behavioral inconsistencies (UI actions, timeout layering, kill semantics) that would affect users if changed; these deserve careful review before implementing fixes.

### What should be done in the future
- N/A (execution repro + bug report are the next steps in this ticket).

### Code review instructions
- Read the analysis doc first for a map, then jump into:
  - `devctl/pkg/supervise/supervisor.go`
  - `devctl/cmd/devctl/cmds/wrap_service.go`
  - `devctl/pkg/tui/state_watcher.go`
  - `devctl/pkg/tui/action_runner.go`
  - `devctl/pkg/tui/transform.go`

### Technical details
- `docmgr doc relate` error (fixed by editing frontmatter):
  - `taxonomy: docmgr.frontmatter.parse/yaml_syntax ... cannot unmarshal !!map into string`

## Step 4: Reproduced Comprehensive Fixture `devctl up` Failure

This step executed the comprehensive fixture setup script and reproduced the `devctl up` failure in a clean, local fixture repo. The goal was to capture the exact repro commands and outputs so the bug report can be verified independently by anyone with the devctl repo.

The reproduction aligned with the previously documented root cause: wrapper mode uses the `devctl` binary as the wrapper (`__wrap-service`), and the wrapper binary runs dynamic plugin command discovery before it can execute the wrapper subcommand. In the fixture, the `logger` plugin never responds to `commands.list`, so the wrapper misses the supervisor’s 2-second ready-file deadline.

**Commit (code):** N/A

### What I did
- Ran the fixture setup script from the `devctl` module root:
  - `cd /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl`
  - `./ttmp/2026/01/06/MO-009-TUI-COMPLETE-FEATURES--complete-tui-features-per-mo-006-design/scripts/setup-comprehensive-fixture.sh`
- Ran `devctl up` against the generated repo root:
  - `go run ./cmd/devctl --repo-root "/tmp/devctl-comprehensive-iiof7p" up`
- Confirmed that the fixture `.devctl/logs/` directory existed but was empty after the failure (no `*.ready` / per-service logs).

### Why
- The user requested running the system “for real” and filing a bug report if the comprehensive fixture `up` failure reproduces.

### What worked
- The fixture script reliably produced a repo root and reproduced the failure without additional manual steps.

### What didn't work
- `devctl up` failed at the supervise phase with the exact error:
  - `Error: wrapper did not report child start`

### What I learned
- The failure mode leaves very few artifacts (empty `.devctl/logs`), which makes the error difficult to debug without understanding the wrapper startup ordering and timeouts.

### What was tricky to build
- Capturing a “verbatim” output trace is important here because the failure happens after a non-trivial pipeline run, and the supervise error itself is not timestamped.

### What warrants a second pair of eyes
- Confirm whether the best fix is (a) bypassing dynamic command discovery for internal commands, (b) capability-gating `commands.list`, (c) increasing/removing the hard-coded 2s deadline, or a combination.

### What should be done in the future
- N/A (bug report writing is the next step and is now complete).

### Code review instructions
- Review the bug report doc:
  - `devctl/ttmp/2026/01/07/MO-010-DEVCTL-CLEANUP-PASS--devctl-cleanup-pass/analysis/02-comprehensive-fixture-devctl-up-failure-bug-report.md`

### Technical details
- Fixture repo created:
  - `/tmp/devctl-comprehensive-iiof7p`
- Key commands and outcomes:
  - `go run ./cmd/devctl --repo-root "/tmp/devctl-comprehensive-iiof7p" up` → `wrapper did not report child start`

## Step 5: Uploaded Analysis PDFs to reMarkable

This step converted the two MO-010 analysis documents to PDFs and uploaded them to the reMarkable device using the local `remarkable_upload.py` helper. The goal was to preserve a readable, annotated-friendly copy on-device under a stable, ticket-specific folder.

I first attempted the “ticket-dir default documents” mode (no explicit md args), but the helper guessed a non-existent bug-report markdown path for this ticket and aborted. I then switched to explicitly passing the two markdown files while still using `--ticket-dir` for date inference and `--mirror-ticket-structure` to keep uploads organized under `ai/YYYY/MM/DD/<ticket>/analysis/`.

**Commit (code):** N/A

### What I did
- Confirmed tooling was present:
  - `python3 /home/manuel/.local/bin/remarkable_upload.py --help`
  - `command -v rmapi; command -v pandoc; command -v xelatex`
- Tried a dry-run using `--ticket-dir` with no explicit docs (failed; see below).
- Ran a dry-run with explicit markdown paths and ticket mirroring:
  - `python3 /home/manuel/.local/bin/remarkable_upload.py --dry-run --ticket-dir ... --mirror-ticket-structure --remote-ticket-root MO-010-DEVCTL-CLEANUP-PASS <doc1> <doc2>`
- Uploaded both docs (same command without `--dry-run`).

### Why
- The user requested uploading both documents to reMarkable, and mirroring keeps the folder structure stable and collision-free.

### What worked
- Upload succeeded for both PDFs:
  - `OK: uploaded 01-service-supervision-architecture-events-and-ui-integration.pdf -> ai/2026/01/07/MO-010-DEVCTL-CLEANUP-PASS/analysis/`
  - `OK: uploaded 02-comprehensive-fixture-devctl-up-failure-bug-report.pdf -> ai/2026/01/07/MO-010-DEVCTL-CLEANUP-PASS/analysis/`

### What didn't work
- The “ticket-dir default documents” dry-run failed because the helper expected a missing file:
  - `python3 /home/manuel/.local/bin/remarkable_upload.py --dry-run --ticket-dir /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/ttmp/2026/01/07/MO-010-DEVCTL-CLEANUP-PASS--devctl-cleanup-pass`
  - Error:
    - `ERROR: markdown file not found: /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/ttmp/2026/01/07/MO-010-DEVCTL-CLEANUP-PASS--devctl-cleanup-pass/reference/01-bug-report-doc-relate-fails-on-non-docmgr-markdown-files.md`

### What I learned
- For MO-010, passing explicit markdown paths is the safest approach; the helper’s “default bug report + analysis doc” heuristic does not match this ticket’s doc naming.

### What was tricky to build
- N/A (tooling usage), but the choice of flags matters for on-device organization (`--mirror-ticket-structure` + `--remote-ticket-root`).

### What warrants a second pair of eyes
- N/A.

### What should be done in the future
- If desired, adjust `remarkable_upload.py`’s “default docs for ticket-dir” heuristic to match docmgr-created analysis docs more robustly (outside the scope of this diary step).

### Code review instructions
- N/A (no code changes).

### Technical details
- Uploaded to:
  - `ai/2026/01/07/MO-010-DEVCTL-CLEANUP-PASS/analysis/`
- Source markdown files:
  - `/home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/ttmp/2026/01/07/MO-010-DEVCTL-CLEANUP-PASS--devctl-cleanup-pass/analysis/01-service-supervision-architecture-events-and-ui-integration.md`
  - `/home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/ttmp/2026/01/07/MO-010-DEVCTL-CLEANUP-PASS--devctl-cleanup-pass/analysis/02-comprehensive-fixture-devctl-up-failure-bug-report.md`

## Step 6: Capability-Checking Deep Review and Long-Term Pattern Design Doc

This step reviewed the MO-009 capability-checking analysis and re-audited the current devctl codebase for plugin invocation call sites, error handling, and timeout policy. The intent was to decide on a long-term pattern that prevents the same bug class from reappearing as devctl grows (especially once streaming ops get used in production).

The main conclusion is that “helpers are good but not sufficient”: the most robust approach is to make capability enforcement the default in `runtime.Client` (or a mandatory wrapper), so unsupported ops are rejected locally without sending a request. Then call sites can focus on the remaining hard parts: required vs optional semantics, deadline policy, and diagnostics.

**Commit (code):** N/A

### What I did
- Re-read MO-009’s recommendations section and mapped them onto current code realities (call sites, timeouts, stream usage).
- Scanned call sites for `Client.Call` and `StartStream`:
  - production `.Call`: pipeline + dynamic command discovery only
  - production `StartStream`: none yet (tests only)
- Authored a MO-010 design doc that:
  - inventories current call sites + gaps,
  - reviews MO-009 recommendations point-by-point,
  - proposes a safe-by-default runtime enforcement pattern,
  - proposes explicit required/optional wrappers and typed error mapping,
  - proposes command discovery hardening (gating, conditional execution, parallelism, short timeouts, preserve repo_root).

### Why
- The user asked for a “complete analysis of the codebase” around capability checking and a review of the MO-009 proposals, culminating in a long-term robust pattern.

### What worked
- The codebase is still small enough that a full call-site inventory is tractable and precise; this makes the proposed enforcement changes easy to reason about.

### What didn't work
- N/A (this step was analysis and design; no runtime changes implemented yet).

### What I learned
- `runtime.Client.Call` currently converts protocol errors into untyped `error` strings (e.g. `"E_UNSUPPORTED: ..."`) which makes it hard to treat unsupported as a first-class, non-fatal condition without brittle string parsing.
- Dynamic command discovery has an extra, separate robustness issue: it uses `context.Background()` so `request.ctx.repo_root` is empty during `commands.list` (plugins may need repo context even for discovery).

### What was tricky to build
- Designing a pattern that is both robust and realistically adoptable: enforcing at the runtime layer prevents regressions, but still needs a clean “required vs optional” API and typed errors so call sites stay simple.

### What warrants a second pair of eyes
- Whether to gate streams on `capabilities.ops` only, or require both `ops` and `streams` to contain the stream op name.
- Whether runtime should enforce “ctx must have a deadline” (strict) or silently apply defaults when missing (permissive).

### What should be done in the future
- Implement the recommended runtime enforcement + typed error mapping + dynamic discovery changes (separate implementation ticket/PR).

### Code review instructions
- Read the design doc first:
  - `devctl/ttmp/2026/01/07/MO-010-DEVCTL-CLEANUP-PASS--devctl-cleanup-pass/design-doc/01-safe-plugin-invocation-and-capability-enforcement-long-term-pattern.md`
- Then cross-check these files as you read:
  - `devctl/pkg/runtime/client.go`
  - `devctl/cmd/devctl/cmds/dynamic_commands.go`
  - `devctl/pkg/engine/pipeline.go`

### Technical details
- Related source doc (MO-009):
  - `devctl/ttmp/2026/01/06/MO-009-TUI-COMPLETE-FEATURES--complete-tui-features-per-mo-006-design/analysis/05-capability-checking-and-safe-plugin-invocation-ops-commands-streams.md`

## Step 7: Uploaded Capability-Checking Design Doc to reMarkable

This step converted the new MO-010 design doc to PDF and uploaded it to the reMarkable device, mirroring the ticket structure so it lands under `ai/YYYY/MM/DD/MO-010-DEVCTL-CLEANUP-PASS/design-doc/`.

Pandoc/xelatex emitted warnings about missing glyphs for the “✅” character in DejaVu Sans; the upload still succeeded. If the PDF rendering on-device shows blank boxes where those checkmarks are used, we should either replace them with ASCII markers in the markdown and re-upload with `--force`, or switch the PDF font set to one that includes the glyph.

**Commit (code):** N/A

### What I did
- Dry-run:
  - `python3 /home/manuel/.local/bin/remarkable_upload.py --dry-run --ticket-dir ... --mirror-ticket-structure --remote-ticket-root MO-010-DEVCTL-CLEANUP-PASS <design-doc.md>`
- Upload:
  - `python3 /home/manuel/.local/bin/remarkable_upload.py --ticket-dir ... --mirror-ticket-structure --remote-ticket-root MO-010-DEVCTL-CLEANUP-PASS <design-doc.md>`

### Why
- The user requested uploading the new design doc to reMarkable.

### What worked
- Upload succeeded:
  - `OK: uploaded 01-safe-plugin-invocation-and-capability-enforcement-long-term-pattern.pdf -> ai/2026/01/07/MO-010-DEVCTL-CLEANUP-PASS/design-doc/`

### What didn't work
- PDF generation warnings (xelatex / DejaVu Sans):
  - `[WARNING] Missing character: There is no ✅ (U+2705) ...`

### What I learned
- The current upload pipeline uses DejaVu Sans; some emoji/glyphs won’t render. For “long-term docs meant for PDF”, prefer ASCII markers.

### What was tricky to build
- N/A.

### What warrants a second pair of eyes
- Whether we should standardize “PDF-safe markdown” conventions (no emoji) in ticket docs intended for reMarkable export.

### What should be done in the future
- If readability is impacted, replace “✅” markers in the design doc with ASCII and re-upload with `--force` (only if explicitly requested).

### Code review instructions
- N/A.

### Technical details
- Uploaded to:
  - `ai/2026/01/07/MO-010-DEVCTL-CLEANUP-PASS/design-doc/01-safe-plugin-invocation-and-capability-enforcement-long-term-pattern.pdf`

## Step 8: Wrote a Runtime Client + Streams + Commands “Textbook” Reference (and Prepared for reMarkable)

This step created a deep, backend-focused reference document that explains how devctl interacts with plugins via the runtime layer: protocol frames, the factory/client/router internals, streaming semantics, and the dynamic CLI command mechanism (`commands.list`/`command.run`). The intent is to have a single onboarding chapter for the “plugin backend” so future changes (especially around streaming) don’t regress into hard-to-debug startup hangs.

The doc is written to be PDF-friendly (no emoji glyphs) and includes Mermaid diagrams; if Mermaid doesn’t render in pandoc output, the text still stands on its own.

**Commit (code):** N/A

### What I did
- Audited runtime implementation files (`devctl/pkg/runtime/*`) and protocol schemas (`devctl/pkg/protocol/*`).
- Mapped “who uses what” across pipeline, dynamic commands, and the TUI action runner.
- Wrote the new reference doc:
  - `devctl/ttmp/2026/01/07/MO-010-DEVCTL-CLEANUP-PASS--devctl-cleanup-pass/reference/02-runtime-client-plugin-protocol-ops-streams-and-commands.md`
- Related the core runtime/protocol/consumer files to the doc using `docmgr doc relate`.

### Why
- The user asked for a “textbook doc” about the runtime client, streams, and commands, focusing on the backend interaction layer with plugins.

### What worked
- The current codebase is compact enough that a complete, code-referenced inventory is feasible and stays accurate.

### What didn't work
- N/A.

### What I learned
- `commands.list` discovery currently uses `context.Background()` which drops `repo_root` from request ctx; this can surprise plugins that want repo context even for discovery.
- Streams are implemented and tested in runtime, but not yet used in production code; documenting the semantics now makes future TUI log-follow work safer.

### What was tricky to build
- Explaining the router buffering semantics clearly: events can arrive before a subscriber subscribes, so the router buffers by stream_id and flushes on subscribe.

### What warrants a second pair of eyes
- PDF rendering of Mermaid diagrams: if they don’t render cleanly, we may want to also include ASCII diagrams or keep diagrams as prose-only.

### What should be done in the future
- Upload the doc to reMarkable (next step).

### Code review instructions
- Start with the new reference doc, then jump into:
  - `devctl/pkg/runtime/client.go`
  - `devctl/pkg/runtime/router.go`
  - `devctl/pkg/runtime/factory.go`
  - `devctl/cmd/devctl/cmds/dynamic_commands.go`

### Technical details
- Document path:
  - `devctl/ttmp/2026/01/07/MO-010-DEVCTL-CLEANUP-PASS--devctl-cleanup-pass/reference/02-runtime-client-plugin-protocol-ops-streams-and-commands.md`

## Step 9: Uploaded Runtime Client Reference to reMarkable

This step converted the new runtime-client textbook reference to PDF and uploaded it to reMarkable under the MO-010 ticket folder, mirroring the ticket structure. This provides an on-device copy for annotation and review.

**Commit (code):** N/A

### What I did
- Dry-run:
  - `python3 /home/manuel/.local/bin/remarkable_upload.py --dry-run --ticket-dir ... --mirror-ticket-structure --remote-ticket-root MO-010-DEVCTL-CLEANUP-PASS <reference.md>`
- Upload:
  - `python3 /home/manuel/.local/bin/remarkable_upload.py --ticket-dir ... --mirror-ticket-structure --remote-ticket-root MO-010-DEVCTL-CLEANUP-PASS <reference.md>`

### Why
- The user requested storing the doc in the ticket and uploading it to reMarkable.

### What worked
- Upload succeeded:
  - `OK: uploaded 02-runtime-client-plugin-protocol-ops-streams-and-commands.pdf -> ai/2026/01/07/MO-010-DEVCTL-CLEANUP-PASS/reference/`

### What didn't work
- N/A.

### What I learned
- Unlike the earlier design doc, this reference contained no emoji glyphs, so PDF generation was warning-free.

### What was tricky to build
- N/A.

### What warrants a second pair of eyes
- N/A.

### What should be done in the future
- N/A.

### Code review instructions
- N/A.

### Technical details
- Uploaded to:
  - `ai/2026/01/07/MO-010-DEVCTL-CLEANUP-PASS/reference/02-runtime-client-plugin-protocol-ops-streams-and-commands.pdf`

## Step 10: Added Appendix on Explicit Repo/Plugin Context (Repository + Meta)

This step updated the capability-enforcement design doc with an appendix that explores a more structural fix: stop passing repo-root and other request metadata via `context.Context` values, and instead make it explicit via a `RequestMeta` and a `Repository`/`RuntimeEnv` style container. The goal is to remove an entire class of “ambient context” bugs (like dynamic command discovery using `context.Background()` and dropping `repo_root`).

The appendix also clarifies what `runtime.Client` represents (a handle to one running plugin process), why devctl uses multiple clients (multiple plugins, stacked/merged in priority order), and how request context should be built deterministically from meta + deadline.

**Commit (code):** N/A

### What I did
- Added `Appendix A` to:
  - `devctl/ttmp/2026/01/07/MO-010-DEVCTL-CLEANUP-PASS--devctl-cleanup-pass/design-doc/01-safe-plugin-invocation-and-capability-enforcement-long-term-pattern.md`
- Covered:
  - why `context.Value` is fragile for repo-root,
  - what `Client` is and why there are multiple,
  - what “Meta” should be,
  - how to build `protocol.RequestContext` from Meta + `ctx.Deadline()`,
  - a unifying `Repository`/`RuntimeEnv` pattern that naturally supports “single discovery pass”.

### Why
- The user requested an explicit write-up of patterns that avoid passing repo/plugin data through context and asked whether a Repository struct can unify the model.

### What worked
- The appendix ties directly into the earlier capability-enforcement direction: both changes (capability enforcement + explicit meta) reduce “ambient” failure modes.

### What didn't work
- N/A.

### What I learned
- The current protocol already has a natural “meta” surface (`protocol.RequestContext`); the main missing piece is making its inputs explicit at call sites / client construction time.

### What was tricky to build
- Keeping the appendix concrete without committing to a breaking API change: it outlines multiple implementation shapes (meta-on-client vs meta-per-call) and how they interact with existing code.

### What warrants a second pair of eyes
- Where to store Meta in practice:
  - in the runtime client wrapper,
  - or in a higher-level Repository/Session object that constructs clients.

### What should be done in the future
- If desired, re-upload the updated design doc PDF to reMarkable (would require `--force` to overwrite the existing PDF).

### Code review instructions
- Start at Appendix A in:
  - `devctl/ttmp/2026/01/07/MO-010-DEVCTL-CLEANUP-PASS--devctl-cleanup-pass/design-doc/01-safe-plugin-invocation-and-capability-enforcement-long-term-pattern.md`

## Step 11: Drafted a “No-Compat” Protocol v2 Design (Handshake Commands + Repository + Capability-Enforced Call)

This step produced a clean, breaking-change design document that formalizes three related simplifications: (1) eliminate `commands.list` and move structured command specs into the handshake, (2) introduce a `Repository` struct as the explicit carrier of repo/plugin context, and (3) make `Client.Call` fail fast if an op is not declared in capabilities.

It intentionally does not include backwards compatibility. The aim is to reduce complexity and to eliminate an entire class of startup stalls (especially those caused by command discovery running before every command).

**Commit (code):** N/A

### What I did
- Added a new design doc:
  - `devctl/ttmp/2026/01/07/MO-010-DEVCTL-CLEANUP-PASS--devctl-cleanup-pass/design-doc/02-protocol-v2-handshake-command-specs-repository-context-capability-enforced-calls.md`
- Documented:
  - protocol v2 handshake schema change (`capabilities.commands` becomes structured `[]CommandSpec`)
  - removal of `commands.list` as an op
  - Repository + RequestMeta container (explicit repo_root/cwd/dry_run, one bootstrap pass)
  - runtime behavior change: unsupported ops return `E_UNSUPPORTED` locally (so a dedicated invocation helper is not required)

### Why
- The user requested a “clean design” that removes `commands.list`, makes context explicit via a Repository struct, and enforces capabilities in `Client.Call` with no compatibility burden.

### What worked
- The design aligns well with the previously observed failure modes:
  - command discovery stalls disappear if discovery is handshake-only,
  - repo_root cannot be dropped if it is explicit meta,
  - forgotten gating cannot cause hangs if `Call` rejects unsupported ops.

### What didn't work
- N/A (doc-only).

### What I learned
- Even with handshake-only command discovery, we still must start each plugin to read handshake; the protocol change removes the extra round-trip, not the process-start cost. The design doc calls this out explicitly.

### What was tricky to build
- Keeping the proposal “no-compat” while still sketching a realistic implementation plan (protocol version bump, plugin updates, test updates).

### What warrants a second pair of eyes
- Whether to keep `command.run` vs making each command a per-op endpoint (simpler execution surface vs more ops in capabilities).
- Whether Repository should own long-lived plugin processes (pool) vs the current “start per run” approach.

### What should be done in the future
- N/A (this is a design proposal; implementation would be a follow-on ticket/PR).

### Code review instructions
- Read:
  - `devctl/ttmp/2026/01/07/MO-010-DEVCTL-CLEANUP-PASS--devctl-cleanup-pass/design-doc/02-protocol-v2-handshake-command-specs-repository-context-capability-enforced-calls.md`

## Step 8: Textbook Walkthrough — Fundamentals → Domain Events → UI Messages

This step focused specifically on the TUI event pipeline: how “devctl fundamentals” (plugins, pipeline actions, supervision, persisted state) are reflected as domain events, then transformed into UI messages, and finally consumed as BubbleTea `tea.Msg` values by the UI models. The goal was to produce an exhaustive, code-referential explanation that makes boundary leaks and robustness seams obvious.

The most important outcome was surfacing several concrete “design fray” points (UI-owned side effects, stringly-typed couplings, high-frequency polling translated into log spam, and partially-implemented message plumbing). These are the exact kinds of issues that make a system feel non-robust over time because the UI starts to encode real behavior and the “event contracts” become implicit rather than explicit.

**Commit (code):** N/A

### What I did
- Audited the wiring in the TUI entrypoint:
  - `sed -n '1,220p' devctl/cmd/devctl/cmds/tui.go`
- Traced the in-memory bus and topic split (domain events vs UI messages vs UI actions):
  - `sed -n '1,240p' devctl/pkg/tui/bus.go`
  - `sed -n '1,240p' devctl/pkg/tui/topics.go`
- Traced the three major translation boundaries:
  - fundamentals → domain events: `devctl/pkg/tui/state_watcher.go`, `devctl/pkg/tui/action_runner.go`
  - domain events → UI messages: `devctl/pkg/tui/transform.go`
  - UI messages → BubbleTea messages: `devctl/pkg/tui/forward.go`, `devctl/pkg/tui/msgs.go`
- Verified where BubbleTea consumes the messages:
  - `devctl/pkg/tui/models/root_model.go`
  - `devctl/pkg/tui/models/dashboard_model.go`
  - `devctl/pkg/tui/models/pipeline_model.go`
  - `devctl/pkg/tui/models/service_model.go`
- Created the textbook-style analysis doc in the ticket:
  - `docmgr doc add --ticket MO-010-DEVCTL-CLEANUP-PASS --doc-type analysis --title "Devctl fundamentals → events → UI messages (textbook walkthrough)"`
- Related the key code files to the new analysis doc:
  - `docmgr doc relate --doc devctl/ttmp/2026/.../analysis/03-...md --file-note "/abs/path:reason" ...`
- Added a ticket changelog entry for the new analysis:
  - `docmgr changelog update --ticket MO-010-DEVCTL-CLEANUP-PASS --entry "Added textbook walkthrough of fundamentals→events→UI messages ..." ...`

### Why
- The user explicitly wants to learn the system well enough to see where the current design starts to fray (layering violations, implicit contracts, UI-hidden functionality, and brittle couplings).
- A precise “event pipeline” understanding is a prerequisite for making safe refactors in this space.

### What worked
- `rg` across `devctl/pkg/tui` quickly located the full pipeline (publishers, transformer, forwarder, consumers):
  - `rg -n "TopicDevctlEvents|TopicUIMessages|RegisterDomainToUITransformer|RegisterUIForwarder|RegisterUIActionRunner" devctl/pkg/tui -S`
- Reading “entrypoint → bus → handlers → models” in that order kept the mental model coherent and prevented chasing details prematurely.

### What didn't work
- A fat-fingered `ls` option caused a confusing error while enumerating the tree:
  - `ls -لا devctl`
  - `ls: invalid option -- 'á'`
- A couple of “expected but missing” files were referenced in commands and resulted in `sed` errors:
  - `sed -n '1,260p' devctl/pkg/runtime/plugin.go` → `sed: can't read devctl/pkg/runtime/plugin.go: No such file or directory`
  - `sed -n '1,260p' devctl/pkg/state/io.go` → `sed: can't read devctl/pkg/state/io.go: No such file or directory`
- Initially tried relating files to the new analysis doc using a docs-root-relative `--doc` value; `docmgr` didn’t match it:
  - `docmgr doc relate --doc 2026/.../analysis/03-...md ...` → `expected exactly 1 doc ... got 0`
  - Fixed by passing the filesystem path: `--doc devctl/ttmp/2026/.../analysis/03-...md`
- Similarly, `docmgr validate frontmatter` expects paths relative to the docs root; passing a filesystem path duplicated the root:
  - `docmgr validate frontmatter --doc devctl/ttmp/2026/...` → `open .../devctl/ttmp/devctl/ttmp/2026/...: no such file or directory`
  - Fixed by using docs-root-relative paths: `--doc 2026/.../analysis/03-...md`

### What I learned
- The current “domain event stream” is largely produced from within `devctl/pkg/tui/` itself (not from a UI-agnostic domain package), which makes the boundary easy to blur.
- Several UI features bypass or mismatch the event pipeline:
  - Dashboard “kill” uses `syscall.Kill` directly (UI-owned side effect).
  - `ActionStop` exists in UI models but is not implemented by the action runner, so the UI can emit an action that will always fail.
- The transformer emits a log entry for every state snapshot (“state: loaded/missing/error”), which can flood the event log because snapshots are emitted on a timer.
- `PipelineModel` is ready for live output/config patches/progress messages, but the bus/transformer/forwarder/action-runner do not currently emit or forward those events.

### What was tricky to build
- Keeping the explanation “textbook-like” without losing fidelity required treating the event pipeline as an API contract: enumerating each message type, its producer/consumer, and its invariants (ordering, correlation keys, failure cases).

### What warrants a second pair of eyes
- Whether the most problematic seams (UI kill path, snapshot-log spam, string parsing for status line, ActionStop mismatch) should be addressed immediately as part of MO-010, or deferred behind a larger refactor that relocates event definitions/publishers out of `pkg/tui/`.

### What should be done in the future
- If we decide to harden this architecture:
  - Implement `ActionStop` in the runner (or remove the UI affordance).
  - Replace status-line string parsing with a typed UI message.
  - Throttle or transition-detect snapshot-derived log entries.
  - Wire the “planned” pipeline live output/config patch/progress events end-to-end.

### Code review instructions
- Start with the analysis doc:
  - `devctl/ttmp/2026/01/07/MO-010-DEVCTL-CLEANUP-PASS--devctl-cleanup-pass/analysis/03-devctl-fundamentals-events-ui-messages-textbook-walkthrough.md`
- Then review the critical pipeline files in order:
  - `devctl/cmd/devctl/cmds/tui.go`
  - `devctl/pkg/tui/state_watcher.go`
  - `devctl/pkg/tui/action_runner.go`
  - `devctl/pkg/tui/transform.go`
  - `devctl/pkg/tui/forward.go`
  - `devctl/pkg/tui/models/root_model.go`

### Technical details
- Document created:
  - `devctl/ttmp/2026/01/07/MO-010-DEVCTL-CLEANUP-PASS--devctl-cleanup-pass/analysis/03-devctl-fundamentals-events-ui-messages-textbook-walkthrough.md`

## Step 9: Uploaded Textbook Walkthrough to reMarkable

This step converted the new “fundamentals → events → UI messages” textbook walkthrough markdown to a PDF and uploaded it to the reMarkable device. The goal was to have an annotated-friendly copy under the ticket’s mirrored folder structure alongside the other MO-010 documents.

The upload succeeded without requiring `--force` (no overwrite needed). The PDF landed under `ai/2026/01/07/MO-010-DEVCTL-CLEANUP-PASS/analysis/`, consistent with the ticket mirroring conventions used elsewhere in MO-010.

**Commit (code):** N/A

### What I did
- Dry-run to confirm the remote destination and commands:
  - `python3 /home/manuel/.local/bin/remarkable_upload.py --dry-run --ticket-dir "/home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/ttmp/2026/01/07/MO-010-DEVCTL-CLEANUP-PASS--devctl-cleanup-pass" --mirror-ticket-structure --remote-ticket-root MO-010-DEVCTL-CLEANUP-PASS "/home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/ttmp/2026/01/07/MO-010-DEVCTL-CLEANUP-PASS--devctl-cleanup-pass/analysis/03-devctl-fundamentals-events-ui-messages-textbook-walkthrough.md"`
- Uploaded the document:
  - `python3 /home/manuel/.local/bin/remarkable_upload.py --ticket-dir "/home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/ttmp/2026/01/07/MO-010-DEVCTL-CLEANUP-PASS--devctl-cleanup-pass" --mirror-ticket-structure --remote-ticket-root MO-010-DEVCTL-CLEANUP-PASS "/home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/ttmp/2026/01/07/MO-010-DEVCTL-CLEANUP-PASS--devctl-cleanup-pass/analysis/03-devctl-fundamentals-events-ui-messages-textbook-walkthrough.md"`

### Why
- The user asked to upload the new analysis doc for reading/annotation on the reMarkable.

### What worked
- Upload succeeded:
  - `OK: uploaded 03-devctl-fundamentals-events-ui-messages-textbook-walkthrough.pdf -> ai/2026/01/07/MO-010-DEVCTL-CLEANUP-PASS/analysis/`

### What didn't work
- N/A.

### What I learned
- `--mirror-ticket-structure` + `--remote-ticket-root` keeps all PDFs for a ticket collision-free and consistently organized on-device.

### What was tricky to build
- N/A.

### What warrants a second pair of eyes
- N/A.

### What should be done in the future
- N/A.

### Code review instructions
- N/A (upload only).

### Technical details
- Uploaded to:
  - `ai/2026/01/07/MO-010-DEVCTL-CLEANUP-PASS/analysis/03-devctl-fundamentals-events-ui-messages-textbook-walkthrough.pdf`

## Step 12: Protocol v2 Cleanup Pass — Plan + Tasks (Kickoff)

This step converted the “protocol v2 handshake + repository context + capability enforcement” design into an executable, phased implementation plan and a concrete `docmgr` task list. The goal was to make the cleanup pass reviewable and progress-trackable before touching protocol/runtime code.

The immediate outcome is that MO-010 now has a detailed plan embedded in the v2 design doc and a numbered task list that maps directly to the planned phases (protocol schema, runtime enforcement, repository/meta plumbing, dynamic commands, docs/plugins/tests, and validation).

**Commit (code):** N/A

### What I did
- Expanded the “Implementation Plan” section in:
  - `devctl/ttmp/2026/01/07/MO-010-DEVCTL-CLEANUP-PASS--devctl-cleanup-pass/design-doc/02-protocol-v2-handshake-command-specs-repository-context-capability-enforced-calls.md`
- Created a ticket task list aligned to the phases:
  - `docmgr task edit --ticket MO-010-DEVCTL-CLEANUP-PASS --id 1 --text "..."`
  - `docmgr task add --ticket MO-010-DEVCTL-CLEANUP-PASS --text "..."`

### Why
- The protocol changes are intentionally breaking; a concrete plan and task breakdown reduces “half-migrated” risk and makes it easier to batch commits cleanly (protocol → runtime → call sites → docs/plugins/tests).

### What worked
- `docmgr task` made it quick to turn the design into a checklist without hand-editing `tasks.md`.

### What didn't work
- N/A.

### What I learned
- The highest-risk integration points are not the schema change itself but the “plumbing” surfaces (`runtime.Factory.Start` call sites, request context construction, and dynamic command bootstrapping before Cobra executes internal commands).

### What was tricky to build
- Keeping the plan “no-compat” while still sequencing steps so the codebase can compile and tests can be updated incrementally.

### What warrants a second pair of eyes
- The Repository/meta plumbing approach: whether to attach request meta to clients at start-time vs passing request context explicitly per call.

### What should be done in the future
- N/A (the tasks list is the follow-up plan for this work).

### Code review instructions
- Review the updated plan in:
  - `devctl/ttmp/2026/01/07/MO-010-DEVCTL-CLEANUP-PASS--devctl-cleanup-pass/design-doc/02-protocol-v2-handshake-command-specs-repository-context-capability-enforced-calls.md`
- Review the tracked task list in:
  - `devctl/ttmp/2026/01/07/MO-010-DEVCTL-CLEANUP-PASS--devctl-cleanup-pass/tasks.md`

### Technical details
- Commands executed (selection):
  - `docmgr task list --ticket MO-010-DEVCTL-CLEANUP-PASS`
  - `docmgr task edit --ticket MO-010-DEVCTL-CLEANUP-PASS --id 1 --text "..."`
  - `docmgr task add --ticket MO-010-DEVCTL-CLEANUP-PASS --text "..."`

## Step 13: Implemented Protocol v2 Handshake + Capability Enforcement + Repository Meta

This step implemented the core “no-compat” protocol v2 changes in code: handshake v2 validation, structured `capabilities.commands`, handshake-driven dynamic command registration, explicit request metadata (no `context.Value`), and runtime-level capability enforcement for both calls and streams.

The result is that devctl no longer issues `commands.list` requests at startup, unsupported ops fail fast locally with `E_UNSUPPORTED`, and all in-repo example/test plugins have been updated to speak protocol v2 so `go test ./...` remains green.

**Commit (code):** 7fce1bc — "Protocol v2: handshake command specs and safe runtime calls"

### What I did
- Protocol v2 + handshake schema:
  - Changed `devctl/pkg/protocol/types.go` to add `ProtocolV2` and structured `Capabilities.Commands` (`CommandSpec`/`CommandArg`).
  - Updated `devctl/pkg/protocol/validate.go` to accept only v2 and to validate command specs (names/uniqueness/arg fields).
- Runtime safety defaults:
  - Updated `devctl/pkg/runtime/client.go` so `Call`/`StartStream` fail fast with `E_UNSUPPORTED` when an op is not declared in handshake capabilities.
  - Added `devctl/pkg/runtime/errors.go` (`OpError` + `ErrUnsupported`) so callers/tests can detect `E_UNSUPPORTED` without string parsing.
- Explicit request metadata (no ambient context values):
  - Added `devctl/pkg/runtime/meta.go` (`RequestMeta`) and plumbed it through `runtime.Factory.Start(..., StartOptions{Meta: ...})`.
  - Deleted `devctl/pkg/runtime/context.go` and updated call sites (CLI + TUI) to pass explicit request meta at client start.
- Repository container + dynamic commands:
  - Added `devctl/pkg/repository/repository.go` to centralize repo root/config/spec discovery and request meta.
  - Refactored `devctl/cmd/devctl/cmds/dynamic_commands.go` to read command specs from handshake and to stop re-discovering providers at runtime.
- Plugin updates + tests:
  - Updated all in-repo example/test plugins to emit `protocol_version: "v2"` and (where applicable) advertise commands via handshake.
  - Added `devctl/testdata/plugins/ignore-unknown/plugin.py` and `TestRuntime_CallUnsupportedFailsFast` to prove “unsupported op” no longer hangs on misbehaving plugins.
- Validation:
  - `go test ./...` (from `devctl/`) after updating plugins and call sites.

### Why
- Removing `commands.list` eliminates a common startup stall and removes a fragile “discovery request” surface from the protocol.
- Runtime capability enforcement makes “forgot to gate” bugs structurally hard to reintroduce.
- Explicit meta removes hidden dependency on `context.Value`, so repo_root/cwd/dry_run cannot be silently dropped.

### What worked
- The existing `cmds/dynamic_commands_test.go` became a good regression test once the command plugin was migrated to handshake-advertised commands.
- Adding an “ignore unknown ops” plugin fixture made it straightforward to validate that runtime short-circuits unsupported calls.

### What didn't work
- N/A (no unexpected failures after the v2 migration; tests passed once in-repo plugins were updated).

### What I learned
- The “request ctx metadata” and “capability enforcement” changes compose well: once `Start` carries `RequestMeta`, there is no need for context mutation helpers anywhere.

### What was tricky to build
- Refactoring `dynamic_commands.go` to avoid provider re-discovery while still respecting user flags (`--repo-root`, `--config`, `--dry-run`, `--timeout`) required splitting “startup discovery metadata” (handshake/commands) from “per-invocation meta” (dry-run/timeout).

### What warrants a second pair of eyes
- The decision to reject protocol v1 entirely in `protocol.ValidateHandshake` (intentional no-compat): review that every in-repo fixture and any expected plugin ecosystem is migrated before rollout.

### What should be done in the future
- N/A (remaining work is tracked in MO-010 tasks, primarily ongoing doc polish and any follow-on ergonomics).

### Code review instructions
- Start with protocol + runtime invariants:
  - `devctl/pkg/protocol/types.go`
  - `devctl/pkg/protocol/validate.go`
  - `devctl/pkg/runtime/client.go`
  - `devctl/pkg/runtime/errors.go`
  - `devctl/pkg/runtime/factory.go`
- Then review wiring and bootstraps:
  - `devctl/pkg/repository/repository.go`
  - `devctl/cmd/devctl/cmds/dynamic_commands.go`
  - `devctl/cmd/devctl/cmds/up.go`
  - `devctl/pkg/tui/action_runner.go`
- Validate:
  - `cd devctl && go test ./...`

### Technical details
- Commands executed (selection):
  - `cd devctl && go test ./...`
  - `cd devctl && go fmt ./...` (then reverted unrelated gofmt-only diffs before committing)

## Step 14: Added an Exhaustive Real-World Test Task Matrix (CLI + Fixtures + TUI)

This step translated the protocol v2 refactor into a long, explicit “what to actually try” checklist that leans on existing in-repo fixtures and smoke commands. The goal is to validate behavior the way users experience it (CLI + state files + wrapper + TUI), not just via unit tests.

The outcome is a large set of MO-010 tasks covering: core CLI flows (`plan/up/status/logs/down/plugins`), negative protocol cases (handshake contamination, v1 rejection, invalid commands), both fixture generators (MO-006 and MO-009), wrapper edge cases, and TUI behavior tested via `tmux` with concrete keybindings for each view.

**Commit (code):** N/A

### What I did
- Enumerated fixtures and test surfaces:
  - CLI smoketests (`dev smoketest`, `dev smoketest e2e`, `dev smoketest failures`, `dev smoketest logs`, `dev smoketest supervise`)
  - Fixture scripts:
    - `devctl/ttmp/2026/01/06/MO-006-DEVCTL-TUI--create-a-devctl-tui/scripts/setup-fixture-repo-root.sh`
    - `devctl/ttmp/2026/01/06/MO-009-TUI-COMPLETE-FEATURES--complete-tui-features-per-mo-006-design/scripts/setup-comprehensive-fixture.sh`
  - Plugin fixtures (`devctl/testdata/plugins/*`) used for protocol validation and negative cases.
- Added an exhaustive task list for “real world tests” and TUI testing (via `tmux`) to:
  - `devctl/ttmp/2026/01/07/MO-010-DEVCTL-CLEANUP-PASS--devctl-cleanup-pass/tasks.md`

### Why
- Protocol v2 is intentionally breaking; the safest way to ship it is to validate the user-facing loop (CLI + wrapper + state + TUI) against realistic fixtures and failure modes.

### What worked
- The fixture scripts already define a “feature coverage matrix”; converting them into tasks makes the validation path explicit and repeatable.

### What didn't work
- N/A (task creation only).

### What I learned
- The TUI surface is large enough that the only sane validation approach is “view-by-view + keybinding-by-keybinding” on both a basic fixture and a high-entropy comprehensive fixture.

### What was tricky to build
- Keeping tasks actionable without burying them in prose: the compromise is consistent prefixes (`[Fixture/...], [TUI/...], [CLI/...]`) and referencing the canonical fixture scripts.

### What warrants a second pair of eyes
- The wrapper regression tasks around “slow handshake” vs “dynamic discovery runs before __wrap-service”: if this becomes a real risk, it likely requires a design change (skip discovery for internal commands), not just more tests.

### What should be done in the future
- If we keep accumulating TUI features, promote the task list into a dedicated “manual test matrix” doc so tasks stay short and the matrix holds the details.

### Code review instructions
- Review the task matrix in:
  - `devctl/ttmp/2026/01/07/MO-010-DEVCTL-CLEANUP-PASS--devctl-cleanup-pass/tasks.md`

### Technical details
- Commands executed (selection):
  - `docmgr task add --ticket MO-010-DEVCTL-CLEANUP-PASS --text "..."`

## Step 15: Make __wrap-service Robust to CLI Startup Logic

This step fixed a real-world regression surfaced during exhaustive testing: `devctl up` could fail with `wrapper did not report child start` if dynamic command discovery was slow (e.g., plugins that sleep before emitting the handshake). The root cause was that `AddDynamicPluginCommands` ran unconditionally during process startup, even for the supervisor’s internal `__wrap-service` invocation, delaying wrapper execution long enough to trip the supervisor’s ready-file deadline.

It also fixed direct `__wrap-service` runs outside supervisor. When invoked manually, `__wrap-service` wasn’t guaranteed to be a process-group leader, which caused child startup to fail during process-group wiring; making the process-group invariant explicit resolved the confusing “operation not permitted” error and made the internal tool reliably runnable during debugging.

**Commit (code):** a6c4e52 — "wrap-service: skip dynamic discovery and setpgid"

### What I did
- Reproduced the wrapper failure deterministically using a config with multiple “slow handshake” plugins.
- Updated dynamic command discovery to skip plugin startup entirely when executing `__wrap-service`:
  - `devctl/cmd/devctl/cmds/dynamic_commands.go`
- Made `__wrap-service` establish a stable process-group invariant before starting its child:
  - `devctl/cmd/devctl/cmds/wrap_service.go` (`syscall.Setpgid(0, 0)`)
- Added unit coverage to prevent regression:
  - `devctl/cmd/devctl/cmds/dynamic_commands_test.go` (`TestDynamicCommands_SkipsWrapService`)

### Why
- `__wrap-service` is an internal supervisor primitive; its readiness must not depend on unrelated user-facing startup features (dynamic command registration).
- Direct execution of internal commands is a valuable debugging tool; it should work reliably without requiring a supervisor to pre-configure process groups.

### What worked
- The wrapper failure is no longer reproducible under the same slow-handshake stress scenario.
- Direct `__wrap-service` runs now start children and write ready/exit-info files as expected.

### What didn't work
- Before the fix, direct wrapper invocation failed with:
  - `Error: start child: fork/exec /usr/bin/bash: operation not permitted`
- Before the fix, `up` failed under slow handshake conditions with:
  - `Error: wrapper did not report child start`

### What I learned
- Any “global startup” logic in `main.go` that touches plugins can break internal subcommands unless explicitly scoped.

### What was tricky to build
- Avoiding an overly-broad skip (e.g. skipping discovery whenever an argument equals `__wrap-service`), since `__wrap-service` can also appear as an argument to real commands.

### What warrants a second pair of eyes
- The interplay between Cobra arg parsing and the minimal flag parsing in `parseRepoArgs` (the “first positional command” detection).

### What should be done in the future
- Consider moving dynamic command discovery into a Cobra `PreRun` hook that runs only for user-facing commands (not internal subcommands), if more internal commands are added.

### Code review instructions
- Start with:
  - `devctl/cmd/devctl/cmds/dynamic_commands.go`
  - `devctl/cmd/devctl/cmds/wrap_service.go`
  - `devctl/cmd/devctl/cmds/dynamic_commands_test.go`
- Validate:
  - `cd devctl && go test ./... -count=1`
  - Re-run a slow-handshake wrapper stress fixture and confirm `up` succeeds.
