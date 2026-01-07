#!/usr/bin/env python3
import json
import os
import sys


def emit(obj):
    sys.stdout.write(json.dumps(obj) + "\n")
    sys.stdout.flush()


FAIL = os.environ.get("DEVCTL_VALIDATE_FAIL", "") in ("1", "true", "yes")

emit(
    {
        "type": "handshake",
        "protocol_version": "v2",
        "plugin_name": "validate-passfail",
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
        if FAIL:
            emit(
                {
                    "type": "response",
                    "request_id": rid,
                    "ok": True,
                    "output": {
                        "valid": False,
                        "errors": [{"code": "E_VALIDATE_FAIL", "message": "forced validation failure"}],
                        "warnings": [],
                    },
                }
            )
        else:
            emit(
                {
                    "type": "response",
                    "request_id": rid,
                    "ok": True,
                    "output": {"valid": True, "errors": [], "warnings": []},
                }
            )
    elif op == "launch.plan":
        emit({"type": "response", "request_id": rid, "ok": True, "output": {"services": []}})
    else:
        emit(
            {
                "type": "response",
                "request_id": rid,
                "ok": False,
                "error": {"code": "E_UNSUPPORTED", "message": "unsupported op"},
            }
        )
