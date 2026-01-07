---
Title: 'Comprehensive fixture: devctl up failure (bug report)'
Ticket: MO-010-DEVCTL-CLEANUP-PASS
Status: active
Topics:
    - backend
    - tui
    - refactor
    - ui-components
DocType: analysis
Intent: long-term
Owners: []
RelatedFiles:
    - Path: devctl/cmd/devctl/cmds/dynamic_commands.go
      Note: |-
        Unconditionally calls commands.list (3s timeout) for each plugin; no capability gating
        Unconditionally calls commands.list with 3s timeout per plugin
    - Path: devctl/cmd/devctl/cmds/wrap_service.go
      Note: |-
        Wrapper writes ready file after child.Start(); never reached if wrapper is delayed before Cobra dispatch
        Wrapper writes ready file after child.Start(); never reached if wrapper delayed
    - Path: devctl/cmd/devctl/main.go
      Note: |-
        Always runs dynamic plugin command discovery before executing any command (including __wrap-service)
        Runs AddDynamicPluginCommands before executing any command
    - Path: devctl/pkg/supervise/supervisor.go
      Note: |-
        Wrapper-mode ready file wait uses a hard-coded 2s deadline; error is “wrapper did not report child start”
        Wrapper-mode ready-file deadline is hard-coded to 2s
    - Path: devctl/ttmp/2026/01/06/MO-009-TUI-COMPLETE-FEATURES--complete-tui-features-per-mo-006-design/scripts/setup-comprehensive-fixture.sh
      Note: |-
        Creates the fixture repo with a logger plugin that never answers commands.list
        Creates the fixture; includes logger plugin that blocks on commands.list
ExternalSources: []
Summary: 'Repro: running `devctl up` on the comprehensive fixture fails with `wrapper did not report child start` due to dynamic command discovery delaying `__wrap-service` beyond the supervisor’s 2s ready-file deadline.'
LastUpdated: 2026-01-07T13:47:37.167709934-05:00
WhatFor: Provide a copy/paste repro and root-cause analysis for the comprehensive fixture startup failure.
WhenToUse: When fixing supervise wrapper startup timing, dynamic command discovery, or fixture stability.
---


# Comprehensive fixture: `devctl up` fails with “wrapper did not report child start”

## 1) Summary

The comprehensive fixture produced by `setup-comprehensive-fixture.sh` reliably fails during the supervise phase of `devctl up` with:

```
Error: wrapper did not report child start
```

Root cause (high confidence):
- The supervisor runs services in wrapper mode by launching the `devctl` binary with the hidden command `__wrap-service`.
- The `devctl` binary **always** executes dynamic plugin command discovery before executing any command (`main.go` calls `AddDynamicPluginCommands` unconditionally).
- Dynamic plugin command discovery **always** calls `commands.list` (3s timeout) for every plugin, even if the plugin does not support it (`dynamic_commands.go`).
- The comprehensive fixture includes a `logger` plugin that declares no ops and consumes stdin forever (never responds). That makes `commands.list` block until the 3s timeout.
- The supervisor’s wrapper-mode “ready file exists” deadline is **hard-coded to 2 seconds**, so the wrapper can be killed before it even reaches `__wrap-service` and writes the ready file.

## 2) Reproduction (copy/paste)

Run from the `devctl` module root (directory containing `devctl/go.mod`):

```bash
cd /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl
REPO_ROOT=$(./ttmp/2026/01/06/MO-009-TUI-COMPLETE-FEATURES--complete-tui-features-per-mo-006-design/scripts/setup-comprehensive-fixture.sh)
go run ./cmd/devctl --repo-root "$REPO_ROOT" up
```

Observed fixture path during this repro:
- `REPO_ROOT=/tmp/devctl-comprehensive-iiof7p`

## 3) Observed output (verbatim)

Command:

```bash
go run ./cmd/devctl --repo-root "/tmp/devctl-comprehensive-iiof7p" up
```

Output:

```text
2026-01-07T13:53:15.1906292-05:00 INF [deps] Checking go.mod... plugin=comprehensive
2026-01-07T13:53:15.69070609-05:00 INF [deps] Downloading dependencies... plugin=comprehensive
2026-01-07T13:53:16.19080074-05:00 INF [deps] Verifying checksums... plugin=comprehensive
2026-01-07T13:53:16.690928399-05:00 INF [backend] Compiling pkg/api... plugin=comprehensive
2026-01-07T13:53:17.191023863-05:00 INF [backend] Compiling pkg/handlers... plugin=comprehensive
2026-01-07T13:53:17.691125733-05:00 INF [backend] Compiling cmd/backend... plugin=comprehensive
2026-01-07T13:53:18.191227261-05:00 INF [backend] Linking backend binary... plugin=comprehensive
2026-01-07T13:53:18.691342276-05:00 INF [worker] Compiling pkg/jobs... plugin=comprehensive
2026-01-07T13:53:19.191524641-05:00 INF [worker] Compiling cmd/worker... plugin=comprehensive
2026-01-07T13:53:19.691525411-05:00 INF [worker] Linking worker binary... plugin=comprehensive
2026-01-07T13:53:19.941627111-05:00 INF [assets] Copying static assets... plugin=comprehensive
2026-01-07T13:53:20.191699563-05:00 INF [assets] Done. plugin=comprehensive
2026-01-07T13:53:20.692938735-05:00 INF service started pid=330516 service=backend
Error: wrapper did not report child start
...
Error: wrapper did not report child start
exit status 1
```

## 4) Expected vs actual

Expected:
- `devctl up` starts all services in the launch plan, writes `.devctl/state.json`, and returns `ok`.

Actual:
- `devctl up` fails immediately after starting the wrapper process for the first service with `wrapper did not report child start`.
- The fixture repo has `.devctl/logs/` created, but it is empty (no per-service stdout/stderr logs and no `*.ready` file), which strongly suggests the wrapper never reached `wrap_service.go` before being killed.

## 5) Evidence from fixture configuration (why the fixture triggers the bug)

The fixture config includes a `logger` plugin with `ops: []`:

File: `REPO_ROOT/.devctl.yaml`

```yaml
plugins:
  - id: logger
    path: python3
    args:
      - "/tmp/devctl-comprehensive-iiof7p/plugins/logger.py"
    priority: 20
```

The plugin itself never responds to requests:

File: `REPO_ROOT/plugins/logger.py`

```python
emit({
  "type": "handshake",
  "protocol_version": "v2",
  "plugin_name": "logger-plugin",
  "capabilities": {"ops": [], "streams": ["logs.aggregate"]},
})

for line in sys.stdin:
  pass  # Just consume stdin
```

So any unconditional request (like `commands.list`) will block until the client-side timeout.

## 6) Root cause analysis (code-path walkthrough)

### 6.1 Wrapper mode uses the `devctl` binary as the wrapper
`devctl/pkg/supervise/supervisor.go` (wrapper path in `startService`):
- builds args: `WrapperExe __wrap-service --service ... --ready-file ... -- <service cmd>`
- starts the wrapper process
- waits for the ready file with a hard-coded 2s deadline:
  - on timeout: kills the process group and returns `wrapper did not report child start`

### 6.2 The wrapper command is delayed by dynamic plugin command discovery
`devctl/cmd/devctl/main.go`:
- calls `cmds.AddDynamicPluginCommands(rootCmd, os.Args)` before `rootCmd.Execute()`.

`devctl/cmd/devctl/cmds/dynamic_commands.go`:
- discovers plugins from config
- for each plugin:
  - starts the plugin process
  - calls `c.Call(..., "commands.list", ..., timeout=3s)` unconditionally

In the fixture, the `logger` plugin never responds, so wrapper startup pays a ~3 second delay *before* Cobra executes `__wrap-service`.

### 6.3 The timeouts are inverted
- Dynamic command discovery per-plugin timeout: 3s
- Supervisor wrapper ready deadline: 2s

This guarantees the wrapper can miss the ready deadline when any plugin stalls `commands.list`.

## 7) Impact

- Breaks `devctl up` for the comprehensive fixture (and potentially any real repo where a plugin ignores unsupported ops).
- Makes wrapper startup failures hard to diagnose because:
  - wrapper disables logging to its own stderr/stdout,
  - per-service logs are not created if wrapper never reaches `wrap_service.go`,
  - supervisor error message does not include underlying timing context.

## 8) Workarounds (short-term)

- Avoid wrapper mode (not currently exposed as a user flag in `devctl up`; only feasible by code change or alternate invocation).
- Ensure plugins used in the repo respond to `commands.list` quickly (even if empty), or implement capability gating in dynamic discovery so unsupported ops are not invoked.

## 9) Candidate fixes (directional, not yet implemented)

- Bypass dynamic plugin command discovery when running `__wrap-service` (and potentially other internal commands).
- Add capability gating to dynamic command discovery:
  - only call `commands.list` if the plugin declares it (or declares a relevant capability).
- Make wrapper “ready file” deadline configurable and consistent with `Options.ReadyTimeout` (remove the hard-coded 2s).
- Improve diagnostics for wrapper failures (capture wrapper stderr, or write a dedicated wrapper error file).
