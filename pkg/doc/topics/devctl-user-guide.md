---
Title: devctl User Guide (CLI + TUI + Plugins)
Slug: devctl-user-guide
Short: "A practical, end-to-end guide to using devctl: from your first .devctl.yaml to the TUI and real plugins."
Topics:
  - devctl
  - dev-environment
  - cli
  - tui
  - plugins
  - scripting
  - process-supervision
IsTemplate: false
IsTopLevel: true
ShowPerDefault: true
SectionType: GeneralTopic
---

# devctl User Guide

## What devctl is (and why you'd want it)

Every repo has "how we run this locally" knowledge. Over time, this knowledge accumulates as scattered scripts, undocumented flags, and tribal knowledge that only a few people really understand. Onboarding new developers becomes slow. CI diverges from local dev. The startup script grows into something fragile.

devctl exists to solve this problem. It's a **dev environment orchestrator** that lets you capture "how we run this repo" in a testable, versionable plugin—while devctl itself handles the boring-but-hard parts:

- **Ordering**: run build steps before prepare steps before launching services
- **Process supervision**: start services, track PIDs, capture stdout/stderr
- **State management**: know what's running, stop it cleanly, resume later
- **Consistency**: same commands work across repos, machines, and CI

The core idea: **your plugin knows your repo; devctl knows how to run things reliably**.

## The pipeline: how devctl works

When you run `devctl up`, devctl executes a pipeline of phases. Each phase can be implemented by one or more plugins, and devctl merges their outputs.

```
┌──────────────────────────────────────────────────────────────────────┐
│                         devctl up pipeline                           │
├──────────────────────────────────────────────────────────────────────┤
│                                                                      │
│   ┌─────────────┐    ┌─────────────┐    ┌─────────────┐             │
│   │  config     │───▶│   build     │───▶│  prepare    │             │
│   │  .mutate    │    │   .run      │    │   .run      │             │
│   └─────────────┘    └─────────────┘    └─────────────┘             │
│         │                  │                  │                      │
│         │    Derive env    │   Compile code   │  Install deps        │
│         │    vars, ports   │   bundle assets  │  run migrations      │
│         ▼                  ▼                  ▼                      │
│   ┌─────────────┐    ┌─────────────┐    ┌─────────────┐             │
│   │  validate   │───▶│   launch    │───▶│  supervise  │             │
│   │   .run      │    │   .plan     │    │  (devctl)   │             │
│   └─────────────┘    └─────────────┘    └─────────────┘             │
│         │                  │                  │                      │
│    Check prereqs     Return service      Start processes,           │
│    (docker? deps?)   definitions         capture logs, track PIDs   │
│                                                                      │
└──────────────────────────────────────────────────────────────────────┘
```

**Key insight**: your plugin computes *what* to do (config, build steps, services). devctl handles *how* to run it (processes, state, logs). This separation keeps plugins simple and testable.

## Quick start: 5 minutes to a working dev environment

Let's make this concrete. Say you have a repo with a backend API and a frontend dev server.

### 1. Create `.devctl.yaml` at your repo root

```yaml
plugins:
  - id: myrepo
    path: python3
    args: ["./devctl-plugin.py"]
    priority: 10
```

### 2. Create `devctl-plugin.py`

```python
#!/usr/bin/env python3
import json, sys

def emit(obj):
    sys.stdout.write(json.dumps(obj) + "\n")
    sys.stdout.flush()

# Handshake: tell devctl who we are and what we support
emit({
    "type": "handshake",
    "protocol_version": "v2",
    "plugin_name": "myrepo",
    "capabilities": {"ops": ["config.mutate", "validate.run", "launch.plan"]},
})

# Handle requests from devctl
for line in sys.stdin:
    if not line.strip():
        continue
    req = json.loads(line)
    rid, op = req.get("request_id", ""), req.get("op", "")

    if op == "config.mutate":
        # Derive config values (ports, URLs, env vars)
        emit({"type": "response", "request_id": rid, "ok": True,
              "output": {"config_patch": {
                  "set": {"env.API_PORT": "8080", "env.VITE_API_URL": "http://localhost:8080"},
                  "unset": []
              }}})

    elif op == "validate.run":
        # Check prerequisites
        import shutil
        errors = []
        if not shutil.which("node"):
            errors.append({"code": "E_MISSING", "message": "node not found. Install: brew install node"})
        emit({"type": "response", "request_id": rid, "ok": True,
              "output": {"valid": len(errors) == 0, "errors": errors, "warnings": []}})

    elif op == "launch.plan":
        # Define services for devctl to supervise
        emit({"type": "response", "request_id": rid, "ok": True,
              "output": {"services": [
                  {"name": "api", "cwd": "backend", "command": ["go", "run", "."],
                   "env": {"PORT": "8080"},
                   "health": {"type": "http", "url": "http://localhost:8080/health", "timeout_ms": 30000}},
                  {"name": "web", "cwd": "frontend", "command": ["npm", "run", "dev"],
                   "env": {"VITE_API_URL": "http://localhost:8080"}}
              ]}})

    else:
        emit({"type": "response", "request_id": rid, "ok": False,
              "error": {"code": "E_UNSUPPORTED", "message": f"unsupported op: {op}"}})
```

Make it executable: `chmod +x devctl-plugin.py`

### 3. Run the core loop

```bash
devctl plugins list   # Verify plugin loads correctly
devctl plan           # See what would run (no side effects)
devctl up             # Start everything
devctl status         # What's running?
devctl logs --service api --follow   # Tail logs
devctl down           # Stop everything
```

That's it. You now have a dev environment that anyone can start with `devctl up`.

## The CLI: your daily workflow

devctl commands are designed for a simple, repeatable workflow: **plan → up → observe → down**.

### Inspect before running

```bash
devctl plugins list   # What plugins are configured?
devctl plan           # What config and services would be created?
```

### Start and observe

```bash
devctl up                          # Run pipeline, start services
devctl status                      # Show running services, PIDs, health
devctl status --tail-lines 10      # Include stderr tails for dead services
devctl logs --service api          # Show stdout for a service
devctl logs --service api --stderr # Show stderr
devctl logs --service api --follow # Live tail
```

### Stop and cleanup

```bash
devctl down   # Stop all services, remove state
```

### Common flags you'll use (command-local)

devctl has a small set of “repo context” flags that apply to most verbs. These are **command-local** flags, which means they appear after the verb:

```bash
devctl status --repo-root /path/to/repo
devctl plan --repo-root /path/to/repo --timeout 10s
```

| Flag | Purpose |
|------|---------|
| `--repo-root <path>` | Override the repo root (default: cwd) |
| `--config <file>` | Override config file (default: `.devctl.yaml`) |
| `--timeout <dur>` | Per-operation timeout (default: 30s) |
| `--dry-run` | Skip side effects; plugins see `ctx.dry_run=true` |
| `--strict` | Error on service/config collisions instead of "last wins" |

## The TUI: an always-on dashboard

The TUI gives you a persistent, interactive view of your dev environment. Start it with:

```bash
devctl tui
```

### Navigation

| Key | Action |
|-----|--------|
| `Tab` | Switch views: Dashboard → Events → Pipeline → Plugins |
| `?` | Toggle help overlay |
| `q` | Quit |

### Dashboard view (where you'll spend most time)

| Key | Action |
|-----|--------|
| `j/k` or `↑/↓` | Select service |
| `l` or `Enter` | Open service logs |
| `u` | Start (or restart if already running) |
| `d` | Stop (with confirmation) |
| `r` | Restart (with confirmation) |
| `x` | Kill selected service (with confirmation) |

### Service view (logs)

| Key | Action |
|-----|--------|
| `Tab` | Toggle stdout/stderr |
| `f` | Toggle follow mode |
| `/` | Filter logs |
| `Esc` | Back to dashboard |

For the full TUI reference, see `glaze help devctl-tui-guide`.

## Writing plugins: from shell script to devctl

If you have an existing setup script, converting it to a devctl plugin follows a pattern:

| Your script does... | devctl phase | Plugin returns |
|---------------------|--------------|----------------|
| Sets environment variables | `config.mutate` | A patch with dotted keys |
| Checks if docker is running | `validate.run` | Errors/warnings |
| Runs `npm install` | `prepare.run` | Named steps |
| Runs `go build` | `build.run` | Named steps, artifacts |
| Starts processes | `launch.plan` | Service definitions |

The key shift: **don't start processes in your plugin**. Return a service definition and let devctl handle process management.

### Real-world example: a typical web app

Here's what a plugin for a "backend + frontend + database" repo might look like:

```python
# launch.plan response
"services": [
    {
        "name": "postgres",
        "command": ["docker", "compose", "up", "postgres"],
        "health": {"type": "tcp", "address": "localhost:5432", "timeout_ms": 30000}
    },
    {
        "name": "api",
        "cwd": "backend",
        "command": ["go", "run", "./cmd/server"],
        "env": {"DATABASE_URL": "postgres://localhost:5432/dev"},
        "health": {"type": "http", "url": "http://localhost:8080/health"}
    },
    {
        "name": "web",
        "cwd": "frontend",
        "command": ["npm", "run", "dev"],
        "env": {"VITE_API_URL": "http://localhost:8080"}
    }
]
```

For complete plugin authoring guidance, see `glaze help devctl-plugin-authoring`.

## Where devctl stores things

devctl writes to `.devctl/` in your repo root:

```
.devctl/
├── state.json              # What's running (PIDs, start times)
└── logs/
    ├── api.stdout.log      # Service stdout
    ├── api.stderr.log      # Service stderr
    ├── api.ready           # Ready file (wrapper mode)
    └── api.exit.json       # Exit info (wrapper mode)
```

You can safely `rm -rf .devctl/` to reset state. Add `.devctl/` to `.gitignore`.

## Troubleshooting

### "No plugins configured"

Your `.devctl.yaml` isn't being found. Check `--repo-root`:

```bash
devctl plugins list --repo-root /path/to/repo
```

### Plugin fails with "stdout contamination"

Your plugin is printing non-JSON to stdout. Move all logging to stderr:

```python
# Wrong
print("Starting up...")

# Right
import sys
print("Starting up...", file=sys.stderr)
```

### "Unknown service" when using `logs`

The service name doesn't match what's in state. Check `status` first:

```bash
devctl status   # See actual service names
devctl logs --service <name-from-status>
```

### "Read state: no such file"

No state exists—either `up` hasn't run or `down` already cleaned up. Run `up` first:

```bash
devctl up
devctl status
```

### Timeout errors

A plugin is blocking too long. Debug by reducing scope:

```bash
devctl plugins list --timeout 5s   # Does handshake work?
devctl plan --timeout 5s           # Does planning work?
```

## Next steps

| Want to... | Read |
|------------|------|
| Write your first plugin | `glaze help devctl-scripting-guide` |
| Understand the full protocol | `glaze help devctl-plugin-authoring` |
| Learn all TUI features | `glaze help devctl-tui-guide` |
