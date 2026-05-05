---
Title: Profiles and Local Overrides
Slug: profiles-guide
Short: Use profiles and .devctl.override.yaml to select plugins and keep local adjustments out of shared config.
Topics:
  - devctl
  - profiles
  - configuration
  - overrides
  - plugins
Commands:
  - profiles
  - up
  - plan
  - plugins
Flags:
  - profile
IsTemplate: false
IsTopLevel: true
ShowPerDefault: true
SectionType: GeneralTopic
---

Profiles let a repository define named ways to run devctl. A profile selects which plugins participate in the pipeline and can add environment overrides for those plugins. This is useful when a repository has several valid development modes: frontend-only, backend-only, full-stack, production-like, or a local debugging setup.

A profile is not a separate pipeline. It is a selection step before plugins are started. Once devctl has selected the active profile, the rest of the system behaves normally: selected plugins run `config.mutate`, `build.run`, `prepare.run`, `validate.run`, and `launch.plan`, and devctl supervises the services returned by the launch plan.

## 1. Defining profiles in `.devctl.yaml`

Profiles live in the shared project config under `profiles:`. Each profile names plugin IDs from the top-level `plugins:` list.

```yaml
profile:
  active: development

profiles:
  development:
    display_name: Development
    description: Hot reload, verbose logs, local services
    plugins:
      - api
      - web
    env:
      LOG_LEVEL: debug

  backend:
    display_name: Backend Only
    description: API plus database, no frontend
    plugins:
      - api
      - database

plugins:
  - id: api
    path: python3
    args: [./plugins/api.py]
  - id: web
    path: python3
    args: [./plugins/web.py]
  - id: database
    path: python3
    args: [./plugins/database.py]
```

The `plugins:` field inside a profile is a list of plugin IDs. It does not control ordering. Ordering still comes from plugin `priority`, with ID as the stable tie-breaker.

## 2. Selecting a profile

Use `--profile` when you want an explicit one-off choice:

```bash
devctl up --profile backend
devctl plan --profile development
devctl plugins list --profile backend
```

Use `profile.active` when the repository should have a default profile:

```yaml
profile:
  active: development
```

Selection precedence is:

```text
1. --profile <name>
2. profile.active from .devctl.override.yaml
3. profile.active from .devctl.yaml
4. no profile: load all top-level plugins
```

The final case is the backward-compatible mode. Existing repositories with only a top-level `plugins:` list continue to load all plugins.

## 3. The explicit `default` profile

A profile named `default` is allowed, but it is not magic. It is active only when selected explicitly:

```yaml
profile:
  active: default

profiles:
  default:
    plugins: [api, web]
```

or:

```bash
devctl up --profile default
```

If `.devctl.yaml` defines `profiles.default` but neither `profile.active: default` nor `--profile default` is provided, devctl loads all top-level plugins. The mere presence of a profile named `default` does not change old behavior.

## 4. Local overrides with `.devctl.override.yaml`

A local `.devctl.override.yaml` can add or adjust profiles without changing the shared project config. devctl loads `.devctl.yaml` first, then merges `.devctl.override.yaml` if it exists.

```yaml
# .devctl.override.yaml
profile:
  active: local-debug

profiles:
  local-debug:
    display_name: Local Debug
    description: Backend plus local trace settings
    plugins:
      - api
      - database
    env:
      LOG_LEVEL: trace
      DEVCTL_TRACE: "1"
```

The expected workflow is that `.devctl.override.yaml` is local to one developer. Add it to `.gitignore` unless your team intentionally wants to commit a shared override template.

## 5. Merge rules

The override file is deliberately simple. It is the same schema as `.devctl.yaml`, but every field is optional.

| Field shape | Merge rule |
|---|---|
| Scalars such as `profile.active` and `strictness` | Override value wins when non-empty. |
| Maps such as `profiles` and `env` | Keys merge; override keys win. |
| Profile records | Merge field by field. |
| Profile `plugins` list | Replaced when the override provides a non-empty list. |
| Top-level `plugins` list | Merged by plugin `id`; existing IDs are patched, new IDs are appended. |
| Plugin `args` list | Replaced when the override provides a non-empty list. |
| Plugin `env` map | Merged; override keys win. |

This means a local override can adjust an existing profile's env without restating its plugin list:

```yaml
profiles:
  development:
    env:
      LOG_LEVEL: trace
```

It can also add a local plugin and a profile that selects it:

```yaml
profiles:
  tracing:
    plugins: [api, jaeger-local]

plugins:
  - id: jaeger-local
    path: ./plugins/devctl-jaeger-local
```

## 6. Profile env and plugin env

Profile env is applied to every selected plugin and overlays that plugin's configured env for that run.

```yaml
profiles:
  backend:
    plugins: [api]
    env:
      LOG_LEVEL: debug

plugins:
  - id: api
    path: python3
    args: [./plugins/api.py]
    env:
      LOG_LEVEL: info
      API_ONLY: "1"
```

When `backend` is active, the `api` plugin receives `LOG_LEVEL=debug` and `API_ONLY=1`.

## 7. Inspecting profiles

List the profiles devctl sees after applying the local override:

```bash
devctl profiles list
```

Show the resolved active profile:

```bash
devctl profiles active
```

If no profile is selected, `profiles active` prints `(none)`. That means devctl will load all top-level plugins.

## 8. Dynamic commands and profiles

Dynamic commands are discovered from the plugins selected by the active profile. If a plugin is excluded by `--profile backend`, dynamic commands from that plugin are not registered for that invocation.

This is intentional. A profile describes the active operating mode, and dynamic commands should come from the plugins participating in that mode.

## Troubleshooting

| Problem | Cause | Solution |
|---|---|---|
| `profile "x" not found` | The selected profile is not present after merging `.devctl.yaml` and `.devctl.override.yaml`. | Run `devctl profiles list` and check spelling. |
| `profile "x" references unknown plugin "y"` | The profile names a plugin ID that is not defined or discovered. | Add the plugin to top-level `plugins:` or fix the ID. |
| `profiles active` prints `(none)` | No `--profile` and no `profile.active` are set. | This is valid backward-compatible all-plugins mode. Set `profile.active` if you want a default. |
| `profiles.default` is ignored | `default` is not implicit. | Use `profile.active: default` or `--profile default`. |
| Local changes affect teammates | `.devctl.override.yaml` was committed accidentally. | Add `.devctl.override.yaml` to `.gitignore` unless the team wants it committed. |

## See Also

- `devctl help user-guide`
- `devctl help scripting-guide`
- `devctl help plugin-authoring`
- `devctl profiles list`
- `devctl profiles active`
