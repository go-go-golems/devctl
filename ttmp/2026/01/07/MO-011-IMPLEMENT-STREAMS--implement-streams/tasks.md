# Tasks

## TODO

- [ ] Add tasks here

- [x] Decide and document capability gating semantics for StartStream (ops authoritative; streams informational vs strict ops+streams)
- [x] Add TUI stream envelope types + structs (StreamStartRequest/StreamEvent/StreamEnded) and Bubble Tea msg types
- [x] Implement RegisterUIStreamRunner: load repo config, start one plugin client per stream, SupportsOp gate, StartStream with start-timeout, forward events, cleanup
- [x] Extend domain→UI transformer to map stream domain events to UI messages (and add corresponding topic constants)
- [x] Extend UI forwarder to deliver stream UI messages into Bubble Tea program
- [x] Wire UIStreamRunner into devctl tui command startup (alongside action runner/state watcher)
- [ ] Add minimal TUI surface for streams (e.g., append to Events view and/or create a Streams view)
- [ ] Implement devctl stream CLI (start op, parse input-json/file, print events, handle ctrl+c, enforce SupportsOp)
- [x] Add a telemetry stream fixture plugin under devctl/testdata/plugins (telemetry.stream) for repeatable manual and automated validation
- [x] Add negative fixture coverage: plugin advertises capabilities.streams only and never responds; verify runner/CLI fail fast (no hangs)
- [ ] Add TUI e2e-ish validation playbook: start telemetry stream, render events, stop stream, ensure plugin process cleaned up
- [ ] Decide first UI integration target: Events log vs new Streams view vs Service view logs.follow; implement the chosen rendering path
- [ ] Optional: introduce protocol-level stream stop semantics (stream.stop or op-specific stop) to enable client reuse and avoid one-client-per-stream
