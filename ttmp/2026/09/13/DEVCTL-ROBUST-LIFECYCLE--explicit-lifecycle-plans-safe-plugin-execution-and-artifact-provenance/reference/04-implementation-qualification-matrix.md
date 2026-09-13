---
Title: Implementation Qualification Matrix
Ticket: DEVCTL-ROBUST-LIFECYCLE
Status: active
Topics:
    - devctl
    - workflow
    - architecture
    - supervisor
DocType: reference
Intent: long-term
Owners: []
RelatedFiles:
    - Path: repo://cmd/devctl/cmds/cli_contract_test.go
      Note: Built executable contract matrix
    - Path: repo://pkg/operator/artifacts_test.go
      Note: Artifact identity concurrency and collection matrix
    - Path: repo://pkg/operator/planner.go
      Note: Recipe and preparation acceptance evidence
    - Path: repo://pkg/runtime/shutdown_test.go
      Note: Plugin shutdown escalation matrix
ExternalSources: []
Summary: Requirement-to-test and revision evidence for the robust lifecycle implementation.
LastUpdated: 2026-09-13T12:28:00Z
WhatFor: Auditing implementation completeness before closing DEVCTL-ROBUST-LIFECYCLE.
WhenToUse: During final review regression diagnosis and release qualification.
---

# Implementation Qualification Matrix

## Scope

This matrix maps each explicit ticket requirement and design acceptance boundary to source, tests, commits, and qualification commands. A passing broad suite is supporting evidence; the focused fixtures below establish individual behavioral claims.

## Requirement Evidence

| Requirement | Implementation | Focused evidence | Revision |
|---|---|---|---|
| Current health must not present stale historical health as live | `runstate.ProjectHealth`; structured and human status projection | `pkg/runstate/health_test.go`, status tests | `bf6eb6e` |
| Catalog inspection must not execute providers | `plugincatalog.Inspect`, `devctl plugins catalog` | inspection tests and built CLI marker fixture | `6ffd108` |
| Catalog entries must identify source and declared input fingerprints | catalog schema v2 provider provenance and `catalog_inputs` hashes | catalog fingerprint and inspection tests | `6ffd108` |
| Help/schema discovery must be machine-readable and phase order executable | `devctl schema`, raw help export, phase matrix | built CLI contract and `TestPipelinePlannerPhaseMatrix` | `bb61458` |
| Plugin shutdown must have one Wait owner and bounded EOF/TERM/KILL escalation | `runtime.processLifetime` | EOF, TERM, KILL, descendant, concurrent Close, cancellation, and race fixtures | `53ee866` |
| Supported plugin subprocess execution must share one monotonic request budget | `sdk/python/devctl_runner.py` | six Python fixtures covering budget, output, dry-run, cancellation, descendants, and missing executable | `f7abf04` |
| Devctl must use the workspace Glazed API without bypassing the workspace | canonical command builders and structured output `--format` | full workspace build/tests, CLI contract, no removed output symbols | `b2ac21a`; Glazed `e0cfa33` |
| Recipe resolution must be effect-free and label unresolved facts | `PipelinePlanner.ResolveRecipe`; lifecycle `--explain` | no-execution marker tests and five-phase explain rows | `f3eb1ad`, `a14d611` |
| Replacement preparation must precede stop | `PrepareReplacement` before lifecycle lock/down | planning/preparation failure restart fixtures | `f3eb1ad` |
| Apply must reject stale configuration before stop | fingerprint plus under-lock `ValidatePrepared` | stale-config planner test and stale-restart controller test | `f3eb1ad` |
| Timeout semantics must be explicit | per-phase `PipelinePolicy.Timeout`; caller context remains overall bound | phase matrix, cancellation tests, decision record | `f3eb1ad` |
| Build and prepare results must survive planning | `PreparedLaunch.Build` and `.Prepare` | planner retention fixture | `f3eb1ad` |
| A service may select one build-produced native executable | `engine.ExecutableRef`; artifact staging and publication | ambiguity/missing reference tests and built CLI artifact service | `af0d5bb` |
| Launched bytes must have durable typed identity | run schema v2 `ArtifactRecord` | CLI run JSON and structured status digest assertions | `af0d5bb` |
| Mutable or corrupt bytes must be rejected before launch | SHA-256/size validation in planner and supervisor | staged corruption, changed bytes, non-executable, and supervisor corruption tests | `af0d5bb` |
| Identical outputs must deduplicate and concurrent preparations must not share staging | digest path plus recipe-owned staging | sequential reuse and concurrent preparation race fixture | `a14d611` |
| Garbage collection must be simple and bounded | current/last digest mark set and removal of other valid digest directories | collector unit test and automatic post-up integration test | `af0d5bb` |
| Plugin authors and operators must discover artifact behavior in CLI help | embedded `artifact-provenance` help plus authoring/user/upgrade updates | unique-slug check, `devctl help artifact-provenance`, structured help export | `af0d5bb` |
| Documentation workflow and reusable context audit must remain reproducible | installed/project `session-context-audit` skill and archived HTML | bundled validation script and ticket report | `b9d859f`, `f72e74a`, shared skill `4e6e3d5` |

## Offline Failure Matrix

| Failure or boundary | Expected result | Fixture evidence |
|---|---|---|
| Unknown service selection | usage-class operator error; no state mutation | controller and built CLI selector tests |
| Build/preparation failure during restart | old owned run remains active | restart planning failure test |
| Configuration changes after preparation | `E_RECIPE_STALE`; no stop | planner stale fingerprint and controller stale apply tests |
| Artifact missing or ambiguous | preparation fails before stop | artifact reference table tests |
| Artifact changes in staging | `E_ARTIFACT_INVALID`; no apply | prepared artifact corruption test |
| Published artifact changes | wrapper is not launched; run becomes failed | supervisor corruption fixture |
| Plugin exits on EOF | no TERM | runtime shutdown fixture |
| Plugin ignores EOF | TERM escalation | runtime shutdown fixture |
| Plugin ignores TERM | KILL escalation and reap | runtime shutdown fixture |
| Plugin leaves descendant | cleanup waits for process-group disappearance | runtime descendant fixture |
| Close caller is cancelled | caller returns while bounded owner continues cleanup | runtime cancellation fixture |
| Dynamic help/completion | no provider process | built CLI marker fixture |
| Catalog is missing/stale/conflicted | non-executing diagnostic state with provenance | plugincatalog inspection fixtures |
| Followed machine logs/stream | one JSON object per line | logs/stream CLI tests using `--format jsonl` |
| Validation emits a row and returns an error | complete JSON is flushed before propagated error | devctl validation test plus Glazed `cobra_error_test.go` |
| Old run schema | explicit rejection, no fabricated provenance | `TestLoadRunRejectsPreviousSchemaWithoutImplicitMigration` |
| Unreferenced valid artifact digest | removed after locked update | collector and post-up integration tests |
| Current or immediately previous digest | retained | collector protection test |
| Malformed/symlink artifact-root entry | ignored without touching symlink target | collector malformed-directory and symlink fixture |

## Qualification Commands

The final qualification was run in the parent workspace without `GOWORK=off`:

```sh
go test ./...
go test -race ./pkg/operator ./pkg/runtime ./pkg/plugincatalog ./pkg/runstate ./pkg/supervise ./pkg/tui
golangci-lint run -v
go vet ./...
go build ./...
PYTHONDONTWRITEBYTECODE=1 python3 sdk/python/test_devctl_runner.py -v
PYTHONDONTWRITEBYTECODE=1 skills/session-context-audit/scripts/validate.sh
docmgr doctor --ticket DEVCTL-ROBUST-LIFECYCLE --stale-after 30
go run ./cmd/devctl help artifact-provenance
go run ./cmd/devctl help export --slug artifact-provenance --format json --output-fields slug,title
```

Results:

- all Go tests passed;
- race tests passed for lifecycle, runtime, catalog, run state, supervision, and TUI packages;
- lint reported zero issues;
- vet and build passed;
- six Python runner tests passed;
- five session-context report-model tests and fixture validation passed;
- docmgr doctor reported all checks passed;
- artifact help rendered and exported as one structured row;
- eight embedded help slugs were unique.

Two initial Python qualification invocations used unittest module-style paths that were incompatible with their local import layout. Running the scripts through their supported entry points passed. This was invocation friction, not product failure.

## Deliberate Boundaries

- Artifact provenance covers directly launched native executables only.
- Content-addressed executable bytes remain for current and immediately previous runs; older run records retain identity evidence but not guaranteed bytes.
- Run schema v1 is rejected after the v2 provenance change; no compatibility adapter is present.
- `--timeout` remains a per-phase budget. A separately named overall deadline can be added later if needed.
- The Python runner is supported standard-library code; no declarative execution protocol was added.
- Hardware and remote actuator shutdown remain outside routine lifecycle guarantees.

## Review Order

1. `pkg/operator/planner.go` and `controller.go` for recipe/preparation/apply ordering.
2. `pkg/operator/artifacts.go`, `pkg/runstate/artifact.go`, and `pkg/supervise/supervisor.go` for executable identity.
3. `pkg/runtime/process_lifetime.go` for shutdown ownership.
4. `pkg/plugincatalog` for inspection and provenance.
5. Built CLI contracts and focused package fixtures.
6. Embedded help entries and the timeout/schema decision record.
