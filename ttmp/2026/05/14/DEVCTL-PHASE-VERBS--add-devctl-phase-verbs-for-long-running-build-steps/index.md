---
Title: Add devctl phase verbs for long-running build steps
Ticket: DEVCTL-PHASE-VERBS
Status: active
Topics:
    - devctl
    - cli
    - build
    - workflow
DocType: index
Intent: long-term
Owners: []
RelatedFiles:
    - Path: cmd/devctl/cmds/plan.go
      Note: Existing partial phase verb used as template
    - Path: cmd/devctl/cmds/up.go
      Note: Full pipeline command that currently owns build/prepare/validate sequencing
    - Path: pkg/engine/pipeline.go
      Note: Core phase API and merge semantics
    - Path: pkg/protocol/types.go
      Note: Protocol frames and event stream schema
    - Path: pkg/runtime/client.go
      Note: Runtime request/response and stream invocation API
ExternalSources: []
Summary: Research and design ticket for adding first-class devctl phase verbs such as devctl build.
LastUpdated: 2026-05-14T14:30:00-04:00
WhatFor: Track the design and implementation plan for direct devctl phase commands with visible long-running build progress.
WhenToUse: Use before implementing phase verbs or changing devctl build/prepare/validate workflow semantics.
---






# Add devctl phase verbs for long-running build steps

## Overview

This ticket captures the design for adding first-class devctl commands that run individual pipeline phases. The motivating case is running the Pinocchio web-chat build phase directly in `/home/manuel/workspaces/2026-05-13/coinvault-loop-analysis/pinocchio`, with a longer timeout and visible progress, instead of invoking the full `devctl up` lifecycle.

The recommended first implementation adds top-level `devctl build`, `devctl prepare`, and `devctl validate` commands that reuse existing repository loading and `engine.Pipeline` methods. Structured protocol streaming is documented as a follow-up; the first implementation should rely on plugin stderr progress, which the current runtime already reads safely without contaminating protocol stdout.

## Key Links

- [Design: devctl phase verbs for streaming long-running steps](./design-doc/01-design-devctl-phase-verbs-for-streaming-long-running-steps.md)
- [Diary](./reference/01-diary.md)
- [Tasks](./tasks.md)
- [Changelog](./changelog.md)

## Status

Current status: **active**. Research/design is complete; implementation remains a follow-up.

## Topics

- devctl
- cli
- build
- workflow

## Structure

- `design-doc/` — primary architecture and implementation guide.
- `reference/` — investigation diary.
- `tasks.md` — completed documentation tasks and follow-up implementation tasks.
- `changelog.md` — ticket-level changes and decisions.
