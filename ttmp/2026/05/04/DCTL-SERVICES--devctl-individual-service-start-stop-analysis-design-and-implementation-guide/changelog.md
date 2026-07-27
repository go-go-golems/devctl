# Changelog

## 2026-05-04

- Initial workspace created


## 2026-05-04

Completed investigation and design doc for individual service start/stop. Identified 5 core problems (lost provenance, config mutation correlation, atomic state, no single-service supervisor ops, health check deps). Proposed kill-and-respawn approach with stored ServiceSpec. 12-section design doc with pseudocode, diagrams, and 5-phase implementation plan. Uploaded to reMarkable.

### Related Files

- /home/manuel/workspaces/2026-05-04/devctl-multiple-profiles/devctl/ttmp/2026/05/04/DCTL-SERVICES--devctl-individual-service-start-stop-analysis-design-and-implementation-guide/design-doc/01-individual-service-start-stop-analysis-design-and-implementation-guide.md — Primary design document
- /home/manuel/workspaces/2026-05-04/devctl-multiple-profiles/devctl/ttmp/2026/05/04/DCTL-SERVICES--devctl-individual-service-start-stop-analysis-design-and-implementation-guide/reference/01-investigation-diary.md — Investigation diary


## 2026-05-04

Implemented all 5 phases: Spec storage in state (70d90b0), single-service supervisor ops (8a41a37), CLI restart/stop-service commands (33ca243), TUI ActionStop/ActionRestart implementation (9b81ffb), state tests with backward compat (d2d09e2). All tests pass.

### Related Files

- /home/manuel/workspaces/2026-05-04/devctl-multiple-profiles/devctl/cmd/devctl/cmds/restart.go — New restart command
- /home/manuel/workspaces/2026-05-04/devctl-multiple-profiles/devctl/cmd/devctl/cmds/stop_service.go — New stop-service command
- /home/manuel/workspaces/2026-05-04/devctl-multiple-profiles/devctl/pkg/state/state.go — Added ServiceSpecRecord
- /home/manuel/workspaces/2026-05-04/devctl-multiple-profiles/devctl/pkg/state/state_test.go — Round-trip and backward compat tests
- /home/manuel/workspaces/2026-05-04/devctl-multiple-profiles/devctl/pkg/supervise/supervisor.go — Added StopService/StartService/RestartService and helpers
- /home/manuel/workspaces/2026-05-04/devctl-multiple-profiles/devctl/pkg/tui/action_runner.go — Implemented ActionStop and per-service ActionRestart


## 2026-05-04

Added devctl start command (228fbf0). Tested all commands (restart, stop-service, start) in tmux against real multi-service environment. All worked correctly: restart gives new PID while siblings run, stop-service kills only target, start respawns from stored spec.

### Related Files

- /home/manuel/workspaces/2026-05-04/devctl-multiple-profiles/devctl/cmd/devctl/cmds/start_service.go — New start command


## 2026-05-05

Fixed CI failure in TestSupervisor_ReadinessTimeoutStopsServices (commit 10a61c1). Increased readiness timeout and passed pid-file path through env to avoid CI race where readiness timeout killed the child before pid.txt was written.

### Related Files

- /home/manuel/workspaces/2026-05-04/devctl-multiple-profiles/devctl/pkg/supervise/supervisor_test.go — Made readiness timeout leak test robust on CI


## 2026-05-05

Addressed PR #6 review comments (commit ae537e9): blocked duplicate starts, removed raw env from persisted ServiceSpecRecord, added servicecontrol.ResolveServiceSpec to re-run config.mutate + launch.plan for start/restart, updated CLI/TUI flows and tests. Documented that start/restart run the first two planning phases.

### Related Files

- /home/manuel/workspaces/2026-05-04/devctl-multiple-profiles/devctl/pkg/servicecontrol/resolve.go — Re-runs planning phases and selects target ServiceSpec
- /home/manuel/workspaces/2026-05-04/devctl-multiple-profiles/devctl/pkg/state/state.go — ServiceSpecRecord no longer persists raw env
- /home/manuel/workspaces/2026-05-04/devctl-multiple-profiles/devctl/pkg/supervise/supervisor.go — StartService now accepts fresh spec and blocks duplicate starts


## 2026-07-24

Superseded by DEVCTL-OPERATOR-UX-001 for individual-service lifecycle safety and the consolidated up/down/restart controller.

