#!/usr/bin/env python3
import json
import sys

def emit(obj):
    sys.stdout.write(json.dumps(obj) + "\n")
    sys.stdout.flush()

emit({
    "type": "handshake",
    "protocol_version": "v1",
    "plugin_name": "python-minimal",
    "capabilities": {"ops": ["config.mutate", "launch.plan", "commands.list", "command.run"]},
})

for line in sys.stdin:
    req = json.loads(line)
    rid = req.get("request_id", "")
    op = req.get("op", "")

    if op == "config.mutate":
        emit({"type": "response", "request_id": rid, "ok": True, "output": {"config_patch": {"set": {"services.demo.port": 1234}, "unset": []}}})
    elif op == "launch.plan":
        emit({"type": "response", "request_id": rid, "ok": True, "output": {"services": [{"name": "demo", "command": ["bash", "-lc", "echo demo && sleep 3600"]}]}})
    elif op == "commands.list":
        emit({"type": "response", "request_id": rid, "ok": True, "output": {"commands": [{"name": "hello", "help": "Say hello"}]}})
    elif op == "command.run":
        name = req.get("input", {}).get("name")
        if name != "hello":
            emit({"type": "response", "request_id": rid, "ok": False, "error": {"code": "ENOENT", "message": "unknown command"}})
            continue
        print("hello from python-minimal", file=sys.stderr)
        emit({"type": "response", "request_id": rid, "ok": True, "output": {"exit_code": 0}})
    else:
        emit({"type": "response", "request_id": rid, "ok": False, "error": {"code": "E_UNSUPPORTED", "message": "unsupported op"}})

