---
Title: devctl TUI Guide
Slug: devctl-tui-guide
Short: "A practical guide to devctl's terminal UI: views, keybindings, workflows, and capture/debug tips."
Topics:
  - devctl
  - tui
  - terminal
  - debugging
  - dev-environment
IsTemplate: false
IsTopLevel: true
ShowPerDefault: true
SectionType: GeneralTopic
---

# devctl TUI Guide

The devctl TUI is an "always-on" dashboard for your dev environment. It's designed to let you start/stop, inspect logs, and understand pipeline progress without constantly switching between separate terminal commands.

## Quick workflow: start, debug, restart

Here's the most common TUI workflow—starting your environment, finding a problem, and restarting:

```
1. Start the TUI              devctl tui
2. Press `u` to start         → Dashboard shows services spinning up
3. See a service fail?        → Press `j/k` to select it, `l` to see logs
4. Fix the issue              → In your editor
5. Press `Esc` to return      → Back to Dashboard
6. Press `r` to restart       → Confirms, runs down + up
7. Press `q` to quit          → When done for the day
```

The TUI remembers state between restarts—if you quit and come back, it'll show what's currently running.

## 1. Starting the TUI

The TUI is just another devctl command. It reads the same `.devctl.yaml`, uses the same repo-root resolution rules, and will surface the same plugin/plan errors you’d see in the CLI.

```bash
devctl tui
```

Useful flags:

- `--alt-screen` (default `true`): use the terminal alternate screen buffer.
- `--refresh 1s`: state polling interval.
- `--debug-logs`: allow logs to stdout/stderr while the TUI runs (may corrupt the UI).

```bash
devctl tui --alt-screen=false --refresh 1s
```

## 2. Global navigation and help

The TUI is organized into views. You can switch between views quickly and always pull up a help overlay that matches the current build of the UI.

Global keys:

- `q` or `ctrl+c`: quit
- `?`: toggle help overlay
- `tab`: switch view (Dashboard → Events → Pipeline → Plugins → Dashboard)

## 3. Dashboard view: the “control panel”

The Dashboard view is where you’ll spend most of your time. It shows whether state exists, lists services, and offers the common actions: up, down, restart, kill, and open logs.

Dashboard keys:

- `↑/↓` or `k/j`: select a service
- `enter` or `l`: open the Service view (logs) for the selected service
- `u`: start the environment (or prompt to restart if state already exists)
- `d`: stop the environment (with confirmation)
- `r`: restart the environment (with confirmation)
- `x`: send `SIGTERM` to the selected service PID (with confirmation)

Practical workflow:

1. Press `u` to start.
2. If something fails, switch to Pipeline (`tab`) to see which phase errored.
3. Switch back to Dashboard to select a service and press `l` for logs.

## 4. Service view: logs, filtering, and follow mode

The Service view shows process information plus a scrollable log viewport. It can show stdout or stderr, and it supports simple filtering for “find the one error line”.

Service keys:

- `tab`: toggle stdout/stderr stream
- `f`: toggle follow mode (auto-refresh the viewport)
- `/`: set a filter string (press `enter` to apply)
- `ctrl+l`: clear the filter
- `d`: detach back to the Dashboard
- `esc`: also returns to the Dashboard

If you only need a quick tail outside the TUI, remember the equivalent CLI commands:

```bash
devctl logs --service <name>
devctl logs --service <name> --stderr
devctl logs --service <name> --follow
```

## 5. Events view: a live event log with filters

The Events view is the “what happened?” view. It aggregates system and per-service events and lets you filter aggressively when a noisy environment is drowning out the signal.

Core keys:

- `/`: set a text filter (press `enter` to apply)
- `ctrl+l`: clear the filter
- `c`: clear the event buffer

Advanced filtering keys:

- `l`: open the log-level filter menu
  - `d/i/w/e`: toggle debug/info/warn/error
  - `a`: enable all levels
  - `n`: disable all levels
  - `esc`/`enter`/`l`: close the menu
- `p`: pause/unpause event ingestion (useful during fast scroll); unpausing flushes a bounded queue
- `space`: toggle “system” events
- `1`–`9`: toggle service filters by index (useful when you have a stable service list)

## 6. Pipeline view: understand what devctl is doing

The Pipeline view shows phases and results from the most recent `up`/`restart`. This is where you go to understand “why did up fail?” without digging through raw logs.

Pipeline keys:

- `b`: focus the Build section
- `p`: focus the Prepare section
- `v`: focus the Validation section
- `↑/↓` or `k/j`: move selection
- `enter`: toggle details for the selected item
- `o`: toggle the live output viewport (when available)

## 7. Plugins view: inspect configured plugins

The Plugins view is a quick inventory: it lists configured plugins and whether they are active/disabled/error, and allows expanding items for details.

Plugins keys:

- `↑/↓` or `k/j`: select a plugin
- `enter` or `i`: expand/collapse the selected plugin
- `a`: expand all
- `A`: collapse all
- `esc`: back

If you’re debugging protocol-level issues (handshake failures, invalid JSON), the CLI `plugins list` output is still the best raw signal:

```bash
devctl plugins list
```

## 8. Capturing output and debugging UI issues

The TUI is interactive, which makes “capture a bug report” slightly harder than with a plain CLI command. The trick is to run with settings that make capture deterministic.

### 8.1. Disable alt-screen for capture

Running with `--alt-screen=false` keeps the UI in the normal terminal buffer so your terminal (or `tmux`) can capture it.

```bash
devctl tui --alt-screen=false
```

### 8.2. Avoid debug logs unless you need them

`--debug-logs` allows logging to stdout/stderr while the UI is running. This is useful when debugging the UI itself, but it can corrupt rendering and make screenshots/captures misleading.

```bash
devctl tui --debug-logs
```

## 9. Where to go next

If you’re new to devctl overall, start with the user guide:

```text
glaze help devctl-user-guide
```

If you want to extend devctl with real repo logic, move on to scripting and plugin authoring:

```text
glaze help devctl-scripting-guide
glaze help devctl-plugin-authoring
```
