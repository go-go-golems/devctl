# devctl VHS Demo Recordings

This directory contains [VHS](https://github.com/charmbracelet/vhs) tape files for generating demo GIFs of devctl.

## Prerequisites

Install VHS:

```bash
brew install vhs
```

Or see the [VHS installation docs](https://github.com/charmbracelet/vhs#installation).

## Generating GIFs

To generate all demo GIFs:

```bash
./generate-all.sh
```

Or generate individual demos:

```bash
vhs 01-cli-workflow.tape
vhs 02-tui-demo.tape
vhs 03-plugin-development.tape
vhs 04-troubleshooting.tape
```

The output GIFs will be placed in the `vhs/` directory.

## Demo Descriptions

### 01-cli-workflow.tape

Shows the basic CLI workflow:
- `devctl plugins list` - verify plugin configuration
- `devctl plan` - see what would run
- `devctl up` - start services
- `devctl status` - check what's running
- `devctl logs --service api` - view service logs
- `devctl down` - stop everything

Duration: ~30 seconds

### 02-tui-demo.tape

Shows the interactive TUI:
- Starting the TUI with `devctl tui`
- Toggling help with `?`
- Starting services with `u`
- Navigating between services
- Opening service logs
- Toggling stdout/stderr
- Switching between views (Dashboard, Events, Pipeline, Plugins)
- Stopping services with `d`

Duration: ~40 seconds

### 03-plugin-development.tape

Shows creating a plugin from scratch:
- Writing a minimal Python plugin
- Creating `.devctl.yaml`
- Testing with `devctl plugins list`
- Seeing the plan with `devctl plan`
- Running it with `devctl up`

Duration: ~35 seconds

### 04-troubleshooting.tape

Shows common errors and fixes:
- Missing config file error
- Plugin with stdout contamination
- How to fix by using stderr
- Successful plugin loading after fix

Duration: ~30 seconds

## Using in Documentation

Include these GIFs in documentation:

```markdown
![devctl CLI workflow](vhs/01-cli-workflow.gif)
```

Or in the README:

```markdown
## Quick Demo

<p align="center">
  <img src="vhs/01-cli-workflow.gif" alt="devctl CLI workflow" width="800">
</p>
```

## Customizing

Edit the `.tape` files to:
- Change timing with `Sleep` commands
- Adjust window size with `Set Width` and `Set Height`
- Change theme with `Set Theme` (try "GitHub Dark", "Tokyo Night", "Catppuccin Mocha")
- Modify typing speed with `Type@<speed>ms`





