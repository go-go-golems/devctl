---
Title: 'Analysis: Sandboxing goja_nodejs require()'
Ticket: MO-007-LOG-PARSER
Status: active
Topics:
    - backend
DocType: design-doc
Intent: long-term
Owners: []
RelatedFiles:
    - Path: go-go-goja/engine/runtime.go
      Note: 'Shows safe ordering: enable require() then console.Enable()'
    - Path: go-go-goja/modules/common.go
      Note: Defines go-go-goja module registry and Enable() which registers native modules into goja_nodejs require
    - Path: jesus/pkg/engine/bindings.go
      Note: 'Alternative approach: custom console without goja_nodejs console'
    - Path: jesus/pkg/engine/engine.go
      Note: Example of using require.NewRegistry + moduleRegistry.Enable in a real app
ExternalSources: []
Summary: How goja_nodejs require() resolves modules and how to sandbox it to only load files within an allowed directory tree rooted at the entry script.
LastUpdated: 2026-01-06T18:56:23-05:00
WhatFor: Enable Node-style require() for ergonomics (console/util and local modules) without granting arbitrary filesystem module loading.
WhenToUse: Use when adding require() to devctl/pkg/logjs or when reviewing sandbox/security constraints for embedded JS.
---


# Analysis: Sandboxing goja_nodejs require()

## Executive Summary

`goja_nodejs/require` provides a Node-like `require()` system for `goja` runtimes. By default it can load modules from the host filesystem via `DefaultSourceLoader`, and it uses the Node-style algorithm to resolve bare imports via `node_modules` directories up the directory tree.

For `MO-007-LOG-PARSER`, we may want to enable `require()` primarily for:

- “core modules” like `console` and `util` (so `console.log()` has Node-like formatting),
- and local relative requires from the parser script directory (`require("./helpers")`).

We **do not** want `require()` to become “arbitrary file read” or “load modules from anywhere on disk”.

This document explains how `require()` resolves modules in goja_nodejs and proposes a practical sandbox:

1) Use `require.NewRegistry(require.WithLoader(...))` and implement a `SourceLoader` that:
   - allows `.js` and `.json` loads *only* under an allowed root directory tree,
   - returns `require.ModuleFileDoesNotExistError` for any disallowed path.

2) Ensure the entry script path is absolute and define the root as `dirname(entryScriptAbs)`.

3) Avoid registering dangerous native modules (`fs`, `exec`) unless explicitly opted in.

## Problem Statement

We currently run user-provided JavaScript inside `goja` for log parsing. The MVP intentionally avoided Node-style `require()` to keep the runtime safe-by-default.

However, not having `require()` means:

- user scripts cannot be split across multiple files (`require("./foo")`)
- using `goja_nodejs/console` is awkward (it requires `require()` to be enabled)

If we enable `require()` naively, we risk:

- allowing scripts to load arbitrary `.js`/`.json` from the filesystem
- allowing resolution to walk up the directory tree (Node `node_modules` search) and unexpectedly load dependencies from outside the intended sandbox

We need a concrete and reviewable sandbox policy:

- “Only load modules from relative directories down from the entry script”
- and allow only a minimal set of built-in and native modules.

## Proposed Solution

### How goja_nodejs require() works (implementation model)

At a high level:

```
JS code: require("X")
    │
    ▼
RequireModule.resolve("X")  // Node-like module resolution
    │
    ├─ native/core module?  → load from registered module loader (no file reads)
    │
    └─ file/directory module? → call Registry.SourceLoader with candidate paths
                                 (DefaultSourceLoader reads host filesystem)
```

Important implementation details (as of the `goja_nodejs` version in this workspace):

- `require.NewRegistry()` returns a `*require.Registry`.
- `reg.Enable(vm)` installs a global `require` function into the runtime.
- If no loader is provided, `DefaultSourceLoader` reads files from disk.
- “Native modules” are registered via `Registry.RegisterNativeModule(name, loader)` and bypass `SourceLoader` entirely.
- “Core modules” (like `console`) are registered globally via `require.RegisterCoreModule(...)`.

### Resolution algorithm: file path vs bare module name

`require` distinguishes:

1) **file-or-directory path** (examples: `./x`, `../x`, `/abs/x`):
   - resolves relative to the requiring module’s directory
   - tries as file (`x`, `x.js`, `x.json`) then as directory (`x/package.json` main, else `x/index.js`, `x/index.json`)

2) **bare module name** (examples: `console`, `util`, `leftpad`):
   - tries native/core modules first
   - then searches `node_modules` directories upward from the requiring module’s directory
   - then searches any configured “global folders” (if any)

This is why sandboxing must consider:

- `require("console")` is not a file read (it is a core module load), but it may itself require other core modules (e.g. `util`).
- `require("leftpad")` will attempt `node_modules/leftpad` paths up the directory tree and may escape a naive “relative only” mindset.

### The sandbox policy we want

Given an entry script:

```
entry = /abs/path/to/parser.js
root  = /abs/path/to
```

Allow:

- core modules needed for ergonomics: `console`, `util`, possibly `buffer`, `url`, `process` (optional)
- native modules that we register explicitly (e.g. a future `log` helper module)
- file-backed requires only under:
  - `/abs/path/to/**`

Deny:

- any module file load outside `/abs/path/to/**`
- any attempt to use native modules we did not register (`fs`, `exec`, etc.)

### Where to hook sandboxing: SourceLoader

The critical seam is `require.WithLoader(loader)` which sets `Registry.srcLoader`.
All file-based module resolution ultimately calls `srcLoader(resolvedPath)`.

Therefore:

- If `resolvedPath` is outside the allowed root, return `require.ModuleFileDoesNotExistError`.
  - This signals “not found” and makes the resolver continue searching.
  - This avoids leaking “permission denied” vs “not found” and keeps behavior predictable.

### Pseudocode: sandboxed SourceLoader

```go
type Sandbox struct {
	RootAbs string // e.g. /abs/path/to (OS path)
}

func (s Sandbox) SourceLoader(resolved string) ([]byte, error) {
	// resolved uses forward slashes; normalize to OS path.
	osPath := filepath.FromSlash(resolved)

	// Make absolute relative to the root if needed.
	if !filepath.IsAbs(osPath) {
		osPath = filepath.Join(s.RootAbs, osPath)
	}

	// Clean and (optionally) resolve symlinks.
	clean := filepath.Clean(osPath)
	// stronger but more expensive:
	// clean, _ = filepath.EvalSymlinks(clean)

	// Enforce root containment (prefix check on clean absolute paths).
	root := filepath.Clean(s.RootAbs) + string(os.PathSeparator)
	if clean != filepath.Clean(s.RootAbs) && !strings.HasPrefix(clean, root) {
		return nil, require.ModuleFileDoesNotExistError
	}

	// Optional: allow only certain extensions.
	ext := strings.ToLower(filepath.Ext(clean))
	if ext != ".js" && ext != ".json" {
		return nil, require.ModuleFileDoesNotExistError
	}

	return os.ReadFile(clean)
}
```

Notes:

- goja_nodejs uses the POSIX `path` package internally, so module paths in `resolved` will use `/`.
- It is important that we run the entry script using an **absolute path** so `getCurrentModulePath()` is absolute in subsequent requires, simplifying containment checks.
- Returning `ModuleFileDoesNotExistError` causes the resolver to continue searching; if all disallowed, the require ultimately fails as “module not found”.

### Sandboxing native modules (separate plane)

Even with a restricted `SourceLoader`, native modules are loaded via `RegisterNativeModule` and are not affected by the loader.

So sandboxing requires a second control plane:

- do not register dangerous modules by default
- register only what `log-parse` needs (ideally: none, or only pure helpers)
- if we want `fs`/`exec` for power users, put them behind an explicit flag and only register them when requested

### Enabling goja_nodejs console safely

`goja_nodejs/console.Enable(vm)` requires `require()` to be enabled in the runtime. But enabling `require()` does not necessarily mean “enable filesystem module loading” if we provide a safe `SourceLoader`:

- core module `console` is loaded via `RegisterCoreModule`
- `console` requires `util` (also a core module)
- neither require filesystem loader

So the safe sequence is:

1. `reg := require.NewRegistry(require.WithLoader(sandbox.SourceLoader))`
2. optionally `reg = require.NewRegistry(require.WithLoader(...), require.WithGlobalFolders(/* none */))`
3. `reg.Enable(vm)`  // install require()
4. `console.Enable(vm)`  // now safe
5. register any allowed native modules into the registry

## Design Decisions

### 1) Deny-by-default for file-backed module loading

We use a loader that denies any path outside the allowed root by returning `ModuleFileDoesNotExistError`.

Rationale:
- simple and easy to audit
- avoids leaking filesystem information
- makes module resolution deterministic (no “surprise module from parent node_modules”)

### 2) Separate controls for native modules

Even a perfect loader does not sandbox native modules.

Rationale:
- go-go-goja makes it easy to accidentally enable `fs`/`exec` via blank imports
- we need explicit opt-in for side-effectful modules

### 3) Entry script path should be absolute

Rationale:
- simplifies containment checks (absolute prefix compare)
- avoids ambiguity of “current working directory” resolution

## Alternatives Considered

### 1) No require() at all (status quo MVP)

Pros:
- simplest safety model

Cons:
- no multi-file scripts
- cannot use goja_nodejs console formatting without custom console implementation

### 2) Allow require() but only for core modules

Pros:
- enables goja_nodejs `console` without filesystem access

Cons:
- still no multi-file scripts; users will ask for `require("./helpers")`

### 3) Full require() with filesystem access

Rejected:
- too dangerous-by-default for `log-parse` and `devctl` usage

## Implementation Plan

1. Add a “sandboxed require” mode to `devctl/pkg/logjs` (opt-in at first):
   - convert entry script path to absolute
   - compute root = dirname(entry script)
2. Create a `require.Registry` with `require.WithLoader(sandbox.SourceLoader)`.
3. Enable require, then enable `goja_nodejs/console`.
4. Decide the allowed set of core modules (console/util/buffer/url/process).
5. Add tests:
   - `require("./relative.js")` inside root succeeds
   - `require("../escape.js")` fails
   - `require("/etc/passwd")` fails
   - `require("console")` works
6. Add a `--js-require` / `--require` flag to `log-parse` to toggle behavior (default off until we’re confident).

## Open Questions

1) Do we allow `node_modules` inside the root directory (e.g. `/abs/path/to/node_modules/**`)?
2) Do we allow `.json` requires in addition to `.js`?
3) Should we enforce symlink containment via `filepath.EvalSymlinks`?

## References

- go-go-goja runtime wiring: `/home/manuel/workspaces/2026-01-06/log-parser-module/go-go-goja/engine/runtime.go`
- go-go-goja registry: `/home/manuel/workspaces/2026-01-06/log-parser-module/go-go-goja/modules/common.go`
- jesus engine (uses registry + require): `/home/manuel/workspaces/2026-01-06/log-parser-module/jesus/pkg/engine/engine.go`
- jesus custom console bindings: `/home/manuel/workspaces/2026-01-06/log-parser-module/jesus/pkg/engine/bindings.go`

## Design Decisions

<!-- Document key design decisions and rationale -->

## Alternatives Considered

<!-- List alternative approaches that were considered and why they were rejected -->

## Implementation Plan

<!-- Outline the steps to implement this design -->

## Open Questions

<!-- List any unresolved questions or concerns -->

## References

<!-- Link to related documents, RFCs, or external resources -->
