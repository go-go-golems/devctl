# Tasks

## TODO

- [x] Decide MVP normalization rules (timestamp + unknown fields)
- [x] Implement `devctl/pkg/logjs` (goja runtime + `register()` + helpers)
- [x] Implement standalone `log-parse` CLI (`devctl/cmd/log-parse`) to exercise `devctl/pkg/logjs`
- [x] Add golden tests for a few JS scripts (JSON, logfmt, regex)
- [x] Add timeout test (infinite loop interrupted via `Runtime.Interrupt`)
- [x] Add examples under `devctl/examples/` (optional)
- [ ] (Future) Integrate with `devctl logs` (`devctl/cmd/devctl/cmds/logs.go`)
