---
Title: devctl cleanup pass
Ticket: MO-010-DEVCTL-CLEANUP-PASS
Status: active
Topics:
    - backend
    - tui
    - refactor
    - ui-components
DocType: index
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: ""
LastUpdated: 2026-01-07T13:47:33.628021485-05:00
WhatFor: ""
WhenToUse: ""
---

# devctl cleanup pass

## Overview

MO-010 captures a documentation-first cleanup pass over `devctl`’s service supervision and TUI integration:
- how services are started/stopped (wrapper vs direct mode),
- what state/log artifacts exist on disk,
- what “events” are emitted and how the UI consumes them,
- and a reproduced comprehensive fixture failure during `devctl up` (with a copy/paste repro + root cause).

## Key Links

- `analysis/01-service-supervision-architecture-events-and-ui-integration.md`
- `analysis/02-comprehensive-fixture-devctl-up-failure-bug-report.md`
- `reference/01-diary.md`
- **Related Files**: See frontmatter RelatedFiles field
- **External Sources**: See frontmatter ExternalSources field

## Status

Current status: **active**

## Topics

- backend
- tui
- refactor
- ui-components

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
