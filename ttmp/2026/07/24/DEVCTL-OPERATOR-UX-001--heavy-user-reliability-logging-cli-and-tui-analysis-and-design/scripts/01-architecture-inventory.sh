#!/usr/bin/env bash
set -euo pipefail

# Reproduce the static inventory used by the operator-experience design.
# Run from the devctl repository root.

if [[ ! -f go.mod ]] || ! grep -q 'github.com/go-go-golems/devctl' go.mod; then
  printf 'run this script from the devctl repository root\n' >&2
  exit 2
fi

printf 'revision\n'
git rev-parse HEAD

printf '\npackage_line_counts\n'
wc -l \
  cmd/devctl/cmds/*.go \
  pkg/supervise/*.go \
  pkg/state/*.go \
  pkg/logjs/*.go \
  pkg/tui/*.go \
  pkg/tui/models/*.go \
  pkg/tui/widgets/*.go |
  sort -n

printf '\npublic_and_internal_symbols\n'
rg -n '^type |^func ' \
  pkg/supervise \
  pkg/state \
  pkg/logjs \
  pkg/tui \
  cmd/devctl/cmds

printf '\ntest_inventory\n'
rg -n '^func Test' \
  pkg/supervise \
  pkg/state \
  pkg/logjs \
  pkg/tui \
  cmd/devctl/cmds || true

printf '\noperator_commands\n'
rg -n -e 'Use:' -e 'NewCommandDescription' cmd/devctl/cmds --glob '*.go'

printf '\nactive_or_inconsistent_tickets\n'
find ttmp -type f -name index.md -print0 |
  xargs -0 rg -l 'Status: active|Current status: \\*\\*active\\*\\*' |
  sort
