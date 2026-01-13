---
Title: JavaScript log processor for devctl
Ticket: MO-007-LOG-PARSER
Status: active
Topics:
    - backend
DocType: index
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources:
    - local:js-log-parser-spec.md
    - local:js-log-parser-examples.md
Summary: ""
LastUpdated: 2026-01-06T18:28:08-05:00
WhatFor: ""
WhenToUse: ""
---



# JavaScript log processor for devctl

## Overview

Build a small, safe-by-default JavaScript log parser (goja-based) that turns raw log lines into structured events (NDJSON). The MVP ships first as a standalone `cmd/log-parse` binary for fast iteration; `devctl` integration is explicitly future work.

We imported an intentionally over-scoped “LogFlow” spec + examples as *sources* to learn from, but the MVP explicitly trims that scope down to a synchronous, single-module, line-oriented pipeline.

## Key Links

- MVP design doc: `design-doc/01-mvp-design-javascript-log-parser-goja.md`
- Trimmed spec notes: `reference/01-source-notes-provided-spec-trimmed-for-mvp.md`
- Diary: `diary/01-diary.md`
- Imported sources (do not treat as MVP requirements):
  - `sources/local/js-log-parser-spec.md`
  - `sources/local/js-log-parser-examples.md`

## Status

Current status: **active**

## Topics

- backend

## Tasks

See [tasks.md](./tasks.md) for the current task list.

## Changelog

See [changelog.md](./changelog.md) for recent changes and decisions.

## Structure

- design/ - Architecture and design documents
- reference/ - Prompt packs, API contracts, context summaries
- playbooks/ - Command sequences and test procedures
- scripts/ - Temporary code and tooling
- various/ - Working notes and research
- archive/ - Deprecated or reference-only artifacts
