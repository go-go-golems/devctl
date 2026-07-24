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
