# Changelog

## 2026-01-07

- Initial workspace created


## 2026-01-07

Documented devctl service supervision and reproduced comprehensive fixture up failure; captured root cause and workarounds.

### Related Files

- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/ttmp/2026/01/07/MO-010-DEVCTL-CLEANUP-PASS--devctl-cleanup-pass/analysis/01-service-supervision-architecture-events-and-ui-integration.md — Textbook-style supervision and UI interaction analysis
- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/ttmp/2026/01/07/MO-010-DEVCTL-CLEANUP-PASS--devctl-cleanup-pass/analysis/02-comprehensive-fixture-devctl-up-failure-bug-report.md — Bug report with copy/paste repro and root-cause explanation
- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/ttmp/2026/01/07/MO-010-DEVCTL-CLEANUP-PASS--devctl-cleanup-pass/reference/01-diary.md — Diary of investigation steps and commands


## 2026-01-07

Uploaded MO-010 analysis PDFs to reMarkable under ai/2026/01/07/MO-010-DEVCTL-CLEANUP-PASS/analysis/.

### Related Files

- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/ttmp/2026/01/07/MO-010-DEVCTL-CLEANUP-PASS--devctl-cleanup-pass/analysis/01-service-supervision-architecture-events-and-ui-integration.md — Uploaded as PDF
- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/ttmp/2026/01/07/MO-010-DEVCTL-CLEANUP-PASS--devctl-cleanup-pass/analysis/02-comprehensive-fixture-devctl-up-failure-bug-report.md — Uploaded as PDF
- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/ttmp/2026/01/07/MO-010-DEVCTL-CLEANUP-PASS--devctl-cleanup-pass/reference/01-diary.md — Recorded upload commands and outcomes


## 2026-01-07

Added design doc for long-term safe plugin invocation/capability enforcement (reviewing MO-009 proposals and current call sites).

### Related Files

- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/ttmp/2026/01/07/MO-010-DEVCTL-CLEANUP-PASS--devctl-cleanup-pass/design-doc/01-safe-plugin-invocation-and-capability-enforcement-long-term-pattern.md — Design decisions
- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/ttmp/2026/01/07/MO-010-DEVCTL-CLEANUP-PASS--devctl-cleanup-pass/reference/01-diary.md — Recorded review steps and findings


## 2026-01-07

Uploaded capability-checking design doc PDF to reMarkable under ai/2026/01/07/MO-010-DEVCTL-CLEANUP-PASS/design-doc/.

### Related Files

- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/ttmp/2026/01/07/MO-010-DEVCTL-CLEANUP-PASS--devctl-cleanup-pass/design-doc/01-safe-plugin-invocation-and-capability-enforcement-long-term-pattern.md — Uploaded as PDF
- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/ttmp/2026/01/07/MO-010-DEVCTL-CLEANUP-PASS--devctl-cleanup-pass/reference/01-diary.md — Recorded upload and font warnings


## 2026-01-07

Added textbook walkthrough of fundamentals→events→UI messages and documented robustness seams

### Related Files

- devctl/pkg/tui/action_runner.go — One of the primary producers of pipeline domain events


## 2026-01-07

Added textbook-style reference on runtime.Client, streaming semantics, and plugin-defined commands (commands.list/command.run).

### Related Files

- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/ttmp/2026/01/07/MO-010-DEVCTL-CLEANUP-PASS--devctl-cleanup-pass/reference/01-diary.md — Recorded authoring steps
- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/ttmp/2026/01/07/MO-010-DEVCTL-CLEANUP-PASS--devctl-cleanup-pass/reference/02-runtime-client-plugin-protocol-ops-streams-and-commands.md — New backend reference doc


## 2026-01-07

Uploaded runtime client textbook reference PDF to reMarkable under ai/2026/01/07/MO-010-DEVCTL-CLEANUP-PASS/reference/.

### Related Files

- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/ttmp/2026/01/07/MO-010-DEVCTL-CLEANUP-PASS--devctl-cleanup-pass/reference/01-diary.md — Recorded upload
- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/ttmp/2026/01/07/MO-010-DEVCTL-CLEANUP-PASS--devctl-cleanup-pass/reference/02-runtime-client-plugin-protocol-ops-streams-and-commands.md — Uploaded as PDF


## 2026-01-07

Updated capability-enforcement design doc: added FAQ (commands.list elimination, one-pass discovery, logs.follow status, definition of startup UX calls) and removed emoji for PDF safety.

### Related Files

- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/ttmp/2026/01/07/MO-010-DEVCTL-CLEANUP-PASS--devctl-cleanup-pass/design-doc/01-safe-plugin-invocation-and-capability-enforcement-long-term-pattern.md — Added discussion section and clarified terminology


## 2026-01-07

Extended capability-enforcement design doc with Appendix A: explicit RequestMeta + Repository/RuntimeEnv pattern to avoid context.Value for repo_root; clarified Client semantics.

### Related Files

- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/ttmp/2026/01/07/MO-010-DEVCTL-CLEANUP-PASS--devctl-cleanup-pass/design-doc/01-safe-plugin-invocation-and-capability-enforcement-long-term-pattern.md — Added Appendix A


## 2026-01-07

Added Protocol v2 no-compat design doc: command specs in handshake (no commands.list), Repository context container, and capability-enforced Client.Call.

### Related Files

- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/ttmp/2026/01/07/MO-010-DEVCTL-CLEANUP-PASS--devctl-cleanup-pass/design-doc/02-protocol-v2-handshake-command-specs-repository-context-capability-enforced-calls.md — New design doc


## 2026-01-07

Kickoff protocol v2 cleanup pass: expanded implementation plan and created docmgr task breakdown

### Related Files

- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/ttmp/2026/01/07/MO-010-DEVCTL-CLEANUP-PASS--devctl-cleanup-pass/design-doc/02-protocol-v2-handshake-command-specs-repository-context-capability-enforced-calls.md — Detailed phased implementation plan
- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/ttmp/2026/01/07/MO-010-DEVCTL-CLEANUP-PASS--devctl-cleanup-pass/tasks.md — Created concrete task checklist aligned to plan


## 2026-01-07

Implement protocol v2 handshake command specs, repository meta, and capability-enforced runtime calls (commit 7fce1bc)

### Related Files

- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/pkg/protocol/types.go — Protocol v2 + CommandSpec schema
- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/pkg/repository/repository.go — Repository container for config/specs/meta
- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/pkg/runtime/client.go — Call/StartStream capability enforcement + request ctx from meta


## 2026-01-07

Add exhaustive real-world test tasks (fixtures + CLI + TUI via tmux)

### Related Files

- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/ttmp/2026/01/07/MO-010-DEVCTL-CLEANUP-PASS--devctl-cleanup-pass/reference/01-diary.md — Recorded Step 14
- /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/ttmp/2026/01/07/MO-010-DEVCTL-CLEANUP-PASS--devctl-cleanup-pass/tasks.md — Added extensive manual validation matrix

