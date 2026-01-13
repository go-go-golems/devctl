---
Title: Port startdev.sh to devctl plugin(s)
Ticket: MO-013-PORT-STARTDEV
Status: complete
Topics:
    - devctl
    - moments
    - devtools
    - scripting
DocType: index
Intent: long-term
Owners: []
RelatedFiles:
    - Path: moments/.devctl.yaml
      Note: Moments devctl configuration (entrypoint for new workflow).
    - Path: moments/plugins/moments-plugin.py
      Note: Plugin implementation target for the port.
    - Path: moments/scripts/startdev.sh
      Note: Legacy dev startup script being replaced.
ExternalSources: []
Summary: Design proposal to replace moments/scripts/startdev.sh with a protocol v2 devctl plugin (Moments backend + web dev server), explicitly dropping compatibility with the existing Moments devctl stub.
LastUpdated: 2026-01-13T16:09:43.78724396-05:00
WhatFor: Provide a single, devctl-native developer startup workflow replacing the legacy startdev.sh script.
WhenToUse: Use to find the current analysis/design docs and track progress via tasks and changelog.
---



# Port startdev.sh to devctl plugin(s)

## Overview

This ticket designs a protocol v2 `devctl` plugin to replace `moments/scripts/startdev.sh`. The plugin should encode repo knowledge (build/prepare steps, env derivation, service plan) while `devctl` owns lifecycle (supervision, logs, state).

## Key Links

- Diary: `reference/01-diary.md`
- Analysis: `working-note/01-analysis-replace-startdev-sh-with-devctl-plugin-s.md`
- Design: `design-doc/01-design-moments-devctl-plugin-s-replacing-startdev-sh.md`

## Status

Current status: **active**

## Topics

- devctl
- moments
- devtools
- scripting

## Tasks

See [tasks.md](./tasks.md) for the current task list.

## Changelog

See [changelog.md](./changelog.md) for recent changes and decisions.

## Structure

- `design-doc/` — Architecture and design documents
- `working-note/` — Research and analysis notes
- `reference/` — Long-lived reference docs (incl. diary)
- `scripts/` — Temporary code and tooling (if needed)
- `archive/` — Deprecated or reference-only artifacts
