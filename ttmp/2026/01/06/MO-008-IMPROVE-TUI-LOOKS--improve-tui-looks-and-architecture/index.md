---
Title: Improve TUI Looks and Architecture
Ticket: MO-008-IMPROVE-TUI-LOOKS
Status: complete
Topics:
    - tui
    - ui-components
    - backend
DocType: index
Intent: long-term
Owners: []
RelatedFiles:
    - Path: pkg/tui/models/dashboard_model.go
      Note: Dashboard model (services table
    - Path: pkg/tui/models/eventlog_model.go
      Note: Event log model (timeline
    - Path: pkg/tui/models/pipeline_model.go
      Note: Pipeline model (phases/steps
    - Path: pkg/tui/models/root_model.go
      Note: Root model coordinator (primary refactoring target)
    - Path: pkg/tui/models/service_model.go
      Note: Service detail model (log viewport
    - Path: ttmp/2026/01/06/MO-006-DEVCTL-TUI--create-a-devctl-tui/sources/local/01-devctl-tui-layout.md
      Note: Original ASCII baseline mockups defining target UX
ExternalSources: []
Summary: Analyze and improve devctl TUI visual appearance using lipgloss styling, bordered widgets, status icons, and color-coded states. Refactor model rendering to match MO-006 ASCII baseline mockups.
LastUpdated: 2026-01-13T17:06:17.328753105-05:00
WhatFor: ""
WhenToUse: ""
---




# Improve TUI Looks and Architecture

## Overview

<!-- Provide a brief overview of the ticket, its goals, and current status -->

## Key Links

- **Related Files**: See frontmatter RelatedFiles field
- **External Sources**: See frontmatter ExternalSources field

## Status

Current status: **active**

## Topics

- tui
- ui-components
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
