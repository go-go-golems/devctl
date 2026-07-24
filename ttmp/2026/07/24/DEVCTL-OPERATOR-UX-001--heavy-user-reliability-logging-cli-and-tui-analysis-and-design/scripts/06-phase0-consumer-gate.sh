#!/usr/bin/env bash
set -uo pipefail

# Search sibling go-go-golems repositories for consumers of surfaces proposed
# for removal. This script is read-only outside the ticket output directory.

if [[ ! -f go.mod ]] || ! rg -q 'github.com/go-go-golems/devctl' go.mod; then
  printf 'run this script from the devctl repository root\n' >&2
  exit 2
fi

search_root="/home/manuel/code/wesen/go-go-golems"
ticket_dir="ttmp/2026/07/24/DEVCTL-OPERATOR-UX-001--heavy-user-reliability-logging-cli-and-tui-analysis-and-design"
output_dir="$ticket_dir/sources/local/phase0"
mkdir -p "$output_dir"

if [[ ! -d "$search_root" ]]; then
  printf 'search root does not exist: %s\n' "$search_root" >&2
  exit 2
fi

common_globs=(
  --glob '!**/.git/**'
  --glob '!**/node_modules/**'
  --glob '!**/vendor/**'
  --glob '!**/ttmp/**'
  --glob '!**/go-go-golems/devctl/**'
)

rg --files "$search_root" --glob 'go.work' \
  >"$output_dir/go-work-files.txt" 2>&1 || true

rg -n \
  "${common_globs[@]}" \
  'github\.com/go-go-golems/devctl/pkg/logjs' \
  "$search_root" \
  >"$output_dir/external-logjs-imports.txt" 2>&1 || true

rg -n \
  "${common_globs[@]}" \
  -e '(^|[^[:alnum:]_-])log-parse([^[:alnum:]_-]|$)' \
  -e 'devctl[[:space:]]+start([^[:alnum:]_-]|$)' \
  -e 'devctl[[:space:]]+stop-service([^[:alnum:]_-]|$)' \
  -e 'devctl[[:space:]]+logs([^\\n]*[[:space:]])--service([^[:alnum:]_-]|$)' \
  "$search_root" \
  >"$output_dir/external-legacy-cli-uses.txt" 2>&1 || true

{
  printf 'external_logjs_import_lines\t'
  wc -l <"$output_dir/external-logjs-imports.txt"
  printf 'external_legacy_cli_lines\t'
  wc -l <"$output_dir/external-legacy-cli-uses.txt"
  printf 'go_work_files\t'
  wc -l <"$output_dir/go-work-files.txt"
} >"$output_dir/consumer-gate-summary.tsv"
