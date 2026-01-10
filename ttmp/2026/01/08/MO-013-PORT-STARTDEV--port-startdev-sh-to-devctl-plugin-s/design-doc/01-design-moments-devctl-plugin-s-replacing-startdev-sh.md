---
Title: 'Design: Moments devctl plugin(s) replacing startdev.sh'
Ticket: MO-013-PORT-STARTDEV
Status: active
Topics:
    - devctl
    - moments
    - devtools
    - scripting
DocType: design-doc
Intent: long-term
Owners: []
RelatedFiles:
    - Path: devctl/pkg/doc/topics/devctl-scripting-guide.md
      Note: Practical plugin patterns (stdout discipline
    - Path: devctl/pkg/doc/topics/devctl-user-guide.md
      Note: Authoritative pipeline mental model and phase mapping.
    - Path: moments/.devctl.yaml
      Note: Existing config that should keep pointing at the Moments plugin.
    - Path: moments/backend/cmd/moments-config/get_cmd.go
      Note: Candidate extension point to support AppConfig-derived VITE_* values.
    - Path: moments/plugins/moments-plugin.py
      Note: Primary implementation location for the v2 plugin described here.
    - Path: moments/scripts/startdev.sh
      Note: Replacement target; defines expected dev workflow responsibilities.
ExternalSources: []
Summary: Proposed v2 devctl plugin design (ops, steps, services, config/env rules) to replace moments/scripts/startdev.sh; explicitly breaks compatibility with the previous Moments devctl stub.
LastUpdated: 2026-01-08T02:02:45-05:00
WhatFor: Serve as the implementation-ready spec for porting startdev.sh to a devctl plugin.
WhenToUse: Use when implementing or reviewing the Moments devctl plugin and associated helpers.
---


# Design: Moments devctl plugin(s) replacing startdev.sh

## Executive Summary

Replace `moments/scripts/startdev.sh` with a single `devctl` plugin (protocol v2) configured via `moments/.devctl.yaml`. The plugin implements `build.run` (build backend artifacts), `prepare.run` (web deps + generated version file), `validate.run` (tooling + port checks), and `launch.plan` (backend + web service definitions), while `devctl` owns process supervision, logs, and state.

To preserve the “derive VITE_* from AppConfig” behavior without forcing a build during `devctl plan`, the design computes VITE env in `launch.plan` (after `build.run`) using `backend/dist/moments-config` when available, and falls back to safe defaults otherwise.

## Problem Statement

`moments/scripts/startdev.sh` is a repo-specific orchestration script that:
- does environment/config derivation
- performs setup steps (JS deps, version generation)
- and also performs lifecycle work (killing processes on ports, starting and supervising background processes, writing log files, waiting for readiness)

This makes local development brittle and opaque. `devctl` exists to standardize orchestration: the plugin should compute “what to run”, and `devctl` should own “how to run it” (supervision, logs, state, restart/stop).

Additionally, the repo already contains a Moments plugin stub (`moments/plugins/moments-plugin.py`) but it uses protocol v1, which `devctl` rejects (v2-only). We need a clear v2 plugin design that can become the new supported “one command startup”.

## Proposed Solution

### One plugin: `moments` (preferred)

Keep a single repo-local plugin that owns all “Moments dev environment” knowledge. Use `.devctl.yaml` to wire it into `devctl`.

**Files (existing paths kept):**
- `moments/.devctl.yaml` (plugin config)
- `moments/plugins/moments-plugin.py` (plugin implementation; upgrade to protocol v2)

### Plugin handshake (protocol v2)

Handshake frame:
- `type`: `handshake`
- `protocol_version`: `v2`
- `plugin_name`: `moments`
- `capabilities.ops`: `["config.mutate","validate.run","build.run","prepare.run","launch.plan"]`
- Optional: `capabilities.ops` includes `command.run` and `capabilities.commands` for helper commands (see below).

### Config and naming conventions

Follow `devctl` key guidelines (stable, dotted keys):
- `services.backend.port`: backend listen port (default `8083`)
- `services.web.port`: Vite dev server port (default `5173`)
- `env.VITE_IDENTITY_BACKEND_URL`, `env.VITE_IDENTITY_SERVICE_URL`: identity base URL (default `http://localhost:<backend_port>`)
- `env.VITE_BACKEND_URL`: optional “public backend” URL (default `http://localhost:8082`, matching current `startdev.sh` fallback)
- `env.VITE_STYTCH_PUBLIC_TOKEN`: optional; only set if explicitly resolved

Service names in `launch.plan`:
- Prefer namespaced service identities to avoid collisions if plugins stack later:
  - `moments.backend`
  - `moments.web`

### `config.mutate` (no side effects)

Purpose: publish stable defaults into config for visibility and downstream use.

Behavior:
- Read ports from environment overrides if provided (recommended: `MOMENTS_BACKEND_PORT`, `MOMENTS_WEB_PORT`), else default to `8083` and `5173`.
- Patch:
  - `services.backend.port`, `services.web.port`
  - default `env.VITE_*` values that do not require reading AppConfig (safe fallbacks only)

Rationale: `config.mutate` runs before `build.run`, so it should not depend on build artifacts.

### `validate.run` (actionable checks)

Minimum checks:
- Tools: `go`, `make`, `pnpm`, `python3` (and optionally `node` if needed separately).
- Repo layout: expected directories exist (`backend/`, `web/`).
- Ports: fail early if `services.backend.port` or `services.web.port` are already in use (instead of killing processes).

Output:
- `valid=false` with clear install instructions / remediation (e.g., “run `devctl down` if these are devctl-owned processes; otherwise free the port”).

### `build.run` (named steps, deterministic)

Default steps:
- `backend.build` → runs `GOWORK=off make -C backend build`

Artifacts (optional but useful):
- `artifacts.moments_config`: `backend/dist/moments-config`

Dry-run:
- When `ctx.dry_run=true`, do not execute builds; instead emit intended commands to stderr and return `ok=true`.

### `prepare.run` (named steps, deterministic)

Default steps:
- `web.deps` → `pnpm install --prefer-offline` (cwd `web/`)
- `web.version` → `pnpm generate-version` (cwd `web/`)

Dry-run:
- Skip execution; log intended commands to stderr.

### `launch.plan` (service definitions; devctl supervises)

Services:
- `moments.backend`
  - `cwd`: `backend`
  - `command`: `["make","run"]`
  - `env`: `{ "PORT": "<services.backend.port>", "GOWORK": "off" }`
  - `health`: TCP on `127.0.0.1:<services.backend.port>` (matches current “port is listening” readiness)

- `moments.web`
  - `cwd`: `web`
  - `command`: `["pnpm","run","dev","--","--port","<services.web.port>"]`
  - `env`: computed VITE vars (see below)
  - `health`: TCP on `127.0.0.1:<services.web.port>`

Computing VITE env (priority order):
1. Respect explicit user-provided env (already present in the parent environment): if `VITE_IDENTITY_BACKEND_URL` / `VITE_IDENTITY_SERVICE_URL` / `VITE_BACKEND_URL` are set, use them as-is.
2. If `backend/dist/moments-config` exists, query AppConfig values (see “AppConfig resolution” below).
3. Fallbacks (match current script intent):
   - `VITE_BACKEND_URL="http://localhost:8082"`
   - `VITE_IDENTITY_BACKEND_URL="http://localhost:<services.backend.port>"`
   - `VITE_IDENTITY_SERVICE_URL="http://localhost:<services.backend.port>"`

Secret handling:
- Never print secrets to stdout (protocol); avoid printing them to stderr as well. If logging that a token is present, log `***` only.

### AppConfig resolution (for parity with `startdev.sh`)

Goal: keep the “derive VITE_* from AppConfig” behavior without introducing Python YAML dependencies.

Approach:
- Treat `backend/dist/moments-config` as the “AppConfig reader” helper invoked by the plugin.
- Extend `moments-config get` to explicitly support the keys `startdev.sh` uses:
  - `platform.mento-service-public-base-url`
  - `platform.mento-service-identity-base-url`
  - `integrations.stytch.stytch-public-token` (optional)

This extension is small and keeps the plugin dependency-free while preserving the documented flow in `moments/docs/getting-started.md`.

If the helper cannot resolve a key, the plugin should treat it as unset and use the fallback behavior above.

### Optional: `command.run` helpers (escape hatches)

`startdev.sh` currently kills whatever is on the backend/web ports. `devctl` should not do this implicitly, but we can offer an explicit command for developers who need it.

Proposed commands:
- `kill-ports` (dangerous; requires `--force`)
  - Args: `ports` (string, default: backend+web ports), `force` (bool)
  - Behavior: if `force` is not set, exit non-zero with a message; if set, attempt to kill listeners on the given ports.

This keeps the default workflow safe, while still providing a “break glass” option.

## Design Decisions

1. Single plugin (not a multi-plugin stack): smallest surface area, easiest to maintain, matches the “one repo = one plugin” onboarding story from the user/scripting guides.
2. Protocol v2 handshake: required by current `devctl` implementation.
3. No automatic port killing: lifecycle belongs to `devctl` and the operator; `validate.run` should be the primary guardrail.
4. Compute AppConfig-derived values in `launch.plan`: it runs after `build.run`, so it can safely depend on the presence of built helpers without violating `devctl plan` / dry-run expectations.
5. No backwards compatibility with existing Moments devctl behavior: do not preserve protocol v1 handshakes, legacy service naming, or “compat” config keys if they differ from this spec.

## Non-Goals

- Keep the old Moments devctl plugin working (protocol v1).
- Preserve prior `devctl` service identifiers if they differ from the chosen names in this design.

## Alternatives Considered

1. Keep `startdev.sh` and add minor fixes: doesn’t address supervision/log/state consistency; continues to kill arbitrary processes on ports.
2. Split into multiple plugins (config vs. services): clearer separation, but unnecessary complexity for a single-repo dev environment and increases collision/priority concerns.
3. Write the plugin in bash + `jq`: viable, but higher risk of stdout contamination and harder to keep robust; Python is already present and the repo already uses a Python plugin stub.
4. Parse YAML directly in the Python plugin: would require bundling a YAML parser or assuming `PyYAML` is installed, which is fragile for onboarding.

## Implementation Plan

1. Update `moments/plugins/moments-plugin.py` to protocol v2 handshake and align op schemas to the v2 authoring guide.
2. Implement/confirm `validate.run` tool + port checks (no killing).
3. Implement `build.run` and `prepare.run` named steps (with dry-run behavior).
4. Implement `launch.plan` with `moments.backend` and `moments.web` services and readiness checks.
5. Extend `moments/backend/cmd/moments-config/get_cmd.go` to support the AppConfig keys required by the plugin and the existing documentation.
6. Update developer docs to recommend `devctl up` from `moments/` and deprecate `scripts/startdev.sh` (optionally keep a wrapper script that prints migration instructions).
7. Add `.devctl/` to `moments/.gitignore` if not already present.

## Open Questions

1. Should readiness use HTTP health endpoints (`/healthz` or `/health`) instead of TCP listen checks? (TCP matches existing behavior; HTTP is more correct if the routes are stable.)
2. Should service names be short (`backend`, `web`) to match current docs, or namespaced (`moments.backend`, `moments.web`) to future-proof plugin stacking?
3. If `devctl plan` is expected to resolve AppConfig-derived VITE_* values, do we accept that it may require `backend/dist/moments-config` to already exist (built previously), or do we keep the “fallback to defaults” semantics during plan?

## References

- `devctl/pkg/doc/topics/devctl-user-guide.md`
- `devctl/pkg/doc/topics/devctl-scripting-guide.md`
- `devctl/pkg/doc/topics/devctl-plugin-authoring.md`
- `moments/scripts/startdev.sh`
