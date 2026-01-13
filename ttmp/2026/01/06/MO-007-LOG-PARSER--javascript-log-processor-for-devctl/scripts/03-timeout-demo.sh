#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../../../../../.." && pwd)"
cd "$repo_root/devctl"

echo "This should exit quickly and print nothing:"
echo "x" | go run ./cmd/log-parse --module examples/log-parse/parser-infinite-loop.js --js-timeout 10ms >/dev/null
echo "OK"
