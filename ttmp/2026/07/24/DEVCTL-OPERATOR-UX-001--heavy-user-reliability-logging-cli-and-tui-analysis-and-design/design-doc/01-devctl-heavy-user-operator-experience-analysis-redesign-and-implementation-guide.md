---
Title: devctl Heavy-User Operator Experience Analysis, Redesign, and Implementation Guide
Ticket: DEVCTL-OPERATOR-UX-001
Status: active
Topics:
    - devctl
    - tui
    - architecture
    - supervisor
    - cli
    - refactor
DocType: design-doc
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: "Intern-ready analysis and redesign of devctl's process supervision, logging, CLI, and TUI operator architecture."
LastUpdated: 2026-07-24T13:22:12.13927521-04:00
WhatFor: "Provide the technical basis and phased implementation contract for making devctl reliable, observable, ergonomic, and maintainable."
WhenToUse: "Read before implementing changes to devctl service lifecycle, state, logs, commands, or terminal UI."
---

# devctl Heavy-User Operator Experience Analysis, Redesign, and Implementation Guide

## Executive Summary

This document is under active evidence gathering. Its final form will teach a
new engineer how devctl works today, identify concrete operator and maintenance
failures, compare alternative designs, and specify a phased redesign without
requiring the implementer to make unresolved architectural decisions.

## Problem Statement

Devctl has grown from a process launcher into a plugin pipeline, supervisor,
state store, log viewer, command framework, and multi-view terminal
application. The central research question is whether those parts form a
coherent daily operator tool and, where they do not, which parts should be
repaired, consolidated, or removed.

## Research Status

- Repository and ticket baseline: complete.
- Current architecture mapping: in progress.
- Comparative operator research: pending.
- Design decisions and implementation roadmap: pending.

## References

- [Investigation Diary](../reference/01-investigation-diary.md)
- [Ticket Tasks](../tasks.md)
