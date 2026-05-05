---
doc_type: reference
title: Investigation Diary
status: active
intent: long-term
topics:
  - devctl
  - profiles
  - architecture
  - pinocchio
  - plugins
created_at: 2026-05-04
updated_at: 2026-05-04
---

# Investigation Diary

## Goal

Chronological record of the investigation and design process for adding multiple profiles support to devctl. Captures what was examined, what was learned, what worked, and what the key decisions are.

---

## Step 1: Codebase Discovery and Architecture Mapping

Investigated the full devctl and pinocchio codebases to understand current architecture, plugin loading, and the existing profile system in pinocchio/geppetto.

### Prompt Context

**User prompt (verbatim):** "Create a new docmgr ticket to examine and implement multiple profiles for devctl. Currently devctl only has a single path, so if for example I want to run the service in production mode, I can't easily switch things to use developer mode. Or maybe I have different services I might or might not want to run. Examine how we can add the concept of different profiles for devctl where each profile can load a different set of plugins or do other different things. We also have the concept of profiles in pinocchio in a pretty advanced manner, and it might be worth inspecting how that is handled when loading them and executing different commands..."

**Assistant interpretation:** Investigate devctl's current single-config architecture, study pinocchio's profile system for reusable patterns, and produce a comprehensive design doc that a new intern could use to implement profiles in devctl.

**Inferred user intent:** Create a thorough onboarding-level design document that both analyzes the problem and provides a concrete implementation roadmap, drawing lessons from pinocchio's more mature profile system.

### What I did

1. Mapped the full devctl codebase: `config.go`, `discovery.go`, `repository.go`, `engine/pipeline.go`, `runtime/client.go`, `runtime/factory.go`, `protocol/types.go`, all CLI commands.
2. Mapped the pinocchio profile system: `configdoc/types.go`, `configdoc/profiles.go`, `configdoc/merge.go`, `configdoc/load.go`, `profilebootstrap/profile_selection.go`.
3. Mapped the geppetto engine profiles library: `engineprofiles/types.go`, `stack_resolver.go`, `stack_merge.go`, `registry.go`, `store.go`.
4. Mapped the web-chat profile resolver and API: `profiles/resolver.go`, `profiles/api.go`, `profiles/types.go`.
5. Created docmgr ticket DCTL-PROFILES with design doc and diary.
6. Related 25 key source files to the design doc.

### What I learned

- devctl currently has a flat config model: `.devctl.yaml` → list of plugins → all loaded unconditionally.
- There is no concept of "mode" or "profile" — every plugin runs on every `devctl up`.
- Pinocchio's profile system is deep: layered config docs, stack composition (DFS with cycle detection), inline + external registries, store abstraction (in-memory, SQLite, YAML file).
- The geppetto `engineprofiles` package is generic enough to be reused but is heavily tied to `InferenceSettings` (AI model config). Devctl would need a different "settings" type.
- The pinocchio web-chat shows profiles resolved per-request (cookie/query/body), not just at CLI startup. This is a more dynamic model than devctl needs.
- Key devctl plugin protocol ops: `config.mutate`, `launch.plan`, `validate.run`, `build.run`, `prepare.run`, `command.run`.
- Dynamic commands are discovered at startup by starting all plugins and reading their handshakes. This is where profiles could filter which plugins to start.

### What was tricky to build

- Understanding the full profile resolution chain in pinocchio (config files → documents → merge → registry → stack resolve → merge layers → effective profile) took careful reading across 6+ files.
- The geppetto `engineprofiles` package mixes generic patterns (registry, store, slug, stack) with domain-specific concerns (`InferenceSettings` merge). Reuse would require factoring out the generic parts.

### What should be done in the future

- Implement the profile system based on the design doc.
- Consider contributing a generic "profile stack" library back to geppetto if devctl's profile needs stabilize.

---

## Step 2: Design Document Written and Uploaded

Wrote the full 12-section design document covering executive summary, problem statement, current-state architecture (with code references), pinocchio profile system analysis, gap analysis, proposed architecture with config model changes, profile resolution flow, plugin filtering, CLI changes, state file changes, API references with Go pseudocode, ASCII diagrams, 6-phase implementation plan, test strategy, risks/alternatives/open questions, and a complete file reference index. Uploaded to reMarkable as a bundled PDF.

### Prompt Context

**User prompt (verbatim):** (see Step 1)

**Assistant interpretation:** Continue the investigation by producing the full design document in sections.

**Inferred user intent:** Create an intern-ready guide that is comprehensive enough to hand off for implementation.

### What I did

1. Wrote Section 1-2 (Executive Summary, Problem Statement) — defines the core problem and success criteria.
2. Wrote Section 3 (Current-State Architecture) — exhaustive walkthrough of devctl's config, discovery, pipeline, protocol, CLI, and state with code references to every relevant file.
3. Wrote Section 4 (Pinocchio Reference) — analysis of pinocchio's profile system: Document model, inline vs registry profiles, stack composition, resolution chain, web-chat per-request selection, and a lesson table mapping pinocchio patterns to devctl applicability.
4. Wrote Section 5-6 (Gap Analysis, Proposed Architecture) — 6 specific gaps mapped to code changes; proposed config model with ProfileBlock/Profile types; filtering flow; CLI/state/dynamic command changes.
5. Wrote Section 7 (API References) — Go pseudocode for config types, profile resolver, repository filtering, pipeline approach.
6. Wrote Section 8 (Diagrams) — 4 ASCII diagrams showing current vs proposed architecture, resolution decision tree, state file format.
7. Wrote Section 9-12 (Implementation Phases, Test Strategy, Risks, File Index) — 6 phases with file-level guidance; test matrix; 5 open questions; complete file reference tables.
8. Ran docmgr doctor — all checks pass.
9. Uploaded to reMarkable at `/ai/2026/05/04/DCTL-PROFILES`.

### What worked

- Writing the doc in small chunks (as requested by the user) kept each write manageable.
- The pinocchio comparison provided a rich reference architecture without over-engineering the devctl design.
- docmgr doctor caught vocabulary issues immediately; easy to fix.

### What didn't work

- N/A — smooth workflow.

### What was tricky to build

- Deciding how much of pinocchio's profile system to reuse vs. implement from scratch. The geppetto engineprofiles library is generic in concept but tightly coupled to InferenceSettings. The decision to implement a simpler inline-only model for devctl v1 was clear once I mapped the applicability table.

### What warrants a second pair of eyes

- The profile env override semantics (profile-level vs per-plugin). Currently profile `env:` overrides plugin `env:`. This might surprise users.
- The dynamic commands behavior with profiles — should commands from inactive plugins still appear?

### What should be done in the future

- Implement Phases 1-6 as described in the design doc.
- Revisit open questions before implementation begins.

### Code review instructions

- Read the design doc start to finish: `devctl/ttmp/2026/05/04/DCTL-PROFILES--devctl-multiple-profiles-analysis-design-and-implementation-guide/design-doc/01-multiple-profiles-for-devctl-analysis-design-and-implementation-guide.md`
- Verify the pinocchio comparison is accurate by spot-checking against the source files listed in Section 12.
- Verify the proposed config model is backward-compatible by checking `config.LoadOptional()` and `discovery.Discover()` behavior.

### Technical details

- Bundle upload command: `remarquee upload bundle <design-doc> <diary> --name "DCTL-PROFILES: devctl Multiple Profiles Design" --remote-dir /ai/2026/05/04/DCTL-PROFILES --toc-depth 2`
- docmgr doctor: all checks pass after adding vocabulary entries for architecture, devctl, pinocchio, plugins, profiles.

## Step 4: Implementation Started — Task Breakdown

The implementation phase starts from the v2 design: profiles live in `.devctl.yaml`, a local `.devctl.override.yaml` can add or adjust profiles, `--profile` is the highest-precedence explicit selection, and a profile named `default` is not implicit.

### Prompt Context

**User prompt (verbatim):** "Ok, now add tasks, and implement task by task, commit at appropriat eintervals, and keeping a detailed diary"

**Assistant interpretation:** Turn the design into an implementation checklist, then implement it in phases with commits after coherent working increments.

### Current working rule

Implement and commit one coherent phase at a time. The first commit should cover the config model and local override stacking because later repository, CLI, and state changes depend on that foundation.


## Step 5: Phase 1 Implemented — Config Model and Override Stacking

Implemented the config-layer foundation for profiles. This phase does not yet filter plugins in repository loading or expose a CLI flag. It only teaches `pkg/config` how to parse profile fields, merge a local override file, resolve the active profile name, and validate profile plugin references.

### What changed

- Added `ProfileBlock` and `Profile` to `pkg/config/config.go`.
- Added `DefaultOverrideFilename` and `DefaultOverridePath()` for `.devctl.override.yaml`.
- Added `LoadStacked(basePath, overridePath)` to load `.devctl.yaml` and then optionally merge `.devctl.override.yaml`.
- Added `Merge()` and clone helpers so the merged config is a new value rather than a mutation of the base config.
- Added `ResolveProfile()`, `GetProfile()`, and `ValidateProfile()`.
- Added config tests covering missing configs, missing overrides, local profile additions, existing profile adjustments, plugin-by-ID patches, explicit `default` profile selection, and validation errors.

### Design details captured in code

The `default` profile is not magic. If a config defines `profiles.default` but neither `profile.active: default` nor `--profile default` is supplied, `ResolveProfile("")` returns the empty string. Later phases will interpret the empty string as "load all top-level plugins," preserving backward compatibility.

The local override merge follows the design doc:

- non-empty scalar values override base values,
- maps merge with override keys winning,
- profile records merge field-by-field,
- profile `plugins` lists replace when provided,
- top-level plugins merge by `id`,
- plugin env maps merge.

### Validation

Ran:

```bash
go test ./pkg/config -count=1
```

Result: passed.


## Step 6: Phase 2 Implemented — Repository Profile Filtering

Implemented repository-level profile filtering. This is the first point where profiles affect runtime behavior: `repository.Load()` now loads the stacked config, resolves the active profile name, filters discovered plugin specs, and merges profile env into selected specs.

### What changed

- Added `ProfileName` and `OverridePath` to `repository.Options`.
- Added `ProfileName`, `Profile`, and `OverrideAbs` to `repository.Repository`.
- Changed `repository.Load()` to call `config.LoadStacked()` instead of `config.LoadOptional()`.
- Added filtering after discovery, before `SpecByID` is built and before plugin clients are started.
- Added a clear error for profiles that reference unknown discovered plugin IDs.
- Added tests for no-profile backward compatibility, explicit `default`, profile env merging, override-defined local profiles, plugin patching, and unknown plugin errors.

### Why filtering lives here

The pipeline should not know about profiles. It already operates on a list of plugin clients. The repository loader is the correct boundary because it is responsible for turning config and discovery into the concrete plugin specs that later phases start.

### Notable behavior

A profile named `default` remains explicit. A config with `profiles.default` but no active profile still loads all top-level plugins. Passing `ProfileName: "default"` filters to that profile.

Profile-level env currently overlays plugin env for selected plugins. This matches the v2 design: profile env is a mode-level adjustment and wins over the plugin's base env for that mode.

### Validation

Ran:

```bash
go test ./pkg/repository ./pkg/config -count=1
```

Result: passed.


## Step 7: Phase 3 Implemented — CLI Profile Plumbing

Threaded profile selection through the shared CLI context and all repository-loading command paths.

### What changed

- Added shared `--profile` to the repo flag section in `cmd/devctl/cmds/common.go`.
- Added `Profile` to `RepoSettings`, `RepoContext`, `rootOptions`, and TUI `RootOptions`.
- Passed `ProfileName` into `repository.Load()` from `up`, `plan`, `stream`, `plugins list`, dynamic command discovery, and TUI runners.
- Passed `ProfileName` into `servicecontrol.ResolveServiceSpec()` so `start` and `restart` re-plan using the same selected profile.
- Added `devctl profiles list` and `devctl profiles active`.
- Updated dynamic command bootstrap parsing to understand `--profile`, so dynamic commands are discovered from the selected profile's plugins.
- Updated dynamic command strictness loading to use the stacked config instead of only `.devctl.yaml`.

### Behavior notes

`devctl profiles active` prints `(none)` when no profile is selected. This is intentional: `(none)` means backward-compatible all-top-level-plugins mode, not the `default` profile.

Dynamic command discovery now obeys `--profile`. If a plugin is excluded by the active profile, its dynamic commands are not registered for that invocation.

### Validation

Ran:

```bash
go test ./... -count=1
```

Result: passed.


## Step 8: Phase 4 Implemented — Profile Recording in State

Recorded the resolved profile in `.devctl/state.json` and surfaced it through status/down output.

### What changed

- Added `Profile string` to `state.State` with `json:"profile,omitempty"` for backward-compatible state files.
- Set `st.Profile = repo.ProfileName` after `Supervisor.Start()` in `devctl up`, before saving state.
- Included `profile` in `devctl status` JSON output.
- Made `devctl down` include the profile in its final confirmation when the state was profile-backed.
- Extended the state save/load test to verify profile round-tripping.

### Compatibility

Old state files do not have a `profile` field. JSON unmarshalling leaves the field as `""`, which is the same value used for no-profile/all-top-level-plugins mode.

### Validation

Ran:

```bash
go test ./pkg/state ./cmd/devctl/cmds -count=1
```

Result: passed.

