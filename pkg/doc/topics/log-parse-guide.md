---
Title: log-parse Developer Guide (JavaScript Log Parsing with goja)
Slug: log-parse-guide
Short: A complete guide to parsing and transforming log streams using JavaScript modules—from first script to Go integration.
Topics:
  - devctl
  - log-parsing
  - javascript
  - goja
  - observability
  - scripting
IsTemplate: false
IsTopLevel: true
ShowPerDefault: true
SectionType: GeneralTopic
---

# log-parse Developer Guide

log-parse is a JavaScript-based log processor that lets you parse, filter, and transform log lines into structured events without inventing a new DSL. It uses the goja JavaScript runtime embedded in Go, which means you get a familiar JavaScript syntax with the performance and deployment simplicity of a single binary.

The core idea: write small JavaScript modules that describe *how* to parse your logs, and let log-parse handle the streaming, error isolation, and output normalization. You can run a single module for simple cases, or fan out to many modules that each produce a different "view" of the same log stream (errors, metrics, security events).

## 0. Start here (if you're new)

This guide goes deep on the JavaScript module API, the fan-out pipeline, and Go integration. If you just want to run a quick example:

```bash
# From the devctl repo root
cat examples/log-parse/sample-json-lines.txt | go run ./cmd/log-parse --module examples/log-parse/parser-json.js
```

You'll see NDJSON output with structured events. The rest of this guide explains how to write your own modules, use the helper API, and integrate log-parse into larger systems.

## 1. What log-parse does

Every log stream has structure—timestamps, levels, trace IDs, error codes—but it's often buried in text. log-parse extracts that structure by running your JavaScript parsing logic against each line and emitting normalized JSON events.

```
┌─────────────────────────────────────────────────────────────────┐
│                       log-parse pipeline                         │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│   raw log lines          JavaScript modules         NDJSON out   │
│   ──────────────>        ┌──────────────┐        ──────────────> │
│                          │  parse       │                        │
│   INFO: startup          │  filter      │        {"level":"INFO",│
│   ERROR: db down         │  transform   │         "message":...} │
│   ...                    └──────────────┘                        │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

**Key properties:**

- **Line-oriented**: input is a stream of lines; each line goes through all modules independently.
- **Synchronous**: all JavaScript hooks run synchronously (no async/await). This keeps the runtime simple and deterministic.
- **Safe by default**: the JavaScript runtime has no filesystem, network, or exec access unless explicitly enabled.
- **Fan-out capable**: run many modules on the same input; each module emits its own tagged event stream.

## 2. Quick start: your first parser

Let's parse JSON logs. Create a file `my-parser.js`:

```javascript
register({
  name: "my-json-parser",

  parse(line, ctx) {
    const obj = log.parseJSON(line);
    if (!obj) return null;  // skip non-JSON lines

    return {
      timestamp: obj.ts,
      level: obj.level || "INFO",
      message: obj.msg || line,
      service: obj.service,
      trace_id: obj.trace_id,
    };
  },
});
```

Run it:

```bash
echo '{"ts":"2026-01-06T12:00:00Z","level":"INFO","msg":"startup","service":"api"}' | \
  go run ./cmd/log-parse --module my-parser.js
```

Output:

```json
{"timestamp":"2026-01-06T12:00:00Z","level":"INFO","message":"startup","fields":{"_module":"my-json-parser","_tag":"my-json-parser","service":"api"},"tags":["my-json-parser"],"source":"stdin","raw":"{\"ts\":\"2026-01-06T12:00:00Z\",\"level\":\"INFO\",\"msg\":\"startup\",\"service\":\"api\"}","lineNumber":1}
```

The module parsed the JSON, extracted fields, and log-parse normalized everything into a consistent event schema.

## 3. The module contract: `register({ ... })`

Every log-parse module calls `register()` exactly once with a configuration object. This object defines the module's name, optional tag, and hook functions.

```javascript
register({
  name: "my-module",           // required: unique module name
  tag: "errors",               // optional: derived stream tag (defaults to name)

  // Required hook: parse each line
  parse(line, ctx) {
    // return object, string, array, or null
  },

  // Optional hooks
  filter(event, ctx) { return true; },      // return false to drop
  transform(event, ctx) { return event; },  // return modified event
  init(ctx) {},                              // called once at startup
  shutdown(ctx) {},                          // called once at shutdown
  onError(err, payload, ctx) {},             // called when hooks throw
});
```

### 3.1. Hook execution order

For each input line, log-parse executes:

```
line ──> parse() ──> filter() ──> transform() ──> emit
              │          │              │
              └──────────┴──────────────┘
                    (if any returns null/false, line is dropped)
```

### 3.2. Required: `name`

The `name` field is required and must be unique within a run. It's used for:

- Diagnostics and error messages
- The `_module` field in emitted events
- Stats tracking

### 3.3. Optional: `tag`

The `tag` field sets the derived stream identifier. If omitted, it defaults to `name`. Use explicit tags when you want multiple modules to contribute to the same logical stream:

```javascript
// Both modules emit to the "security" stream
register({ name: "auth-failures", tag: "security", ... });
register({ name: "access-denials", tag: "security", ... });
```

### 3.4. The context object

Every hook receives a `ctx` object with:

```javascript
{
  hook: "parse",        // current hook name
  source: "stdin",      // input source label
  lineNumber: 42,       // 1-indexed line number
  now: Date,            // snapshot of current time
  state: {},            // mutable per-module state (persists across lines)
}
```

The `state` object is your module's scratch space. Use it for counters, buffers, or any state that spans multiple lines:

```javascript
register({
  name: "line-counter",
  parse(line, ctx) {
    ctx.state.count = (ctx.state.count || 0) + 1;
    return { message: line, line_count: ctx.state.count };
  },
});
```

## 4. Hook semantics and return values

### 4.1. `parse(line, ctx)`

The `parse` hook is required. It receives the raw line (without trailing newline) and returns:

| Return value | Behavior |
|--------------|----------|
| `null` or `undefined` | Drop line (no event emitted) |
| `string` | Shorthand for `{ message: string }` |
| `object` | Treated as an event (normalized by log-parse) |
| `array` | Each element is treated as a separate event |

**Example: returning multiple events**

```javascript
parse(line, ctx) {
  const obj = log.parseJSON(line);
  if (!obj) return null;

  // Emit both raw event and a derived metric
  return [
    { level: obj.level, message: obj.msg },
    { level: "INFO", message: "metric", fields: { duration_ms: obj.duration } },
  ];
}
```

### 4.2. `filter(event, ctx)`

The `filter` hook receives the parsed event and returns a boolean:

- `true`: keep the event
- `false`: drop the event

```javascript
filter(event, ctx) {
  // Only keep errors and warnings
  return event.level === "ERROR" || event.level === "WARN";
}
```

### 4.3. `transform(event, ctx)`

The `transform` hook receives a parsed (and filtered) event and returns a modified event:

| Return value | Behavior |
|--------------|----------|
| `null` or `undefined` | Drop event |
| `object` | Use as the new event |
| `array` | Each element becomes a separate event |

```javascript
transform(event, ctx) {
  // Redact sensitive fields
  if (event.fields && event.fields.password) {
    event.fields.password = "[REDACTED]";
  }
  return event;
}
```

### 4.4. `init(ctx)` and `shutdown(ctx)`

These lifecycle hooks run once per module:

- `init`: called after the module is loaded, before processing any lines
- `shutdown`: called after all lines are processed

Use them for setup (initializing buffers) and cleanup (flushing state):

```javascript
register({
  name: "buffered",
  init(ctx) {
    ctx.state.buffer = log.createMultilineBuffer({
      pattern: /^ERROR/,
      match: "after",
    });
  },
  parse(line, ctx) {
    return ctx.state.buffer.add(line);
  },
  shutdown(ctx) {
    const remaining = ctx.state.buffer.flush();
    if (remaining) console.warn("unflushed:", remaining);
  },
});
```

### 4.5. `onError(err, payload, ctx)`

When a hook throws an exception, log-parse calls `onError` (if defined) with:

- `err`: the error object
- `payload`: the value passed to the failing hook (line or event)
- `ctx`: the context at the time of error

Use this for logging, metrics, or graceful degradation:

```javascript
onError(err, payload, ctx) {
  console.error(`[${ctx.hook}] error: ${err.message}`);
}
```

## 5. Event schema and normalization

log-parse normalizes every returned event into a consistent schema before emitting:

```json
{
  "timestamp": "2026-01-06T12:00:00Z",
  "level": "INFO",
  "message": "something happened",
  "fields": { "service": "api", "trace_id": "abc123" },
  "tags": ["my-module"],
  "source": "stdin",
  "raw": "original log line",
  "lineNumber": 42
}
```

### 5.1. Normalization rules

When your hook returns an object, log-parse applies these rules:

| Field | Default | Notes |
|-------|---------|-------|
| `timestamp` | omitted | Pass a `Date` object or ISO string |
| `level` | `"INFO"` | Uppercase recommended |
| `message` | raw line | Falls back to raw input if missing |
| `fields` | `{}` | Merged from returned `fields` + extra keys |
| `tags` | `[]` | Empty strings are filtered out |
| `source` | from context | Usually the input filename or "stdin" |
| `raw` | original line | Always preserved |
| `lineNumber` | from context | 1-indexed |

**Extra keys become fields**: Any key you return that isn't in the reserved set (`timestamp`, `level`, `message`, `fields`, `tags`) is moved into `fields`:

```javascript
// This:
return { level: "ERROR", message: "boom", trace_id: "abc" };

// Becomes:
{ "level": "ERROR", "message": "boom", "fields": { "trace_id": "abc" }, ... }
```

### 5.2. Timestamp handling

Pass timestamps as:

- **Date objects**: converted via `toISOString()`
- **Strings**: kept as-is
- **`log.parseTimestamp()` results**: best-effort parsing (see helpers section)

```javascript
return {
  timestamp: log.parseTimestamp(obj.ts),  // robust parsing
  message: obj.msg,
};
```

## 6. The helper API: `log.*`

log-parse provides a global `log` object with parsing helpers. These are pure functions (no I/O) designed for common log parsing tasks.

### 6.1. Parsing helpers

#### `log.parseJSON(line)`

Parse a JSON string. Returns the parsed object or `null` on failure.

```javascript
const obj = log.parseJSON(line);
if (!obj) return null;
```

#### `log.parseLogfmt(line)`

Parse logfmt-style key=value pairs. Supports quoted values with escapes.

```javascript
// Input: level=INFO msg="hello world" trace_id=abc
const obj = log.parseLogfmt(line);
// Result: { level: "INFO", msg: "hello world", trace_id: "abc" }
```

#### `log.parseKeyValue(line, delimiter?, separator?)`

Generic key-value parsing with configurable delimiters.

```javascript
// Default: space-delimited, "=" separator
log.parseKeyValue("a=1 b=2")  // { a: "1", b: "2" }

// Custom: comma-delimited, ":" separator
log.parseKeyValue("a:1,b:2", ",", ":")  // { a: "1", b: "2" }
```

### 6.2. Regex helpers

#### `log.capture(line, regex)`

Return an array of capture groups (without the full match).

```javascript
const m = log.capture(line, /^(\w+)\s+\[([^\]]+)\]\s+(.*)$/);
if (m) {
  return { level: m[0], service: m[1], message: m[2] };
}
```

#### `log.namedCapture(line, regex)`

Return an object from named capture groups. Note: goja's RegExp engine doesn't support `(?<name>...)` syntax, so this is limited.

#### `log.extract(line, regex, group?)`

Extract a single group (default: group 1).

```javascript
const traceId = log.extract(line, /trace_id=(\w+)/);
```

### 6.3. Object traversal

#### `log.getPath(obj, path)` / `log.field(obj, path)`

Dot-notation path access. Returns `null` if any segment is missing.

```javascript
const userId = log.getPath(obj, "user.profile.id");
```

#### `log.hasPath(obj, path)`

Returns `true` if the path exists and is not null.

### 6.4. Tag helpers

#### `log.addTag(event, tag)`

Add a tag to an event's `tags` array (idempotent).

```javascript
log.addTag(event, "security");
log.addTag(event, "auth_failed");
```

#### `log.removeTag(event, tag)`

Remove a tag from an event's `tags` array.

#### `log.hasTag(event, tag)`

Check if an event has a specific tag.

### 6.5. Type conversion

#### `log.toNumber(value)`

Convert to number safely. Returns `null` if not a finite number.

```javascript
const duration = log.toNumber(obj.duration_ms);
if (duration == null) return null;
```

#### `log.parseTimestamp(value, formats?)`

Parse timestamps with best-effort heuristics. Supports:

- ISO 8601 strings
- Unix timestamps (seconds or milliseconds)
- Common date formats (via dateparse library)

```javascript
return { timestamp: log.parseTimestamp(obj.ts) };

// With explicit formats (Go time.Parse layouts)
log.parseTimestamp(obj.ts, ["2006-01-02 15:04:05", "Jan 2 2006"])
```

### 6.6. Multiline buffer

#### `log.createMultilineBuffer(config)`

Create a buffer for accumulating multiline log entries (like stack traces).

```javascript
const buffer = log.createMultilineBuffer({
  pattern: /^\s+at /,  // continuation pattern
  negate: true,        // true = pattern marks START of new record
  match: "after",      // only "after" is supported
  maxLines: 200,       // max lines per record
  timeout: "5s",       // flush after timeout (best-effort)
});

// In parse():
const complete = buffer.add(line);
if (complete) {
  return { message: complete.split("\n")[0], fields: { stack: complete } };
}
return null;
```

The buffer returns `null` while accumulating and returns the complete record when a new record starts.

## 7. Multi-module fan-out pipeline

log-parse can run multiple modules simultaneously, each producing its own tagged event stream. This is the "fan-out" model: one input stream, many derived output streams.

### 7.1. Why fan-out?

Instead of building one giant module that handles everything, you write small, focused modules:

```
                              ┌─────────────┐
                          ┌──>│ errors.js   │──> tag: errors
                          │   └─────────────┘
input lines ──────────────┼──>┌─────────────┐
                          │   │ metrics.js  │──> tag: metrics
                          │   └─────────────┘
                          └──>┌─────────────┐
                              │ security.js │──> tag: security
                              └─────────────┘
```

Each module:
- Receives the same input line
- Decides independently whether to emit (return null if not relevant)
- Emits events tagged with its tag
- Has isolated state (no cross-module interference)

### 7.2. Loading multiple modules

Use `--module` (repeatable) or `--modules-dir`:

```bash
# Explicit module list
log-parse --module errors.js --module metrics.js --input app.log

# Load all *.js from a directory (lexicographic order)
log-parse --modules-dir ./parsers/ --input app.log
```

Files in `--modules-dir` are loaded in lexicographic order. Use numeric prefixes for explicit ordering:

```
parsers/
  01-errors.js
  02-metrics.js
  03-security.js
```

### 7.3. Tag injection

For each event emitted from a module, log-parse automatically:

1. Adds the module's `tag` to `event.tags` (if not already present)
2. Sets `event.fields._tag` to the tag
3. Sets `event.fields._module` to the module name

This ensures downstream consumers can always group/filter by tag.

### 7.4. Example: errors, metrics, security

**01-errors.js**:
```javascript
register({
  name: "errors",
  tag: "errors",
  parse(line, ctx) {
    const obj = log.parseJSON(line);
    if (!obj) return null;

    const level = String(obj.level || "").toUpperCase();
    if (level !== "ERROR" && level !== "FATAL") return null;

    return {
      level,
      message: obj.msg || obj.message || line,
      fields: { service: obj.service, trace_id: obj.trace_id },
    };
  },
});
```

**02-metrics.js**:
```javascript
register({
  name: "metrics",
  tag: "metrics",
  parse(line, ctx) {
    const obj = log.parseJSON(line);
    if (!obj) return null;

    const durationMs = log.toNumber(obj.duration_ms);
    if (durationMs == null) return null;

    return {
      level: "INFO",
      message: "request_duration_ms",
      fields: { service: obj.service, route: obj.route, duration_ms: durationMs },
    };
  },
});
```

**03-security.js**:
```javascript
register({
  name: "security",
  tag: "security",
  parse(line, ctx) {
    const obj = log.parseJSON(line);
    if (!obj) return null;

    const msg = String(obj.msg || obj.message || "");
    if (!msg.toLowerCase().includes("authentication failed")) return null;

    const ev = {
      level: "WARN",
      message: msg,
      fields: { service: obj.service, user: obj.user, ip: obj.ip },
      tags: [],
    };
    log.addTag(ev, "auth_failed");
    return ev;
  },
});
```

Run:

```bash
cat app.log | log-parse --modules-dir ./parsers/ --print-pipeline --stats
```

Output includes the pipeline summary and per-module statistics.

## 8. CLI reference

### 8.1. Basic usage

```bash
log-parse [flags] [command]
```

### 8.2. Flags

| Flag | Description | Default |
|------|-------------|---------|
| `--module <path>` | Path to a JS module file (repeatable) | required |
| `--modules-dir <dir>` | Directory to load all *.js files from (repeatable) | - |
| `--input <path>` | Input file path | stdin |
| `--source <label>` | Source label in events | filename or "stdin" |
| `--format <fmt>` | Output format: `ndjson` or `pretty` | `ndjson` |
| `--js-timeout <dur>` | Per-hook timeout (e.g. `50ms`, `200ms`) | 0 (no timeout) |
| `--print-pipeline` | Print loaded modules and hooks (stderr) | false |
| `--stats` | Print per-module stats on exit (stderr) | false |
| `--errors <path>` | Write error records to file (`stderr` or `-` for stderr) | - |

### 8.3. Commands

#### `log-parse validate`

Validate modules without processing input. Checks:

- Script compilation
- `register()` call
- Unique module names
- Required hooks

```bash
log-parse validate --modules-dir ./parsers/
```

### 8.4. Examples

```bash
# Parse stdin with a single module
cat app.log | log-parse --module parser.js

# Parse file with multiple modules
log-parse --modules-dir ./parsers/ --input app.log

# Pretty-print output
log-parse --module parser.js --input app.log --format pretty

# Show pipeline and stats
log-parse --modules-dir ./parsers/ --input app.log --print-pipeline --stats

# Set timeout to prevent infinite loops
log-parse --module parser.js --js-timeout 100ms --input app.log

# Write errors to file
log-parse --modules-dir ./parsers/ --input app.log --errors errors.ndjson

# Validate modules
log-parse validate --modules-dir ./parsers/
```

## 9. Error handling and debugging

### 9.1. Error isolation

Errors in one module don't affect other modules. If a hook throws:

1. The error is recorded (see `--errors`)
2. `onError` is called (if defined)
3. The line is dropped for that module
4. Other modules continue processing

### 9.2. Error records

With `--errors`, log-parse writes structured error records:

```json
{
  "module": "my-parser",
  "tag": "my-parser",
  "hook": "parse",
  "source": "stdin",
  "lineNumber": 42,
  "timeout": false,
  "message": "TypeError: Cannot read property 'foo' of undefined",
  "rawLine": "the original line"
}
```

### 9.3. Timeout protection

Use `--js-timeout` to prevent infinite loops from blocking the pipeline:

```bash
log-parse --module parser.js --js-timeout 50ms --input app.log
```

If a hook exceeds the timeout, it's interrupted and treated as an error.

### 9.4. Debugging tips

- Use `console.log()` and `console.error()` in your modules (output goes to stdout/stderr)
- Use `--print-pipeline` to verify which modules and hooks are loaded
- Use `--stats` to see drop rates and error counts
- Use `--format pretty` for readable output during development
- Test with small sample files before processing large logs

## 10. Integrating with Go applications

The `pkg/logjs` package provides a Go API for embedding log-parse in your own applications.

### 10.1. Core types

```go
import "github.com/go-go-golems/devctl/pkg/logjs"

// Event is the normalized output event
type Event struct {
    Timestamp  *string        `json:"timestamp,omitempty"`
    Level      string         `json:"level"`
    Message    string         `json:"message"`
    Fields     map[string]any `json:"fields"`
    Tags       []string       `json:"tags"`
    Source     string         `json:"source"`
    Raw        string         `json:"raw"`
    LineNumber int64          `json:"lineNumber"`
}

// ErrorRecord captures hook failures
type ErrorRecord struct {
    Module     string  `json:"module"`
    Tag        string  `json:"tag"`
    Hook       string  `json:"hook"`
    Source     string  `json:"source"`
    LineNumber int64   `json:"lineNumber"`
    Timeout    bool    `json:"timeout"`
    Message    string  `json:"message"`
    RawLine    *string `json:"rawLine,omitempty"`
}

// Options configures module loading
type Options struct {
    HookTimeout string  // e.g. "50ms"
}
```

### 10.2. Loading a single module

```go
import (
    "context"
    "github.com/go-go-golems/devctl/pkg/logjs"
)

func main() {
    ctx := context.Background()

    // Load a module from file
    module, err := logjs.LoadFromFile(ctx, "parser.js", logjs.Options{
        HookTimeout: "100ms",
    })
    if err != nil {
        log.Fatal(err)
    }
    defer module.Close(ctx)

    // Process lines
    events, errors, err := module.ProcessLine(ctx, logLine, "source", lineNumber)
    if err != nil {
        log.Fatal(err)
    }

    for _, ev := range events {
        // Handle event
        fmt.Printf("%s: %s\n", ev.Level, ev.Message)
    }

    for _, errRec := range errors {
        // Handle error record
        fmt.Fprintf(os.Stderr, "error in %s: %s\n", errRec.Hook, errRec.Message)
    }
}
```

### 10.3. Loading a fan-out pipeline

```go
func main() {
    ctx := context.Background()

    // Load multiple modules
    scriptPaths := []string{"errors.js", "metrics.js", "security.js"}
    fanout, err := logjs.LoadFanoutFromFiles(ctx, scriptPaths, logjs.Options{
        HookTimeout: "100ms",
    })
    if err != nil {
        log.Fatal(err)
    }
    defer fanout.Close(ctx)

    // Process lines through all modules
    events, errors, err := fanout.ProcessLine(ctx, logLine, "source", lineNumber)
    // events contains tagged results from all modules
}
```

### 10.4. Module introspection

```go
// Get module info
info := module.Info()
fmt.Printf("Name: %s, Tag: %s\n", info.Name, info.Tag)
fmt.Printf("Has filter: %v, Has transform: %v\n", info.HasFilter, info.HasTransform)

// Get stats after processing
stats := module.Stats()
fmt.Printf("Lines: %d, Emitted: %d, Dropped: %d\n",
    stats.LinesProcessed, stats.EventsEmitted, stats.LinesDropped)
fmt.Printf("Hook errors: %d, Timeouts: %d\n",
    stats.HookErrors, stats.HookTimeouts)
```

### 10.5. Streaming integration pattern

For real-time log processing (like `tail -f`):

```go
func streamLogs(ctx context.Context, reader io.Reader, module *logjs.Module, sink func(*logjs.Event)) error {
    scanner := bufio.NewScanner(reader)
    var lineNumber int64

    for scanner.Scan() {
        lineNumber++
        line := scanner.Text()

        events, _, err := module.ProcessLine(ctx, line, "stream", lineNumber)
        if err != nil {
            return err
        }

        for _, ev := range events {
            sink(ev)
        }
    }

    return scanner.Err()
}
```

## 11. Real-world patterns

### 11.1. Parsing different log formats

**JSON with nested fields:**
```javascript
register({
  name: "nested-json",
  parse(line, ctx) {
    const obj = log.parseJSON(line);
    if (!obj) return null;

    return {
      timestamp: log.getPath(obj, "metadata.timestamp"),
      level: log.getPath(obj, "metadata.level") || "INFO",
      message: log.getPath(obj, "payload.message"),
      trace_id: log.getPath(obj, "context.trace_id"),
      fields: obj.payload,
    };
  },
});
```

**Regex for custom formats:**
```javascript
register({
  name: "custom-format",
  parse(line, ctx) {
    // Format: [2026-01-06 12:00:00] [INFO] [service] message
    const m = log.capture(line, /^\[([^\]]+)\]\s+\[(\w+)\]\s+\[(\w+)\]\s+(.*)$/);
    if (!m) return null;

    return {
      timestamp: log.parseTimestamp(m[0]),
      level: m[1],
      service: m[2],
      message: m[3],
    };
  },
});
```

### 11.2. Multiline stack traces

```javascript
register({
  name: "java-exceptions",
  init(ctx) {
    ctx.state.buffer = log.createMultilineBuffer({
      pattern: /^\s+at |^\s+\.\.\. \d+ more|^Caused by:/,
      negate: true,  // pattern marks continuation, not start
      match: "after",
      maxLines: 100,
    });
  },
  parse(line, ctx) {
    const complete = ctx.state.buffer.add(line);
    if (!complete) return null;

    const lines = complete.split("\n");
    const firstLine = lines[0];

    // Extract exception class and message
    const m = log.capture(firstLine, /^(\w+(?:\.\w+)*Exception):\s*(.*)$/);

    return {
      level: "ERROR",
      message: m ? m[1] + ": " + m[0] : firstLine,
      fields: {
        exception_class: m ? m[0] : null,
        stack_trace: complete,
        stack_depth: lines.length,
      },
    };
  },
});
```

### 11.3. Derived metrics

```javascript
register({
  name: "latency-buckets",
  tag: "metrics",
  parse(line, ctx) {
    const obj = log.parseJSON(line);
    if (!obj) return null;

    const latencyMs = log.toNumber(obj.latency_ms);
    if (latencyMs == null) return null;

    let bucket = "fast";
    if (latencyMs > 1000) bucket = "slow";
    else if (latencyMs > 100) bucket = "medium";

    return {
      message: "request_latency",
      fields: {
        latency_ms: latencyMs,
        bucket: bucket,
        endpoint: obj.endpoint,
      },
    };
  },
});
```

### 11.4. Security event detection

```javascript
register({
  name: "suspicious-activity",
  tag: "security",
  init(ctx) {
    ctx.state.failedLogins = {};  // Track per-IP failures
  },
  parse(line, ctx) {
    const obj = log.parseJSON(line);
    if (!obj) return null;

    const msg = String(obj.message || "").toLowerCase();
    const ip = obj.client_ip;

    if (msg.includes("login failed") && ip) {
      const count = (ctx.state.failedLogins[ip] || 0) + 1;
      ctx.state.failedLogins[ip] = count;

      if (count >= 5) {
        const ev = {
          level: "WARN",
          message: "Potential brute force attack",
          fields: { ip: ip, failed_attempts: count },
        };
        log.addTag(ev, "brute_force");
        return ev;
      }
    }

    return null;
  },
});
```

## 12. Troubleshooting

### Common issues

**"script did not call register()"**

Your module file doesn't call `register({ ... })`. Every module must call it exactly once.

**"register({ name: string, ... }): name is required"**

The `register()` call is missing the `name` field. Add `name: "my-module"`.

**"register({ parse: function, ... }): parse is required"**

The `register()` call is missing the `parse` function. Add a `parse(line, ctx) { ... }` function.

**"duplicate module name"**

Two modules have the same `name`. Module names must be unique within a run.

**Infinite loop / timeout**

Your JavaScript has an infinite loop or very slow logic. Use `--js-timeout` to protect against this:

```bash
log-parse --module parser.js --js-timeout 50ms --input app.log
```

**Non-JSON lines cause errors**

Your parser might be throwing on non-JSON input. Check for null:

```javascript
parse(line, ctx) {
  const obj = log.parseJSON(line);
  if (!obj) return null;  // gracefully skip non-JSON
  // ...
}
```

**Events missing expected fields**

Check the normalization rules. Extra keys are moved to `fields`:

```javascript
// Your return:
return { level: "INFO", message: "hi", my_field: 123 };

// After normalization:
{ "level": "INFO", "message": "hi", "fields": { "my_field": 123 }, ... }
```

## 13. Further reading

For more details on specific topics:

- Example modules: `examples/log-parse/` in the devctl repo
- Design documents: `ttmp/2026/01/06/MO-007-LOG-PARSER--*/design-doc/`
- Tests: `pkg/logjs/*_test.go` for edge cases and behavior examples

