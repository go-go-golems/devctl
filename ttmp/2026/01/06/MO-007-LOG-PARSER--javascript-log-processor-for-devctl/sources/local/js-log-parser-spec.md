---
Title: "Imported Source: js-log-parser-spec"
Ticket: MO-007-LOG-PARSER
Status: active
Topics:
  - backend
DocType: reference
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: "Upstream (over-scoped) LogFlow spec imported from /tmp/js-log-parser-spec.md; used as design input, not as MVP requirements."
LastUpdated: 2026-01-06T18:08:52-05:00
WhatFor: "Preserve the original upstream spec text inside the ticket workspace for traceability."
WhenToUse: "Use as reference while trimming scope; do not treat as an implementation checklist."
---

# Imported Source (Read-Only)

# LogFlow: JavaScript-Based Log Processing System

## Overview

LogFlow is a programmable log processing pipeline that allows developers to write JavaScript modules to parse, filter, transform, and route log data. Instead of learning complex YAML DSLs or configuration languages, you write simple JavaScript functions using a rich set of provided helpers.

## Design Philosophy

1. **Code over Configuration**: Write JavaScript instead of learning YAML syntax
2. **Composable Hooks**: Break processing into discrete lifecycle stages
3. **Batteries Included**: Rich standard library of parsing and filtering helpers
4. **Stateful by Default**: Built-in caching and rate limiting primitives
5. **Fail Gracefully**: Error hooks and retry logic baked in
6. **Performance First**: Efficient batch processing and async I/O support

## System Architecture

```
┌─────────────┐
│   Sources   │  (Files, stdin, syslog, network, etc.)
└──────┬──────┘
       │ Raw log lines
       ▼
┌─────────────┐
│    PARSE    │  line: string → LogEvent | null
└──────┬──────┘
       │ Structured events
       ▼
┌─────────────┐
│   FILTER    │  event: LogEvent → boolean
└──────┬──────┘
       │ Filtered events
       ▼
┌─────────────┐
│ TRANSFORM   │  event: LogEvent → LogEvent | null
└──────┬──────┘
       │ Enriched events
       ▼
┌─────────────┐
│  AGGREGATE  │  events: LogEvent[] → AggregateResult[]
└──────┬──────┘
       │ Aggregated metrics
       ▼
┌─────────────┐
│   OUTPUT    │  event: LogEvent → void
└──────┬──────┘
       │
       ▼
┌─────────────┐
│ Destinations│  (Files, databases, webhooks, etc.)
└─────────────┘
```

## Core Concepts

### 1. Modules

A module is a JavaScript object that you register with LogFlow. It defines how to process logs through various hooks:

```javascript
register({
  name: "my-log-processor",
  version: "1.0.0",
  description: "Processes application logs",
  
  // Optional hooks
  parse: (line, context) => { /* ... */ },
  filter: (event, context) => { /* ... */ },
  transform: (event, context) => { /* ... */ },
  aggregate: (events, context) => { /* ... */ },
  output: (event, context) => { /* ... */ },
  
  // Lifecycle hooks
  init: (context) => { /* ... */ },
  shutdown: (context) => { /* ... */ },
  
  // Error handling
  onError: (error, eventOrLine, context) => { /* ... */ }
});
```

### 2. Hooks

Hooks are functions called at different stages of log processing:

#### **parse** - Convert raw strings to structured events
- **Input**: Raw log line (string)
- **Output**: LogEvent object or null (to skip)
- **Purpose**: Parse unstructured logs into structured data
- **When to use**: Always - this is where you define your log format

#### **filter** - Decide which events to keep
- **Input**: Parsed LogEvent
- **Output**: boolean (true = keep, false = discard)
- **Purpose**: Reduce volume by filtering out unwanted logs
- **When to use**: When you only want specific logs (errors, specific services, etc.)

#### **transform** - Modify or enrich events
- **Input**: Filtered LogEvent
- **Output**: Modified LogEvent or null (to discard)
- **Purpose**: Add fields, normalize data, enrich with external info
- **When to use**: When you need to add context or modify log structure

#### **aggregate** - Compute metrics over time windows
- **Input**: Array of LogEvents from a time window
- **Output**: Array of AggregateResults (key-value metrics)
- **Purpose**: Calculate rates, counts, percentiles, etc.
- **When to use**: When you need metrics or statistics

#### **output** - Send events to destinations
- **Input**: Final processed LogEvent
- **Output**: void (or Promise<void> for async)
- **Purpose**: Write to files, databases, APIs, etc.
- **When to use**: When default outputs aren't sufficient

#### **init** - Initialize resources
- **Input**: InitContext with config and environment
- **Output**: void (or Promise<void>)
- **Purpose**: Set up connections, load data, initialize state
- **When to use**: When you need database connections, HTTP clients, etc.

#### **shutdown** - Clean up resources
- **Input**: ShutdownContext with graceful flag
- **Output**: void (or Promise<void>)
- **Purpose**: Close connections, flush buffers, cleanup
- **When to use**: When you have resources to release

#### **onError** - Handle errors
- **Input**: Error object, the event/line that caused it, ErrorContext
- **Output**: void
- **Purpose**: Log errors, send to dead letter queue, retry logic
- **When to use**: When you need custom error handling

### 3. LogEvent Structure

After parsing, all logs are represented as LogEvent objects:

```javascript
{
  timestamp: Date,           // When the log occurred
  level: string,            // "DEBUG", "INFO", "WARN", "ERROR", "FATAL"
  message: string,          // The main log message
  fields: Object,           // Arbitrary key-value data
  tags: string[],           // Labels for categorization
  source: string,           // Where the log came from
  raw: string              // Original unparsed line
}
```

### 4. Context Objects

Each hook receives a context object with useful metadata and utilities:

- **cache**: Map for storing state between invocations
- **lineNumber**: Current line being processed (parse only)
- **processedCount**: Total events processed (filter only)
- **timestamp**: Current processing time
- **source**: Source file/stream identifier
- **batchSize**: Number of events in current batch (output only)
- **retryCount**: Number of retry attempts (output only)

### 5. State Management

LogFlow provides built-in state management through the context cache and helper functions:

```javascript
// In-memory cache (per module instance)
context.cache.set('key', value);
const value = context.cache.get('key');

// Persistent state with TTL
remember('key', value, '5m');  // Store for 5 minutes
const value = recall('key');    // Retrieve

// Counters
increment('counter_name', 5);   // Add 5
decrement('counter_name', 1);   // Subtract 1
```

## Standard Library

LogFlow provides a comprehensive standard library organized by functionality:

### Parsing Functions

Parse common log formats:
- `parseJSON(line)` - Parse JSON logs
- `parseKeyValue(line, delimiter, separator)` - Parse key=value logs
- `parseLogfmt(line)` - Parse logfmt format
- `parseCSV(line, headers)` - Parse CSV logs
- `parseSyslog(line)` - Parse syslog format
- `parseCommonLog(line)` - Parse Apache/Nginx common format
- `parseCombinedLog(line)` - Parse Apache/Nginx combined format

Extract patterns:
- `namedCapture(line, pattern)` - Regex with named groups → object
- `capture(line, pattern)` - Regex capture groups → array
- `extract(str, pattern, group)` - Extract single match
- `extractAll(str, pattern)` - Extract all matches

### Field Access Functions

Navigate event data:
- `field(event, path)` - Get field value (supports dot notation)
- `hasField(event, path)` - Check if field exists
- `getPath(obj, path)` - Navigate nested objects
- `hasPath(obj, path)` - Check nested path exists

### Comparison Functions

Test values:
- `equals(a, b)` - Equality check
- `contains(haystack, needle)` - Substring/array membership
- `startsWith(str, prefix)` - String prefix check
- `endsWith(str, suffix)` - String suffix check
- `between(value, min, max)` - Numeric range check
- `oneOf(value, options)` - Value in array

### Pattern Matching Functions

Match against patterns:
- `match(value, pattern)` - Single regex/string match
- `matchAny(value, patterns)` - OR logic for multiple patterns
- `matchAll(value, patterns)` - AND logic for multiple patterns

### Time Functions

Work with timestamps:
- `parseTimestamp(value, formats)` - Parse time strings
- `detectTimestampFormat(value)` - Auto-detect time format
- `parseTime(value, format)` - Parse with specific format
- `isAfter(date, reference)` - Compare dates
- `isBefore(date, reference)` - Compare dates
- `withinLast(date, duration)` - Check if recent ("5m", "2h", "1d")
- `age(date)` - Milliseconds since timestamp

### String Functions

Manipulate text:
- `normalize(str)` - Lowercase, trim, normalize whitespace
- `tokenize(str, delimiter)` - Split into tokens

### Numeric Functions

Handle numbers:
- `toNumber(value)` - Safe conversion to number
- `isNumeric(value)` - Check if parseable as number
- `percentile(value, min, max)` - Calculate percentile

### Array Functions

Work with collections:
- `some(arr, predicate)` - Test if any element matches
- `every(arr, predicate)` - Test if all elements match
- `count(arr, predicate)` - Count matching elements

### IP Address Functions

Validate and check IPs:
- `isIP(value)` - Check if valid IP
- `isIPv4(value)` - Check if IPv4
- `isIPv6(value)` - Check if IPv6
- `inCIDR(ip, cidr)` - Check if IP in CIDR range
- `isPrivateIP(ip)` - Check if private/internal IP

### Rate Limiting Functions

Control event flow:
- `rateLimit(key, limit, window)` - Allow N events per time window
- `sampleRate(rate)` - Random sampling (0.0 to 1.0)

### State Functions

Persistent storage:
- `remember(key, value, ttl)` - Store value with expiration
- `recall(key)` - Retrieve stored value
- `increment(key, amount)` - Atomic counter increment
- `decrement(key, amount)` - Atomic counter decrement

### Tagging Functions

Manipulate event tags:
- `addTag(event, tag)` - Add a tag to event
- `removeTag(event, tag)` - Remove a tag from event
- `hasTag(event, tag)` - Check if event has tag

### Hashing Functions

Generate fingerprints:
- `hash(value, algorithm)` - Hash value (md5, sha1, sha256)
- `fingerprint(event, fields)` - Generate event fingerprint from fields

### Alert Functions

Send notifications:
- `alert(event, channel, message)` - Send alert to channel
- `throttleAlert(key, duration)` - Prevent alert spam

### Multi-line Functions

Handle multi-line logs:
- `createMultilineBuffer(config)` - Buffer for multi-line events
  - Config: `{pattern, negate, match, maxLines, timeout}`
  - Returns buffer with: `add(line)` and `flush()` methods

## Use Cases and Examples

### Use Case 1: Error Monitoring

**Goal**: Capture all errors, deduplicate them, and alert on critical ones.

```javascript
register({
  name: "error-monitor",
  
  parse(line, context) {
    return parseJSON(line);
  },
  
  filter(event, context) {
    return oneOf(event.level, ["ERROR", "FATAL"]);
  },
  
  transform(event, context) {
    // Deduplicate based on error signature
    const sig = fingerprint(event, ["service", "error_type", "message"]);
    const key = `seen:${sig}`;
    
    if (recall(key) !== null) {
      return null; // Skip duplicate
    }
    
    remember(key, true, "10m");
    
    // Add severity
    if (equals(event.level, "FATAL")) {
      event.fields.severity = "critical";
      addTag(event, "page_oncall");
    }
    
    return event;
  },
  
  output(event, context) {
    if (hasTag(event, "page_oncall")) {
      alert(event, "pagerduty", `FATAL: ${event.message}`);
    }
  }
});
```

### Use Case 2: Performance Monitoring

**Goal**: Track slow requests and calculate percentiles.

```javascript
register({
  name: "perf-monitor",
  
  parse(line, context) {
    return parseCombinedLog(line);
  },
  
  filter(event, context) {
    const duration = toNumber(field(event, "request_time"));
    return duration !== null && duration > 1.0; // Slower than 1s
  },
  
  transform(event, context) {
    const duration = toNumber(field(event, "request_time"));
    const endpoint = field(event, "request");
    
    // Add performance category
    if (duration > 10) {
      event.fields.perf_category = "critical";
    } else if (duration > 5) {
      event.fields.perf_category = "warning";
    } else {
      event.fields.perf_category = "slow";
    }
    
    return event;
  },
  
  aggregate(events, context) {
    const byEndpoint = new Map();
    
    for (const event of events) {
      const endpoint = field(event, "request");
      if (!byEndpoint.has(endpoint)) {
        byEndpoint.set(endpoint, []);
      }
      byEndpoint.get(endpoint).push(
        toNumber(field(event, "request_time"))
      );
    }
    
    const results = [];
    for (const [endpoint, durations] of byEndpoint.entries()) {
      durations.sort((a, b) => a - b);
      const p95 = durations[Math.floor(durations.length * 0.95)];
      
      results.push({
        key: `perf.${endpoint}.p95`,
        value: p95,
        timestamp: context.windowEnd
      });
    }
    
    return results;
  }
});
```

### Use Case 3: Security Monitoring

**Goal**: Detect brute force login attempts.

```javascript
register({
  name: "security-monitor",
  
  parse(line, context) {
    const match = namedCapture(line, 
      /^(?<timestamp>\S+\s+\S+)\s+(?<level>\w+)\s+\[(?<module>\w+)\]\s+(?<message>.+)$/
    );
    
    if (!match) return null;
    
    return {
      timestamp: parseTimestamp(match.timestamp) || new Date(),
      level: match.level,
      message: match.message,
      fields: { module: match.module },
      tags: [],
      source: context.source,
      raw: line
    };
  },
  
  filter(event, context) {
    return contains(event.message, "authentication failed");
  },
  
  transform(event, context) {
    // Extract IP and username
    const ip = extract(event.message, /ip=(\S+)/, 1);
    const user = extract(event.message, /user=(\S+)/, 1);
    
    event.fields.source_ip = ip;
    event.fields.username = user;
    
    // Track failed attempts per IP
    const key = `failed_auth:${ip}`;
    const attempts = increment(key, 1);
    event.fields.attempt_count = attempts;
    
    // Flag suspicious activity
    if (attempts > 5) {
      addTag(event, "brute_force");
      event.fields.threat_level = "high";
    }
    
    return event;
  },
  
  output(event, context) {
    if (hasTag(event, "brute_force")) {
      const ip = field(event, "source_ip");
      const alertKey = `alert:brute_force:${ip}`;
      
      if (throttleAlert(alertKey, "30m")) {
        alert(event, "security_team", 
          `Brute force detected from ${ip}`
        );
      }
    }
  }
});
```

### Use Case 4: Application-Specific Parsing

**Goal**: Parse custom application format with correlation IDs.

```javascript
register({
  name: "app-log-parser",
  
  parse(line, context) {
    // Format: [trace_id] timestamp LEVEL service: message {json_context}
    const pattern = /^\[(?<trace_id>[^\]]+)\]\s+(?<timestamp>\S+)\s+(?<level>\w+)\s+(?<service>\w+):\s+(?<message>[^{]+)(?<context>\{.+\})?$/;
    
    const match = namedCapture(line, pattern);
    if (!match) return null;
    
    const event = {
      timestamp: parseTimestamp(match.timestamp) || new Date(),
      level: match.level,
      message: match.message.trim(),
      fields: {
        trace_id: match.trace_id,
        service: match.service
      },
      tags: [],
      source: context.source,
      raw: line
    };
    
    // Parse JSON context if present
    if (match.context) {
      const ctx = parseJSON(match.context);
      if (ctx) {
        Object.assign(event.fields, ctx);
      }
    }
    
    return event;
  },
  
  filter(event, context) {
    // Keep errors and specific trace IDs we're debugging
    if (equals(event.level, "ERROR")) return true;
    
    const debugTraces = ["abc-123", "def-456"];
    return oneOf(field(event, "trace_id"), debugTraces);
  }
});
```

### Use Case 5: CSV Processing with Headers

**Goal**: Process CSV logs with dynamic headers.

```javascript
register({
  name: "csv-processor",
  
  parse(line, context) {
    // First line contains headers
    if (context.lineNumber === 1) {
      const headers = line.split(',').map(h => h.trim());
      context.cache.set('headers', headers);
      return null; // Skip header line
    }
    
    const headers = context.cache.get('headers');
    if (!headers) {
      throw new Error("Headers not found");
    }
    
    const data = parseCSV(line, headers);
    
    return {
      timestamp: parseTimestamp(data.timestamp) || new Date(),
      level: data.level || "INFO",
      message: data.message || "",
      fields: data,
      tags: [],
      source: context.source,
      raw: line
    };
  }
});
```

### Use Case 6: Multi-line Stack Traces

**Goal**: Combine Java exception stack traces into single events.

```javascript
register({
  name: "java-exception-parser",
  
  init(context) {
    // Create multi-line buffer
    context.cache.set('buffer', createMultilineBuffer({
      pattern: /^\s+at /,  // Continuation lines start with "at"
      negate: true,        // Pattern matches start of NEW event
      match: 'after',      // Include matching line AFTER pattern
      maxLines: 200,       // Max lines per event
      timeout: '5s'        // Flush if no new lines for 5s
    }));
  },
  
  parse(line, context) {
    const buffer = context.cache.get('buffer');
    const completeLine = buffer.add(line);
    
    if (!completeLine) return null; // Still accumulating
    
    // Parse first line of exception
    const firstLine = completeLine.split('\n')[0];
    const match = namedCapture(firstLine,
      /^(?<timestamp>\S+)\s+(?<level>\w+)\s+\[(?<thread>[^\]]+)\]\s+(?<logger>\S+)\s+-\s+(?<message>.+)$/
    );
    
    if (!match) return null;
    
    return {
      timestamp: parseTimestamp(match.timestamp) || new Date(),
      level: match.level,
      message: match.message,
      fields: {
        thread: match.thread,
        logger: match.logger,
        stacktrace: completeLine
      },
      tags: ["exception"],
      source: context.source,
      raw: completeLine
    };
  },
  
  filter(event, context) {
    return hasTag(event, "exception");
  }
});
```

## Configuration and Deployment

### Module Loading

Modules are loaded from JavaScript files:

```bash
# Single module
logflow --module ./filters/errors.js --input /var/log/app.log

# Multiple modules (pipeline)
logflow --module ./parse.js --module ./filter.js --module ./output.js

# Directory of modules
logflow --modules ./modules/ --input /var/log/app.log
```

### Command Line Options

```bash
logflow [options]

Options:
  --module, -m <file>       Load a module file (can specify multiple)
  --modules-dir <dir>       Load all .js files from directory
  --input, -i <source>      Input source (file, stdin, tcp://host:port)
  --output, -o <dest>       Default output destination
  --config, -c <file>       Load configuration file
  --env <file>              Load environment variables from file
  --workers <n>             Number of worker threads (default: CPU cores)
  --batch-size <n>          Events per batch (default: 1000)
  --buffer-size <n>         Max events to buffer (default: 10000)
  --window <duration>       Aggregation window size (default: 1m)
  --verbose, -v             Verbose logging
  --validate                Validate modules without running
```

### Configuration File

For complex setups, use a configuration file:

```javascript
// logflow.config.js
module.exports = {
  inputs: [
    { type: 'file', path: '/var/log/app/*.log', follow: true },
    { type: 'tcp', host: '0.0.0.0', port: 5140 }
  ],
  
  modules: [
    './modules/parse.js',
    './modules/filter.js',
    './modules/enrich.js'
  ],
  
  outputs: [
    { type: 'file', path: '/var/log/filtered/output.log' },
    { type: 'elasticsearch', host: 'localhost:9200' }
  ],
  
  settings: {
    workers: 4,
    batchSize: 1000,
    bufferSize: 10000,
    aggregateWindow: '5m'
  },
  
  environment: {
    DATABASE_URL: 'postgres://...',
    API_KEY: process.env.API_KEY
  }
};
```

### Environment Variables

Modules can access environment variables through the init context:

```javascript
register({
  name: "db-logger",
  
  init(context) {
    const dbUrl = context.environment.DATABASE_URL;
    const apiKey = context.environment.API_KEY;
    
    // Initialize connections
    context.cache.set('db', connectDB(dbUrl));
    context.cache.set('apiKey', apiKey);
  }
});
```

## Performance Considerations

### Batching

LogFlow processes events in batches for efficiency:
- Parse, filter, and transform hooks are called per-event
- Aggregate hooks receive batches
- Output hooks can be called per-event or per-batch (configurable)

### Async Operations

Output and lifecycle hooks support async/await:

```javascript
register({
  name: "async-output",
  
  async output(event, context) {
    await fetch('https://api.example.com/logs', {
      method: 'POST',
      body: JSON.stringify(event)
    });
  }
});
```

### Worker Threads

LogFlow can spawn multiple worker threads to process logs in parallel:
- Each worker runs its own instance of your modules
- State (remember/recall) is shared across workers
- Useful for high-volume log processing

### Memory Management

- Context cache is per-module instance
- Use `remember()` with TTL to avoid unbounded memory growth
- Large objects in cache should be cleaned up in shutdown hook

## Error Handling

### Parse Errors

If parse hook throws, the line is skipped and onError is called:

```javascript
register({
  name: "resilient-parser",
  
  parse(line, context) {
    const data = parseJSON(line);
    if (!data) throw new Error("Invalid JSON");
    return data;
  },
  
  onError(error, line, context) {
    if (context.hook === "parse") {
      // Log to dead letter queue
      console.error(`Parse error: ${error.message}`);
      console.error(`Line: ${line}`);
    }
  }
});
```

### Filter/Transform Errors

If filter/transform throws, event is skipped and onError is called.

### Output Errors

If output throws, LogFlow will retry based on configuration:
- Retries with exponential backoff
- Dead letter queue after max retries
- onError called on each failure

## Best Practices

### 1. Start Simple

Begin with just parse and filter hooks:

```javascript
register({
  name: "simple-filter",
  parse: (line) => parseJSON(line),
  filter: (event) => equals(event.level, "ERROR")
});
```

### 2. Use Type Checking

Validate fields before processing:

```javascript
filter(event, context) {
  const status = toNumber(field(event, "status_code"));
  if (status === null) return false; // Skip if not a number
  
  return between(status, 400, 599);
}
```

### 3. Handle Missing Fields

Always check if fields exist:

```javascript
filter(event, context) {
  if (!hasField(event, "user_id")) return false;
  
  const userId = field(event, "user_id");
  return userId !== null && userId !== "";
}
```

### 4. Use Descriptive Names

Name your modules and cache keys clearly:

```javascript
register({
  name: "payment-error-detector",  // Not "filter1"
  
  filter(event, context) {
    const key = `rate_limit:payment_errors:${field(event, "user_id")}`;
    // Not just "rl"
  }
});
```

### 5. Limit State Growth

Always use TTLs with remember():

```javascript
// Good
remember(`seen:${eventId}`, true, "10m");

// Bad - leaks memory
remember(`seen:${eventId}`, true); // No TTL!
```

### 6. Test Incrementally

Test each hook independently before combining:

```bash
# Test just parsing
logflow --module parse.js --input test.log --output parsed.json

# Add filtering
logflow --module parse.js --module filter.js --input test.log
```

### 7. Use onError for Debugging

Log what's happening in your error handler:

```javascript
onError(error, eventOrLine, context) {
  console.error(`[${context.hook}] ${error.message}`);
  if (context.hook === "parse") {
    console.error(`Failed line: ${eventOrLine}`);
  } else {
    console.error(`Failed event: ${JSON.stringify(eventOrLine)}`);
  }
}
```

### 8. Clean Up Resources

Always implement shutdown for connections:

```javascript
async init(context) {
  context.cache.set('db', await connectDB());
},

async shutdown(context) {
  const db = context.cache.get('db');
  if (db) await db.close();
}
```

## Comparison with Other Tools

### vs. Logstash

**Logstash**: Config-based, Ruby filters, JVM-based
**LogFlow**: Code-based, JavaScript, Node.js/V8

Advantages:
- Easier to test (just JavaScript)
- Better IDE support (autocomplete, refactoring)
- More flexible (full programming language)
- Lower memory footprint

### vs. Fluentd

**Fluentd**: Config-based, Ruby plugins, buffer-focused
**LogFlow**: Code-based, JavaScript hooks, pipeline-focused

Advantages:
- Simpler mental model (hooks vs. plugins)
- No need to package/distribute plugins
- Easier to compose multiple processing steps

### vs. Vector

**Vector**: Config-based, Rust-native, VRL language
**LogFlow**: Code-based, JavaScript, standard language

Advantages:
- No new language to learn (VRL)
- Larger ecosystem (npm packages)
- More developers know JavaScript

## Future Extensions

Possible enhancements:

1. **TypeScript Support**: Type-safe module definitions
2. **Hot Reload**: Reload modules without restart
3. **Module Registry**: Share modules via npm
4. **Visual Debugger**: Step through events in UI
5. **Performance Profiler**: Identify slow filters
6. **Schema Validation**: Validate event structure
7. **Machine Learning**: Anomaly detection hooks
8. **Distributed Mode**: Process across multiple machines

## Conclusion

LogFlow gives you the full power of JavaScript to process logs, with a clean hook-based architecture and rich standard library. Instead of fighting with YAML and limited DSLs, you write real code that you can test, debug, and version control like any other software.
