#!/usr/bin/env python3
import json
import sys

def emit(obj):
    sys.stdout.write(json.dumps(obj) + "\n")
    sys.stdout.flush()

emit({
    "type": "handshake",
    "protocol_version": "v2",
    "plugin_name": "command-plugin",
    "capabilities": {
        "ops": ["command.run"],
        "commands": [
            {"name": "echo", "help": "Echo arguments to stderr", "args_spec": []},
        ],
    },
})

for line in sys.stdin:
    line = line.strip()
    if not line:
        continue
    req = json.loads(line)
    rid = req.get("request_id", "")
    op = req.get("op", "")
    inp = req.get("input", {})

    if op == "command.run":
        name = inp.get("name", "")
        argv = inp.get("argv", [])
        if name != "echo":
            emit({"type": "response", "request_id": rid, "ok": False, "error": {"code": "ENOENT", "message": "unknown command"}})
            continue
        print(" ".join(argv), file=sys.stderr)
        emit({"type": "response", "request_id": rid, "ok": True, "output": {"exit_code": 0}})
    else:
        emit({"type": "response", "request_id": rid, "ok": False, "error": {"code": "E_UNSUPPORTED", "message": "unsupported op"}})
