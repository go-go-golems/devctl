#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../../../../../.." && pwd)"
cd "$repo_root/devctl"

echo "== validate modules-dir =="
go run ./cmd/log-parse validate --modules-dir examples/log-parse/modules >/dev/null
echo "OK"

