# Changelog

## 2026-09-13

- Initial workspace created

## 2026-09-13

Created source-grounded intern design covering six lifecycle improvements and supporting documentation/test ergonomics. Uploaded guide successfully to /ai/2026/09/13/DEVCTL-ROBUST-LIFECYCLE; runtime implementation remains open.

## 2026-09-13

Implemented a shared runstate health projection that separates current liveness from retained timed health observations; status rows and human output now use it.

### Related Files

- /home/manuel/workspaces/2026-09-13/devctl-improve/devctl/cmd/devctl/cmds/status.go — Structured and human status projection consumer
- /home/manuel/workspaces/2026-09-13/devctl-improve/devctl/pkg/runstate/health.go — Canonical current-versus-historical health projection
