# Changelog

## 2026-07-24

- Initial workspace created


## 2026-07-24

Step 1: established the devctl-local docmgr root, created the research ticket, and defined eleven evidence and delivery tasks

### Related Files

- /home/manuel/workspaces/2026-07-07/prod-tiny-idp/devctl/.ttmp.yaml — Prevents nested workspaces from routing devctl tickets into another repository

## 2026-07-24

Step 2: mapped supervision, wrapper process groups, state durability, health checks, and lifecycle failure paths; added reproducible architecture and CLI probes

### Related Files

- pkg/supervise/supervisor.go — Current startup, readiness, service lifecycle, and process-group termination implementation
- cmd/devctl/cmds/wrap_service.go — Wrapper-to-child signal and exit-record boundary
- pkg/state/state.go — Persistent service ownership and liveness implementation
- ttmp/2026/07/24/DEVCTL-OPERATOR-UX-001--heavy-user-reliability-logging-cli-and-tui-analysis-and-design/scripts/01-architecture-inventory.sh — Read-only architecture inventory
- ttmp/2026/07/24/DEVCTL-OPERATOR-UX-001--heavy-user-reliability-logging-cli-and-tui-analysis-and-design/scripts/02-cli-contract-probe.sh — Isolated operator-contract probe

## 2026-07-24

Step 3: audited service and protocol logs, CLI contracts, the six-view TUI, duplicated control paths, historical ticket drift, and reproducible log/TUI failures

### Related Files

- cmd/devctl/cmds/logs.go — Current single-file log CLI and follow loop
- pkg/tui/action_runner.go — Duplicated TUI lifecycle control plane
- pkg/tui/state_watcher.go — Polling, health, process stats, and plugin introspection
- pkg/tui/transform.go — Snapshot-to-event flood and JSON message transform
- pkg/tui/models/root_model.go — Six-view routing and text-derived status
- pkg/tui/models/service_model.go — Independent TUI log reader
- pkg/logjs/module.go — Standalone JavaScript parser subsystem evaluated for removal
- ttmp/2026/07/24/DEVCTL-OPERATOR-UX-001--heavy-user-reliability-logging-cli-and-tui-analysis-and-design/scripts/03-log-follow-lifecycle-probe.sh — Append/truncate/rotation reproduction
- ttmp/2026/07/24/DEVCTL-OPERATOR-UX-001--heavy-user-reliability-logging-cli-and-tui-analysis-and-design/scripts/04-tui-state-event-probe.sh — Tmux capture of repeated snapshot events

## 2026-07-24

Step 4: compared official Process Compose, Docker Compose, Tilt, and journalctl operator contracts; saved six web sources and defined adopted and rejected patterns

### Related Files

- ttmp/2026/07/24/DEVCTL-OPERATOR-UX-001--heavy-user-reliability-logging-cli-and-tui-analysis-and-design/sources/README.md — External-source provenance and research questions
- ttmp/2026/07/24/DEVCTL-OPERATOR-UX-001--heavy-user-reliability-logging-cli-and-tui-analysis-and-design/sources/web/01-process-compose-tui.md — Status/log/action TUI precedent
- ttmp/2026/07/24/DEVCTL-OPERATOR-UX-001--heavy-user-reliability-logging-cli-and-tui-analysis-and-design/sources/web/04-docker-compose-logs.md — Multi-service log CLI precedent
- ttmp/2026/07/24/DEVCTL-OPERATOR-UX-001--heavy-user-reliability-logging-cli-and-tui-analysis-and-design/sources/web/06-tilt-logs.md — Structured development-log CLI precedent

## 2026-07-24

Step 5: completed the target state/run/log/controller architecture, breaking CLI and reduced TUI specifications, and seven-phase intern implementation and verification guide

### Related Files

- pkg/state/state.go — Current state implementation mapped to the proposed runstate replacement
- pkg/supervise/supervisor.go — Current lifecycle implementation mapped to proposed supervisor primitives
- cmd/devctl/cmds/common.go — Existing shared Glazed repository section retained in the design
- pkg/tui/models/root_model.go — Current six-view root mapped to the proposed three-view typed model

## 2026-07-24

Step 6: related eleven source files and seven external references, passed docmgr doctor and go test ./..., and uploaded the primary guide to reMarkable

### Delivery

- Document: DEVCTL-OPERATOR-UX-001 - devctl Heavy-User Operator Redesign.pdf
- Remote folder: /ai/2026/07/24/DEVCTL-OPERATOR-UX-001
- Result: upload confirmed

## 2026-07-24

Research, design, validation, source relations, commits, and reMarkable delivery completed

## 2026-07-24

Revised the operator design to preserve automatic top-level plugin commands
and harden them with a validated catalog, deterministic collision handling,
zero-plugin typo/help/completion paths, explicit refresh and inspection, and
single-provider execution.

### Related Files

- cmd/devctl/cmds/dynamic_commands.go — Existing dynamic registration path retained as a product capability and redesigned for robust discovery
- pkg/protocol/types.go — Existing command and handshake schema used as the catalog contract baseline
- cmd/devctl/cmds/dynamic_commands_test.go — Existing behavior tests extended by the proposed robustness matrix

### Delivery

- Document: DEVCTL-OPERATOR-UX-001 devctl Heavy User Operator Redesign Revised.pdf
- Remote folder: /ai/2026/07/24/DEVCTL-OPERATOR-UX-001
- Rendering: default
- Result: upload confirmed

## 2026-07-24

Phase 0: archived green build test lint and UX baselines; added isolated repository fixtures; passed the external logjs consumer gate and recorded legacy CLI migrations (commit 82244d4)

### Related Files

- /home/manuel/workspaces/2026-07-07/prod-tiny-idp/devctl/internal/testrepo/repository.go — Safe repository-local test fixture
- /home/manuel/workspaces/2026-07-07/prod-tiny-idp/devctl/ttmp/2026/07/24/DEVCTL-OPERATOR-UX-001--heavy-user-reliability-logging-cli-and-tui-analysis-and-design/sources/local/phase0/README.md — Baseline and removal-gate evidence index
