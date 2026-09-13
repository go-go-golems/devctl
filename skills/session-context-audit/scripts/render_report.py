#!/usr/bin/env python3
"""Render a standalone session context audit from a strict JSON model."""

from __future__ import annotations

import argparse
import functools
import html
import http.server
import json
from pathlib import Path
import shutil
import socketserver
import subprocess
import threading
import time
from typing import Any
from urllib.parse import quote
import webbrowser

CATEGORIES = {"analysis", "engineering", "menial/support", "issue/churn"}
TOP_LEVEL_KEYS = {"title", "kicker", "subtitle", "turns", "timeline", "knowledge", "file_groups", "change_groups", "api_groups", "state"}


def require_strings(values: Any, scope: str) -> None:
    if not isinstance(values, list) or not all(isinstance(value, str) for value in values):
        raise ValueError(f"{scope} must be a list of strings")


def text(value: Any) -> str:
    return html.escape(str(value), quote=True)


def require_list(model: dict[str, Any], key: str) -> list[Any]:
    value = model.get(key)
    if not isinstance(value, list):
        raise ValueError(f"{key} must be a list")
    return value


def validate_items(items: list[Any], scope: str) -> None:
    for index, item in enumerate(items):
        if not isinstance(item, dict) or not isinstance(item.get("name"), str) or not isinstance(item.get("purpose"), str):
            raise ValueError(f"{scope}[{index}] must contain string name and purpose")


def validate(model: dict[str, Any]) -> None:
    if not isinstance(model, dict):
        raise ValueError("report must be a JSON object")
    unknown = set(model) - TOP_LEVEL_KEYS
    if unknown:
        raise ValueError(f"unknown top-level keys: {', '.join(sorted(unknown))}")
    for key in ("title", "kicker", "subtitle"):
        if not isinstance(model.get(key), str) or not model[key].strip():
            raise ValueError(f"{key} must be a non-empty string")
    for key in ("turns", "timeline", "knowledge", "file_groups", "change_groups", "api_groups", "state"):
        require_list(model, key)
    numbers: set[str] = set()
    for index, turn in enumerate(model["turns"]):
        if not isinstance(turn, dict) or set(turn) != {"number", "title", "bullets"} or not isinstance(turn.get("number"), str) or not isinstance(turn.get("title"), str):
            raise ValueError(f"turns[{index}] is invalid")
        require_strings(turn.get("bullets"), f"turns[{index}].bullets")
        if turn["number"] in numbers:
            raise ValueError(f"turns[{index}] has duplicate number {turn['number']!r}")
        numbers.add(turn["number"])
    for index, entry in enumerate(model["timeline"]):
        if not isinstance(entry, dict) or set(entry) != {"period", "category", "title", "bullets", "advice"} or entry.get("category") not in CATEGORIES:
            raise ValueError(f"timeline[{index}] has invalid category or shape")
        if not isinstance(entry.get("period"), str) or not isinstance(entry.get("title"), str):
            raise ValueError(f"timeline[{index}] is invalid")
        require_strings(entry.get("bullets"), f"timeline[{index}].bullets")
        require_strings(entry.get("advice"), f"timeline[{index}].advice")
        if entry["category"] == "issue/churn" and not any(value.strip() for value in entry["advice"]):
            raise ValueError(f"timeline[{index}] issue/churn requires prevention advice")
    for index, item in enumerate(model["knowledge"]):
        if not isinstance(item, dict) or set(item) != {"label", "text"} or not isinstance(item.get("label"), str) or not isinstance(item.get("text"), str):
            raise ValueError(f"knowledge[{index}] must contain string label and text")
    require_strings(model["state"], "state")
    for group_key in ("file_groups", "change_groups", "api_groups"):
        for index, group in enumerate(model[group_key]):
            if not isinstance(group, dict) or set(group) != {"title", "items"} or not isinstance(group.get("title"), str) or not isinstance(group.get("items"), list):
                raise ValueError(f"{group_key}[{index}] is invalid")
            validate_items(group["items"], f"{group_key}[{index}].items")


def bullets(values: list[Any]) -> str:
    return "<ul>" + "".join(f"<li>{text(value)}</li>" for value in values) + "</ul>"


def grouped_section(title: str, groups: list[dict[str, Any]]) -> str:
    rows = []
    for group in groups:
        items = "".join(
            f"<li><code>{text(item['name'])}</code><span class='purpose'>{text(item['purpose'])}</span></li>"
            for item in group["items"]
        )
        rows.append(f"<div class='group-row'><h3>{text(group['title'])}</h3><ul>{items}</ul></div>")
    return f"<section><h2>{text(title)}</h2><div class='groups'>{''.join(rows)}</div></section>"


def render(model: dict[str, Any]) -> str:
    validate(model)
    turns = "".join(
        f"<article class='row'><div class='period'>TURN {text(turn['number'])}</div>"
        f"<div class='body'><strong>{text(turn['title'])}</strong>{bullets(turn['bullets'])}</div></article>"
        for turn in model["turns"]
    )
    timeline_parts = []
    for entry in model["timeline"]:
        advice = entry.get("advice", [])
        advice_html = ""
        if advice:
            advice_html = "<div class='advice'><b>Prevention:</b>" + bullets(advice) + "</div>"
        timeline_parts.append(
            f"<article class='row'><div class='period'>{text(entry['period'])}</div><div class='body'>"
            f"<span class='badge'>{text(entry['category']).upper()}</span> "
            f"<strong>{text(entry['title'])}</strong>{bullets(entry['bullets'])}{advice_html}</div></article>"
        )
    knowledge = "".join(
        f"<p><span class='badge'>{text(item.get('label', 'FACT'))}</span> {text(item.get('text', ''))}</p>"
        for item in model["knowledge"]
    )
    style = """
:root{--paper:#eee9dc;--ink:#151515;--red:#ce382d;--blue:#274c77;--muted:#625f58}*{box-sizing:border-box}body{margin:0;background:var(--paper);color:var(--ink);font:16px/1.45 ui-monospace,SFMono-Regular,Menlo,monospace}header{padding:42px max(24px,6vw) 28px;border-bottom:8px solid var(--ink);background:#faf6eb}h1{max-width:1100px;margin:0;font:900 clamp(36px,7vw,84px)/.9 Arial Black,Impact,sans-serif;letter-spacing:-.045em;text-transform:uppercase}.kicker{display:inline-block;background:var(--red);color:#fff;padding:6px 10px;margin-bottom:18px;font-weight:900}.subtitle{margin-top:22px;color:var(--muted)}main{max-width:1400px;margin:auto;padding:32px max(24px,6vw) 80px}section{border:3px solid var(--ink);margin-bottom:24px;background:#faf6eb}h2{margin:0;padding:12px 16px;background:var(--ink);color:#fff;font:900 24px/1 Arial,sans-serif;text-transform:uppercase}h3{margin:0 0 12px;color:var(--red);font:900 20px Arial,sans-serif;text-transform:uppercase}.row{display:grid;grid-template-columns:130px 1fr;border-top:2px solid var(--ink)}.row:first-of-type{border-top:0}.period{padding:16px 12px;background:var(--blue);color:#fff;font-weight:900}.body,.content{padding:16px 20px}.group-row{display:grid;grid-template-columns:minmax(220px,28%) 1fr;border-top:2px solid var(--ink)}.group-row:first-child{border-top:0}.group-row h3{padding:18px 20px;margin:0;border-right:2px solid var(--ink)}.group-row ul{padding:12px 22px 12px 42px;margin:0}.group-row li{margin:8px 0}.purpose{display:block;margin-top:2px;font-family:Georgia,serif;font-style:italic;color:var(--muted)}.badge{display:inline-block;border:2px solid var(--ink);padding:2px 6px;font-size:12px;font-weight:900}.advice{border-left:6px solid var(--red);padding-left:12px}ul{margin:8px 0;padding-left:22px}li{margin:5px 0}code{overflow-wrap:anywhere}@media(max-width:760px){.row,.group-row{grid-template-columns:1fr}.group-row h3{border-right:0;border-bottom:2px solid var(--ink)}}
"""
    sections = [
        f"<section><h2>Conversation turns</h2>{turns}</section>",
        f"<section><h2>Session timeline</h2>{''.join(timeline_parts)}</section>",
        f"<section><h2>Current classified knowledge</h2><div class='content'>{knowledge}</div></section>",
        grouped_section("Files inspected or analyzed", model["file_groups"]),
        grouped_section("Files edited or created", model["change_groups"]),
        grouped_section("API and package inventory", model["api_groups"]),
        f"<section><h2>Repository and report state</h2><div class='content'>{bullets(model['state'])}</div></section>",
    ]
    return (
        "<!doctype html><html lang='en'><head><meta charset='utf-8'>"
        "<meta name='viewport' content='width=device-width,initial-scale=1'>"
        f"<title>{text(model['title'])} — Context Audit</title><style>{style}</style></head><body>"
        f"<header><div class='kicker'>{text(model['kicker'])}</div><h1>{text(model['title'])}</h1>"
        f"<div class='subtitle'>{text(model['subtitle'])}</div></header><main>{''.join(sections)}</main></body></html>"
    )


def open_uri(uri: str) -> None:
    opener = shutil.which("xdg-open")
    if opener:
        subprocess.run([opener, uri], check=True)
    elif not webbrowser.open(uri):
        raise RuntimeError(f"no browser opener accepted {uri}")


def serve_and_open(output: Path, seconds: float) -> None:
    handler = functools.partial(http.server.SimpleHTTPRequestHandler, directory=str(output.parent))
    with socketserver.TCPServer(("127.0.0.1", 0), handler) as server:
        thread = threading.Thread(target=server.serve_forever, daemon=True)
        thread.start()
        try:
            host, port = server.server_address
            open_uri(f"http://{host}:{port}/{quote(output.name)}")
            time.sleep(seconds)
        finally:
            server.shutdown()
            thread.join()


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--input", required=True, type=Path)
    parser.add_argument("--output", required=True, type=Path)
    opening = parser.add_mutually_exclusive_group()
    opening.add_argument("--open", action="store_true", help="open the standalone file:// URI")
    opening.add_argument("--serve-open", action="store_true", help="serve on an OS-selected loopback port, open, then stop")
    parser.add_argument("--serve-seconds", type=float, default=3.0, help="seconds to retain --serve-open server after opening")
    args = parser.parse_args()
    if args.serve_seconds < 0:
        parser.error("--serve-seconds must be non-negative")
    model = json.loads(args.input.read_text(encoding="utf-8"))
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(render(model), encoding="utf-8")
    output = args.output.resolve()
    print(output)
    if args.open:
        open_uri(output.as_uri())
    elif args.serve_open:
        serve_and_open(output, args.serve_seconds)


if __name__ == "__main__":
    main()
