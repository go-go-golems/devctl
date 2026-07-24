#!/usr/bin/env bash
set -euo pipefail

# Capture the Events view while no .devctl/state.json exists. This requires
# tmux because devctl tui is interactive. Run from a devctl repository whose
# current state can safely be observed; the script does not invoke up/down.

if [[ ! -f go.mod ]] || ! grep -q 'github.com/go-go-golems/devctl' go.mod; then
  printf 'run this script from the devctl repository root\n' >&2
  exit 2
fi

session="devctl-operator-ux-probe-$$"

cleanup() {
  if tmux has-session -t "$session" 2>/dev/null; then
    tmux send-keys -t "$session" q 2>/dev/null || true
  fi
}
trap cleanup EXIT

tmux new-session -d -s "$session" \
  "cd '$PWD' && go run ./cmd/devctl tui --alt-screen=false --refresh=200ms"

sleep 1
tmux send-keys -t "$session" Tab
sleep 1
tmux capture-pane -pt "$session" -S -120
tmux send-keys -t "$session" q
