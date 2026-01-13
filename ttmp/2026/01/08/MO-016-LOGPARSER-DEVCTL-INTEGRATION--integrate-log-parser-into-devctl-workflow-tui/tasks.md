# Tasks

## TODO

- [ ] Define CLI wrappers: `devctl log-parse` + `devctl logs --parse` (service-aware log path resolution).
- [ ] Decide integration path: domain event `log.parsed` vs. stream event `protocol.Event{event:"log.parsed"}`.
- [ ] Implement parsed log event routing in `pkg/tui/transform.go` and UI forwarder.
- [ ] Build Parsed Logs TUI model (filters for service/tag/module/level, raw toggle, stats).
- [ ] Add TUI actions to start/stop parsing per service (dashboard/service view entry points).
- [ ] Add parser error panel + error ribbon in Parsed Logs view.
- [ ] Add module configuration persistence + defaults (per repo/service).
- [ ] Add sampling/rate limiting behavior for high-volume streams.
- [ ] Document user workflow in help docs and reference guides.

## Done

- [x] Draft integration study + UX spec for parsed logs.
