# Changelog

## 2026-01-08

- Initial workspace created


## 2026-01-08

Step 1: scope stream/action runner lifetimes to the TUI context (commit 1cfee17)

### Related Files

- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/cmd/devctl/cmds/tui.go — Passes TUI context into runner registration
- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/pkg/tui/action_runner.go — Action runner now uses TUI context
- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/pkg/tui/stream_runner.go — TUI-scoped stream context and bounded cleanup helper


## 2026-01-08

Step 2: bind Bubbletea program lifecycle to the TUI context (commit 1cfee17)

### Related Files

- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/cmd/devctl/cmds/tui.go — Adds tea.WithContext to program options
