---
Title: Diary
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
    - Path: devctl/cmd/devctl/cmds/up.go
      Note: End-to-end up pipeline (mutate/validate/plan + supervisor + state)
    - Path: devctl/pkg/engine/pipeline.go
      Note: Phase pipeline pieces implemented so far (config.mutate + launch.plan merge)
    - Path: devctl/pkg/supervise/supervisor.go
      Note: Plan-mode service supervision (start/stop + health + log capture)
    - Path: moments/plugins/moments-plugin.py
      Note: Moments plugin implementing phases and launch plan
    - Path: moments/ttmp/2026/01/06/MO-005-IMPROVE-STARTDEV--improve-startdev-sh-script-analysis-and-enhancement/design-doc/04-script-plugin-protocol-config-mutation-build-prepare-validate-launch-monitor-logs-commands.md
      Note: Source-of-truth protocol design the task list is based on
    - Path: moments/ttmp/2026/01/06/MO-005-IMPROVE-STARTDEV--improve-startdev-sh-script-analysis-and-enhancement/design-doc/05-go-runner-architecture-ndjson-plugin-protocol-runner.md
      Note: Go architecture reference used to implement devctl
    - Path: moments/ttmp/2026/01/06/MO-005-IMPROVE-STARTDEV--improve-startdev-sh-script-analysis-and-enhancement/tasks.md
      Note: Concrete implementation task breakdown for the generic runner
ExternalSources: []
Summary: Step-by-step research diary documenting the analysis of startdev.sh script
LastUpdated: 2026-01-06T15:17:16-05:00
WhatFor: Tracking research process, findings, and decisions made during startdev.sh analysis
WhenToUse: When reviewing how the analysis was conducted or continuing work on startdev.sh improvements
---











# Diary

## Goal

Document the exhaustive step-by-step analysis of `moments/scripts/startdev.sh`, capturing every function, variable, command, and interaction with detailed pseudocode and code references. This diary tracks the research process, code exploration, and document creation.

Session:          019b94f5-80e5-7dd3-ac1d-795982d224c7

## Step 1: Ticket Creation and Initial Exploration

Created ticket MO-005-IMPROVE-STARTDEV and began exploring the codebase to understand startdev.sh and its dependencies. The script was already attached to the conversation, so I started by understanding what it does at a high level, then identified all the files it interacts with.

**Commit (code):** N/A — Analysis phase only

### What I did
- Created ticket using `docmgr ticket create-ticket`
- Read `moments/scripts/startdev.sh` (already attached)
- Searched for related files: Makefile, moments-config CLI, bootstrap script
- Read `moments/backend/Makefile` to understand `make run` target
- Read `moments/backend/scripts/bootstrap-startup.sh` to understand bootstrap process

### Why
- Need to understand the complete execution flow before writing analysis
- Identify all dependencies and interactions
- Map out the configuration resolution flow

### What worked
- Found all key files quickly using codebase search
- Makefile clearly shows `run` depends on `bootstrap`
- Bootstrap script is well-documented with comments

### What I learned
- `startdev.sh` orchestrates both backend (`make run`) and frontend (`pnpm run dev`)
- Configuration resolution uses `moments-config` CLI to read YAML config
- Bootstrap process ensures DB exists, runs migrations, and generates JWT keys
- Script uses `lsof` for port conflict detection and resolution

### What was tricky to build
- Understanding the configuration flow: `startdev.sh` → `moments-config get` → `appconfig` → YAML files
- Tracing how `VITE_*` environment variables propagate to Vite dev server
- Understanding the relationship between `make bootstrap` and `bootstrap-startup.sh`

### What warrants a second pair of eyes
- Verify the configuration key names match actual YAML structure
- Confirm port defaults are correct (backend_url defaults to 8082 but backend runs on 8083)
- Review the health checking logic (only checks ports, not actual HTTP health)

### What should be done in the future
- Verify configuration keys exist in YAML files
- Test the script end-to-end to validate analysis
- Consider adding actual HTTP health checks instead of just port checks

### Code review instructions
- Start with `moments/scripts/startdev.sh` to understand main flow
- Review `moments/backend/Makefile` lines 28-39 for `bootstrap` and `run` targets
- Check `moments/backend/scripts/bootstrap-startup.sh` for bootstrap implementation

### Technical details
- Script uses `set -Eeuo pipefail` for strict error handling
- Configuration keys use dot-notation: `platform.mento-service-public-base-url`
- Port checking uses `lsof -Pi :PORT -sTCP:LISTEN -t`
- Background processes started with `&` and PIDs captured

## Step 2: Deep Dive into moments-config CLI

Explored the moments-config CLI implementation to understand how configuration values are resolved. This is critical because startdev.sh relies heavily on this CLI for configuration resolution.

**Commit (code):** N/A — Analysis phase only

### What I did
- Read `moments/backend/cmd/moments-config/main.go` - entry point
- Read `moments/backend/cmd/moments-config/get_cmd.go` - get command implementation
- Read `moments/backend/cmd/moments-config/bootstrap_config.go` - bootstrap config computation
- Traced how `cfg_get()` function calls the CLI
- Understood the supported configuration keys

### Why
- Need to document how configuration resolution works
- Understand what keys are supported and how they map to YAML
- See how appconfig package is used

### What worked
- CLI code is well-structured with clear separation of concerns
- `get_cmd.go` shows all supported keys explicitly
- `bootstrap_config.go` shows how values are computed from appconfig

### What I learned
- Supported keys: `repo_root`, `database_url`, `db_name`, `db_admin_url`, `keys_dir`, `private_key_filename`, `public_key_filename`, `openssl_bin`
- CLI uses `appconfig` package to load YAML files
- Configuration resolution respects environment variable overrides (MOMENTS_* prefix)
- `computeBootstrapConfig()` function does the heavy lifting

### What was tricky to build
- Understanding how `appconfig` package loads and merges YAML files
- Tracing the path from YAML key to CLI output
- Understanding the relationship between bootstrap config and appconfig settings

### What warrants a second pair of eyes
- Verify that the keys used in startdev.sh (`platform.mento-service-public-base-url`, etc.) actually exist in YAML files
- Confirm the appconfig initialization flow matches what's documented
- Check if there are other configuration keys that should be supported

### What should be done in the future
- Add validation to ensure configuration keys exist before using them
- Document the full list of supported configuration keys
- Consider adding a `list-keys` command to moments-config CLI

### Code review instructions
- Review `moments/backend/cmd/moments-config/get_cmd.go` lines 31-72 for supported keys
- Check `moments/backend/cmd/moments-config/bootstrap_config.go` lines 102-178 for config computation
- Verify appconfig package usage matches expectations

### Technical details
- CLI uses Cobra for command structure
- `get` command takes a single key argument
- Bootstrap config computed from multiple appconfig settings (bootstrapcfg, keycfg, sqlcfg, platform)
- Database URL extracted from connection string via URL parsing

## Step 3: Understanding Backend Startup Flow

Traced the complete backend startup flow from `make run` through bootstrap to server initialization. This is complex because it involves multiple layers: Makefile → bootstrap script → migrations → server startup.

**Commit (code):** N/A — Analysis phase only

### What I did
- Read `moments/backend/Makefile` lines 28-39 for `bootstrap` and `run` targets
- Read `moments/backend/scripts/bootstrap-startup.sh` completely
- Read `moments/backend/cmd/moments-server/serve.go` to understand server initialization
- Traced the execution flow: make run → bootstrap → go run serve
- Understood how ldflags inject version information

### Why
- Need to document the complete backend startup process
- Understand dependencies between steps
- See how database and keys are initialized

### What worked
- Makefile clearly shows dependencies (`run: bootstrap`)
- Bootstrap script is well-commented and idempotent
- Server code shows route registration and initialization order

### What I learned
- `make bootstrap` builds moments-config CLI, then runs bootstrap-startup.sh
- Bootstrap script: waits for Postgres, ensures DB exists, runs migrations, generates keys
- `make run` sets PORT env var, gets git commit/build time, runs `go run` with ldflags
- Server initialization: loads appconfig, connects DB/Redis, registers routes, starts HTTP server
- Version info injected via ldflags: `-X main.gitCommit` and `-X main.buildTime`

### What was tricky to build
- Understanding the relationship between Makefile targets and shell scripts
- Tracing how environment variables propagate through make → go run → server
- Understanding the appconfig initialization in serve.go

### What warrants a second pair of eyes
- Verify that bootstrap script handles all edge cases (DB already exists, migrations already run, keys already exist)
- Check that version information is correctly injected and accessible
- Confirm that all routes are registered in the correct order (health before SPA handler)

### What should be done in the future
- Add health check endpoint verification to startdev.sh (not just port check)
- Consider adding startup time metrics
- Document the route registration order and why it matters

### Code review instructions
- Start with `moments/backend/Makefile` lines 28-39
- Review `moments/backend/scripts/bootstrap-startup.sh` lines 226-238 (main function)
- Check `moments/backend/cmd/moments-server/serve.go` lines 68-406 (Run function)

### Technical details
- Bootstrap script uses `moments-config bootstrap env` to get configuration
- Migrations run via `go run ./cmd/moments-server migrate up`
- JWT keys generated with OpenSSL: `genrsa` for private, `rsa -pubout` for public
- Server uses Gorilla Mux for routing, registers SPA handler last (catch-all)

## Step 4: Understanding Frontend Startup Flow

Explored the frontend startup process to understand how Vite dev server is configured and how it uses the environment variables set by startdev.sh.

**Commit (code):** N/A — Analysis phase only

### What I did
- Read `moments/web/package.json` to see dev script
- Read `moments/web/vite.config.mts` to understand Vite configuration
- Traced how `VITE_*` environment variables are used
- Understood the proxy configuration
- Checked how `pnpm generate-version` works

### Why
- Need to document frontend startup process
- Understand how configuration propagates to frontend
- See how proxy routes are configured

### What worked
- package.json clearly shows dev script: `pnpm generate-version && vite`
- vite.config.mts shows proxy configuration using env vars
- Vite's `loadEnv()` function loads `VITE_*` prefixed variables

### What I learned
- `pnpm run dev` runs `generate-version` then `vite`
- Vite reads `VITE_*` environment variables via `loadEnv()`
- Proxy routes configured in vite.config.mts:
  - `/api/v1/*` → Identity backend
  - `/rpc/v1/*` → Backend (with WebSocket support)
  - `/config.js` → Identity backend (runtime config)
- Frontend logs to both terminal and log file using `tee -a`

### What was tricky to build
- Understanding how Vite's proxy works and when it's used
- Tracing which routes go to which backend
- Understanding the difference between `VITE_BACKEND_URL` and `VITE_IDENTITY_BACKEND_URL`

### What warrants a second pair of eyes
- Verify proxy configuration matches actual backend routes
- Check that WebSocket support is correctly configured for `/rpc/v1/*`
- Confirm that `/config.js` endpoint exists and works correctly

### What should be done in the future
- Document the proxy routing rules clearly
- Add validation that backend URLs are reachable before starting frontend
- Consider adding health checks for proxy endpoints

### Code review instructions
- Review `moments/web/vite.config.mts` lines 69-103 for proxy configuration
- Check `moments/web/package.json` line 7 for dev script
- Verify environment variable usage in vite.config.mts lines 34-37

### Technical details
- Vite only exposes `VITE_*` prefixed variables to frontend code
- Proxy uses `changeOrigin: true` for CORS handling
- WebSocket support enabled for `/rpc/v1/*` routes
- SPA fallback plugin handles client-side routing

## Step 5: Writing Comprehensive Analysis Document

Created the exhaustive analysis document with step-by-step breakdown, pseudocode, code references, and detailed explanations of every function and interaction.

**Commit (code):** N/A — Documentation only

### What I did
- Created analysis document using `docmgr doc add`
- Wrote comprehensive analysis covering all phases of script execution
- Included pseudocode for every function
- Added code references with line numbers
- Documented dependencies, error handling, process management
- Included configuration flow diagrams
- Documented limitations and improvement opportunities

### Why
- User requested exhaustive analysis with pseudocode and code references
- Need to document every detail for future maintenance and enhancement
- Provide reference for understanding script behavior

### What worked
- Structured analysis by execution phases
- Used pseudocode to clarify logic flow
- Included code references for easy navigation
- Documented both what happens and why

### What didn't work
- Initial attempt was too verbose, had to balance detail with readability
- Some sections needed multiple revisions to get the right level of detail

### What I learned
- Breaking down by phases makes the analysis more digestible
- Pseudocode helps clarify complex bash logic
- Code references are essential for maintainability
- Documenting limitations is as important as documenting functionality

### What was tricky to build
- Balancing exhaustiveness with readability
- Deciding how much detail to include for each step
- Organizing the information in a logical flow
- Ensuring all code references are accurate

### What warrants a second pair of eyes
- Verify all code references point to correct lines
- Check that pseudocode accurately represents bash logic
- Confirm that configuration flow diagram is correct
- Review improvement opportunities for feasibility

### What should be done in the future
- Test the analysis against actual script execution
- Add more examples of error scenarios
- Consider adding a troubleshooting guide
- Create a visual diagram of the execution flow

### Code review instructions
- Read analysis document from top to bottom
- Verify code references by checking actual files
- Test pseudocode logic against actual script
- Review improvement opportunities section

### Technical details
- Analysis document is ~1000+ lines covering all aspects
- Includes 9 phases of execution
- Documents 10+ functions and their interactions
- References 10+ related files with line numbers

## Step 6: Relating Files and Updating Documentation

Related all relevant files to the analysis document and updated metadata to make the documentation searchable and linked.

**Commit (code):** N/A — Documentation only

### What I did
- Used `docmgr doc relate` to link 9 files to analysis document
- Added descriptive notes explaining why each file matters
- Updated diary with research steps
- Prepared for changelog update

### Why
- File relationships make documentation searchable
- Helps future readers understand dependencies
- Enables reverse lookups (find docs referencing a file)

### What worked
- docmgr tool made it easy to relate multiple files at once
- File notes provide context for why each file is important
- All key files successfully related

### What I learned
- File relationships are crucial for documentation discoverability
- Notes should explain why a file matters, not just that it's related
- Using absolute paths ensures relationships work across environments

### What was tricky to build
- Deciding which files to relate (not too many, not too few)
- Writing concise but informative file notes
- Ensuring absolute paths are correct

### What warrants a second pair of eyes
- Verify all related files are actually relevant
- Check that file notes are accurate and helpful
- Confirm absolute paths are correct

### What should be done in the future
- Add more files if they become relevant during implementation
- Update file notes as understanding deepens
- Consider adding reverse relationship documentation

### Code review instructions
- Review related files list in analysis document frontmatter
- Check file notes for accuracy and usefulness
- Verify file paths are absolute and correct

### Technical details
- Related 9 files: startdev.sh, Makefile, bootstrap-startup.sh, moments-config files, serve.go, vite.config.mts, package.json
- All files use absolute paths from workspace root
- File notes explain each file's role in the script execution

## Step 7: Review the Go-based replacement design (and tighten the plugin story)

This step pressure-tested the “Go replacement + plugin protocol” design as if we were about to implement it. The key outcome was identifying where the design is strong (phase separation, typed config, observability) and where it risks becoming too big or too fragile (especially the stdio protocol details). The review also pulled in the configuration analysis finding that `moments-config get` cannot currently return the dot-notation keys that `startdev.sh` tries to read, which changes what “compatibility” should mean for the Go tool.

**Commit (code):** N/A — Documentation only

### What I did
- Wrote a dedicated architecture review doc: `analysis/03-review-go-based-startdev-replacement-architecture.md`
- Assessed the plugin protocol for framing, lifecycle, determinism, cancellation, and safety
- Recommended an MVP scope that replaces `startdev.sh` without building a “platform” up-front

### Why
- A design doc that’s “flexible” can hide high implementation risk unless we explicitly pin down contracts and scope
- Stdio plugin protocols are easy to get subtly wrong (stdout contamination, framing, timeouts)

### What worked
- The phase separation maps well to the real operational phases of dev startup
- The plugin idea is viable if we constrain it to a robust framed protocol with handshake and stderr-only logs

### What I learned
- Without message framing + stdout/stderr separation, a stdio protocol will be brittle in real teams
- The Go tool should integrate directly with `backend/pkg/appconfig` rather than shelling out to `moments-config`

### What was tricky to build
- Making the plugin story “powerful” without making the orchestrator non-deterministic and hard to support
- Defining safe merge rules when multiple plugins modify the same phase output

### What warrants a second pair of eyes
- Whether NDJSON vs length-prefixed framing is better for our plugin ecosystem
- The proposed “core vs plugin responsibility” boundary (what plugins are allowed to change by default)

### What should be done in the future
- Decide on a strict MVP feature set and a de-scope list
- If we keep `moments-config` in the loop for compatibility, add explicit commands for the values dev tooling needs (e.g., `vite-env`) or make the new tool the canonical path

### Code review instructions
- Start with the review doc: `analysis/03-review-go-based-startdev-replacement-architecture.md`
- Cross-check the config mismatch described in `analysis/02-moments-config-and-configuration-phase-analysis.md`

## Step 8: Create an up-to-date task breakdown for the generic runner

This step turned the latest “script plugin protocol” design into a concrete, implementable task list for the generic runner. The goal was to translate the protocol into work that can be executed incrementally (protocol types → process runner → merge rules → phase pipeline → UX), while keeping compatibility with the real `startdev.sh` behavior in view.

The task list intentionally anchors on `design-doc/04-...` (newest version) and uses the `startdev.sh` analysis as a reality check on what “MVP parity” means (ports, pnpm install, logs, basic health waiting).

**Commit (code):** N/A — Tasks + documentation only

### What I did
- Read `design-doc/04-script-plugin-protocol-config-mutation-build-prepare-validate-launch-monitor-logs-commands.md` and re-validated the intended protocol surface
- Re-scanned `analysis/01-startdev-sh-complete-step-by-step-analysis.md` for the concrete behaviors the runner must support in practice
- Added a sequenced set of tasks to `tasks.md` for building the generic runner (protocol, runner, merge rules, phase pipeline, streams, commands, examples)

### Why
- We need a task list that reflects the most current design and can drive implementation work in small, reviewable increments
- Earlier design exploration drifted; anchoring the tasks on the newest protocol doc reduces rework risk

### What worked
- The protocol doc’s “Implementation Plan” maps cleanly to runner-centric tasks (discovery → framing → patches → pipeline → commands/examples)
- The `startdev.sh` analysis provides a clear parity checklist for what the MVP needs to feel usable day-to-day

### What didn't work
- N/A

### What I learned
- The highest-risk part of the runner is framing + supervision invariants (stdout purity, cancellation, timeouts, stream routing)
- Making merge rules explicit early is essential for deterministic composition when multiple plugins participate

### What was tricky to build
- Writing tasks that are “generic runner” work (protocol + engine) without prematurely hard-coding Moments-specific behaviors into the core
- Sequencing tasks so we can implement and test incrementally without requiring the full CLI UX finished up-front

### What warrants a second pair of eyes
- Whether the MVP ordering is right (e.g., do we need stream support before plan-mode launch is shippable?)
- Whether “strictness / collision policy” should be implemented earlier to avoid subtle nondeterminism

### What should be done in the future
- Break the “Spike: Moments plugin set” task into smaller follow-ups once the runner skeleton exists (config, build/prepare, launch plan, logs)

### Code review instructions
- Review the runner task list in `moments/ttmp/2026/01/06/MO-005-IMPROVE-STARTDEV--improve-startdev-sh-script-analysis-and-enhancement/tasks.md`
- Cross-check the task wording against `moments/ttmp/2026/01/06/MO-005-IMPROVE-STARTDEV--improve-startdev-sh-script-analysis-and-enhancement/design-doc/04-script-plugin-protocol-config-mutation-build-prepare-validate-launch-monitor-logs-commands.md`
- Confirm IDs/order via `docmgr task list --ticket MO-005-IMPROVE-STARTDEV`

### Technical details
- Commands run:
  - `docmgr task add --ticket MO-005-IMPROVE-STARTDEV --text \"…\"`
  - `docmgr task list --ticket MO-005-IMPROVE-STARTDEV`

## Step 9: Scaffold a fresh `devctl/` Go repo and get a first end-to-end smoketest passing

This step started implementation in a fresh `devctl/` repository. The immediate goal was to prove the core invariant end-to-end: spawn a plugin process, read a handshake frame, send a request, receive a response, and keep stdout strictly protocol-only.

The milestone is intentionally small but foundational: a minimal Cobra CLI with a `dev smoketest` command, a first-cut protocol package, a config patch package with unit tests, and a runtime client capable of handshake + request/response. This gives us a compiling base to iterate on engine + supervisor next.

**Commit (code):** 38a39b93ec00a03f51630f61ffa23185abe6b683 — "devctl: scaffold protocol, runtime, smoketest"

### What I did
- Created a fresh Go module in `devctl/` with `cmd/devctl` and `pkg/{protocol,patch,runtime}`
- Implemented:
  - protocol v1 structs + handshake validation
  - dotted-path `ConfigPatch` apply/merge with unit tests
  - plugin runtime: start process, read handshake, correlate request/response, enforce stdout JSON-only, and a `dev smoketest` cobra command
- Added a tiny Python test plugin in `devctl/testdata/plugins/ok-python/plugin.py` used by both unit tests and the CLI smoketest

### Why
- We needed a compiling “thin slice” that validates the protocol wiring and provides a stable base for the next layers (engine + supervisor)
- A smoke test under the CLI makes regressions obvious while iterating on protocol/runtime behavior

### What worked
- `go test ./...` passes for the initial protocol/patch/runtime implementation
- `go run ./cmd/devctl dev smoketest` exercises handshake + request/response and returns `ok`

### What didn't work
- Initial `go test ./...` failed because the parent repository’s `go.work` did not include the new module; adding a local `devctl/go.work` isolated the module cleanly

### What I learned
- Go workspaces (`go.work`) in parent directories can break “fresh repo” module work unless you isolate with a closer `go.work` (or run with `GOWORK=off`)
- The runtime needs a dedicated “line reader with context” to avoid `bufio.Scanner` token limits; the current implementation uses `ReadBytes('\\n')`

### What was tricky to build
- Ensuring stdout is treated as protocol-only while still capturing stderr for human logs without contaminating frames
- Getting a reliable cross-test plugin implementation: using a Python plugin avoids bash+jq dependencies and makes parsing deterministic

### What warrants a second pair of eyes
- The runtime implementation currently treats any non-JSON stdout as a hard failure and “fails all pending requests”; review whether that error propagation strategy is what we want long-term
- Process termination semantics (process-group kill) should be reviewed carefully once we add long-running supervision

### What should be done in the future
- Add stream support to the runtime API (`StartStream`) and tests for event routing and stream end handling
- Add a “noisy stdout” test plugin fixture to ensure contamination is caught during handshake and after handshake
- Start implementing the engine pipeline and plan-mode supervisor on top of this runtime

### Code review instructions
- Start with the CLI entrypoint: `/home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/cmd/devctl/main.go`
- Review the smoketest command: `/home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/cmd/devctl/cmds/dev/smoketest/root.go`
- Review protocol + validation: `/home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/pkg/protocol/types.go`
- Review runtime handshake/call routing: `/home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/pkg/runtime/factory.go`
- Validate:
  - `cd devctl && go test ./...`
  - `cd devctl && go run ./cmd/devctl dev smoketest`

### Technical details
- Commands run (key ones):
  - `cd devctl && go mod tidy`
  - `cd devctl && go test ./...`
  - `cd devctl && go run ./cmd/devctl dev smoketest`
  - `cd devctl && git commit -m \"devctl: scaffold protocol, runtime, smoketest\"`

## Step 10: Harden the runtime with stream support and stdout contamination tests

This step expanded the plugin runtime beyond simple request/response by adding a first usable stream primitive and by tightening failure behavior around stdout contamination. The key goal was to make the runtime robust enough to support `logs.follow` / monitor-style streaming without dropping early events or hanging after a protocol violation.

The milestone added both fixture plugins and unit tests so we can iterate on engine/supervisor work with confidence: noise before handshake must fail startup, noise after handshake must poison the client, and a simple stream must deliver all events deterministically.

**Commit (code):** 785a5f8c7976b3c6f1fd4bffcb7017d2c4f57b08 — "runtime: add stream support and stdout contamination handling"  
**Commit (code):** 18a9589a30f7cd5b1ce7ab13161d71739983a80e — "cleanup: remove placeholder cmd/XXX"

### What I did
- Added `Client.StartStream` and event routing via `stream_id`
- Implemented buffering for stream events that arrive before the caller subscribes (avoids race between response arrival and first events)
- Added test plugins:
  - `noisy-handshake`: prints non-JSON before handshake (startup must fail)
  - `noisy-after-handshake`: prints non-JSON after handshake (client becomes fatally errored)
  - `stream`: emits a small log stream with an `end` event
- Added unit tests covering all three fixtures

### Why
- Streaming is required for logs/monitor output and needs to be correct before we build UX on top of it
- Protocol violations must fail fast and leave the client in a safe “poisoned” state (no hangs, no silent drops)

### What worked
- `go test ./...` passes with stream + contamination fixtures
- `devctl dev smoketest` still passes

### What didn't work
- The initial stream implementation dropped the first event due to a race (events could arrive between response and subscribe); fixed by buffering events until a subscriber is present

### What I learned
- Even with NDJSON and explicit `stream_id`, you need a concrete strategy for “events-before-subscribe” to avoid subtle flakes
- Once a plugin violates stdout purity, the runtime must enter a terminal failure mode; otherwise later requests can hang forever

### What was tricky to build
- Designing the buffering logic so it preserves ordering and still closes the stream cleanly on an `end` event (including when `end` arrives before subscribe)

### What warrants a second pair of eyes
- Buffering/backpressure policy: current buffering is unbounded per stream until first subscribe; we may want caps once real logs are involved
- Whether we should surface a richer typed error for “poisoned client” vs returning generic `E_RUNTIME`

### What should be done in the future
- Add explicit `logs.follow` protocol shapes and a CLI UX around streams (filters, multi-service follow)
- Implement better shutdown semantics (Close should respect caller context and return the underlying process exit error deterministically)

### Code review instructions
- Runtime stream + contamination routing: `/home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/pkg/runtime/router.go`
- Stream API plumbing: `/home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/pkg/runtime/client.go`
- Fixture-driven runtime tests: `/home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/pkg/runtime/runtime_test.go`
- Validate:
  - `cd devctl && go test ./...`
  - `cd devctl && go run ./cmd/devctl dev smoketest`

### Technical details
- Commands run (key ones):
  - `cd devctl && go test ./...`
  - `cd devctl && git commit -m \"runtime: add stream support and stdout contamination handling\"`
  - `cd devctl && git commit -m \"cleanup: remove placeholder cmd/XXX\"`

## Step 11: Add repo config, plugin discovery, and a minimal pipeline (`plan` + `plugins list`)

This step started connecting the runtime to a more “real” CLI shape: load a repo config (`.devctl.yaml`), discover plugins in a deterministic order, and run a minimal pipeline that can call `config.mutate` and merge `launch.plan` outputs. The goal is to get to a place where we can iterate on engine/supervisor behavior without rewriting wiring each time.

The milestone also adds basic engine tests so ordering and strictness semantics don’t regress while we build more phases.

**Commit (code):** b12826591c904a707f7c101a4e91de306fa6b0d9 — "devctl: add config/discovery and plan/plugins commands"  
**Commit (code):** 42182216818e397876a612db6d4c3c31d8ed2a2c — "plan: honor config strictness"

### What I did
- Added repo config parsing (`.devctl.yaml`) with plugin entries (`id/path/args/priority/workdir/env`)
- Implemented discovery:
  - config-driven plugin list (deterministic ordering by priority then id)
  - auto-discovery of executable `plugins/devctl-*` files (default priority 1000)
- Added CLI commands:
  - `devctl plugins list` (runs handshake and prints capabilities)
  - `devctl plan` (runs `config.mutate` then merges `launch.plan`, prints `{config, plan}`)
- Implemented engine pipeline pieces:
  - plugin ordering
  - `config.mutate` patch application loop
  - `launch.plan` merge by service name with `--strict` collision behavior
- Added engine tests for ordering and strict collision behavior

### Why
- We need a stable “driver” to exercise plugins and phase orchestration before building supervision/log UX
- Discovery + ordering is the foundation for deterministic composition across multiple repo plugins

### What worked
- `cd devctl && go test ./...` passes (including engine ordering/strictness tests)
- Example config works:
  - `cd devctl && go run ./cmd/devctl --config .devctl.example.yaml plugins list`
  - `cd devctl && go run ./cmd/devctl --config .devctl.example.yaml plan`

### What didn't work
- N/A

### What I learned
- Even for “plan-only” workflows, the CLI needs a first-class repo-root/config story early, otherwise every test run becomes bespoke
- Keeping ordering rules explicit (priority then id) makes test expectations stable

### What was tricky to build
- Avoiding the “anonymous struct type” trap in tests; using JSON round-tripping in the fake client keeps test doubles simple and close to real behavior

### What warrants a second pair of eyes
- Whether auto-discovery should include non-executable scripts (would require interpreter inference or config hints)
- Whether plan merging should default to strict errors for collisions (right now it’s `--strict` / `strictness: error`)

### What should be done in the future
- Implement `validate.run`, `build.run`, and `prepare.run` in the engine pipeline
- Add more protocol error codes and unify error reporting between runtime and engine

### Code review instructions
- Config + discovery:
  - `/home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/pkg/config/config.go`
  - `/home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/pkg/discovery/discovery.go`
- Engine pipeline:
  - `/home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/pkg/engine/pipeline.go`
  - `/home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/pkg/engine/pipeline_test.go`
- CLI wiring:
  - `/home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/cmd/devctl/cmds/plan.go`
  - `/home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/cmd/devctl/cmds/plugins.go`
- Validate:
  - `cd devctl && go test ./...`
  - `cd devctl && go run ./cmd/devctl --help`

### Technical details
- Example config file: `/home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/.devctl.example.yaml`

## Step 12: Extend the engine with `validate.run`, `build.run`, and `prepare.run` aggregation

This step pushes the engine beyond “plan-only” by implementing the remaining MVP pipeline aggregators: validate, build, and prepare. The immediate goal is not to run real builds yet, but to define and test deterministic merge behavior so that multiple plugins can participate without surprises.

It also adds a simple fixture plugin that implements `config.mutate`, `validate.run`, and `launch.plan` so we can manually exercise the pipeline from the CLI with non-empty output.

**Commit (code):** b54476d52e979f874749a215a278c2af1790fc1c — "engine: add validate/build/prepare aggregation"

### What I did
- Added engine result types:
  - `ValidateResult` (valid + errors/warnings)
  - `BuildResult` / `PrepareResult` (step results + artifacts)
- Implemented pipeline aggregation methods:
  - `Pipeline.Validate` (valid=AND; append errors/warnings)
  - `Pipeline.Build` and `Pipeline.Prepare` (merge steps/artifacts; collision policy respects `Strict`)
- Added tests for validate merge behavior and build step collision strictness
- Added a fixture plugin: `/home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/testdata/plugins/pipeline/plugin.py`

### Why
- `validate.run` needs to exist before we can safely launch and supervise services
- `build.run` and `prepare.run` are core phases of replacing `startdev.sh` behavior, but we want deterministic composition rules first

### What worked
- `cd devctl && go test ./...` passes with new engine tests
- Manual pipeline output works with a simple config that points at the fixture plugin

### What didn't work
- N/A

### What I learned
- Encoding engine test doubles via JSON round-tripping keeps them close to the real protocol shapes (and avoids anonymous-struct pitfalls)

### What was tricky to build
- Choosing collision semantics that are deterministic and testable (default “last wins” with `--strict` to error)

### What warrants a second pair of eyes
- Whether build/prepare collisions should “last wins” or always error by default (even without `--strict`)
- Whether we should introduce structured warnings for collisions rather than silently overwriting

### What should be done in the future
- Wire `validate.run` into `devctl plan` (or a `devctl validate` command) before we start building supervision
- Add step-level selection behavior (`steps` allowlists) and strict handling for unknown steps

### Code review instructions
- Engine pipeline implementation: `/home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/pkg/engine/pipeline.go`
- Engine tests: `/home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/pkg/engine/pipeline_test.go`
- Fixture plugin: `/home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/testdata/plugins/pipeline/plugin.py`
- Validate:
  - `cd devctl && go test ./...`
  - `cd devctl && go run ./cmd/devctl --config .devctl.example.yaml plan`

### Technical details
- Protocol error constants expanded in `/home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/pkg/protocol/errors.go`

## Step 13: Implement plan-mode supervision (`up/down/status/logs`) with a state file and a smoke test

This step added the first operationally useful behavior: take a `launch.plan` and actually start processes, capture logs, wait for readiness, and stop them later. The goal was to get a minimal “devctl can bring up services” loop working end-to-end before we attempt richer UX or more phases.

The implementation is intentionally simple and file-based: `devctl up` writes `.devctl/state.json` under the repo root, and `devctl down/status/logs` operate off that persisted state.

**Commit (code):** 5bcfd486d5f81132aa31d9a5b836a596946b1e4c — "supervise: add plan-mode service start/stop/status/logs"

### What I did
- Added `pkg/state` for persisted runner state:
  - `.devctl/state.json` for service PIDs + log file paths
  - `.devctl/logs/` for per-service stdout/stderr logs
- Added `pkg/supervise`:
  - start services from `launch.plan` (cwd/env/command, process group)
  - health checks (tcp/http) with a ready timeout
  - stop services by PID / process group with SIGTERM→SIGKILL fallback
  - reap child processes in the running `devctl` process to avoid zombies
- Wired CLI commands:
  - `devctl up` (config.mutate → validate.run → launch.plan → supervise → persist state)
  - `devctl down` (stop from state + remove state)
  - `devctl status` (report alive/dead per PID + log paths)
  - `devctl logs --service <name> [--stderr] [--follow]`
- Added a smoke-test command `devctl dev smoketest supervise` which brings up a tiny HTTP server (via a fixture plugin) and performs an HTTP GET

### Why
- We needed a concrete, testable implementation of plan-mode supervision before working on richer orchestration and logs UX
- Persisting state enables a usable multi-command workflow (`up` then `status/logs` then `down`)

### What worked
- `cd devctl && go test ./...` passes (including a supervisor start/stop test)
- `cd devctl && go run ./cmd/devctl dev smoketest supervise` starts an HTTP server, verifies it, and tears it down

### What didn't work
- The first supervisor version did not reap child processes, which caused the test PID to remain “alive” as a zombie; fixed by `go cmd.Wait()` after `cmd.Start()`

### What I learned
- If `devctl` is the parent process for services (tests and long-running sessions), we must reap via `Wait` to avoid zombie PID false-positives
- A simple state file is enough to enable the common workflow without building a full daemon

### What was tricky to build
- Getting stop semantics correct without requiring the original `exec.Cmd` objects (supporting `down` in a fresh process via PID/process-group kill)
- Implementing `--follow` log tailing in a portable way without external `tail -f`

### What warrants a second pair of eyes
- Process-group kill behavior across platforms; current implementation assumes unix semantics (`Setpgid` + negative PGID kill)
- Health check semantics: currently http considers any 2xx–4xx response “ready”; we may want stricter expectations per service

### What should be done in the future
- Add `devctl up --build/--prepare` wiring and step selection once build/prepare plugins exist
- Add UX for showing last N log lines and multi-service follow

### Code review instructions
- State persistence: `/home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/pkg/state/state.go`
- Supervisor core: `/home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/pkg/supervise/supervisor.go`
- CLI wiring:
  - `/home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/cmd/devctl/cmds/up.go`
  - `/home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/cmd/devctl/cmds/down.go`
  - `/home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/cmd/devctl/cmds/status.go`
  - `/home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/cmd/devctl/cmds/logs.go`
- Smoke test:
  - `/home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/cmd/devctl/cmds/dev/smoketest/supervise.go`
  - `/home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/testdata/plugins/http-service/plugin.py`
- Validate:
  - `cd devctl && go test ./...`
  - `cd devctl && go run ./cmd/devctl dev smoketest supervise`

## Step 14: Implement plugin-provided commands with dynamic Cobra subcommands

This step added support for the protocol’s “git-xxx style” command extensions: plugins can register commands via `commands.list`, and `devctl` exposes them as first-class subcommands. The goal was to make repo-specific workflows pluggable without bloating the core CLI.

The implementation builds dynamic subcommands at startup by discovering plugins (using `--repo-root/--config` from argv), asking each plugin for `commands.list`, and then generating Cobra commands that dispatch to `command.run`.

**Commit (code):** 02522903f1ee769e4d45aa81a9353b5f60cc61ac — "commands: add commands.list and dynamic command dispatch"

### What I did
- Added dynamic command registration during CLI startup:
  - discover plugins
  - call `commands.list`
  - generate `devctl <name> ...` cobra commands
- Implemented dispatch path:
  - compute config via `config.mutate`
  - call `command.run` with `{name, argv, config}`
- Added a fixture plugin implementing `commands.list` + `command.run` (`echo`)
- Added an integration-style unit test that verifies dynamic command registration and execution

### Why
- Commands are the escape hatch for repo-specific workflows (db-reset, seed, etc.) without growing the core CLI surface
- Dynamic subcommands make the UX discoverable (`devctl --help` shows plugin commands)

### What worked
- `go test ./...` covers dynamic command registration and dispatch end-to-end
- Manual run works with a config pointing at the fixture plugin:
  - `cd devctl && go run ./cmd/devctl --config .devctl.command.yaml echo hello world`

### What didn't work
- The first attempt used a canceled context for plugin startup during command discovery, which immediately killed the plugin process; fixed by letting the factory manage handshake timeouts and only applying timeouts around the `commands.list` call

### What I learned
- “Build command tree dynamically” requires parsing only the minimal argv flags up front and ignoring unknown flags safely

### What was tricky to build
- Avoiding stdout/stderr contamination while still allowing plugin commands to emit human output (right now the runtime logs plugin stderr; later we may want passthrough modes for commands)

### What warrants a second pair of eyes
- Collision policy: currently “first wins” with a warning; decide if `--strict` should error on collisions for command names
- Output policy: for command UX, consider whether plugin stderr should be passed through verbatim instead of being logged

### What should be done in the future
- Add structured arg specs to dynamically create cobra flags from `args_spec`
- Decide whether plugin commands should run with the full multi-plugin config (mutate from all plugins) vs only the provider plugin

### Code review instructions
- Dynamic registration: `/home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/cmd/devctl/cmds/dynamic_commands.go`
- Fixture plugin: `/home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/testdata/plugins/command/plugin.py`
- Test: `/home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/cmd/devctl/cmds/dynamic_commands_test.go`

## Step 15: Publish a plugin authoring guide and copy/paste examples

This step documented the protocol constraints and added minimal example plugins so teammates can get started quickly without reading Go code. The goal is pragmatic onboarding: “copy this file, edit a few fields, and you have a working plugin.”

**Commit (code):** dc9443705e58d905bd38217d5a812eb6eeb8de8d — "docs: add plugin authoring guide and examples"

### What I did
- Added a short plugin authoring guide that describes framing rules, handshake, request/response, and common ops
- Added copy/paste example plugins:
  - python: config.mutate + launch.plan + commands.list/command.run
  - bash: minimal config.mutate example (jq-based)

### Why
- The “script-first” value proposition only works if writing a plugin is genuinely easy and well-documented
- Examples reduce the chance of stdout contamination and other protocol mistakes

### What worked
- The examples are self-contained and can be referenced directly from repo config

### What didn't work
- N/A

### What I learned
- Keeping examples minimal (single file, tiny dependencies) is key; anything “frameworky” will deter adoption

### What was tricky to build
- Picking example ops that are representative but don’t force us to finalize every detail of the engine/supervisor UX

### What warrants a second pair of eyes
- Whether we want to standardize on Python as the “default” plugin language for fixtures and examples
- Whether the bash example should be included given it depends on `jq`

### What should be done in the future
- Add an “FAQ/troubleshooting” section based on real plugin author onboarding issues

### Code review instructions
- Guide: `/home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/docs/plugin-authoring.md`
- Examples:
  - `/home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/examples/plugins/python-minimal/plugin.py`
  - `/home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/examples/plugins/bash-minimal/plugin.sh`

## Step 16: Add a Moments plugin set and config that maps `startdev.sh` into phases

This step created a first Moments plugin and config file so the generic runner can produce a sensible plan for bringing up the Moments backend and web dev server. The focus is on phase alignment (mutate/validate/build/prepare/launch) rather than perfect parity with every `startdev.sh` nuance.

The runner was also adjusted so `devctl up` executes `build.run` and `prepare.run` (when provided by plugins) before validation and launch, matching the intended pipeline ordering.

**Commit (code):** ae137d4cd0de1ace75836d1faf7844099374f2ed — "up: run build/prepare phases (optional)"

### What I did
- Added a Moments plugin: `/home/manuel/workspaces/2026-01-06/moments-dev-tool/moments/plugins/moments-plugin.py`
  - `config.mutate` sets ports and Vite env defaults
  - `validate.run` checks tool availability (`go`, `make`, `pnpm`, `python3`)
  - `build.run` and `prepare.run` implement basic steps (backend build, web deps/version)
  - `launch.plan` returns two services (`backend`, `web`) with tcp health checks
- Added a repo config for Moments: `/home/manuel/workspaces/2026-01-06/moments-dev-tool/moments/.devctl.yaml`
- Verified the plan output:
  - `cd devctl && go run ./cmd/devctl --repo-root ../moments --config ../moments/.devctl.yaml plan`

### Why
- To prove the “generic runner + script plugin protocol” can model a real repo’s dev lifecycle with phases instead of one giant script

### What worked
- `devctl plan` against the Moments repo produces a merged plan with backend + web services and env wiring

### What didn't work
- N/A

### What I learned
- Auto-discovery naming (`plugins/devctl-*`) can unintentionally double-register plugins if you also list them in `.devctl.yaml`; avoid the `devctl-` prefix for config-listed scripts

### What was tricky to build
- Choosing safe defaults for Vite envs without depending on the existing `moments-config get` mismatch noted earlier in the ticket

### What warrants a second pair of eyes
- Whether the chosen default URLs/ports match the expected Moments dev topology (notably the historical `8082` vs `8083` ambiguity in `startdev.sh`)
- Whether build/prepare step naming should be standardized across repos (`backend`, `web.deps`, etc.)

### What should be done in the future
- Replace “basic steps” with more faithful parity (pnpm install behavior, version generation, backend bootstrap ordering) once we validate expected workflows

### Code review instructions
- Moments plugin: `/home/manuel/workspaces/2026-01-06/moments-dev-tool/moments/plugins/moments-plugin.py`
- Moments devctl config: `/home/manuel/workspaces/2026-01-06/moments-dev-tool/moments/.devctl.yaml`
- Up pipeline wiring: `/home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl/cmd/devctl/cmds/up.go`

## Step 17: Create a test roadmap task set (fixtures + test apps + smoketests)

This step translated the testing strategy design doc into a concrete implementation task list. The goal is to make it easy to execute the testing work incrementally (new fixture plugins → new Go test apps → new smoketest commands → CI wiring) without losing coverage of important operational failure modes.

**Commit (code):** N/A — tasks only

### What I did
- Added a new set of ticket tasks for building out fixtures, Go test apps, and smoketests for devctl

### Why
- The test plan includes multiple feature areas (long-running tools, validation, launch failures, logs) and needs an explicit roadmap to avoid gaps and thrash

### What worked
- Tasks were added cleanly and show up at the end of `docmgr task list --ticket MO-005-IMPROVE-STARTDEV`

### What didn't work
- N/A

### What I learned
- Keeping the testing tasks grouped by “fixtures / testapps / smoketests / CI” makes it much easier to parallelize the work later

### What was tricky to build
- Ensuring tasks reflect the full feature surface (streams, cancellation, readiness timeouts, post-ready crashes, log follow) without turning into a single unbounded “write more tests” item

### What warrants a second pair of eyes
- Whether we should bias more of the test surface toward pure `go test` (vs CLI smoketest commands) for CI speed and determinism

### What should be done in the future
- Implement the new testing tasks in a dedicated follow-up session (starting with fixture plugins + a dev smoketest e2e)

### Code review instructions
- Review the testing strategy: `/home/manuel/workspaces/2026-01-06/moments-dev-tool/moments/ttmp/2026/01/06/MO-005-IMPROVE-STARTDEV--improve-startdev-sh-script-analysis-and-enhancement/design-doc/06-devctl-testing-strategy-smoke-tests-fixtures-and-end-to-end-runs.md`
- Review the task list: `/home/manuel/workspaces/2026-01-06/moments-dev-tool/moments/ttmp/2026/01/06/MO-005-IMPROVE-STARTDEV--improve-startdev-sh-script-analysis-and-enhancement/tasks.md`

### Technical details
- Commands run:
  - `docmgr task add --ticket MO-005-IMPROVE-STARTDEV --text \"Testing: ...\"`
  - `docmgr task list --ticket MO-005-IMPROVE-STARTDEV`
