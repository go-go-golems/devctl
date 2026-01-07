#!/usr/bin/env python3
import json
import os
import sys


def emit(obj):
    sys.stdout.write(json.dumps(obj) + "\n")
    sys.stdout.flush()


HTTP_ECHO_BIN = os.environ.get("DEVCTL_HTTP_ECHO_BIN", "")
LOG_SPEWER_BIN = os.environ.get("DEVCTL_LOG_SPEWER_BIN", "")
HTTP_ECHO_PORT = int(os.environ.get("DEVCTL_HTTP_ECHO_PORT", "0"))

emit(
    {
        "type": "handshake",
        "protocol_version": "v2",
        "plugin_name": "e2e-plugin",
        "capabilities": {
            "ops": ["config.mutate", "validate.run", "build.run", "prepare.run", "launch.plan"]
        },
    }
)

for line in sys.stdin:
    line = line.strip()
    if not line:
        continue
    req = json.loads(line)
    rid = req.get("request_id", "")
    op = req.get("op", "")

    if op == "config.mutate":
        emit(
            {
                "type": "response",
                "request_id": rid,
                "ok": True,
                "output": {
                    "config_patch": {
                        "set": {
                            "services.http.port": HTTP_ECHO_PORT,
                        },
                        "unset": [],
                    }
                },
            }
        )
    elif op == "validate.run":
        errors_ = []
        if not HTTP_ECHO_BIN:
            errors_.append({"code": "E_MISSING", "message": "missing DEVCTL_HTTP_ECHO_BIN"})
        if not LOG_SPEWER_BIN:
            errors_.append({"code": "E_MISSING", "message": "missing DEVCTL_LOG_SPEWER_BIN"})
        if HTTP_ECHO_PORT <= 0:
            errors_.append({"code": "E_MISSING", "message": "missing/invalid DEVCTL_HTTP_ECHO_PORT"})
        emit(
            {
                "type": "response",
                "request_id": rid,
                "ok": True,
                "output": {
                    "valid": len(errors_) == 0,
                    "errors": errors_,
                    "warnings": [],
                },
            }
        )
    elif op == "build.run":
        emit(
            {
                "type": "response",
                "request_id": rid,
                "ok": True,
                "output": {
                    "steps": [
                        {"name": "testapps", "ok": True, "duration_ms": 0},
                    ],
                    "artifacts": {
                        "http-echo": HTTP_ECHO_BIN,
                        "log-spewer": LOG_SPEWER_BIN,
                    },
                },
            }
        )
    elif op == "prepare.run":
        emit(
            {
                "type": "response",
                "request_id": rid,
                "ok": True,
                "output": {"steps": [{"name": "noop", "ok": True, "duration_ms": 0}], "artifacts": {}},
            }
        )
    elif op == "launch.plan":
        emit(
            {
                "type": "response",
                "request_id": rid,
                "ok": True,
                "output": {
                    "services": [
                        {
                            "name": "http",
                            "command": [HTTP_ECHO_BIN, "--port", str(HTTP_ECHO_PORT)],
                            "health": {"type": "http", "url": f"http://127.0.0.1:{HTTP_ECHO_PORT}/health", "timeout_ms": 5000},
                        },
                        {
                            "name": "spewer",
                            "command": [LOG_SPEWER_BIN, "--interval", "25ms", "--lines", "100"],
                        },
                    ]
                },
            }
        )
    else:
        emit(
            {
                "type": "response",
                "request_id": rid,
                "ok": False,
                "error": {"code": "E_UNSUPPORTED", "message": "unsupported op"},
            }
        )
