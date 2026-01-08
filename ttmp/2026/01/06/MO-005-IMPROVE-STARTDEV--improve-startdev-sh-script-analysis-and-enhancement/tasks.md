# Tasks

## TODO

### Go CLI scaffolding

- [x] Create `devctl` cobra root command with `--repo-root/--dry-run/--strict/--timeout/--log-level`
- [x] Add `devctl up/down/status/logs/plugins` command skeletons (wiring only)

### Protocol v1 (NDJSON)

- [x] Implement protocol types (`Handshake/Request/Response/Event`) + JSON (un)marshal helpers
- [x] Implement protocol validation (required fields, protocol_version, capabilities normalization)
- [x] Define shared error codes (`E_PROTOCOL_*`, `E_TIMEOUT`, `E_UNSUPPORTED`, etc.)

### Plugin runtime (process + IO)

- [x] Implement plugin process start + handshake read with startup timeout
- [x] Implement NDJSON stdout reader that fails hard on non-JSON stdout contamination
- [x] Implement stderr capture (human logs) with structured forwarding (plugin name + timestamps)
- [x] Implement request/response correlation by `request_id` (pending map) + per-call timeouts
- [x] Implement stream routing by `stream_id` (fan-out to subscribers; end event closes channel)
- [x] Implement process-group termination on cancel/shutdown (SIGTERM → grace → SIGKILL)

### Config patching (deterministic composition)

- [x] Implement dotted-path parser + setters for `map[string]any` (create intermediate maps)
- [x] Implement `ConfigPatch.Apply` (set/unset) + tests for collisions and type mismatches
- [x] Implement `ConfigPatch.Merge` + tests (later wins on set, dedupe unset)

### Engine pipeline (phase orchestration)

- [x] Implement plugin ordering (priority + stable tie-break) and per-op support checks
- [x] Implement `config.mutate` loop: call plugins → merge patches → apply to config
- [x] Implement `validate.run` loop: call plugins → AND validity → append errors/warnings
- [x] Implement `build.run` and `prepare.run` step aggregation (collision policy for step names)
- [x] Implement `launch.plan` merge by service name + strictness policy

### Plan-mode supervision (launch + health + logs)

- [x] Implement `Supervisor.Start/Stop/Status` for `launch.plan` services (cwd/env/command)
- [x] Implement health checks (tcp + http) + wait-for-ready with timeout
- [x] Implement log capture strategy (per-service stdout/stderr → files + follow channel)
- [x] Implement `devctl logs --follow <service>` against supervisor-captured logs

### Plugin commands (git-xxx style)

- [x] Implement `commands.list` aggregation + collision policy (warn/error)
- [x] Implement dynamic cobra subcommands for plugin commands (`devctl <cmd> ...`)
- [x] Implement `command.run` dispatch (argv passthrough + config in input)

### Discovery + repo configuration

- [x] Define repo config file format (e.g. `.devctl.yaml`) for plugin list/order/env/workdir
- [x] Implement discovery: configured plugins + `plugins/` directory scan + PATH prefix scan (optional)
- [x] Implement `devctl plugins list` output (path, priority, capabilities)

### Tests + fixtures

- [x] Add runtime tests with fake plugins: ok, noisy stdout, invalid handshake, slow timeout
- [x] Add patch apply tests (nested set/unset, bad types, overwrite semantics)
- [x] Add engine tests for deterministic merge ordering + strictness behavior

### Docs + examples

- [x] Publish plugin authoring guide (stdout/stderr rules, example frames, troubleshooting)
- [x] Add copy/paste example plugins (bash + python) for `config.mutate`, `launch.plan`, `commands.list`

### Moments parity spike (after runner skeleton works)

- [x] Build Moments plugin set to reproduce `scripts/startdev.sh` via phases (config/build/prepare/validate/launch/logs)
- [x] Testing: add fixture plugin long-running-stream (logs.follow + cancellation)
- [x] Testing: add fixture plugin validate-passfail (env toggles validate.run pass/fail)
- [x] Testing: add fixture plugin launch-fail (service exits immediately / health fails)
- [x] Testing: add Go test app http-echo (health endpoint + periodic logs)
- [x] Testing: add Go test app crash-after (exits non-zero after delay)
- [x] Testing: add Go test app log-spewer (stdout/stderr at controlled rate)
- [ ] Testing: add Go test app slow-start (bind after delay; readiness tests)
- [ ] Testing: add Go test app hang (never ready; timeout tests)
- [x] Testing: implement devctl dev smoketest e2e (build testapps; up/status/logs/down)
- [x] Testing: implement devctl dev smoketest logs (follow; assert lines; cancel)
- [x] Testing: implement devctl dev smoketest failures (validate fail, launch fail, plugin timeout)
- [x] Testing: add runtime tests for request cancellation + plugin timeout
- [x] Testing: add supervisor tests for readiness timeout and post-ready crash handling
- [x] Testing: add CI targets to run go test + smoketests (fast vs slow split)
