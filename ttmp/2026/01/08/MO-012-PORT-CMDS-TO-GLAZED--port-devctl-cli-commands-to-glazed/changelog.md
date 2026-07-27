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


## 2026-01-08

Docs: add Cobra↔Glazed porting friction report (improvement backlog for persistent layers, precedence, parameter types, dynamic commands)

### Related Files

- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/ttmp/2026/01/08/MO-012-PORT-CMDS-TO-GLAZED--port-devctl-cli-commands-to-glazed/analysis/02-cobra-glazed-porting-friction-report.md — Exhaustive report + proposed improvements
- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/ttmp/2026/01/08/MO-012-PORT-CMDS-TO-GLAZED--port-devctl-cli-commands-to-glazed/reference/01-diary.md — Diary Step 5


## 2026-01-08

Repo context flags are now command-local (no root persistent flags) (commit b465e03)

### Related Files

- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/cmd/devctl/cmds/common.go — Repo flag layer + normalization (command-local)
- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/cmd/devctl/cmds/dynamic_commands.go — Ensure dynamic commands also get repo flags


## 2026-01-08

Add Glazed help system + embed devctl docs; port status to Glazed (commit 2b209b2)

### Related Files

- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/cmd/devctl/cmds/status.go — Status ported to Glazed WriterCommand
- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/cmd/devctl/main.go — Wire help system and doc embedding into devctl root
- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/pkg/doc/doc.go — Embed Markdown topics for help


## 2026-01-08

Update MO-012 plan/tasks; add testing diary and record validations (commit 012b0ca)

### Related Files

- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/ttmp/2026/01/08/MO-012-PORT-CMDS-TO-GLAZED--port-devctl-cli-commands-to-glazed/reference/02-testing-diary.md — Recorded go test + smoketest runs and failures verbatim
- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/ttmp/2026/01/08/MO-012-PORT-CMDS-TO-GLAZED--port-devctl-cli-commands-to-glazed/tasks.md — Checked off help/status/tests; added exhaustive validation backlog


## 2026-01-08

Port plugins list to Glazed and add intern porting playbook

### Related Files

- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/cmd/devctl/cmds/plugins.go — plugins list is now a Glazed WriterCommand
- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/ttmp/2026/01/08/MO-012-PORT-CMDS-TO-GLAZED--port-devctl-cli-commands-to-glazed/playbook/01-playbook-port-a-devctl-cobra-verb-to-a-glazed-command.md — Intern-facing porting procedure and gotchas


## 2026-07-24

Superseded by DEVCTL-OPERATOR-UX-001 for public operator command output contracts and Glazed migration.

