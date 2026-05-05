# Changelog

## 2026-05-04

- Initial workspace created


## 2026-05-04

Completed investigation and design doc for devctl multiple profiles. Analyzed current devctl architecture, pinocchio profile system, and geppetto engine profiles library. Produced 12-section design doc with config model, filtering pipeline, CLI changes, diagrams, and 6-phase implementation plan.

### Related Files

- /home/manuel/workspaces/2026-05-04/devctl-multiple-profiles/devctl/ttmp/2026/05/04/DCTL-PROFILES--devctl-multiple-profiles-analysis-design-and-implementation-guide/design-doc/01-multiple-profiles-for-devctl-analysis-design-and-implementation-guide.md — Primary design document
- /home/manuel/workspaces/2026-05-04/devctl-multiple-profiles/devctl/ttmp/2026/05/04/DCTL-PROFILES--devctl-multiple-profiles-analysis-design-and-implementation-guide/reference/01-investigation-diary.md — Investigation diary


## 2026-05-05

Updated the multiple-profiles design to include local .devctl.override.yaml stacking. The design now loads .devctl.yaml first, merges an optional repository-local override, and then applies --profile as the highest-precedence selection. Documented merge rules for profile additions, profile adjustments, plugin-by-id patches, and local profile defaults.

### Related Files

- /home/manuel/workspaces/2026-05-04/devctl-multiple-profiles/devctl/ttmp/2026/05/04/DCTL-PROFILES--devctl-multiple-profiles-analysis-design-and-implementation-guide/design-doc/01-multiple-profiles-for-devctl-analysis-design-and-implementation-guide.md — Added local override file section and updated config/repository/test phases


## 2026-05-05

Clarified default-profile semantics: a profile named default is allowed but not implicit. It is selected only via profile.active: default or --profile default. If no profile is selected at all, devctl remains backward-compatible and loads the top-level plugins list.

### Related Files

- /home/manuel/workspaces/2026-05-04/devctl-multiple-profiles/devctl/ttmp/2026/05/04/DCTL-PROFILES--devctl-multiple-profiles-analysis-design-and-implementation-guide/design-doc/01-multiple-profiles-for-devctl-analysis-design-and-implementation-guide.md — Default profile semantics and backward compatibility tests


## 2026-05-05

Implemented devctl profile v1 task-by-task: config profile model and .devctl.override.yaml stacking, repository profile filtering, CLI --profile plumbing and profiles list/active commands, state profile recording, command tests, fixtures, and dry-run smoke validation. Full go test ./... passed.

### Related Files

- /home/manuel/workspaces/2026-05-04/devctl-multiple-profiles/devctl/cmd/devctl/cmds/profiles.go — profiles list and profiles active commands
- /home/manuel/workspaces/2026-05-04/devctl-multiple-profiles/devctl/pkg/config/config.go — Profile model
- /home/manuel/workspaces/2026-05-04/devctl-multiple-profiles/devctl/pkg/repository/repository.go — Filters discovered plugin specs by resolved profile
- /home/manuel/workspaces/2026-05-04/devctl-multiple-profiles/devctl/pkg/state/state.go — Records active profile in state

