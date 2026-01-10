---
Title: Research Plan - Runtime Plugin Introspection
Ticket: RUNTIME-PLUGIN-INTROSPECTION
Status: active
Topics:
    - devctl
    - plugins
    - introspection
DocType: design
Intent: long-term
Owners: []
RelatedFiles:
    - Path: pkg/tui/state_watcher.go
      Note: Currently reads plugin config but doesn't introspect capabilities
    - Path: pkg/tui/state_events.go
      Note: PluginSummary struct - has Ops/Streams fields but never populated
    - Path: pkg/repository/repository.go
      Note: StartClients() starts plugins and captures handshake
    - Path: pkg/runtime/factory.go
      Note: Factory.Start() reads handshake from plugin stdout
    - Path: pkg/runtime/client.go
      Note: Client interface exposes Handshake() for capability access
    - Path: pkg/protocol/types.go
      Note: Handshake struct with Capabilities (Ops, Streams, Commands)
    - Path: pkg/discovery/discovery.go
      Note: Static plugin discovery from .devctl.yaml config
    - Path: pkg/tui/action_runner.go
      Note: Uses StartClients() - example of runtime introspection pattern
    - Path: pkg/tui/models/plugin_model.go
      Note: Displays plugin info including Ops/Streams (if populated)
ExternalSources: []
Summary: Research plan for implementing runtime plugin introspection to discover plugin capabilities (ops, streams, commands) for TUI display.
LastUpdated: 2026-01-08
WhatFor: Guide research and implementation of plugin capability discovery.
WhenToUse: When working on plugin introspection feature.
---

# Research Plan: Runtime Plugin Introspection

## Problem Statement

The devctl TUI needs to display plugin capabilities (ops, streams, commands) to enable features like:
- Stream indicator on plugin rows (📊 stream)
- Quick-start stream picker showing available stream ops
- Command palette showing available plugin commands
- Op discovery for better error messages

Currently, the `PluginSummary` struct has `Ops` and `Streams` fields but they are **never populated** because the state watcher only reads static config without starting plugins.

## Current Architecture

### Data Flow Diagram

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                            STATIC DISCOVERY                                  │
│                                                                              │
│  .devctl.yaml ──► config.LoadOptional() ──► discovery.Discover()            │
│                                                                              │
│                           ↓                                                  │
│                                                                              │
│                   []runtime.PluginSpec                                       │
│                   (ID, Path, Args, Env, Priority)                            │
│                                                                              │
│                   No Ops, No Streams, No Commands                            │
│                   (These require runtime handshake)                          │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────────┐
│                         STATE WATCHER (TUI)                                  │
│                                                                              │
│  StateWatcher.readPlugins() ──► Reads config.File.Plugins                   │
│                                                                              │
│                           ↓                                                  │
│                                                                              │
│                   []PluginSummary                                            │
│                   {ID, Path, Priority, Status,                               │
│                    Ops: nil, Streams: nil}  ◄── NEVER POPULATED             │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────────┐
│                        RUNTIME INTROSPECTION                                 │
│                        (Only during operations)                              │
│                                                                              │
│  action_runner.go / up.go / plan.go / stream.go                             │
│                                                                              │
│       repo.StartClients() ──► factory.Start() ──► readHandshake()           │
│                                                                              │
│                           ↓                                                  │
│                                                                              │
│                   runtime.Client with Handshake()                            │
│                   {PluginName, Capabilities:                                 │
│                    {Ops: [...], Streams: [...], Commands: [...]}}            │
│                                                                              │
│                   ⚠️ Client only lives during operation                      │
│                   ⚠️ Handshake data not persisted or cached                  │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Key Files and Their Roles

| File | Role | Gap |
|------|------|-----|
| `pkg/tui/state_watcher.go` | Periodic state polling | `readPlugins()` only reads config, no handshake |
| `pkg/tui/state_events.go` | `PluginSummary` struct | Has `Ops`/`Streams` fields, never populated |
| `pkg/repository/repository.go` | `Load()` and `StartClients()` | `StartClients()` gets handshake but is operation-scoped |
| `pkg/runtime/factory.go` | `Factory.Start()` | Reads handshake, but client is short-lived |
| `pkg/runtime/client.go` | `Client` interface | `Handshake()` method returns capabilities |
| `pkg/protocol/types.go` | `Handshake` struct | Defines `Capabilities{Ops, Streams, Commands}` |
| `pkg/discovery/discovery.go` | Static plugin discovery | No runtime component |

### Protocol Handshake Structure

```go
// pkg/protocol/types.go
type Handshake struct {
    Type            FrameType       `json:"type"`              // "handshake"
    ProtocolVersion ProtocolVersion `json:"protocol_version"`  // "v1" or "v2"
    PluginName      string          `json:"plugin_name"`
    Capabilities    Capabilities    `json:"capabilities"`
    Declares        map[string]any  `json:"declares,omitempty"`
}

type Capabilities struct {
    Ops      []string      `json:"ops,omitempty"`       // Available operations
    Streams  []string      `json:"streams,omitempty"`   // Stream-producing ops
    Commands []CommandSpec `json:"commands,omitempty"`  // Interactive commands
}
```

## Why Repository Init Doesn't Solve This

The user expected `repository.Load()` to introspect plugins, but it doesn't because:

1. **`repository.Load()` is synchronous and fast** - It only reads config and validates paths
2. **`StartClients()` is separate** - It starts plugins but is called explicitly for operations
3. **No caching** - Handshake data isn't cached between operations
4. **TUI lifecycle is different** - StateWatcher polls periodically but never starts plugins

```go
// pkg/repository/repository.go
func Load(opts Options) (*Repository, error) {
    // ... reads config, runs discovery ...
    // NO plugin startup, NO handshake reading
    return &Repository{
        Specs: specs,  // Static PluginSpec, no capabilities
    }, nil
}

// This exists but is only called during operations (up, plan, stream, etc)
func (r *Repository) StartClients(ctx context.Context, factory *runtime.Factory) ([]runtime.Client, error) {
    for _, spec := range r.Specs {
        c, err := factory.Start(ctx, spec, ...)  // Reads handshake here!
        clients = append(clients, c)
    }
    return clients, nil
}
```

## Research Questions

### Q1: When should introspection happen?

**Options:**

1. **On TUI startup (blocking)**
   - Start all plugins once, read handshakes, close plugins
   - Pro: Simple, complete data immediately
   - Con: Slow startup (2s+ per plugin), plugins may have side effects

2. **Lazy on-demand**
   - Start plugin only when user tries to use it
   - Pro: Fast startup, no unnecessary work
   - Con: Delayed discovery, poor UX for browse/discover features

3. **Background async on startup**
   - Start introspection in goroutine, update UI when done
   - Pro: Non-blocking startup, progressive enhancement
   - Con: UI flicker, race conditions, complexity

4. **Cached from last run**
   - Persist handshake data to disk, reload on startup
   - Pro: Fast startup with stale data
   - Con: Stale data problems, cache invalidation

5. **Hybrid: Cache + background refresh**
   - Load from cache, refresh in background
   - Pro: Best of both worlds
   - Con: Most complex

### Q2: How to start plugins for introspection only?

Currently `factory.Start()` leaves the plugin running and expects further requests. For introspection-only:

```go
// Current pattern - plugin stays alive until Close()
client, err := factory.Start(ctx, spec, opts)
defer client.Close(ctx)
// client expects requests...
```

**Options:**

1. **Use existing Start() + immediate Close()**
   - Start, read handshake (already done), close immediately
   - Works but may trigger plugin cleanup/side effects

2. **Add Factory.Introspect() method**
   - New method that only reads handshake, doesn't keep alive
   - Cleaner semantics, plugin knows it's introspection-only

3. **Protocol-level introspection request**
   - Add new frame type: `{"type":"introspect"}`
   - Plugin responds with capabilities without entering request loop
   - More protocol changes needed

### Q3: Where to store introspection results?

**Options:**

1. **In-memory in StateWatcher**
   - Add `capabilities map[string]protocol.Capabilities` field
   - Populate once, include in PluginSummary

2. **In Repository struct**
   - Add `Handshakes map[string]protocol.Handshake`
   - Repository becomes "complete" plugin registry

3. **Persistent cache file**
   - Write to `.devctl-cache/plugins.json`
   - Survives TUI restarts

4. **State file (same as services)**
   - Extend `.devctl/state.json` with plugin handshakes
   - Ties introspection to "up" lifecycle

### Q4: What about plugin changes?

If plugin code is updated, cached capabilities may be stale.

**Detection options:**
- Check plugin file mtime vs cache time
- Hash plugin path + args
- Always refresh on manual trigger
- TTL-based refresh (e.g., re-introspect every 5 minutes)

## Interesting Avenues to Explore

### Avenue 1: StateWatcher + Background Introspection

**Concept:** Extend StateWatcher to start plugins in background on first poll, cache results.

```go
// pkg/tui/state_watcher.go

type StateWatcher struct {
    // ... existing fields ...
    
    introspectOnce sync.Once
    introspectWg   sync.WaitGroup
    pluginCaps     map[string]*protocol.Handshake
    pluginCapsMu   sync.RWMutex
}

func (w *StateWatcher) introspectPlugins(ctx context.Context) {
    w.introspectOnce.Do(func() {
        go w.runIntrospection(ctx)
    })
}

func (w *StateWatcher) runIntrospection(ctx context.Context) {
    repo, err := repository.Load(...)
    factory := runtime.NewFactory(...)
    
    for _, spec := range repo.Specs {
        c, err := factory.Start(ctx, spec, ...)
        if err == nil {
            hs := c.Handshake()
            w.pluginCapsMu.Lock()
            w.pluginCaps[spec.ID] = &hs
            w.pluginCapsMu.Unlock()
            _ = c.Close(ctx)
        }
    }
}

func (w *StateWatcher) readPlugins() []PluginSummary {
    // ... existing code ...
    
    w.pluginCapsMu.RLock()
    defer w.pluginCapsMu.RUnlock()
    
    for i, p := range plugins {
        if hs := w.pluginCaps[p.ID]; hs != nil {
            plugins[i].Ops = hs.Capabilities.Ops
            plugins[i].Streams = hs.Capabilities.Streams
        }
    }
    return plugins
}
```

**Pros:**
- Minimal changes to existing architecture
- Non-blocking startup
- Capabilities available after short delay

**Cons:**
- UI shows incomplete data initially
- Need to handle refresh/invalidation

### Avenue 2: Repository with Introspection Mode

**Concept:** Add `repository.LoadWithIntrospection()` that also reads handshakes.

```go
// pkg/repository/repository.go

type Repository struct {
    // ... existing fields ...
    Handshakes map[string]protocol.Handshake
}

type LoadOptions struct {
    Options
    Introspect bool
    Factory    *runtime.Factory
}

func LoadWithIntrospection(ctx context.Context, opts LoadOptions) (*Repository, error) {
    repo, err := Load(opts.Options)
    if err != nil || !opts.Introspect {
        return repo, err
    }
    
    repo.Handshakes = make(map[string]protocol.Handshake)
    for _, spec := range repo.Specs {
        c, err := opts.Factory.Start(ctx, spec, ...)
        if err == nil {
            repo.Handshakes[spec.ID] = c.Handshake()
            _ = c.Close(ctx)
        }
    }
    return repo, nil
}
```

**Pros:**
- Clean API - Repository is complete
- Callers choose whether to pay introspection cost

**Cons:**
- Blocking if called from main thread
- Need to propagate factory to Load()

### Avenue 3: Plugin Capability Cache

**Concept:** Persist handshakes to disk, load on startup, refresh in background.

```go
// pkg/introspection/cache.go

type PluginCache struct {
    Plugins map[string]CachedPlugin `json:"plugins"`
    Version int                      `json:"version"`
}

type CachedPlugin struct {
    SpecHash    string                 `json:"spec_hash"`    // Hash of path+args
    LastChecked time.Time              `json:"last_checked"`
    Handshake   protocol.Handshake     `json:"handshake"`
}

func CachePath(repoRoot string) string {
    return filepath.Join(repoRoot, ".devctl", "plugin-cache.json")
}

func LoadCache(repoRoot string) (*PluginCache, error) { ... }
func (c *PluginCache) Save(repoRoot string) error { ... }
func (c *PluginCache) NeedsRefresh(spec PluginSpec) bool { ... }
```

**Pros:**
- Fast startup with cached data
- Survives restarts

**Cons:**
- Cache invalidation is hard
- Extra file management

### Avenue 4: Protocol Introspection Op

**Concept:** Add a protocol-level introspection operation that returns capabilities without side effects.

```json
// Request
{"type":"request","request_id":"intro-1","op":"__introspect__","input":{}}

// Response (mirrors handshake)
{"type":"response","request_id":"intro-1","ok":true,"output":{
  "plugin_name":"my-plugin",
  "capabilities":{"ops":["build","deploy"],"streams":["logs.follow"]}
}}
```

**Pros:**
- Explicit introspection semantics
- Plugins can opt-out or customize

**Cons:**
- Protocol change
- All plugins need updating
- More complex than just reading handshake

### Avenue 5: Long-Running Plugin Pool

**Concept:** Keep plugins running in a pool, reuse for both introspection and operations.

```go
// pkg/runtime/pool.go

type PluginPool struct {
    mu      sync.Mutex
    clients map[string]runtime.Client
    factory *runtime.Factory
}

func (p *PluginPool) Get(ctx context.Context, spec PluginSpec) (runtime.Client, error) {
    p.mu.Lock()
    defer p.mu.Unlock()
    
    if c, ok := p.clients[spec.ID]; ok && !c.IsClosed() {
        return c, nil
    }
    
    c, err := p.factory.Start(ctx, spec, ...)
    if err != nil {
        return nil, err
    }
    p.clients[spec.ID] = c
    return c, nil
}

func (p *PluginPool) Handshake(id string) (protocol.Handshake, bool) {
    p.mu.Lock()
    defer p.mu.Unlock()
    if c, ok := p.clients[id]; ok {
        return c.Handshake(), true
    }
    return protocol.Handshake{}, false
}
```

**Pros:**
- Plugins started once, reused
- Introspection is always instant after first use
- Efficient for operation-heavy workflows

**Cons:**
- Plugins stay running (resource usage)
- Complex lifecycle management
- Plugin crashes need handling

## Recommended Investigation Order

1. **Start with Avenue 1 (StateWatcher + Background)**
   - Lowest risk, integrates with existing architecture
   - Can be done incrementally

2. **Then evaluate Avenue 3 (Cache)**
   - If startup performance matters
   - Add cache layer on top of Avenue 1

3. **Consider Avenue 5 (Pool) for future**
   - If operations become frequent
   - Larger architectural change

## Implementation Tasks

1. [ ] Add `introspectPlugins()` to StateWatcher
2. [ ] Store capabilities in StateWatcher.pluginCaps map
3. [ ] Populate PluginSummary.Ops/Streams from cached caps
4. [ ] Emit updated StateSnapshot when introspection completes
5. [ ] Handle introspection errors gracefully (partial success)
6. [ ] Add refresh trigger (manual and/or TTL-based)
7. [ ] Test with slow/failing plugins
8. [ ] Consider cache persistence (Avenue 3)

## Testing Considerations

- What if a plugin takes 5+ seconds to start?
- What if a plugin fails to start?
- What if a plugin has side effects on startup?
- What if plugin capabilities change between introspection and use?
- What if introspection is interrupted (TUI quit during)?

## References

- STREAMS-TUI ticket: Identified this gap
- Plugin authoring guide: `pkg/doc/topics/devctl-plugin-authoring.md`
- Protocol spec: `pkg/protocol/types.go`
