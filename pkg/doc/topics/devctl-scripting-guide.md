---
Title: devctl Scripting Guide (Writing Practical Plugins)
Slug: devctl-scripting-guide
Short: "How to write real devctl plugins in Python or shell: patterns, pitfalls, testing loops, and dynamic commands."
Topics:
  - devctl
  - plugins
  - scripting
  - protocol
  - ndjson
  - debugging
IsTemplate: false
IsTopLevel: true
ShowPerDefault: true
SectionType: GeneralTopic
---

# devctl Scripting Guide (Writing Practical Plugins)

This guide is the "how do I actually ship this?" companion to the protocol reference. It focuses on practical patterns: how to structure a plugin, how to debug it when it breaks, and how to turn repo knowledge into a predictable `devctl up/status/logs/down` loop.

**Prerequisites**: This guide assumes you've read the user guide (`glaze help devctl-user-guide`) and understand the basic devctl workflow.

If you're starting from a big `startdev.sh`, the most important mindset shift is: your plugin computes *facts* (config, validation, and a plan), and devctl owns the lifecycle (starting processes, tracking state, capturing logs).

## 1. The two hard rules: handshake first, stdout is sacred

devctl plugins are NDJSON-over-stdio programs. That’s deliberately boring: if you can write to stdin/stdout, you can write a plugin in almost any language.

Two rules matter more than everything else:

1. The very first line on stdout must be a valid JSON handshake.
2. After that, every line on stdout must be a valid JSON frame (request/response/event). Logs must go to stderr.

If you violate rule (2), devctl will fail with an error like:

```text
E_PROTOCOL_STDOUT_CONTAMINATION: ... invalid character ...
```

## 2. A minimal, production-friendly plugin skeleton (Python)

A good plugin starts strict and small. It should flush output, return `E_UNSUPPORTED` for unknown ops, and keep all logs on stderr.

Create `plugins/myrepo.py`:

```python
#!/usr/bin/env python3
import json
import sys

def emit(obj):
    sys.stdout.write(json.dumps(obj) + "\n")
    sys.stdout.flush()

def log(msg):
    sys.stderr.write(msg + "\n")
    sys.stderr.flush()

emit({
    "type": "handshake",
    "protocol_version": "v2",
    "plugin_name": "myrepo",
    "capabilities": {"ops": ["config.mutate", "validate.run", "launch.plan"]},
})

for line in sys.stdin:
    line = line.strip()
    if not line:
        continue
    req = json.loads(line)
    rid = req.get("request_id", "")
    op = req.get("op", "")
    ctx = req.get("ctx", {}) or {}
    inp = req.get("input", {}) or {}

    try:
        if op == "config.mutate":
            emit({
                "type": "response",
                "request_id": rid,
                "ok": True,
                "output": {"config_patch": {"set": {"services.api.port": 8080}, "unset": []}},
            })
        elif op == "validate.run":
            # Use ctx.get("repo_root") to locate files.
            emit({
                "type": "response",
                "request_id": rid,
                "ok": True,
                "output": {"valid": True, "errors": [], "warnings": []},
            })
        elif op == "launch.plan":
            dry_run = bool(ctx.get("dry_run", False))
            if dry_run:
                log("dry-run: computing plan without side effects")
            emit({
                "type": "response",
                "request_id": rid,
                "ok": True,
                "output": {
                    "services": [
                        {"name": "api", "command": ["bash", "-lc", "python3 -m http.server 8080"]},
                    ]
                },
            })
        else:
            emit({
                "type": "response",
                "request_id": rid,
                "ok": False,
                "error": {"code": "E_UNSUPPORTED", "message": f"unsupported op: {op}"},
            })
    except Exception as e:
        emit({
            "type": "response",
            "request_id": rid,
            "ok": False,
            "error": {"code": "E_PLUGIN", "message": str(e)},
        })
```

## 3. The request context: repo_root, cwd, dry_run, deadline_ms

Every request includes a `ctx` object. This is how devctl passes “where am I?” and “how much time do you have?” information to your plugin.

In practice:

- `ctx.repo_root`: the repo root chosen by the user (via `--repo-root`, or CWD by default).
- `ctx.cwd`: the current working directory of the devctl process.
- `ctx.dry_run`: best-effort “no side effects”.
- `ctx.deadline_ms`: how long until devctl will cancel this operation.

The simplest safe behavior is:

- treat relative paths as relative to `ctx.repo_root`
- avoid side effects when `ctx.dry_run` is true
- ensure your own subprocesses/timeouts respect `ctx.deadline_ms`

## 4. Implementing the pipeline ops in the right order

devctl’s pipeline is intentionally consistent across repos. You can implement any subset, and devctl will only call what you declare in the handshake.

Common ops:

- `config.mutate`: return a config patch (dotted keys) that devctl applies.
- `validate.run`: return errors/warnings that make failures actionable.
- `build.run`: run named build steps (return artifacts and step results).
- `prepare.run`: run named prepare steps (same “step result” pattern).
- `launch.plan`: return the list of services devctl should supervise.

If you want the full schema for each op’s input/output, use the protocol guide:

```text
glaze help devctl-plugin-authoring
```

## 5. Dynamic commands: turning scripts into `devctl <cmd>`

Dynamic commands are for “repo helpers” that you want to standardize and ship alongside the rest of the dev environment knowledge (for example, `db-reset`, `seed-data`, or `gen-certs`).

To expose a dynamic command:

1. Add `command.run` to `capabilities.ops`
2. Add a command spec to `capabilities.commands`
3. Implement `command.run` to execute the command and return an `exit_code`

Example handshake snippet:

```json
{
  "type": "handshake",
  "protocol_version": "v2",
  "plugin_name": "myrepo",
  "capabilities": {
    "ops": ["command.run"],
    "commands": [
      { "name": "db-reset", "help": "Reset the dev database", "args_spec": [] }
    ]
  }
}
```

Example `command.run` response:

```json
{ "type": "response", "request_id": "x", "ok": true, "output": { "exit_code": 0 } }
```

Practical guidance:

- `argv` should behave like a normal CLI argv: treat it as untrusted user input.
- Use `ctx.dry_run` to implement “no side effects” modes when reasonable.
- If your command needs repo config, use the `config` object included in the `command.run` input (it’s the merged config after `config.mutate`).

## 6. A shell plugin pattern (bash + jq) that stays safe

Shell plugins are totally viable, but they require discipline because stdout is the protocol. The safest pattern is:

- write a strict handshake to stdout once
- read stdin line-by-line
- parse with `jq` (recommended; assume it’s available on developer machines)
- write *only* JSON responses to stdout
- log only to stderr

Minimal sketch:

```bash
#!/usr/bin/env bash
set -euo pipefail

emit() { jq -c . <<<"$1"; }
log() { printf '%s\n' "$*" >&2; }

emit '{"type":"handshake","protocol_version":"v2","plugin_name":"bash","capabilities":{"ops":["launch.plan"]}}'

while IFS= read -r line; do
  [ -z "$line" ] && continue
  rid="$(jq -r '.request_id // ""' <<<"$line")"
  op="$(jq -r '.op // ""' <<<"$line")"

  if [ "$op" = "launch.plan" ]; then
    emit "$(jq -nc --arg rid "$rid" '{
      type:"response", request_id:$rid, ok:true,
      output:{services:[{name:"api", command:["bash","-lc","echo api && sleep 3600"]}]}
    }')"
  else
    emit "$(jq -nc --arg rid "$rid" --arg op "$op" '{
      type:"response", request_id:$rid, ok:false,
      error:{code:"E_UNSUPPORTED", message:("unsupported op: "+$op)}
    }')"
  fi
done
```

## 7. Testing loops that catch real problems early

Good plugin testing is less about unit tests and more about tight feedback loops: validate handshake, validate pipeline behavior, validate timeouts and failure reporting.

A practical progression:

1. Validate handshake and capabilities:
   - `devctl plugins list`
2. Validate planning:
   - `devctl plan`
3. Validate supervision and logs:
   - `devctl up`
   - `devctl status`
   - `devctl logs --service <name> --follow`
   - `devctl down`

When debugging protocol issues, run with a higher log level:

```bash
devctl --log-level debug plugins list
```

## 8. Common pitfalls (and how to avoid them)

The failure modes are predictable. If you build guardrails into your plugin from day one, you’ll avoid most of them.

- **Printing to stdout:** write logs to stderr only.
- **Not flushing:** always flush after emitting JSON.
- **Ignoring timeouts:** use `ctx.deadline_ms` to bound subprocess waits.
- **Hiding validation failures:** put clear, actionable error messages into `validate.run` output.
- **Doing lifecycle inside the plugin:** return a service plan; let devctl supervise it.

## 9. Where to go next

If you want the complete protocol details (schemas, more examples, and deeper guidance on merging/strictness), use the authoring guide:

```text
glaze help devctl-plugin-authoring
```

If you want to understand devctl as a user first (before writing plugins), start with:

```text
glaze help devctl-user-guide
```
