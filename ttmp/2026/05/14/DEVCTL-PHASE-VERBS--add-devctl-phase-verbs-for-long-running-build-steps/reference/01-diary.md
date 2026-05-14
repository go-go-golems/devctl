---
Title: Diary
Ticket: DEVCTL-PHASE-VERBS
Status: active
Topics:
    - devctl
    - cli
    - build
    - workflow
DocType: reference
Intent: long-term
Owners: []
RelatedFiles:
    - Path: ../../../../../../../../../2026-05-13/coinvault-loop-analysis/pinocchio/cmd/web-chat/plugins/webchat.py
      Note: Motivating plugin and long-running build behavior
    - Path: cmd/devctl/cmds/plan.go
      Note: Existing partial phase command used as design template
    - Path: cmd/devctl/cmds/up.go
      Note: Full up pipeline and existing build/prepare flags shape the phase verb design
    - Path: pkg/engine/pipeline.go
      Note: Engine phase API and merge semantics for build/prepare/validate/launch-plan
    - Path: pkg/runtime/client.go
      Note: Runtime call and stream APIs determine logging/streaming options
    - Path: ttmp/2026/05/14/DEVCTL-PHASE-VERBS--add-devctl-phase-verbs-for-long-running-build-steps/design-doc/01-design-devctl-phase-verbs-for-streaming-long-running-steps.md
ExternalSources: []
Summary: Chronological investigation diary for designing devctl phase verbs.
LastUpdated: 2026-05-14T14:30:00-04:00
WhatFor: Use to understand how the DEVCTL-PHASE-VERBS design was produced and what evidence shaped it.
WhenToUse: When continuing implementation or reviewing the design rationale.
---






# Diary

## Goal

This diary records the investigation and design work for adding first-class devctl commands that run individual pipeline phases, especially a `devctl build` command suitable for long-running builds with visible progress.

## Step 1: Created the docmgr ticket and mapped the existing devctl phase architecture

I created a new docmgr ticket for the phase-verb work, then inspected devctl’s command registration, `up` pipeline, `plan` command, engine phase methods, runtime protocol, and the specific Pinocchio plugin that motivated the request. The goal was to avoid designing a new abstraction before understanding how much of the desired behavior already exists.

The main discovery is that devctl already has strong internal phase boundaries (`Pipeline.Build`, `Pipeline.Prepare`, `Pipeline.Validate`, `Pipeline.LaunchPlan`), but only `up` and `plan` expose selected combinations of those phases as CLI verbs. This makes `devctl build` mostly a CLI/helper extraction problem for the first implementation, with protocol-level streaming as a follow-up rather than a prerequisite.

### Prompt Context

**User prompt (verbatim):** "Create a new docmgr ticket to add devctl a bunch more verbs to run individual phases. For example, I want to run the build phase fo devctl in /home/manuel/workspaces/2026-05-13/coinvault-loop-analysis/pinocchio because it takes a while , and too long for up to time out. Also I want to kind of see the logs stream as I do the build step. Ideally I'd like something like devctl build , but there might be better names, and I could see that for other steps (we already have plan it looks like).

Keep a diary as you work.

Create  a detailed analysis / design / implementation guide that is for a new intern, explaining all the parts of the system needed to understand what it is, with prose paragraphs and bullet point sand pseudocode and diagrams and api references and file references. It should be very clear and technical. Store in the ticket and the nupload to remarkable."

**Assistant interpretation:** Create a structured docmgr ticket with a long-form design/implementation guide for new devctl phase verbs, maintain a diary, inspect the existing codebase and Pinocchio use case, then upload the resulting docs to reMarkable.

**Inferred user intent:** The user wants an implementation-ready technical package that a new intern can follow to add `devctl build` and related commands safely, with enough codebase orientation and evidence to reduce ramp-up time.

**Commit (code):** N/A — documentation/research only.

### What I did

- Loaded the `ticket-research-docmgr-remarkable` and `diary` skill instructions.
- Ran:
  - `docmgr status --summary-only`
  - `docmgr ticket create-ticket --ticket DEVCTL-PHASE-VERBS --title "Add devctl phase verbs for long-running build steps" --topics devctl,cli,build,workflow`
  - `docmgr doc add --ticket DEVCTL-PHASE-VERBS --doc-type design-doc --title "Design: devctl phase verbs for streaming long-running steps"`
  - `docmgr doc add --ticket DEVCTL-PHASE-VERBS --doc-type reference --title "Diary"`
- Inspected repository files with `rg --files`, `rg -n`, `read`, and line-numbered `nl -ba ... | sed -n ...` commands.
- Read key devctl files:
  - `cmd/devctl/main.go`
  - `cmd/devctl/cmds/root.go`
  - `cmd/devctl/cmds/common.go`
  - `cmd/devctl/cmds/up.go`
  - `cmd/devctl/cmds/plan.go`
  - `cmd/devctl/cmds/stream.go`
  - `cmd/devctl/cmds/dynamic_commands.go`
  - `pkg/engine/pipeline.go`
  - `pkg/engine/types.go`
  - `pkg/runtime/client.go`
  - `pkg/runtime/router.go`
  - `pkg/protocol/types.go`
  - `pkg/repository/repository.go`
  - `README.md`
  - `pkg/doc/topics/devctl-plugin-authoring.md`
- Inspected the concrete Pinocchio repo config and plugin:
  - `/home/manuel/workspaces/2026-05-13/coinvault-loop-analysis/pinocchio/.devctl.yaml`
  - `/home/manuel/workspaces/2026-05-13/coinvault-loop-analysis/pinocchio/cmd/web-chat/plugins/webchat.py`

### Why

- The requested `devctl build` command should reuse existing devctl semantics instead of becoming a separate plugin-command path.
- The user specifically mentioned that `plan` already exists, so I needed to compare `plan` to the missing build/prepare/validate verbs.
- The streaming/logging request required understanding the protocol distinction between normal calls, stream-start ops, stderr logging, and stdout contamination rules.

### What worked

- `docmgr` created the ticket and documents cleanly.
- The existing codebase has a clear engine API for all needed phases.
- `devctl plan` is a useful template for a partial phase command.
- Runtime stream support already exists, which gives a future path for structured progress.
- The plugin authoring guide already documents stderr as the correct place for human progress logs.

### What didn't work

- The initial broad line-number extraction command produced output that hit the 50KB tool truncation limit:
  - `for f in cmd/devctl/cmds/root.go cmd/devctl/cmds/up.go ...; do ...; done`
  - The output was truncated and saved to `/tmp/pi-bash-037a3bca2316ea50.log`.
- I corrected this by running narrower `nl -ba ... | sed -n ...` ranges for each key file.

### What I learned

- `devctl up` already runs `config.mutate → build.run → prepare.run → validate.run → launch.plan → supervise` and exposes skip/step flags.
- `devctl plan` already runs a subset: `config.mutate + launch.plan`.
- The engine methods are already separated enough to support top-level phase commands with minimal new engine work.
- `build.run` currently uses `Client.Call`; therefore structured live event streaming is not automatic for build phases.
- Pinocchio’s plugin writes high-level messages to stderr but captures long subprocess output and imposes internal `timeout=120` values, so `devctl --timeout 10m` alone will not guarantee a ten-minute Go build unless the plugin timeout is adjusted.

### What was tricky to build

The tricky part was separating three related but distinct concepts that all sound like “stream logs”:

1. Plugin stderr progress during a normal `build.run` call.
2. Protocol event streams started through `Client.StartStream` and `devctl stream start`.
3. Supervised service logs after `launch.plan` and `supervise.Start`.

The immediate `devctl build` feature can use the first path. The second path requires a protocol/API design for streamed phase results. The third path is unrelated to build because services are not started during a standalone build command.

### What warrants a second pair of eyes

- Whether the first patch should extract a shared phase runner helper immediately, or add `newBuildCmd` with localized duplication first and refactor after tests.
- Whether `devctl validate` should be a top-level verb in the first patch or a follow-up.
- Whether users expect `devctl build` to print a terse human summary by default instead of pretty JSON like `devctl plan`.

### What should be done in the future

- Implement static `build`, `prepare`, and `validate` commands.
- Add command-level tests for step filtering, profile selection, dry-run behavior, timeouts, and invalid validation exit behavior.
- Patch the Pinocchio plugin separately to honor `input.steps`, stream subprocess output to stderr, and remove or parameterize its internal 120-second subprocess timeouts.

### Code review instructions

- Start review with `cmd/devctl/cmds/up.go` and `cmd/devctl/cmds/plan.go` to understand current orchestration.
- Then inspect `pkg/engine/pipeline.go` for phase behavior and merge semantics.
- For runtime/protocol implications, inspect `pkg/runtime/client.go`, `pkg/runtime/router.go`, and `pkg/protocol/types.go`.
- Validate the future implementation with:
  - `go test ./cmd/devctl/cmds ./pkg/engine ./pkg/runtime -count=1`
  - `go test ./... -count=1`
  - a manual `devctl build --repo-root /home/manuel/workspaces/2026-05-13/coinvault-loop-analysis/pinocchio --timeout 10m` once implemented.

### Technical details

Key evidence collected:

- Static commands are registered in `cmd/devctl/cmds/root.go:8-24`; there is no `build` command today.
- `devctl up` runs build at `cmd/devctl/cmds/up.go:130-138` and exposes `--build-step` at `up.go:209`.
- `Pipeline.Build` passes `steps` to `build.run` and merges returned steps/artifacts at `pkg/engine/pipeline.go:102-135`.
- `Client.Call` is request/response only at `pkg/runtime/client.go:73-130`.
- `Client.StartStream` exists for stream-start ops at `pkg/runtime/client.go:133-198`.
- Pinocchio declares `build.run` and dynamic build commands at `/home/manuel/workspaces/2026-05-13/coinvault-loop-analysis/pinocchio/cmd/web-chat/plugins/webchat.py:102-123`.
- Pinocchio’s `build.run` captures subprocess output and uses `timeout=120` at `webchat.py:220-268`.

## Step 2: Wrote the intern-facing design and prepared ticket bookkeeping

I wrote the primary design document as an implementation guide rather than just a proposal. It includes the problem statement, current-state architecture, gap analysis, CLI API proposal, internal helper pseudocode, logging/streaming tradeoffs, phased implementation steps, testing strategy, diagrams, and file references.

The design intentionally recommends a two-stage approach: ship phase verbs first using the existing `Pipeline` request/response methods and stderr progress, then add structured event streaming only if users need machine-readable progress. This keeps the first implementation small enough for an intern while still explaining the longer-term protocol path.

### Prompt Context

**User prompt (verbatim):** (same as Step 1)

**Assistant interpretation:** Produce a detailed, evidence-backed implementation design and store it in the docmgr ticket.

**Inferred user intent:** Give future implementers a clear technical roadmap and reduce ambiguity around command names, timeouts, streaming, and plugin responsibilities.

**Commit (code):** N/A — documentation/research only.

### What I did

- Replaced the generated design-doc scaffold with a complete design guide at:
  - `ttmp/2026/05/14/DEVCTL-PHASE-VERBS--add-devctl-phase-verbs-for-long-running-build-steps/design-doc/01-design-devctl-phase-verbs-for-streaming-long-running-steps.md`
- Replaced the generated diary scaffold with this chronological diary at:
  - `ttmp/2026/05/14/DEVCTL-PHASE-VERBS--add-devctl-phase-verbs-for-long-running-build-steps/reference/01-diary.md`
- Included references to the concrete Pinocchio use case and the devctl code paths a new intern should read first.

### Why

The user asked for a guide “for a new intern,” so the design needed to explain the system, not just list code changes. The guide therefore includes concepts, diagrams, API contracts, pseudocode, implementation phases, and file-level references.

### What worked

- The evidence gathered in Step 1 mapped cleanly onto the proposed implementation.
- The existing code’s phase boundaries made the implementation plan straightforward.
- The design could clearly separate immediate CLI value from future structured streaming.

### What didn't work

- No implementation or tests were run in this ticket because the user asked to create a research/design ticket, not to change devctl code yet.

### What I learned

- The best command naming is likely simple top-level verbs: `devctl build`, `devctl prepare`, `devctl validate`.
- `devctl plan` should probably remain the canonical launch-planning command; `devctl launch-plan` can be an alias later if desired.
- For immediate progress visibility, plugin stderr is the intended and least risky path.

### What was tricky to build

The design had to avoid promising that `devctl build` alone can stream detailed tool logs. In the current architecture, devctl can stream plugin stderr lines, but plugin authors control whether subprocess output is emitted to stderr or captured. The guide calls this out explicitly and recommends a Pinocchio follow-up.

### What warrants a second pair of eyes

- The proposed output shape `{config, build}` mirrors `plan`, but a CLI reviewer may prefer a human-first summary with `--json` for machine output.
- The protocol extension section is intentionally deferred; it needs review before anyone implements `build.stream`.

### What should be done in the future

- Run `docmgr doctor` and fix any metadata/vocabulary issues.
- Upload the design bundle to reMarkable.
- Implement the phase commands in a follow-up development task.

### Code review instructions

- Review the design document for correctness against the referenced files and line numbers.
- Confirm that the proposed commands preserve existing repo/profile/strict/dry-run/timeout semantics.
- Confirm the Pinocchio-specific notes match the current plugin behavior before using them as implementation requirements.

### Technical details

The primary design document includes:

- CLI examples for the Pinocchio repo.
- Pseudocode for `withPhaseRunner`, `newBuildCmd`, `newPrepareCmd`, and `newValidateCmd`.
- Diagrams for current `up`, proposed `build`, and future structured streaming flows.
- Implementation phases and validation commands.

## Step 3: Replanned the ticket into implementation phases

I converted the ticket task list from a flat checklist into a phased implementation plan. This gives the implementation a reviewable order: core commands first, tests second, documentation third, and ticket closeout last.

The documentation phase is explicit because the new commands change the primary user workflow. The README and built-in help topics need to describe `devctl build`, `devctl prepare`, and `devctl validate`, and they need to explain that long-running progress should be written to plugin stderr rather than protocol stdout.

### Prompt Context

**User prompt (verbatim):** "Add tasks to the ticket, grouped in phases, also make a phase to make sure the documentation is up to date (and the README). 

Then iplement them one by one, keeping a diary as you work, and committing at appropriate intervals."

**Assistant interpretation:** Update the ticket tasks into phased implementation buckets, then implement the feature incrementally with diary updates and focused git commits.

**Inferred user intent:** The user wants the ticket to become an executable implementation plan and wants the repository history to show sensible milestones.

**Commit (code):** pending — ticket/task update commit.

### What I did
- Rewrote `tasks.md` into five phases: research, core implementation, tests, documentation, and closeout.
- Added explicit README/help documentation tasks.
- Added deferred follow-ups for the Pinocchio plugin and structured phase stream protocol.

### Why
- Grouped phases make it easier to review progress and avoid mixing code, tests, and docs in one large change.
- Documentation is part of the user-facing feature, not an afterthought.

### What worked
- The existing design document already had enough structure to translate directly into phased tasks.

### What didn't work
- N/A.

### What I learned
- The implementation can be split cleanly into commits: ticket planning, command implementation/tests, docs, and closeout.

### What was tricky to build
- The task list needs to track both work completed in the earlier research pass and new implementation work without implying the feature is already shipped.

### What warrants a second pair of eyes
- Whether `devctl launch-plan` should be a tracked implementation task now or remain deferred; I left it out of the initial core phase because `devctl plan` already exists.

### What should be done in the future
- Check off tasks as code, tests, and docs land.

### Code review instructions
- Review `tasks.md` before implementation to confirm the planned phases match the desired scope.

### Technical details
- The task file now includes a dedicated “Phase 3 — Documentation refresh” section covering `README.md`, `devctl-user-guide`, and `devctl-plugin-authoring`.
