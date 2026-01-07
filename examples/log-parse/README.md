# log-parse examples

This directory contains small JavaScript parser modules and sample inputs for exercising `log-parse`.

Run everything from the `devctl/` module root:

```bash
cd devctl
```

## Example 1: JSON logs

```bash
cat examples/log-parse/sample-json-lines.txt | go run ./cmd/log-parse --js examples/log-parse/parser-json.js
```

## Example 2: logfmt-ish logs

```bash
cat examples/log-parse/sample-logfmt-lines.txt | go run ./cmd/log-parse --js examples/log-parse/parser-logfmt.js
```

## Example 3: regex capture

```bash
cat examples/log-parse/sample-regex-lines.txt | go run ./cmd/log-parse --js examples/log-parse/parser-regex.js
```

## Timeout demo (should not hang)

```bash
echo "x" | go run ./cmd/log-parse --js examples/log-parse/parser-infinite-loop.js --js-timeout 10ms
```

