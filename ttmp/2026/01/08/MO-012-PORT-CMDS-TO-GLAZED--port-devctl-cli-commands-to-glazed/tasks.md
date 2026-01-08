# Tasks

## TODO

- [ ] Root: restructure devctl main.go to Glazed style (logging layer + help system + BuildCobraCommand)
- [ ] Help: initialize HelpSystem and load devctl docs into it (pkg/doc)
- [ ] Help: wire help_cmd.SetupCobraRootCommand and ensure cobra help output is correct
- [ ] Layer: implement RepoSettings custom layer (repo-root/config/strict/dry-run/timeout)
- [ ] Layer: add normalization+validation tests for RepoSettings (abs paths, timeout>0)
- [ ] Port: status command to Glazed (tail-lines flag, JSON output)
- [ ] Port: plan command to Glazed (repo layer, JSON output, no-plugins {} behavior)
- [ ] Port: plugins list command to Glazed (handshake inventory JSON)
- [ ] Port: logs command to Glazed (service required, follow mode)
- [ ] Port: down command to Glazed (stop+remove state)
- [ ] Port: up command to Glazed (flags, interactive prompts, dry-run JSON)
- [ ] Port: stream start command to Glazed (input-json/input-file validation, raw JSON mode)
- [ ] Port: tui command wrapper to Glazed (refresh/alt-screen/debug-logs flags)
- [x] Decide: keep smoketest* as Cobra-only vs port to Glazed; document decision
- [ ] Internal: keep __wrap-service isolated from startup logic; verify no Glazed init breaks it
- [ ] Dynamic commands: design how handshake-advertised commands will be registered in Glazed root
- [ ] Dynamic commands: ensure built-in verbs do not start plugins; preserve completion semantics
- [ ] Docs: add devctl help topics for the Glazed-ported CLI surface and flags
- [ ] Validation: add parity checklist + run through fixture repos to compare outputs
- [x] Smoketest: move under hidden group 'dev smoketest' (no top-level smoketest* verbs)
- [x] Smoketest: refactor cmd layout to groups (cmd/devctl/cmds/dev/root.go + cmd/devctl/cmds/dev/smoketest/root.go + subcommands)
- [x] Smoketest: update all call sites (CI workflows, pkg/doc, scripts) from 'smoketest-*' to 'dev smoketest ...'
- [x] Smoketest: add backwards-compat decision (no shim vs temporary aliases) + document in help
