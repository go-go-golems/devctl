# Plugin authoring (NDJSON stdio protocol v2)

For the full playbook (protocol, patterns, examples, diagrams), see:

- `pkg/doc/topics/devctl-plugin-authoring.md`

## Rules (non-negotiable)

- **stdout is protocol only**: every line must be a single JSON object (NDJSON).
- **stderr is for humans**: print logs and progress to stderr (not stdout).
- The first stdout frame must be a **handshake**.

If you print anything non-JSON to stdout (even a single character), `devctl` treats it as a protocol violation and the plugin is considered failed.

## Handshake

First frame (stdout):

```json
{"type":"handshake","protocol_version":"v2","plugin_name":"example","capabilities":{"ops":["config.mutate","launch.plan","command.run"],"commands":[{"name":"db-reset","help":"Reset local DB"}]}}
```

- `capabilities.ops` declares which `op` values the plugin can handle.

## Requests and responses

`devctl` sends request frames on stdin:

```json
{"type":"request","request_id":"p1-1","op":"config.mutate","ctx":{"repo_root":"/abs/repo","deadline_ms":30000,"dry_run":false},"input":{"config":{}}}
```

Plugins reply on stdout:

```json
{"type":"response","request_id":"p1-1","ok":true,"output":{}}
```

Errors:

```json
{"type":"response","request_id":"p1-1","ok":false,"error":{"code":"E_UNSUPPORTED","message":"unsupported op"}}
```

## Streaming (events)

Some ops return a `stream_id` in the response output and then emit `event` frames:

```json
{"type":"event","stream_id":"s1","event":"log","level":"info","message":"hello"}
{"type":"event","stream_id":"s1","event":"end","ok":true}
```

## Common ops

### `config.mutate`

Return a config patch:

```json
{"type":"response","request_id":"...","ok":true,"output":{"config_patch":{"set":{"services.backend.port":8083},"unset":[]}}}
```

### `launch.plan`

Return services to run:

```json
{"services":[{"name":"backend","cwd":"backend","command":["go","run","./cmd/server"],"health":{"type":"http","url":"http://127.0.0.1:8083/health"}}]}
```

### `command.run`

Declare commands in the handshake:

```json
{"type":"handshake","protocol_version":"v2","plugin_name":"example","capabilities":{"ops":["command.run"],"commands":[{"name":"db-reset","help":"Reset local DB"}]}}
```

Run command:

```json
{"type":"request","op":"command.run","input":{"name":"db-reset","argv":["--force"],"config":{...}}}
```

## Service schema

A `launch.plan` service has these fields:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | `string` | yes | Stable identifier for logs/status/down. |
| `command` | `string[]` | yes | argv array. Use `["bash","-lc","..."]` for shell features. |
| `cwd` | `string` | optional | Working directory (relative to `repo_root`). |
| `env` | `map<string,string>` | optional | Extra env vars merged with parent env. |
| `health.type` | `"tcp" \| "http"` | optional | Readiness probe type. |
| `health.address` | `string` | for `tcp` | Host:port to dial (e.g. `"127.0.0.1:8080"`). |
| `health.url` | `string` | for `http` | URL to GET. 2xx–4xx counts as healthy. |
| `health.timeout_ms` | `number` | optional | Ready timeout (default: 30s). |

## Examples

- `examples/plugins/python-minimal/plugin.py`
- `examples/plugins/bash-minimal/plugin.sh`
- `testdata/plugins/e2e/plugin.py` — full pipeline with multiple services
- `testdata/plugins/http-service/plugin.py` — HTTP health check example
