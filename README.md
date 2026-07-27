# devctl — Dev Environment Orchestrator

**devctl** makes starting your development environment as simple as `devctl up`.

Instead of remembering which services to start, what order they need, and what environment variables to set, you write a small script (a "plugin") that describes your environment — and devctl handles the rest.

<p align="center">
  <img src="docs/screenshots/devctl-tui-dashboard.png" alt="devctl TUI dashboard" width="800">
</p>

## The Problem

Starting a development environment often looks like this:

```bash
# Terminal 1: Start the database
docker-compose up -d postgres
sleep 5  # wait for it...

# Terminal 2: Run migrations
cd backend && make migrate

# Terminal 3: Start the API
export DATABASE_URL=postgres://...
go run ./cmd/api

# Terminal 4: Start the frontend  
cd frontend && npm run dev

# Oh, and don't forget to set these env vars...
# And restart the API if you change config...
```

This knowledge lives in your head, in scattered README files, or in tribal knowledge. New team members struggle. You forget steps after a vacation.

## The Solution

With devctl, you write a **plugin** — a simple script that answers three questions:

1. **What configuration does my environment need?** (ports, env vars, etc.)
2. **Is everything ready?** (dependencies installed, database running, etc.)
3. **What services should run?** (API server, frontend, workers, etc.)

Then starting your environment becomes:

```bash
devctl up      # Start everything
devctl status  # See what's running
devctl down    # Stop everything
```

## What's a Plugin?

A plugin is just a script (Python, Bash, Node — any language) that communicates with devctl via JSON messages. It's not a framework or SDK — it's a simple protocol.

Here's the mental model:

```
┌─────────────────────────────────────────────────────────────────┐
│  Your Repository                                                 │
│  ┌──────────────────┐    ┌──────────────────────────────────┐   │
│  │  .devctl.yaml    │    │  devctl-plugin.py (or .sh, .js)  │   │
│  │  ───────────     │    │  ────────────────                │   │
│  │  Points to your  │───▶│  Answers questions about your   │   │
│  │  plugin script   │    │  environment (config, services)  │   │
│  └──────────────────┘    └──────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
                                      │
                                      ▼
┌─────────────────────────────────────────────────────────────────┐
│  devctl                                                          │
│  ───────                                                         │
│  • Asks your plugin what to run                                  │
│  • Starts services in the right order                            │
│  • Manages logs, health checks, restarts                         │
│  • Provides TUI dashboard and CLI                                │
└─────────────────────────────────────────────────────────────────┘
```

**The plugin describes WHAT to run. devctl handles HOW to run it.**

## Quick Start

### 1. Install devctl

```bash
# macOS
brew tap go-go-golems/go-go-go
brew install go-go-golems/go-go-go/devctl

# Or with Go
go install github.com/go-go-golems/devctl/cmd/devctl@latest
```

(See [Installation](#installation) for more options.)

### 2. Create a config file

In your project root, create `.devctl.yaml`:

```yaml
plugins:
  - id: myproject
    path: python3
    args: [./devctl-plugin.py]
    priority: 10
```

This tells devctl: "Run my plugin using `python3 ./devctl-plugin.py`".

### 3. Write your plugin

Create `devctl-plugin.py`:

```python
#!/usr/bin/env python3
"""
A minimal devctl plugin that starts a simple HTTP server.
Plugins communicate via JSON over stdin/stdout.
"""
import json
import sys

def respond(obj):
    """Send a JSON response to devctl."""
    print(json.dumps(obj), flush=True)

# Step 1: Handshake - tell devctl what we can do
respond({
    "type": "handshake",
    "protocol_version": "v2",
    "plugin_name": "myproject",
    "capabilities": {
        "ops": ["config.mutate", "validate.run", "launch.plan"]
    },
})

# Step 2: Handle requests from devctl
for line in sys.stdin:
    if not line.strip():
        continue
    
    request = json.loads(line)
    request_id = request.get("request_id", "")
    operation = request.get("op", "")

    if operation == "config.mutate":
        # Tell devctl about configuration (e.g., which port to use)
        respond({
            "type": "response",
            "request_id": request_id,
            "ok": True,
            "output": {
                "config_patch": {
                    "set": {"services.api.port": 8080},
                    "unset": []
                }
            }
        })

    elif operation == "validate.run":
        # Check if prerequisites are met (e.g., dependencies installed)
        respond({
            "type": "response",
            "request_id": request_id,
            "ok": True,
            "output": {"valid": True, "errors": [], "warnings": []}
        })

    elif operation == "launch.plan":
        # Tell devctl what services to start
        respond({
            "type": "response",
            "request_id": request_id,
            "ok": True,
            "output": {
                "services": [
                    {
                        "name": "api",
                        "command": ["python3", "-m", "http.server", "8080"]
                    }
                ]
            }
        })

    else:
        respond({
            "type": "response",
            "request_id": request_id,
            "ok": False,
            "error": {"code": "E_UNSUPPORTED", "message": f"Unknown: {operation}"}
        })
```

Make it executable:

```bash
chmod +x devctl-plugin.py
```

### 4. `.devctl.yaml` schema reference

```yaml
# Optional profile selection. Can be overridden with --profile.
profile:
  active: development

# Optional named profiles. A profile selects plugin IDs from top-level plugins.
profiles:
  development:
    display_name: Development
    description: Hot reload and local debug settings
    plugins: [myproject]
    env:
      LOG_LEVEL: debug

plugins:
  - id: <string>              # Required. Unique identifier for this plugin.
    path: <string>            # Required. Executable to run the plugin (e.g. python3, bash, go run).
    args: [<string>]          # Optional. Arguments passed to path.
    env: {<string>: <string>} # Optional. Extra env vars for the plugin process.
    priority: <int>           # Optional. Lower runs first (default: 0).

# Optional strictness mode for multiple plugins:
# strictness: warn   # or "error"
```

A local `.devctl.override.yaml`, if present, is merged over `.devctl.yaml`. Use it for personal profiles or local profile adjustments that should not change the shared project config.

A profile named `default` is allowed but not implicit. It is used only when selected with `profile.active: default` or `--profile default`. If no profile is selected, devctl loads all top-level plugins for backward compatibility.

### 5. Use devctl

```bash
# Verify your plugin loads correctly
devctl plugins list

# See what would run (without actually starting services)
devctl plan

# Run one phase directly when you need to iterate or use a longer timeout
devctl build --timeout 10m

# Start your environment
devctl up

# Check status
devctl status

# View logs
devctl logs api --follow

# Stop everything
devctl down
```

That's it! Your development environment is now codified and repeatable.

## Features

### CLI Commands

| Command | What it does |
|---------|--------------|
| `devctl plugins list` | Show loaded plugins |
| `devctl plan` | Preview merged config and launch plan without starting services |
| `devctl build` | Run `config.mutate + build.run` and print build steps/artifacts |
| `devctl prepare` | Run `config.mutate + prepare.run` and print prepare steps/artifacts |
| `devctl validate` | Run `config.mutate + validate.run`; exits non-zero when invalid |
| `devctl up` | Start all selected services |
| `devctl status` | Show running services |
| `devctl logs NAME` | View service logs |
| `devctl restart NAME` | Restart one supervised service |
| `devctl down NAME` | Stop one supervised service |
| `devctl start NAME` | Start one stopped/crashed tracked service |
| `devctl profiles list` | List available profiles |
| `devctl profiles active` | Show the resolved active profile |
| `devctl down` | Stop all services |

### Interactive TUI

For a visual dashboard, run:

```bash
devctl tui
```

| Key | Action |
|-----|--------|
| `1` / `2` / `3` | Open Overview, Logs, or Runs |
| `u` / `d` / `r` | Confirm up, down, or restart for the selected scope |
| `Enter` | Open logs for the selected service |
| `p` / `f` / `w` | Pause, follow, or wrap in Logs |
| `:` | Open the command palette |
| `Tab` | Cycle Overview, Logs, and Runs |
| `?` | Help |
| `q` | Quit |

The TUI reads typed durable state and structured run journals. It does not
parse rendered CLI output, and its bounded log buffer strips terminal control
sequences before display.

### Dynamic top-level commands

Plugins can publish commands that appear directly under `devctl`. This
automatic injection is intentional: repository-specific workflows remain
convenient while still using the plugin protocol.

The catalog makes injection deterministic and inspectable:

- built-in static commands always keep their names;
- a dynamic name with multiple providers is recorded as a conflict instead of
  selecting one arbitrarily;
- `devctl plugins commands` lists validated, injectable commands;
- `devctl plugins inspect PLUGIN` shows a provider's catalog entries;
- `devctl plugins refresh` explicitly rebuilds the persistent catalog; and
- `devctl plugins run PLUGIN COMMAND -- [ARGS...]` executes an explicit
  provider when a top-level name is unavailable or ambiguous.

Root help, typo handling, and shell completion use the validated catalog
without starting every configured plugin. At execution time devctl starts only
the selected provider and rejects stale catalog identity before dispatch.

### What Plugins Can Do

Plugins participate in a **pipeline** — a series of phases that run in order:

```
config.mutate → build.run → prepare.run → validate.run → launch.plan → supervise
```

| Phase | Purpose | Example |
|-------|---------|---------|
| `config.mutate` | Set configuration values | Ports, env vars, feature flags |
| `build.run` | Build steps | `npm install`, `go build` |
| `prepare.run` | Pre-launch setup | Run migrations, seed data |
| `validate.run` | Check prerequisites | Is Docker running? Is the DB up? |
| `launch.plan` | Define services | API server, frontend, workers |

You only implement the phases you need. Most simple plugins implement `config.mutate`, `validate.run`, and `launch.plan`.

### Lifecycle of `devctl up`

When you run `devctl up`, here's what actually happens:

1. **Load the selected catalog** — validate configuration and provider identity
2. **config.mutate** — merge config patches from all plugins
3. **build.run** — run named build steps (skipped with `--skip-build`)
4. **prepare.run** — run named prepare steps (skipped with `--skip-prepare`)
5. **validate.run** — check prerequisites; abort if `valid: false`
6. **launch.plan** — collect service definitions
7. **Supervise** — start processes, run health checks, write state

**State detection**: If a previous `devctl up` left state behind, `devctl up` will detect it and prompt:

```bash
$ devctl up
state exists; restart (down then up)? (y/N):
```

Use `--force` to skip the prompt and restart automatically.

### Running individual phases

You can run the expensive or diagnostic pipeline phases without starting services:

```bash
# Build everything declared by build.run. Increase the per-operation timeout for slow builds.
devctl build --timeout 10m

# Ask plugins to run only named build or prepare steps. Plugins decide how to honor names.
devctl build --step backend --step frontend --timeout 10m
devctl prepare --step pnpm-install --timeout 5m

# Check prerequisites and get the same validation JSON that up would use.
devctl validate
```

Each standalone phase command first runs `config.mutate`, then runs the named
phase and emits a Glazed row containing the mutated `config` plus the phase
result (`build`, `prepare`, or `validate`). The default renderer is
human-readable; automation should request `--output json`. For live progress,
plugins should write human-readable progress to **stderr**. Plugin stdout must
remain protocol NDJSON only.

**Shutdown**: `devctl down` sends SIGTERM to each service process group, waits up to 3s, then sends SIGKILL if needed. State is removed after stop.

## Installation

### Homebrew (macOS/Linux)

```bash
brew tap go-go-golems/go-go-go
brew install go-go-golems/go-go-go/devctl
```

### apt-get (Debian/Ubuntu)

```bash
echo "deb [trusted=yes] https://apt.fury.io/go-go-golems/ /" | sudo tee /etc/apt/sources.list.d/fury.list
sudo apt-get update
sudo apt-get install devctl
```

### yum (RHEL/CentOS/Fedora)

```bash
sudo tee /etc/yum.repos.d/fury.repo <<EOF
[fury]
name=Gemfury Private Repo
baseurl=https://yum.fury.io/go-go-golems/
enabled=1
gpgcheck=0
EOF
sudo yum install devctl
```

### go install

```bash
go install github.com/go-go-golems/devctl/cmd/devctl@latest
```

### Download binaries

Download from [GitHub Releases](https://github.com/go-go-golems/devctl/releases).

### Run from source

```bash
git clone https://github.com/go-go-golems/devctl
cd devctl
go run ./cmd/devctl --help
```

## Common Flags

| Flag | Description |
|------|-------------|
| `--repo-root <path>` | Project root (defaults to current directory) |
| `--config <file>` | Config file (defaults to `.devctl.yaml`) |
| `--profile <name>` | Select active profile (overrides `profile.active`) |
| `--timeout <dur>` | Timeout per operation (default `30s`) |
| `--dry-run` | Show what would happen without doing it |
| `--force` | Stop existing state before starting |
| `--skip-validate` | Skip validate.run |
| `--skip-build` | Skip build.run |
| `--skip-prepare` | Skip prepare.run |
| `--step <name>` | Select a named build/prepare step on `devctl build` or `devctl prepare` (repeatable) |

## Plugin Protocol Details

Plugins communicate via **NDJSON** (newline-delimited JSON) over stdio.

### Rules

1. **stdout** — JSON messages only (one per line)
2. **stderr** — Human-readable messages (for debugging)
3. **First message** — Must be a handshake

### Handshake Example

```json
{
  "type": "handshake",
  "protocol_version": "v2",
  "plugin_name": "myproject",
  "capabilities": {
    "ops": ["config.mutate", "validate.run", "launch.plan"]
  }
}
```

### Learn More

```bash
devctl help plugin-authoring  # Plugin development guide
devctl help profiles-guide    # Profiles and local overrides
devctl help user-guide        # Full user guide
devctl help tui-guide         # TUI reference
```

Or see `docs/plugin-authoring.md` for extended examples.

## State and Logs

devctl stores state and logs under `.devctl/` in your repo:

```
.devctl/
├── state.json              # Versioned environment index
└── runs/
    └── <run-id>/
        ├── run.json        # Durable typed service-attempt state
        ├── owner.json      # Wrapper process identity
        ├── ready.json      # Child identity and readiness handshake
        ├── stdout.log      # Raw stdout
        ├── stderr.log      # Raw stderr
        ├── logs.jsonl      # Sequenced structured journal
        └── exit.json       # Terminal result
```

- **Logs**: Each service attempt has raw streams and a structured journal.
- **State**: schema-v2 state tracks desired state and current/last run IDs.
- **Retention**: `down` preserves completed run directories for diagnosis.
- **Upgrade**: stop old environments with the old binary before upgrading;
  see `devctl help v2-upgrade`.

**Tip:** Add `.devctl/` to `.gitignore`.

## Shell Completion

```bash
# Bash
source <(devctl completion bash)

# Zsh  
devctl completion zsh > ~/.zfunc/_devctl

# Fish
devctl completion fish > ~/.config/fish/completions/devctl.fish
```

## Development

```bash
go build ./...           # Build
go test ./...            # Test
golangci-lint run        # Lint
```

## License

MIT
