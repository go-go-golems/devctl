---
Title: "Imported Source: js-log-parser-examples"
Ticket: MO-007-LOG-PARSER
Status: active
Topics:
  - backend
DocType: reference
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: "Upstream examples imported from /tmp/js-log-parser-examples.md; used as inspiration and test cases, not as MVP requirements."
LastUpdated: 2026-01-06T18:08:52-05:00
WhatFor: "Preserve the original upstream example scripts inside the ticket workspace for traceability."
WhenToUse: "Use to derive MVP example scripts and future test cases."
---

Excellent idea! Here's a hooks-based design where you register a module with different lifecycle hooks:

## Module Registration Signature

```typescript
// Main registration function
function register(config: LogFilterModule): void;

interface LogFilterModule {
  // Module metadata
  name: string;
  version?: string;
  description?: string;
  
  // Hooks (all optional)
  parse?: ParseHook;
  filter?: FilterHook;
  transform?: TransformHook;
  aggregate?: AggregateHook;
  output?: OutputHook;
  
  // Lifecycle hooks
  init?: InitHook;
  shutdown?: ShutdownHook;
  
  // Error handling
  onError?: ErrorHook;
}

// Hook type definitions
type ParseHook = (line: string, context: ParseContext) => LogEvent | null;
type FilterHook = (event: LogEvent, context: FilterContext) => boolean;
type TransformHook = (event: LogEvent, context: TransformContext) => LogEvent | null;
type AggregateHook = (events: LogEvent[], context: AggregateContext) => AggregateResult[];
type OutputHook = (event: LogEvent, context: OutputContext) => void | Promise<void>;
type InitHook = (context: InitContext) => void | Promise<void>;
type ShutdownHook = (context: ShutdownContext) => void | Promise<void>;
type ErrorHook = (error: Error, event: LogEvent | string, context: ErrorContext) => void;

// Context objects
interface ParseContext {
  lineNumber: number;
  source: string;
  encoding: string;
  previousLine?: string;
  cache: Map<string, any>;
}

interface FilterContext {
  timestamp: Date;
  processedCount: number;
  cache: Map<string, any>;
}

interface TransformContext {
  timestamp: Date;
  cache: Map<string, any>;
}

interface AggregateContext {
  windowStart: Date;
  windowEnd: Date;
  windowSize: string; // "1m", "5m", etc.
}

interface AggregateResult {
  key: string;
  value: any;
  timestamp: Date;
}

interface OutputContext {
  batchSize: number;
  retryCount: number;
}

interface InitContext {
  config: Record<string, any>;
  environment: Record<string, string>;
}

interface ShutdownContext {
  graceful: boolean;
  reason: string;
}

interface ErrorContext {
  hook: string;
  retryable: boolean;
}

// LogEvent structure (output of parse hook)
interface LogEvent {
  timestamp: Date;
  level?: string;
  message: string;
  fields: Record<string, any>;
  tags: string[];
  source: string;
  raw: string;
}
```

## Parser Helper Signatures

```typescript
// Common parsing patterns
function parseJSON(line: string): Record<string, any> | null;
function parseKeyValue(line: string, delimiter?: string, separator?: string): Record<string, any>;
function parseLogfmt(line: string): Record<string, any>; // key=value key2=value2
function parseCSV(line: string, headers?: string[]): Record<string, any>;
function parseSyslog(line: string): LogEvent | null;
function parseCommonLog(line: string): LogEvent | null; // Apache/Nginx common format
function parseCombinedLog(line: string): LogEvent | null; // Apache/Nginx combined format

// Regex helpers
function namedCapture(line: string, pattern: RegExp): Record<string, string> | null;
function capture(line: string, pattern: RegExp): string[] | null;

// Time parsing
function parseTimestamp(value: string, formats?: string[]): Date | null;
function detectTimestampFormat(value: string): string | null;

// Multi-line handling
function createMultilineBuffer(config: MultilineConfig): MultilineBuffer;

interface MultilineConfig {
  pattern: RegExp;
  negate?: boolean;
  match?: 'after' | 'before';
  maxLines?: number;
  timeout?: string;
}

interface MultilineBuffer {
  add(line: string): string | null; // Returns complete event when ready
  flush(): string | null;
}
```

## Example Module Scripts

```javascript
// Example 1: Simple JSON log parser
register({
  name: "json-error-filter",
  
  parse(line, context) {
    const data = parseJSON(line);
    if (!data) return null;
    
    return {
      timestamp: parseTimestamp(data.timestamp) || new Date(),
      level: data.level,
      message: data.message || data.msg || "",
      fields: data,
      tags: [],
      source: context.source,
      raw: line
    };
  },
  
  filter(event, context) {
    return equals(event.level, "ERROR");
  }
});

// Example 2: Nginx access log parser with filtering
register({
  name: "nginx-slow-requests",
  
  parse(line, context) {
    return parseCombinedLog(line);
  },
  
  filter(event, context) {
    const duration = toNumber(field(event, "request_time"));
    const status = toNumber(field(event, "status"));
    
    return duration > 1.0 && status === 200;
  },
  
  transform(event, context) {
    // Add severity based on duration
    const duration = toNumber(field(event, "request_time"));
    
    if (duration > 5.0) {
      event.fields.severity = "critical";
      addTag(event, "performance_issue");
    } else if (duration > 2.0) {
      event.fields.severity = "warning";
    }
    
    return event;
  }
});

// Example 3: Custom regex parser with stateful filtering
register({
  name: "app-error-dedup",
  
  parse(line, context) {
    // Pattern: [2024-01-06 10:30:45] ERROR [ServiceName] Message here
    const pattern = /^\[(?<timestamp>[^\]]+)\]\s+(?<level>\w+)\s+\[(?<service>[^\]]+)\]\s+(?<message>.+)$/;
    const match = namedCapture(line, pattern);
    
    if (!match) return null;
    
    return {
      timestamp: parseTimestamp(match.timestamp) || new Date(),
      level: match.level,
      message: match.message,
      fields: {
        service: match.service
      },
      tags: [],
      source: context.source,
      raw: line
    };
  },
  
  filter(event, context) {
    if (!equals(event.level, "ERROR")) return false;
    
    // Deduplicate based on service + message
    const fp = fingerprint(event, ["service", "message"]);
    const key = `seen:${fp}`;
    
    if (recall(key) !== null) {
      return false; // Skip duplicates
    }
    
    remember(key, true, "5m");
    return true;
  }
});

// Example 4: Multi-line Java stack trace parser
register({
  name: "java-exception-parser",
  
  init(context) {
    context.cache.set('buffer', createMultilineBuffer({
      pattern: /^\s+at /,  // Stack trace lines start with whitespace + "at"
      negate: true,
      match: 'after',
      maxLines: 100
    }));
  },
  
  parse(line, context) {
    const buffer = context.cache.get('buffer');
    const completeLine = buffer.add(line);
    
    if (!completeLine) return null; // Still buffering
    
    // Parse the complete multi-line event
    const firstLine = completeLine.split('\n')[0];
    const match = namedCapture(firstLine, 
      /^(?<timestamp>\S+\s+\S+)\s+(?<level>\w+)\s+(?<logger>\S+)\s+-\s+(?<message>.+)$/
    );
    
    if (!match) return null;
    
    return {
      timestamp: parseTimestamp(match.timestamp) || new Date(),
      level: match.level,
      message: match.message,
      fields: {
        logger: match.logger,
        stacktrace: completeLine
      },
      tags: ["exception"],
      source: context.source,
      raw: completeLine
    };
  },
  
  filter(event, context) {
    return hasTag(event, "exception") &&
           contains(event.message, "NullPointerException");
  }
});

// Example 5: Key-value parser with alerting
register({
  name: "payment-failure-alerter",
  
  parse(line, context) {
    // Parse logfmt: timestamp=2024-01-06T10:30:45Z level=error service=payment msg="Payment failed"
    const data = parseLogfmt(line);
    if (!data) return null;
    
    return {
      timestamp: parseTimestamp(data.timestamp) || new Date(),
      level: data.level,
      message: data.msg || data.message || "",
      fields: data,
      tags: [],
      source: context.source,
      raw: line
    };
  },
  
  filter(event, context) {
    return equals(field(event, "service"), "payment") &&
           equals(event.level, "error");
  },
  
  transform(event, context) {
    const userId = field(event, "user_id");
    const amount = field(event, "amount");
    
    // Enrich with alert metadata
    event.fields.alert_priority = amount > 1000 ? "high" : "medium";
    event.fields.requires_refund = true;
    
    addTag(event, "payment_failure");
    return event;
  },
  
  output(event, context) {
    const alertKey = `payment_alert:${field(event, "transaction_id")}`;
    
    if (throttleAlert(alertKey, "10m")) {
      alert(event, "slack", 
        `Payment failure: $${field(event, "amount")} for user ${field(event, "user_id")}`
      );
    }
  }
});

// Example 6: Aggregation example
register({
  name: "error-rate-monitor",
  
  parse(line, context) {
    return parseJSON(line);
  },
  
  filter(event, context) {
    return oneOf(event.level, ["ERROR", "FATAL", "CRITICAL"]);
  },
  
  aggregate(events, context) {
    // Count errors by service
    const counts = new Map();
    
    for (const event of events) {
      const service = field(event, "service");
      counts.set(service, (counts.get(service) || 0) + 1);
    }
    
    const results = [];
    for (const [service, count] of counts.entries()) {
      results.push({
        key: `error_rate.${service}`,
        value: count,
        timestamp: context.windowEnd
      });
    }
    
    return results;
  }
});

// Example 7: CSV log parser with transformation
register({
  name: "csv-security-logs",
  
  parse(line, context) {
    if (context.lineNumber === 1) {
      // Skip header line
      context.cache.set('headers', line.split(','));
      return null;
    }
    
    const headers = context.cache.get('headers');
    const data = parseCSV(line, headers);
    
    return {
      timestamp: parseTimestamp(data.timestamp) || new Date(),
      level: data.severity || "INFO",
      message: data.event || "",
      fields: data,
      tags: [],
      source: context.source,
      raw: line
    };
  },
  
  filter(event, context) {
    const action = field(event, "action");
    const failedAttempts = toNumber(field(event, "failed_attempts"));
    
    return equals(action, "login") && failedAttempts > 5;
  }
});

// Example 8: Sampling with state
register({
  name: "intelligent-sampler",
  
  parse(line, context) {
    return parseJSON(line);
  },
  
  filter(event, context) {
    const level = event.level;
    
    // Always keep errors
    if (oneOf(level, ["ERROR", "FATAL", "CRITICAL"])) {
      return true;
    }
    
    // Sample debug logs at 1%
    if (equals(level, "DEBUG")) {
      return sampleRate(0.01);
    }
    
    // Rate limit info logs per service
    const service = field(event, "service");
    const key = `rate:${service}:info`;
    return rateLimit(key, 1000, "1m"); // 1000 per minute per service
  }
});

// Example 9: Error handling hook
register({
  name: "resilient-parser",
  
  parse(line, context) {
    const data = parseJSON(line);
    if (!data) {
      throw new Error("Invalid JSON");
    }
    return data;
  },
  
  onError(error, eventOrLine, context) {
    // Log parse failures to a dead letter queue
    if (context.hook === "parse") {
      console.error(`Parse error at line ${context.lineNumber}: ${error.message}`);
      // Could write to a separate file, send to monitoring, etc.
    }
  }
});

// Example 10: Init/shutdown lifecycle
register({
  name: "database-logger",
  
  async init(context) {
    // Set up database connection, HTTP clients, etc.
    const dbUrl = context.environment.DATABASE_URL;
    context.cache.set('db', await connectToDatabase(dbUrl));
  },
  
  parse(line, context) {
    return parseJSON(line);
  },
  
  filter(event, context) {
    return equals(event.level, "ERROR");
  },
  
  async output(event, context) {
    const db = context.cache.get('db');
    await db.insert('error_logs', {
      timestamp: event.timestamp,
      level: event.level,
      message: event.message,
      fields: JSON.stringify(event.fields)
    });
  },
  
  async shutdown(context) {
    const db = context.cache.get('db');
    if (db) {
      await db.close();
    }
  }
});
```

This hooks-based design gives you:
- **Separation of concerns** - each hook does one thing
- **Raw line parsing** - start with unparsed strings
- **Lifecycle management** - init/shutdown for resources
- **Error handling** - dedicated error hooks
- **Stateful processing** - cache available across hooks
- **Flexible pipelines** - use only the hooks you need
- **Async support** - for I/O operations

Much cleaner than trying to do everything in one function!
