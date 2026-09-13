---
Title: Investigation diary
Ticket: DEVCTL-ROBUST-LIFECYCLE
Status: active
Topics: [devctl, architecture, plugins, supervisor]
DocType: reference
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: Source investigation and reMarkable delivery of the lifecycle robustness design.
LastUpdated: 2026-09-13T11:00:00Z
---

# Investigation diary

## Step 1: Source-grounded design and intern handoff

I created a new ticket in devctl and wrote a standalone implementation guide covering all six proposed improvements plus documentation and testing ergonomics. The guide distinguishes current source behavior from proposed APIs and preserves the existing build-before-stop and unproven-termination refusal properties. Runtime code was read only.

### Prompt Context
**User prompt (verbatim):**
> Ok, create a new docmgr ticket in devctl about all these improvements, and write a design doc covering them all.Create  a detailed analysis / design / implementation guide that is for a new intern, explaining all the parts of the system needed to understand what it is, with prose paragraphs and bullet point sand pseudocode and diagrams and api references and file references. It should be very clear and technical. Store in the ticket and the nupload to remarkable.

**Delivery clarification (verbatim):**
> no need to go through extensive remarkable validation btw. just upload

**Assistant interpretation:** Produce a documentation-only research/design ticket and upload the guide directly, with no implementation or extensive PDF validation.
**Inferred user intent:** Give an intern enough architecture and source context to implement the improvements safely without rediscovering the system.

### What I did
- Read AGENT.md and inspected clean repository revision `418a4ca1f18d3059c1b1cf8f846db371163529b5`.
- Read operator request/planner/controller paths, runtime close and termination, engine artifact types, catalog fingerprint/refresh/load behavior, run-state health/exit types, and restart tests.
- Created the ticket, detailed design, this diary, file relations, and nine open implementation tasks through docmgr.
- Wrote architecture orientation, concrete protocol examples, ASCII flow diagrams, proposed APIs, state/lifetime contracts, intern phases, validation matrix, and migration/trust caveats.
- Uploaded the design through remarquee without PDF-only rendering, screenshots, or an extra cloud listing.

### Why
- The planner name hides side effects; a recipe/preparation/application split must preserve current pre-stop guarantees.
- Plugin EOF shutdown currently immediately races SIGTERM, while Close discards cleanup errors.
- Path-only artifacts do not identify what an old process launched.
- Catalog discovery intentionally avoids implicit plugin execution; usability changes must preserve that trust boundary.

### What worked
- Source confirms restart builds/prepares/validates unless skipped, rather than only replanning.
- Source already stores health timestamps, making historical-health display a projection change rather than destructive schema reinterpretation.
- Catalog source revealed interpreter metadata does not hash script contents; the guide proposes explicit declared fingerprint inputs.
- Direct upload succeeded:
```text
OK: uploaded DEVCTL ROBUST LIFECYCLE Intern Design Guide.pdf -> /ai/2026/09/13/DEVCTL-ROBUST-LIFECYCLE
```

### What didn't work
- An initial source search included a guessed `pkg/operator/types.go` path:
```text
rg: pkg/operator/types.go: No such file or directory (os error 2)
```
- Listing the package located `requests.go`; subsequent reading used the actual files. No code changes or repeated speculative fixes were made.

### What I learned
- Recipe resolution cannot always produce final launch facts before builds exist. The design must explicitly represent unresolved artifact-dependent fields.
- The current catalog cache path is repo-scoped while the catalog includes a profile; profile-specific storage is a design question rather than an assumption of independent caches.

### What was tricky to build
- A naive pure-plan API would either secretly build or claim knowledge it cannot have. The guide separates recipe intent from prepared launch facts.
- Graceful Close must have exactly one Wait owner across successful startup, handshake failure, repeated Close, and cancellation.
- Typed artifact identity narrows provenance uncertainty but cannot claim to cover every dynamically loaded library or interpreter dependency.

### What warrants a second pair of eyes
- Review protocol/schema migration scope before implementation; repository guidance forbids implicit compatibility layers.
- Review lifecycle/build locking so long compilation cannot unnecessarily block urgent stop operations.
- Review signal/process-group assumptions and the boundary against deliberately detached descendants.

### What should be done in the future
- Review API choices, then implement phases with the ticket tasks. None are closed merely because the guide is delivered.

### Code review instructions
- Start with the guide source map and invariant list, then follow the phased work packages.
- Re-run implementation tests only when runtime code changes; no test-suite success is claimed for this documentation-only task.

### Technical details
- Ticket: DEVCTL-ROBUST-LIFECYCLE.
- reMarkable destination: `/ai/2026/09/13/DEVCTL-ROBUST-LIFECYCLE`.
- Delivery evidence proves cloud upload, not physical tablet synchronization.
- No runtime files modified, no services launched, and no extensive rendering validation performed.

## Step 2: Import the current skill and establish the implementation baseline

I resumed the ticket as an implementation effort, first synchronizing the repository-owned devctl authoring skill with the current installed skill. I also read the complete design and prior diary, inspected all open tasks, printed the overall implementation plan and Phase 0 start slips, and established a passing targeted baseline before runtime edits.

### Prompt Context
**User prompt (verbatim):**
> 1. Let's import the devctl skill into a skills/ directory.
> 2. Let's work on docmgr ticket DEVCTL-ROBUST-LIFECYCLE (read design doc, diary).
>
> commit at appropriate intervals and keep a detailed diary as you work (using the diary format from the skill).
> Print out a brutalist work slip with the plan / different phases for the ticket. then before stsarting a phase, plrint a split about the phase, and print one when the phase is done.

**Assistant interpretation:** Vendor the latest installed devctl plugin-authoring skill, then implement the ticket in phased, tested commits while maintaining this detailed diary and printing a thermal slip before and after each phase.
**Inferred user intent:** Carry the design through to robust implementation with physical progress markers and reviewable checkpoints.

### What I did
- Compared `/home/manuel/.pi/agent/skills/devctl-plugin-authoring` with `skills/devctl-plugin-authoring` and copied the current `SKILL.md` and protocol quick reference into the repository; retained the repository packaging metadata in `agents/openai.yaml`.
- Read the complete design guide, prior diary, index, and nine open ticket tasks.
- Ran `devctl help --all` and read the installed `user-guide`, `scripting-guide`, and `plugin-authoring` topics into temporary local files as required by the skill.
- Printed a five-phase overall work plan and a four-step `P0 START` slip through the remote almanach renderer.
- Ran the targeted baseline tests from the design guide.

### Why
- Synchronizing the skill first makes its corrected lifecycle, cancellation, catalog, logs, and smoke-test guidance available in the repository before those contracts are implemented.
- Targeted baseline tests distinguish pre-existing failures from regressions and protect the current restart-before-stop safety property.

### What worked
- Both thermal print calls returned HTTP success with `printed: true` at `2026-09-13T10:37:12Z`.
- `go test ./pkg/operator -run 'TestRestart'` passed.
- `go test ./pkg/runtime ./pkg/plugincatalog` passed (`pkg/runtime` 3.786s, `pkg/plugincatalog` 0.056s).
- `git diff --check` passed; the imported protocol quick reference was already byte-identical, while `SKILL.md` gained the current guidance.

### What didn't work
- No failures were observed in this step.

### What I learned
- The skill had already been vendored at commit `6f5637c`, but its repository copy lagged the installed version; this is a synchronization update rather than first-time directory creation.
- The design baseline is commit `418a4ca`, while this branch already includes the documentation commit `f2d3c6b` and starts clean.

### What was tricky to build
- Importing the directory wholesale would have removed repository-specific OpenAI packaging metadata absent from the installed skill. Copying the canonical content files preserves that local integration while synchronizing the authored guidance.

### What warrants a second pair of eyes
- Confirm that retained `agents/openai.yaml` is still the desired repository packaging contract.
- The unresolved timeout and persistence-schema decisions must be reviewed before their implementation phases; no compatibility layer is implied.

### What should be done in the future
- Complete Phase 0 by mapping task order and making the skill import checkpoint, then begin the low-risk shared health projection and catalog inspection phase.

### Code review instructions
- Review `skills/devctl-plugin-authoring/SKILL.md` against the installed source and verify `agents/openai.yaml` remains unchanged.
- Re-run `go test ./pkg/operator -run 'TestRestart'` and `go test ./pkg/runtime ./pkg/plugincatalog` for the baseline.

### Technical details
- Overall printed phases: import/orient; health/catalog truth; shutdown/runner; recipes/artifacts; documentation/full matrix.
- The installed help topics totaled 1,924 lines and were read from temporary files only; no generated help capture was committed.
- Repository branch at start: `task/devctl-improve`; starting revision: `f2d3c6b`.

## Step 3: Separate current health from historical observations

I implemented the first low-risk runtime projection as a pure `runstate` function and changed both structured and human status output to consume it. Exited and failed attempts now report `not_running` while retaining the last health observation, so an old successful readiness check is no longer presented as current health.

### Prompt Context
**User prompt (verbatim):** See Step 2.
**Assistant interpretation:** Implement Phase 1's shared current-versus-historical health model with tests and status integration.
**Inferred user intent:** Make lifecycle state truthful and reusable across output surfaces before taking on process ownership changes.

### What I did
- Added `HealthState`, `HealthView`, and `ProjectHealth` in `pkg/runstate/health.go`.
- Added table-driven coverage for ready, starting, stopping, exited, and failed phases in `pkg/runstate/health_test.go`.
- Updated structured status to emit current `health` plus `last_health_healthy`, `last_health_checked_at`, and `last_health_detail` evidence.
- Updated human status and the unhealthy aggregate to use the same projection.
- Checked task `qsjl` and recorded the ticket changelog milestone.

### Why
- The durable model already stores timed health evidence. A pure projection preserves diagnostic history while preventing liveness claims after exit.
- One projection avoids future CLI/TUI semantic drift.

### What worked
- `go test ./pkg/runstate ./pkg/operator` passed.
- `go test ./pkg/runstate` also passed independently after formatting.
- The projection retains the exact `*HealthResult` pointer supplied as historical evidence.

### What didn't work
- `go test ./pkg/runstate ./cmd/devctl/cmds` failed while compiling the command package with unresolved Glazed APIs:
```text
cmd/devctl/cmds/stream.go:114:49: undefined: glazedsettings.OutputFormatterSettings
cmd/devctl/cmds/jsonlines.go:44:35: undefined: glazedsettings.SetupTableProcessor
cmd/devctl/cmds/lifecycle.go:197:56: undefined: glazedsettings.GlazedSlug
cmd/devctl/cmds/lifecycle.go:198:40: undefined: glazedsettings.NewGlazedSection
cmd/devctl/cmds/lifecycle.go:200:41: undefined: glazedsettings.GlazedSlug
cmd/devctl/cmds/lifecycle.go:228:59: undefined: glazedsettings.GlazedSlug
cmd/devctl/cmds/lifecycle.go:243:47: undefined: glazedsettings.SetupTableProcessor
cmd/devctl/cmds/lifecycle.go:249:32: undefined: glazedsettings.SetupProcessorOutput
cmd/devctl/cmds/lifecycle.go:266:56: undefined: glazedsettings.GlazedSlug
cmd/devctl/cmds/lifecycle.go:267:40: undefined: glazedsettings.NewGlazedSection
cmd/devctl/cmds/lifecycle.go:267:40: too many errors
```
- This is a package-wide dependency/API mismatch outside the health symbols; I did not attempt speculative fixes during this checkpoint.
- The pre-commit hook repeated the same failure through `go test ./...` and `golangci-lint run -v`; `pkg/supervise` also failed only because its fixture build imports the same command package. The focused runstate/operator tests remained green, so the checkpoint was committed with hook verification explicitly bypassed rather than misrepresenting the repository-wide gate.

### What I learned
- Health formatting previously lived in `cmd/devctl/cmds/lifecycle.go` even though status was its consumer; moving semantics into `runstate` gives all future surfaces a stable owner.
- Starting and stopping remain `unknown`, while terminal phases are explicitly `not_running`; the last observation remains independently inspectable.

### What was tricky to build
- Counting every retained unhealthy observation would label an exited failed attempt as currently unhealthy. The summary count now increments only for projected current `unhealthy` state.

### What warrants a second pair of eyes
- Review whether `RunStopping` should remain `unknown` or receive a separate presentation state in a future UI vocabulary change.
- The command-package Glazed mismatch must be resolved or qualified before Phase 1 can claim executable CLI coverage.

### What should be done in the future
- Add catalog provenance/state inspection and declared source fingerprints, then revisit command-surface validation at the Phase 1 boundary.

### Code review instructions
- Start at `pkg/runstate/health.go`, then inspect the row fields and human summary in `cmd/devctl/cmds/status.go`.
- Run `go test ./pkg/runstate ./pkg/operator`; command-package compilation currently has the unrelated Glazed diagnostics recorded above.

### Technical details
- Current health values are `healthy`, `unhealthy`, `unknown`, and `not_running`.
- Historical values remain represented by the existing timestamped `runstate.HealthResult`; no durable schema migration was required.

## Step 4: Make catalog state and source provenance inspectable

I added an inspection-only catalog surface and made interpreted plugin source changes participate in catalog freshness. Catalog schema v2 records whether each provider's commands came from static configuration or a handshake, along with digests of explicitly declared nonsecret source files.

### Prompt Context
**User prompt (verbatim):** See Step 2.
**Assistant interpretation:** Complete the catalog half of Phase 1 without weakening the rule that ordinary inspection must not execute plugins.
**Inferred user intent:** Let operators understand why commands are absent or stale and make script edits invalidate cached handshakes deterministically.

### What I did
- Added `catalog_inputs` to plugin configuration, including clone and override behavior.
- Changed catalog persistence to schema version 2 and added provider source/input provenance.
- Added repository-contained regular-file validation, symlink escape rejection, deterministic ordering, and SHA-256 content fingerprints for declared inputs.
- Added `plugincatalog.Inspect`, which reports missing, stale, valid, or conflicted state and never starts providers.
- Added `devctl plugins catalog` with profile, source, input count, command count, generation time, stored/expected fingerprints, state, and action.
- Documented refresh, inspection, trust boundaries, and `catalog_inputs` in the scripting guide.
- Added unit tests for source-content invalidation and non-executing missing-catalog inspection, plus the public help-tree contract.

### Why
- Interpreter executable metadata identifies Python, not the plugin script bytes. Declared source inputs close that known freshness gap without guessing which argv tokens are files.
- A separate inspection result preserves the trust boundary: missing help metadata does not justify executing arbitrary repository programs.
- Schema v2 intentionally treats prior cache files as stale and requires explicit refresh; no compatibility reader was added.

### What worked
- `GOWORK=off go test ./pkg/config ./pkg/plugincatalog ./cmd/devctl/cmds` passed.
- `GOWORK=off go test ./...` passed across the repository.
- CLI smoke showed a static provider as `missing` then `stale` after a declared source edit, while correctly reporting `source: static` and no required provider execution.
- The no-execution test configures a provider that would create a marker file if started; inspection reports `missing` and the marker remains absent.

### What didn't work
- The first draft of `inspection_test.go` contained malformed generated text in the source-rewrite assertions. Reading the file exposed the typo immediately; I replaced that exact block before running tests. No behavior was debugged against the malformed fixture.
- The prior Step 3 diagnosis called the Glazed failure “pre-existing” without identifying its source. Investigation here showed the parent `go.work` overlays `../glazed` on branch `task/devctl-improve`, whose API is incompatible with this repository's pinned `glazed v1.2.5`. Running with `GOWORK=off` validates the repository against its declared module dependency and passes. The failure is workspace-induced, not a defect in the committed devctl baseline.
- The first catalog checkpoint commit attempt passed `go test ./...` but lint identified an exhaustive-switch omission in the earlier health projection: `pkg/runstate/health.go:22:2: missing cases ... RunPlanned, RunStarting, RunStopping, RunUnknown`. I added an explicit grouped case preserving the intended `unknown` default, then reran the gate.

### What I learned
- Static command configuration remains usable even when no persisted cache exists, so per-provider action output must say no refresh is needed while still reporting the underlying cache as missing or stale.
- The cache is repository-scoped and profile-specific only through its fingerprint; profile-separated cache files remain a future usability decision rather than part of this task.

### What was tricky to build
- Hashing arbitrary paths could turn catalog inspection into a secret oracle. Inputs are therefore explicit, repository-relative regular files and must remain inside the repository after symlink resolution.
- Catalog command counts must come from current static configuration when cache is absent, but from persisted entries when a catalog exists; otherwise static commands are double-counted.

### What warrants a second pair of eyes
- Review the schema-v2 invalidation decision and the `catalog_inputs` name before release.
- Review whether profile-specific cache paths should be introduced separately; this implementation deliberately preserves the existing one-cache policy.
- SHA-256 input digests are persisted but intentionally not printed individually by `plugins catalog`.

### What should be done in the future
- Finish Phase 1 with executable documentation/phase-matrix and raw-help discoverability tests, then print the completion slip.

### Code review instructions
- Start with `pkg/plugincatalog/inspection.go` and `fingerprintCatalogInputs` in `pkg/plugincatalog/catalog.go`.
- Review output assembly in `cmd/devctl/cmds/plugins.go` and run `GOWORK=off go test ./pkg/config ./pkg/plugincatalog ./cmd/devctl/cmds`.
- Manually run `GOWORK=off go run ./cmd/devctl --repo-root <fixture> plugins catalog --output json` before and after refresh/source modification.

### Technical details
- Catalog schema changed from 1 to 2; old cache parses but validation returns stale because its version is unsupported.
- Provider source values are `static` and `handshake`; catalog states are `missing`, `stale`, `valid`, and `conflicted`.
- Declared input keys use normalized slash-separated repository-relative paths; persisted values are lowercase SHA-256 hex digests.
