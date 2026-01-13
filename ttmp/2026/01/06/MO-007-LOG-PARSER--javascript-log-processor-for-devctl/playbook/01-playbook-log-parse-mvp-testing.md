---
Title: 'Playbook: log-parse MVP testing'
Ticket: MO-007-LOG-PARSER
Status: active
Topics:
    - backend
DocType: playbook
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: ""
LastUpdated: 2026-01-06T19:15:54.557506839-05:00
WhatFor: ""
WhenToUse: ""
---

# Playbook: log-parse MVP testing

## Purpose

Provide a repeatable manual test procedure for the `log-parse` MVP:

- verify line-by-line streaming output
- verify JS helper functions (`log.parseJSON`, `log.parseLogfmt`, `log.namedCapture`)
- verify normalization behavior (unknown keys moved into `fields`)
- verify timeout behavior for runaway JS
- provide quick example scripts and sample inputs

## Environment Assumptions

- You are in this repository with `devctl/` checked out.
- Go toolchain is installed.
- You use example scripts under `devctl/examples/log-parse/` (recommended) or you provide your own `--module` file.

## Commands

From the `devctl/` module root:

```bash
cd devctl
```

### 1) Run unit tests

```bash
go test ./... -count=1
```

Exit criteria:
- all tests pass

### 2) Smoke test: JSON example

```bash
cat examples/log-parse/sample-json-lines.txt | go run ./cmd/log-parse --module examples/log-parse/parser-json.js
```

Exit criteria:
- prints NDJSON objects for the JSON lines
- the `not json at all` line is dropped

### 3) Smoke test: logfmt example

```bash
cat examples/log-parse/sample-logfmt-lines.txt | go run ./cmd/log-parse --module examples/log-parse/parser-logfmt.js
```

Exit criteria:
- prints NDJSON objects
- `fields` contains key/value strings from the input line

### 4) Smoke test: regex example

```bash
cat examples/log-parse/sample-regex-lines.txt | go run ./cmd/log-parse --module examples/log-parse/parser-regex.js
```

Exit criteria:
- prints NDJSON objects with `level`, `message`, and `fields.service`

### 5) Streaming behavior (should print per line)

```bash
printf '%s\n' '{"msg":"one"}' '{"msg":"two"}' | go run ./cmd/log-parse --module examples/log-parse/parser-json.js
```

## Exit Criteria

- All tests pass.
- Example scripts run and emit NDJSON.
- Output appears per input line (not only when stdin closes).
- Timeout test does not hang.

## Notes

Timeout behavior demo (should not hang):

```bash
echo "x" | go run ./cmd/log-parse --module examples/log-parse/parser-infinite-loop.js --js-timeout 10ms
```
