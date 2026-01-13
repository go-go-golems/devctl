# Tasks

## TODO

- [x] Decide MVP normalization rules (timestamp + unknown fields)
- [x] Implement `devctl/pkg/logjs` (goja runtime + `register()` + helpers)
- [x] Implement standalone `log-parse` CLI (`devctl/cmd/log-parse`) to exercise `devctl/pkg/logjs`
- [x] Add golden tests for a few JS scripts (JSON, logfmt, regex)
- [x] Add timeout test (infinite loop interrupted via `Runtime.Interrupt`)
- [x] Add examples under `devctl/examples/` (optional)
- [ ] (Future) Integrate with `devctl logs` (`devctl/cmd/devctl/cmds/logs.go`)
- [x] Design/Implement: multi-module fan-out mode (each script self-contained; run all parsers per line; tag output with module tag)
- [x] CLI: add repeatable --module and --modules-dir for loading many scripts; add --validate and --print-pipeline
- [x] Tests: add multi-module unit tests (tagging, independent state, module error isolation)
- [x] Scripts/: add runnable scripts for build+test loops (e.g. scripts/01-run-fanout-demo.sh, scripts/02-run-validation.sh)
- [x] Scripts: add build/test loop for multi-module fan-out
- [x] Scripts: add runnable multi-module demo (many JS modules)
- [x] Implement logjs Fanout runner (N modules per line, tagged outputs)
- [x] CLI: --module repeat + --modules-dir + deterministic load order
- [x] CLI: validate mode (compile, one register per file, unique names)
- [x] CLI: --print-pipeline and --stats (per module)
- [x] Engine: allow parse to return array (0..N events per line)
- [x] Engine: per-module error isolation + optional dead-letter output
- [x] Stdlib v1: parsing helpers (parseJSON/logfmt/kv + capture)
- [x] Stdlib v1: event/tag helpers (field/getPath, addTag/hasTag)
- [x] Stdlib v1: time + numeric helpers (parseTimestamp, toNumber)
- [x] Multiline v1: createMultilineBuffer (single-worker, per-module)
- [x] Tests: multi-module fanout (tagging, state isolation, error isolation)
- [x] Examples: many modules directory + sample input + README
- [x] Scripts: multi-module demo runner + validation runner
