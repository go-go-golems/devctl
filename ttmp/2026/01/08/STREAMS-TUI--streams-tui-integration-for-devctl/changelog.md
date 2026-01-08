# Changelog

## 2026-01-08

- Initial workspace created


## 2026-01-08

Fixed critical stream context bug (commit f1b1761) - streams now run to completion

### Related Files

- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/pkg/tui/stream_runner.go — Fixed context.Background() usage for stream/plugin lifecycle


## 2026-01-08

Enhanced stream row display with duration and event count (commit 946fcc3)

### Related Files

- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/pkg/tui/models/streams_model.go — Added EventCount field and enhanced renderStreamList


## 2026-01-08

Improved streams empty state with instructions (commit d50557b)

### Related Files

- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/pkg/tui/models/streams_model.go — Enhanced empty state view with help text


## 2026-01-08

Step 7: treat missing state as stopped (commit 32c537d)

### Related Files

- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/cmd/devctl/cmds/status.go — Status output treats missing state as normal
- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/pkg/tui/models/dashboard_model.go — Dashboard shows stopped when state missing
- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/pkg/tui/transform.go — UI missing state event now info


## 2026-01-08

Step 8: fix status missing-state payload scope (commit a0c50b3)

### Related Files

- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/cmd/devctl/cmds/status.go — Move svc type before missing-state return

