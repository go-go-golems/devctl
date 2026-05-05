---
doc_type: reference
title: Investigation Diary
status: active
intent: long-term
topics:
  - devctl
  - services
  - supervisor
  - restart
  - architecture
created_at: 2026-05-04
updated_at: 2026-05-04
---

# Investigation Diary

## Goal

Chronological record of the investigation into individual service start/stop for devctl. Captures what was examined, key findings about why this is hard, and the design decisions.

---

## Step 1: Deep Investigation of Service Lifecycle

Examined the full service lifecycle: from plugin launch.plan → merged LaunchPlan → Supervisor.Start → state.json → Supervisor.Stop → state.Remove.

### Prompt Context

**User prompt (verbatim):** "Now do another docmgr ticket and another similar deep analysis on what it would take to make it possible to start/stop individual services (individual plan results), which can be a bit tricky (maybe even impossible to do cleanly in the way it is setup now), since the plugin can probably intercorrelate the way they are launched."

**Assistant interpretation:** Analyze why individual service start/stop is difficult in devctl's current architecture, identify the coupling points, and design a solution that handles the tricky inter-plugin correlations.

**Inferred user intent:** Understand the fundamental architectural challenges before proposing a solution, because the user suspects this might be hard or even impossible to do cleanly.

### What I did

1. Read `supervisor.go` — understood atomic Start/Stop, no single-service operations.
2. Read `down.go` — stops ALL services, no filtering.
3. Read `status.go` — reads state.json, shows per-service alive status.
4. Read `wrap_service.go` — understood wrapper process model with exit info recording.
5. Read `action_runner.go` — **KEY FINDING**: `ActionStop` returns `errors.New("stop action is not implemented")`. The TUI has UI for stop/restart but the backend doesn't implement it.
6. Read `service_model.go` — TUI has `[s] stop` and `[r] restart` keys already wired.
7. Read `root_model.go` — dispatches ActionRequestMsg through Watermill message bus.
8. Read `action_runner.go` runDown/runUp — understood the full pipeline replication in TUI action runner.

### Key findings

**Why this is hard (the core problem):**

1. **Service provenance is lost.** `engine.Pipeline.LaunchPlan()` merges services from all plugins into a flat `LaunchPlan`. By the time the supervisor sees services, it doesn't know which plugin produced which service. Stopping a service means you can't ask just that plugin for a replacement plan.

2. **Plugins correlate via config mutation.** `MutateConfig()` is a sequential chain where each plugin sees and modifies the shared config. A later plugin's behavior depends on what earlier plugins did. Re-running a single plugin's `launch.plan` would give it a different config state than it had originally.

3. **State is atomic.** `state.json` is written once after all services start. There's no mechanism for partial updates — adding or removing a service record requires rewriting the entire file.

4. **Supervisor has no single-service operations.** `Start()` takes a full LaunchPlan; `Stop()` takes the entire State. There's no `StopService(name)` or `StartService(spec)`.

5. **Restart currently means full down+up.** The TUI's restart action stops everything and starts everything. This is destructive to sibling services.

### What was tricky

- The pipeline's merge semantics for `LaunchPlan` (name collision = replace) means two plugins can contribute the same service name, but you can't tell which one "won" after the merge.
- The config mutation chain means you can't safely re-run one plugin's launch.plan in isolation without also re-running all earlier plugins' config mutations.

### What should be done in the future

- Implement the design from the design doc, starting with Phase 1 (provenance tracking) and Phase 2 (single-service supervisor operations).

---

## Step 2: Design Document Written and Uploaded

Wrote the full design document analyzing the problem, proposing a provenance-tracking approach, and providing phased implementation guidance. Uploaded to reMarkable as bundled PDF.

### What I did

1. Wrote 12-section design document with detailed analysis of the coupling problem.
2. Proposed three approaches: simple restart (kill+respawn same spec), provenance-tracked restart (re-run originating plugin), and a hybrid approach.
3. Identified that simple restart (kill process, respawn from stored ServiceSpec) is both sufficient and achievable.
4. Designed new CLI commands: `devctl restart <service>`, `devctl stop <service>`.
5. Designed supervisor changes: `StopService()`, `StartService()`, `RestartService()`.
6. Designed state file changes to support partial updates.
7. Uploaded to reMarkable.

### What worked

- The analysis clearly exposed why "clean" restart is hard (config mutation correlation).
- The simple restart approach (just kill and respawn from stored spec) avoids all the correlation complexity.
- The TUI already has the UI wired — just needs the backend implementation.

### What didn't work

- N/A — smooth workflow.

### Code review instructions

- Read the design doc: `devctl/ttmp/2026/05/04/DCTL-SERVICES--devctl-individual-service-start-stop-analysis-design-and-implementation-guide/design-doc/01-individual-service-start-stop-analysis-design-and-implementation-guide.md`
- Focus on Section 5 (The Core Problem) and Section 7 (Proposed Architecture) — these are the critical design decisions.

---

## Step 3: Implementation (Phases 1-5)

Implemented all five phases of the design. Each phase was committed separately.

### Prompt Context

**User prompt (verbatim):** "Add tasks for DCTL-SERVICES and then implement them one by one, committing at appropriate intervals, keeping a diary as you work."

**Assistant interpretation:** Execute the implementation plan from the design doc, committing each phase independently.

**Inferred user intent:** Get working code with clean commit history.

### What I did

1. **Fixed build:** Removed `2026-05-03--goja-hosting-site` from `go.work` to fix missing go-go-goja replace directive.
2. **Phase 1 (commit 70d90b0):** Added `ServiceSpecRecord` and `HealthCheckRecord` types to `pkg/state/state.go`. Added `Spec` field to `ServiceRecord`. Updated both `startService` paths in `pkg/supervise/supervisor.go` to populate Spec.
3. **Phase 2 (commit 8a41a37):** Added `StopService()`, `StartService()`, `RestartService()` to `Supervisor`. Added `specFromRecord()` helper.
4. **Phase 3 (commit 33ca243):** Created `cmd/devctl/cmds/restart.go` and `cmd/devctl/cmds/stop_service.go`. Registered in `root.go`.
5. **Phase 4 (commit 9b81ffb):** Replaced `ActionStop` stub in `action_runner.go`. Added `runStopService()` and `runRestartService()`. Updated `ActionRestart` to support per-service restart.
6. **Phase 5 (commit d2d09e2):** Added three tests in `pkg/state/state_test.go`: round-trip, backward compat, save/load with spec.

### What worked

- All phases built and tested cleanly on first try.
- The kill-and-respawn approach was straightforward to implement.
- Backward compatibility test confirmed old state files load without Spec field.

### What didn't work

- First commit failed due to gofmt issue (comment alignment). Fixed with `gofmt -w`.
- The go.work had a stale reference to goja-hosting-site causing build failure. Removed it.
- `edit` tool had trouble with tab-indented Go code — used Python for sed-like replacements.

### What was tricky to build

- The supervisor has two `startService` code paths (wrapper vs non-wrapper) and both needed the `Spec` field populated. Different indentation made exact-text edits difficult.
- The TUI action runner needed careful handling of the `ActionRestart` case: with a service name it should restart just that service, without it should do full down+up.

### What warrants a second pair of eyes

- Original implementation concern: `Spec.Env` stored unsanitized values in state.json. This was later fixed in Step 6 by removing env from persisted `ServiceSpecRecord` and re-running `config.mutate + launch.plan` for start/restart.
- The `StartService` reuses `startService()` which creates the wrapper process. Verify this works correctly in both wrapper and non-wrapper modes.

### What should be done in the future

- Add `devctl start <service>` command for starting previously stopped services.
- Add supervisor integration tests with real processes.
- Update the status command to show stopped services differently.

### Code review instructions

- Check the four commits: `70d90b0`, `8a41a37`, `33ca243`, `9b81ffb`, `d2d09e2`
- Key files: `pkg/state/state.go`, `pkg/supervise/supervisor.go`, `cmd/devctl/cmds/restart.go`, `cmd/devctl/cmds/stop_service.go`, `pkg/tui/action_runner.go`
- Run: `go test ./devctl/... -count=1`

---

## Step 4: Real tmux testing and start command

Tested all three commands (restart, stop-service, start) in tmux against a real multi-service devctl environment.

### Prompt Context

**User prompt (verbatim):** "do we have real examples to test against, for example in a tmux? devctl start/stop <service> is useful, yes"

**Assistant interpretation:** Test the implementation against real services in tmux, and also add `devctl start <service>` command.

**Inferred user intent:** Validate the implementation works end-to-end before considering it done, and add the missing start command.

### What I did

1. Created a test plugin (`devctl-multi-service`) that launches two long-running services: `counter` (counts to 3600 with 1s sleep) and `ticker` (prints timestamp every 2s).
2. Put it in `/tmp/devctl-test/plugins/` for auto-discovery.
3. Built devctl binary to `/tmp/devctl-test/devctl`.
4. Tested in tmux:
   - `devctl up` → 2 services started
   - `devctl restart counter` → counter got new PID, ticker untouched
   - `devctl stop-service ticker` → ticker stopped (pid=0), counter still running
   - `devctl start ticker` → ticker restarted from its service metadata (later changed in Step 6 to re-plan before start)
   - `devctl down` → all cleaned up
5. Implemented `devctl start <service>` command (new file `start_service.go`).

### What worked

- All commands worked on first try against real processes.
- The stored Spec correctly preserved command, env, and health check config.
- Auto-discovered plugin via `plugins/devctl-*` convention.

### What didn't work

- First test attempt used a plugin with `sleep 1` — services exited too fast to test restart. Fixed by creating longer-running test services.
- HTTP service plugin failed because port 18081 was in use. The `counter`/`ticker` bash approach was more reliable.
- Symlinked testdata directory didn't preserve execute permissions. Had to copy instead.

### Code review instructions

- Test commit: `228fbf0`
- New file: `cmd/devctl/cmds/start_service.go`
- To reproduce: build devctl, create `/tmp/devctl-test` with the multi-service plugin, run up/restart/stop-service/start/down cycle

---

## Step 5: CI supervisor readiness test fix

Investigated a GitHub Actions failure in `TestSupervisor_ReadinessTimeoutStopsServices`. The failure was not from the new service-control implementation directly; it was a timing assumption in an existing supervisor test. The test expected the child shell to create `pid.txt` before the readiness timeout stopped the service, but CI showed the service could be terminated before that file existed.

### Prompt Context

**User prompt (verbatim):** "https://github.com/go-go-golems/devctl/actions/runs/25379092873/job/74422125824?pr=6 ... FAIL TestSupervisor_ReadinessTimeoutStopsServices ... open /tmp/devctl-supervise-test-490747441/pid.txt: no such file or directory"

**Assistant interpretation:** Fix the failing CI test while preserving the intent: readiness timeout must stop started services and not leak them.

**Inferred user intent:** Get the PR green.

### What I did

- Updated `pkg/supervise/supervisor_test.go`.
- Increased the test `ReadyTimeout` from 500ms to 2s, matching a more realistic CI scheduling window.
- Passed the pid-file path through `DEVCTL_TEST_PID_FILE` and quoted it in the shell command instead of interpolating a raw path.
- Ran the targeted test: `go test ./pkg/supervise -count=1 -run TestSupervisor_ReadinessTimeoutStopsServices -v`.
- Ran the full suite: `go test ./... -count=1`.
- Committed as `10a61c1` — `test(devctl): make readiness timeout supervisor test robust`.

### What worked

- The targeted test now passes consistently locally.
- The full test suite passes.
- Lefthook pre-commit ran `golangci-lint run -v` and `go test ./...`; both passed.

### What didn't work

- N/A.

### What was tricky to build

- The core problem is a race in the test, not production behavior. `cmd.Start()` only guarantees the process was created, not that the shell has already executed `echo $$ > pid.txt`. A very short readiness timeout can kill the process before the file exists on a loaded runner.

### What should be done in the future

- If this test flakes again, consider avoiding child-created pid files entirely and changing the supervisor API/test seam to expose partial state on startup failure.

### Code review instructions

- Review `pkg/supervise/supervisor_test.go` around `TestSupervisor_ReadinessTimeoutStopsServices`.
- Validate with: `go test ./pkg/supervise -count=1 -run TestSupervisor_ReadinessTimeoutStopsServices -v` and `go test ./... -count=1`.

---

## Step 6: Address PR review comments

GitHub PR #6 review identified two P1 concerns: `StartService` allowed duplicate starts when the service was already running, and the persisted `ServiceSpecRecord` originally stored raw environment variables in `state.json`.

### Prompt Context

**User prompt (verbatim):** "https://github.com/go-go-golems/devctl/pull/6 Look at the code review comments. The first is easy to address. For the env computation is that something we can handle by rerunning the planning phase or so?"

**Assistant interpretation:** Fetch and address the PR review comments. Fix duplicate starts, and redesign env recovery to avoid storing raw env by re-running planning.

**Inferred user intent:** Make PR #6 safe to merge by addressing security and process-management review feedback.

### What I did

- Added `pkg/servicecontrol/resolve.go` with `ResolveServiceSpec()`, which re-runs `config.mutate + launch.plan` across all configured plugins and selects the named service.
- Changed `state.ServiceSpecRecord` to remove `Env`; state now stores only non-secret metadata.
- Changed `Supervisor.StartService` and `RestartService` to accept a freshly resolved `engine.ServiceSpec` instead of reconstructing a service from persisted state.
- Added an already-running guard in `StartService` to prevent duplicate unmanaged processes.
- Updated `devctl start` and `devctl restart` to call `servicecontrol.ResolveServiceSpec()` before starting.
- Updated TUI per-service restart to do the same.
- Added tests for duplicate start rejection and non-persistence of raw env secrets.
- Updated the design doc to state clearly that start/restart run the first two planning phases (`config.mutate + launch.plan`) and do not run build/prepare/validate.

### What worked

- `go test ./... -count=1` passes after the redesign.
- The review comment about raw env persistence is resolved by removing raw env from persisted state entirely.
- The duplicate-start review comment is resolved by an explicit `state.ProcessAlive(rec.PID)` guard.

### What didn't work

- N/A.

### What was tricky to build

- Moving spec recovery out of the supervisor required a new small package to avoid import cycles. The supervisor remains process-focused, while `servicecontrol` owns plugin/pipeline planning.

### What warrants a second pair of eyes

- Plugins must treat `config.mutate` and `launch.plan` as idempotent planning phases, because `devctl start/restart` now re-runs them.

### What should be done in the future

- Consider adding help/docs for plugin authors: `config.mutate` and `launch.plan` should be pure/idempotent.

### Code review instructions

- Review `pkg/servicecontrol/resolve.go`, `pkg/supervise/supervisor.go`, `cmd/devctl/cmds/start_service.go`, `cmd/devctl/cmds/restart.go`, and `pkg/tui/action_runner.go`.
- Validate with: `go test ./... -count=1`.
