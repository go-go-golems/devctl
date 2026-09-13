# Changelog

## 2026-09-13

- Initial workspace created

## 2026-09-13

Created source-grounded intern design covering six lifecycle improvements and supporting documentation/test ergonomics. Uploaded guide successfully to /ai/2026/09/13/DEVCTL-ROBUST-LIFECYCLE; runtime implementation remains open.

## 2026-09-13

Implemented a shared runstate health projection that separates current liveness from retained timed health observations; status rows and human output now use it.

### Related Files

- /home/manuel/workspaces/2026-09-13/devctl-improve/devctl/cmd/devctl/cmds/status.go — Structured and human status projection consumer
- /home/manuel/workspaces/2026-09-13/devctl-improve/devctl/pkg/runstate/health.go — Canonical current-versus-historical health projection

## 2026-09-13

Added non-executing catalog inspection, static-versus-handshake provenance, schema-v2 declared catalog source fingerprints, and documented catalog_inputs.

### Related Files

- /home/manuel/workspaces/2026-09-13/devctl-improve/devctl/cmd/devctl/cmds/plugins.go — plugins catalog command output
- /home/manuel/workspaces/2026-09-13/devctl-improve/devctl/pkg/plugincatalog/catalog.go — Schema-v2 provider provenance and declared source hashing
- /home/manuel/workspaces/2026-09-13/devctl-improve/devctl/pkg/plugincatalog/inspection.go — Non-executing missing stale valid and conflicted catalog inspection

## 2026-09-13

Added tested raw help export and compact JSON command schema discovery, corrected stale lifecycle examples, and locked the current lifecycle phase order with an executable fixture journal.

### Related Files

- /home/manuel/workspaces/2026-09-13/devctl-improve/devctl/cmd/devctl/cmds/schema.go — Machine-readable command schema surface
- /home/manuel/workspaces/2026-09-13/devctl-improve/devctl/pkg/operator/planner_test.go — Executable default and skipped phase-order matrix

## 2026-09-13

Implemented one-owner plugin process waiting with idempotent graceful EOF, TERM, and KILL shutdown, cancellation-safe background cleanup, descendant verification, and race-tested fixtures.

### Related Files

- /home/manuel/workspaces/2026-09-13/devctl-improve/devctl/pkg/runtime/process_lifetime.go — Single Wait owner and bounded process-group shutdown state machine
- /home/manuel/workspaces/2026-09-13/devctl-improve/devctl/pkg/runtime/shutdown_test.go — EOF TERM KILL descendant concurrent and canceled Close fixtures

## 2026-09-13

Shipped a standalone supported Python subprocess runner with shared monotonic budgets, protocol-safe streaming, bounded tails, dry-run, cancellation, TERM/KILL process-group cleanup, and CI-backed fixtures.

### Related Files

- /home/manuel/workspaces/2026-09-13/devctl-improve/devctl/sdk/python/devctl_runner.py — Supported standalone bounded runner for Python plugins
- /home/manuel/workspaces/2026-09-13/devctl-improve/devctl/sdk/python/test_devctl_runner.py — Runner budget output dry-run cancellation and descendant fixtures

## 2026-09-13

Archived the browser-readable context inventory and classified session timeline beside the investigation diary.

### Related Files

- /home/manuel/workspaces/2026-09-13/devctl-improve/devctl/ttmp/2026/09/13/DEVCTL-ROBUST-LIFECYCLE--explicit-lifecycle-plans-safe-plugin-execution-and-artifact-provenance/reference/02-context-window-and-session-timeline.html — Session context audit requested during Phase 3

## 2026-09-13

Expanded the archived context report with concrete read edited added and removed API/function inventories, including concise ownership descriptions and the in-progress Glazed v1.4 migration.

### Related Files

- /home/manuel/workspaces/2026-09-13/devctl-improve/devctl/ttmp/2026/09/13/DEVCTL-ROBUST-LIFECYCLE--explicit-lifecycle-plans-safe-plugin-execution-and-artifact-provenance/reference/02-context-window-and-session-timeline.html — Function-level session implementation audit

## 2026-09-13

Extracted the context-window and classified session-timeline report into a reusable session-context-audit skill with a strict JSON contract, standalone renderer, fixture, escaping tests, and the motivating report as a visual example.

### Related Files

- /home/manuel/workspaces/2026-09-13/devctl-improve/devctl/skills/session-context-audit/SKILL.md — Project skill for evidence-backed session context audits

## 2026-09-13

Installed and committed the session-context-audit skill in the shared skill repository, then published a 2,967-word textbook-style technical deep dive to the go-go-parc Obsidian vault.

### Related Files

- /home/manuel/code/wesen/go-go-golems/go-go-parc/Projects/2026/09/13/ARTICLE - Session Context Audits - Evidence Models Timelines and Reusable HTML Reports.md — Published technical analysis of the skill architecture and evidence model

## 2026-09-13

Migrated devctl to Glazed v1.4 command builders and structured-output flags, removed the private legacy output bridge, made streaming JSONL explicit, propagated command-construction errors, and fixed Glazed to flush structured rows on command failure.

### Related Files

- /home/manuel/workspaces/2026-09-13/devctl-improve/devctl/cmd/devctl/cmds/lifecycle.go — Canonical Glazed builder integration and constructor error propagation
- /home/manuel/workspaces/2026-09-13/devctl-improve/devctl/cmd/devctl/cmds/logs.go — Human/structured dual-mode log output
- /home/manuel/workspaces/2026-09-13/devctl-improve/devctl/cmd/devctl/cmds/stream.go — Human/JSONL dual-mode stream output
- /home/manuel/workspaces/2026-09-13/devctl-improve/glazed/pkg/cli/cobra.go — Structured-output flushing and Cobra writer ownership

## 2026-09-13

Separated lifecycle recipe resolution, effectful replacement preparation, and under-lock stale validation while retaining build and prepare results for artifact provenance.

### Related Files

- /home/manuel/workspaces/2026-09-13/devctl-improve/devctl/pkg/operator/planner.go — Versioned recipes prepared launches phase results and configuration fingerprints
- /home/manuel/workspaces/2026-09-13/devctl-improve/devctl/pkg/operator/controller.go — Stale validation before applying up or stopping during restart
- /home/manuel/workspaces/2026-09-13/devctl-improve/devctl/ttmp/2026/09/13/DEVCTL-ROBUST-LIFECYCLE--explicit-lifecycle-plans-safe-plugin-execution-and-artifact-provenance/reference/03-lifecycle-recipe-and-schema-decisions.md — Accepted timeout and migration decisions

## 2026-09-13

Implemented simplified native executable provenance with content-addressed publication, run schema v2 linkage, pre-launch digest validation, current/last reference garbage collection, structured status evidence, and embedded Glazed help.

### Related Files

- /home/manuel/workspaces/2026-09-13/devctl-improve/devctl/pkg/operator/artifacts.go — Staging publication deduplication and reference collector
- /home/manuel/workspaces/2026-09-13/devctl-improve/devctl/pkg/runstate/artifact.go — SHA-256 and executable identity validation
- /home/manuel/workspaces/2026-09-13/devctl-improve/devctl/pkg/supervise/supervisor.go — Final artifact integrity check before wrapper launch
- /home/manuel/workspaces/2026-09-13/devctl-improve/devctl/pkg/doc/topics/devctl-artifact-provenance.md — Embedded help for plugin authors and operators

## 2026-09-13

Added effect-free lifecycle recipe explanation, concurrent artifact-preparation evidence, and a requirement-to-test qualification matrix; full tests, race tests, lint, vet, build, Python fixtures, embedded help, and docmgr hygiene passed.

### Related Files

- /home/manuel/workspaces/2026-09-13/devctl-improve/devctl/cmd/devctl/cmds/lifecycle.go — Effect-free up and restart recipe projection
- /home/manuel/workspaces/2026-09-13/devctl-improve/devctl/cmd/devctl/cmds/cli_contract_test.go — Built-binary no-provider explain fixture
- /home/manuel/workspaces/2026-09-13/devctl-improve/devctl/pkg/operator/artifacts_test.go — Concurrent staging deduplication and collector safety evidence
- /home/manuel/workspaces/2026-09-13/devctl-improve/devctl/ttmp/2026/09/13/DEVCTL-ROBUST-LIFECYCLE--explicit-lifecycle-plans-safe-plugin-execution-and-artifact-provenance/reference/04-implementation-qualification-matrix.md — Completion audit

## 2026-09-13

Closed after full offline lifecycle qualification at devctl a14d611 with Glazed dependency e0cfa33
