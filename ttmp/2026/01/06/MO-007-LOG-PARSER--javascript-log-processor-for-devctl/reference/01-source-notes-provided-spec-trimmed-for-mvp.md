---
Title: 'Source Notes: Provided Spec (trimmed for MVP)'
Ticket: MO-007-LOG-PARSER
Status: active
Topics:
    - backend
DocType: reference
Intent: long-term
Owners: []
RelatedFiles:
    - Path: devctl/ttmp/2026/01/06/MO-007-LOG-PARSER--javascript-log-processor-for-devctl/sources/local/js-log-parser-examples.md
      Note: Imported upstream examples used as input corpus
    - Path: devctl/ttmp/2026/01/06/MO-007-LOG-PARSER--javascript-log-processor-for-devctl/sources/local/js-log-parser-spec.md
      Note: Imported upstream LogFlow spec used as input corpus
ExternalSources: []
Summary: Distill the imported LogFlow spec/examples into a concrete, small MVP contract suitable for goja/go-go-goja.
LastUpdated: 2026-01-06T18:28:08-05:00
WhatFor: Keep a clear boundary between the exhaustive upstream spec and what we actually plan to implement first.
WhenToUse: Use while implementing or reviewing the MVP to avoid scope creep and to keep compatibility decisions explicit.
---


# Source Notes: Provided Spec (trimmed for MVP)

## Goal

Turn the imported documents:

- `sources/local/js-log-parser-spec.md`
- `sources/local/js-log-parser-examples.md`

…into a clear “MVP subset contract” for `devctl` that is implementable with `goja` (using patterns from `go-go-goja`) without committing to the full LogFlow vision.

## Context

The imported spec describes a complete log-processing platform: multi-stage hooks, large helper standard library, batching, worker pools, async hooks, state/rate-limiting helpers, destinations, retries, etc.

For `devctl`, we want the smallest slice that is immediately useful in a developer workflow:

- take raw log lines (usually from files that `devctl` already manages)
- run a user-provided JavaScript “parser module”
- output structured events (as NDJSON) that downstream tools can grep/jq/filter

We intentionally keep the MVP synchronous and single-module so we can ship a robust foundation before adding power features. We also intentionally ship it first as a standalone `log-parse` CLI to iterate in isolation, and only integrate with `devctl logs` later.

## Quick Reference

### MVP scope: what we keep vs drop

| Area in imported spec | Keep in MVP? | MVP decision / shape |
|---|---:|---|
| Module registration (`register({ ... })`) | Yes | Keep `register(config)`; only one module per run. |
| `parse(line, context)` hook | Yes (core) | Required. Returns event object (or `null` to drop). |
| `filter(event, context)` hook | Yes | Optional. Returns boolean; `false` drops event. |
| `transform(event, context)` hook | Yes | Optional. Returns event object or `null` to drop. |
| `init(context)` / `shutdown(context)` | Yes (minimal) | Optional and synchronous only (no `Promise` in MVP). |
| `onError(error, eventOrLine, context)` | Yes | Optional. Called on hook exceptions; default is “drop + count error”. |
| `aggregate`, `output`, destinations | No | Out of scope; MVP only emits events to stdout (NDJSON). |
| “Standard library” helpers (large list) | No (trim) | MVP provides a tiny helper surface: JSON + regex + logfmt + field helpers. |
| Stateful caches, TTL state, rate limiting | No (trim) | MVP provides only a per-runtime `state` object (in-memory, no TTL). |
| Multi-line buffering | No | Out of scope; revisit once single-line pipeline is stable. |
| Batching / worker threads | Partial | MVP is single-worker by default; `--workers` is optional and off by default to preserve ordering. |
| Async output / I/O | No | Out of scope until we add `goja_nodejs/eventloop` and a clear safety model. |

### MVP JS contract (copy/paste)

```js
// parser.js
register({
  name: "my-parser",

  // Optional: called once before the first line.
  init(ctx) {
    ctx.state.count = 0;
  },

  // Required.
  parse(line, ctx) {
    ctx.state.count++;

    // Helper: try to parse JSON; return null to drop unparseable lines.
    const obj = log.parseJSON(line);
    if (!obj) return null;

    return {
      timestamp: obj.ts,          // string or Date accepted
      level: obj.level || "INFO",
      message: obj.msg || line,
      fields: obj,
      tags: [],
    };
  },

  // Optional.
  filter(event, ctx) {
    return event.level !== "DEBUG";
  },

  // Optional.
  transform(event, ctx) {
    event.fields._source = ctx.source;
    event.fields._line = ctx.lineNumber;
    return event;
  },

  // Optional: hook exception handler (parse/filter/transform/init/shutdown).
  onError(err, payload, ctx) {
    console.error(`[${ctx.hook}] ${err.message}`);
  },
});
```

### MVP event schema (normalized)

MVP normalizes whatever JS returns into:

```json
{
  "timestamp": "2026-01-06T18:00:00Z",
  "level": "INFO",
  "message": "text",
  "fields": { "any": "json-compatible" },
  "tags": ["optional"],
  "source": "service-name-or-path",
  "raw": "original line",
  "lineNumber": 123
}
```

Notes:
- `timestamp` may be missing; MVP may set it to “now” or omit it (decision captured in the design-doc).
- `fields` defaults to `{}`; `tags` defaults to `[]`.

## Usage Examples

### Example: parse logfmt-ish lines quickly

```js
register({
  name: "logfmt",
  parse(line, ctx) {
    const obj = log.parseLogfmt(line);
    if (!obj) return null;
    return {
      timestamp: obj.timestamp,
      level: obj.level,
      message: obj.msg || line,
      fields: obj,
      tags: [],
    };
  },
});
```

### Example: regex capture to event fields

```js
register({
  name: "regex",
  parse(line, ctx) {
    const m = log.namedCapture(line, /^(?<level>\\w+)\\s+(?<msg>.*)$/);
    if (!m) return null;
    return { level: m.level, message: m.msg, fields: m, tags: [] };
  },
});
```

## Related

- Design doc: `design-doc/01-mvp-design-javascript-log-parser-goja.md`
- Imported spec: `sources/local/js-log-parser-spec.md`
- Imported examples: `sources/local/js-log-parser-examples.md`
