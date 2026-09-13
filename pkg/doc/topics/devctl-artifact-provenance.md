---
Title: Verified Service Executables and Artifact Garbage Collection
Slug: artifact-provenance
Short: Launch build-produced native executables from verified content-addressed storage and understand their retention.
Topics:
- plugins
- lifecycle
- artifacts
- provenance
Commands:
- up
- restart
- status
- build
- prepare
Flags:
- skip-build
- build-step
- prepare-step
IsTopLevel: false
IsTemplate: false
ShowPerDefault: true
SectionType: GeneralTopic
---

Devctl can bind a service attempt to the exact native executable bytes produced by `build.run` or `prepare.run`. The plugin names the output, `launch.plan` selects that name, and devctl launches a verified read-only copy instead of the mutable build path. This makes `status` and persisted run evidence identify what actually ran.

## Declare a build artifact

A build or prepare response maps a stable artifact ID to an output path. Relative paths resolve against the repository root. The path must identify a regular executable file when a service selects it.

```json
{
  "type": "response",
  "request_id": "request-1",
  "ok": true,
  "output": {
    "steps": [{"name": "backend", "ok": true}],
    "artifacts": {
      "backend-bin": "backend/dist/server"
    }
  }
}
```

Returning an artifact does not automatically launch it. Unselected outputs remain ordinary plugin result metadata and are not copied into devctl's executable store.

## Select the executable in `launch.plan`

A service selects the artifact by ID and supplies only the arguments that follow the executable path:

```json
{
  "type": "response",
  "request_id": "request-2",
  "ok": true,
  "output": {
    "services": [{
      "name": "backend",
      "cwd": "backend",
      "executable": {
        "artifact_id": "backend-bin",
        "args": ["--port", "8083"]
      },
      "health": {
        "type": "http",
        "url": "http://127.0.0.1:8083/health"
      }
    }]
  }
}
```

A service provides exactly one launch form:

| Form | Use |
|---|---|
| `command` | Launch an ordinary external command by argv. |
| `executable` | Launch a native executable produced by this preparation's build or prepare results. |

Declaring both is an error. Referencing an artifact ID absent from the merged build and prepare results is also an error. Devctl rejects either condition before stopping a service during restart.

## Publication and verification

Preparation copies the selected output into operation-owned staging while checking its digest and executable mode. Under the repository lifecycle lock, devctl publishes it at:

```text
.devctl/artifacts/sha256/<sha256>/executable
```

Identical bytes reuse the same path even when later builds produce them again. Devctl records the artifact ID, path, SHA-256, and byte size in `RunRecord` before wrapper launch. The supervisor verifies those bytes once more immediately before starting the wrapper.

A changed, missing, non-regular, or non-executable file produces `E_ARTIFACT_INVALID`. Restart performs these checks before stopping the current attempt.

During `--dry-run`, devctl validates artifact IDs, launch-form exclusivity, merged build/prepare declarations, and nonempty intended output paths without reading, copying, or requiring the reported executable bytes. A plugin that honors `ctx.dry_run=true` therefore does not need to create its intended output.

## Inspect selected identity

Structured status output includes the selected executable identity:

```bash
devctl status --with-glaze-output --format json \
  --output-fields service,run_id,artifact_id,artifact_sha256,artifact_path,artifact_size_bytes
```

Services launched through an ordinary `command` have empty artifact fields. Historical run JSON under `.devctl/runs/<run-id>/run.json` retains the same identity record.

## Garbage collection

After a successful locked `up` or `restart` state update, devctl scans the content-addressed store. It protects every digest referenced by each service's current and immediately previous run and removes other valid digest directories.

This policy bounds normal retained executable payloads to at most two distinct digests per known service. Shared or unchanged executables reduce the count because content-addressed storage deduplicates identical bytes.

Older run records keep the artifact ID, digest, size, and former path as evidence. Their executable bytes may no longer exist after collection, so an old run record is not a promise that devctl can relaunch that historical binary.

Collection ignores malformed directory names and symlink entries. If authoritative environment or run state cannot be read, collection returns an error and deletes nothing.

## Run schema compatibility

Artifact provenance advances service run records to schema version 2. Devctl rejects version-1 run records instead of silently fabricating missing identity. Stop environments with the older binary before moving a repository to this version.

## Troubleshooting

| Problem | Cause | Solution |
|---|---|---|
| `E_ARTIFACT_INVALID` reports a missing artifact | The build did not create the path returned in `artifacts` | Make the build step write the file before returning success and resolve relative paths from `repo_root`. |
| Artifact is not executable | The output has no executable mode bits | Set the native binary mode before returning the build response. |
| Service declares both launch forms | `launch.plan` contains both `command` and `executable` | Keep exactly one; use `executable` only for a selected build-produced native binary. |
| Artifact ID is undeclared | `executable.artifact_id` does not match a build or prepare result key | Use the same stable ID in both protocol responses. |
| A historical artifact path no longer exists | The run is older than the current and immediately previous attempt | Use the retained SHA-256 as evidence; rebuild explicitly if executable bytes are needed again. |
| Version-1 run state is rejected | The record predates executable provenance | Stop with the earlier binary, then start again with the current version; do not edit run JSON manually. |

## See Also

- `devctl help plugin-authoring` — complete NDJSON plugin and lifecycle response contracts.
- `devctl help plugin-migration` — move existing process ownership into durable devctl supervision.
- `devctl help user-guide` — operate environments, inspect status, and recover failures.
- `devctl help v2-upgrade` — state and command migration requirements.
