#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../../../../../.." && pwd)"
cd "$repo_root/devctl"

echo "== fan-out modules-dir demo =="
tmp_err="$(mktemp)"
trap 'rm -f "$tmp_err"' EXIT

cat examples/log-parse/sample-fanout-json-lines.txt | go run ./cmd/log-parse \
  --modules-dir examples/log-parse/modules \
  --errors "$tmp_err" \
  --print-pipeline \
  --stats \
  >/dev/null

if [[ -s "$tmp_err" ]]; then
  echo "Errors captured (unexpected in this demo):"
  cat "$tmp_err"
  exit 1
fi

echo "OK"

