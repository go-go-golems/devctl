---
Title: Lifecycle Recipe Timeout and Schema Decisions
Ticket: DEVCTL-ROBUST-LIFECYCLE
Status: active
Topics:
    - devctl
    - architecture
    - supervisor
DocType: reference
Intent: long-term
Owners: []
RelatedFiles:
    - Path: repo://pkg/operator/controller.go
      Note: Prepared replacement application under lifecycle lock
    - Path: repo://pkg/operator/planner.go
      Note: Lifecycle recipe resolution preparation and stale-apply guard
ExternalSources: []
Summary: Accepted lifecycle staging timeout and persisted schema decisions.
LastUpdated: 2026-09-13T11:45:00Z
WhatFor: Implementation guardrails for recipe separation and artifact provenance.
WhenToUse: Before changing lifecycle orchestration timeout semantics or run-state schemas.
---

# Lifecycle Recipe, Timeout, and Schema Decisions

## Status

Accepted for `DEVCTL-ROBUST-LIFECYCLE` on 2026-09-13.

## Lifecycle stages

The operator lifecycle is split into three semantic stages:

1. `ResolveRecipe` loads repository configuration, resolves the selected profile, records phase intent, and computes a repository fingerprint. It does not start plugins or run build, prepare, validation, or launch operations.
2. `PrepareReplacement` verifies the recipe fingerprint, starts selected plugins, runs enabled phases, retains build and prepare results, and resolves launch facts.
3. Apply runs under the repository lifecycle lock. It calls `ValidatePrepared` before stopping any owned process. A changed configuration or profile produces `E_RECIPE_STALE`; devctl does not silently replan under the lock.

Preparation remains outside the lifecycle lock so a long build does not block an unrelated explicit stop. Immutable artifact publication addresses concurrent output ownership separately.

## Timeout scope

`PipelinePolicy.Timeout` remains a per-phase and service-operation budget. Each plugin phase receives a fresh child context with that duration. This preserves existing CLI behavior and avoids silently converting `--timeout` into one shrinking operation-wide budget.

The caller's context remains an overall upper bound when it has a deadline or is cancelled. Plugin startup and cleanup retain their explicit bounded policies. The supported Python subprocess runner uses one monotonic request budget internally; that runner contract does not redefine the operator's phase timeout.

A future separate overall deadline must use a separately named field and flag. It must not reinterpret `--timeout`.

## Recipe schema

`LifecycleRecipeSchemaVersion` starts at `1`. Recipes and prepared launches are ephemeral typed values, not persisted state. Unsupported recipe versions are rejected. No compatibility reader or implicit upgrade is added.

Recipes contain phase names, enabled/skipped state, selected step names, selection, profile, and unresolved launch fields. Runtime policy needed for preparation is not serialized. Raw plugin environments and mutated configuration are not stored in the recipe.

## Staleness identity

The repository fingerprint is SHA-256 over a canonical JSON projection of:

- lifecycle recipe schema version;
- resolved profile name;
- merged repository configuration.

The digest can reflect secret-bearing configuration without storing the original values. It is checked before preparation and again under the lifecycle lock immediately before apply. Configuration changes require explicit re-resolution and preparation.

## Run-state and artifact migration

Typed artifact provenance is an intentional persisted schema change. `RunSchemaVersion` will advance from `1` to `2`. Readers reject version-1 run records rather than fabricating provenance or silently upgrading historical state. This follows the repository's no-implicit-compatibility policy.

The first artifact scope is a directly launched native executable. A service either uses its ordinary command path or explicitly references one produced executable artifact; ambiguous simultaneous declarations are rejected. Interpreter scripts, container images, dynamic libraries, and arbitrary asset sets remain outside the first schema.

Artifact records contain only artifact ID, content-addressed path, SHA-256, and byte size. Referenced executables are staged during preparation, published under `.devctl/artifacts/sha256/<digest>/` while the lifecycle lock is held, copied into the service run, and revalidated before wrapper launch.

Garbage collection is included in the first implementation. It protects the artifacts referenced by every service's current and immediately previous run and removes other valid digest directories. Older run records retain identity evidence even when their executable bytes have been collected. No leases, manifests, pinning, quotas, quarantine, or background collector are introduced.
