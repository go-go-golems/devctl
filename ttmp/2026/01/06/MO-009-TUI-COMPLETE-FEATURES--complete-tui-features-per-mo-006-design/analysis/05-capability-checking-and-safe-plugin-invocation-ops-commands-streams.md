---
Title: Capability checking and safe plugin invocation (ops/commands/streams)
Ticket: MO-009-TUI-COMPLETE-FEATURES
Status: active
Topics:
    - backend
    - ui-components
DocType: analysis
Intent: long-term
Owners: []
RelatedFiles:
    - Path: devctl/cmd/devctl/cmds/dynamic_commands.go
      Note: Unconditional commands.list and command.run calls
    - Path: devctl/cmd/devctl/cmds/plugins.go
      Note: plugins list prints handshake capabilities
    - Path: devctl/cmd/devctl/main.go
      Note: Runs dynamic command discovery before executing any command
    - Path: devctl/pkg/engine/pipeline.go
      Note: Pipeline ops are gated by SupportsOp
    - Path: devctl/pkg/protocol/types.go
      Note: Handshake capability schema (ops/streams/commands)
    - Path: devctl/pkg/protocol/validate.go
      Note: Handshake validation does not enforce capabilities
    - Path: devctl/pkg/runtime/client.go
      Note: SupportsOp and raw Call/StartStream semantics
    - Path: devctl/pkg/runtime/factory.go
      Note: Plugin startup + handshake reading (timeouts
ExternalSources: []
Summary: How devctl uses handshake capabilities to decide which plugin ops/commands/streams to call, where calls are currently unguarded, and how to avoid hangs
LastUpdated: 2026-01-07T00:21:01.52199953-05:00
WhatFor: Prevent startup delays/hangs by making capability checks consistent across pipeline ops, dynamic commands, and streams
WhenToUse: When changing plugin protocol, adding new plugin ops, or debugging slow devctl startup
---


# Capability checking and safe plugin invocation (ops/commands/streams)

## Goal

Explain how devctl decides whether it can call a plugin operation (“capability checking”), how it actually sends requests and waits for replies (“invocation”), and where the current codebase *does not* guard calls—leading to startup delays or hangs when a plugin ignores an unsupported op.

This document is written as a debugging + design analysis. It includes:
- an inventory of every Go call site that invokes plugin operations
- the semantics of handshake capabilities vs reality
- failure modes when plugins misbehave
- a safer, reusable pattern to prevent similar issues

## Executive summary

Today, capability checking works well for the “pipeline ops” (`config.mutate`, `validate.run`, `build.run`, `prepare.run`, `launch.plan`) because the pipeline gates each call with `Client.SupportsOp(op)` (`devctl/pkg/engine/pipeline.go`).

However, capability checking is *inconsistent* for:

- **Dynamic CLI command discovery**: `devctl/cmd/devctl/cmds/dynamic_commands.go` calls `commands.list` for every plugin unconditionally (3s timeout), even when the plugin does not declare it. A plugin that ignores unknown ops will stall startup for the full timeout.
- **Dynamic command execution**: the same file calls `command.run` unconditionally when a dynamic command is invoked (risk: stall until the per-op timeout).
- **Streams**: `runtime.Client.StartStream()` sends a request without any “supports” check (and there is no `SupportsStream` helper).
- **Misc smoke tests**: `dev smoketest` commands call “ping” without checking capabilities (less important for users, but it’s the same pattern).

The root cause is structural: `runtime.Client.Call()` has no built-in “supports” check (it only implements the protocol send+wait), so every call site must remember to gate on handshake capabilities (and many don’t).

## System model: what “capabilities” are supposed to mean

### The protocol surface

The protocol types are defined in:
- `devctl/pkg/protocol/types.go`

Key shapes:

```go
type Capabilities struct {
    Ops      []string `json:"ops,omitempty"`
    Streams  []string `json:"streams,omitempty"`
    Commands []string `json:"commands,omitempty"`
}

type Handshake struct {
    Type            FrameType       `json:"type"` // "handshake"
    ProtocolVersion ProtocolVersion `json:"protocol_version"` // "v1"
    PluginName      string          `json:"plugin_name"`
    Capabilities    Capabilities    `json:"capabilities"`
}
```

Important nuance: `ValidateHandshake` only validates `{type, protocol_version, plugin_name}`:
- `devctl/pkg/protocol/validate.go`

It does **not** validate that `capabilities.ops` is present, or that the plugin’s ops are “real”.

So capabilities are a *self-declared contract* by the plugin; devctl must still use timeouts and tolerate lies/bugs.

### devctl’s “should call” contract (as documented)

The plugin authoring guide states:
> devctl will call you only for the ops you declare

This is true for the pipeline, but currently false for dynamic command discovery (`commands.list` is called unconditionally).

The relevant background doc is:
- `devctl/pkg/doc/topics/devctl-plugin-authoring.md` (protocol reference + best practices)

## Implementation model: how devctl starts plugins and makes calls

### Step 1: start plugin process + read handshake

- `devctl/pkg/runtime/factory.go`

Pseudocode:
```go
cmd := exec.CommandContext(ctx, spec.Path, spec.Args...)
cmd.Dir = spec.WorkDir
cmd.Env = mergeEnv(os.Environ(), spec.Env)
cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

stdin := cmd.StdinPipe()
stdout := cmd.StdoutPipe()
stderr := cmd.StderrPipe()

cmd.Start()
hs := readHandshake(ctx, bufio.NewReader(stdout), HandshakeTimeout)
return newClient(spec, hs, cmd, stdin, reader, stderr)
```

Key details:
- Plugin processes are placed into their own process group (`Setpgid: true`) so devctl can stop them reliably.
- Handshake reading is time-bounded (`HandshakeTimeout`, default 2s).
- After handshake, devctl runs two goroutines:
  - one reads stdout frames and routes responses/events
  - one reads stderr lines and logs them (prefixed by plugin id)

### Step 2: send request and await response

- `devctl/pkg/runtime/client.go` (`Call`)

Pseudocode:
```go
rid := nextRequestID()
router.register(rid) -> respCh

req := {type:"request", request_id: rid, op: op, ctx: requestContextFrom(ctx), input: json(input)}
writeFrame(req) // to stdin

select {
  case resp := <-respCh:
      if !resp.Ok: return error(resp.Error or generic)
      json.Unmarshal(resp.Output, output)
      return nil
  case <-ctx.Done():
      router.cancel(rid, ctx.Err())
      return ctx.Err()
}
```

Critical property:
- `Call` has **no capability check**. It will happily send a request for any `op` string.
- If the plugin ignores the request, the call waits until `ctx` times out.

### Step 3: start a stream

- `devctl/pkg/runtime/client.go` (`StartStream`)

It behaves like `Call`, except it expects the response output to contain `stream_id`, and then returns a channel of events for that stream.

Critical property:
- `StartStream` also has **no capability check**.
- There is no `SupportsStream(op)` helper; you can only check `SupportsOp` (ops list) yourself.
- The handshake also contains `Capabilities.Streams`, but the runtime does not use it at all today.

## How capability checking currently works

### The only “supports” helper: `SupportsOp`

- `devctl/pkg/runtime/client.go:68`

```go
func (c *client) SupportsOp(op string) bool {
    return contains(c.hs.Capabilities.Ops, op)
}
```

So “support” == string containment in handshake `capabilities.ops`.

No equivalent exists for:
- “supports streams”
- “supports commands.list”
- “supports command.run”

Those are expected to be represented in `capabilities.ops` too (because they are request/response operations), but the code often does not enforce that.

## Inventory: every op invocation site in the codebase

This section enumerates all non-test invocation sites found via searching for `.Call(` and `StartStream(`.

### A) Pipeline ops (good: gated by SupportsOp)

File: `devctl/pkg/engine/pipeline.go`

Each method uses the same pattern:

```go
for _, c := range orderedClients {
    if !c.SupportsOp("<op>") { continue }
    c.Call(ctx, "<op>", input, &out)
    merge(out)
}
```

Affected ops and the exact gating:
- `MutateConfig`: checks `SupportsOp("config.mutate")`
- `Build`: checks `SupportsOp("build.run")`
- `Prepare`: checks `SupportsOp("prepare.run")`
- `Validate`: checks `SupportsOp("validate.run")`
- `LaunchPlan`: checks `SupportsOp("launch.plan")`

This means:
- plugins that don’t implement an op are not called
- plugins that lie and claim an op may still hang (until ctx timeout), but only for ops they declared

### B) Dynamic CLI command discovery (bad: not gated)

File: `devctl/cmd/devctl/cmds/dynamic_commands.go`

Call site:
```go
err = c.Call(callCtx, "commands.list", map[string]any{}, &out)
```

Observations:
- There is no `SupportsOp("commands.list")` check before calling it.
- The timeout is 3 seconds (`context.WithTimeout(..., 3*time.Second)`).
- This runs at devctl startup (see `devctl/cmd/devctl/main.go`), even for commands that don’t need plugin commands.

Failure mode:
- any plugin that ignores unknown ops causes devctl startup to stall for ~3 seconds per plugin.

This is exactly what happens in the comprehensive fixture with `logger.py` (which reads stdin but never responds).

### C) Dynamic CLI command execution (bad: not gated)

Still in `devctl/cmd/devctl/cmds/dynamic_commands.go`:

When a dynamic command is invoked, devctl does:

1) start plugin
2) `MutateConfig` (pipeline) — this is gated (`config.mutate`)
3) call:
```go
err = client.Call(opCtx, "command.run", map[string]any{
    "name": name,
    "argv": argv,
    "config": conf,
}, &cmdOut)
```

There is no `SupportsOp("command.run")` check at execution time.

Risk:
- If a plugin mistakenly lists commands via `commands.list` but doesn’t actually implement `command.run` (or hangs), the CLI command blocks until the timeout.

### D) Streams (currently no “supports” gate)

The primary stream machinery is runtime-level:
- `devctl/pkg/runtime/client.go:StartStream`

Call sites inside the repo are mostly tests today. But structurally:
- any future `logs follow` command using streams must explicitly gate ops or it may hang.
- there is no direct helper to check `Capabilities.Streams`, and no guarantee `Capabilities.Streams` is even populated.

### E) Smoke tests (no gate, but test-only)

`devctl/cmd/devctl/cmds/dev/smoketest/root.go` and `smoketest_failures.go` call:
- `c.Call(ctx, "ping", ...)`

These are developer-facing test commands, not production flows, but they show the “raw Call” style that can hang if the plugin ignores requests.

## Why “capabilities” are not sufficient by themselves

Even if devctl gates calls based on handshake capabilities, the following can still occur:

1) A plugin lies (claims op support but doesn’t implement it)
2) A plugin is buggy (deadlocks, ignores stdin, crashes)
3) A plugin is slow (implementation takes longer than expected)

Therefore, devctl must also:
- always use contexts with deadlines for calls
- consider per-op timeout policies (startup calls should be short)
- make “unsupported op” a fast path (do not even send requests if we know it’s unsupported)

## Recommendations: a safer and more uniform approach

This section proposes an approach that prevents the specific failure class (“devctl calls op that plugin doesn’t support and plugin hangs”) and reduces the chance of reintroducing it.

### Recommendation 1: Centralize “support check + call” into runtime helpers

Today, the burden is on each call site to:
- remember to check `SupportsOp`
- decide what error to return if unsupported
- decide timeout policy

Instead, provide helpers in `devctl/pkg/runtime`:

Pseudocode API:
```go
var ErrUnsupportedOp = errors.New("unsupported op")

func CallIfSupported(ctx context.Context, c Client, op string, input any, output any) (called bool, err error) {
    if !c.SupportsOp(op) {
        return false, nil
    }
    return true, c.Call(ctx, op, input, output)
}

func RequireOp(c Client, op string) error {
    if !c.SupportsOp(op) {
        return errors.Errorf("plugin %q does not support op %q", c.Spec().ID, op)
    }
    return nil
}
```

Then update call sites to be explicit:
- pipeline ops: can keep current pattern (already correct), but helpers reduce boilerplate.
- dynamic commands: should use `RequireOp("commands.list")` / `RequireOp("command.run")` and skip otherwise.

### Recommendation 2: Treat stream-producing operations as ops (and gate on SupportsOp)

Even though handshake has `Capabilities.Streams`, the runtime model uses an operation name (`StartStream(ctx, op, input)`), which is structurally identical to any other op.

Minimum safe behavior:
- before `StartStream(ctx, op, ...)`, check `SupportsOp(op)` (or use `RequireOp`).

Optional stronger behavior:
- introduce `SupportsStream(op)` that checks:
  - `SupportsOp(op)` AND
  - `contains(handshake.Capabilities.Streams, op)`

This makes handshake `streams` meaningful and avoids mismatches where a plugin claims it can stream but doesn’t.

### Recommendation 3: Make dynamic command discovery opt-in and capability-gated

Dynamic command discovery is the most visible “startup tax” and the easiest place to reintroduce hangs.

Safe minimum:
- only call `commands.list` if `SupportsOp("commands.list")`

Safer still:
- only attempt dynamic command discovery if the plugin declares command support in handshake, e.g.:
  - `SupportsOp("commands.list")` OR `len(handshake.Capabilities.Commands) > 0`

Even safer (architectural):
- do not run dynamic discovery on every invocation:
  - run it only for `devctl` invocations that need it (maybe only when the user invokes an unknown command)
  - cache discovery results (requires persistent store + invalidation policy)

### Recommendation 4: Document and enforce “always respond with E_UNSUPPORTED”

The plugin authoring guide already shows returning:
```json
{"type":"response","request_id":rid,"ok":false,"error":{"code":"E_UNSUPPORTED","message":"unsupported op"}}
```

But the fixture `logger.py` violates this by never responding.

Two approaches:

1) Enforcement by convention (docs + examples):
   - all plugins must implement a default else-branch returning `E_UNSUPPORTED` quickly.
2) Enforcement by tooling:
   - add a `devctl plugins check` command that sends a small set of “must respond” ops and fails if the plugin is non-responsive within a short timeout.

This does not replace capability gating, but it reduces the chance of pathological hangs.

### Recommendation 5: Timeouts should be proportional to phase

Dynamic discovery is “startup UX”, so a 3s timeout per plugin is punishing.

Suggested policy:
- handshake: keep small (2s default already)
- command discovery: very small (250ms–500ms) and/or parallelize across plugins
- pipeline ops: default 30s ok, but per-op override might be needed (build can be long; validate should be short)

## Concrete “risk checklist” for reviewers

When reviewing new code that calls plugins, ask:

1) Does it call `Client.Call` or `Client.StartStream` directly?
2) If so, does it:
   - check `SupportsOp(op)` (or use a helper that does)?
   - bound the call with a context deadline appropriate for the operation?
3) If the plugin ignores the request, what is the worst-case user-visible stall?
4) If the plugin returns `ok=false`, is the error message actionable?

## Suggested follow-ups (implementation tasks)

Not implemented in this document, but recommended follow-ups:

1) Fix `dynamic_commands.go` to gate `commands.list` and `command.run` by `SupportsOp`.
2) Add a runtime helper (`RequireOp` / `CallIfSupported`) to eliminate repeated patterns and reduce regressions.
3) Consider adding `SupportsStream` and gating `StartStream`.
