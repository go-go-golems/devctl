# Changelog

## 2026-04-28

- Initial workspace created


## 2026-04-28

Migrate devctl to new glazed schema/fields/values API (commit 349af84)

### Related Files

- /home/manuel/workspaces/2026-04-28/update-devctl/devctl/cmd/devctl/cmds/common.go — Replaced layers/parameters with schema/fields/values
- /home/manuel/workspaces/2026-04-28/update-devctl/devctl/cmd/devctl/cmds/plugins.go — Updated RunIntoWriter signature and WithSections wiring
- /home/manuel/workspaces/2026-04-28/update-devctl/devctl/cmd/devctl/cmds/status.go — Updated RunIntoWriter signature and field definitions
- /home/manuel/workspaces/2026-04-28/update-devctl/devctl/cmd/devctl/main.go — Renamed AddLoggingLayerToRootCommand to AddLoggingSectionToRootCommand
- /home/manuel/workspaces/2026-04-28/update-devctl/devctl/cmd/log-parse/main.go — Renamed AddLoggingLayerToRootCommand to AddLoggingSectionToRootCommand

