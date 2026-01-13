# log-parse examples

This directory contains small JavaScript parser modules and sample inputs for exercising `log-parse`.

Run everything from the `devctl/` module root:

```bash
cd devctl
```

## Example 1: JSON logs

```bash
cat examples/log-parse/sample-json-lines.txt | go run ./cmd/log-parse --module examples/log-parse/parser-json.js
```

## Example 2: logfmt-ish logs

```bash
cat examples/log-parse/sample-logfmt-lines.txt | go run ./cmd/log-parse --module examples/log-parse/parser-logfmt.js
```

## Example 3: regex capture

```bash
cat examples/log-parse/sample-regex-lines.txt | go run ./cmd/log-parse --module examples/log-parse/parser-regex.js
```

## Timeout demo (should not hang)

```bash
echo "x" | go run ./cmd/log-parse --module examples/log-parse/parser-infinite-loop.js --js-timeout 10ms
```

## Example 4: fan-out many modules (tagged derived streams)

This runs multiple self-contained modules on the same input stream and emits multiple tagged outputs per line.

```bash
cat examples/log-parse/sample-fanout-json-lines.txt | go run ./cmd/log-parse \
  --modules-dir examples/log-parse/modules \
  --print-pipeline \
  --stats
```

If you want structured errors as NDJSON, write them to a file so you don't mix them with the text output from `--print-pipeline`/`--stats`:

```bash
tmp_err="$(mktemp)"
cat examples/log-parse/sample-fanout-json-lines.txt | go run ./cmd/log-parse \
  --modules-dir examples/log-parse/modules \
  --errors "$tmp_err" \
  --print-pipeline \
  --stats
cat "$tmp_err"
```
