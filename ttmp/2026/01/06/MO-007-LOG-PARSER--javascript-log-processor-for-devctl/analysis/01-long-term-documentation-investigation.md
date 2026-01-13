---
Title: Long-Term Documentation Investigation
Ticket: MO-007-LOG-PARSER
Status: active
Topics:
    - backend
DocType: analysis
Intent: long-term
Owners: []
RelatedFiles:
    - Path: ttmp/2026/01/06/MO-007-LOG-PARSER--javascript-log-processor-for-devctl/design-doc/01-mvp-design-javascript-log-parser-goja.md
      Note: MVP architecture and contract slated for long-term docs
    - Path: ttmp/2026/01/06/MO-007-LOG-PARSER--javascript-log-processor-for-devctl/design-doc/02-next-step-design-multi-script-pipeline-for-log-parse.md
      Note: Defines fan-out semantics and tagging rules to preserve
    - Path: ttmp/2026/01/06/MO-007-LOG-PARSER--javascript-log-processor-for-devctl/design-doc/03-roadmap-design-from-fan-out-log-parse-to-logflow-ish-system.md
      Note: Roadmap framing for long-term scope decisions
    - Path: ttmp/2026/01/06/MO-007-LOG-PARSER--javascript-log-processor-for-devctl/diary/01-diary.md
      Note: Summarizes implementation steps and feature surface used in the investigation
    - Path: ttmp/2026/01/06/MO-007-LOG-PARSER--javascript-log-processor-for-devctl/playbook/01-playbook-log-parse-mvp-testing.md
      Note: Testing playbook to migrate into durable docs
    - Path: ttmp/2026/01/06/MO-008-REQUIRE-SANDBOX--sandbox-goja-nodejs-require-for-log-parse/design-doc/02-analysis-sandboxing-goja-nodejs-require.md
      Note: Require sandboxing analysis to migrate for security guidance
ExternalSources: []
Summary: Inventory of log-parser ticket docs that should move to long-term documentation and a list of missing docs to support adoption.
LastUpdated: 2026-01-08T08:18:29.817672286-05:00
WhatFor: Identify long-term documentation that should move out of ttmp/ and outline missing docs needed for the log-parser feature.
WhenToUse: Use when migrating ticket docs into durable documentation or planning new docs for log-parse/logjs.
---


# Long-Term Documentation Investigation

## Goal

Inventory the log-parser related ticket docs that should graduate to long-term documentation, then list new long-term docs needed to help teams adopt and integrate the feature.

## Sources Reviewed

- MO-007 diary and design docs (MVP, fan-out pipeline, roadmap).
- MO-007 playbook for testing.
- MO-008 require() sandboxing analysis.
- Git history for log-parse/logjs (`cmd/log-parse`, `pkg/logjs`) to understand current feature surface.

## Long-Term Docs to Move Out of `ttmp/`

These are already written as long-term docs but currently live under `ttmp/`. They should move into a durable documentation area (suggested target under `docs/log-parser/` or `docs/architecture/log-parser/`).

- `ttmp/2026/01/06/MO-007-LOG-PARSER--javascript-log-processor-for-devctl/design-doc/01-mvp-design-javascript-log-parser-goja.md`
  - Suggested target: `docs/architecture/log-parser/mvp-design.md`
  - Reason: Canonical MVP contract and runtime boundaries; useful for future maintenance and onboarding.
- `ttmp/2026/01/06/MO-007-LOG-PARSER--javascript-log-processor-for-devctl/design-doc/02-next-step-design-multi-script-pipeline-for-log-parse.md`
  - Suggested target: `docs/architecture/log-parser/fanout-pipeline-design.md`
  - Reason: Defines fan-out semantics and tagging contracts that are still active in code.
- `ttmp/2026/01/06/MO-007-LOG-PARSER--javascript-log-processor-for-devctl/design-doc/03-roadmap-design-from-fan-out-log-parse-to-logflow-ish-system.md`
  - Suggested target: `docs/architecture/log-parser/roadmap.md`
  - Reason: Captures phased evolution and non-goals; still the best reference for future scope decisions.
- `ttmp/2026/01/06/MO-007-LOG-PARSER--javascript-log-processor-for-devctl/playbook/01-playbook-log-parse-mvp-testing.md`
  - Suggested target: `docs/playbooks/log-parse-testing.md`
  - Reason: Repeatable validation commands for regressions and onboarding.
- `ttmp/2026/01/06/MO-007-LOG-PARSER--javascript-log-processor-for-devctl/reference/01-source-notes-provided-spec-trimmed-for-mvp.md`
  - Suggested target: `docs/architecture/log-parser/spec-trim-notes.md`
  - Reason: Captures the rationale for trimmed MVP scope; helpful for future scope debates.
- `ttmp/2026/01/06/MO-008-REQUIRE-SANDBOX--sandbox-goja-nodejs-require-for-log-parse/design-doc/02-analysis-sandboxing-goja-nodejs-require.md`
  - Suggested target: `docs/architecture/log-parser/require-sandbox.md`
  - Reason: Security-sensitive design guidance for enabling `require()` safely.

Docs that should stay in `ttmp/`:

- `ttmp/2026/01/06/MO-007-LOG-PARSER--javascript-log-processor-for-devctl/diary/01-diary.md` (implementation diary).
- `ttmp/2026/01/06/MO-007-LOG-PARSER--javascript-log-processor-for-devctl/sources/local/js-log-parser-spec.md` and `.../js-log-parser-examples.md` (external source snapshots).

## Log-Parser Feature Surface (from git log + diary)

Key capabilities currently implemented in `cmd/log-parse` and `pkg/logjs`:

- Goja-backed JS module loader with `register({ ... })` contract.
- Fan-out runner that loads many modules (`--module`, `--modules-dir`) and emits tagged derived streams.
- NDJSON output with normalization (event schema, tags, fields, source, lineNumber).
- `validate` subcommand and `--print-pipeline` / `--stats` for introspection.
- Multi-event returns from `parse`/`transform` (array outputs).
- Structured error records (`--errors` stream) for hook failures.
- Stdlib helpers: parseKeyValue, capture, getPath/hasPath, tag helpers, toNumber, parseTimestamp.
- Multiline buffer helper (`log.createMultilineBuffer`) with deterministic flush-on-next-line semantics.
- Example suite under `examples/log-parse` and testing playbook scripts in `ttmp/`.

## Potential Long-Term Docs to Write

Based on the implementation and current gaps, these documents would help adoption and integration:

- `docs/log-parser/README.md` or `docs/log-parser/overview.md`
  - What the log parser is, when to use it, high-level architecture.
- `docs/log-parser/cli-reference.md`
  - `log-parse` CLI flags, subcommands, examples, exit codes.
- `docs/log-parser/js-module-api.md`
  - `register()` contract, hook semantics, context shape, normalization rules.
- `docs/log-parser/event-schema.md`
  - NDJSON event schema, tagging rules, `_tag`/`_module` fields, error record schema.
- `docs/log-parser/helpers-reference.md`
  - `log.*` helper APIs, behavior notes, examples (parseTimestamp, createMultilineBuffer).
- `docs/log-parser/fanout-pipeline.md`
  - Multi-module semantics, `--modules-dir` ordering, module tags, error isolation.
- `docs/log-parser/multiline-guide.md`
  - Multiline buffer usage patterns, flush behavior, EOF considerations.
- `docs/log-parser/sandboxing.md`
  - `require()` policy, filesystem restrictions, native module allowlist (from MO-008).
- `docs/log-parser/testing-playbook.md`
  - Migrate and expand the current playbook with regression checks.
- `docs/log-parser/integration-with-devctl.md`
  - How (and when) `log-parse` will integrate with `devctl logs` and pipeline hooks.
