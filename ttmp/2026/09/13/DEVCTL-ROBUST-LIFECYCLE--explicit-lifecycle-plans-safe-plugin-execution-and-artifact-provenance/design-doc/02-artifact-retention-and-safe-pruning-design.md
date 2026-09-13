---
Title: Simple Content-Addressed Artifact Retention and Garbage Collection
Ticket: DEVCTL-ROBUST-LIFECYCLE
Status: active
Topics:
    - devctl
    - architecture
    - supervisor
DocType: design-doc
Intent: long-term
Owners: []
RelatedFiles:
    - Path: repo://pkg/doc/topics/devctl-artifact-provenance.md
      Note: Embedded Glazed operator and plugin help
    - Path: repo://pkg/operator/artifacts.go
      Note: Content-addressed staging publication and reference collector
    - Path: repo://pkg/operator/controller.go
      Note: Lifecycle lock protects publication run linkage and collection
    - Path: repo://pkg/operator/planner.go
      Note: Prepared launches stage referenced executable artifacts
    - Path: repo://pkg/runstate/artifact.go
      Note: Artifact digest inspection and validation
    - Path: repo://pkg/runstate/schema.go
      Note: Run records retain selected artifact identity
ExternalSources: []
Summary: Minimal content-addressed storage and reference-based collection for build-produced service executables.
LastUpdated: 2026-09-13T11:56:00Z
WhatFor: Bound artifact storage while preserving executables needed by current and immediately previous service runs.
WhenToUse: When implementing or reviewing artifact publication run linkage and garbage collection.
---

# Simple Content-Addressed Artifact Retention and Garbage Collection

## Scope

Devctl needs to answer one operational question: which exact executable bytes did a service attempt launch? Build and prepare plugins currently return named artifact paths, but mutable paths do not identify bytes and can be replaced by a later build.

The feature is restricted to native service executables explicitly referenced by `launch.plan`. It does not track plugin executables, interpreter scripts, containers, dynamic libraries, or arbitrary build assets.

The implementation has four operations:

```text
copy -> hash -> publish -> record
```

Garbage collection is a small reference scan over the resulting content-addressed files. There are no leases, manifests, quarantine directories, automatic schedules, retention configuration, or resumable deletion protocol.

## Data Model

A build or prepare response continues to declare named paths:

```json
{
  "artifacts": {
    "api-server": "build/bin/api-server"
  }
}
```

A service launch plan may select one produced executable:

```json
{
  "name": "api",
  "executable": {
    "artifact_id": "api-server",
    "args": ["--port", "8080"]
  }
}
```

A service must declare either an ordinary `command` or an `executable` reference, never both. The referenced artifact ID must exist in the merged build/prepare outputs.

The selected run evidence is intentionally small:

```go
type ArtifactRecord struct {
    ID        string `json:"id"`
    Path      string `json:"path"`
    SHA256    string `json:"sha256"`
    SizeBytes int64  `json:"size_bytes"`
}
```

`RunRecord.Artifact` stores this record before wrapper launch. Historical records preserve the digest even if garbage collection later removes the stored executable.

## Publication

Preparation copies each referenced executable to operation-owned staging while computing its digest. The lifecycle apply path then publishes it beneath:

```text
.devctl/artifacts/sha256/<sha256>/executable
```

The SHA-256 directory is the content identity. Identical bytes reuse the same directory. Different builds that produce identical bytes do not create duplicate payloads.

Publication occurs while the repository lifecycle lock is held:

1. Validate the staged file is regular and executable.
2. Verify its digest and size.
3. Create the digest directory.
4. Atomically move the staged file to its content-addressed destination.
5. If the destination already exists, verify it and discard the duplicate staging file.
6. Rewrite the resolved service command to the immutable destination plus declared arguments.
7. Create the run record with the selected `ArtifactRecord`.
8. Launch the wrapper only after one final digest check.

Builds and plugin calls remain outside the lifecycle lock. Only the short publication and run-linkage step is locked.

## Garbage Collection

Collection protects artifacts selected by each service's:

- current run;
- immediately previous (`last`) run.

These references come from `EnvironmentState` and the corresponding `RunRecord` values. Every digest directory not in that protected set is removable.

```text
protected = empty set
for each service slot:
    for current_run_id and last_run_id:
        load run
        if run has artifact:
            protected.add(run.artifact.sha256)

for each digest directory in .devctl/artifacts/sha256:
    if digest not in protected:
        remove digest directory
```

Collection runs under the repository lifecycle lock after durable state changes. Staging is stored outside the `sha256` subtree, so collection never examines an in-progress preparation.

The collector is deliberately conservative when state cannot be read: it returns an error and deletes nothing. Unknown files and malformed digest directory names are ignored rather than guessed safe.

Older run records retain artifact ID, digest, size, and original content-addressed path as historical evidence. Their executable bytes may no longer exist once the run is neither current nor last. Devctl must not claim those older artifacts remain launchable.

## Storage Bound

The simple policy retains at most the distinct executable digests referenced by current and last runs, plus temporary orphans until the next successful collection. Content deduplication can reduce this further when multiple runs use identical bytes.

The number of protected payloads is therefore bounded by service history rather than elapsed time:

```text
maximum protected digests <= 2 * number of services
```

Several services may share one digest, so actual storage can be smaller. Failed publication can leave an unreferenced digest, which the next collection removes.

## Failure Semantics

- Missing referenced artifact: preparation fails before any existing service is stopped.
- Non-regular or non-executable artifact: preparation fails.
- Digest changes while copying or validating: preparation/apply fails.
- Both `command` and `executable` declared: plan is rejected.
- Destination already exists with matching identity: reuse it.
- Destination exists with mismatched identity: report corruption and do not launch.
- State cannot be loaded for collection: delete nothing.
- One unreferenced digest cannot be removed: report the failure; protected digests remain untouched.

## Path Safety

Artifact IDs use a restrictive filename grammar. Digest directories must contain exactly a lowercase hexadecimal SHA-256 value. The publisher constructs every destination beneath the repository's fixed `.devctl/artifacts/sha256` root. The collector enumerates only direct digest children and does not follow symlinked entries.

No user-supplied garbage-collection root or shell deletion command is supported.

## Tests

The implementation must prove:

1. Recipe resolution does not execute plugins.
2. A referenced build artifact is copied and launched from the SHA-256 store.
3. The run record contains matching ID, path, digest, and size.
4. A service declaring both command forms is rejected.
5. A missing, non-regular, or non-executable output is rejected before stop.
6. Corrupting the staged artifact rejects apply before stop.
7. Corrupting the published artifact rejects wrapper launch.
8. Identical bytes reuse one digest directory.
9. Current and last run artifacts survive collection.
10. An unreferenced digest directory is removed.
11. Malformed and symlinked directory entries are ignored.
12. A state-read failure causes no deletion.

## Non-Goals

This feature does not provide configurable age limits, byte quotas, pinning, archival manifests, preparation leases, quarantine, resumable deletion, or background maintenance. Those mechanisms are unnecessary for the current bounded current/last retention policy.

If operational evidence later requires longer executable retention, extend the protected-reference calculation rather than replacing the content-addressed store.
