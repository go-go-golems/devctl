---
Title: Command Namespace Registry Design
Ticket: DEVCTL-ROBUST-LIFECYCLE
Status: complete
Topics:
    - devctl
    - architecture
    - plugins
    - supervisor
DocType: design-doc
Intent: long-term
Owners: []
RelatedFiles:
    - Path: repo://cmd/devctl/cmds/command_namespace.go
      Note: Authoritative root command and alias namespace
    - Path: repo://cmd/devctl/cmds/plugins.go
      Note: Catalog consumers use registry snapshots
    - Path: repo://cmd/devctl/cmds/root.go
      Note: Built-in registration through the namespace
ExternalSources: []
Summary: Use one discovered command namespace for Cobra registration and plugin catalog collision checks.
LastUpdated: 2026-09-13T13:07:09.563536317-04:00
WhatFor: Prevent built-in and dynamic plugin command names from drifting between registration and catalog validation.
WhenToUse: When adding root commands, aliases, or plugin catalog validation paths.
---

# Command Namespace Registry Design

## Executive Summary

Treat root command names and aliases as one namespace shared by Cobra construction and plugin-catalog validation. Built-ins are registered through a small generic `CommandNamespace`; catalog paths consume immutable snapshots of that same registry.

## Problem Statement

The old `defaultReservedCommandNames` manually duplicated the commands attached in `AddCommands`. Adding `schema` updated Cobra but not catalog refresh, so refresh could accept a plugin command that bootstrap later rejected.

## Proposed Solution

- `CommandNamespace.Add` atomically reserves a command name and aliases before attaching it to a Cobra parent.
- `RootCommandNamespace` discovers already attached commands and reserves Cobra's lazily materialized `help` and `completion` names.
- The plugins command retains the namespace populated by `AddCommands`; inspect, refresh, static fallback, and explicit execution all use `Snapshot()`.
- Dynamic bootstrap derives its namespace from the completed root rather than maintaining another built-in list.

## Design Decisions

- Scope the utility to one immediate Cobra child namespace; recursive command trees have independent namespaces.
- Return copied maps at the catalog boundary so consumers cannot mutate registry state.
- Reject duplicate names and aliases during construction, before Cobra execution or catalog persistence.
- Keep Cobra as rendering/execution infrastructure, not the authoritative collision policy.

## Alternatives Considered

Keeping a corrected string list was rejected because every future root command could recreate the drift. Deriving names only from Cobra was insufficient for plugin subcommands that need the same policy later and for Cobra's lazy implicit commands.

## Implementation and Validation

Refactor `AddCommands`, dynamic bootstrap, and all plugin catalog operations to use the registry. Cover registration, aliases, `schema` reservation during static/refresh validation, and abandoned-stream shutdown independently. Existing catalog and built-CLI tests remain the integration gate.

## Open Questions

None for this change. If nested plugin commands become supported, instantiate a namespace per parent rather than making this registry recursive.
