---
Title: Log-Parser Documentation Writing Diary
Ticket: MO-007-LOG-PARSER
Status: active
Topics:
    - backend
    - documentation
DocType: reference
Intent: short-term
Owners: []
RelatedFiles:
    - Path: cmd/log-parse/main.go
      Note: CLI implementation reviewed for flag documentation
    - Path: examples/log-parse
      Note: Example modules and sample data used to verify docs
    - Path: pkg/doc/topics/log-parse-guide.md
      Note: Comprehensive developer guide created in this diary
    - Path: pkg/logjs/fanout.go
      Note: Multi-module fan-out runner
    - Path: pkg/logjs/helpers.go
      Note: JavaScript helper functions (log.* API)
    - Path: pkg/logjs/module.go
      Note: Core module loader and hook execution logic
    - Path: pkg/logjs/types.go
      Note: Event and ErrorRecord type definitions
ExternalSources: []
Summary: Step-by-step diary of creating the comprehensive log-parse developer guide.
LastUpdated: 2026-01-08T09:00:00-05:00
WhatFor: Track the creation of log-parse documentation and capture design decisions made during writing.
WhenToUse: Reference when updating the log-parse guide or understanding documentation scope decisions.
---


# Log-Parser Documentation Writing Diary

## Goal

Create a comprehensive developer guide for the log-parse feature that covers:
1. Getting started (quick start, first parser)
2. Module API reference (register, hooks, context)
3. Helper API reference (log.* functions)
4. Multi-module fan-out pipeline
5. CLI reference
6. Go integration guide
7. Real-world patterns and troubleshooting

## Step 1: Codebase Exploration and Understanding

I started by exploring the log-parse implementation to understand the full feature surface. This exploration focused on understanding the actual behavior rather than relying solely on design docs.

### What I did
- Read `pkg/logjs/module.go` for the core module loader and hook execution logic
- Read `pkg/logjs/types.go` for Event and ErrorRecord schemas
- Read `pkg/logjs/helpers.go` for the JavaScript helper API (embedded in helpersJS constant)
- Read `pkg/logjs/fanout.go` for multi-module execution and tag injection
- Read `cmd/log-parse/main.go` for CLI flags and usage
- Read all example parsers in `examples/log-parse/`
- Read existing design docs in `ttmp/.../design-doc/`
- Ran all examples to verify they work as documented

### What I learned

**Module contract:**
- `register({ name, tag?, parse, filter?, transform?, init?, shutdown?, onError? })`
- `name` is required and must be unique
- `tag` defaults to `name` if not specified
- `parse` is the only required hook function

**Hook semantics:**
- `parse` can return null (drop), string (shorthand), object, or array (multiple events)
- `filter` returns boolean (true=keep, false=drop)
- `transform` can return null (drop), object, or array
- All hooks receive `(value, ctx)` where ctx has `hook`, `source`, `lineNumber`, `now`, `state`

**Helper API (log.*):**
- Parsing: `parseJSON`, `parseLogfmt`, `parseKeyValue`
- Regex: `capture`, `namedCapture`, `extract`
- Object: `getPath`/`field`, `hasPath`
- Tags: `addTag`, `removeTag`, `hasTag`
- Conversion: `toNumber`, `parseTimestamp`
- Multiline: `createMultilineBuffer`

**Event normalization:**
- Extra top-level keys are moved to `fields`
- Empty tags are filtered out
- `timestamp` accepts Date objects (converted via toISOString) or strings
- Default level is "INFO", default message is raw line

**Fan-out behavior:**
- Each module runs independently on same input
- Tag injection adds `_tag` and `_module` to fields
- Error isolation: one module's error doesn't affect others
- State isolation: each module has its own `ctx.state`

### What was tricky
- The helper API is embedded as a JS string constant in Go, so I had to carefully read the inline JS code
- `log.parseTimestamp` has Go-side enhancement via dateparse library
- Multiline buffer only supports `match: "after"` in current iteration

## Step 2: Review Documentation Guidelines

Read `glaze help how-to-write-good-documentation-pages` and existing devctl docs:
- `devctl-user-guide.md` - good example of structure and diagrams
- `devctl-plugin-authoring.md` - comprehensive reference with examples

Key takeaways for style:
- Topic-focused intro paragraphs for each section
- Tables for flags and quick reference
- ASCII diagrams for architecture
- Copy-paste-ready code examples
- Real-world patterns section
- Troubleshooting section

## Step 3: Write Comprehensive Developer Guide

Created `pkg/doc/topics/log-parse-guide.md` with the following structure:

1. **Introduction** - What log-parse does, core properties
2. **Quick start** - First parser in 5 minutes
3. **Module contract** - register(), name, tag, context
4. **Hook semantics** - parse/filter/transform return values
5. **Event schema** - Normalization rules, timestamp handling
6. **Helper API** - Complete log.* reference
7. **Multi-module fan-out** - Why fan-out, loading, tag injection
8. **CLI reference** - All flags and commands
9. **Error handling** - Isolation, error records, timeouts
10. **Go integration** - pkg/logjs API for embedding
11. **Real-world patterns** - JSON, regex, multiline, metrics, security
12. **Troubleshooting** - Common issues and fixes
13. **Design rationale** - Why JavaScript, goja, fan-out, sync-only

### What worked
- Started with the quick start section to ensure readers can get running fast
- Used tables for reference material (flags, normalization rules)
- Included complete working examples that mirror the examples/ directory
- Added Go integration section for developers embedding log-parse

### What was tricky to get right
- Balancing depth vs. readability for the helper API section
- Deciding how much implementation detail to expose in Go integration
- Organizing the real-world patterns to cover the most common cases

### Code review instructions
- Start with `pkg/doc/topics/log-parse-guide.md`
- Verify YAML frontmatter matches other docs in topics/
- Check that code examples compile/run
- Verify flag names match actual CLI

## Step 4: Verification

Ran all examples from the documentation:

```bash
# JSON parser example
cat examples/log-parse/sample-json-lines.txt | go run ./cmd/log-parse --module examples/log-parse/parser-json.js

# Pretty-print format
echo '{"ts":"...","level":"INFO","msg":"startup","service":"api"}' | go run ./cmd/log-parse --module examples/log-parse/parser-json.js --format pretty

# Multi-module fan-out
cat examples/log-parse/sample-fanout-json-lines.txt | go run ./cmd/log-parse --modules-dir examples/log-parse/modules --print-pipeline --stats

# Validate command
go run ./cmd/log-parse validate --modules-dir examples/log-parse/modules

# Timeout protection
echo "x" | go run ./cmd/log-parse --module examples/log-parse/parser-infinite-loop.js --js-timeout 10ms
```

All examples work correctly. Also ran unit tests:

```bash
go test ./pkg/logjs/... -v
# All 11 tests pass
```

## What should be done in the future

1. **Add more real-world examples**: Common log formats like nginx access logs, syslog, AWS CloudWatch
2. **Multiline documentation**: Currently only `match: "after"` is supported; document when other modes are added
3. **Performance guidance**: When to use `--js-timeout`, memory considerations for large buffers
4. **Integration with devctl logs**: When this integration lands, update the guide
5. **Sandboxed require()**: When MO-008 lands, document how to share code between modules

## Related

- Design doc: `design-doc/01-mvp-design-javascript-log-parser-goja.md`
- Fan-out design: `design-doc/02-next-step-design-multi-script-pipeline-for-log-parse.md`
- Testing playbook: `playbook/01-playbook-log-parse-mvp-testing.md`
- Documentation investigation: `analysis/01-long-term-documentation-investigation.md`
