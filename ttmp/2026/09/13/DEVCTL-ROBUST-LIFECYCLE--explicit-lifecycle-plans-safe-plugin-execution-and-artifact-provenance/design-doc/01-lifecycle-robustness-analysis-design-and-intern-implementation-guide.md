---
Title: Lifecycle robustness analysis design and intern implementation guide
Ticket: DEVCTL-ROBUST-LIFECYCLE
Status: active
Topics:
    - devctl
    - architecture
    - plugins
    - supervisor
DocType: design-doc
Intent: long-term
Owners: []
RelatedFiles:
    - Path: repo://pkg/engine/types.go
      Note: Current path-only artifact model
    - Path: repo://pkg/operator/controller.go
      Note: Restart pre-stop preparation and locked application
    - Path: repo://pkg/operator/planner.go
      Note: Current side-effecting planner and phase order
    - Path: repo://pkg/plugincatalog/catalog.go
      Note: Discovery persistence and fingerprints
    - Path: repo://pkg/runstate/schema.go
      Note: Run health exit and future artifact provenance
    - Path: repo://pkg/runtime/client.go
      Note: Plugin Close and request lifetime
    - Path: repo://pkg/runtime/factory.go
      Note: Process-group termination and Wait ownership
ExternalSources: []
Summary: Source-grounded guide to explicit execution plans, graceful plugin shutdown, supervised build steps, artifact identity, historical health, and command catalog discovery.
LastUpdated: 2026-09-13T11:00:00Z
WhatFor: ""
WhenToUse: ""
---

# Lifecycle robustness: analysis, design, and intern implementation guide

## 1. Purpose and delivery boundary

Devctl starts development environments from repository-specific plugins. A plugin supplies configuration, build and preparation behavior, validation, and service definitions. Devctl supervises the resulting services and preserves run state and logs. This division works well when process ownership and lifecycle effects are explicit. It becomes harder to use when a method called `Plan` performs builds, a command catalog is invisible until invocation fails, or a health column looks current after its process exited.

This ticket proposes improvements to those boundaries. It is an implementation guide for a new engineer, not an assertion that the proposed APIs already exist. Source observations refer to repository revision `418a4ca1f18d3059c1b1cf8f846db371163529b5`. The investigation changed documentation only. Implementation tasks remain open.

The proposal covers six connected changes:

1. Separate execution planning from side-effecting phase execution.
2. Give plugin shutdown a graceful EOF interval and one authoritative wait result.
3. Supply a supported bounded subprocess runner, with declarative build steps as a separately approved extension.
4. Associate typed artifact identities with launched service attempts.
5. Distinguish current process state from historical health observations.
6. Make command-catalog state and provenance discoverable without silently executing plugins.

Supporting work includes executable documentation examples, cleanup-safe smoke tests, and a compact raw/schema help surface. These address the authoring workflow rather than introducing a second orchestration engine.

## 2. Vocabulary and the first complete example

A **plugin** is a child process that exchanges newline-delimited JSON with devctl. A **phase** is an operation such as configuration mutation or building. A **service** is a long-running process described by a plugin. A **run** is one attempt to execute a service; restarting creates a new attempt. An **operation** is a user lifecycle request, such as restarting selected services, and can include outcomes from several runs.

An **artifact** is an output used later, such as an executable. A path locates an artifact; a content digest identifies its bytes. A **catalog** is stored metadata that lets the CLI discover plugin-defined commands without launching every plugin on each invocation.

Consider a Go server with a browser UI. Its plugin returns an API URL from `config.mutate`, builds the executable in `build.run`, checks prerequisites in `validate.run`, and returns the executable argv in `launch.plan`. Devctl then starts a wrapper that owns the actual service process, checks readiness, and retains logs. On restart, the old and new attempts must remain distinguishable.

```text
operator: restart api
        |
        v
CLI parses repository, profile, phase policy
        |
        v
operator controller resolves selected services
        |
        v
pipeline invokes configured plugins
        |
        v
supervisor replaces owned service attempt
        |
        v
run-state + logs retain operation and exit evidence
```

The word “resolves” in that diagram hides an important current behavior: the existing operator planner also executes build and prepare. Understanding that behavior is the first task in this guide.

## 3. Source map: what to read and why

Read in this order rather than browsing the entire repository:

- `cmd/devctl/cmds/lifecycle.go`: CLI fields, policy conversion, lifecycle commands, and health formatting.
- `pkg/operator/requests.go`: `PipelinePolicy`, `UpRequest`, `RestartRequest`, selections, and snapshot requests.
- `pkg/operator/controller.go`: public controller interface, restart ordering, repository locks, and snapshots.
- `pkg/operator/planner.go`: current `Planner` interface and `PipelinePlanner.Plan` implementation.
- `pkg/engine/types.go` and `pipeline.go`: plugin operation results, phase dispatch, and merge rules.
- `pkg/protocol/types.go` and `validate.go`: NDJSON frame and capability contracts.
- `pkg/runtime/client.go`, `factory.go`, and `runtime_test.go`: plugin process startup, request routing, and shutdown.
- `pkg/supervise/supervisor.go`: service attempt startup, readiness, and process supervision; do not confuse this with plugin runtime shutdown.
- `pkg/runstate/schema.go`: durable service, process, health, and exit records.
- `pkg/plugincatalog/catalog.go`: fingerprinting, static catalogs, explicit discovery, atomic persistence, and validation.
- `cmd/devctl/cmds/dynamic_commands.go`: command registration from stored/static catalogs and recovery errors.
- `pkg/doc/topics/devctl-user-guide.md`: operator-facing claims that must match executable behavior.

Primary regression anchors are `pkg/operator/controller_test.go`, especially `TestRestartPlansBeforeStopAndUsesOneOperation` and `TestRestartPlanningFailureDoesNotStopService`. Retain their essential property: failing to prepare a replacement must not unnecessarily stop the current service.

## 4. Current architecture and evidence

### 4.1 The NDJSON boundary

A plugin emits a handshake first, declaring protocol version and capabilities. Devctl sends a request with an ID, operation name, context, and input. The plugin returns one response carrying the same ID. Events may be routed separately for streams. Stdout is protocol-only; stderr is for human-readable logs.

A shortened current request shape is:

```json
{
  "type": "request",
  "request_id": "build-1",
  "op": "build.run",
  "ctx": {
    "repo_root": "/work/example",
    "deadline_ms": 30000,
    "dry_run": false
  },
  "input": {"config": {}, "steps": ["backend"]}
}
```

`runtime.Client.Call` registers a response route, serializes input, writes a frame, and waits for a correlated response or context cancellation. Cancelling that wait does not, by itself, prove a plugin's subprocess has stopped. This distinction matters for the subprocess proposal later.

### 4.2 Restart executes phases before stopping

`PipelinePlanner.Plan` currently performs:

```text
load repository and selected plugin specifications
start plugin clients
config.mutate
build.run       unless SkipBuild
prepare.run     unless SkipPrepare
validate.run    unless SkipValidate
launch.plan
close plugin clients
```

Each phase receives a timeout context derived from the operation context. The default phase timeout is 30 seconds. This is not one new global wall-clock budget for the entire pipeline.

`controller.Restart` converts its request to an `UpRequest` and calls the planner before taking the repository lifecycle lock and stopping services. It checks service selection, then performs down/up under the lock. It refuses to proceed after an unproven service termination. These are useful safety properties.

However, builds can mutate files before the lock is acquired. Two concurrent lifecycle operations can therefore contend over mutable build outputs even though process replacement itself is serialized. Separating the semantic stages must preserve pre-stop preparation while addressing artifact publication conflicts.

### 4.3 Plugin shutdown immediately signals after EOF

`runtime.client.close` sets the closing flag, closes stdin, invokes `terminateProcessGroup`, and returns nil. The helper immediately sends SIGTERM, calls `cmd.Wait` in a goroutine, and escalates to SIGKILL after its timeout. The caller discards that helper's error, and its `ctx` is not used to select the shutdown budget.

There is no deliberate interval for a plugin that exits normally on stdin EOF. A Python plugin's normal interpreter shutdown can race SIGTERM handling. The required fix is a defined shutdown sequence, not a special case for one Python warning.

### 4.4 Artifacts are paths, not launch identities

`engine.BuildResult` and `PrepareResult` contain `Artifacts map[string]string`. `PipelinePlanner.Plan` discards the build and prepare return values. `PlanResult` contains only launch plan and profile name.

`RunRecord` retains run ID, phase, command specification, process identities, health, and exit records. It does not currently establish the content identity of an executable chosen from a mutable path. Printing a hash from a helper command describes the file at the time of inspection, not necessarily the executable of an existing process.

### 4.5 Health is an observation already carrying time

`runstate.HealthResult` contains `Healthy`, `CheckedAt`, `DurationMs`, and `Detail`. `controller.Snapshot` copies it alongside run phase and exit evidence. The data model can retain a successful readiness observation even after the run exits. Displaying that as unqualified “healthy” creates avoidable ambiguity; deleting it would instead destroy useful history.

### 4.6 Command discovery has a deliberate trust boundary

The dynamic CLI first loads the stored catalog, with a static configuration fallback. Explicit `plugincatalog.Refresh` either uses static command specifications or starts a plugin to read its handshake, validates conflicts, and atomically writes the catalog. This design avoids automatically executing arbitrary plugin processes just to render ordinary help.

The fingerprint includes profile, plugin specifications, configured commands, and executable identity metadata. For an interpreter invocation such as `python3 plugin.py`, executable metadata describes Python. The argv string identifies `plugin.py` by name, but the inspected fingerprint function does not hash that script's contents. A script edit may therefore need explicit refresh even if the interpreter identity is unchanged.

## 5. Design principles and non-goals

Preserve the existing architecture's ownership boundaries. Plugins know repository-specific facts; devctl owns lifecycle supervision. A change should strengthen that division rather than make the plugin recursively invoke lifecycle commands.

The implementation must preserve these invariants:

- Inspection does not silently execute side-effecting phases.
- Failure to build or validate a replacement does not stop a healthy current run.
- One owner waits for and reaps each child process.
- Timeout is not success, and incomplete process termination is observable.
- A historical health result is not presented as current process liveness.
- Help/catalog inspection does not silently run untrusted plugins.
- Artifact identity is attached to the attempt that actually selected it.
- Cleanup touches only resources whose ownership has been established.

Non-goals include distributed orchestration, container-level security isolation, proving arbitrary plugins are side-effect-free, and treating devctl as an emergency-stop mechanism for hardware. No automatic compatibility adapter or protocol dual-mode implementation is authorized by this design. Before changing public/protocol schemas, decide migration scope explicitly in accordance with the repository's no-implicit-compatibility convention.

## 6. Proposal A: explicit lifecycle plans

### 6.1 Separate a recipe from resolved launch facts

The simplest API split is useful but incomplete:

```go
ResolvePlan(ctx, request) (ExecutionPlan, error)
ExecutePlan(ctx, plan) (OperationResult, error)
```

A launch command can depend on a build artifact that does not exist yet. Therefore `ResolvePlan` should produce a recipe describing phases and dependencies, not pretend to know every final launch field. Separate recipe resolution, replacement preparation, and process application:

```go
// Proposed API; not present in the current source.
type LifecycleRecipe struct {
    ID string
    RepositoryFingerprint string
    Profile string
    Operation string
    Selection Selection
    Phases []PhaseSpec
}

type PreparedLaunch struct {
    RecipeID string
    Services []engine.ServiceSpec
    Artifacts []ArtifactRecord
}

ResolveRecipe(ctx, request) (LifecycleRecipe, error)
PrepareReplacement(ctx, recipe) (PreparedLaunch, error)
ApplyReplacement(ctx, prepared) (OperationResult, error)
```

`ResolveRecipe` can load configuration and enumerate known phase/step intent without running builds. If it needs plugin-derived facts, the preview must state whether plugin processes will run and which operations are permitted. “No lifecycle effects” is not a security sandbox for arbitrary plugin code.

### 6.2 Expose effects in the operator interface

A proposed preview should distinguish known information from unresolved information:

```text
restart api
  config:   resolve selected plugins
  build:    backend
  prepare:  skipped
  validate: enabled
  launch:   resolved after build
  replace:  api only, after successful preparation
  cleanup:  stop owned old attempt; retain run logs
```

Possible CLI surface, pending naming review:

```text
 devctl restart api --explain
 devctl restart api --dry-run
```

Do not make `--explain` perform a build merely to produce a more detailed preview. Return a schema version, phase list, skip reasons, selection, and unresolved fields in structured output. Avoid persisting raw secret environments in a recipe.

### 6.3 Ordering and concurrency

```text
resolve recipe
      |
prepare replacement artifacts in unique workspace
      | failure -> leave existing run untouched
validate replacement
      |
acquire repository lifecycle lock
      |
recheck config/selection/artifact identity
      | stale -> reject or explicitly replan, never silently apply
stop old owned attempt
      | unproven exit -> do not start replacement
start new attempt and observe readiness
      |
persist outcomes and release lock
```

A separate build lock or immutable per-build directories prevent concurrent preparation from overwriting the same executable. Do not hold the lifecycle lock across a long build merely as an accidental workaround without measuring the effect on stop availability. A manual stop should not have to wait behind compilation.

Tests must exercise selection errors, build errors, stale plans, concurrent builds, and cancellation before/after process replacement. Extend the existing restart tests rather than replacing their stronger pre-stop guarantees with a generic happy path.

## 7. Proposal B: graceful and observable plugin shutdown

### 7.1 One process wait owner

Create one completion channel for every started plugin process. Exactly one goroutine calls `cmd.Wait`; Close, request routing, and shutdown escalation observe the same result. This prevents competing shutdown calls from each trying to reap the process.

```go
// Proposed internal structure.
type processLifetime struct {
    done chan struct{}
    result ExitResult
    beginClose sync.Once
}

type ShutdownResult struct {
    ExitMode string // eof, term, kill, unconfirmed
    ExitCode *int
    Error error
}
```

Register the wait owner after successful `Start`, including handshake-failure paths. Audit every current call to `terminateProcessGroup` when moving ownership; startup failure cannot create a second Wait caller.

### 7.2 Shutdown sequence

```text
reject new requests; mark closing
close stdin once
wait for EOF grace or observed process exit
if still running: signal owned process group with SIGTERM
wait for TERM grace or exit
if still running: SIGKILL owned group
wait/reap under bounded cleanup policy
publish shutdown result; close pending routes
```

Use monotonic remaining budgets. `Close(ctx)` must define what happens when the caller is already cancelled: return promptly while a single internal cleanup owner continues with its bounded cleanup budget, or perform bounded synchronous cleanup and explain the semantics. Do not let cancellation abandon a known live process with no owner.

Separate expected signal termination from supervisor failures. Nonzero exit from a plugin already failing a request should not overwrite the original operation error; preserve primary and cleanup errors together. Repeated Close returns or observes the same terminal result and never signals a reused PID through stale ownership information.

### 7.3 Fixture matrix

Use test plugins that: exit on EOF; ignore EOF but exit on TERM; ignore TERM and require KILL; emit a last stderr line during exit; retain a child process; exit before Close; fail handshake; and receive concurrent Close calls. Assertions should check escalation count, no premature TERM, final reaping, route closure, and retained diagnostics.

Do not rely solely on sleeps. Fixtures should report readiness through pipes/files under the test directory, and tests should use bounded waits for explicit states. Test process identity and ownership before signaling.

## 8. Proposal C: supported subprocess execution

### 8.1 Recommended first implementation: one supported runner

Start with a Python helper because the concrete authoring workflow already uses Python plugins. Put the helper in an explicitly maintained SDK/example location with fixture tests; decide packaging before telling users to import a module that is not distributed. Keep service supervision in devctl.

```python
# Proposed interface, not an installed module.
result = runner.run(
    argv=["go", "build", "-o", output, "./cmd/server"],
    cwd=repo / "backend",
    deadline=request.deadline,
    output="stderr",
)
```

The runner owns argv execution without implicit shell parsing, monotonic remaining time, streaming output, bounded diagnostic tails, process groups, timeout escalation, cancellation, and reaping. It returns exit code, cancellation reason, duration, and bounded output metadata. It does not create independent full timeouts for every step in one request.

The shutdown contract must specify whether descendants remain in the plugin group or move to a separate group. A child using a separate group will not necessarily die when devctl terminates only the plugin group. The helper must own that group explicitly. A generic process-group mechanism cannot guarantee containment of a malicious child that deliberately detaches; stronger isolation is a separate feature.

### 8.2 Alternative: declarative build steps

A declarative protocol could let devctl execute temporary steps directly:

```go
// Proposed optional protocol extension, not a v2 method to call today.
type ExecStep struct {
    Name string
    Argv []string
    Cwd string
    DependsOn []string
    Timeout time.Duration
    Outputs []ArtifactDeclaration
}
```

This centralizes supervision but changes the protocol and dynamic preparation model. It requires dependency resolution, secret handling, cancellation, log attribution, output validation, and clear ownership if a plugin also executes steps. Do not implement both SDK execution and declarative execution as interchangeable hidden paths. Ship the supported runner first; use actual adoption evidence to decide whether a protocol extension earns the additional complexity.

### 8.3 Test and publishing contract

Test high-volume stdout/stderr, a quiet hung child, a child spawning descendants, cancellation between process creation and registration, nonexistent executables, nonzero exit, exhausted budget, and dry-run with zero writes. Ensure protocol stdout remains parseable throughout. Publish runnable fixture commands and a versioned support statement with the helper.

## 9. Proposal D: artifacts as run evidence

### 9.1 Preserve results through the pipeline

Begin by retaining BuildResult and PrepareResult in the prepared operation result; the current planner discards them. Then introduce typed artifact records after an explicit schema decision:

```go
// Proposed API.
type ArtifactRecord struct {
    ID string
    Path string
    SHA256 string
    SizeBytes int64
    ProducerStep string
    BuildID string
}

type ExecutableRef struct {
    ArtifactID string
    Args []string
}
```

A service should either identify an ordinary external executable or reference a produced artifact. Do not allow ambiguous simultaneous command and artifact declarations without a documented precedence rule.

### 9.2 Publish immutably and validate at launch

```text
build into operation-owned staging directory
validate declared outputs
compute digest and metadata
publish into immutable build-specific location
resolve service artifact reference
check bytes immediately before launch
record selected artifact in new RunRecord
```

Hashing a mutable path and then launching it still leaves a time-of-check/time-of-use race. Immutable, non-overwritten publication narrows that race and provides durable evidence. For stronger identity, platform-specific execution from an already-open descriptor can be evaluated separately. Do not claim a path digest universally proves all code loaded by an interpreter, dynamic library loader, or container.

The first scope should be directly launched native binaries. Interpreter scripts, image digests, assets, and dynamic libraries require explicit artifact sets or a different provenance model. Retain build and run IDs so an operator can answer which build was selected for a specific attempt, not merely which file exists now.

### 9.3 Persistence and garbage collection

`RunRecord` gains additive provenance only after deciding its schema version and reader behavior. No automatic compatibility layer is authorized here. Retain artifacts referenced by active runs and retained evidence policies. Garbage collection must never delete the executable or dependencies of a live attempt merely because a newer build exists.

Tests should corrupt an artifact, change a path between preview and apply, build two revisions concurrently, restart with skip-build, and inspect an old run after the current path changes. Secret values must never enter artifact metadata, command previews, or run manifests.

## 10. Proposal E: health observation presentation

The stored model already distinguishes phase, process identity, exit, and timed health evidence. Introduce a pure presentation projection rather than deleting historical observations:

```go
// Proposed view model.
type HealthView struct {
    Current string // healthy, unhealthy, unknown, not_running
    Last *runstate.HealthResult
}

func ProjectHealth(phase RunPhase, last *HealthResult) HealthView
```

For an exited run, `Current` is `not_running` even if `Last.Healthy` is true. CLI output should label the old observation as last health and expose its age. TUI and structured output must use the same projection rather than independently inventing meanings.

Readiness and health are not universally equivalent. A one-time successful startup probe is not continuous monitoring. Document whether subsequent health checks occur; avoid presenting a historical startup probe as an ongoing guarantee.

Tests cover ready/healthy, ready/unhealthy, starting/unknown, exited with formerly healthy, failed startup, and absent timestamp. Retain the timestamp and detail for diagnostics. This feature is a good small first intern task because it can be implemented as a pure function with table-driven tests.

## 11. Proposal F: catalog discoverability and safe refresh

### 11.1 Surface current metadata

Add a catalog inspection view that reports: repository/profile, missing/stale/valid/conflicted state, generation time, fingerprint status, command source, and the next action. A possible CLI is `devctl plugins catalog`, but choose final naming alongside existing plugins subcommands.

```text
provider   commands   source      catalog state   action
api        2          static      valid           none
tools      unknown    handshake   missing         refresh
```

`plugins list` can include a compact catalog-status column. Ordinary help should not auto-start plugins when the catalog is missing. Explicit refresh may execute plugin processes and should say which providers require it; static declarations can be inspected without starting those processes.

### 11.2 Fingerprint declared source inputs

For interpreted plugins, introduce explicit catalog inputs such as script files or a provider-supplied version declaration. Do not infer that every argv token is a path. Hash declared nonsecret inputs in deterministic order and reject unreadable inputs with actionable errors. Document that explicit refresh is still needed when undeclared dependencies change.

Config/profile changes, executable changes, declared script changes, reserved-name conflicts, static/handshake differences, and provider removal must have separate tests. The catalog cache currently uses one repo cache path; assess whether profiles should have independent entries rather than repeatedly replacing a single profile's catalog. This is a usability/design decision, not permission to merge commands from incompatible profiles.

### 11.3 Registration and execution references

Keep `plugincatalog.Static`, `Refresh`, `Load`, and `Validate` as recognizable responsibilities. The CLI should continue returning actionable refresh instructions, but include the correct repository and profile context. Distinguish an unavailable catalog from an unknown command within a valid catalog.

Catalog freshness is not a substitute for validating the actual plugin operation. Test end-to-end invocation after refresh, including argv forwarding, command conflicts, a provider no longer advertising `command.run`, and dry-run behavior. Do not hide mutating command execution inside discovery.

## 12. Documentation and authoring ergonomics

### 12.1 Make examples executable

The published lifecycle guide and CLI implementation must share a tested phase matrix. Tests should construct the actual Cobra commands and verify documented examples accept their flags. For semantic examples, use fixture plugins whose operations append bounded events to a test-owned journal, then assert the expected phase order.

Known motivating examples are status/log-tail syntax, restart phase behavior, and command-catalog refresh. The durable solution is not a growing list of removed flags; it is a set of maintained examples tested against the command surface.

### 12.2 Raw and schema help

Rendered terminal help can be heavily wrapped and padded when captured by tools. First inspect the inherited Glazed help options to determine whether raw output already exists and merely needs documentation. If absent, propose explicit raw Markdown and JSON schema modes rather than duplicating documentation in another manually maintained source.

A compact plugin reference should list handshake, request context, phase input/output, error shape, and lifecycle ownership. Long tutorials can then teach examples without repeating every schema. Generate field names and required flags from actual types/command metadata where practical; prose about trust and cancellation still needs editorial review.

### 12.3 Cleanup-safe smoke fixtures

A smoke test must own its environment before registering cleanup. Use a fresh temporary repository/config and nonconflicting ports, register cleanup before startup, and ensure assertions do not stop pre-existing services. A failed check after up must still run teardown. Record final process/run state and exit evidence; a successful health field alone does not prove cleanup.

Hardware-backed fixtures need a separate operator procedure. Devctl down or restart is not proof that external machinery, remote jobs, or persistent actuators have stopped. Keep those out of routine CI.

## 13. Implementation phases and intern work packages

### Phase 0: baseline and contract decisions

Read the source map, run targeted tests, and capture the existing phase matrix. Decide naming for recipe/prepared plan, the timeout scope, public schema changes, and artifact migration. Do not create compatibility shims without approval. Produce a short decision record for each unresolved API choice.

### Phase 1: low-risk projections and documentation

Implement shared health presentation and catalog-status inspection. Add raw-help discoverability or document an existing mode. Add example parsing and phase-order tests. These changes provide immediate usability improvements without changing process termination.

### Phase 2: plugin process lifetime

Introduce a single Wait owner, idempotent Close, EOF/TERM/KILL sequencing, and observable cleanup errors. Update handshake-failure paths and request/stream cleanup. Use fixture-driven race tests before touching the service supervisor; plugin runtime and service wrapper are separate owners.

### Phase 3: supported runner

Publish the bounded runner with cancellation/descendant fixtures and dry-run semantics. Update Python examples to use it. Measure whether plugin authors can implement a build without copying custom signal code. Defer declarative execution unless separately approved.

### Phase 4: lifecycle recipe separation

Refactor `PipelinePlanner.Plan` into explicit resolve/prepare/apply responsibilities. Preserve build-before-stop and unproven-termination refusal. Add explain output and stale-plan checks. Review build locking and stop responsiveness together.

### Phase 5: artifact provenance

Retain producer outputs, add typed references, implement immutable publication and run linkage, and test corruption/replacement scenarios. Document native-binary scope before expanding to script/container artifacts.

### Phase 6: integration qualification

Run the complete offline lifecycle matrix with dynamic commands, profile changes, failed builds, cancelled plugins, and exact-artifact restart. Validate CLI and TUI agreement. Regenerate documentation and run smoke examples. This closes implementation only after evidence covers every feature, not merely because unit tests compile.

## 14. Validation commands and expected evidence

Start narrow during investigation:

```sh
go test ./pkg/operator -run 'TestRestart'
go test ./pkg/runtime ./pkg/plugincatalog
```

At integration boundaries, follow repository commands:

```sh
go test -race ./pkg/operator ./pkg/runtime ./pkg/plugincatalog
go test ./...
go vet ./...
go build ./...
```

These are recommended future implementation checks, not test results claimed by this documentation task. CLI fixture smoke should run through `go run ./cmd/devctl ...` as the repository guide requests. Launch any manually operated long-running test service in an owned tmux session; do not kill unrelated listeners to make a test pass.

Evidence should include code revision, fixture configuration, exact command, outcome, operation/run IDs, and relevant diagnostics. Retain raw output only where byte identity or protocol framing matters. Keep future ad hoc scripts under this ticket's numbered `scripts/` directory.

Acceptance criteria:

- Preview never runs build/prepare effects and accurately labels unresolved/plugin-derived facts.
- Restart preparation failures preserve the existing run.
- Shutdown fixtures prove one waiter and correct escalation, with no orphaned owned children.
- Runner tests prove deadline sharing, output routing, dry-run no-write, and descendant cleanup.
- Artifact/run association survives concurrent builds and mutable-path changes within the declared scope.
- CLI/TUI distinguish exited state from historical health.
- Catalog inspection does not silently execute plugins; refresh/context errors are actionable.
- Documented commands and phase expectations are tested against the executable interface.

## 15. Risks and decisions requiring review

**API growth:** adding every proposed type at once could create more concepts than it removes. Implement the low-risk views and runner first, then introduce recipe/artifact types around demonstrated boundaries.

**Plan purity:** no process-level API can prove arbitrary plugin code has no side effects. Promise only the phases devctl itself invokes, and make execution of plugin discovery explicit.

**Signal behavior:** process-group semantics are platform-dependent; fixture expectations must specify supported operating systems. A child intentionally detaching is outside ordinary process-group ownership guarantees.

**Timeout meaning:** current phase budgets are not a single operation budget. Decide whether to retain per-phase policy plus an overall deadline, and expose both rather than silently changing timeout meaning.

**Persistence:** changing RunRecord and catalog schemas needs an explicit version/migration decision. Do not quietly reinterpret old state or add adapters contrary to repository policy.

**Secrets:** previews, logs, manifests, and fingerprints must not expose raw secret environments or file contents. A digest is not automatically harmless for low-entropy secret material; exclude secrets from catalog inputs.

**Artifact retention:** immutable outputs require garbage collection. Retention must account for active and historical run references before deleting builds.

**Shutdown observability:** returning a cleanup error may uncover failures previously hidden by Close always returning nil. Update callers to preserve both primary and cleanup outcomes rather than merely ignoring the new error.

## 16. Suggested first day for the intern

Run one fixture plugin and trace its handshake through `runtime.Client.Call`. Follow restart from CLI policy through `PipelinePlanner.Plan` and identify exactly where the first side effect occurs. Read a completed RunRecord and distinguish phase, health timestamp, and exit code. Inspect the command catalog for a static and handshake-discovered provider.

Then implement one small table-driven health projection in a separate implementation branch. This introduces the repository's types and tests without changing process ownership. Before implementing shutdown, review the proposed Wait-owner lifecycle with a maintainer. Before implementing artifact schema changes, obtain the migration decision.

The goal is not to memorize the package tree. It is to be able to answer, for every effect, who owns it, which identity it belongs to, how cancellation is handled, and what evidence survives failure.
