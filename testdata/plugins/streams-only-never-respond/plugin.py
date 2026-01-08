#!/usr/bin/env python3
import json
import sys


def emit(obj):
    sys.stdout.write(json.dumps(obj) + "\n")
    sys.stdout.flush()


# Intentionally advertises a stream capability without declaring the op in capabilities.ops,
# and never responds to requests. This fixture is used to validate that devctl fails fast
# on stream start (capabilities.ops is authoritative) rather than hanging until timeout.
emit(
    {
        "type": "handshake",
        "protocol_version": "v2",
        "plugin_name": "streams-only-never-respond",
        "capabilities": {
            "ops": [],
            "streams": ["telemetry.stream"],
        },
    }
)

for _line in sys.stdin:
    pass

