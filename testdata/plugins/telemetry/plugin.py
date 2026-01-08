#!/usr/bin/env python3
import json
import sys
import time


def emit(obj):
    sys.stdout.write(json.dumps(obj) + "\n")
    sys.stdout.flush()


emit(
    {
        "type": "handshake",
        "protocol_version": "v2",
        "plugin_name": "telemetry",
        "capabilities": {
            "ops": ["telemetry.stream"],
            "streams": ["telemetry.stream"],
        },
        "declares": {"side_effects": "none", "idempotent": True},
    }
)


for line in sys.stdin:
    line = line.strip()
    if not line:
        continue
    req = json.loads(line)
    rid = req.get("request_id", "")
    op = req.get("op", "")

    if op != "telemetry.stream":
        emit(
            {
                "type": "response",
                "request_id": rid,
                "ok": False,
                "error": {"code": "E_UNSUPPORTED", "message": f"unsupported op: {op}"},
            }
        )
        continue

    inp = req.get("input", {}) or {}
    count = int(inp.get("count", 3))
    interval_ms = int(inp.get("interval_ms", 10))

    stream_id = f"telemetry-{rid}"
    emit(
        {
            "type": "response",
            "request_id": rid,
            "ok": True,
            "output": {"stream_id": stream_id, "schema": "telemetry.v1"},
        }
    )

    for i in range(count):
        emit(
            {
                "type": "event",
                "stream_id": stream_id,
                "event": "metric",
                "fields": {"name": "counter", "value": i, "unit": "count"},
            }
        )
        time.sleep(interval_ms / 1000.0)

    emit({"type": "event", "stream_id": stream_id, "event": "end", "ok": True})

