---
Title: 'Analysis: Replace startdev.sh with devctl plugin(s)'
Ticket: MO-013-PORT-STARTDEV
Status: active
Topics:
    - devctl
    - moments
    - devtools
    - scripting
DocType: working-note
Intent: long-term
Owners: []
RelatedFiles:
    - Path: devctl/pkg/protocol/validate.go
      Note: Handshake validation enforces protocol v2 (explains why current plugin fails).
    - Path: moments/.devctl.yaml
      Note: Existing devctl wiring for Moments (plugin config).
    - Path: moments/backend/cmd/moments-config/get_cmd.go
      Note: moments-config get currently only supports bootstrap keys; impacts VITE_* derivation.
    - Path: moments/plugins/moments-plugin.py
      Note: Existing plugin stub; currently protocol v1 and incomplete parity.
    - Path: moments/scripts/startdev.sh
      Note: Current one-command dev workflow baseline (config+deps+lifecycle).
ExternalSources: []
Summary: Baseline responsibilities of moments/scripts/startdev.sh, mapping to devctl pipeline ops, and gaps in the existing Moments plugin stub.
LastUpdated: 2026-01-08T02:02:45-05:00
WhatFor: Capture the current-state analysis that motivates and constrains the plugin design.
WhenToUse: Use when reviewing why the design makes specific tradeoffs vs startdev.sh.
---


# Analysis: Replace startdev.sh with devctl plugin(s)

## Summary

- `moments/scripts/startdev.sh` mixes three concerns: config derivation (VITE_*), one-time setup (pnpm install + version gen), and lifecycle/supervision (kill ports, background processes, log files, “wait until ports are up”).
- `devctl`’s model cleanly separates these: plugin computes config + steps + a service plan; `devctl` supervises processes and owns logs/state.
- The repo already has `moments/.devctl.yaml` + `moments/plugins/moments-plugin.py`, but it declares `protocol_version: v1`; current `devctl` rejects anything but v2.
- `startdev.sh` attempts to read AppConfig via `backend/dist/moments-config get <key>`, but the current `moments-config get` command only exposes a small “bootstrap” key set; the AppConfig key lookups are effectively best-effort (stderr suppressed) and often fall back to defaults.

## Notes

### Baseline behavior: `moments/scripts/startdev.sh`

Responsibilities, in order:
- Ensure `backend/dist/moments-config` exists (`make -C backend build` if missing).
- Attempt to read config values (stderr suppressed):
  - `platform.mento-service-public-base-url` → `VITE_BACKEND_URL` (fallback `http://localhost:8082`)
  - `platform.mento-service-identity-base-url` → `VITE_IDENTITY_BACKEND_URL` + `VITE_IDENTITY_SERVICE_URL` (fallback `http://localhost:8083`)
  - `integrations.stytch.stytch-public-token` → `VITE_STYTCH_PUBLIC_TOKEN` (optional)
- Decide ports:
  - backend: `${PORT:-8083}`
  - web: `${VITE_PORT:-5173}`
- Kill any existing processes listening on those ports (hard `kill -9`).
- Ensure web deps:
  - `pnpm install --prefer-offline`
  - `pnpm generate-version`
- Start services:
  - backend: `GOWORK=off PORT=<backend_port> make -C backend run` (background)
  - web: `pnpm run dev -- --port <frontend_port>` (background)
- Wait up to 30s for both ports to listen; print status + log file locations in `moments/tmp/`.

### devctl model (from the user + scripting guides)

Mapping from shell-script actions to `devctl` pipeline ops:
- Env derivation → `config.mutate` (or compute directly in `launch.plan` if values depend on build artifacts).
- Building binaries → `build.run` (named steps, artifacts).
- Installing JS deps / generating files → `prepare.run` (named steps).
- Prerequisite + port checks → `validate.run` (actionable errors/warnings).
- Starting processes → `launch.plan` (service definitions; devctl supervises them).

### Existing Moments devctl wiring (already in repo)

- `moments/.devctl.yaml` exists and points to a Python plugin: `./plugins/moments-plugin.py`.
- `moments/plugins/moments-plugin.py` already has rough phase coverage (`config.mutate`, `build.run`, `prepare.run`, `validate.run`, `launch.plan`) and roughly matches the backend/web commands.
- But it declares `"protocol_version": "v1"`, and `devctl/pkg/protocol/validate.go` rejects non-v2 handshakes. Result: the plugin can’t be used as-is.

### Moments-config mismatch

`startdev.sh` expects `moments-config get` to return arbitrary AppConfig keys like `platform.mento-service-public-base-url`. The current implementation at `moments/backend/cmd/moments-config/get_cmd.go` only exposes a small set of explicit “bootstrap keys” and will error on unknown keys. Because `startdev.sh` suppresses stderr and falls back, this likely means:
- `VITE_BACKEND_URL` and identity URLs usually come from defaults (not from AppConfig).
- `VITE_STYTCH_PUBLIC_TOKEN` usually remains unset (unless developers set it manually).

## Decisions

- Design for a single `moments` plugin (v2 protocol) that fully replaces `startdev.sh` behavior, with optional `command.run` helpers for the few lifecycle behaviors that `devctl` intentionally doesn’t do (like “kill whatever is on this port”).
- Prefer `validate.run` + actionable guidance over automatically killing processes on ports.
- Do not preserve backwards compatibility with the existing Moments devctl stub (protocol v1/plugin semantics); treat it as deprecated/removed.

## Next Steps

- Implement tasks in `devctl/ttmp/2026/01/08/MO-013-PORT-STARTDEV--port-startdev-sh-to-devctl-plugin-s/tasks.md`.
- If extending `moments-config get`, document the supported key list as part of that change.
