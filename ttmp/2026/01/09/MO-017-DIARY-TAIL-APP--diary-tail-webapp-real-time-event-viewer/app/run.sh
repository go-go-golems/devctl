#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")"

# Default to watching the ttmp root (4 levels up from app/)
DEFAULT_DIR="$(cd ../../../.. && pwd)"
DIR="${1:-$DEFAULT_DIR}"
PORT="${2:-8765}"

echo "📓 Diary Tail App"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Watching: $DIR"
echo "Server:   http://localhost:$PORT"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

go run main.go -dir "$DIR" -port "$PORT"

