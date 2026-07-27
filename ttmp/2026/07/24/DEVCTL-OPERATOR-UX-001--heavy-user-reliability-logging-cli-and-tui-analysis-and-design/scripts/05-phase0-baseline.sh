#!/usr/bin/env bash
set -uo pipefail

# Archive the Phase 0 build, test, lint, and operator-probe baseline.
# Run from the devctl repository root.

if [[ ! -f go.mod ]] || ! rg -q 'github.com/go-go-golems/devctl' go.mod; then
  printf 'run this script from the devctl repository root\n' >&2
  exit 2
fi

ticket_dir="ttmp/2026/07/24/DEVCTL-OPERATOR-UX-001--heavy-user-reliability-logging-cli-and-tui-analysis-and-design"
output_dir="$ticket_dir/sources/local/phase0"
summary="$output_dir/summary.tsv"
mkdir -p "$output_dir"

printf 'name\tstatus\toutput\n' >"$summary"
failed=0

run_capture() {
  local name=$1
  shift
  local output="$output_dir/$name.txt"
  local status

  printf 'running %s\n' "$name"
  "$@" >"$output" 2>&1
  status=$?
  # CLI help and terminal captures intentionally pad lines for display. Strip
  # that presentation whitespace so archived evidence passes Git hygiene.
  sed -i 's/[[:space:]]*$//' "$output"
  sed -i '${/^$/d;}' "$output"
  printf '%s\t%d\t%s\n' "$name" "$status" "$output" >>"$summary"
  if [[ $status -ne 0 ]]; then
    failed=1
    printf '%s failed with status %d; see %s\n' "$name" "$status" "$output" >&2
  fi
}

run_capture go-test go test ./...
run_capture go-build go build ./...
run_capture make-lint make lint
run_capture probe-01 bash "$ticket_dir/scripts/01-architecture-inventory.sh"
run_capture probe-02 bash "$ticket_dir/scripts/02-cli-contract-probe.sh"
run_capture probe-03 bash "$ticket_dir/scripts/03-log-follow-lifecycle-probe.sh"
run_capture probe-04 bash "$ticket_dir/scripts/04-tui-state-event-probe.sh"

exit "$failed"
