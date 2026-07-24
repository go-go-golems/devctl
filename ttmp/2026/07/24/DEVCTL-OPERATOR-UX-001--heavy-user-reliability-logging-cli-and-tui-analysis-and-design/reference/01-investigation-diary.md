---
Title: Investigation Diary
Ticket: DEVCTL-OPERATOR-UX-001
Status: active
Topics:
    - devctl
    - tui
    - architecture
    - supervisor
    - cli
    - refactor
DocType: reference
Intent: long-term
Owners: []
RelatedFiles:
    - Path: repo://.ttmp.yaml
      Note: Repository-local docmgr ownership boundary discovered during Step 1
    - Path: repo://cmd/devctl/cmds/wrap_service.go
      Note: Existing supervision correction that anchors the reliability case study
    - Path: repo://cmd/devctl/cmds/wrap_service_test.go
      Note: Regression evidence for the committed process-group fix
ExternalSources: []
Summary: Chronological evidence, experiments, failures, and decisions for the devctl operator-experience research ticket.
LastUpdated: 2026-07-24T13:22:12.311783489-04:00
WhatFor: Preserve how the architecture findings and redesign recommendations were derived.
WhenToUse: Read when reviewing the design, reproducing an observation, or resuming the investigation.
---


# Investigation Diary

## Goal

Record the evidence-first investigation of devctl's supervision, state,
logging, CLI, and TUI behavior, including failed attempts and the reasoning
behind each design recommendation. The diary is deliberately separate from
the polished design guide: it preserves chronology and uncertainty so a future
engineer can audit how conclusions were reached.

## Step 1: Establish the repository and ticket baseline

This step established the correct documentation root, created the research
ticket, and converted the broad request into explicit tasks. It also fixed a
workspace-local tooling ambiguity before any research artifact could be placed
in the wrong repository.

The scope is analysis and design. The branch already contains the focused
supervision correction at commit `39ba416`; this ticket will explain and
evaluate it, but will not opportunistically implement the wider redesign.

### Prompt Context

**User prompt (verbatim):**

> Create a new ticket in devctl to fix devctl. Furthermore, before you start implementating, research about possible improvements you could make to devctl while you are at it, especially in terms of properly displaying and handling logs, ergonomic improvements to the CLI and TUI, things that could be removed and simplified that don't seem useful, etc...
>
> Review it all from the perspective of an actual heavy user like you are, with potentially more complex setups too.
>
> Create  a detailed analysis / design / implementation guide that is for a new intern, explaining all the parts of the system needed to understand what it is, with prose paragraphs and bullet point sand pseudocode and diagrams and api references and file references. It should be very clear and technical. Store in the ticket and the nupload to remarkable.

**Assistant interpretation:** Create a documentation-first devctl ticket that
audits the complete operator experience, compares it with mature tools,
recommends evidence-based simplifications, and supplies a decision-complete
future implementation guide without carrying out the broad refactor.

**Inferred user intent:** Make devctl dependable and pleasant enough for daily
use in complex repositories, while reducing accidental complexity and giving
a new contributor enough technical context to implement the redesign safely.

### What I did

- Verified the devctl branch and worktree were clean at
  `39ba416 fix(supervise): isolate wrapper from service signals`.
- Ran `docmgr status --summary-only` from the devctl worktree.
- Discovered that docmgr selected the parent workspace configuration and
  reported the TinyIDP documentation root.
- Added a repository-local `.ttmp.yaml` pointing to `devctl/ttmp` and its
  vocabulary.
- Re-ran docmgr status and confirmed the root was
  `/home/manuel/workspaces/2026-07-07/prod-tiny-idp/devctl/ttmp`.
- Created ticket `DEVCTL-OPERATOR-UX-001`, its primary design document, this
  diary, and eleven stable-ID tasks.

### Why

- A ticket created under the parent configuration would have violated the
  explicit requirement to store the work in devctl.
- Stable tasks make the research auditable and prevent a polished narrative
  from hiding incomplete evidence lanes.
- The existing supervision fix must be separated from proposed improvements so
  observed code and future design are not conflated.

### What worked

- A nearest-repository `.ttmp.yaml` cleanly overrides the parent workspace
  configuration.
- The existing devctl vocabulary already contains the required topics:
  `devctl`, `tui`, `architecture`, `supervisor`, `cli`, and `refactor`.
- Docmgr created the expected index, task list, changelog, design document,
  diary, and supporting directories.

### What didn't work

The first status check from `devctl/` resolved the wrong root:

```text
root=/home/manuel/workspaces/2026-07-07/prod-tiny-idp/tiny-idp/ttmp
config=/home/manuel/workspaces/2026-07-07/prod-tiny-idp/.ttmp.yaml
```

No ticket was created before this was corrected. The repository-local
configuration changed the result to:

```text
root=/home/manuel/workspaces/2026-07-07/prod-tiny-idp/devctl/ttmp
config=/home/manuel/workspaces/2026-07-07/prod-tiny-idp/devctl/.ttmp.yaml
```

The first commit gate then stopped because the changelog updater left an extra
blank line at the end of its generated section:

```text
ttmp/2026/07/24/DEVCTL-OPERATOR-UX-001--heavy-user-reliability-logging-cli-and-tui-analysis-and-design/changelog.md:15: new blank line at EOF.
```

No commit was created. The trailing blank line was removed and the same
`git diff --cached --check` gate was rerun.

### What I learned

- A nested worktree can inherit an unrelated parent docmgr root even when it
  already contains its own `ttmp/` directory.
- The existence of `ttmp/` is not enough to establish docmgr ownership; the
  nearest `.ttmp.yaml` is the controlling boundary.
- Current ticket count and staleness metrics are only meaningful after that
  boundary is verified.

### What was tricky to build

- The wrong root was syntactically valid and writable, so this failure mode
  would not have produced an error. The only safe approach was to inspect the
  resolved root before mutation.
- The research scope spans several partially independent systems. The task
  list separates them so evidence from one surface, such as service logs,
  cannot be used to imply completion of another, such as TUI action handling.

### What warrants a second pair of eyes

- Confirm that committing `.ttmp.yaml` is the desired long-term behavior for
  devctl when it is checked out inside larger workspaces. It matches sibling
  repositories in this workspace.
- Review the final pruning decisions carefully because the user explicitly
  permits removal without compatibility adapters, but removal still requires
  evidence that a surface is not carrying important workflows.

### What should be done in the future

- Keep ticket-local source captures and probes docmgr-compatible.
- Distinguish current observations, measured behavior, external precedents,
  and proposed design throughout the primary guide.

### Code review instructions

- Start with `.ttmp.yaml` and verify `docmgr status --summary-only` resolves
  `devctl/ttmp`.
- Inspect `tasks.md` and confirm every requested analysis lane is represented.
- Verify the branch begins with the supervision fix at `39ba416`.

### Technical details

The evidence hierarchy for the ticket is:

1. Executed behavior and captured artifacts.
2. Current source code and tests with line references.
3. Current repository documentation and historical ticket records.
4. Official upstream documentation for comparison tools.
5. Design inference, explicitly labeled as proposed.
