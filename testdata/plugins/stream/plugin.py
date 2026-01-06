#!/usr/bin/env python3
import json
import sys
import time

def emit(obj):
    sys.stdout.write(json.dumps(obj) + "\n")
    sys.stdout.flush()

emit({
    "type": "handshake",
    "protocol_version": "v1",
    "plugin_name": "stream-plugin",
    "capabilities": {"ops": ["stream.start"]},
})

for line in sys.stdin:
    line = line.strip()
    if not line:
        continue
    req = json.loads(line)
    rid = req.get("request_id", "")
    op = req.get("op", "")

    if op != "stream.start":
        emit({"type": "response", "request_id": rid, "ok": False, "error": {"code": "E_UNSUPPORTED", "message": "unsupported op"}})
        continue

    stream_id = "s1"
    emit({"type": "response", "request_id": rid, "ok": True, "output": {"stream_id": stream_id}})
    emit({"type": "event", "stream_id": stream_id, "event": "log", "level": "info", "message": "hello"})
    time.sleep(0.01)
    emit({"type": "event", "stream_id": stream_id, "event": "log", "level": "info", "message": "world"})
    emit({"type": "event", "stream_id": stream_id, "event": "end", "ok": True})

