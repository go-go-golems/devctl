# Tasks

## TODO

- [ ] Add tasks here

- [ ] Audit TUI long-lived operations and document all context.Background/msg.Context usages.
- [ ] Refactor stream runner to use a TUI-scoped context for stream lifetimes and plugin start; keep cleanup on fresh timeout contexts.
- [ ] Refactor action runner to use a TUI-scoped context for runUp/runDown and action phases (no msg.Context for lifetimes).
- [ ] Decide on Bubbletea WithContext and message publish context propagation; implement if agreed.
- [ ] Validate shutdown: streams/actions stop on TUI exit; no blocked publishes or orphaned processes.
