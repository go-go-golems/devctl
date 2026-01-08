---
Title: Testing Diary
Ticket: MO-005-IMPROVE-STARTDEV
Status: active
Topics:
    - dev-tooling
    - scripts
    - bash
    - dev-environment
DocType: reference
Intent: long-term
Owners:
    - team
RelatedFiles:
    - Path: devctl/.github/workflows/push.yml
      Note: CI jobs for smoketests fast/slow split (commit 0ffc326)
    - Path: devctl/cmd/devctl/cmds/dev/smoketest/e2e.go
      Note: Added dev smoketest e2e and testapp build flow (commit 80aaaec)
    - Path: devctl/pkg/runtime/context.go
      Note: Propagate repo_root/dry_run into protocol requests (commit 80aaaec)
    - Path: devctl/pkg/supervise/supervisor_test.go
      Note: Supervisor readiness timeout + post-ready crash tests (commit 0ffc326)
    - Path: moments/backend/pkg/stytchcfg/settings.go
      Note: Root cause of initial backend boot failure (stytch required)
    - Path: moments/docker-compose.yml
      Note: Infra dependencies for end-to-end run
    - Path: moments/plugins/moments-plugin.py
      Note: Honor dry-run; set GOWORK=off for backend build/run (commit 9f67600)
    - Path: moments/scripts/startdev.sh
      Note: Force GOWORK=off for backend build/run (commit 9f67600)
ExternalSources: []
Summary: ""
LastUpdated: 2026-01-06T15:20:50.387337179-05:00
WhatFor: ""
WhenToUse: ""
---




# Testing Diary

## Goal

Keep a frequent, step-by-step diary of end-to-end testing and build verification for MO-005, including exact commands, outputs, failures, and checkpoints along the full `devctl` pipeline.

Session:          019b94f5-80e5-7dd3-ac1d-795982d224c7

## Step 1: Baseline devctl tests and Moments plugin discovery

Validated the current `devctl` baseline in isolation (Go tests + existing smoke tests), then sanity-checked that `devctl` can discover and handshake with the Moments plugin configured in `moments/.devctl.yaml`. This establishes a known-good starting point before expanding fixtures and adding new smoke/e2e flows.

**Commit (code):** N/A — testing-only

### What I did
- Ran Go tests in `devctl/`: `go test ./... -count=1`
- Ran existing smoke tests:
  - `go run ./cmd/devctl dev smoketest`
  - `go run ./cmd/devctl dev smoketest supervise`
- Verified Moments plugin discovery/handshake:
  - `go run ./cmd/devctl plugins list --repo-root ../moments`

### Why
- Establish a baseline that new fixtures/smoke tests must continue to pass.
- Confirm the Moments repo can be used as a “real plugin” target for manual end-to-end runs when needed.

### What worked
- `go test ./... -count=1` passed in `devctl/`.
- `dev smoketest` and `dev smoketest supervise` both passed.
- Moments plugin handshake succeeded and reported expected ops (`config.mutate`, `validate.run`, `prepare.run`, `build.run`, `launch.plan`).

### What didn't work
- `plugins list` emitted an error log even though output was correct:
  - Command: `go run ./cmd/devctl plugins list --repo-root ../moments`
  - Log: `ERR stdout read error error="read |0: file already closed" plugin=moments`

### What I learned
- We likely need a “clean shutdown” path in runtime stdout/stderr readers so a normal plugin exit doesn’t show up as an error.

### What was tricky to build
- N/A (no build changes yet)

### What warrants a second pair of eyes
- Whether the “file already closed” error should be treated as benign (debug-level) or eliminated with a more explicit shutdown contract.

### What should be done in the future
- Decide the expected lifecycle for `plugins list` (keep plugin running vs start→handshake→stop) and make runtime logs reflect that cleanly.

### Code review instructions
- Start at `devctl/pkg/runtime/*` to inspect reader shutdown behavior and error logging.
- Re-run the same command: `go run ./cmd/devctl plugins list --repo-root ../moments`

### Technical details
- `devctl` smoke tests run quickly and are currently green; use them as a “must pass” gate while adding fixtures.

## Step 2: Moments plan computation + attempted dry-run up

Verified that the Moments plugin can compute a complete merged plan (config + services), then attempted a `devctl up --dry-run` to exercise the full pipeline without starting the world. The run surfaced two issues: backend build fails under this multi-repo workspace due to `go.work` module selection, and `--dry-run` still performs major side effects (build/install/start).

**Commit (code):** N/A — testing-only

### What I did
- Computed the plan:
  - `go run ./cmd/devctl plan --repo-root ../moments`
- Attempted to run the pipeline in “dry-run” mode:
  - `go run ./cmd/devctl up --repo-root ../moments --dry-run --timeout 10s`

### Why
- Confirm the plugin can generate a realistic multi-service plan (backend + web).
- Try to capture intermediate pipeline behavior under an “intended safe” mode.

### What worked
- `plan` produced config and a 2-service launch plan (backend + web) with TCP health checks.

### What didn't work
- Backend build failed:
  - `directory cmd/moments-server is contained in a module that is not one of the workspace modules listed in go.work`
  - Suggested fix from `go`: `go work use .` (within `moments/backend`)
- `--dry-run` still ran side-effectful operations (notably):
  - `make -C backend build`
  - `pnpm install --prefer-offline`
  - `pnpm generate-version`
  - attempted to start `backend` and `web` services and waited on health
- The run ended with: `Error: tcp health timeout: context deadline exceeded`

### What I learned
- In this “multi-repo workspace” layout, a top-level `go.work` that doesn’t include `moments/backend` breaks `make build` (and likely `scripts/startdev.sh`) unless we disable workspace mode (`GOWORK=off`) or include the module.
- The current `--dry-run` is “best-effort” but is not safe for “no side effects” usage; plugin/build/launch phases still run.

### What was tricky to build
- N/A (no build changes yet)

### What warrants a second pair of eyes
- What `--dry-run` is intended to guarantee: “no destructive ops” vs “no side effects”.
- Whether Moments plugin should force `GOWORK=off` for backend builds in mixed workspaces, or whether the workspace root should own a comprehensive `go.work`.

### What should be done in the future
- Decide and codify `--dry-run` semantics (and test them).
- Fix the Moments backend build under this workspace layout (either via `go.work` membership or explicit `GOWORK=off` for backend make/go steps).

### Code review instructions
- Review `devctl`’s dry-run plumbing (how/if it reaches plugins and the supervisor).
- Review Moments plugin command execution (`moments/plugins/moments-plugin.py`) for how it invokes `make`/`pnpm`.

### Technical details
- `plan` output included TCP health checks for `127.0.0.1:8083` and `127.0.0.1:5173` with `timeout_ms: 30000`.

## Step 3: devctl test scaffolding + new smoke tests (e2e/logs/failures)

Expanded `devctl`’s test surface area to cover “full pipeline” behavior in a hermetic way: Go unit/integration tests, fixture plugins, small Go “test apps”, and three new CLI smoke tests that exercise end-to-end service supervision, log follow cancellation, and key failure modes. This makes it much easier to validate changes quickly while iterating on MO-005 and reduces reliance on the real Moments repo for basic runner correctness.

**Commit (code):** 80aaaec — "Add e2e smoketests and propagate dry-run context"

### What I did
- Implemented request context propagation (`repo_root`, `dry_run`) into protocol requests so plugins can actually respect `--dry-run`.
- Made plugin shutdown / short-lived handshakes quieter by avoiding “file already closed” error logs during `plugins list`.
- Added fixture plugins:
  - `devctl/testdata/plugins/validate-passfail/plugin.py`
  - `devctl/testdata/plugins/launch-fail/plugin.py`
  - `devctl/testdata/plugins/timeout/plugin.py`
  - `devctl/testdata/plugins/long-running-plugin/plugin.py`
  - `devctl/testdata/plugins/e2e/plugin.py`
- Added Go “test apps”:
  - `devctl/testapps/cmd/http-echo`
  - `devctl/testapps/cmd/crash-after`
  - `devctl/testapps/cmd/log-spewer`
- Added CLI smoke tests:
  - `go run ./cmd/devctl dev smoketest e2e`
  - `go run ./cmd/devctl dev smoketest logs`
  - `go run ./cmd/devctl dev smoketest failures`
- Ran verification gates:
  - `go test ./... -count=1`
  - `make lint`

### Why
- Provide fast, deterministic coverage of process orchestration edge cases (streams, timeouts, failures).
- Ensure `--dry-run` can be meaningfully implemented at the plugin layer.

### What worked
- New smoke tests run quickly and passed locally:
  - `dev smoketest`, `dev smoketest supervise`, `dev smoketest e2e`, `dev smoketest logs`, `dev smoketest failures`
- `make lint` is clean after fixing `dynamic_commands.go` lint issues.

### What didn't work
- `make lint` initially failed with:
  - `nonamedreturns` on `parseRepoArgs` and
  - `SA1019` deprecation for `ParseErrorsWhitelist` (fixed by switching to `ParseErrorsAllowlist`).

### What I learned
- Without explicit request-context wiring, plugins can’t tell they’re in dry-run mode; the flag was effectively “dead” before.

### What was tricky to build
- Keeping smoke tests hermetic while still exercising “real” process behavior (ports, logs, readiness) without flakes.

### What warrants a second pair of eyes
- Runtime shutdown semantics: confirm the chosen “don’t log errors when intentionally closing” behavior is consistent with desired observability.

### What should be done in the future
- Expand failure-mode coverage to include readiness timeouts/post-ready crash semantics in dedicated supervisor tests (not only smoke tests).

### Code review instructions
- Start at `devctl/pkg/runtime/client.go` and `devctl/pkg/runtime/context.go` to see request-context + shutdown behavior.
- Run: `cd devctl && go test ./... -count=1 && go run ./cmd/devctl dev smoketest e2e`

### Technical details
- New smoke tests build test apps using `go build` with `GOWORK=off` to avoid workspace leakage.

## Step 4: Moments end-to-end: docker compose + devctl up/status/logs/down

Ran the real Moments dev environment end-to-end using `devctl` + the Moments plugin, including the full intermediate steps (build, prepare, validate, launch, health, status, log follow, down). The first attempt failed due to required Stytch configuration; after adding a local ignored config file with placeholder Stytch values, the environment started cleanly and responded on both backend and web ports.

**Commit (code):** N/A — mixed local config + docs; code changes committed separately in Moments

### What I did
- Started infra dependencies:
  - `cd moments && docker compose up -d db redis elasticsearch`
- Attempted full pipeline:
  - `cd devctl && timeout 240s go run ./cmd/devctl up --repo-root ../moments --timeout 3m`
- Investigated backend failure via devctl logs directory:
  - `ls -lt moments/.devctl/logs`
  - inspected `moments/.devctl/logs/backend-*.stderr.log`
- Added local ignored config to satisfy startup validation:
  - created `moments/config/app/local.yaml` with placeholder `integrations.stytch.stytch-project-id` and `integrations.stytch.stytch-secret`
- Re-ran pipeline successfully:
  - `cd devctl && timeout 240s go run ./cmd/devctl up --repo-root ../moments --timeout 3m`
  - `cd devctl && go run ./cmd/devctl status --repo-root ../moments`
  - `cd devctl && go run ./cmd/devctl logs --repo-root ../moments --service backend --stderr | tail -40`
  - `curl -fsS http://127.0.0.1:8083/health`
  - `curl -I -fsS http://127.0.0.1:5173/`
  - `cd devctl && go run ./cmd/devctl down --repo-root ../moments`

### Why
- Validate that the “real repo plugin” path works, not only hermetic fixtures.
- Capture intermediate steps and failure points for MO-005.

### What worked
- After adding local Stytch placeholders, `devctl up` completed and both services were alive.
- Backend health endpoint responded:
  - `curl -fsS http://127.0.0.1:8083/health`
- Web dev server responded on `5173`.

### What didn't work
- First `devctl up` failed; backend exited with appconfig validation error:
  - `Error: failed to initialize appconfig: initialize appconfig: validate "stytch": stytch configuration required: set stytch-project-id and stytch-secret`
  - `make: *** [Makefile:32: bootstrap] Error 1`
  - `Error: tcp health timeout: context deadline exceeded`

### What I learned
- Moments backend requires Stytch credentials even for local dev; a minimal local config is necessary for boot.

### What was tricky to build
- Separating “infra not running” failures from “appconfig required values” failures quickly; the `.devctl/logs` files were the fastest signal.

### What warrants a second pair of eyes
- Whether Stytch should be required in `development` by default, or if validation should be conditional for local dev (policy decision).

### What should be done in the future
- Decide on a first-class “local dev defaults” story for required integrations (documented env vars or a template `local.yaml.example`).

### Code review instructions
- Re-run the same flow:
  - `cd moments && docker compose up -d db redis elasticsearch`
  - `cd devctl && go run ./cmd/devctl up --repo-root ../moments --timeout 3m`
  - `cd devctl && go run ./cmd/devctl down --repo-root ../moments`

### Technical details
- The backend health check is currently TCP-level in the plugin plan, but the service itself provides `/health` which is useful for verification.

## Step 5: Supervisor failure-mode tests + CI smoketest split

Expanded the remaining testing coverage called out by the ticket: supervisor-level tests for readiness timeout cleanup and “post-ready crash” observability. Also updated CI to run smoketests in a fast/slow split so regressions are caught automatically without making every job heavy.

**Commit (code):** 0ffc326 — "CI: add smoketest jobs; test supervisor failures"

### What I did
- Added supervisor tests in `devctl/pkg/supervise/supervisor_test.go`:
  - readiness timeout returns error and the started service is terminated (no leaked PID)
  - a service that becomes healthy then exits is observable via `state.ProcessAlive(pid)==false`
- Wired smoketests into CI in `devctl/.github/workflows/push.yml`:
  - `unit` job: `go test ./... -count=1`
  - `smoke-fast` job: `dev smoketest`, `dev smoketest supervise`, `dev smoketest logs`, `dev smoketest failures`
  - `smoke-e2e` job: `dev smoketest e2e`
- Ran local gates:
  - `GOWORK=off go test ./... -count=1`
  - `make lint`

### Why
- Readiness failures and crash-after-ready are the most common real-world orchestration failure modes; these tests ensure we don’t regress cleanup or observability.
- CI smoke coverage makes it safe to iterate quickly on the runner/supervisor.

### What worked
- The new supervisor tests pass and run quickly.
- CI workflow now exercises smoketests in separate jobs.

### What didn't work
- N/A

### What I learned
- The workspace `go.work` in this environment can interfere with `go test` runs; using `GOWORK=off` remains the safest default in CI commands for `devctl`.

### What was tricky to build
- Making the “post-ready crash” test deterministic without flaky port races; using a short-lived `python3 -m http.server` behind `timeout` proved reliable.

### What warrants a second pair of eyes
- Whether CI should treat `smoke-e2e` as required for PRs (currently it runs, but the job split is the first step toward marking it optional if needed).

### What should be done in the future
- Complete remaining fixture apps (`slow-start`, `hang`) and add explicit readiness-timeout smoke coverage at the CLI level.

### Code review instructions
- Start at `devctl/pkg/supervise/supervisor_test.go` and `devctl/.github/workflows/push.yml`.
- Validate locally:
  - `cd devctl && GOWORK=off go test ./... -count=1 && make lint && GOWORK=off go run ./cmd/devctl dev smoketest e2e`

### Technical details
- The readiness-timeout test writes the service PID to a file before sleeping so the test can assert `Start` properly terminates the process on failure.

## Related

- `moments/ttmp/2026/01/06/MO-005-IMPROVE-STARTDEV--improve-startdev-sh-script-analysis-and-enhancement/design-doc/06-devctl-testing-strategy-smoke-tests-fixtures-and-end-to-end-runs.md`
- `moments/ttmp/2026/01/06/MO-005-IMPROVE-STARTDEV--improve-startdev-sh-script-analysis-and-enhancement/reference/01-diary.md`
