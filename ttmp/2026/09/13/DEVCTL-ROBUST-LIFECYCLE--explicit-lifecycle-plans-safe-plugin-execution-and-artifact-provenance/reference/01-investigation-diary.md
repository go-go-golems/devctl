---
Title: Investigation diary
Ticket: DEVCTL-ROBUST-LIFECYCLE
Status: active
Topics: [devctl, architecture, plugins, supervisor]
DocType: reference
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: Source investigation and reMarkable delivery of the lifecycle robustness design.
LastUpdated: 2026-09-13T11:00:00Z
---

# Investigation diary

## Step 1: Source-grounded design and intern handoff

I created a new ticket in devctl and wrote a standalone implementation guide covering all six proposed improvements plus documentation and testing ergonomics. The guide distinguishes current source behavior from proposed APIs and preserves the existing build-before-stop and unproven-termination refusal properties. Runtime code was read only.

### Prompt Context
**User prompt (verbatim):**
> Ok, create a new docmgr ticket in devctl about all these improvements, and write a design doc covering them all.Create  a detailed analysis / design / implementation guide that is for a new intern, explaining all the parts of the system needed to understand what it is, with prose paragraphs and bullet point sand pseudocode and diagrams and api references and file references. It should be very clear and technical. Store in the ticket and the nupload to remarkable.

**Delivery clarification (verbatim):**
> no need to go through extensive remarkable validation btw. just upload

**Assistant interpretation:** Produce a documentation-only research/design ticket and upload the guide directly, with no implementation or extensive PDF validation.
**Inferred user intent:** Give an intern enough architecture and source context to implement the improvements safely without rediscovering the system.

### What I did
- Read AGENT.md and inspected clean repository revision `418a4ca1f18d3059c1b1cf8f846db371163529b5`.
- Read operator request/planner/controller paths, runtime close and termination, engine artifact types, catalog fingerprint/refresh/load behavior, run-state health/exit types, and restart tests.
- Created the ticket, detailed design, this diary, file relations, and nine open implementation tasks through docmgr.
- Wrote architecture orientation, concrete protocol examples, ASCII flow diagrams, proposed APIs, state/lifetime contracts, intern phases, validation matrix, and migration/trust caveats.
- Uploaded the design through remarquee without PDF-only rendering, screenshots, or an extra cloud listing.

### Why
- The planner name hides side effects; a recipe/preparation/application split must preserve current pre-stop guarantees.
- Plugin EOF shutdown currently immediately races SIGTERM, while Close discards cleanup errors.
- Path-only artifacts do not identify what an old process launched.
- Catalog discovery intentionally avoids implicit plugin execution; usability changes must preserve that trust boundary.

### What worked
- Source confirms restart builds/prepares/validates unless skipped, rather than only replanning.
- Source already stores health timestamps, making historical-health display a projection change rather than destructive schema reinterpretation.
- Catalog source revealed interpreter metadata does not hash script contents; the guide proposes explicit declared fingerprint inputs.
- Direct upload succeeded:
```text
OK: uploaded DEVCTL ROBUST LIFECYCLE Intern Design Guide.pdf -> /ai/2026/09/13/DEVCTL-ROBUST-LIFECYCLE
```

### What didn't work
- An initial source search included a guessed `pkg/operator/types.go` path:
```text
rg: pkg/operator/types.go: No such file or directory (os error 2)
```
- Listing the package located `requests.go`; subsequent reading used the actual files. No code changes or repeated speculative fixes were made.

### What I learned
- Recipe resolution cannot always produce final launch facts before builds exist. The design must explicitly represent unresolved artifact-dependent fields.
- The current catalog cache path is repo-scoped while the catalog includes a profile; profile-specific storage is a design question rather than an assumption of independent caches.

### What was tricky to build
- A naive pure-plan API would either secretly build or claim knowledge it cannot have. The guide separates recipe intent from prepared launch facts.
- Graceful Close must have exactly one Wait owner across successful startup, handshake failure, repeated Close, and cancellation.
- Typed artifact identity narrows provenance uncertainty but cannot claim to cover every dynamically loaded library or interpreter dependency.

### What warrants a second pair of eyes
- Review protocol/schema migration scope before implementation; repository guidance forbids implicit compatibility layers.
- Review lifecycle/build locking so long compilation cannot unnecessarily block urgent stop operations.
- Review signal/process-group assumptions and the boundary against deliberately detached descendants.

### What should be done in the future
- Review API choices, then implement phases with the ticket tasks. None are closed merely because the guide is delivered.

### Code review instructions
- Start with the guide source map and invariant list, then follow the phased work packages.
- Re-run implementation tests only when runtime code changes; no test-suite success is claimed for this documentation-only task.

### Technical details
- Ticket: DEVCTL-ROBUST-LIFECYCLE.
- reMarkable destination: `/ai/2026/09/13/DEVCTL-ROBUST-LIFECYCLE`.
- Delivery evidence proves cloud upload, not physical tablet synchronization.
- No runtime files modified, no services launched, and no extensive rendering validation performed.
