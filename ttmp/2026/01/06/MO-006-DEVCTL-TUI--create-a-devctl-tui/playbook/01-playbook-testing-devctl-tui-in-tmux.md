---
Title: 'Playbook: Testing devctl tui in tmux'
Ticket: MO-006-DEVCTL-TUI
Status: active
Topics:
    - backend
    - ui-components
DocType: playbook
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: ""
LastUpdated: 2026-01-06T17:19:12.538729231-05:00
WhatFor: ""
WhenToUse: ""
---

# Playbook: Testing devctl tui in tmux

## Purpose

Give a repeatable way to test the new `devctl tui` command against a *realistic* (but small and self-contained) dev environment, inside `tmux`, so we can:
- confirm the UI starts and quits cleanly,
- confirm it reads `.devctl/state.json` and reflects running services,
- confirm event plumbing works (state snapshots → UI messages → Bubble Tea models),
- capture the screen output for quick review (via `tmux capture-pane`).

## Environment Assumptions

Required:
- `tmux` available (this repo’s agent workflow assumes tmux for TUI testing)
- `python3` available (fixture plugins are Python)
- Go toolchain available for building/running `devctl` and fixture test apps

Nice-to-have:
- A terminal that supports Unicode box drawing (not required for the current minimal UI)

## Commands

### 1) Create a sensible fixture repo-root (E2E plugin + 2 services)

This fixture mirrors what a “real” repo looks like:
- `.devctl.yaml` config
- a plugin that implements `config.mutate`, `validate.run`, `build.run`, `prepare.run`, `launch.plan`
- supervised services:
  - `http` (health endpoint)
  - `spewer` (writes logs continuously)

It is copied from the existing smoke test logic in `devctl/cmd/devctl/cmds/dev/smoketest/e2e.go`, and is also available as a ticket-local setup script so you don’t have to copy/paste this every time.

Preferred (script):

```bash
cd /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl
REPO_ROOT="$(./ttmp/2026/01/06/MO-006-DEVCTL-TUI--create-a-devctl-tui/scripts/setup-fixture-repo-root.sh)"
echo "REPO_ROOT=$REPO_ROOT"
```

Manual (inline steps, equivalent):

```bash
set -euo pipefail

cd /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl

# Create a temp repo-root for devctl to treat as the “target repo”
REPO_ROOT="$(mktemp -d -t devctl-tui-fixture-XXXXXX)"
echo "REPO_ROOT=$REPO_ROOT"

# Build tiny fixture services into $REPO_ROOT/bin
mkdir -p "$REPO_ROOT/bin"
GOWORK=off go build -o "$REPO_ROOT/bin/http-echo"   ./testapps/cmd/http-echo
GOWORK=off go build -o "$REPO_ROOT/bin/log-spewer"  ./testapps/cmd/log-spewer

# Pick a free port (mac/linux compatible)
PORT="$(python3 - <<'PY'
import socket
s=socket.socket()
s.bind(("127.0.0.1",0))
print(s.getsockname()[1])
s.close()
PY
)"

PLUGIN="$(pwd)/testdata/plugins/e2e/plugin.py"

cat > "$REPO_ROOT/.devctl.yaml" <<YAML
plugins:
  - id: e2e
    path: python3
    args:
      - "$PLUGIN"
    env:
      DEVCTL_HTTP_ECHO_BIN: "$REPO_ROOT/bin/http-echo"
      DEVCTL_LOG_SPEWER_BIN: "$REPO_ROOT/bin/log-spewer"
      DEVCTL_HTTP_ECHO_PORT: "$PORT"
    priority: 10
YAML

echo "Fixture ready: $REPO_ROOT"

# Optional sanity checks (should show one plugin with ops)
go run ./cmd/devctl --repo-root "$REPO_ROOT" plugins list
```

### 2) Start the environment and persist state

This should create:
- `$REPO_ROOT/.devctl/state.json`
- `$REPO_ROOT/.devctl/logs/*.log`

```bash
cd /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl
go run ./cmd/devctl --repo-root "$REPO_ROOT" up --timeout 30s

# Verify state exists and is readable
ls -la "$REPO_ROOT/.devctl/state.json"
go run ./cmd/devctl --repo-root "$REPO_ROOT" status
```

Optional (new): you can now start from a stopped state and press `u` inside the TUI to run `up` in-process.

### 3) Run the TUI inside tmux (and capture output)

```bash
cd /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl

# Start a dedicated tmux session
tmux new-session -d -s devctl-tui "go run ./cmd/devctl --repo-root \"$REPO_ROOT\" tui --refresh 1s"

# Give it a moment to render
sleep 1

# Capture the screen so you can review output without “watching live”
tmux capture-pane -t devctl-tui -p -S -200 > /tmp/devctl-tui-capture.txt
echo "Captured to /tmp/devctl-tui-capture.txt"

# Optional: attach interactively
tmux attach -t devctl-tui

# Inside the TUI:
# - use `↑/↓` to select a service
# - press `enter` (or `l`) to open the service log view
#   - if a service is dead, the header shows exit_code/signal and a small stderr tail excerpt
#   - the dashboard row also shows a compact hint like `dead (exit=2)` or `dead (sig=KILL)`
# - press `tab` to switch stdout/stderr within the service view
# - press `f` to toggle follow
# - press `/` to filter log lines (type and press enter; `ctrl+l` clears)
# - press `esc` to go back to the dashboard
# - press `x` (then `y`) on the dashboard to SIGTERM the selected service (useful to verify `service exit: ...` events)
# - press `d` (then `y`) to run `down` in-process (stops services + removes state)
# - press `u` to run `up` in-process (starts services + writes state)
# - press `r` (then `y`) to run `restart` in-process (down then up)
# - press `tab` from the dashboard to switch to the event view
#   - press `tab` again to reach the pipeline view (pipeline phases + validation issues + step results)
#     - in the pipeline view: `b` focuses build, `p` focuses prepare, `v` focuses validation, `↑/↓` selects, `enter` toggles details
# - in the event view, press `/` to filter and `c` to clear the event log
# - press `?` to toggle the help overlay
# - press `q` to quit
```

Note: `tmux capture-pane` is useful for quick “does it render” checks, but because the TUI is constantly re-rendering, the captured output can look like a mix of multiple frames. For reliable verification, attach interactively and rely on what you see live.

### 4) Cleanup

```bash
cd /home/manuel/workspaces/2026-01-06/moments-dev-tool/devctl

# Stop supervised processes
go run ./cmd/devctl --repo-root "$REPO_ROOT" down --timeout 10s || true

# Kill tmux session if still running
tmux kill-session -t devctl-tui 2>/dev/null || true

# Remove fixture repo
rm -rf "$REPO_ROOT"
```

## Exit Criteria

Minimum success:
- `devctl up` prints `ok` and leaves services running
- `.devctl/state.json` exists under `$REPO_ROOT/.devctl/`
- `devctl tui` starts and shows:
  - `System: Running`
  - a `Services` list with `http` and `spewer` and their PIDs
- `enter` opens a service detail view and shows log output (or a clear error if logs are missing)
- `tab` switches to the event view and shows lines like `state: loaded` (and optionally `service exit: ...` if you kill a PID)
- `q` exits the TUI without hanging
- the TUI output is not polluted by router debug logs (e.g., no `[watermill] ...` lines)
- the TUI uses the terminal alternate screen (quitting returns you to the original shell screen)

Bonus success:
- `devctl logs --service spewer --follow` shows output while TUI is running (proves supervision + logs are alive)

## Notes

- If you see `System: Stopped (no state)` in the TUI, it usually means you forgot to run `devctl up` (or pointed `--repo-root` at the wrong directory).
- If `devctl up` fails validation, re-check that `python3` is present and the fixture binaries exist at the paths written into `.devctl.yaml`.
- The current TUI is deliberately minimal; it’s meant to validate the event/message spine first. UI polish will come in later milestones.
