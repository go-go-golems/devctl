# Tasks

## TODO

- [ ] Add tasks here

- [x] Audit TUI long-lived operations and document all context.Background/msg.Context usages.
- [x] Refactor stream runner to use a TUI-scoped context for stream lifetimes and plugin start; keep cleanup on fresh timeout contexts.
- [x] Refactor action runner to use a TUI-scoped context for runUp/runDown and action phases (no msg.Context for lifetimes).
- [x] Decide on Bubbletea WithContext and message publish context propagation; implement if agreed.
- [ ] Validate shutdown: streams/actions stop on TUI exit; no blocked publishes or orphaned processes.
