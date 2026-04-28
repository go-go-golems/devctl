---
Title: ""
Ticket: ""
Status: ""
Topics: []
DocType: ""
Intent: ""
Owners: []
RelatedFiles:
    - Path: cmd/devctl/cmds/common.go
      Note: Core repo layer and parsing helpers - main migration target
    - Path: cmd/devctl/cmds/plugins.go
      Note: Plugins list command using old WriterCommand interface
    - Path: cmd/devctl/cmds/status.go
      Note: Status command using old WriterCommand interface
ExternalSources: []
Summary: ""
LastUpdated: 0001-01-01T00:00:00Z
WhatFor: ""
WhenToUse: ""
---




# Migration Analysis: devctl → glazed schema/fields/values

## Goal

Migrate devctl from the legacy glazed API (`layers`, `parameters`, `middlewares`, `parsedlayers`) to the new facade API (`schema`, `fields`, `values`, `sources`). This is a breaking change; the old alias shims have been removed in glazed v1.2.3.

## Scope

Only Go source files under `cmd/` that import glazed command packages. The core devctl packages (`pkg/engine`, `pkg/runtime`, `pkg/state`, etc.) do not use glazed and are untouched.

## Files Affected

| File | Imports Used | Action |
|------|-------------|--------|
| `cmd/devctl/cmds/common.go` | `cmds/layers`, `cmds/parameters` | Full rewrite of layer/parameter usage |
| `cmd/devctl/cmds/plugins.go` | `cmds/layers` | Update `RunIntoWriter` sig, `WithLayersList` → `WithSections` |
| `cmd/devctl/cmds/status.go` | `cmds/layers`, `cmds/parameters` | Update `RunIntoWriter` sig, field definitions |
| `cmd/devctl/main.go` | `cmds/logging` | `AddLoggingLayerToRootCommand` → `AddLoggingSectionToRootCommand` |
| `cmd/log-parse/main.go` | `cmds/logging` | `AddLoggingLayerToRootCommand` → `AddLoggingSectionToRootCommand` |

## Conceptual Mapping

| Legacy | New |
|--------|-----|
| `layers.ParameterLayer` | `schema.Section` |
| `layers.ParameterLayerImpl` | `schema.SectionImpl` |
| `layers.NewParameterLayer` | `schema.NewSection` |
| `layers.ParsedLayers` | `values.Values` |
| `parsedLayers.InitializeStruct(slug, &s)` | `vals.DecodeSectionInto(slug, &s)` |
| `parameters.ParameterDefinition` | `fields.Definition` |
| `parameters.NewParameterDefinition` | `fields.New` |
| `parameters.ParameterTypeString` | `fields.TypeString` |
| `parameters.ParameterTypeBool` | `fields.TypeBool` |
| `parameters.ParameterTypeInteger` | `fields.TypeInteger` |
| `cmds.WithLayersList` | `cmds.WithSections` |
| `logging.AddLoggingLayerToRootCommand` | `logging.AddLoggingSectionToRootCommand` |
| Struct tag `glazed.parameter:"x"` | Struct tag `glazed:"x"` |
| `layer.AddLayerToCobraCommand(cmd)` | `schema.AddSectionToCobraCommand(cmd)` |
| `RunIntoWriter(ctx, *layers.ParsedLayers, w)` | `RunIntoWriter(ctx, *values.Values, w)` |

## Implementation Order

1. **common.go** — This is the foundation. Defines `getRepoLayer()`, `RepoContextFromParsedLayers()`, `AddRepoFlags()`. All other commands depend on it.
2. **plugins.go** — Depends on common.go helpers.
3. **status.go** — Depends on common.go helpers.
4. **main.go + log-parse/main.go** — Trivial logging renames.

## Validation Plan

- `cd devctl && go build ./...`
- `cd devctl && go test ./...`
- Run `devctl --help`, `devctl plugins list --help`, `devctl status --help` to confirm no runtime panics.
