#!/usr/bin/env python3
import json
import sys

PORT = 18081
URL = f"http://127.0.0.1:{PORT}/"

def emit(obj):
    sys.stdout.write(json.dumps(obj) + "\n")
    sys.stdout.flush()

emit({
    "type": "handshake",
    "protocol_version": "v1",
    "plugin_name": "http-service-plugin",
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
        emit({
            "type": "response",
            "request_id": rid,
            "ok": True,
            "output": {"config_patch": {"set": {"services.http.port": PORT, "services.http.url": URL}, "unset": []}},
        })
    elif op == "validate.run":
        emit({
            "type": "response",
            "request_id": rid,
            "ok": True,
            "output": {"valid": True, "errors": [], "warnings": []},
        })
    elif op == "launch.plan":
        emit({
            "type": "response",
            "request_id": rid,
            "ok": True,
            "output": {
                "services": [{
                    "name": "http",
                    "command": ["python3", "-m", "http.server", str(PORT)],
                    "health": {"type": "http", "url": URL, "timeout_ms": 5000},
                }]
            },
        })
    else:
        emit({
            "type": "response",
            "request_id": rid,
            "ok": False,
            "error": {"code": "E_UNSUPPORTED", "message": "unsupported op"},
        })

