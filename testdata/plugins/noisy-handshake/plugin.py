#!/usr/bin/env python3
import json
import sys

sys.stdout.write("NOT JSON\n")
sys.stdout.flush()

sys.stdout.write(json.dumps({
    "type": "handshake",
    "protocol_version": "v2",
    "plugin_name": "noisy-handshake",
    "capabilities": {"ops": ["ping"]},
}) + "\n")
sys.stdout.flush()

for line in sys.stdin:
    pass
