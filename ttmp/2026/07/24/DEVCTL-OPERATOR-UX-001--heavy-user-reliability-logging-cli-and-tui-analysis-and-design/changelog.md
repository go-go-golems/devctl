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
