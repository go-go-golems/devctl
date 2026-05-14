# Changelog

## 2026-05-14

- Initial workspace created.
- Added primary design document: `design-doc/01-design-devctl-phase-verbs-for-streaming-long-running-steps.md`.
- Added chronological investigation diary: `reference/01-diary.md`.
- Mapped existing devctl phase architecture across Cobra commands, repository loading, engine pipeline methods, runtime request/response, and protocol event streams.
- Documented the Pinocchio motivating case and noted that its plugin currently captures subprocess output and uses internal 120-second build/prepare timeouts.
- Recommended first implementing `devctl build`, `devctl prepare`, and `devctl validate` using existing pipeline calls and stderr progress, with structured phase streams deferred.

## 2026-05-14

Completed research/design package for devctl phase verbs and Pinocchio build use case.

### Related Files

- /home/manuel/workspaces/2026-05-14/devctl-build/devctl/ttmp/2026/05/14/DEVCTL-PHASE-VERBS--add-devctl-phase-verbs-for-long-running-build-steps/design-doc/01-design-devctl-phase-verbs-for-streaming-long-running-steps.md — Primary design deliverable


## 2026-05-14

Implemented standalone devctl build/prepare/validate commands and tests (commit 9f82a09).

### Related Files

- /home/manuel/workspaces/2026-05-14/devctl-build/devctl/cmd/devctl/cmds/phase.go — New phase commands


## 2026-05-14

Updated README and built-in help docs for phase commands and stderr progress guidance (commit c3606e7).

### Related Files

- /home/manuel/workspaces/2026-05-14/devctl-build/devctl/README.md — Phase command documentation

