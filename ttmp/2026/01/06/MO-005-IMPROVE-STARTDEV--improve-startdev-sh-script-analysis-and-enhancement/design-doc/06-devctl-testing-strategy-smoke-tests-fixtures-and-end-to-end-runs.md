---
Title: 'devctl Testing Strategy: Smoke Tests, Fixtures, and End-to-End Runs'
Ticket: MO-005-IMPROVE-STARTDEV
Status: active
Topics:
    - dev-tooling
    - scripts
    - bash
    - dev-environment
DocType: design-doc
Intent: long-term
Owners:
    - team
RelatedFiles:
    - Path: devctl/cmd/devctl/cmds/dev/smoketest/supervise.go
      Note: Existing supervise smoketest; template for new dev smoketest e2e/logs/failures
    - Path: devctl/pkg/runtime/runtime_test.go
      Note: Current runtime fixture tests; baseline for expanding fixtures
    - Path: devctl/pkg/supervise/supervisor.go
      Note: Supervision behavior under test (start/stop/health/log capture)
    - Path: devctl/testdata/plugins/http-service/plugin.py
      Note: Example fixture plugin that launches a real long-running service
ExternalSources: []
Summary: ""
LastUpdated: 2026-01-06T15:14:40.597080266-05:00
WhatFor: Define a comprehensive testing strategy for devctl (protocol/runtime/engine/supervision/log tailing), including fixture plugins and small Go 'test apps' used by smoke tests and an end-to-end run.
WhenToUse: When implementing or expanding devctl tests, adding new protocol features, or setting up CI to run smoke and end-to-end validation.
---


# devctl Testing Strategy: Smoke Tests, Fixtures, and End-to-End Runs

## Executive Summary

This document specifies how to test devctl’s full feature set with a layered strategy:

- **Unit tests** for pure logic (patching, merge rules, parsing).
- **Integration tests** for runtime invariants (handshake, request/response, streams, stdout contamination, timeouts).
- **Smoke tests (CLI-level)** that run a small, deterministic “fake repo” and validate:
  - long-running plugins and long-running services,
  - config mutation and propagation,
  - validation pass/fail scenarios,
  - service launch and failure handling,
  - log tail/follow behavior.
- **One end-to-end run** that executes the full pipeline (`build → prepare → validate → launch → status/logs → down`) using real processes (small Go “test apps” compiled and run like a real system).

The core idea is to make tests **hermetic**, **fast**, and **debuggable** by using:
- fixture plugins (Python, sometimes bash) with deterministic behavior, and
- small Go binaries under `devctl/testapps/` that simulate servers and failures.

## Problem Statement

devctl is a process orchestrator with a plugin protocol. The failure modes we care about are operational and subtle:

- protocol framing issues (stdout contamination, invalid JSON, missing fields),
- concurrency and routing issues (request correlation, streams, ordering),
- timeouts/cancellation behavior (hung plugin, hung service, slow readiness),
- deterministic composition rules (patch merge, collisions, strictness),
- supervision correctness (start/stop, reaping, state file integrity),
- log tail/follow correctness (missing lines, ordering, follow termination).

We need a test strategy that covers these behaviors without being flaky or dependent on the real Moments repo and environment.

## Proposed Solution

### Test layers

1. **Package unit tests** (Go `testing`):
   - `pkg/patch`: dotted-path set/unset and merge behavior.
   - `pkg/engine`: deterministic ordering and merge rules (strict vs non-strict).
2. **Runtime integration tests**:
   - spawn fixture plugins,
   - validate handshake correctness,
   - validate stdout purity enforcement,
   - validate stream buffering and stream end behavior,
   - validate request cancellation.
3. **CLI smoke tests (black-box-ish)**:
   - drive `cmd/devctl` commands in a temporary repo root,
   - create `.devctl.yaml` config pointing to fixture plugins and compiled test apps,
   - assert state file contents and log outputs.
4. **End-to-end run**:
   - compile test apps,
   - run `devctl up` for a multi-service plan (one healthy server + one failing service),
   - tail logs and assert expected lines,
   - run `devctl down` and ensure processes are dead and state removed.

### Test scaffolding (new code to add)

Create two “fixture families”:

1) **Fixture plugins** (protocol handlers), stored under:

```text
devctl/testdata/plugins/
  ok-python/                  # already exists
  noisy-handshake/
  noisy-after-handshake/
  stream/
  command/
  pipeline/
  http-service/
  long-running-plugin/        # NEW: supports cancellation + streaming
  validate-passfail/          # NEW: validate.run can be toggled to fail
  launch-fail/                # NEW: launch.plan returns failing command
```

2) **Go test apps** (real processes with predictable behavior), stored under:

```text
devctl/testapps/
  cmd/
    http-echo/                # HTTP server with /health and periodic logs
    crash-after/              # exits non-zero after N seconds, emits logs
    log-spewer/               # writes lines to stdout/stderr at a rate
    slow-start/               # binds port only after delay (readiness test)
    hang/                     # never becomes ready (timeout test)
```

These test apps simulate “real world systems” without pulling in Moments.

### How smoke tests should be written

Smoke tests should:
- create a temp repo root: `repo := t.TempDir()`
- write `.devctl.yaml` pointing to fixture plugins and test apps
- run the `devctl` binary (or `go run ./cmd/devctl`) with `--repo-root repo`
- assert:
  - `repo/.devctl/state.json` created on `up`
  - `repo/.devctl/logs/*.log` created and contains expected lines
  - `status` reports correct alive/dead states
  - `down` removes state and terminates processes

In addition, for *tailing logs*, tests should:
- start a service that emits logs periodically (test app),
- call `devctl logs --follow --service X` and capture output for a short time,
- cancel/kill and assert follow stops promptly.

### CLI smoke test entrypoints

Prefer CLI commands under `cmd/devctl/cmds/` for smoke testing:

- `devctl dev smoketest` (already exists: handshake + request/response)
- `devctl dev smoketest supervise` (already exists: health + service start/stop)
- `devctl dev smoketest e2e` (NEW: full pipeline with multiple services + logs + down)
- `devctl dev smoketest logs` (NEW: explicit log-follow assertions)
- `devctl dev smoketest failures` (NEW: validation fail, launch fail, timeout)

These are intentionally not “unit tests”; they are fast operational checks runnable by humans and CI.

## Design Decisions

### Why write Go “test apps” instead of shelling out to random system tools?

Because:
- behavior must be deterministic and cross-machine,
- we need precise control over readiness delay, exit codes, and logging cadence,
- Go apps compile quickly and can be structured to expose test hooks (ports, delays).

### Ports: avoid collisions

Test apps should bind to `:0` (ephemeral port) by default and print the chosen address to stderr. Fixture plugins can then:
- parse that address and patch it into config, or
- the smoke test can pre-allocate ports and pass them as env vars.

If we do pre-allocation, do it safely: bind to `:0`, read port, close, then hand it to the app. (Still races, but usually acceptable for tests; better is “app chooses port and reports it”.)

### Long-running plugins and cancellation

We need explicit fixtures for:
- a plugin that never responds to a request (timeout path),
- a plugin that starts a stream and then runs forever until stdin closes or ctx cancels,
- a plugin that emits events quickly (buffer/backpressure path).

### Supervision failure semantics

We need fixtures for:
- “service process exits immediately” (start failure),
- “service starts but fails health check” (readiness failure),
- “service runs but later crashes” (post-ready failure; status/logs should reflect).

### Log-follow semantics

Define what “follow” means for tests:
- follow reads appended lines until canceled,
- follow returns promptly on ctx cancellation,
- follow does not miss lines (within a reasonable bound).

Tests should always enforce timeouts (no hangs).

## Alternatives Considered

### Only unit tests (no smoke/e2e)

Rejected: devctl’s biggest risks are integration-level (process spawning, IO framing, log follow, timeouts).

### Only end-to-end tests against the real Moments repo

Rejected: too slow, too flaky, too environment-dependent; also makes CI heavy.

### Use Docker for test apps

Deferred: could be useful later, but would increase complexity and runtime significantly.

## Implementation Plan

### Phase 1: Expand fixture plugins (fast)

1. Add `long-running-plugin` fixture:
   - supports `logs.follow` stream that emits a line every 100ms and ends on stdin close.
2. Add `validate-passfail` fixture:
   - `validate.run` can be toggled with an env var to fail.
3. Add `launch-fail` fixture:
   - returns a plan with one service that exits non-zero immediately.

### Phase 2: Add Go test apps

1. Implement `http-echo` with:
   - `/health` returning 200,
   - `/` returning body with version string,
   - periodic log output (stdout/stderr).
2. Implement `crash-after`:
   - logs start, sleeps N, exits with code K.
3. Implement `slow-start` and `hang` for readiness testing.

### Phase 3: Add smoke test commands

1. `dev smoketest e2e`:
   - build test apps,
   - configure 2 services (one healthy, one crash-after),
   - run `up`, verify status/logs, then `down`.
2. `dev smoketest logs`:
   - start `log-spewer`, then run `logs --follow`, verify output and cancellation.
3. `dev smoketest failures`:
   - validate failure path,
   - launch failure path (health fails),
   - plugin timeout path.

### Phase 4: CI wiring

Run:
- `go test ./...`
- `go run ./cmd/devctl dev smoketest`
- `go run ./cmd/devctl dev smoketest supervise`
- `go run ./cmd/devctl dev smoketest e2e` (optional “slow” job)

## Open Questions

1. Should `devctl up` stop at first service failure, or keep other services running?
2. Should log-follow be implemented via file tailing only, or via plugin `logs.follow` streams as well?
3. Do we want a “test mode” that disables dynamic plugin commands to reduce startup work in CI?
4. How do we ensure Windows compatibility (if desired), given process groups and signals?

## References

- `devctl` repo root: `/home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl`
- `design-doc/05-go-runner-architecture-ndjson-plugin-protocol-runner.md`
- `design-doc/04-script-plugin-protocol-config-mutation-build-prepare-validate-launch-monitor-logs-commands.md`
