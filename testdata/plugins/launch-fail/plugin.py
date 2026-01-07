#!/usr/bin/env python3
import json
import os
import sys


def emit(obj):
    sys.stdout.write(json.dumps(obj) + "\n")
    sys.stdout.flush()


HEALTH_PORT = int(os.environ.get("DEVCTL_FAIL_HEALTH_PORT", "19099"))

emit(
    {
        "type": "handshake",
        "protocol_version": "v2",
        "plugin_name": "launch-fail",
        "capabilities": {"ops": ["config.mutate", "validate.run", "launch.plan"]},
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
                "output": {"config_patch": {"set": {}, "unset": []}},
            }
        )
    elif op == "validate.run":
        emit(
            {
                "type": "response",
                "request_id": rid,
                "ok": True,
                "output": {"valid": True, "errors": [], "warnings": []},
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
                            "name": "fail",
                            "command": ["bash", "-lc", "echo failing && exit 2"],
                            "health": {
                                "type": "tcp",
                                "address": f"127.0.0.1:{HEALTH_PORT}",
                                "timeout_ms": 1000,
                            },
                        }
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
