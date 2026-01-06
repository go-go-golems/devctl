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

