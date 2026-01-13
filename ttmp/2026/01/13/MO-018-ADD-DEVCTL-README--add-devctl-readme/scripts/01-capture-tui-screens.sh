#!/usr/bin/env bash
set -euo pipefail

# Capture devctl TUI screens via tmux and write ANSI dumps for rendering.
#
# Usage (run from devctl repo root):
#   ./ttmp/2026/01/13/MO-018-ADD-DEVCTL-README--add-devctl-readme/scripts/01-capture-tui-screens.sh
#
# Environment overrides:
#   DEVCTL_BIN   Path to devctl binary (default: devctl)
#   DEMO_REPO    Repo with .devctl.yaml (default: /tmp/devctl-demo-repo)
#   OUT_DIR      Output directory (default: ./docs/screenshots)
#   TMUX_SESSION Session name (default: devctl-shot)
#   COLS, ROWS   tmux size (default: 120x40)

DEVCTL_BIN=${DEVCTL_BIN:-devctl}
DEMO_REPO=${DEMO_REPO:-/tmp/devctl-demo-repo}
OUT_DIR=${OUT_DIR:-"$(pwd)/docs/screenshots"}
TMUX_SESSION=${TMUX_SESSION:-devctl-shot}
COLS=${COLS:-120}
ROWS=${ROWS:-40}

if ! command -v "$DEVCTL_BIN" >/dev/null 2>&1; then
  echo "DEVCTL_BIN not found: $DEVCTL_BIN" >&2
  exit 1
fi

if ! command -v tmux >/dev/null 2>&1; then
  echo "tmux is required" >&2
  exit 1
fi

if [ ! -f "$DEMO_REPO/.devctl.yaml" ]; then
  echo "DEMO_REPO missing .devctl.yaml: $DEMO_REPO" >&2
  echo "Create it or export DEMO_REPO to a configured repo." >&2
  exit 1
fi

mkdir -p "$OUT_DIR"

"$DEVCTL_BIN" down --repo-root "$DEMO_REPO" >/dev/null 2>&1 || true

tmux new-session -d -s "$TMUX_SESSION" -x "$COLS" -y "$ROWS"
tmux send-keys -t "$TMUX_SESSION" "cd $DEMO_REPO && $DEVCTL_BIN tui --repo-root $DEMO_REPO --alt-screen=false" Enter
sleep 2

tmux send-keys -t "$TMUX_SESSION" "u"
sleep 5

tmux capture-pane -e -p -t "$TMUX_SESSION" > "$OUT_DIR/devctl-tui-dashboard.ansi"

tmux send-keys -t "$TMUX_SESSION" Tab Tab
sleep 2

tmux capture-pane -e -p -t "$TMUX_SESSION" > "$OUT_DIR/devctl-tui-pipeline.ansi"

tmux send-keys -t "$TMUX_SESSION" Tab
sleep 2

tmux capture-pane -e -p -t "$TMUX_SESSION" > "$OUT_DIR/devctl-tui-plugins.ansi"

tmux send-keys -t "$TMUX_SESSION" "q"
sleep 1

tmux kill-session -t "$TMUX_SESSION"
"$DEVCTL_BIN" down --repo-root "$DEMO_REPO" >/dev/null 2>&1 || true

echo "Captured ANSI screens in $OUT_DIR"
