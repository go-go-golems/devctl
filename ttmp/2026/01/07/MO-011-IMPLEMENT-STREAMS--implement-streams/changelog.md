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


## 2026-01-07

Step 4: Added TUI stream message plumbing (topics, transformer, forwarder) (commit 472593f).

### Related Files

- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/pkg/tui/forward.go — Forwards stream UI messages into Bubble Tea
- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/pkg/tui/models/root_model.go — Publishes stream start/stop requests via bus
- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/pkg/tui/topics.go — New stream-related domain/UI envelope types
- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/pkg/tui/transform.go — Maps stream domain events to UI messages (logs only start/end)


## 2026-01-07

Step 5: Implemented UIStreamRunner and wired it into devctl tui startup (commit e0db4d5).

### Related Files

- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/cmd/devctl/cmds/tui.go — RegisterUIStreamRunner wired into TUI startup
- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/pkg/tui/stream_runner.go — Centralized stream lifecycle management (start/stop

