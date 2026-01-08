# Tasks

## Plugin (no backwards compatibility with existing devctl stub)

- [x] Replace `moments/plugins/moments-plugin.py` entirely with a protocol v2 plugin (no v1 handshake support).
- [x] Confirm/adjust `moments/.devctl.yaml` to point at the new plugin shape (no legacy flags/paths kept “just in case”).
- [x] Choose and lock service names; default to `moments.backend` + `moments.web` and update any docs accordingly.
- [x] Implement `config.mutate` defaults (ports + safe fallback `env.VITE_*` keys).
- [x] Implement `validate.run` with tooling checks + port-in-use errors (no automatic killing).
- [x] Implement `build.run` with step `backend.build` (`GOWORK=off make -C backend build`) and dry-run behavior.
- [x] Implement `prepare.run` with steps `web.deps` + `web.version` and dry-run behavior.
- [x] Implement `launch.plan` emitting `moments.backend` + `moments.web` services, env derivation, and readiness checks.
- [x] Decide whether to include a dangerous escape-hatch `command.run kill-ports` (requires `--force`), or omit it entirely.

## AppConfig-derived VITE_* values

- [x] Extend `moments/backend/cmd/moments-config/get_cmd.go` to explicitly support:
  - `platform.mento-service-public-base-url`
  - `platform.mento-service-identity-base-url`
  - `integrations.stytch.stytch-public-token`
- [x] Use the helper from the plugin (in `launch.plan`, post-`build.run`) and fall back cleanly if unset/unavailable.

## Docs + hygiene

- [x] Update `moments/docs/getting-started.md` to recommend `devctl up` (and stop describing `startdev.sh` as the primary path).
- [x] Decide whether to delete `moments/scripts/startdev.sh` or keep it as a thin deprecation message that points to `devctl up`.
- [x] Ensure `.devctl/` is ignored under `moments/` (gitignore hygiene).

## Validation (smoke)

- [x] `devctl --repo-root moments plugins list` (plugin loads, stdout clean).
- [x] `devctl --repo-root moments plan` (no side effects, reasonable plan).
- [x] `devctl --repo-root moments up` + `devctl status` + `devctl logs --service moments.backend --follow` (supervision + logs).
- [x] `devctl --repo-root moments down` (clean shutdown + state cleanup).
