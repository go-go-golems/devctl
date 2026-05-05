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
