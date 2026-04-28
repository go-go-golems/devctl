---
Title: ""
Ticket: ""
Status: ""
Topics: []
DocType: ""
Intent: ""
Owners: []
RelatedFiles:
    - Path: cmd/devctl/cmds/dynamic_commands.go
      Note: Cobra integration - stays in devctl
    - Path: pkg/discovery/discovery.go
      Note: Plugin discovery - candidate for pluggable/registry extraction
    - Path: pkg/engine/pipeline.go
      Note: Domain-specific orchestration - stays in devctl
    - Path: pkg/protocol/types.go
      Note: Protocol frame types - candidate for pluggable/protocol extraction
    - Path: pkg/repository/repository.go
      Note: Repository lifecycle - candidate for pluggable/registry extraction
    - Path: pkg/runtime/client.go
      Note: Client interface + routing - candidate for pluggable/transport extraction
    - Path: pkg/runtime/factory.go
      Note: Process transport - candidate for pluggable/transport extraction
    - Path: pkg/runtime/router.go
      Note: Request/response routing - candidate for pluggable/transport extraction
ExternalSources: []
Summary: ""
LastUpdated: 0001-01-01T00:00:00Z
WhatFor: ""
WhenToUse: ""
---


# Extracting a Reusable Plugin Framework from devctl

## Executive Summary

devctl's plugin system is a well-designed **stdio-based plugin host** with process isolation, NDJSON framing, handshake-based capability discovery, request/response routing, and event streaming. Most of this infrastructure is generic and could be extracted into a reusable Go library (`go-plugin-host`, `pluggable`, or similar). This document analyzes the current architecture, identifies what is generic vs. devctl-specific, and proposes a layered framework design with concrete API sketches.

## Current Architecture

devctl's plugin stack has roughly 5 layers:

```
┌─────────────────────────────────────────┐
│  CLI Integration (dynamic_commands.go)  │  ← devctl-specific: Cobra registration
├─────────────────────────────────────────┤
│  Pipeline / Engine (pkg/engine)         │  ← devctl-specific: config.mutate,
│                                         │    launch.plan, validate.run, ...
├─────────────────────────────────────────┤
│  Repository (pkg/repository)            │  ← partially generic
├─────────────────────────────────────────┤
│  Discovery (pkg/discovery)              │  ← partially generic
├─────────────────────────────────────────┤
│  Runtime / Transport (pkg/runtime)      │  ← mostly generic
├─────────────────────────────────────────┤
│  Protocol (pkg/protocol)                │  ← mostly generic
└─────────────────────────────────────────┘
```

### What's Tightly Coupled to devctl

| Component | Coupling | Why |
|-----------|----------|-----|
| `RequestContext` | Strong | Fields: `RepoRoot`, `Cwd`, `DryRun` — pure devctl domain |
| `Capabilities.Commands` | Medium | Only needed for Cobra dynamic command registration |
| Pipeline ops | Strong | `config.mutate`, `launch.plan`, `validate.run`, `build.run`, `prepare.run`, `command.run` |
| `patch.Config` | Strong | Engine passes devctl config objects to plugins |
| Result types (`LaunchPlan`, `BuildResult`, etc.) | Strong | Domain-specific structs |
| `plugins/` auto-scan | Medium | Hardcoded `devctl-*` prefix convention |
| Dynamic Cobra command wiring | Strong | `cmd/devctl/cmds/dynamic_commands.go` |

### What's Generic

| Component | Generic Value |
|-----------|--------------|
| NDJSON frame protocol | Any stdio plugin needs framing |
| Handshake + capability discovery | Self-describing plugins are universally useful |
| Request/response routing | Multiplexing over single stdio pair |
| Event streaming | Pub/sub for long-running operations |
| Process lifecycle | Spawn, handshake, graceful shutdown, process groups |
| Plugin discovery (config + filesystem) | Most plugin systems need this |
| Priority-based ordering | Deterministic execution order |

## Proposed Framework: `pluggable`

A new Go module at `github.com/go-go-golems/pluggable` with three layers:

```
pluggable/
  protocol/     # Generic NDJSON frame types
  transport/    # Stdio process transport + routing
  registry/     # Discovery, specs, lifecycle management
```

Applications (like devctl) build on top of these layers, adding their domain-specific pipeline and CLI integration.

### Layer 1: `protocol/` — Generic Frame Protocol

Keep the core frame types but make them application-agnostic. The key change: **Capabilities become extensible**.

```go
package protocol

type FrameType string
const (
    FrameHandshake FrameType = "handshake"
    FrameRequest   FrameType = "request"
    FrameResponse  FrameType = "response"
    FrameEvent     FrameType = "event"
)

type Handshake struct {
    Type            FrameType       `json:"type"`
    ProtocolVersion string          `json:"protocol_version"`
    PluginName      string          `json:"plugin_name"`
    Capabilities    Capabilities    `json:"capabilities"`
    Declares        map[string]any  `json:"declares,omitempty"`
}

type Capabilities struct {
    Ops      []string        `json:"ops,omitempty"`      // generic: what ops does this plugin handle?
    Streams  []string        `json:"streams,omitempty"`  // generic: what streams does it emit?
    Metadata map[string]any  `json:"metadata,omitempty"` // extensible: app-specific capability blobs
}

type RequestContext map[string]any  // generic bag, app defines its own schema

type Request struct {
    Type      FrameType       `json:"type"`
    RequestID string          `json:"request_id"`
    Op        string          `json:"op"`
    Ctx       RequestContext  `json:"ctx"`
    Input     json.RawMessage `json:"input,omitempty"`
}

type Response struct {
    Type      FrameType       `json:"type"`
    RequestID string          `json:"request_id"`
    Ok        bool            `json:"ok"`
    Output    json.RawMessage `json:"output,omitempty"`
    Warnings  []Note          `json:"warnings,omitempty"`
    Notes     []Note          `json:"notes,omitempty"`
    Error     *Error          `json:"error,omitempty"`
}

type Event struct {
    Type     FrameType      `json:"type"`
    StreamID string         `json:"stream_id"`
    Event    string         `json:"event"`
    Level    string         `json:"level,omitempty"`
    Message  string         `json:"message,omitempty"`
    Fields   map[string]any `json:"fields,omitempty"`
    Ok       *bool          `json:"ok,omitempty"`
}

type Error struct {
    Code    string         `json:"code"`
    Message string         `json:"message"`
    Details map[string]any `json:"details,omitempty"`
}

type Note struct {
    Level   string `json:"level"`
    Message string `json:"message"`
}
```

**Design decision:** Drop devctl's `Commands` from the base `Capabilities`. If an app needs command registration, it can store command specs in `Capabilities.Metadata["commands"]` or define its own `Handshake` wrapper.

### Layer 2: `transport/` — Stdio Process Transport

This layer is almost a direct extraction of `pkg/runtime`, with minor generalizations.

```go
package transport

// PluginSpec is the generic descriptor for a plugin executable.
type PluginSpec struct {
    ID       string
    Path     string           // executable path
    Args     []string         // args to pass
    Env      map[string]string
    WorkDir  string
    Priority int              // lower = earlier in ordered execution
}

type Client interface {
    Spec() PluginSpec
    Handshake() protocol.Handshake
    SupportsOp(op string) bool
    SupportsStream(stream string) bool
    Call(ctx context.Context, op string, input any, output any) error
    StartStream(ctx context.Context, op string, input any) (streamID string, events <-chan protocol.Event, err error)
    Close(ctx context.Context) error
}

type Factory struct { /* ... */ }

func NewFactory(opts FactoryOptions) *Factory

func (f *Factory) Start(ctx context.Context, spec PluginSpec) (Client, error)

type FactoryOptions struct {
    HandshakeTimeout time.Duration
    ShutdownTimeout  time.Duration
}
```

**Key internal components extracted as-is:**

- `client` → `transport.client` (stdio I/O, handshake, frame parsing)
- `router` → `transport.router` (request/response matching, event pub/sub, buffering)
- `readHandshake` → stays internal
- `terminateProcessGroup` → stays internal, uses `SysProcAttr{Setpgid: true}`

**What's removed or made configurable:**
- `RequestMeta` (RepoRoot, Cwd, DryRun) → removed; `RequestContext` is now `map[string]any` and the **host application** is responsible for populating it before each `Call()`
- `StartOptions.Meta` → removed; replaced by a `BeforeCall` hook or explicit `RequestContext` construction at the application layer

### Layer 3: `registry/` — Discovery and Lifecycle

This combines devctl's `pkg/discovery` + `pkg/repository` into a generic plugin registry.

```go
package registry

// SpecSource discovers PluginSpecs from some configuration.
type SpecSource interface {
    Discover(ctx context.Context) ([]transport.PluginSpec, error)
}

// ConfigSource loads plugin declarations from a config file.
// The application provides the concrete config schema.
type ConfigSource struct {
    Load    func() ([]ConfigPlugin, error)  // app-defined
    RepoRoot string
}

type ConfigPlugin struct {
    ID       string
    Path     string
    Args     []string
    Priority int
    WorkDir  string
    Env      map[string]string
}

// FilesystemScanner auto-discovers plugins in a directory.
type FilesystemScanner struct {
    Dir            string
    Prefix         string   // e.g. "devctl-"
    RequireExecutable bool
}

func (s *FilesystemScanner) Discover(ctx context.Context) ([]transport.PluginSpec, error)

// Registry manages a set of plugins: discovery → start → ordered access → shutdown.
type Registry struct {
    factory  *transport.Factory
    sources  []SpecSource
    clients  []transport.Client
    byID     map[string]transport.Client
}

func NewRegistry(factory *transport.Factory, sources ...SpecSource) *Registry

func (r *Registry) Load(ctx context.Context) error   // discover + start all
func (r *Registry) Close(ctx context.Context) error  // shutdown all
func (r *Registry) Clients() []transport.Client      // in priority order
func (r *Registry) ByID(id string) (transport.Client, bool)
func (r *Registry) Filter(pred func(transport.Client) bool) []transport.Client
```

**Design decision:** The `Registry` replaces both `repository.Repository` and `repository.StartClients()`. It owns the full lifecycle. Applications configure it with one or more `SpecSource`s.

### What Stays in devctl

After extraction, devctl keeps its domain-specific code:

```
devctl/
  pkg/protocol/          # DELETE — use pluggable/protocol
  pkg/runtime/           # DELETE — use pluggable/transport
  pkg/discovery/         # DELETE — use pluggable/registry
  pkg/repository/        # DELETE — use pluggable/registry

  pkg/config/config.go   # KEEP — devctl-specific config schema
  pkg/engine/            # KEEP — domain pipeline (config.mutate, launch.plan, ...)
  cmd/devctl/cmds/dynamic_commands.go  # KEEP — Cobra integration
```

devctl's `engine.Pipeline` would become a thin wrapper:

```go
package engine

import (
    "github.com/go-go-golems/pluggable/registry"
    "github.com/go-go-golems/pluggable/transport"
)

type Pipeline struct {
    Registry *registry.Registry
    Opts     Options
}

func (p *Pipeline) MutateConfig(ctx context.Context, cfg patch.Config) (patch.Config, error) {
    current := cfg
    for _, c := range p.Registry.Clients() {
        if !c.SupportsOp("config.mutate") {
            continue
        }
        // Construct devctl-specific RequestContext
        reqCtx := transport.RequestContext{
            "repo_root": p.Opts.RepoRoot,
            "cwd":       p.Opts.Cwd,
            "dry_run":   p.Opts.DryRun,
        }
        var out struct { ConfigPatch patch.ConfigPatch `json:"config_patch"` }
        if err := c.Call(ctx, "config.mutate", map[string]any{"config": current}, &out); err != nil {
            return nil, err
        }
        // ...
    }
    return current, nil
}
```

The `RequestContext` construction moves from `transport.requestContextFrom()` into the application layer, which is the correct separation.

## API Sketch: What a Consumer Looks Like

Here's what another application (say, a build tool or a deploy tool) would look like:

```go
package main

import (
    "context"
    "time"

    "github.com/go-go-golems/pluggable/registry"
    "github.com/go-go-golems/pluggable/transport"
)

func main() {
    ctx := context.Background()

    factory := transport.NewFactory(transport.FactoryOptions{
        HandshakeTimeout: 2 * time.Second,
        ShutdownTimeout:  2 * time.Second,
    })

    reg := registry.NewRegistry(factory,
        // Config-based plugins
        &registry.ConfigSource{
            Load:     loadMyConfigPlugins,
            RepoRoot: ".",
        },
        // Auto-scan ./plugins/myapp-*
        &registry.FilesystemScanner{
            Dir:               "./plugins",
            Prefix:            "myapp-",
            RequireExecutable: true,
        },
    )

    if err := reg.Load(ctx); err != nil {
        log.Fatal(err)
    }
    defer reg.Close(ctx)

    for _, c := range reg.Clients() {
        if c.SupportsOp("greet") {
            var out struct{ Message string `json:"message"` }
            _ = c.Call(ctx, "greet",
                map[string]any{"name": "world"},
                &out,
                transport.WithRequestContext(map[string]any{"trace_id": "abc123"}),
            )
            fmt.Println(out.Message)
        }
    }
}
```

## Design Decisions

### 1. Keep stdio, don't add gRPC/HTTP

**Rationale:** devctl's stdio approach is its strength. It's simple, debuggable (`strace`, `tee`), works over SSH, requires no port management, and has zero dependency footprint for plugin authors. gRPC would add protobuf complexity and port conflicts. HTTP would require address allocation and health checking. stdio is the right default for CLI-adjacent tools.

### 2. Process-based, not in-process

**Rationale:** Process isolation is a feature. Plugins can crash without bringing down the host. They can be written in any language. The cost (fork/exec + JSON marshaling) is acceptable for CLI tools where latency requirements are human-scale.

### 3. Generic `RequestContext` as `map[string]any`

**Rationale:** Making `RequestContext` a typed struct in the framework would force every application to fork or wrap the protocol. A `map[string]any` with helper functions for common fields (`deadline_ms`, `trace_id`) gives maximum flexibility. Applications cast to their own types.

### 4. Keep `Capabilities` simple, use `Metadata` for extensions

**Rationale:** `Ops` and `Streams` are universal. Everything else goes in `Metadata`. This prevents the base protocol from accumulating domain-specific fields over time.

### 5. Registry owns lifecycle, not the application

**Rationale:** `repository.Repository` + manual `StartClients()` is error-prone. A `Registry.Load()` / `Registry.Close()` pattern is easier to get right and mirrors patterns from `sql.DB`, `http.Server`, etc.

### 6. Expose `Router` or keep it internal?

**Decision:** Keep `router` internal. The `transport.Client` interface is the public API. The routing implementation is an implementation detail. If an application needs custom routing (e.g., persistent connections, reconnect logic), they can implement `transport.Client` themselves.

## Alternatives Considered

### A. HashiCorp go-plugin

HashiCorp's `go-plugin` is the closest existing solution. It supports both RPC (net/rpc, gRPC) and stdio protocols. However:
- It's heavier (~5k LOC vs devctl's ~1k)
- It uses a custom plugin loading model (Go plugin build tags, or external processes with go-plugin protocol)
- Its stdio support is secondary to gRPC
- It doesn't have built-in discovery or registry concepts
- It uses a different handshake format

**Verdict:** Not a drop-in replacement. devctl's system is intentionally lighter and more language-agnostic.

### B. Extract as a subpackage, not a new module

Keep the code in `devctl/pkg/plugin/...` and let other go-go-golems projects vendor it.

**Verdict:** A new module is cleaner. It establishes a stable API boundary, enables independent versioning, and makes it clear what is generic vs. what is devctl-specific. The cost (another module to maintain) is acceptable given the reusable nature of the code.

### C. Add WASM support

Instead of spawning processes, load WASM modules in-process.

**Verdict:** Out of scope. WASM is promising but adds a heavy dependency (wazero, wasmer, etc.) and loses the "any language" benefit. Could be a future `pluggable-wasm` adapter implementing the same `transport.Client` interface.

## Implementation Plan

### Phase 1: Extract Protocol (1-2 days)
1. Create `github.com/go-go-golems/pluggable` repo
2. Copy `pkg/protocol` → `pluggable/protocol`, generalize types
3. Add unit tests for frame marshaling/unmarshaling
4. Tag v0.1.0

### Phase 2: Extract Transport (2-3 days)
1. Copy `pkg/runtime` → `pluggable/transport`
2. Remove `RequestMeta`, replace with generic `RequestContext`
3. Refactor `Factory.Start()` to drop `StartOptions`
4. Make `router` internal (unexported)
5. Add integration tests with mock plugin executables (shell scripts that emit JSON)
6. Tag v0.2.0

### Phase 3: Extract Registry (1-2 days)
1. Design `registry.SpecSource` interface
2. Port `pkg/discovery` → `registry.ConfigSource` + `registry.FilesystemScanner`
3. Port `pkg/repository` → `registry.Registry`
4. Add tests for priority ordering, duplicate ID handling, error cases
5. Tag v0.3.0

### Phase 4: Migrate devctl (2-3 days)
1. Replace devctl's `pkg/protocol`, `pkg/runtime`, `pkg/discovery`, `pkg/repository` with `pluggable` imports
2. Update `engine.Pipeline` to construct its own `RequestContext`
3. Update `dynamic_commands.go` to use `registry.Registry`
4. Verify all tests pass
5. Tag devctl release

### Phase 5: Documentation and Examples (1-2 days)
1. Write `pluggable` README with quickstart
2. Add example plugin in Python (the "any language" promise)
3. Add example host application
4. Write architecture decision record (ADR)

## Open Questions

1. **Should `pluggable` support plugin reloading?** devctl doesn't currently support hot-reload. Adding it to the framework would require watching the `plugins/` directory and gracefully restarting changed plugins. This could be a Phase 3+ feature.

2. **Should the framework include a plugin SDK?** A companion module (`pluggable-sdk-go`?) with helper functions for writing Go plugins (handshake emission, request dispatch loop, op registration) would lower the barrier for plugin authors.

3. **How to handle plugin versioning?** The handshake has `protocol_version` but no plugin version. Should `Declares` include a `version` field convention?

4. **Security model?** Currently plugins are trusted (they run with host permissions). Should the framework support sandboxing hints (e.g., `Seccomp`, `Landlock`, `pledge`)? Probably out of scope for v1.

## Related Files

- `/home/manuel/workspaces/2026-04-28/update-devctl/devctl/pkg/protocol/types.go` — Protocol structs
- `/home/manuel/workspaces/2026-04-28/update-devctl/devctl/pkg/runtime/factory.go` — Process spawning
- `/home/manuel/workspaces/2026-04-28/update-devctl/devctl/pkg/runtime/client.go` — Client interface + I/O loops
- `/home/manuel/workspaces/2026-04-28/update-devctl/devctl/pkg/runtime/router.go` — Request/response routing
- `/home/manuel/workspaces/2026-04-28/update-devctl/devctl/pkg/discovery/discovery.go` — Plugin discovery
- `/home/manuel/workspaces/2026-04-28/update-devctl/devctl/pkg/repository/repository.go` — Repository management
- `/home/manuel/workspaces/2026-04-28/update-devctl/devctl/pkg/engine/pipeline.go` — Domain-specific orchestration
- `/home/manuel/workspaces/2026-04-28/update-devctl/devctl/cmd/devctl/cmds/dynamic_commands.go` — CLI integration
