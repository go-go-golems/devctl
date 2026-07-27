# External Source Register

All captures were retrieved on 2026-07-24 from official project
documentation. They are evidence for interaction patterns, not vendored
specifications or implementation dependencies.

| Capture | Upstream URL | Question answered |
|---|---|---|
| `web/01-process-compose-tui.md` | <https://f1bonacc1.github.io/process-compose/tui/> | Which status, log, action, buffer, and navigation controls does a process-oriented TUI expose? |
| `web/02-process-compose-client.md` | <https://f1bonacc1.github.io/client/> | How can CLI and TUI clients share one process-control service? |
| `web/03-process-compose-lifecycle.md` | <https://f1bonacc1.github.io/process-compose/launcher/> | How are dependencies, restart policy, and successful exits represented? |
| `web/04-docker-compose-logs.md` | <https://docs.docker.com/reference/cli/docker/compose/logs/> | What is the conventional multi-service logs selection and filtering contract? |
| `web/05-docker-compose-ps.md` | <https://docs.docker.com/reference/cli/docker/compose/ps/> | How does a mature CLI separate human tables, JSON, filters, and stopped resources? |
| `web/06-tilt-logs.md` | <https://docs.tilt.dev/cli/tilt_logs.html> | How does a development orchestrator expose resource, source, level, time, tail, and JSON filters? |

The official journalctl page was read through web search but was not captured:
Defuddle received HTTP 418 from both the `latest` and versioned freedesktop
URLs. The design links the authoritative page directly and uses only its
documented filtering, follow, cursor, and structured-output concepts.
