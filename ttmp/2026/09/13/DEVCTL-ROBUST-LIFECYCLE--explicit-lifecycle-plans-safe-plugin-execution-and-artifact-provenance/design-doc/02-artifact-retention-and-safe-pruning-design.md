---
Title: Artifact Retention and Safe Pruning Design
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
    - Path: repo://pkg/operator/controller.go
      Note: Lifecycle lock and durable run-reference boundary
    - Path: repo://pkg/operator/planner.go
      Note: Prepared-launch leases and build ownership boundary
    - Path: repo://pkg/runstate/schema.go
      Note: Run references that protect immutable artifact builds
ExternalSources: []
Summary: Reference-aware mark-and-sweep design for safely pruning immutable service executable artifacts.
LastUpdated: 2026-09-13T11:53:00Z
WhatFor: Prevent unbounded artifact storage without deleting executables referenced by active or retained runs.
WhenToUse: Before implementing artifact list retention prune garbage collection or storage quota behavior.
---

# Artifact Retention and Safe Pruning Design

## Executive Summary

Devctl's immutable artifact publication stores each build-produced native executable at a build-specific path beneath `.devctl/artifacts/`. Immutability makes launch identity auditable and prevents concurrent builds from replacing the executable selected for a service attempt. It also means successive preparations create additional files.

Pruning must be reference-aware. A file is not safe to remove merely because it is old or because a newer build exists. Current runs, live processes, attempts with unconfirmed termination, and retained historical evidence can all refer to older artifacts. The proposed collector therefore uses a locked mark-and-sweep algorithm: derive protected build identities from authoritative run state, select only unreferenced build directories, revalidate the state snapshot, and delete complete immutable units.

This document records a future design. Automatic pruning is not part of the first artifact-provenance implementation. Until this design is implemented and qualified, devctl must not delete `.devctl/artifacts` automatically.

## Problem Statement

A prepared lifecycle operation may copy a build output into a path such as:

```text
.devctl/artifacts/<build-id>/<artifact-id>/<sha256>/<filename>
```

The directory is intentionally never overwritten. Every successful preparation can therefore add one or more artifacts. Long-lived repositories, frequent restarts, CI loops, and failed preparations can produce meaningful disk usage.

A naive age-based cleanup is unsafe. An old executable may still be:

- selected by an active service run;
- referenced by a run whose wrapper or child process has unknown liveness;
- needed to inspect or reproduce a retained failure;
- selected for a prepared operation that has not yet acquired the lifecycle lock;
- the most recent known-good build after a newer preparation failed;
- shared by several service attempts through the same build identity.

Cleanup also races with preparation and apply. A collector that scans before a run record is written could delete a newly published artifact. A collector that deletes one file inside a build directory could leave a partially valid provenance set. A collector that trusts only environment slots can miss retained run records and unindexed recovery evidence.

## Safety Invariants

An implementation must preserve these invariants:

1. Never delete an artifact referenced by a current run.
2. Never delete an artifact referenced by a process whose exit is unconfirmed.
3. Never delete an artifact referenced by an in-flight prepared replacement.
4. Never mutate or partially prune an immutable build directory.
5. Never infer safety from pathname age alone.
6. Never follow symlinks out of the managed artifact root.
7. Never delete a directory whose identity or layout fails validation; report it for operator review.
8. Recompute references under the repository lifecycle lock immediately before deletion.
9. A dry run and an actual prune use the same candidate-selection code.
10. Interrupted deletion leaves either an intact build or a quarantined deletion unit that a later run can finish safely.

## Storage Model

The collector operates on a complete **build unit**. The build ID is the deletion granularity:

```text
.devctl/artifacts/
└── <build-id>/
    ├── manifest.json
    ├── api-server/
    │   └── <sha256>/api-server
    └── worker/
        └── <sha256>/worker
```

`manifest.json` should be written atomically after all referenced artifacts have been published and validated. It should contain:

```go
type ArtifactBuildManifest struct {
    Version     int              `json:"version"`
    BuildID     string           `json:"build_id"`
    RecipeID    string           `json:"recipe_id"`
    CreatedAt   time.Time        `json:"created_at"`
    Artifacts   []ArtifactRecord `json:"artifacts"`
    TotalBytes  int64            `json:"total_bytes"`
}
```

The manifest must not contain raw environments, secrets, plugin request bodies, or mutable source paths unless those paths are explicitly classified as non-sensitive evidence. Artifact records contain immutable publication paths, digests, sizes, producer identity, and build identity.

Prepared operations need discoverable ownership before they create files. One acceptable design is an atomic lease file under `.devctl/artifacts/.leases/<recipe-id>.json`. Preparation creates the lease before publication and removes it only after apply finishes or the preparation is explicitly abandoned. A lease has a bounded expiry plus process identity; expiry alone is not sufficient when the owner is still alive.

## Reference Classes

The mark phase builds a protected set of build IDs from all authoritative sources.

### Hard references

Hard references always prevent deletion:

- `EnvironmentState.Services[*].CurrentRunID` and each referenced run's artifact;
- any `RunRecord` in `planned`, `starting`, `ready`, `stopping`, or `unknown` phase;
- terminal runs whose wrapper or child ownership has not been conclusively reconciled;
- valid unexpired preparation leases;
- build IDs explicitly pinned by an operator.

### Retention references

Retention references depend on configured policy:

- the latest N successful terminal runs per service;
- failed runs younger than a failure-evidence duration;
- all runs younger than a general minimum age;
- named releases or manually retained run IDs;
- the latest N complete builds even when no run references them.

Hard references are correctness requirements. Retention references are evidence and usability policy. Quotas may reduce optional retention but must never override hard references.

## Proposed CLI

```text
devctl artifacts list
devctl artifacts inspect BUILD_ID
devctl artifacts prune --dry-run
devctl artifacts prune --older-than 30d --keep-builds 10
devctl artifacts prune --max-total-size 20GiB --keep-failed-for 14d
devctl artifacts pin BUILD_ID
devctl artifacts unpin BUILD_ID
```

`artifacts list` and `prune --dry-run` should produce structured rows with:

```text
build_id
created_at
total_bytes
artifact_count
reference_class
referenced_by_runs
referenced_by_services
lease_state
candidate
reason
```

Pruning should require an explicit policy. Devctl must not silently choose age, count, or quota defaults in the first release. If automatic maintenance is added later, it must use repository configuration with visible defaults and emit structured operation evidence.

## Mark-and-Sweep Algorithm

### Phase 1: inventory without mutation

```text
validate managed artifact root
read build manifests without following symlinks
read environment state and all run records
read active preparation leases
read explicit pins
construct build inventory
construct hard-reference set
construct policy-retention set
classify candidates and diagnostics
```

Malformed manifests, unknown schema versions, symlinks, path escapes, duplicate build IDs, and size/digest contradictions are diagnostics, not deletion candidates.

### Phase 2: policy selection

```text
candidates = complete builds
             - hard references
             - retention references
             - malformed or uncertain units

apply minimum age
apply keep-latest count per service/build stream
if above quota:
    order remaining candidates oldest first
    select until projected size is below quota
```

Ordering must be deterministic: creation time, then build ID. The report includes both the initial total and projected total.

### Phase 3: locked revalidation

Acquire the same repository lifecycle lock used for apply and stop. Under the lock:

```text
reload environment and run records
reload leases and pins
recompute hard references
compare each candidate with the fresh protected set
reject any newly referenced candidate
atomically rename each accepted build directory into .trash/
release lock
```

Renaming into `.trash/<operation-id>/<build-id>` makes the build unavailable as one complete unit while keeping the locked section short. The rename must remain on the same filesystem. If the artifact root and trash directory cannot support atomic rename, pruning must fail rather than fall back to file-by-file deletion.

### Phase 4: deletion outside the lock

After the lock is released, recursively delete only validated directories beneath the operation-owned trash root. A later prune can resume deletion of abandoned trash operations. No active run can reference a trashed build because the locked revalidation protected all current references before rename.

## Concurrency and Crash Recovery

Preparation, apply, and pruning share these ownership rules:

- preparation creates a lease before publishing its first artifact;
- apply validates artifact bytes and records the artifact in `RunRecord` before wrapper launch;
- the lease remains until the run reference is durable or preparation is abandoned;
- prune treats an uncertain lease as protected;
- prune performs candidate revalidation and quarantine rename under the lifecycle lock;
- recursive deletion occurs outside the lock;
- startup diagnostics report stale leases and incomplete trash operations.

If devctl crashes after quarantine rename, the build is no longer referenced and remains under `.trash`. A subsequent prune may finish deleting it using the operation manifest. If devctl crashes during preparation, the lease and incomplete manifest prevent automatic deletion until reconciliation proves the owner dead and the configured grace period expires.

## Path and Filesystem Safety

The collector must:

- open the configured artifact root and resolve it once;
- reject artifact or manifest paths outside that root;
- use `Lstat` and reject symlinked build, artifact, digest, and trash components;
- validate build IDs and artifact IDs against a restrictive filename grammar;
- avoid shell commands for deletion;
- never accept a user-supplied arbitrary deletion root;
- ensure quarantine source and destination are on the same filesystem;
- use an operation-specific trash directory created with owner-only permissions;
- report, rather than delete, unknown files at the artifact-root level.

Digest verification is not required merely to reclaim bytes from an otherwise valid unreferenced build, but digest contradictions should be reported because they indicate provenance corruption. Policy should offer a separate `artifacts verify` operation rather than making every prune hash every retained executable.

## Retention Policy

A possible repository configuration is:

```yaml
artifacts:
  retention:
    enabled: false
    minimum_age: 168h
    keep_builds: 10
    keep_successful_runs_per_service: 3
    keep_failed_for: 336h
    max_total_size: 20GiB
```

Interpretation:

- `enabled: false` means no automatic prune;
- `minimum_age` protects recent complete builds regardless of references;
- `keep_builds` protects the newest complete build units;
- `keep_successful_runs_per_service` derives references from terminal run history;
- `keep_failed_for` preserves recent failure evidence;
- `max_total_size` triggers additional deletion only among otherwise eligible candidates.

A quota is a target, not permission to violate hard references. If protected builds exceed the quota, pruning reports that the target cannot be met and exits nonzero without deleting protected data.

## Operation Evidence

Every actual prune should write an operation receipt containing:

```go
type ArtifactPruneReceipt struct {
    Version          int                  `json:"version"`
    OperationID      string               `json:"operation_id"`
    StartedAt        time.Time            `json:"started_at"`
    FinishedAt       time.Time            `json:"finished_at"`
    Policy           ArtifactPolicy       `json:"policy"`
    InitialBytes     int64                `json:"initial_bytes"`
    ProjectedBytes   int64                `json:"projected_bytes"`
    ReclaimedBytes   int64                `json:"reclaimed_bytes"`
    Decisions        []ArtifactDecision   `json:"decisions"`
    Errors           []ArtifactPruneError `json:"errors,omitempty"`
}
```

Each decision records the build ID, action (`retain`, `quarantine`, `delete`, or `skip`), and concrete reason. Receipts must not claim reclaimed bytes until deletion succeeds.

## Failure Semantics

- Invalid policy: usage error; no mutation.
- Lifecycle lock unavailable: operation-busy error; no mutation.
- State or run record unreadable: state-corrupt error; no mutation.
- Manifest unknown or malformed: skip that build, report diagnostic, continue only if requested policy permits partial results.
- Candidate becomes referenced during revalidation: retain it and report the changed decision.
- Quarantine rename fails: retain the source; do not attempt recursive deletion.
- Recursive deletion fails after quarantine: preserve the receipt and trash directory for retry; report partial failure.
- Quota cannot be met without protected deletion: retain protected builds and return an unmet-quota diagnostic.

## Alternatives Considered

### Delete by modification time

Rejected because filesystem time does not represent lifecycle ownership and can select an artifact used by an active run.

### Keep only the latest build

Rejected because active and retained attempts can legitimately reference older build IDs, and a newer build may have failed validation.

### Delete individual artifact files

Rejected because build provenance is a set. Partial deletion creates manifests that appear complete while referenced members are missing.

### Hold the lifecycle lock during recursive deletion

Rejected because large directory deletion would block stop and apply operations unnecessarily. Only reference revalidation and atomic quarantine rename require the lock.

### Reference counts updated incrementally

Rejected as the sole source of truth because crashes can desynchronize counters. Run records, environment state, leases, and pins remain authoritative; cached counts may accelerate listing only.

## Implementation Plan

1. Finalize artifact build manifests and preparation leases.
2. Add a read-only inventory package with strict root and manifest validation.
3. Add reference derivation from environment state, all run records, leases, and pins.
4. Implement deterministic policy evaluation as a pure function.
5. Add `devctl artifacts list` and `prune --dry-run` using the same candidate engine.
6. Implement locked reference revalidation and same-filesystem quarantine rename.
7. Implement resumable trash deletion and operation receipts.
8. Add pin/unpin support.
9. Add optional configured automatic pruning only after manual operations have field evidence.
10. Document operational recovery and quota diagnostics.

## Test Matrix

Unit and integration tests must cover:

- active current run protects its build;
- stopped retained run obeys retention policy;
- unknown/unconfirmed process protects its build;
- valid preparation lease protects an unpublished or unapplied build;
- config quota cannot override hard references;
- candidate becomes referenced between inventory and locked revalidation;
- concurrent preparation starts while pruning;
- two services share one build;
- latest build failed while an older successful build remains referenced;
- malformed and unknown-version manifests are skipped;
- symlinked build and artifact paths are rejected;
- path traversal IDs are rejected;
- dry run and actual prune select identical candidates from identical state;
- quarantine rename failure leaves source intact;
- crash after quarantine rename is resumable;
- partial recursive deletion produces an accurate receipt;
- deterministic ordering under equal timestamps;
- bytes reclaimed and projected totals are accurate;
- no secret environment values appear in manifests, rows, errors, or receipts.

## Open Questions

1. Should retained terminal runs protect artifacts indefinitely until an explicit run-retention operation removes them, or should artifact and run retention be evaluated together?
2. Should pins live in repository configuration, a separate state file, or both with distinct team/local semantics?
3. What grace period proves an abandoned preparation lease is collectible after process death?
4. Should successful preparations that never reached apply be retained as latest known builds or treated as short-lived orphans?
5. Should quota units accept decimal and binary suffixes, and which parser owns that contract?
6. Should trash deletion be synchronous by default or optionally detached with a receipt-polled operation?

None of these questions permits deletion of hard-referenced artifacts. Conservative retention is the required fallback.

## References

- `reference/03-lifecycle-recipe-and-schema-decisions.md` — accepted recipe, timeout, and initial artifact migration decisions.
- `design-doc/01-lifecycle-robustness-analysis-design-and-intern-implementation-guide.md` — parent lifecycle robustness design and artifact provenance proposal.
- `pkg/runstate/schema.go` — authoritative environment and service-run records.
- `pkg/operator/planner.go` — lifecycle recipe and prepared-launch ownership.
- `pkg/operator/controller.go` — lifecycle lock and run-record publication boundary.
