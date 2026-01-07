#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
setup-comprehensive-fixture.sh

Creates a comprehensive fixture repo-root for testing ALL devctl TUI features:
  - Multiple services with different behaviors
  - HTTP and TCP health checks
  - Various log levels (DEBUG, INFO, WARN, ERROR)
  - Build pipeline with progress simulation
  - Config patches from plugins
  - Validation warnings
  - Service that crashes (to test exit info display)
  - Multiple plugins (to test plugin list view)

Features Tested:
  - Dashboard: health/CPU/MEM columns, recent events, plugins summary
  - Service Detail: process info, health box, environment display
  - Events View: service filtering, level filtering, event rate, pause
  - Pipeline View: progress bars, live output, config patches
  - Plugins View: expandable plugin cards

Usage:
  ./setup-comprehensive-fixture.sh

Output:
  Prints the created REPO_ROOT path to stdout.

Example:
  REPO_ROOT=$(./setup-comprehensive-fixture.sh)
  go run ./cmd/devctl --repo-root "$REPO_ROOT" up
  go run ./cmd/devctl --repo-root "$REPO_ROOT" tui

Notes:
  - Run from the devctl repo root (where go.mod lives).
  - Cleanup is manual: rm -rf "$REPO_ROOT"
USAGE
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

if [[ ! -f "go.mod" ]]; then
  echo "error: run this from the devctl repo root (expected ./go.mod)" >&2
  exit 2
fi

if ! command -v python3 >/dev/null 2>&1; then
  echo "error: python3 is required for plugins" >&2
  exit 2
fi

# Create fixture directory
REPO_ROOT="$(mktemp -d -t devctl-comprehensive-XXXXXX)"
mkdir -p "$REPO_ROOT/bin" "$REPO_ROOT/plugins"

# Build test binaries
echo "Building test binaries..." >&2
GOWORK=off go build -o "$REPO_ROOT/bin/http-echo" ./testapps/cmd/http-echo
GOWORK=off go build -o "$REPO_ROOT/bin/log-spewer" ./testapps/cmd/log-spewer
GOWORK=off go build -o "$REPO_ROOT/bin/crash-after" ./testapps/cmd/crash-after

# Pick free ports for services
pick_port() {
  python3 -c "import socket; s=socket.socket(); s.bind(('127.0.0.1',0)); print(s.getsockname()[1]); s.close()"
}

PORT_BACKEND=$(pick_port)
PORT_WORKER=$(pick_port)
PORT_FLAKY=$(pick_port)

# Create the comprehensive plugin
cat >"$REPO_ROOT/plugins/comprehensive.py" <<'PYPLUGIN'
#!/usr/bin/env python3
"""
Comprehensive test plugin for devctl TUI testing.

This plugin exercises:
- Config mutation with multiple patches
- Build steps with simulated progress and duration
- Prepare steps
- Validation with warnings (not errors)
- Launch plan with multiple services
"""
import json
import os
import sys
import time

def emit(obj):
    sys.stdout.write(json.dumps(obj) + "\n")
    sys.stdout.flush()

def sleep_emit_progress(step_name, duration_secs, lines_to_emit):
    """Simulate work with progress output."""
    interval = duration_secs / len(lines_to_emit)
    for line in lines_to_emit:
        time.sleep(interval)
        # Emit to stderr as live output
        sys.stderr.write(f"[{step_name}] {line}\n")
        sys.stderr.flush()

# Get environment
HTTP_ECHO_BIN = os.environ.get("DEVCTL_HTTP_ECHO_BIN", "")
LOG_SPEWER_BIN = os.environ.get("DEVCTL_LOG_SPEWER_BIN", "")
CRASH_AFTER_BIN = os.environ.get("DEVCTL_CRASH_AFTER_BIN", "")
PORT_BACKEND = os.environ.get("PORT_BACKEND", "8080")
PORT_WORKER = os.environ.get("PORT_WORKER", "8081")
PORT_FLAKY = os.environ.get("PORT_FLAKY", "8082")

# Handshake
emit({
    "type": "handshake",
    "protocol_version": "v2",
    "plugin_name": "comprehensive-plugin",
    "capabilities": {
        "ops": ["config.mutate", "validate.run", "build.run", "prepare.run", "launch.plan"]
    },
})

for line in sys.stdin:
    line = line.strip()
    if not line:
        continue
    req = json.loads(line)
    rid = req.get("request_id", "")
    op = req.get("op", "")

    if op == "config.mutate":
        # Emit multiple config patches
        emit({
            "type": "response",
            "request_id": rid,
            "ok": True,
            "output": {
                "config_patch": {
                    "set": {
                        "services.backend.port": int(PORT_BACKEND),
                        "services.worker.port": int(PORT_WORKER),
                        "services.flaky.port": int(PORT_FLAKY),
                        "build.cache_enabled": True,
                        "logging.level": "debug",
                    },
                    "unset": ["deprecated.legacy_mode"],
                }
            },
        })

    elif op == "validate.run":
        # Emit warnings (not errors) to test warning display
        emit({
            "type": "response",
            "request_id": rid,
            "ok": True,
            "output": {
                "valid": True,
                "errors": [],
                "warnings": [
                    {"code": "W_DEPRECATED", "message": "Config key 'legacy_mode' is deprecated"},
                    {"code": "W_PORT_CONFLICT", "message": f"Port {PORT_BACKEND} may conflict with common services"},
                ],
            },
        })

    elif op == "build.run":
        # Simulate a multi-step build with duration
        steps = []
        
        # Step 1: Dependencies
        sleep_emit_progress("deps", 1.5, [
            "Checking go.mod...",
            "Downloading dependencies...",
            "Verifying checksums...",
        ])
        steps.append({"name": "dependencies", "ok": True, "duration_ms": 1500})
        
        # Step 2: Backend
        sleep_emit_progress("backend", 2.0, [
            "Compiling pkg/api...",
            "Compiling pkg/handlers...",
            "Compiling cmd/backend...",
            "Linking backend binary...",
        ])
        steps.append({"name": "backend", "ok": True, "duration_ms": 2000})
        
        # Step 3: Worker
        sleep_emit_progress("worker", 1.5, [
            "Compiling pkg/jobs...",
            "Compiling cmd/worker...",
            "Linking worker binary...",
        ])
        steps.append({"name": "worker", "ok": True, "duration_ms": 1500})
        
        # Step 4: Assets
        sleep_emit_progress("assets", 0.5, [
            "Copying static assets...",
            "Done.",
        ])
        steps.append({"name": "assets", "ok": True, "duration_ms": 500})
        
        emit({
            "type": "response",
            "request_id": rid,
            "ok": True,
            "output": {
                "steps": steps,
                "artifacts": {
                    "http-echo": HTTP_ECHO_BIN,
                    "log-spewer": LOG_SPEWER_BIN,
                    "crash-after": CRASH_AFTER_BIN,
                },
            },
        })

    elif op == "prepare.run":
        # Quick prepare steps
        time.sleep(0.5)
        emit({
            "type": "response",
            "request_id": rid,
            "ok": True,
            "output": {
                "steps": [
                    {"name": "directories", "ok": True, "duration_ms": 200},
                    {"name": "config-files", "ok": True, "duration_ms": 300},
                ],
                "artifacts": {}
            },
        })

    elif op == "launch.plan":
        # Plan multiple services with different characteristics
        emit({
            "type": "response",
            "request_id": rid,
            "ok": True,
            "output": {
                "services": [
                    {
                        "name": "backend",
                        "command": [HTTP_ECHO_BIN, "--port", PORT_BACKEND],
                        "env": {
                            "ENV": "development",
                            "DB_URL": "postgresql://localhost:5432/testdb",
                            "API_SECRET": "super-secret-key",  # Should be redacted
                        },
                        "health": {
                            "type": "http",
                            "url": f"http://127.0.0.1:{PORT_BACKEND}/health",
                            "timeout_ms": 5000
                        },
                    },
                    {
                        "name": "worker",
                        "command": [HTTP_ECHO_BIN, "--port", PORT_WORKER],
                        "env": {
                            "WORKER_CONCURRENCY": "4",
                            "REDIS_URL": "redis://localhost:6379",
                            "AUTH_TOKEN": "secret-token",  # Should be redacted
                        },
                        "health": {
                            "type": "tcp",
                            "address": f"127.0.0.1:{PORT_WORKER}",
                            "timeout_ms": 3000
                        },
                    },
                    {
                        "name": "log-producer",
                        "command": [LOG_SPEWER_BIN, "--interval", "100ms", "--lines", "500"],
                        # No health check - tests "unknown" state
                    },
                    {
                        "name": "flaky",
                        "command": [HTTP_ECHO_BIN, "--port", PORT_FLAKY],
                        "health": {
                            "type": "http",
                            "url": f"http://127.0.0.1:{PORT_FLAKY}/health",
                            "timeout_ms": 2000
                        },
                    },
                    {
                        "name": "short-lived",
                        "command": [CRASH_AFTER_BIN, "--after", "30s", "--code", "0"],
                        # Exits after 30 seconds to test exit info display
                    },
                ]
            },
        })

    else:
        emit({
            "type": "response",
            "request_id": rid,
            "ok": False,
            "error": {"code": "E_UNSUPPORTED", "message": f"unsupported op: {op}"},
        })
PYPLUGIN

chmod +x "$REPO_ROOT/plugins/comprehensive.py"

# Create a second plugin for testing plugin list view
cat >"$REPO_ROOT/plugins/logger.py" <<'PYPLUGIN2'
#!/usr/bin/env python3
"""
Simple logger plugin - just for testing plugin list display.
"""
import json
import sys

def emit(obj):
    sys.stdout.write(json.dumps(obj) + "\n")
    sys.stdout.flush()

emit({
    "type": "handshake",
    "protocol_version": "v2",
    "plugin_name": "logger-plugin",
    "capabilities": {
        "ops": [],
        "streams": ["logs.aggregate"],
    },
})

for line in sys.stdin:
    pass  # Just consume stdin
PYPLUGIN2

chmod +x "$REPO_ROOT/plugins/logger.py"

# Create a third plugin for variety
cat >"$REPO_ROOT/plugins/metrics.py" <<'PYPLUGIN3'
#!/usr/bin/env python3
"""
Metrics collector plugin - just for testing plugin list display.
"""
import json
import sys

def emit(obj):
    sys.stdout.write(json.dumps(obj) + "\n")
    sys.stdout.flush()

emit({
    "type": "handshake",
    "protocol_version": "v2",
    "plugin_name": "metrics-plugin",
    "capabilities": {
        "ops": ["metrics.collect"],
        "streams": ["metrics.stream"],
        "commands": [{"name": "metrics"}],
    },
})

for line in sys.stdin:
    line = line.strip()
    if not line:
        continue
    req = json.loads(line)
    rid = req.get("request_id", "")
    emit({
        "type": "response",
        "request_id": rid,
        "ok": True,
        "output": {},
    })
PYPLUGIN3

chmod +x "$REPO_ROOT/plugins/metrics.py"

# Create .devctl.yaml with all plugins
cat >"$REPO_ROOT/.devctl.yaml" <<YAML
# Comprehensive fixture for TUI testing
# Generated by setup-comprehensive-fixture.sh

plugins:
  - id: comprehensive
    path: python3
    args:
      - "$REPO_ROOT/plugins/comprehensive.py"
    env:
      DEVCTL_HTTP_ECHO_BIN: "$REPO_ROOT/bin/http-echo"
      DEVCTL_LOG_SPEWER_BIN: "$REPO_ROOT/bin/log-spewer"
      DEVCTL_CRASH_AFTER_BIN: "$REPO_ROOT/bin/crash-after"
      PORT_BACKEND: "$PORT_BACKEND"
      PORT_WORKER: "$PORT_WORKER"
      PORT_FLAKY: "$PORT_FLAKY"
    priority: 10

  - id: logger
    path: python3
    args:
      - "$REPO_ROOT/plugins/logger.py"
    priority: 20

  - id: metrics
    path: python3
    args:
      - "$REPO_ROOT/plugins/metrics.py"
    priority: 30
YAML

# Print summary to stderr
cat >&2 <<INFO

Comprehensive Fixture Created!
==============================
REPO_ROOT: $REPO_ROOT

Services:
  - backend:      HTTP server with health check on port $PORT_BACKEND
  - worker:       HTTP server with TCP health check on port $PORT_WORKER
  - log-producer: Continuous log output (no health check)
  - flaky:        HTTP server on port $PORT_FLAKY
  - short-lived:  Will exit after 30s (tests exit info display)

Plugins:
  - comprehensive (priority 10): config.mutate, validate, build, prepare, launch
  - logger (priority 20): logs.aggregate stream
  - metrics (priority 30): metrics.collect op, metrics.stream, metrics command

To use:
  cd /path/to/devctl
  go run ./cmd/devctl --repo-root "$REPO_ROOT" up
  go run ./cmd/devctl --repo-root "$REPO_ROOT" tui

Cleanup:
  rm -rf "$REPO_ROOT"

INFO

# Output just the path to stdout (for scripting)
echo "$REPO_ROOT"
