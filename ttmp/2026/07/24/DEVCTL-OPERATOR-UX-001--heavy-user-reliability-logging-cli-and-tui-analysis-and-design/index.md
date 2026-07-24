---
Title: Heavy-User Reliability, Logging, CLI, and TUI Analysis and Design
Ticket: DEVCTL-OPERATOR-UX-001
Status: active
Topics:
    - devctl
    - tui
    - architecture
    - supervisor
    - cli
    - refactor
DocType: index
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: "Evidence-based analysis and redesign of devctl supervision, logging, CLI, and TUI behavior for complex daily development workflows."
LastUpdated: 2026-07-24T13:22:11.981532802-04:00
WhatFor: "Use this ticket to understand devctl's current operator architecture, assess its usability and maintenance costs, and implement the recommended reliability and UX roadmap."
WhenToUse: "Read before changing devctl supervision, logs, CLI commands, TUI models, protocol streams, or the JavaScript log parser."
---

# Heavy-User Reliability, Logging, CLI, and TUI Analysis and Design

## Overview

This ticket is a documentation-first assessment of devctl as a tool used every
day to operate small and complex development environments. It maps the current
process supervisor, persistent state, logging surfaces, command-line
interfaces, plugin protocol, and terminal UI. It then proposes a simpler,
more reliable operator architecture with explicit implementation contracts.

The ticket does not implement the broader redesign. The already committed
process-group correction at `39ba416` is treated as a concrete reliability
case study. All other changes remain proposed until reviewed.

## Key Links

- **Related Files**: See frontmatter RelatedFiles field
- **External Sources**: See frontmatter ExternalSources field
- **Primary guide**: [devctl Heavy-User Operator Experience Analysis, Redesign, and Implementation Guide](./design-doc/01-devctl-heavy-user-operator-experience-analysis-redesign-and-implementation-guide.md)
- **Investigation diary**: [Investigation Diary](./reference/01-investigation-diary.md)

## Status

Current status: **active research**

## Topics

- devctl
- tui
- architecture
- supervisor
- cli
- refactor

## Tasks

See [tasks.md](./tasks.md) for the current task list.

## Changelog

See [changelog.md](./changelog.md) for recent changes and decisions.

## Structure

- `design-doc/` — the primary analysis, architecture, and implementation guide.
- `reference/` — the chronological investigation diary.
- `sources/` — captured external evidence used in comparisons.
- `scripts/` — numbered, reproducible inspection and behavior probes.
- `tasks.md` — research completion and delivery checklist.
- `changelog.md` — concise ticket milestones.
