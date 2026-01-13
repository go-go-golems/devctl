---
Title: Sandbox goja_nodejs require() for log-parse
Ticket: MO-008-REQUIRE-SANDBOX
Status: complete
Topics:
    - backend
DocType: index
Intent: long-term
Owners: []
RelatedFiles:
    - Path: devctl/cmd/log-parse/main.go
      Note: Future flag surface for enabling sandboxed require()/node console
    - Path: devctl/pkg/logjs/module.go
      Note: Future integration point for sandboxed require() (if we add require support)
ExternalSources: []
Summary: ""
LastUpdated: 2026-01-13T17:06:22.617500798-05:00
WhatFor: ""
WhenToUse: ""
---



# Sandbox goja_nodejs require() for log-parse

## Overview

Define and implement a sandboxed `goja_nodejs/require` configuration so we can enable `require()` (and optionally `goja_nodejs/console`) in `log-parse`/`devctl/pkg/logjs` without allowing arbitrary filesystem module loading.

## Key Links

- Design doc: `design-doc/02-analysis-sandboxing-goja-nodejs-require.md`

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
