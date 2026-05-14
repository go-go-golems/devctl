# Tasks

## Phase 0 — Ticket setup and research

- [x] Create docmgr ticket workspace for DEVCTL-PHASE-VERBS.
- [x] Inspect existing devctl CLI, engine, runtime, protocol, and documentation code paths.
- [x] Inspect the Pinocchio motivating repository and plugin build behavior.
- [x] Write an intern-facing design and implementation guide.
- [x] Maintain a chronological investigation diary.
- [x] Relate key source files and update ticket changelog.
- [x] Validate the ticket with `docmgr doctor`.
- [x] Upload the initial design bundle to reMarkable.

## Phase 1 — Core phase command implementation

- [x] Add a shared repo/pipeline setup helper for command implementations that need `config.mutate` plus one downstream phase.
- [x] Add static `devctl build` command that runs `config.mutate + build.run`.
- [x] Add static `devctl prepare` command that runs `config.mutate + prepare.run`.
- [x] Add static `devctl validate` command that runs `config.mutate + validate.run` and exits non-zero when invalid.
- [x] Register the new phase commands next to `plan` in the root command registry.
- [x] Keep `devctl up` behavior and flags backward-compatible.

## Phase 2 — Tests and validation

- [x] Add command tests for `devctl build --step ...` and JSON output.
- [x] Add command tests for `devctl prepare --step ...` and JSON output.
- [x] Add command tests for `devctl validate` success and invalid failure behavior.
- [x] Add command tests showing profile selection is respected.
- [x] Run focused tests for `cmd/devctl/cmds`, `pkg/engine`, and `pkg/runtime`.
- [x] Run full `go test ./... -count=1` before final handoff.

## Phase 3 — Documentation refresh

- [x] Update `README.md` command examples and CLI command table with `build`, `prepare`, and `validate`.
- [x] Update `README.md` lifecycle/flag docs to explain standalone phase verbs and long-running build timeouts.
- [x] Update `pkg/doc/topics/devctl-user-guide.md` with standalone phase command usage.
- [x] Update `pkg/doc/topics/devctl-plugin-authoring.md` with guidance for streaming long-running build/prepare subprocess output to stderr.
- [x] Ensure docs clearly state that protocol stdout must remain NDJSON-only and progress belongs on stderr unless using explicit stream ops.

## Phase 4 — Ticket closeout and delivery

- [x] Update the diary after each implementation phase.
- [x] Update ticket changelog with implementation commits.
- [x] Re-run `docmgr doctor --ticket DEVCTL-PHASE-VERBS --stale-after 30`.
- [x] Upload the final implementation/design bundle to reMarkable.
- [x] Report commit hashes, test results, doc paths, and reMarkable destination.

## Deferred follow-ups

- [ ] Patch the Pinocchio plugin to honor `input.steps`, stream subprocess output to stderr, and remove or parameterize internal 120-second subprocess timeouts.
- [ ] Revisit structured `build.stream`/`prepare.stream` protocol only if stderr progress is insufficient.
