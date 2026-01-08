---
Title: Testing Diary
Ticket: MO-010-DEVCTL-CLEANUP-PASS
Status: active
Topics:
    - backend
    - tui
    - refactor
    - ui-components
DocType: reference
Intent: long-term
Owners: []
RelatedFiles:
    - Path: cmd/devctl/cmds/dynamic_commands.go
      Note: Dynamic discovery behavior and __wrap-service skip
    - Path: cmd/devctl/cmds/logs.go
      Note: Follow/cancel behavior (fixture follow tested via timeout)
    - Path: cmd/devctl/cmds/dev/smoketest/root.go
      Note: Primary protocol v2 smoke test
    - Path: cmd/devctl/cmds/dev/smoketest/e2e.go
      Note: End-to-end up/status/logs/down smoke test
    - Path: cmd/devctl/cmds/wrap_service.go
      Note: Direct __wrap-service execution + process-group semantics
    - Path: pkg/supervise/supervisor.go
      Note: Wrapper ready-file 2s deadline (regression context)
    - Path: ttmp/2026/01/06/MO-006-DEVCTL-TUI--create-a-devctl-tui/scripts/setup-fixture-repo-root.sh
      Note: MO-006 fixture generator used for CLI loop
    - Path: ttmp/2026/01/06/MO-009-TUI-COMPLETE-FEATURES--complete-tui-features-per-mo-006-design/scripts/setup-comprehensive-fixture.sh
      Note: MO-009 comprehensive fixture generator used for CLI loop
    - Path: ttmp/2026/01/07/MO-010-DEVCTL-CLEANUP-PASS--devctl-cleanup-pass/tasks.md
      Note: Manual test matrix and checkoff list
ExternalSources: []
Summary: ""
LastUpdated: 2026-01-07T16:34:53.885126992-05:00
WhatFor: ""
WhenToUse: ""
---



# Testing Diary

## Goal

Record end-to-end, real-world testing for MO-010 (protocol v2 + dynamic commands + capability enforcement), including exact commands, outputs, failures, and task checkoffs.

## Step 1: CLI Smoketests + Basic Sanity Checks

This step ran the existing CLI smoketest commands (which exercise runtime, supervision, and log following) plus basic command sanity checks. The goal was to quickly validate that protocol v2 did not break core devctl flows and that the test commands still pass.

The main “gotcha” encountered was that `devctl version` is not a valid subcommand (the version is exposed via Cobra’s `--version` flag). After correcting that, the smoketest suite executed cleanly.

**Commit (code):** N/A

### What I did
- Ran help output generation:
  - `cd devctl && go run ./cmd/devctl --help`
- Verified version output:
  - Attempted (failed): `cd devctl && go run ./cmd/devctl version`
  - Correct command: `cd devctl && go run ./cmd/devctl --version`
- Ran smoketests:
  - `cd devctl && go run ./cmd/devctl dev smoketest`
  - `cd devctl && go run ./cmd/devctl dev smoketest failures`
  - `cd devctl && go run ./cmd/devctl dev smoketest logs`
  - `cd devctl && go run ./cmd/devctl dev smoketest supervise`
  - `cd devctl && go run ./cmd/devctl dev smoketest e2e`
- Verified plugins list against the shipped example config:
  - `cd devctl && go run ./cmd/devctl --config .devctl.example.yaml plugins list`
- Checked off tasks:
  - `[13]..[20]` (with task 14 edited to use `--version`)

### Why
- These smoketests cover “real behavior” that unit tests won’t: child process management, state file writing/removal, and log reading under follow/cancel.

### What worked
- All smoketests completed successfully (including log following cancel behavior via the dedicated smoketest).
- `plugins list` shows `protocol_version: "v2"` for the sample plugin.

### What didn't work
- `cd devctl && go run ./cmd/devctl version` failed (there is no `version` subcommand):
  - Output:
    - `Error: unknown command "version" for "devctl"`
    - `Run 'devctl --help' for usage.`
    - `exit status 1`

### What I learned
- “Version” is exposed via Cobra’s built-in flag; tests should use `--version` rather than assuming a command.

### What was tricky to build
- N/A.

### What warrants a second pair of eyes
- N/A.

### What should be done in the future
- If we want a `version` subcommand for UX parity, add it explicitly (out of scope for MO-010 tests).

### Code review instructions
- Start with the smoketest implementations:
  - `devctl/cmd/devctl/cmds/dev/smoketest/root.go`
  - `devctl/cmd/devctl/cmds/dev/smoketest/failures.go`
  - `devctl/cmd/devctl/cmds/dev/smoketest/logs.go`
  - `devctl/cmd/devctl/cmds/dev/smoketest/supervise.go`
  - `devctl/cmd/devctl/cmds/dev/smoketest/e2e.go`

### Technical details
- `plugins list` output excerpt (v2 confirmed):
  - `"protocol_version": "v2"`

## Step 2: MO-006 Fixture (CLI-Only)

This step ran the MO-006 fixture generator and exercised a “typical user loop” using only CLI commands (no TUI). The goal was to validate `plan/up/status/logs/down` with a realistic fixture repo that uses the built-in `e2e` plugin.

The first attempt at automating `logs --follow` used backgrounding + `kill -INT`, but it didn’t terminate reliably under `go run` in this harness and caused the overall command to time out. Switching to `timeout 2s ... logs --follow` made the follow test deterministic and avoided harness timeouts.

**Commit (code):** N/A

### What I did
- Created a fixture repo root:
  - `cd devctl && ./ttmp/2026/01/06/MO-006-DEVCTL-TUI--create-a-devctl-tui/scripts/setup-fixture-repo-root.sh`
- Ran the CLI loop (capturing outputs under `/tmp/`):
  - `devctl plan`
  - `devctl up`
  - `devctl status --tail-lines 25`
  - `devctl logs --service http`
  - `devctl logs --service http --stderr`
  - `timeout 2s devctl logs --service spewer --follow` (expected non-zero exit due to timeout)
  - `devctl down`
  - `devctl status` after down (expected failure: missing state)
  - `devctl up --dry-run` (expected: JSON output and no state persisted)
- Checked off tasks:
  - `[30]..[39]`

### Why
- This fixture closely matches the “devctl as a wrapper around small local binaries + python plugin” real-world usage pattern.

### What worked
- `plan/up/status/logs/down` succeeded end-to-end on the fixture.
- Followed logs contained output within 2 seconds and terminated deterministically under `timeout`.
- After `down`, `status` failed as expected with `read state: ... no such file or directory`.

### What didn't work
- First attempt at follow-cancel automation caused the harness command to time out after 10 minutes:
  - Command (abridged):
    - `... && go run ./cmd/devctl --repo-root "$REPO_ROOT" logs --service spewer --follow ... & pid=$!; sleep 1; kill -INT $pid; wait $pid || true; ...`
  - Result:
    - `command timed out after 600005 milliseconds`

### What I learned
- For “manual follow” commands in an automated harness, prefer `timeout` rather than signal delivery to a backgrounded `go run` process.

### What was tricky to build
- The log-follow UX is inherently interactive; forcing it into a scripted test requires carefully-chosen termination mechanics.

### What warrants a second pair of eyes
- N/A.

### What should be done in the future
- If we want this to be fully automated in CI, add a dedicated CLI test helper or reuse `dev smoketest logs` semantics.

### Code review instructions
- Review fixture generator:
  - `devctl/ttmp/2026/01/06/MO-006-DEVCTL-TUI--create-a-devctl-tui/scripts/setup-fixture-repo-root.sh`
- Review log-follow implementation:
  - `devctl/cmd/devctl/cmds/logs.go`

### Technical details
- `status` after down error excerpt:
  - `Error: read state: open .../.devctl/state.json: no such file or directory`

## Step 3: MO-009 Comprehensive Fixture (CLI-Only)

This step ran the comprehensive fixture generator and exercised the same CLI loop, plus the “short-lived service exit info” behavior. The goal was to validate that protocol v2 removed the original startup stall (no `commands.list`) and that a high-entropy environment still works end-to-end.

The fixture came up successfully (regression validated: no wrapper ready-file timeout), and after waiting >30s, `status --tail-lines` showed `short-lived` as dead with an `exit` payload. Notably, the exit info PID differs from the service PID stored in state, which matches the wrapper/child PID indirection documented elsewhere in MO-010.

**Commit (code):** N/A

### What I did
- Created the fixture:
  - `cd devctl && ./ttmp/2026/01/06/MO-009-TUI-COMPLETE-FEATURES--complete-tui-features-per-mo-006-design/scripts/setup-comprehensive-fixture.sh`
- Ran CLI loop (capturing outputs under `/tmp/`):
  - `devctl plan`
  - `devctl up`
  - `devctl status --tail-lines 25`
  - `devctl logs --service <svc>` and `--stderr` for: `backend`, `worker`, `log-producer`, `flaky`, `short-lived`
  - `sleep 35` then `devctl status --tail-lines 25` to validate exit info capture for `short-lived`
  - `devctl down`
- Checked off tasks:
  - `[40]..[44]`

### Why
- This is the closest approximation to “real UI usage” without running the TUI: multiple services, varied health checks, and high log volume.

### What worked
- `devctl up` succeeded on the comprehensive fixture (regression fix validated).
- After 35s, `short-lived` showed `alive: false` with a populated `exit` payload.
- Logs are accessible for all services via `devctl logs`.

### What didn't work
- N/A.

### What I learned
- Exit info PID mismatch is expected in wrapper mode:
  - `state.ServiceRecord.PID` is the wrapper PID (kill handle)
  - `exit.pid` is the child PID written by `__wrap-service`

### What was tricky to build
- Waiting for deterministic exit info requires an explicit sleep; the fixture is intentionally time-based.

### What warrants a second pair of eyes
- The wrapper/child PID semantics (including how they’re surfaced in status output) are subtle; if we want to make status more intuitive, we should make the PID indirection explicit in output.

### What should be done in the future
- Consider adding a `status` output field for wrapper pid vs child pid (out of scope for this test run).

### Code review instructions
- Review fixture generator:
  - `devctl/ttmp/2026/01/06/MO-009-TUI-COMPLETE-FEATURES--complete-tui-features-per-mo-006-design/scripts/setup-comprehensive-fixture.sh`
- Review wrapper PID handling:
  - `devctl/pkg/supervise/supervisor.go`
  - `devctl/cmd/devctl/cmds/wrap_service.go`
  - `devctl/cmd/devctl/cmds/status.go`

### Technical details
- `short-lived` status excerpt (PID mismatch):
  - `pid` (state) != `exit.pid` (child), with `exit_code: 0` and `stderr_tail` lines.

## Step 4: Exhaustive CLI Matrix (Dynamic Commands, Flags, Protocol Negatives)

This step expanded beyond the smoketests to cover the remaining CLI-only validation matrix: dynamic command behavior (including collisions and capability gating), root flag behaviors, handshake validation edge cases, and the “noisy stdout” negative plugins.

The emphasis here was to make sure protocol v2’s stricter handshake validation and capability enforcement show up as actionable errors in real user runs (not just unit tests), and to verify the new request context (`ctx.repo_root`, `ctx.cwd`, `ctx.dry_run`) is actually delivered end-to-end.

**Commit (code):** N/A

### What I did
- Ran unit tests:
  - `cd devctl && go test ./... -count=1`
- Dynamic commands:
  - Created a temp repo with `testdata/plugins/command/plugin.py` and ran `devctl echo hello world` (stderr log line includes `plugin=cmd`).
  - Created a temp repo with *two* command plugins; verified collision warning and that `echo` still runs:
    - warning includes: `command name collision; keeping first`
  - Created a temp plugin that advertises `capabilities.commands` but does **not** declare `command.run`; verified `echo` is not registered:
    - `Error: unknown command "echo" for "devctl"`
- Strictness:
  - Built a 2-plugin pipeline where both return service `demo`:
    - non-strict: last-wins (by priority order)
    - strict: `Error: service name collision: demo`
  - Built a config type-mismatch case (`services.demo` set to scalar, then later attempt to set `services.demo.port`):
    - `Error: cannot set "services.demo.port": path segment "demo" is not an object`
- Protocol/runtime negatives:
  - v1 handshake rejection:
    - `Error: E_PROTOCOL_INVALID_HANDSHAKE: unsupported protocol_version "v1"`
  - noisy handshake (non-JSON before handshake):
    - `Error: E_PROTOCOL_INVALID_JSON: NOT JSON: invalid character 'N' looking for beginning of value`
  - noisy-after-handshake (invalid stdout after handshake) via smoketest:
    - `Error: E_RUNTIME: ... E_PROTOCOL_STDOUT_CONTAMINATION: oops-not-json ...`
- Request ctx correctness:
  - Created a temp command plugin that logs `req["ctx"]` to stderr on `command.run`.
  - Verified:
    - `ctx.repo_root` equals `--repo-root` (or CWD when omitted)
    - `ctx.cwd` equals the actual process cwd
    - `--dry-run` sets `ctx.dry_run: true`
- Root flags and CLI errors:
  - `--timeout 0` is rejected:
    - `Error: timeout must be > 0`
  - Relative `--config .devctl.yaml` resolves under `--repo-root`.
  - Running inside repo root without `--repo-root` uses CWD as repo root.
  - No config present:
    - `plan` prints `{}` and warns: `no plugins configured (add .devctl.yaml)`
    - `up` errors: `no plugins configured (add .devctl.yaml)`
  - Invalid YAML yields a parse error.
  - `logs` errors:
    - missing `--service`: `Error: --service is required`
    - unknown `--service`: `Error: unknown service "nosuch"`
    - follow cancels promptly via `timeout -s INT 1s ... logs --follow`
  - Handshake validation checks:
    - duplicate command names -> `E_PROTOCOL_INVALID_HANDSHAKE: duplicate command name "echo"`
    - missing command name -> `E_PROTOCOL_INVALID_HANDSHAKE: capabilities.commands[0] missing name`
    - missing arg type -> `E_PROTOCOL_INVALID_HANDSHAKE: capabilities.commands[0].args_spec[0] missing type`
  - Multi-plugin start failure cleanup:
    - config `ok-python + noisy-handshake` fails as expected, and no `ok-python/plugin.py` process is left running afterward.
- Checked off tasks:
  - `[21]..[29]`, `[60]..[77]`

### Why
- These are the “real world” surfaces where protocol validation, request context, and capability enforcement can regress without unit tests catching the UX impact.

### What worked
- All expected failures are surfaced as actionable errors (including protocol validation and stdout contamination).
- Dynamic command registration is correctly gated by `command.run`.
- Root flags behave as expected for repo-root/config/timeouts and common CLI error paths.

### What didn't work
- N/A (all cases behaved as expected; strictness behavior is “error vs last-wins”, not “warn vs error”).

### What I learned
- The “no config present” behavior is nicely discoverable: `plan` returns `{}` but warns loudly; `up` hard-errors.

### What was tricky to build
- Avoiding self-matching when checking for leaked plugin processes (`ps | rg` patterns need a “bracket trick” to not match the grep/rg command line).

### What warrants a second pair of eyes
- Whether we want non-strict collisions to log warnings (today it is silent “last wins”), especially for service collisions.

### What should be done in the future
- If we want a more explicit UX for non-strict collisions, add warnings at merge sites (out of scope for this pass).

### Code review instructions
- Start with dynamic command discovery and request ctx wiring:
  - `devctl/cmd/devctl/cmds/dynamic_commands.go`
  - `devctl/pkg/runtime/client.go` (`requestContextFrom`)

### Technical details
- Representative error excerpts captured during this step:
  - `E_PROTOCOL_INVALID_HANDSHAKE: unsupported protocol_version "v1"`
  - `E_PROTOCOL_INVALID_JSON: NOT JSON: invalid character 'N' ...`
  - `E_PROTOCOL_STDOUT_CONTAMINATION: oops-not-json ...`

## Step 5: Wrapper Regression (Dynamic Discovery Stall) + Fix Verification

This step intentionally reproduced a real wrapper startup failure: because `AddDynamicPluginCommands` runs during `devctl` process startup, the supervisor’s internal `__wrap-service` invocation can be delayed by slow plugin handshakes long enough to trip the wrapper ready-file deadline.

After reproducing the failure deterministically, I fixed it by skipping dynamic command discovery when the process is executing `__wrap-service`, and verified the same stress scenario succeeds. I also fixed direct `__wrap-service` invocation to work outside supervisor by ensuring the wrapper becomes a process-group leader before wiring child process groups.

**Commit (code):** a6c4e52 — "wrap-service: skip dynamic discovery and setpgid"

### What I did
- Reproduced the failure with a config containing:
  - three “slow handshake” plugins (sleep before emitting handshake)
  - one pipeline plugin that launches a trivial service
- Observed `up` failing after wrapper start:
  - `Error: wrapper did not report child start`
- Implemented fixes:
  - Skip `AddDynamicPluginCommands` when first positional command is `__wrap-service`.
  - Call `syscall.Setpgid(0, 0)` in `__wrap-service` before starting the child.
  - Added a unit test: `TestDynamicCommands_SkipsWrapService`.
- Verified:
  - The same slow-handshake scenario now succeeds (`up complete`).
  - `__wrap-service` can be run directly and writes:
    - `--ready-file` with PID
    - `--exit-info` JSON (with `stderr_tail`)
  - Many-plugin stress (20× command plugins) no longer causes wrapper readiness failures.
- Checked off tasks:
  - `[45]`, `[78]`, `[79]`

### Why
- The wrapper ready-file deadline is intentionally short to catch “child never started”; it should not be sensitive to unrelated CLI startup behavior like dynamic command discovery.

### What worked
- Reproduction was deterministic (slow handshakes reliably trigger the failure pre-fix).
- Post-fix, wrapper startup is robust to slow plugin discovery because it no longer performs discovery at all.

### What didn't work
- Direct `__wrap-service` invocation initially failed with:
  - `Error: start child: fork/exec /usr/bin/bash: operation not permitted`

### What I learned
- `__wrap-service` implicitly depended on being started as a process-group leader by supervisor; making that invariant explicit prevents confusing “operation not permitted” failures when running it directly.

### What was tricky to build
- It’s easy to fix the symptom by lengthening the ready deadline; skipping discovery for internal commands is the more robust architectural fix.

### What warrants a second pair of eyes
- The interaction between Cobra startup hooks and internal subcommands is subtle; this skip needs to be maintained if we add more internal commands in the future.

### What should be done in the future
- Consider moving dynamic command discovery behind a lazily-initialized mechanism (only for “interactive” CLI commands) if startup cost becomes user-visible.

### Code review instructions
- Focus on the skip logic and the process-group change:
  - `devctl/cmd/devctl/cmds/dynamic_commands.go`
  - `devctl/cmd/devctl/cmds/wrap_service.go`
  - `devctl/cmd/devctl/cmds/dynamic_commands_test.go`

### Technical details
- Failure reproduction excerpt:
  - `Error: wrapper did not report child start`
- Direct wrapper success excerpt includes:
  - `exit_code: 0`
  - `stderr_tail` containing the child's stderr line.

## Step 6: TUI Testing in Tmux

This step validated the TUI functionality end-to-end using a tmux session for capture and testing. The goal was to verify all major TUI keybindings, view cycling, confirmations, and service management features work correctly with protocol v2.

**Commit (code):** N/A

### What I did
- Created tmux session with fixture at `/tmp/devctl-tui-fixture-*`
- Started TUI with `--alt-screen=false` for capture
- Tested view cycling: Dashboard → Events → Pipeline → Plugins → Dashboard (Tab key)
- Tested help toggle (?) shows keybindings for all views
- Tested up flow (u): ActionUp starts services, dashboard shows PIDs and status
- Tested j/k navigation: selection moves between services
- Tested service detail (l/enter): opens log view with stdout/stderr toggle (Tab)
- Tested follow toggle (f): switches between on/off
- Tested exit info display: dead services show exit code and stderr tail
- Tested down confirmation (d then y): stops services, returns to stopped state
- Tested restart confirmation (r then y): performs down+up with new PIDs
- Tested kill flow (x then n cancels, x then y sends SIGTERM): service transitions to dead
- Tested Events view: pause (p), clear (c), level menu (l), service filters [1-9]
- Tested Pipeline view: shows phases from restart operation
- Tested Plugins view: expand (enter) shows plugin details
- Tested `--alt-screen=true`: terminal resets cleanly on exit
- Tested `--refresh 100ms`: UI updates quickly without corruption
- Tested u when state exists: triggers restart confirmation (not second up)

### Why
- TUI is a major user-facing surface; manual validation ensures keybindings and state transitions work as documented in help.

### What worked
- All core keybindings function as expected
- View cycling is consistent (Dashboard → Events → Pipeline → Plugins)
- Confirmations for destructive actions (down, restart, kill) work correctly
- Service detail view shows logs with stdout/stderr toggle
- Exit info appears when services die
- Events pause/clear/level menu work
- Both alt-screen modes work correctly

### What didn't work
- N/A (all tested features work as expected)

### What I learned
- The TUI help overlay accurately reflects the implemented keybindings
- Events view has comprehensive filtering: by service [1-9], by level (d/i/w/e), pause/clear
- Kill confirmation shows PID being killed; SIGTERM event appears in logs

### What was tricky to build
- Capturing TUI output in tmux requires `--alt-screen=false` for reliable capture-pane

### What warrants a second pair of eyes
- Events view high-frequency polling (1/sec state: loaded) could be optimized

### What should be done in the future
- Consider throttling "state: loaded" events in the Events view

### Code review instructions
- Focus on TUI model implementations:
  - `devctl/pkg/tui/models/dashboard_model.go`
  - `devctl/pkg/tui/models/service_model.go`
  - `devctl/pkg/tui/models/events_model.go`
  - `devctl/pkg/tui/models/pipeline_model.go`
  - `devctl/pkg/tui/models/plugins_model.go`

### Technical details
- Fixture used: MO-006 fixture at `/tmp/devctl-tui-fixture-*`
- Services tested: http (long-running), spewer (exits with code 2)
- All views tested: Dashboard, Events, Pipeline, Plugins

<!-- Provide background context needed to use this reference -->

## Quick Reference

<!-- Provide copy/paste-ready content, API contracts, or quick-look tables -->

## Usage Examples

<!-- Show how to use this reference in practice -->

## Related

<!-- Link to related documents or resources -->
