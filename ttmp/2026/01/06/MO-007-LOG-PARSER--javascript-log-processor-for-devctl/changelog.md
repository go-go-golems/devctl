# Changelog

## 2026-01-06

- Initial workspace created


## 2026-01-06

Set up ticket, imported spec/examples as sources, and wrote an MVP goja-based design (single-module, sync, line→event NDJSON) with explicit scope trimming.

### Related Files

- /home/manuel/workspaces/2026-01-06/log-parser-module/devctl/ttmp/2026/01/06/MO-007-LOG-PARSER--javascript-log-processor-for-devctl/design-doc/01-mvp-design-javascript-log-parser-goja.md — Defines MVP contracts and Go implementation plan


## 2026-01-06

MVP delivery: build standalone cmd/log-parse to exercise JS parsing engine; defer devctl logs integration until engine is stable.

### Related Files

- /home/manuel/workspaces/2026-01-06/log-parser-module/devctl/ttmp/2026/01/06/MO-007-LOG-PARSER--javascript-log-processor-for-devctl/design-doc/01-mvp-design-javascript-log-parser-goja.md — Updated MVP delivery plan and CLI UX


## 2026-01-06

Implemented MVP engine (devctl/pkg/logjs) + standalone cmd/log-parse, with tests and timeout handling (commit 9b86bc031454347e03d78b237e817c735dd50392).

### Related Files

- /home/manuel/workspaces/2026-01-06/log-parser-module/devctl/pkg/logjs/module.go — Core engine implementation


## 2026-01-06

Drafted next-step design for multi-script log-parse pipelines (multi parse modules, shared stages, validation/introspection) (commit 8ee3e97).

### Related Files

- /home/manuel/workspaces/2026-01-06/log-parser-module/devctl/ttmp/2026/01/06/MO-007-LOG-PARSER--javascript-log-processor-for-devctl/design-doc/02-next-step-design-multi-script-pipeline-for-log-parse.md — Defines pipeline semantics and CLI evolution


## 2026-01-06

Clarify multi-module fan-out semantics (self-contained modules emit tagged derived streams); fix regex example to avoid goja named capture groups; add tasks for multi-module build/test scripts.

### Related Files

- /home/manuel/workspaces/2026-01-06/log-parser-module/devctl/examples/log-parse/parser-regex.js — Regex example fix
- /home/manuel/workspaces/2026-01-06/log-parser-module/devctl/ttmp/2026/01/06/MO-007-LOG-PARSER--javascript-log-processor-for-devctl/design-doc/02-next-step-design-multi-script-pipeline-for-log-parse.md — Updated design semantics
- /home/manuel/workspaces/2026-01-06/log-parser-module/devctl/ttmp/2026/01/06/MO-007-LOG-PARSER--javascript-log-processor-for-devctl/tasks.md — New tasks 12/13


## 2026-01-06

Roadmap: decompose imported LogFlow spec into phased fan-out plan; add tasks (14–26); add design-doc/03 aligned with design-doc/02.

### Related Files

- /home/manuel/workspaces/2026-01-06/log-parser-module/devctl/ttmp/2026/01/06/MO-007-LOG-PARSER--javascript-log-processor-for-devctl/design-doc/03-roadmap-design-from-fan-out-log-parse-to-logflow-ish-system.md — New roadmap design
- /home/manuel/workspaces/2026-01-06/log-parser-module/devctl/ttmp/2026/01/06/MO-007-LOG-PARSER--javascript-log-processor-for-devctl/tasks.md — New task breakdown


## 2026-01-06

Implement multi-module fan-out: add logjs Fanout runner + tagging, update log-parse CLI to load many modules (--module/--modules-dir) with validate/print-pipeline/stats, add multi-module tests, and update scripts/docs from --js -> --module.

### Related Files

- /home/manuel/workspaces/2026-01-06/log-parser-module/devctl/cmd/log-parse/main.go — CLI multi-module support
- /home/manuel/workspaces/2026-01-06/log-parser-module/devctl/pkg/logjs/fanout.go — Fanout runner
- /home/manuel/workspaces/2026-01-06/log-parser-module/devctl/pkg/logjs/fanout_test.go — Multi-module tests


## 2026-01-06

Implement tasks 18–22: allow parse/transform to return arrays (0..N events), add ErrorRecord + optional NDJSON error stream (--errors), and expand stdlib helpers (parseKeyValue/capture/getPath/addTag/toNumber/parseTimestamp).

### Related Files

- /home/manuel/workspaces/2026-01-06/log-parser-module/devctl/cmd/log-parse/main.go — --errors output
- /home/manuel/workspaces/2026-01-06/log-parser-module/devctl/pkg/logjs/helpers.go — Stdlib helper expansion
- /home/manuel/workspaces/2026-01-06/log-parser-module/devctl/pkg/logjs/module.go — Multi-event returns + error records


## 2026-01-07

Implement multiline helper (createMultilineBuffer), add many-module fan-out example directory + sample input + README updates, and add runnable demo/validate ticket scripts (tasks 23, 12/13, 25/26).

### Related Files

- /home/manuel/workspaces/2026-01-06/log-parser-module/devctl/examples/log-parse/modules/01-errors.js — Example modules
- /home/manuel/workspaces/2026-01-06/log-parser-module/devctl/pkg/logjs/helpers.go — Multiline helper
- /home/manuel/workspaces/2026-01-06/log-parser-module/devctl/ttmp/2026/01/06/MO-007-LOG-PARSER--javascript-log-processor-for-devctl/scripts/04-run-fanout-modules-dir-demo.sh — Demo script


## 2026-01-08

Added analysis of long-term documentation migration candidates and missing log-parser docs

### Related Files

- /home/manuel/workspaces/2026-01-06/log-parser-module/devctl/ttmp/2026/01/06/MO-007-LOG-PARSER--javascript-log-processor-for-devctl/analysis/01-long-term-documentation-investigation.md — Summarizes migration targets and missing docs


## 2026-01-08

Created comprehensive log-parse developer guide covering module API, helper functions, fan-out pipeline, CLI reference, Go integration, real-world patterns, and troubleshooting

### Related Files

- /home/manuel/workspaces/2026-01-06/log-parser-module/devctl/pkg/doc/topics/log-parse-guide.md — Comprehensive developer guide (14 sections


## 2026-01-13

Close: per request (devctl logs integration remaining)

