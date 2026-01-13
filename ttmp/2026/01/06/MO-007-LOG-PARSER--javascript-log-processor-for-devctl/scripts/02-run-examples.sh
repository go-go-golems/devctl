#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../../../../../.." && pwd)"
cd "$repo_root/devctl"

echo "== JSON example =="
cat examples/log-parse/sample-json-lines.txt | go run ./cmd/log-parse --module examples/log-parse/parser-json.js >/dev/null

echo "== logfmt example =="
cat examples/log-parse/sample-logfmt-lines.txt | go run ./cmd/log-parse --module examples/log-parse/parser-logfmt.js >/dev/null

echo "== regex example =="
cat examples/log-parse/sample-regex-lines.txt | go run ./cmd/log-parse --module examples/log-parse/parser-regex.js >/dev/null

echo "OK"
