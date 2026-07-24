#!/usr/bin/env bash
set -euo pipefail

# Exercise the current `devctl logs --follow` behavior across append,
# truncation, and rename-and-recreate rotation. All files and processes are
# scoped to temporary directories.

if [[ ! -f go.mod ]] || ! grep -q 'github.com/go-go-golems/devctl' go.mod; then
  printf 'run this script from the devctl repository root\n' >&2
  exit 2
fi

probe_root=$(mktemp -d)
probe_binary="$probe_root/devctl"
probe_log="$probe_root/.devctl/logs/api.stdout.log"
probe_output="$probe_root/follower.out"
probe_error="$probe_root/follower.err"
follower_pid=""

cleanup() {
  if [[ -n "$follower_pid" ]]; then
    kill "$follower_pid" 2>/dev/null || true
    wait "$follower_pid" 2>/dev/null || true
  fi
  rm -rf "$probe_root"
}
trap cleanup EXIT

go build -buildvcs=false -o "$probe_binary" ./cmd/devctl
mkdir -p "$probe_root/.devctl/logs"

printf '' > "$probe_log"
printf '%s\n' \
  '{"repo_root":"'"$probe_root"'","created_at":"2026-07-24T00:00:00Z","services":[{"name":"api","pid":0,"command":["true"],"cwd":"'"$probe_root"'","stdout_log":"'"$probe_log"'","stderr_log":"'"$probe_root"'/.devctl/logs/api.stderr.log"}]}' \
  > "$probe_root/.devctl/state.json"

"$probe_binary" logs --repo-root "$probe_root" --service api --follow --tail 0 \
  > "$probe_output" 2> "$probe_error" &
follower_pid=$!

sleep 0.3
printf 'append-visible\n' >> "$probe_log"
sleep 0.4

# Copy-truncate style lifecycle: the follower retains its previous byte offset.
printf 'after-truncate\n' > "$probe_log"
sleep 0.4

# Rename-and-create style lifecycle: the follower retains the old inode.
mv "$probe_log" "$probe_log.1"
printf 'after-rotate\n' > "$probe_log"
sleep 0.4

kill "$follower_pid" 2>/dev/null || true
wait "$follower_pid" 2>/dev/null || true
follower_pid=""

printf 'expected_appended_line=append-visible\n'
printf 'expected_after_truncate=after-truncate\n'
printf 'expected_after_rotation=after-rotate\n'
printf 'captured_stdout_begin\n'
sed -n '1,80p' "$probe_output"
printf 'captured_stdout_end\n'
printf 'captured_stderr_begin\n'
sed -n '1,80p' "$probe_error"
printf 'captured_stderr_end\n'
