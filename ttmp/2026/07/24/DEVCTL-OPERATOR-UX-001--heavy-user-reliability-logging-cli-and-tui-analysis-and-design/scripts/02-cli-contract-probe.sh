#!/usr/bin/env bash
set -euo pipefail

# Capture representative non-interactive CLI behavior and exit statuses.
# The probe uses a temporary repository and does not start services.

if [[ ! -f go.mod ]] || ! grep -q 'github.com/go-go-golems/devctl' go.mod; then
  printf 'run this script from the devctl repository root\n' >&2
  exit 2
fi

probe_root=$(mktemp -d)
probe_cache=$(mktemp -d)
probe_binary="$probe_root/devctl"

cleanup() {
  rm -rf "$probe_root" "$probe_cache"
}
trap cleanup EXIT

GOCACHE="$probe_cache" go build -buildvcs=false -o "$probe_binary" ./cmd/devctl

run_case() {
  local name=$1
  shift

  printf '\ncase=%s\n' "$name"
  printf 'command='
  printf '%q ' "$@"
  printf '\n'

  set +e
  "$@" 2>&1
  local status=$?
  set -e
  printf 'exit_status=%d\n' "$status"
}

run_case root_help "$probe_binary" --help
run_case status_without_state "$probe_binary" status --repo-root "$probe_root"
run_case logs_without_service "$probe_binary" logs --repo-root "$probe_root"
run_case down_without_state "$probe_binary" down --repo-root "$probe_root"
run_case plan_without_config "$probe_binary" plan --repo-root "$probe_root"
run_case profiles_without_config "$probe_binary" profiles list --repo-root "$probe_root"
