# Tasks

## TODO

- [ ] Protocol v2 cleanup pass: remove commands.list, add handshake command specs, enforce capabilities, and remove invocation-helper concept

- [x] Scrub MO-010 docs to remove invocation-helper layer references
- [x] Protocol: add v2 handshake + structured capabilities.commands (CommandSpec/CommandArg) + validation
- [x] Runtime: enforce capabilities in Client.Call/StartStream (fast E_UNSUPPORTED) and expose typed op errors
- [x] Runtime: remove context.Value request metadata (delete runtime/context.go) and plumb explicit request meta into requests
- [x] Repository container: centralize repo root/config/plugin discovery + request meta; integrate into CLI and TUI bootstraps
- [x] Dynamic commands: wire Cobra commands from handshake command specs; remove commands.list and provider re-discovery
- [x] CLI: update devctl plugins list output for structured handshake commands
- [x] Docs: update plugin authoring guides to protocol v2 (handshake command specs; remove commands.list)
- [x] Plugins: update in-repo examples + testdata plugins to protocol v2 and handshake-advertised commands
- [x] Tests: update for v2 + add coverage for unsupported fast-fail and dynamic discovery (no commands.list)
- [ ] Validation: gofmt/go test; update diary + changelog; commit code and docs
