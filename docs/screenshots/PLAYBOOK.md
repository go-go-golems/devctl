# Screenshot Playbook

This playbook documents how to capture and update devctl TUI screenshots.

## Goal

Produce consistent, colored PNG screenshots of the devctl TUI for README usage.

## Prerequisites

- [VHS](https://github.com/charmbracelet/vhs) installed
- A `devctl` binary in `PATH` (or set `PATH="/tmp:$PATH"` after building)
- A repo with `.devctl.yaml` configured at `/tmp/devctl-demo-repo`

Install VHS:

```bash
brew install vhs
# or see https://github.com/charmbracelet/vhs#installation
```

## Capture Screenshots (recommended flow)

Run from the devctl repo root:

```bash
# Build devctl
go build -o /tmp/devctl ./cmd/devctl

# Ensure demo repo exists with a valid .devctl.yaml
ls /tmp/devctl-demo-repo/.devctl.yaml

# Run VHS to capture screenshots
cd vhs
PATH="/tmp:$PATH" vhs screenshot-tui.tape
```

Outputs:

- `docs/screenshots/devctl-tui-dashboard.png`
- `docs/screenshots/devctl-tui-pipeline.png`
- `docs/screenshots/devctl-tui-plugins.png`

## VHS tape file

The tape file is at `vhs/screenshot-tui.tape`. It:

1. Starts the TUI with `devctl tui --alt-screen=false`
2. Presses `u` to start services
3. Screenshots the dashboard view
4. Tabs through to Pipeline and Plugins views, taking screenshots
5. Stops services with `d` and quits

## Setting up a demo repo

If `/tmp/devctl-demo-repo` doesn't exist:

```bash
mkdir -p /tmp/devctl-demo-repo
cd /tmp/devctl-demo-repo

# Create .devctl.yaml
cat > .devctl.yaml << 'EOF'
plugins:
  - id: demo
    path: python3
    args:
      - ./devctl-plugin.py
    priority: 10
EOF

# Create a demo plugin
cat > devctl-plugin.py << 'EOF'
#!/usr/bin/env python3
import json
import sys

def emit(obj):
    sys.stdout.write(json.dumps(obj) + "\n")
    sys.stdout.flush()

emit({
    "type": "handshake",
    "protocol_version": "v2",
    "plugin_name": "demo",
    "capabilities": {"ops": ["config.mutate", "validate.run", "launch.plan"]},
})

for line in sys.stdin:
    line = line.strip()
    if not line:
        continue
    req = json.loads(line)
    rid = req.get("request_id", "")
    op = req.get("op", "")

    if op == "config.mutate":
        emit({"type": "response", "request_id": rid, "ok": True,
              "output": {"config_patch": {"set": {"services.api.port": 8080}, "unset": []}}})
    elif op == "validate.run":
        emit({"type": "response", "request_id": rid, "ok": True,
              "output": {"valid": True, "errors": [], "warnings": []}})
    elif op == "launch.plan":
        emit({"type": "response", "request_id": rid, "ok": True,
              "output": {"services": [
                  {"name": "api", "command": ["bash", "-lc", "python3 -m http.server 8080"]},
                  {"name": "web", "command": ["bash", "-lc", "python3 -m http.server 3000"]}
              ]}})
    else:
        emit({"type": "response", "request_id": rid, "ok": False,
              "error": {"code": "E_UNSUPPORTED", "message": f"unsupported op: {op}"}})
EOF
chmod +x devctl-plugin.py
```

## Customizing

Edit `vhs/screenshot-tui.tape` to:

- Change timing with `Sleep` commands
- Adjust window size with `Set Width` and `Set Height` (pixel dimensions)
- Change theme with `Set Theme` (try "GitHub Dark", "Tokyo Night", "Dracula")
- Take additional screenshots at different points

## Alternative: tmux + ANSI capture

For headless/CI environments without VHS, see the legacy scripts:

- `ttmp/.../scripts/01-capture-tui-screens.sh` (tmux capture)
- `ttmp/.../scripts/02-ansi-to-png.py` (ANSI to PNG rendering)

## Validation

- Open the PNGs to confirm readability and color correctness
- Ensure the README references the correct paths
- Run `devctl down --repo-root /tmp/devctl-demo-repo` if captures were interrupted
