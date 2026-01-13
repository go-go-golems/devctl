# Changelog

## 2026-01-08

- Refactor moments plugin structure for readability; add `devctl logs --tail` (default 50); create MO-015 ticket for plan persistence + plugin IO debug tracing.

### Related Files

- /home/manuel/workspaces/2026-01-06/moments-dev-tool/moments/plugins/moments-plugin.py — Refactored plugin layout and shared step runner.
- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/cmd/devctl/cmds/logs.go — Add tail support for logs output.
- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/ttmp/2026/01/08/MO-015-DEVCTL-PLAN-DEBUG-TRACE--persist-launch-plan-state-plugin-io-debug-trace/index.md — New ticket for plan persistence + plugin IO debug tracing.


## 2026-01-08

- Initial workspace created


## 2026-01-08

Draft analysis + design proposal for a v2 devctl plugin to replace moments/scripts/startdev.sh.

### Related Files

- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/ttmp/2026/01/08/MO-013-PORT-STARTDEV--port-startdev-sh-to-devctl-plugin-s/design-doc/01-design-moments-devctl-plugin-s-replacing-startdev-sh.md — Implementation-ready plugin design.
- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/ttmp/2026/01/08/MO-013-PORT-STARTDEV--port-startdev-sh-to-devctl-plugin-s/working-note/01-analysis-replace-startdev-sh-with-devctl-plugin-s.md — Baseline behavior and gap analysis.


## 2026-01-08

Add initial implementation task list for the Moments devctl plugin port.

### Related Files

- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/ttmp/2026/01/08/MO-013-PORT-STARTDEV--port-startdev-sh-to-devctl-plugin-s/tasks.md — Track follow-up work from the design.


## 2026-01-08

Expand task breakdown and explicitly drop backwards compatibility with the existing Moments devctl stub.

### Related Files

- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/ttmp/2026/01/08/MO-013-PORT-STARTDEV--port-startdev-sh-to-devctl-plugin-s/design-doc/01-design-moments-devctl-plugin-s-replacing-startdev-sh.md — Design decision + Non-Goals updated.
- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/ttmp/2026/01/08/MO-013-PORT-STARTDEV--port-startdev-sh-to-devctl-plugin-s/tasks.md — More granular implementation plan.


## 2026-01-08

Implement: protocol v2 Moments devctl plugin + moments-config key lookups; deprecate startdev.sh in favor of devctl up (commits: dd45685, 285fd1d, b624b78, 5c4586a).

### Related Files

- /home/manuel/workspaces/2026-01-06/moments-dev-tool/moments/backend/Makefile — Run lint with GOWORK=off to avoid go.work issues.
- /home/manuel/workspaces/2026-01-06/moments-dev-tool/moments/backend/cmd/moments-config/bootstrap_config.go — Refactor init vs bootstrap compute; use minimal schema registrations.
- /home/manuel/workspaces/2026-01-06/moments-dev-tool/moments/backend/cmd/moments-config/get_cmd.go — Expose AppConfig keys used for VITE_* derivation.
- /home/manuel/workspaces/2026-01-06/moments-dev-tool/moments/docs/getting-started.md — Switch recommended dev startup to devctl.
- /home/manuel/workspaces/2026-01-06/moments-dev-tool/moments/plugins/moments-plugin.py — Rewrite to v2 protocol; services moments.backend/moments.web.
- /home/manuel/workspaces/2026-01-06/moments-dev-tool/moments/scripts/startdev.sh — Replace legacy behavior with deprecation wrapper calling devctl up.

## 2026-01-13

Close: tasks complete, implementation validated in diary/source

