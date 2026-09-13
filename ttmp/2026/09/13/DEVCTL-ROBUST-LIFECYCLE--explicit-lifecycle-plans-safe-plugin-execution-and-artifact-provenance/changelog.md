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
