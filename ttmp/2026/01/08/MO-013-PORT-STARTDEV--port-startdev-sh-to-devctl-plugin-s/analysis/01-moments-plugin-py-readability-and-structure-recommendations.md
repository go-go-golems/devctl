---
Status: active
Intent: long-term
Topics:
  - devctl
  - devtools
  - moments
  - scripting
---

# moments-plugin.py readability and structure recommendations

## Summary

The current `moments/plugins/moments-plugin.py` is correct but hard to scan because the primary control flow (handshake + request loop + op dispatch) appears after a long list of helpers. This makes the "main functionality" feel hidden, even though it is simple. The file also mixes unrelated helpers (I/O, config parsing, process execution, and op handlers), which reduces the reader’s ability to see the plugin’s responsibilities at a glance.

This document proposes a re-organization that preserves behavior while making the core workflow obvious within the first 30-40 lines. It also suggests small structural helpers to reduce duplication in build/prepare step execution and make launch planning read as the central feature rather than an implementation detail.

## Goals

- Make the primary request flow (handshake, parse, dispatch) immediately visible.
- Group core domain handlers together so the plugin’s capabilities are easy to see.
- Reduce incidental complexity (repeated step loops, inline parsing) without changing behavior.
- Keep changes minimal and single-file unless there is a clear reuse benefit.

## Current pain points

1. **Main flow is buried**
   - The handshake + request loop is at the bottom and visually disconnected from the handler definitions.

2. **Helpers are interleaved by type, not by relevance**
   - I/O helpers, config parsing helpers, and process helpers are mixed with op handlers.
   - The reader cannot quickly find the core “op” logic.

3. **Repeated step execution structure**
   - `handle_build_run` and `handle_prepare_run` share almost identical step-loop logic and error handling.

4. **Launch planning looks like a detail**
   - The core output (services plan) is hidden behind config and env resolution that reads as a large block.

5. **Inline parsing/validation distracts from the op intent**
   - Per-request parsing and error paths are intermixed with the business logic.

## Proposed file structure (single-file refactor)

Suggested ordering to make the core flow visible first:

1. **Constants + op registry**
   - `OPS` mapping op name to handler
   - defaults for ports/tools

2. **Entry points**
   - `main()` handles handshake + loop
   - `handle_request(req)` parses, validates, dispatches

3. **Op handlers (domain logic)**
   - `handle_config_mutate`
   - `handle_validate_run`
   - `handle_build_run`
   - `handle_prepare_run`
   - `handle_launch_plan`

4. **Shared step runner**
   - helper that executes steps, reports duration, and standardizes errors

5. **Helpers (supporting code)**
   - I/O (`emit`, `log`, `respond_error`)
   - process helpers (`run`, `which`, `is_port_free`)
   - config helpers (`get_config_*`, `require_int_env`, `compute_vite_env`, `moments_config_*`)

This layout keeps the core plugin responsibilities near the top, while still allowing helpers to be nearby for reference.

## Concrete refactor suggestions

### 1) Explicit entry point and dispatch

Make the main flow visible at the top of the file. This makes it obvious how the plugin behaves without scrolling.

Example structure:

```python
OPS = {
    "config.mutate": handle_config_mutate,
    "validate.run": handle_validate_run,
    "build.run": handle_build_run,
    "prepare.run": handle_prepare_run,
    "launch.plan": handle_launch_plan,
}


def main() -> None:
    emit_handshake()
    for req in read_requests():
        handle_request(req)


def handle_request(req: Dict[str, Any]) -> None:
    ctx = parse_request(req)
    handler = OPS.get(ctx.op)
    if not handler:
        respond_error(ctx.rid, "E_UNSUPPORTED", f"unsupported op: {ctx.op}")
        return
    handler(ctx)
```

Supporting helpers like `read_requests`, `emit_handshake`, and `parse_request` can be small, but they make the flow explicit.

### 2) Introduce a small context object

Use a simple `Context` dict or lightweight dataclass to reduce parameter threading.

Possible fields:
- `rid`
- `op`
- `input`
- `ctx`
- `dry_run`
- `repo_root`

This avoids repeating `rid`, `inp`, and `ctx` across all handlers and makes the function signatures more uniform.

### 3) Standardize step execution

Both build and prepare are simple step runners. A shared helper can centralize logic and make handler functions much shorter.

Example:

```python
def run_steps(rid, steps, registry, *, dry_run):
    results = []
    for step in steps:
        handler = registry.get(step)
        if not handler:
            respond_error(rid, "E_UNKNOWN_STEP", f"unknown step: {step!r}")
            return None
        started = time.time()
        ok, exit_code = handler(dry_run)
        if not ok:
            respond_error(rid, "E_STEP_FAILED", f"step {step!r} failed (exit_code={exit_code})")
            return None
        results.append({"name": step, "ok": True, "duration_ms": int((time.time() - started) * 1000)})
    return results
```

Then `handle_build_run` and `handle_prepare_run` simply define their step registries and call `run_steps`.

### 4) Make launch plan the centerpiece

`handle_launch_plan` is the core of the plugin. It should read as: "compute config/env" then "emit services". Extract `compute_vite_env` into a small wrapper to keep the op handler short.

Example:

```python
vite_env = resolve_vite_env(cfg, repo_root, dry_run)
services = build_services(backend_port, web_port, vite_env)
emit_response(...)
```

This structure highlights the launch plan itself instead of the mechanics of env resolution.

### 5) Group helpers by concern

A reader should be able to see helper categories quickly. Even short comment dividers help:

- `# --- IO helpers ---`
- `# --- Process/system helpers ---`
- `# --- Config helpers ---`

Keep comments minimal, but use them to signal context shifts.

## Optional: multi-file split

If this file grows further, consider extracting helpers into a small local module:

- `moments/plugins/moments_plugin_helpers.py`
- Keep only entry points and op handlers in `moments-plugin.py`

This makes the main file read like a compact plugin “spec” and reduces scroll distance.

## Implementation checklist

- [ ] Move handshake + main loop into `main()` and call it at the bottom.
- [ ] Add `OPS` registry and centralize dispatch logic.
- [ ] Introduce a `Context` object or dict to reduce parameter passing.
- [ ] Add a shared step runner to unify build/prepare loops.
- [ ] Group helpers by concern, with light section headers.
- [ ] Keep behavior unchanged (no changes to output schema, env precedence, or validation rules).

## Expected outcome

After these changes, the main behavior of the plugin is visible immediately: it declares its capabilities, reads requests, and dispatches to handlers. The handlers themselves are clustered together, so the core responsibilities of the plugin can be understood quickly. Helper logic remains intact but is visually demoted, making it easier to understand the file as a whole.
