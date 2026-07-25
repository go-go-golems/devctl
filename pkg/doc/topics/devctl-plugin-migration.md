---
Title: Migrate Existing Plugins to the Durable Operator Model
Slug: plugin-migration
Short: "Audit existing protocol-v2 plugins, remove duplicated supervision, and verify dynamic commands under devctl's durable operator model."
Topics:
  - devctl
  - plugins
  - migration
  - upgrade
  - supervision
  - dynamic-commands
Commands:
  - plugins
  - plan
  - doctor
  - up
  - down
  - restart
  - status
  - logs
Flags:
  - output
  - stream
  - profile
IsTemplate: false
IsTopLevel: true
ShowPerDefault: true
SectionType: Tutorial
---

Most protocol-v2 plugins do not require a source-code port for the durable
operator release. The plugin still describes repository policy and returns a
service plan; devctl still owns process supervision. Migration is therefore an
audit: verify the protocol boundary, remove plugin code that duplicates
devctl's runtime responsibilities, update external scripts, and refresh the
dynamic-command catalog.

Use this guide for every plugin selected by a repository profile. Test each
profile independently because command conflicts, configuration patches, and
launch plans depend on the complete selected plugin set.

## Determine whether the plugin needs source changes

A conforming plugin starts as a long-lived child process, writes a protocol-v2
handshake as its first stdout frame, and answers supported requests with NDJSON
response frames. It returns service specifications from `launch.plan` but does
not detach, monitor, restart, or terminate those services itself. Such a plugin
usually requires validation rather than refactoring.

Classify the plugin before editing it:

| Current behavior | Migration decision |
|---|---|
| Emits a `protocol_version: v2` handshake | Keep the protocol version |
| Returns commands, working directories, environment values, and health checks from `launch.plan` | Keep the declarative service plan |
| Writes diagnostics and progress to stderr | Keep the output separation |
| Starts background processes or writes PID files | Refactor; devctl owns process identity and termination |
| Redirects service output to plugin-managed files | Refactor; devctl captures raw streams and structured journals |
| Implements a restart loop | Refactor; use `devctl restart` and operator outcomes |
| Polls service health after returning the launch plan | Refactor; express the health check in the service specification |
| Deletes `.devctl` state to recover | Remove; use `devctl doctor` and preserve ownership evidence |
| Advertises top-level commands | Keep, then validate catalog identity and conflicts |
| Invokes removed devctl CLI forms from a script | Update the script rather than the plugin protocol |

Do not add a compatibility adapter for old process-management behavior. If a
plugin currently owns a subprocess, change the ownership boundary explicitly:
the plugin computes the service specification and devctl starts the process.

## Step 1: Stop environments with the old binary

State v2 intentionally does not infer ownership from old or unversioned state.
Before replacing the installed binary, let the previous version stop every
environment it owns.

```bash
devctl status
devctl down
```

Confirm that repository services have exited. Do not delete state while its
recorded processes remain alive. The old state is the information the old
binary needs to perform a controlled shutdown.

If the new version is already installed and rejects existing state, reinstall
the old binary temporarily, stop the environment, and then return to the new
version. Do not fabricate a v2 state document.

## Step 2: Audit the handshake and protocol streams

The handshake declares the provider identity and all supported operations.
Dynamic-command discovery also uses the command specifications in this frame,
so names, help text, and argument metadata must remain stable and accurate.

A minimal existing plugin should begin with a frame equivalent to:

```json
{
  "type": "handshake",
  "protocol_version": "v2",
  "plugin_name": "my-repository",
  "capabilities": {
    "ops": [
      "config.mutate",
      "validate.run",
      "launch.plan"
    ]
  }
}
```

Apply these protocol rules:

- write exactly one JSON object per stdout line;
- flush stdout after every frame;
- make the handshake the first stdout frame;
- write logs, progress, warnings, and debug output to stderr;
- echo the request ID in every response;
- return a structured error response for unsupported operations;
- honor context cancellation indirectly by exiting when devctl closes or
  terminates the plugin process;
- never mix a service's stdout with the plugin protocol stream.

Start with the lowest-cost probe:

```bash
devctl plugins list
```

A failure here usually identifies an executable path, handshake, stdout
contamination, or protocol-version problem before any build or service process
is started.

## Step 3: Move process ownership into `launch.plan`

The durable operator records one immutable run for every service attempt. Its
wrapper owns the child process group, captures stdout and stderr, publishes
process identities, performs health completion, and records exit status. A
plugin must not maintain a second ownership record for the same process.

Return the executable directly instead of starting it through a plugin-owned
backgrounding script:

```json
{
  "type": "response",
  "request_id": "REQUEST_ID",
  "ok": true,
  "output": {
    "services": [
      {
        "name": "api",
        "command": ["go", "run", "./cmd/api"],
        "cwd": ".",
        "env": {
          "API_ADDRESS": "127.0.0.1:8080"
        },
        "health": {
          "type": "http",
          "url": "http://127.0.0.1:8080/health"
        }
      }
    ]
  }
}
```

Remove constructs whose only purpose was self-supervision:

```text
shell `&`, `nohup`, `disown`, or daemon flags
plugin-maintained PID and process-group files
trap-based shutdown for service processes
service log redirection and manual tailing
restart-forever loops
post-launch health polling owned by the plugin
```

Keep repository-specific setup in the appropriate pipeline operation:

- `config.mutate` derives effective configuration;
- `build.run` creates build artifacts;
- `prepare.run` performs finite setup such as migrations;
- `validate.run` reports missing prerequisites;
- `launch.plan` describes the long-running services.

A build, migration, or validation command must complete. A server belongs in
`launch.plan` because it remains active after the pipeline finishes.

## Step 4: Review service specifications

The service plan becomes durable run input, so each field must be reproducible
and safe to persist. Review every returned service rather than testing only the
first service in the plan.

For each service, verify:

- the name is stable and unique across the selected plugins;
- the command is an argument vector rather than an assembled shell string when
  shell evaluation is unnecessary;
- the working directory resolves relative to the repository as intended;
- configuration contains no value that should be obtained interactively after
  startup;
- secrets are not printed to stderr, stdout, validation errors, or command
  descriptions;
- the health check tests readiness of the actual service;
- the health timeout permits a normal development build and startup;
- a clean process termination does not require plugin participation.

Run the planner without starting services:

```bash
devctl plan --output json
```

Inspect every service name, command argument, working directory, environment
field, and health definition. A valid protocol response can still contain an
incorrect plan, so successful plugin loading is not sufficient validation.

## Step 5: Update scripts that call devctl

The plugin protocol remains v2, but the operator CLI consolidates lifecycle
and log syntax. Search repository scripts, Makefiles, CI files, help pages, and
developer notes for removed forms.

| Previous invocation | Current invocation |
|---|---|
| `devctl stop-service api` | `devctl down api` |
| `devctl logs --service api` | `devctl logs api` |
| `devctl logs --stderr api` | `devctl logs api --stream stderr` |

Automation should request structured output explicitly:

```bash
devctl status --output json
devctl doctor --output json
devctl logs api --output json
devctl logs api --follow --output json
```

Followed JSON output is JSON Lines: each line is one complete record. Do not
wait for a closing array, and do not parse the human table or progress prose.
Usage failures exit 2, operational failures exit 1, interrupts exit 130, and a
dynamic provider command preserves a valid plugin-reported nonzero exit code.

## Step 6: Refresh and inspect dynamic commands

Plugin commands remain available at the devctl top level. The catalog now
applies deterministic static-command precedence, reports ambiguous names, and
validates the selected provider's live handshake before execution.

Rebuild and inspect the catalog after changing a plugin executable, handshake,
profile, command name, help string, or argument specification:

```bash
devctl plugins refresh
devctl plugins commands
devctl plugins inspect my-plugin
```

Test an unambiguous command through its concise form:

```bash
devctl my-command --example-argument
```

Test the same provider explicitly when diagnosing discovery or conflicts:

```bash
devctl plugins run my-plugin my-command -- --example-argument
```

Static devctl commands and aliases always win a name collision. If two selected
plugins advertise the same dynamic name, devctl returns
`PLUGIN_COMMAND_CONFLICT`; it does not choose a provider by load order. Rename
one command or use the provider-qualified `plugins run` form.

`PLUGIN_CATALOG_STALE` means the cached provider identity or command
specification differs from the live handshake. Refresh the catalog. Do not
work around the check by injecting a duplicate static command or suppressing
handshake validation.

## Step 7: Run the migration verification matrix

Test the complete operator path after the static protocol and plan checks pass.
Use a disposable development profile when the plugin controls databases,
queues, or external resources.

```bash
devctl doctor
devctl plan
devctl up
devctl status --output json
devctl logs --output json
devctl restart SERVICE_NAME
devctl status --output json
devctl down
devctl status --output json
```

For a multi-plugin repository, repeat the matrix for each named profile:

```bash
devctl --profile PROFILE_NAME plugins refresh
devctl --profile PROFILE_NAME plan --output json
devctl --profile PROFILE_NAME doctor --output json
devctl --profile PROFILE_NAME up
devctl --profile PROFILE_NAME down
```

Validate these outcomes:

- every planned service receives a distinct run ID;
- `up` does not return success before configured readiness succeeds;
- stdout and stderr appear under the correct service and stream;
- `restart` creates a new run rather than overwriting the old run;
- an intentionally failing service receives a failed or exited outcome;
- `down` stops only processes whose PID and start identity match;
- completed run directories remain available for diagnosis;
- dynamic commands execute through the expected provider;
- a plugin command's nonzero exit code reaches the caller.

## Step 8: Remove obsolete plugin code

Delete duplicated supervision code only after the verification matrix passes.
Keep the change reviewable by separating protocol-policy changes from removal
of process-management code.

Typical removal candidates are:

- PID-file readers and writers;
- background-process launch helpers;
- service signal and process-group handlers;
- plugin-owned stdout and stderr files;
- file-tail and log-follow routines;
- health loops that run after `launch.plan`;
- restart counters and backoff loops;
- state cleanup that edits `.devctl/`.

Do not remove repository policy merely because devctl owns supervision.
Port allocation, executable selection, environment derivation, prerequisite
validation, build steps, migrations, service dependencies, and health
definitions remain plugin responsibilities.

## Troubleshooting

| Problem | Cause | Solution |
|---|---|---|
| `plugins list` reports protocol contamination | The plugin writes a banner or log line to stdout | Move all human-readable output to stderr and keep stdout as one-frame-per-line NDJSON |
| The new binary rejects repository state | A live or unversioned pre-v2 state remains | Use the old binary to stop the environment, then initialize v2 state |
| `up` succeeds but a service immediately exits | The plan returns a launcher that backgrounds the real process | Return the long-running executable directly and remove shell backgrounding |
| A service never becomes ready | The health definition targets the wrong endpoint or has an insufficient timeout | Correct the `launch.plan` health specification and retest startup |
| Logs are empty or duplicated | The plugin redirects service streams or starts a second copy | Let the wrapper own service stdout and stderr; remove plugin log redirection |
| A dynamic command disappeared | The catalog is stale or the command conflicts | Run `plugins refresh`, inspect the provider, and check `plugins commands` |
| `PLUGIN_COMMAND_CONFLICT` appears | Multiple selected providers advertise the same name | Rename one command or invoke it with provider-qualified `plugins run` |
| `PLUGIN_CATALOG_STALE` appears | The executable or handshake changed after catalog creation | Refresh the catalog and verify stable plugin identity and command metadata |
| JSON automation receives a table | The script relies on the human renderer | Add `--output json`; consume followed output as JSON Lines |
| Shutdown risks signaling an unrelated process | A plugin still uses PID-only ownership | Remove plugin shutdown logic and let devctl validate PID plus process start identity |

## See Also

```text
devctl help v2-upgrade
devctl help plugin-authoring
devctl help profiles-guide
devctl help scripting-guide
devctl help user-guide
```
