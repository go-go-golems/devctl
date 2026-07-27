---
Title: Upgrade to the Durable Operator Model
Slug: v2-upgrade
Short: "Prepare repositories for devctl's versioned state, structured run journals, and consolidated lifecycle CLI."
Topics:
  - devctl
  - upgrade
  - state
  - logs
Commands:
  - up
  - down
  - restart
  - logs
  - plugins
Flags:
  - output
  - stream
IsTemplate: false
IsTopLevel: true
ShowPerDefault: true
SectionType: Tutorial
---

The durable operator model changes process ownership, state validation, log
storage, and lifecycle command contracts as one coordinated release. Complete
the shutdown step with the old binary before installing the new binary because
the new implementation intentionally refuses old or unversioned live state.

## Before upgrading

The old devctl process must stop every environment it owns. This ensures the
old binary performs shutdown using the state schema and PID assumptions it
understands.

```bash
# Run this with the currently installed, pre-upgrade devctl.
devctl status
devctl down
```

Confirm that no repository service is still running. Do not delete state and
leave processes alive: that discards the only ownership record the old binary
can use for a controlled shutdown.

## Install and initialize the new version

After the old environment is stopped, install the new binary and start a fresh
environment. The first `up` creates schema-v2 environment state and one
immutable run directory for each service attempt.

```bash
devctl doctor
devctl plan
devctl up
devctl status
```

State v2 has no automatic v1 migration or compatibility adapter. Refusal is a
safety property: devctl will not guess whether an arbitrary PID still
represents a process it owns.

## Update lifecycle and log commands

Lifecycle selection is now positional and consistent across commands. Removed
commands do not have hidden aliases.

| Removed form | Replacement |
|---|---|
| `devctl stop-service api` | `devctl down api` |
| `devctl logs --service api` | `devctl logs api` |
| `devctl logs --stderr api` | `devctl logs api --stream stderr` |

`devctl up api` is the operation for starting a stopped or failed
tracked service. `devctl restart api` performs a typed down/up operation and
reports partial failures per service.

Scripts should select an explicit output format:

```bash
devctl status --output json
devctl doctor --output json
devctl logs api --output json
devctl logs api --follow --output json  # Compact JSON Lines until interrupted.
```

Usage failures exit 2, operational failures exit 1, interrupts exit 130, and a
provider command preserves its plugin-reported non-zero exit code.

## Understand storage and retention

The environment index is small and mutable. Each service attempt has a stable
run identifier and an immutable directory containing its ownership handshake,
state record, raw streams, structured journal, and terminal exit record.

```text
.devctl/
├── state.json
└── runs/
    └── <run-id>/
        ├── run.json
        ├── owner.json
        ├── ready.json
        ├── stdout.log
        ├── stderr.log
        ├── logs.jsonl
        └── exit.json
```

`devctl down` updates or removes active ownership from `state.json`; it does not
delete completed run directories. This release deliberately performs no
automatic retention deletion. Archive or remove old `.devctl/runs/<run-id>/`
directories only after confirming they are not current in `devctl status
--output json`.

Do not delete `.devctl/state.json` to recover from an ownership error. Run
`devctl doctor` and preserve both state and run artifacts for diagnosis.

## Verify dynamic commands after the upgrade

Repository-specific commands still appear automatically at the `devctl`
top level. The new catalog makes that feature deterministic: static commands
win collisions, ambiguous dynamic names are recorded, and execution verifies
the provider identity.

```bash
devctl plugins refresh
devctl plugins commands
devctl plugins inspect my-plugin
devctl plugins run my-plugin my-command -- --example-argument
```

Root help and completion do not start every plugin. If catalog configuration
has drifted, refresh it explicitly rather than relying on stale command
metadata.

## Troubleshooting

| Problem | Cause | Solution |
|---|---|---|
| State is rejected after installing the new binary | The repository still has v1 or unversioned state | Reinstall the old binary, stop its environment, then return to the new binary; do not fabricate v2 ownership |
| A former command is unknown | The command was consolidated rather than aliased | Use the replacement table above and update scripts |
| Completed logs still consume disk | Run retention is intentionally manual | Verify current run IDs, then archive or remove only old run directories |
| A dynamic command is absent | The catalog is stale, conflicted, or its provider identity changed | Run `plugins refresh`, `plugins inspect`, and use provider-qualified `plugins run` |
| JSON automation receives a table | The default renderer is for humans | Pass `--output json`; followed output is compact JSON Lines |

## See Also

```text
devctl help user-guide
devctl help tui-guide
devctl help scripting-guide
devctl help plugin-authoring
```
