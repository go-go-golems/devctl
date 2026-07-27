# Changelog

## 2026-01-06

- Initial workspace created


## 2026-01-06

Implement event source+level metadata and render [source] prefix (commit 060bd82)

### Related Files

- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/pkg/tui/domain.go — Add LogLevel and EventLogEntry Source/Level
- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/pkg/tui/models/eventlog_model.go — Render log level icon + [source] prefix
- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/pkg/tui/transform.go — Assign Source/Level for domain events


## 2026-01-06

Add Events view source/level filters with status bars and level menu (commit 4ecdb3e)

### Related Files

- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/pkg/tui/models/eventlog_model.go — Service+level filter state


## 2026-01-06

Add deep-dive analysis of wrapper ready-file failure caused by dynamic plugin command discovery timeouts

### Related Files

- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/cmd/devctl/cmds/dynamic_commands.go — commands.list call blocks wrapper
- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/cmd/devctl/main.go — Wrapper startup impacted by AddDynamicPluginCommands
- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/pkg/supervise/supervisor.go — Hard-coded 2s wrapper ready wait


## 2026-01-07

Document capability checking and safe plugin invocation; identify unguarded commands.list/command.run and stream risks

### Related Files

- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/cmd/devctl/cmds/dynamic_commands.go — Calls ops without SupportsOp gating
- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/pkg/runtime/client.go — Call/StartStream rely on ctx timeouts if plugin ignores requests


## 2026-07-24

Superseded by DEVCTL-OPERATOR-UX-001 for the operator TUI feature set; historical work remains recorded here.

