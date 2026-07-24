#!/usr/bin/env bash
set -euo pipefail

repo_root="${1:-$(pwd)}"
fixture_root="${2:-/tmp/devctl-operator-matrix}"
http_port="${3:-18080}"
session_name="${4:-devctl-operator-matrix}"

mkdir -p "${fixture_root}/bin"
lsof-who -p "${http_port}" -k || true

(
  cd "${repo_root}"
  go build -o "${fixture_root}/bin/http-echo" ./testapps/cmd/http-echo
  go build -o "${fixture_root}/bin/log-spewer" ./testapps/cmd/log-spewer
)

config_path="${fixture_root}/.devctl.yaml"
plugin_path="${repo_root}/testdata/plugins/e2e/plugin.py"
{
  printf 'plugins:\n'
  printf '  - id: e2e\n'
  printf '    path: python3\n'
  printf '    args:\n'
  printf '      - %s\n' "${plugin_path}"
  printf '    env:\n'
  printf '      DEVCTL_HTTP_ECHO_BIN: %s/bin/http-echo\n' "${fixture_root}"
  printf '      DEVCTL_LOG_SPEWER_BIN: %s/bin/log-spewer\n' "${fixture_root}"
  printf '      DEVCTL_HTTP_ECHO_PORT: "%s"\n' "${http_port}"
  printf '    priority: 10\n'
} >"${config_path}"

tmux new-session -d -s "${session_name}" -x 120 -y 30 \
  "cd '${repo_root}' && go run ./cmd/devctl tui --repo-root '${fixture_root}' --alt-screen=false --refresh 200ms"

printf 'Fixture: %s\n' "${fixture_root}"
printf 'Session: %s\n' "${session_name}"
printf 'Capture: tmux capture-pane -pt %s -S -100\n' "${session_name}"
printf 'Interact: tmux attach-session -t %s\n' "${session_name}"
printf 'Cleanup through the TUI with down confirmations, then q.\n'
