# Changelog

## 2026-01-07

- Initial workspace created


## 2026-01-07

Created textbook stream analysis (protocol/runtime/TUI integration) and uploaded PDF to reMarkable.

### Related Files

- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/ttmp/2026/01/07/MO-011-IMPLEMENT-STREAMS--implement-streams/analysis/01-streams-codebase-analysis-and-tui-integration.md — Primary deliverable; also uploaded as PDF


## 2026-01-07

Added design doc for telemetry stream plugin shape, UIStreamRunner, and devctl stream CLI.

### Related Files

- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/ttmp/2026/01/07/MO-011-IMPLEMENT-STREAMS--implement-streams/design-doc/01-streams-telemetry-plugin-uistreamrunner-and-devctl-stream-cli.md — Design for stream subsystem (TUI+CLI)


## 2026-01-07

Added implementation task breakdown for streams (UIStreamRunner + devctl stream CLI + fixtures).

### Related Files

- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/ttmp/2026/01/07/MO-011-IMPLEMENT-STREAMS--implement-streams/tasks.md — Stream implementation tasks


## 2026-01-07

Step 2: Make StartStream fail fast when op not declared in capabilities.ops (commit a2013d4).

### Related Files

- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/pkg/runtime/client.go — StartStream now gates on capabilities.ops
- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/pkg/runtime/runtime_test.go — Added StartStream unsupported fail-fast test


## 2026-01-07

Step 3: Added telemetry and negative stream fixtures + runtime tests (commit 25819fd).

### Related Files

- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/pkg/runtime/runtime_test.go — Tests for telemetry fixture + streams-only invocation gating
- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/testdata/plugins/streams-only-never-respond/plugin.py — Streams-only advertisement fixture (never responds)
- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/testdata/plugins/telemetry/plugin.py — Deterministic telemetry.stream fixture

