# Changelog

## 2026-01-08

- Initial workspace created


## 2026-01-08

Create ticket workspace; inventory devctl CLI verbs/flags; draft Glazed port plan and per-command flag mapping

### Related Files

- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/ttmp/2026/01/08/MO-012-PORT-CMDS-TO-GLAZED--port-devctl-cli-commands-to-glazed/analysis/01-devctl-cli-verb-inventory-and-porting-plan-to-glazed.md — Exhaustive mapping doc
- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/ttmp/2026/01/08/MO-012-PORT-CMDS-TO-GLAZED--port-devctl-cli-commands-to-glazed/reference/01-diary.md — Work diary
- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/ttmp/2026/01/08/MO-012-PORT-CMDS-TO-GLAZED--port-devctl-cli-commands-to-glazed/tasks.md — Implementation task list


## 2026-01-08

Decision: move smoketest* under dev-only group (devctl dev smoketest ...); update MO-012 plan/tasks accordingly

### Related Files

- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/ttmp/2026/01/08/MO-012-PORT-CMDS-TO-GLAZED--port-devctl-cli-commands-to-glazed/analysis/01-devctl-cli-verb-inventory-and-porting-plan-to-glazed.md — Updated smoketest command shape in port plan
- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/ttmp/2026/01/08/MO-012-PORT-CMDS-TO-GLAZED--port-devctl-cli-commands-to-glazed/reference/01-diary.md — Recorded decision + next implementation steps
- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/ttmp/2026/01/08/MO-012-PORT-CMDS-TO-GLAZED--port-devctl-cli-commands-to-glazed/tasks.md — Added smoketest refactor + call-site update tasks


## 2026-01-08

Smoketests: move to hidden dev group (devctl dev smoketest ...); update CI/docs; no smoketest-* aliases (commit b27aec4)

### Related Files

- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/.github/workflows/push.yml — CI uses new smoketest paths
- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/cmd/devctl/cmds/dev/root.go — Hidden dev command group
- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/cmd/devctl/cmds/dev/smoketest/root.go — Smoketest group root + ping
- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/cmd/devctl/cmds/root.go — Register dev group
- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/pkg/doc/topics/devctl-plugin-authoring.md — Docs updated to new smoketest paths
- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/ttmp/2026/01/08/MO-012-PORT-CMDS-TO-GLAZED--port-devctl-cli-commands-to-glazed/reference/01-diary.md — Diary Step 4

