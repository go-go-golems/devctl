---
Title: Plugin host architecture (deep dive)
Ticket: MO-015-CREATE-PLUGIN-HOST-DOCUMENTATION
Status: active
Topics:
    - plugins
    - runtime
    - concurrency
    - protocol
    - tui
    - documentation
DocType: analysis
Intent: long-term
Owners: []
RelatedFiles:
    - Path: cmd/devctl/cmds/dynamic_commands.go
      Note: Dynamic cobra commands driven by handshake capabilities.commands + command.run
    - Path: cmd/devctl/cmds/plugins.go
      Note: Handshake-based plugin introspection (plugins list)
    - Path: cmd/devctl/cmds/stream.go
      Note: 'CLI stream start: StartStream + event printing'
    - Path: cmd/devctl/cmds/tui.go
      Note: TUI wiring + concurrency via errgroup
    - Path: cmd/devctl/cmds/up.go
      Note: Concrete pipeline→supervise→state persistence lifecycle
    - Path: pkg/config/config.go
      Note: .devctl.yaml schema and config loader
    - Path: pkg/discovery/discovery.go
      Note: Discover plugin specs from config + plugins/devctl-* scan; deterministic ordering
    - Path: pkg/engine/pipeline.go
      Note: Intermediate state merge pipeline across plugins
    - Path: pkg/protocol/errors.go
      Note: Protocol error code constants
    - Path: pkg/protocol/types.go
      Note: Protocol frame definitions (handshake/request/response/event)
    - Path: pkg/protocol/validate.go
      Note: Handshake validation rules (protocol v2
    - Path: pkg/repository/repository.go
      Note: Repo.Load and StartClients/CloseClients; constructs RequestMeta
    - Path: pkg/runtime/client.go
      Note: Request/response + stream calls; stdout/stderr goroutines; cancellation behavior
    - Path: pkg/runtime/factory.go
      Note: Plugin process start + handshake read + process-group termination
    - Path: pkg/runtime/router.go
      Note: Request correlation + stream fanout + buffering/backpressure semantics
    - Path: pkg/state/state.go
      Note: Persisted .devctl/state.json schema + ProcessAlive
    - Path: pkg/supervise/supervisor.go
      Note: Service supervision + log file management + readiness checks
    - Path: pkg/tui/action_runner.go
      Note: TUI action runner (up/down/restart) publishes pipeline domain events
    - Path: pkg/tui/bus.go
      Note: In-memory Watermill bus (topics + router)
    - Path: pkg/tui/forward.go
      Note: UI forwarder injecting tea.Msg into Bubbletea
    - Path: pkg/tui/state_watcher.go
      Note: TUI state polling + snapshot publishing
    - Path: pkg/tui/stream_runner.go
      Note: TUI stream runner starts plugin streams and forwards protocol events
    - Path: pkg/tui/transform.go
      Note: Domain→UI transformer (topic devctl.events → ui.msgs)
ExternalSources: []
Summary: 'In-depth analysis of devctl''s plugin host: concurrency, protocol, lifecycle/state management, event wiring, and TUI interaction—plus lessons for a static-analysis plugin tool.'
LastUpdated: 2026-01-09T17:20:31.923905622-05:00
WhatFor: Explain precisely how devctl runs plugins and integrates them into the runtime/event/UI pipeline, so we can reuse the approach for a new static-analysis/codebase-inspection tool.
WhenToUse: Use when extending plugin lifecycle/runtime behavior, debugging plugin-related concurrency issues, or designing a new tool with the same plugin-host principle.
---


# How devctl's plugin host actually works

> *"Any sufficiently advanced CLI is indistinguishable from a local orchestrator."*

This document tells the story of how `devctl` runs plugins—not just what the code does, but why it's shaped the way it is, where the sharp edges hide, and what patterns we should steal for building a new static-analysis tool.

## The core insight: plugins are just facts

Here's the mental model that makes everything else make sense:

**Plugins don't run your dev environment. They tell devctl what to run.**

A plugin is a repo-specific adapter that knows things devctl can't know: where your Dockerfile lives, what environment variables your backend needs, which ports your services use, whether you need to run migrations before starting the API. But the plugin doesn't actually *do* any of that—it just *describes* it.

When you run `devctl up`, devctl asks each plugin a series of questions:
- "What config tweaks do you need?" → plugin returns a patch
- "Anything I should build first?" → plugin returns build steps
- "Anything to prepare?" → plugin returns preparation steps
- "Is everything valid?" → plugin returns validation results
- "What services should I run?" → plugin returns a launch plan

Then devctl—not the plugin—actually starts processes, writes log files, tracks PIDs, and manages the whole messy reality of subprocess supervision.

This separation is crucial. It means:
- Plugins can be written in any language (they just emit JSON)
- Plugins can't accidentally leave zombie processes behind
- The host owns all the operational complexity (timeouts, cancellation, cleanup)
- Testing plugins is easy: run them, check their JSON output

## Two different kinds of children

Before diving into the details, there's an important distinction to internalize: devctl manages two completely different kinds of child processes, and they have almost nothing in common.

### Plugins: short-lived advisors

Plugins are **external processes that speak a protocol over stdin/stdout**. They're started, asked questions, and then stopped. Their job is to compute facts. They typically live for seconds, not hours.

The relevant code lives in:
- `pkg/runtime/*` (process lifecycle and protocol handling)
- `pkg/protocol/*` (frame types and validation)

### Services: long-lived workloads

Services are the actual dev environment—your API server, your database, your frontend. They're started based on what plugins *said* to run, and they keep running until you stop them or they crash.

The relevant code lives in:
- `pkg/supervise/*` (process supervision)
- `pkg/state/*` (persistent state tracking)

Think of plugins as consultants who come in, give advice, and leave. Services are the employees who show up every day and do the work.

---

## Part 1: Finding and starting plugins

### Where do plugins come from?

devctl finds plugins in two places:

**1. The config file (`.devctl.yaml`)**

This is the primary source. A typical config looks like:

```yaml
plugins:
  - id: myrepo
    path: ./scripts/devctl-plugin.py
    priority: 10
  - id: docker-compose
    path: devctl-compose
    priority: 20
```

Each plugin entry specifies:
- `id`: A unique identifier (used for logging, error messages, and the `--plugin` flag)
- `path`: Either a file path or a command name
- `priority`: Lower numbers run first (matters for merge semantics—more on this later)
- Optional: `args`, `workdir`, `env`

**2. Auto-discovery in `plugins/`**

devctl also scans `<repo>/plugins/` for executables named `devctl-<something>`. If you drop a `plugins/devctl-myanalyzer` binary in your repo, it'll be discovered automatically with priority 1000 (low priority = runs last).

This is convenient for "drop-in" plugins that don't need configuration.

### The path resolution dance

There's a subtle but important rule about how paths are interpreted:

- If the path contains a `/` or `\`, it's treated as a file path and resolved relative to the repo root
- If it contains no path separators, it's treated as a command name and resolved via `$PATH` at exec time

Why does this matter? Because it means you can write:
```yaml
plugins:
  - id: myrepo
    path: python  # Resolved via $PATH
    args: ["./scripts/plugin.py"]
```

Or:
```yaml
plugins:
  - id: myrepo
    path: ./scripts/plugin.py  # Resolved relative to repo
```

Both work, but they fail differently when things go wrong. The first fails at exec time if `python` isn't in `$PATH`. The second fails at discovery time if the file doesn't exist.

### Starting a plugin process

When devctl needs a plugin, it calls `runtime.Factory.Start()`. Here's what happens:

```
1. Build exec.Cmd with spec.Path and spec.Args
2. Set working directory to spec.WorkDir (default: repo root)
3. Merge environment: os.Environ() + spec.Env
4. Set Setpgid: true (new process group—important for cleanup)
5. Create pipes for stdin, stdout, stderr
6. Start the process
7. Read the handshake (with a 2-second timeout)
8. If handshake fails: kill the process group
9. If handshake succeeds: return a Client
```

The `Setpgid: true` bit is easy to overlook but critical. It puts the plugin in its own process group, which means when we send SIGTERM, we kill the plugin *and* any children it spawned. Without this, you'd slowly accumulate orphaned processes.

---

## Part 2: The handshake dance

The handshake is the plugin's first and most important message. It happens *immediately* after the process starts, and it tells devctl everything it needs to know about what this plugin can do.

### The contract

The plugin's very first stdout line must be a JSON handshake:

```json
{
  "type": "handshake",
  "protocol_version": "v2",
  "plugin_name": "myrepo",
  "capabilities": {
    "ops": ["config.mutate", "validate.run", "launch.plan"],
    "streams": ["logs.follow"],
    "commands": [
      {"name": "db-reset", "help": "Reset the local database"}
    ]
  }
}
```

This handshake is **strictly validated**:
- `type` must be exactly `"handshake"`
- `protocol_version` must be `"v2"` (v1 is no longer supported)
- `plugin_name` must be non-empty
- Commands must have unique names and valid arg specs

If validation fails, devctl kills the plugin immediately. No negotiation, no fallback, no second chances. This strictness is intentional—it surfaces plugin bugs immediately rather than letting them manifest as mysterious runtime errors later.

### Why capabilities matter

The `capabilities.ops` list is the plugin's contract with devctl. If a plugin says it supports `["config.mutate", "launch.plan"]`, that's the *only* things devctl will ask it to do.

This allowlist approach has a subtle benefit: it makes the host's planning explicit. When devctl builds a pipeline, it knows upfront which plugins will participate in each phase. There's no "try it and see" at runtime.

### The 2-second rule

Handshake has a 2-second timeout. If your plugin doesn't emit a handshake within 2 seconds, it's killed.

This might seem harsh, but it catches a common mistake: printing something to stdout before the handshake (debug logs, a welcome banner, a shebang echo). Any non-JSON garbage on stdout is treated as protocol contamination, and the plugin is terminated.

**The rule is simple: stdout is sacred. If you want to log, use stderr.**

---

## Part 3: The protocol (how host and plugin talk)

After the handshake, communication follows a simple request/response pattern with optional streaming.

### Frame types

All messages are NDJSON—one JSON object per line. There are four frame types:

| Type | Direction | Purpose |
|------|-----------|---------|
| `handshake` | plugin → host | Announce capabilities (first line only) |
| `request` | host → plugin | Ask the plugin to do something |
| `response` | plugin → host | Answer to a request |
| `event` | plugin → host | Stream event (asynchronous) |

### Request/response basics

When devctl wants to call an op, it writes a request to the plugin's stdin:

```json
{
  "type": "request",
  "request_id": "myrepo-1",
  "op": "config.mutate",
  "ctx": {
    "repo_root": "/home/user/myproject",
    "cwd": "/home/user/myproject",
    "deadline_ms": 30000,
    "dry_run": false
  },
  "input": {"config": {}}
}
```

The plugin responds on stdout:

```json
{
  "type": "response",
  "request_id": "myrepo-1",
  "ok": true,
  "output": {"config_patch": {"services": {"api": {"port": 8080}}}}
}
```

The `request_id` is generated by the host (`<plugin_id>-<counter>`) and must be echoed back in the response. This allows the host to correlate responses with requests, even if they arrive out of order.

### Error responses

When something goes wrong, the plugin sets `ok: false` and includes an error:

```json
{
  "type": "response",
  "request_id": "myrepo-1",
  "ok": false,
  "error": {
    "code": "E_CONFIG_INVALID",
    "message": "database host is required",
    "details": {"field": "config.database.host"}
  }
}
```

The host wraps this into an `OpError` that bubbles up through the calling code.

### Streaming (the interesting part)

Some operations naturally produce incremental output—think log tailing, build progress, or analysis findings. For these, devctl supports streaming.

A stream-start request looks like any other request, but the response includes a `stream_id`:

```json
{
  "type": "response",
  "request_id": "myrepo-2",
  "ok": true,
  "output": {"stream_id": "logs-main-1"}
}
```

After that, the plugin can emit event frames for that stream:

```json
{"type": "event", "stream_id": "logs-main-1", "event": "log", "message": "Server starting..."}
{"type": "event", "stream_id": "logs-main-1", "event": "log", "message": "Listening on :8080"}
{"type": "event", "stream_id": "logs-main-1", "event": "end", "ok": true}
```

The `event: "end"` frame terminates the stream. Until it arrives, the stream is considered active.

---

## Part 4: Concurrency (where things get interesting)

Now we get to the good stuff. Plugin concurrency in devctl is carefully designed to be predictable, but there are subtle gotchas that will bite you if you don't understand the model.

### The goroutine topology

For each active plugin client, devctl runs exactly two goroutines:

**1. The stdout reader** (`readStdoutLoop`)

This goroutine sits in a loop reading lines from stdout. For each line:
- Parse the JSON envelope (just enough to get the `type` field)
- If it's a `response`: find the pending request channel, deliver the response, close the channel
- If it's an `event`: fan it out to all subscribers of that `stream_id`
- If it's a `handshake` or `request`: that's a protocol violation—fail everything

**2. The stderr reader** (`readStderrLoop`)

This goroutine reads stderr lines and logs them with the plugin ID prefix. Simple, but important for debugging.

### The router: request correlation and stream fanout

The `router` is the heart of the concurrency model. It's a mutex-protected data structure that tracks:

- `pending`: a map from `request_id` → response channel (buffer size 1)
- `streams`: a map from `stream_id` → list of subscriber channels
- `buffer`: events that arrived before anyone subscribed (a "mailbox" pattern)
- `fatal`: the first error that killed the reader

When a response arrives, the router delivers it to the waiting channel and closes it. When an event arrives, it's fanned out to all subscribers.

### The backpressure trap

Here's the most important concurrency detail in the whole system:

**Stream event fanout is synchronous and unbuffered (well, buffer-16).**

Each subscriber channel has a buffer of 16 events. When the router publishes an event, it does `ch <- ev` for each subscriber. If a subscriber's buffer is full, the send blocks.

The consequences:

1. If your subscriber stops consuming, its buffer fills
2. Once the buffer is full, `router.publish()` blocks
3. That blocks the stdout reader goroutine
4. That blocks *all* response processing for that plugin
5. The plugin's stdout pipe fills up and blocks the plugin itself

This is a deliberate design choice: **correctness over lossy sampling**. Events are never dropped, ordering is preserved, and if you can't keep up, everything blocks until you can.

For typical devctl use cases (log tailing, build progress), this is fine—the subscriber drains fast enough. But if you're building a high-frequency stream (think: 1000s of events per second), you need to either drain faster or implement your own lossy sampling layer on top.

### Cancellation semantics

What happens when a request times out or is canceled?

1. The calling goroutine's `ctx.Done()` fires
2. The caller calls `router.cancel(rid, ctx.Err())`
3. `cancel` removes the pending entry and sends a synthetic `E_CANCELED` response
4. The channel is closed

If the plugin eventually sends a real response for that `request_id`, it's simply dropped—there's no pending channel to deliver it to.

### Fatal errors: fail everything

If the stdout reader encounters an error (EOF, invalid JSON, unexpected frame type), it enters "fail everything" mode:

1. Set `fatal` error on the router
2. Send `E_RUNTIME` responses to all pending requests
3. Close all stream subscriber channels
4. Exit the goroutine

This means a single protocol violation takes down the whole plugin client. That sounds aggressive, but it's actually what you want—if the protocol is corrupted, you can't trust anything else from that plugin.

---

## Part 5: The pipeline (how plugins compose)

Individual plugins are useful, but the real power comes from combining multiple plugins. devctl's pipeline is how this composition happens.

### Sequential, deterministic execution

The pipeline runs plugins **sequentially, in priority order**. For each phase (config.mutate, build.run, validate.run, etc.):

1. Sort plugins by (priority, id)
2. For each plugin that supports the op:
   - Call the op with the current state
   - Merge the result into the accumulated state
3. Move to the next phase

There's no parallel plugin execution within a phase. This is intentional—it makes debugging reproducible and avoids merge conflicts.

### The intermediate state

The pipeline maintains several pieces of intermediate state:

**Config** (`patch.Config`)

Starts as an empty map. Each plugin that supports `config.mutate` returns a patch, and patches are applied sequentially. Later plugins see the config as modified by earlier plugins.

This is powerful: a "base" plugin (priority 10) might set up default ports, and a "project" plugin (priority 100) might override specific values.

**Validation results**

Errors and warnings are concatenated. The combined `valid` flag is the AND of all plugins' `valid` flags—any plugin can veto the launch.

**Launch plan**

Services are collected into a list. Name collisions are resolved by strictness:
- In **strict mode**: collision is an error
- In **non-strict mode**: later plugin overwrites earlier (last writer wins)

Because ordering is deterministic, this gives predictable behavior.

### Why sequential matters

You might wonder: why not run plugins in parallel for speed?

The answer is merge semantics. If two plugins both try to configure the same service, which one wins? With sequential execution, it's always "later plugin wins", which is predictable. With parallel execution, it's "whoever finishes last", which is a race condition.

For most repos, plugin calls take milliseconds—the sequential overhead is negligible. For repos with expensive plugin operations, the right answer is to make plugins faster, not to add parallel complexity.

---

## Part 6: The event system (connecting plugins to UI)

So far we've talked about the plugin protocol (how the host talks to plugins) and the pipeline (how plugin outputs are combined). Now let's talk about how all of this connects to the UI.

### The problem: decoupling

The TUI needs to show what's happening—pipeline phases starting and finishing, services going up and down, logs streaming in. But we don't want the UI code reaching directly into the runtime, and we don't want the runtime knowing about Bubbletea.

The solution is an event bus.

### Watermill: the in-memory bus

devctl uses [Watermill](https://watermill.io/) with an in-memory `gochannel` backend. All messages flow through three topics:

| Topic | Purpose |
|-------|---------|
| `devctl.ui.actions` | UI → runners (start/stop requests) |
| `devctl.events` | Domain events (pipeline phases, state changes, stream events) |
| `devctl.ui.msgs` | UI-ready messages (for Bubbletea) |

### The message flow

Here's how a user pressing "u" (for "up") becomes a running dev environment:

```
1. User presses 'u' in the TUI
2. Dashboard model emits ActionRequestMsg{Kind: "up"}
3. RootModel publishes to TopicUIActions
4. ActionRunner receives the message
5. ActionRunner runs the pipeline (start plugins, run phases, supervise)
6. At each phase, ActionRunner publishes to TopicDevctlEvents:
   - PipelinePhaseStarted
   - PipelinePhaseFinished
   - PipelineBuildResult, etc.
7. Domain-to-UI Transformer receives these, converts to UI messages
8. Transformer publishes to TopicUIMessages
9. UI Forwarder receives these, calls program.Send(msg)
10. Bubbletea's Update loop receives the message
11. UI re-renders with new state
```

This might seem like a lot of indirection, but each step has a purpose:
- The runner doesn't know about Bubbletea
- The transformer handles format conversion in one place
- The forwarder is the only thing calling `program.Send`

### How streams connect

Stream events follow a similar path:

```
1. StreamRunner starts a plugin stream
2. Plugin emits event frames
3. StreamRunner reads events, publishes StreamEvent to TopicDevctlEvents
4. Transformer converts to UI message
5. Forwarder injects into Bubbletea
6. StreamsModel updates its display
```

The key insight: **plugin protocol events become domain events become UI messages**. The plugin doesn't know about the UI, and the UI doesn't know about the protocol.

### The StateWatcher: polling as events

One interesting design choice: the TUI doesn't maintain long-lived connections to service state. Instead, it polls.

The `StateWatcher` runs in a background goroutine:
1. Every second, read `.devctl/state.json`
2. Check `ProcessAlive` for each service
3. Run health checks (TCP/HTTP)
4. Publish a `StateSnapshot` to the event bus

This polling approach is simple and robust. If the TUI crashes, nothing bad happens—there's no persistent connection to clean up. If devctl was run from the CLI (not TUI), the state file is still there for the next run.

---

## Part 7: Lessons for building a new tool

We're building this documentation because we want to reuse these patterns for a static-analysis/codebase-inspection tool. Here's what to steal and what to change.

### What to steal

**The plugin protocol**

NDJSON over stdin/stdout is a fantastic choice:
- Works in any language
- Easy to debug (just print the JSON)
- No dependencies (no gRPC, no HTTP server)
- Streaming-friendly

The handshake + allowlist pattern is also worth keeping. It makes the host's behavior explicit and catches plugin bugs early.

**The request/response correlation**

The `request_id` pattern with one-shot channels is clean and handles cancellation gracefully. The router pattern (pending map + stream subscriptions) is reusable.

**The deterministic pipeline**

Priority-ordered sequential execution with explicit merge semantics is boring in the best way. It's predictable, debuggable, and correct.

**The event bus architecture**

Topics + envelopes + transformers + forwarders is a clean way to decouple domain logic from UI. Watermill is maybe overkill for in-memory use, but the pattern is sound.

### What to change

**Consider long-lived plugins**

devctl starts a fresh plugin process for each operation and kills it when done. This is fine for quick operations (config, planning), but expensive for static analysis.

For an analysis tool, consider:
- Keeping plugin processes alive for a session
- Adding a "warmup" phase where plugins load caches
- Potentially running a plugin server that persists across runs

**Rethink backpressure**

Static analysis can generate thousands of findings. The current backpressure model (block everything) might not be right.

Options:
- Bounded queues with explicit drop + metrics
- Separate "critical" events (errors) from "telemetry" (progress)
- Client-side buffering with periodic flush

**Define finding merge semantics explicitly**

Multiple analyzers might report the same issue. You need rules:
- Dedupe by (file, line, message)?
- Keep all and let the UI dedupe?
- Aggregate severity (max? sum?)

This needs to be explicit, not emergent.

### A sketch of the analysis API

Here's a possible plugin API for static analysis:

```json
{
  "type": "handshake",
  "protocol_version": "v2",
  "plugin_name": "my-linter",
  "capabilities": {
    "ops": [
      "analysis.run",
      "analysis.files",
      "command.run"
    ],
    "streams": [
      "analysis.findings",
      "analysis.progress"
    ],
    "commands": [
      {"name": "lint", "help": "Run the linter"}
    ]
  }
}
```

**Ops:**
- `analysis.files`: Return list of files this analyzer cares about
- `analysis.run`: Run full analysis (batch mode)
- `command.run`: Dynamic CLI commands

**Streams:**
- `analysis.findings`: Incremental findings as they're discovered
- `analysis.progress`: Progress events (percent, current file)

**Events:**
```json
{"type": "event", "stream_id": "...", "event": "finding", "fields": {
  "file": "src/main.go",
  "line": 42,
  "column": 10,
  "severity": "warning",
  "rule": "unused-variable",
  "message": "x is declared but never used"
}}

{"type": "event", "stream_id": "...", "event": "progress", "fields": {
  "percent": 45,
  "phase": "analyzing",
  "current_file": "src/main.go"
}}

{"type": "event", "stream_id": "...", "event": "end", "ok": true}
```

---

## Closing thoughts

The plugin host architecture in devctl is surprisingly sophisticated for what looks like a simple developer tool. The key insights:

1. **Plugins are pure functions that emit facts.** The host handles all the messy operational stuff.

2. **The protocol is intentionally boring.** NDJSON, strict validation, obvious failure modes. This is a feature.

3. **Concurrency is explicit.** Two goroutines per plugin, mutex-protected routing, no hidden magic.

4. **Composition is deterministic.** Priority ordering, sequential execution, explicit merge rules.

5. **The UI is decoupled via events.** Domain events transform to UI messages, and the forwarder is the only thing touching Bubbletea.

These patterns compose into a system that's understandable, debuggable, and—most importantly—correct. That's the kind of foundation we want to build on.

---

*This document was produced by tracing the actual code in `pkg/runtime/*`, `pkg/protocol/*`, `pkg/tui/*`, `pkg/engine/*`, and `cmd/devctl/cmds/*`. Every claim maps to real code. If something seems wrong, check the source.*
