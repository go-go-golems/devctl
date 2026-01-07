---
Title: Complete TUI features per MO-006 design
Ticket: MO-009-TUI-COMPLETE-FEATURES
Status: active
Topics:
    - backend
    - ui-components
DocType: index
Intent: long-term
Owners: []
RelatedFiles:
    - Path: devctl/pkg/tui/domain.go
      Note: EventLogEntry schema
    - Path: devctl/pkg/tui/models/eventlog_model.go
      Note: Events view rendering
    - Path: devctl/pkg/tui/transform.go
      Note: Domain-to-UI event mapping
ExternalSources: []
Summary: Implement all 75+ missing TUI features identified in MO-008 gap analysis to achieve full design parity with MO-006
LastUpdated: 2026-01-07T21:23:22-05:00
WhatFor: Complete the devctl TUI implementation
WhenToUse: When implementing new TUI features or checking remaining work
---


# Complete TUI features per MO-006 design

## Overview

This ticket tracks the comprehensive implementation of all missing TUI features identified in the MO-008 gap analysis. The goal is to bring `devctl tui` to full parity with the MO-006 design specification.

### Background

- **MO-006**: Original TUI design with ASCII mockups
- **MO-008**: Added lipgloss styling, widgets, fixed visual issues
- **MO-009 (this)**: Complete all remaining features

### Scope

75+ tasks organized into 8 phases:

1. **Data Layer** (12 tasks): Process stats, health checks, env vars
2. **Dashboard** (11 tasks): Health/CPU/MEM columns, events preview, plugins summary
3. **Service Detail** (9 tasks): Process info, health, env vars, keybindings
4. **Events View** (14 tasks): Service/level columns, filters, stats, pause
5. **Pipeline View** (10 tasks): Progress bars, live output, config patches
6. **Plugin View** (5 tasks): New view for plugin inspection
7. **Navigation** (3 tasks): Direct view keybindings
8. **Polish** (11 tasks): Responsive layout, consistency, testing

## Key Links

- **Implementation Plan**: [design/01-implementation-plan.md](./design/01-implementation-plan.md)
- **Task Tracker**: [tasks.md](./tasks.md)
- **Development Diary**: [reference/01-diary.md](./reference/01-diary.md)

### Related Tickets

- **MO-006**: Original TUI design - `ttmp/2026/01/06/MO-006-DEVCTL-TUI--create-a-devctl-tui/`
- **MO-008**: Visual improvements - `ttmp/2026/01/06/MO-008-IMPROVE-TUI-LOOKS--improve-tui-looks-and-architecture/`

### Key Source Files

- Current TUI models: `pkg/tui/models/*.go`
- Widgets: `pkg/tui/widgets/*.go`
- Styles: `pkg/tui/styles/*.go`
- Domain types: `pkg/tui/domain.go`

## Status

Current status: **active**

## Topics

- backend
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
