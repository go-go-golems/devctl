#!/usr/bin/env python3
import json
import sys

def emit(obj):
    sys.stdout.write(json.dumps(obj) + "\n")
    sys.stdout.flush()

emit({
    "type": "handshake",
    "protocol_version": "v1",
    "plugin_name": "noisy-after-handshake",
    "capabilities": {"ops": ["ping"]},
})

sys.stdout.write("oops-not-json\n")
sys.stdout.flush()

for line in sys.stdin:
    # never reached in practice for the runtime; kept to not exit instantly
    req = json.loads(line)
    rid = req.get("request_id", "")
    emit({"type": "response", "request_id": rid, "ok": True, "output": {"pong": True}})

