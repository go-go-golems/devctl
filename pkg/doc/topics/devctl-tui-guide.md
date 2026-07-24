---
Title: devctl TUI Guide
Slug: tui-guide
Short: "Operate services, inspect durable logs, and review lifecycle outcomes from devctl's terminal UI."
Topics:
  - devctl
  - tui
  - terminal
  - debugging
  - dev-environment
Commands:
  - tui
  - status
  - logs
  - doctor
Flags:
  - profile
  - refresh
  - alt-screen
IsTemplate: false
IsTopLevel: true
ShowPerDefault: true
SectionType: GeneralTopic
---

The devctl TUI is a typed operator interface over the same controller used by
the lifecycle CLI. It reads durable run state and structured log journals
instead of scraping command output, so restarting the TUI does not erase the
environment's identity or current service status.

## Start the TUI

`devctl tui` resolves the repository, profile, configuration, and timeout in
the same way as `devctl up`. The TUI does not start services merely because it
opens; its first action is a read-only snapshot.

```bash
devctl tui
devctl tui --profile backend --refresh 2s
devctl tui --alt-screen=false   # Keep output in the normal terminal buffer.
```

Use `--alt-screen=false` when recording a deterministic bug report in `tmux`.
The default alternate screen is more comfortable for normal interactive use.

## Understand the three views

The interface has exactly three primary views. This small navigation model
keeps service state, logs, and operation outcomes directly reachable without
nested screen-specific routing.

| Key | View | Data source |
|---|---|---|
| `1` | Overview | Typed operator snapshot and health checks |
| `2` | Logs | Durable structured run journals |
| `3` | Runs | Durable service attempts and session operations |

Press `Tab` to cycle, `Esc` to return to Overview, `?` for the key summary, and
`q` or `Ctrl-C` to quit. View changes preserve log filters, pause state, wrap
mode, and the selected service.

## Operate services from Overview

Overview shows desired state, observed run phase, health, child PID, run ID,
exit information, and the last structured error. Lifecycle keys always require
a confirmation that names the selected service. When no durable state exists,
`up` clearly targets all configured services.

- `j`/`k` or arrow keys select a service.
- `Enter` opens Logs filtered to the selected service.
- `u` runs `up` for the selected scope.
- `d` runs `down` for the selected scope.
- `r` runs `restart` for the selected scope.
- `y` or `Enter` confirms; `n` or `Esc` cancels.

The actions call the controller directly. They do not invoke a subprocess,
parse a rendered table, or infer success from log text. Partial service
failures therefore remain typed and appear in Runs.

## Inspect and follow logs

Logs combines stdout, stderr, lifecycle, and plugin records from the durable
run journal. Follow mode uses one in-flight reader at a time and resumes from a
per-run cursor, preventing duplicate concurrent followers during refreshes.

- `p` pauses or resumes visible ingestion.
- `f` toggles follow mode.
- `w` toggles line wrapping.
- `/` edits the text filter; `Enter` applies it and `Esc` leaves search mode.

The in-memory display buffer has record and byte limits. Pausing therefore
cannot grow memory without bound; the header reports how many old records were
dropped. Terminal escape sequences and control characters are removed before
display so untrusted service output cannot move the cursor or rewrite apparent
UI content.

Equivalent non-interactive commands are:

```bash
devctl logs api
devctl logs api --stream stderr
devctl logs api --follow
devctl logs api --output json   # A finite query as structured output.
```

## Review run and operation outcomes

Runs begins with the current durable service attempts from the snapshot. It
then lists lifecycle operations initiated during this TUI session, including
phase events, per-service outcomes, stable error codes, and partial failures.

Use `j` and `k` to select an operation. Press `l` to move to the related log
view. A health refresh at the same state revision updates health without
inventing a new lifecycle operation.

## Use the command palette

Press `:` to open the command palette. Select with `j`/`k`, execute with
`Enter`, and close it with `Esc`.

The palette currently provides:

- refresh the environment snapshot;
- run typed operator diagnostics through `Controller.Doctor`; and
- show the exact `devctl plugins inspect` command for plugin-provider details.

Dynamic plugin commands remain top-level CLI commands, but the TUI does not
silently execute one from the palette. Inspect providers first when a dynamic
command is ambiguous.

## Capture and diagnose a TUI problem

A useful report includes a normal-screen capture plus structured CLI output
from the same repository:

```bash
tmux new-session -s devctl-debug
devctl tui --alt-screen=false

# In another shell:
tmux capture-pane -pt devctl-debug -S -200
devctl status --output json
devctl doctor --output json
devctl plugins inspect --output json
```

Do not enable unrelated application debug logs on the TUI's terminal. Service
output belongs in the run journal and is available through Logs.

## Troubleshooting

| Problem | Cause | Solution |
|---|---|---|
| Overview says `stopped` | No durable environment state exists | Press `u`, confirm all configured services, or run `devctl up` |
| A service is `failed` | The wrapper recorded a launch, exit, or health error | Select it, press `Enter`, then inspect Runs for the stable error code |
| Follow appears idle | No selected run has produced a new journal record | Check the selected service and toggle `f`; use `devctl status --output json` to verify its run ID |
| Old lines disappear while paused | The bounded display buffer reached its record or byte limit | Resume with `p`, narrow the service/filter scope, or query the durable journal with `devctl logs` |
| A dynamic command is missing or conflicted | Catalog discovery changed or a static command owns its name | Run `devctl plugins inspect`; execute an unambiguous provider-qualified command with `devctl plugins run` |
| The screen is hard to capture | The alternate screen does not remain in scrollback | Restart with `--alt-screen=false` inside `tmux` |

## See Also

```text
devctl help user-guide
devctl help profiles-guide
devctl help scripting-guide
devctl help plugin-authoring
```
