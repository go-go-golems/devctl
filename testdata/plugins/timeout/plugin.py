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
        "plugin_name": "timeout-plugin",
        "capabilities": {"ops": ["ping"]},
    }
)

for line in sys.stdin:
    line = line.strip()
    if not line:
        continue
    req = json.loads(line)
    # intentionally never respond within reasonable time
    _ = req.get("request_id", "")
    time.sleep(60)
