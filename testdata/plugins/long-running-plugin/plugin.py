#!/usr/bin/env python3
import json
import sys
import threading
import time


def emit(obj):
    sys.stdout.write(json.dumps(obj) + "\n")
    sys.stdout.flush()


emit(
    {
        "type": "handshake",
        "protocol_version": "v2",
        "plugin_name": "long-running-plugin",
        "capabilities": {"ops": ["logs.follow"]},
    }
)


def stream_worker(stream_id: str, stop: threading.Event) -> None:
    i = 0
    try:
        while not stop.is_set():
            emit({"type": "event", "stream_id": stream_id, "event": "log", "message": f"tick {i}"})
            i += 1
            time.sleep(0.1)
    finally:
        emit({"type": "event", "stream_id": stream_id, "event": "end"})


streams = {}

for line in sys.stdin:
    line = line.strip()
    if not line:
        continue
    req = json.loads(line)
    rid = req.get("request_id", "")
    op = req.get("op", "")

    if op == "logs.follow":
        stream_id = f"stream-{rid}"
        stop = threading.Event()
        t = threading.Thread(target=stream_worker, args=(stream_id, stop), daemon=True)
        streams[stream_id] = stop
        t.start()
        emit(
            {
                "type": "response",
                "request_id": rid,
                "ok": True,
                "output": {"stream_id": stream_id},
            }
        )
    else:
        emit(
            {
                "type": "response",
                "request_id": rid,
                "ok": False,
                "error": {"code": "E_UNSUPPORTED", "message": "unsupported op"},
            }
        )

for _, stop in streams.items():
    stop.set()
time.sleep(0.2)
