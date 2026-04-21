# Task CLI Design

Date: 2026-04-20
Status: Approved design draft

## Summary

Task CLI is a cross-platform, CLI-first task management utility for a single machine with one configured human user and multiple ephemeral agent processes. Its primary job is to let humans and AI agents capture, review, and manage project-related work through the same interface, with a shared local SQLite database as the v1 source of truth.

The selected v1 shape is a CLI-first system with:

- low-friction capture,
- deterministic non-interactive automation,
- exclusive task claims to prevent concurrent work collisions,
- first-class tasks, projects, domains, actors, and event history,
- built-in export formats and a read-only local HTML report server,
- explicit room to evolve toward a future daemon or remote multi-user backend.

## Goals

- Make task capture extremely low friction. Creating a task must require only a title.
- Provide one interface that works well for both humans and AI agents.
- Support exclusive claim locking so only one worker is actively operating on a task at a time.
- Preserve full audit history for all meaningful changes.
- Support first-class domains and projects without requiring them at capture time.
- Support assignment to humans, claims by humans or agents, and explicit accountability.
- Support export, backup, restore, and portable migration to another machine.
- Keep the v1 architecture compatible with a future multi-machine or multi-user backend.

## Non-Goals

- Multi-machine sync in v1.
- Remote multi-human collaboration in v1.
- Browser-based task creation or editing in v1.
- A full-screen TUI in v1.
- Custom relationship types in v1.
- Custom markdown export templates in v1.
- A general query language for saved views in v1.
- Arbitrary user-defined metadata fields in v1 beyond user-defined tags.
- Hard deletion as a normal workflow operation.

## Operating Context

- Deployment model: single machine.
- Human model: one configured local human user.
- Agent model: many ephemeral local agent processes.
- Concurrency expectation: modest concurrency, roughly dozens of agents at most, mostly operating on different tasks.
- Source of truth: one local SQLite database plus linked references to external systems such as local files, Obsidian vault paths, URLs, or Git repositories.

## Selected Approach

V1 will use a CLI-first task system with a local SQLite core, an agent-safe CLI contract, and lightweight local HTML reporting.

Reasons:

- It best matches the requirement for low-friction capture and explicit automation.
- It avoids overbuilding a generalized work platform before the core workflow is proven.
- It is the safest path to future remote or multi-user evolution without forcing a later CLI redesign.

## Core Entities

V1 uses five first-class record types:

- `task`
- `project`
- `domain`
- `actor`
- `event`

The system keeps normalized current-state records for fast reads and an append-only event log for history and audit. State changes and corresponding event writes must happen in the same transaction.

### Task

Tasks are the primary work entity.

Required field:

- `title`

First-class task fields:

- `uuid`: immutable durable identifier
- `handle`: immutable human-usable short identifier, globally unique, auto-generated, format `TASK-<number>`
- `title`: editable human-readable title
- `description`: UTF-8 text with standard Markdown allowed; no HTML or custom markdown extensions are required
- `status`: one of `backlog`, `active`, `paused`, `blocked`, `completed`, `cancelled`
- `domain_id`: optional
- `project_id`: optional
- `assignee_actor_id`: optional
- `due_at`: optional
- `tags`: user-defined tags
- `external_links`: structured references to files, URLs, repos, or other external resources
- `created_at`
- `updated_at`
- `closed_at`: nullable materialized timestamp when terminal

Derived presentation groups:

- `backlog`
- `open(active|paused|blocked)`
- `closed(completed|cancelled)`

New tasks enter `backlog` and are unclaimed by default.

### Project

Projects are first-class managed entities, not just labels.

Project fields:

- `uuid`
- `handle`: immutable, globally unique, auto-generated, format `PROJ-<number>`
- `name`
- `description`
- `status`
- `domain_id`: required
- `default_assignee_actor_id`: optional
- `assignee_actor_id`: optional
- `due_at`: optional
- `tags`
- `external_links`
- `created_at`
- `updated_at`
- `closed_at`

Rules:

- Each project belongs to exactly one domain.
- A domain may contain many projects.
- Projects do not support claims or direct work-session time tracking.
- Project time views are rollups from child tasks.

### Domain

Domains are top-level first-class managed entities.

Domain fields:

- `uuid`
- `handle`: immutable, globally unique, auto-generated, format `DOM-<number>`
- `name`
- `description`
- `status`
- `default_assignee_actor_id`: optional
- `assignee_actor_id`: optional
- `due_at`: optional
- `tags`
- `external_links`
- `created_at`
- `updated_at`
- `closed_at`

Rules:

- Domains do not belong to a project or another domain.
- Domains do not support claims or direct work-session time tracking.
- Domain time views are rollups from child tasks.

### Actor

Actors are first-class identity records used by assignments, claims, and event history.

Actor fields:

- `uuid`
- `handle`: immutable, globally unique, auto-generated, format `ACT-<number>`
- `kind`: `human` or `agent`
- `provider`: optional for humans, required for agents such as `codex` or `claude`
- `external_id`: self-reported provider-scoped identifier
- `display_name`
- `first_seen_at`
- `last_seen_at`
- `created_at`
- `updated_at`

Rules:

- Humans are explicitly configured.
- Agents are implicitly created on first use from structured self-reported identity such as `codex:abc123`.
- Claims, assignments, and events reference actor records, not raw strings.
- Agent records are allowed to be self-reported and unverified in v1.

### Event

Events form the append-only audit and history log.

Each event records:

- event UUID
- event type
- entity type and entity UUID
- actor UUID when applicable
- timestamp
- structured payload capturing the specific change

Representative event types:

- create
- update
- status_change
- assign
- claim_acquired
- claim_released
- claim_expired
- claim_renewed
- unlock
- relationship_added
- relationship_removed
- link_added
- link_removed
- time_started
- time_paused
- time_resumed
- time_closed
- manual_time_added
- manual_time_edited
- restore

## Lifecycle And Status Rules

Canonical task status values are:

- `backlog`
- `active`
- `paused`
- `blocked`
- `completed`
- `cancelled`

Presentation and reporting should group them as:

- `backlog`
- `open(active|paused|blocked)`
- `closed(completed|cancelled)`

Transition rules:

- New tasks start in `backlog`.
- Tasks may move between `backlog` and any open state.
- Tasks may move from any open state to `completed` or `cancelled`.
- Closed tasks may be reopened into an open state.

Projects and domains use the same status vocabulary so they can be promoted or demoted more cleanly, but they do not participate in task claiming or direct work-session tracking.

## Claims, Concurrency, And Locking

Claims are exclusive task locks.

Rules:

- Only tasks may be claimed.
- New tasks are unclaimed by default.
- A task may have at most one active claim at a time.
- Normal state-changing task updates require an active claim.
- Claim acquisition, claim release, and task state changes must be transactionally consistent.
- Claims expire automatically after 24 hours unless renewed explicitly.
- Manual unlock is supported.
- Claim renewal is explicit only; normal task activity does not automatically extend the lease.

Design rationale:

- SQLite WAL mode plus short transactions is sufficient for the expected v1 concurrency profile.
- A separate write queue is not required in v1.
- General stale-write version checks are deferred.
- Claim locking is the workflow-level concurrency control for tasks.

Project and domain updates do not use claim locks in v1. They rely on ordinary transactional writes and are expected to be low contention.

## Assignment Model

Assignment and claim are separate concepts.

- `assignee` expresses responsibility and accountability.
- `claim holder` expresses exclusive active work.

Expected operating pattern:

- Tasks are commonly assigned to a human.
- Tasks are often claimed by an agent performing the work.
- Humans may also claim tasks when needed.

Projects and domains may define default assignees.

Default assignment rules:

- If a task is created with an explicit project or domain, and a default assignee is available, the task inherits it automatically at creation.
- If both project and domain defaults exist, the project default takes precedence because it is more specific.
- If an existing task is later moved into a project or domain with a default assignee, interactive commands prompt for a choice.
- Non-interactive reclassification commands must not mutate `assignee` silently. They must require one of:
  - accept the default assignee,
  - set an explicit assignee,
  - keep the current assignee unchanged.
- If a non-interactive caller does not specify the choice, the command fails with a specific machine-readable error.

## Relationships And Hierarchy

Task hierarchy rules:

- A task has at most one parent task.
- A task may have many child tasks.
- Project-to-domain containment is modeled separately through `project.domain_id`.
- Domains do not nest.

V1 supports the following built-in relationship types:

- `parent_of` / `child_of`
- `blocks` / `blocked_by`
- `related_to`
- `sibling_of`
- `duplicate_of`
- `supersedes`

Sibling semantics:

- Siblings may be derived from a shared parent.
- Siblings may also be linked explicitly even without the same parent.

Relationship scope:

- Non-hierarchical links such as `blocks`, `related_to`, `duplicate_of`, and `supersedes` may cross project and domain boundaries.
- `parent_of` and `child_of` in v1 apply to tasks, not to domains or projects.

Custom relationship types are out of scope for v1.

## Time Tracking

Direct time tracking is task-only.

Supported v1 behavior:

- derive elapsed work time from task events such as start, pause, resume, and close
- support manual time entry and manual time editing
- preserve full event history for time changes
- allow backfilled timestamps when needed for import, correction, or manual entry

Projects and domains do not have their own active work sessions. Their time reports are rollups from descendant tasks.

## Search, Views, And Reporting

### Listing And Filtering

V1 supports:

- list tasks
- filter by built-in fields
- filter by tags
- simple named saved views over built-in fields and tags
- free-text search over titles and descriptions in an agent-friendly style similar to `grep` or `rg`

Saved views are intentionally simple in v1. A general query language is deferred to v2.

### Export

V1 must support:

- JSON
- CSV
- TXT
- built-in Markdown format

Custom markdown templates are deferred to v2.

### Local HTML Report Server

The local HTML report server is read-only in v1.

Must-have views:

- task list with filters
- task detail page or inline task expansion

Other graphical or dashboard-style views are deferred to v2.

## External Links And Repo Awareness

External systems are linked references only in v1. There is no import or sync behavior with GitHub, Obsidian, or other systems.

External links should support at least:

- local file paths
- URLs
- repository or worktree references

Repo awareness is helper-only:

- the CLI may detect the current git repo or worktree for prompts and filters
- the CLI may support commands such as listing tasks for the current repo context
- attaching repo context to a task must always require explicit user intent
- non-interactive agent usage must be able to attach current repo context with an explicit flag

The system must not silently attach repo metadata just because a command ran inside a repository.

## CLI Contract

The CLI is both a human interface and an automation interface. It is not acceptable for agents to scrape human-oriented terminal text as the primary integration method.

Required guarantees:

- every state-changing command must be runnable non-interactively
- JSON output must be stable and documented
- exit codes must be deterministic and granular
- human-friendly output may exist, but cannot be the only interface

V1 remains command/subcommand based. A full-screen TUI is deferred to v2.

Human interaction rules:

- prompts are allowed for interactive workflows
- prompts must never be required for automation
- any prompt-backed decision must also be expressible with explicit non-interactive flags

## Error Model

V1 should define a granular exit-code model and matching structured JSON errors.

Representative error classes:

- entity not found
- claim required
- claim conflict
- claim expired
- invalid state transition
- invalid relationship
- validation failure
- assignment decision required
- export failure
- backup or restore failure
- storage busy or unavailable
- internal error

Error granularity is a requirement, not a nice-to-have. Generic failures should be minimized because they are poor for both humans and agents.

## Backup, Restore, And Portability

V1 must support full-fidelity backup and restore for migration to another machine.

Backup and restore requirements:

- preserve UUIDs
- preserve handles
- preserve current state
- preserve event history
- preserve relationships
- preserve external links
- preserve actor records
- preserve claim state as stored at backup time

On restore, expired claims should behave as expired when the restored database is next opened.

These backup and restore flows are distinct from normal reporting exports such as CSV or Markdown.

## Architecture

Recommended layers:

- CLI layer
- application or service layer
- storage layer
- report and export layer
- actor and context helper layer

### CLI Layer

Responsibilities:

- parse commands and flags
- manage interactive prompts
- render human-readable output
- render structured machine output
- map failures to deterministic exit codes

### Application Or Service Layer

Responsibilities:

- enforce lifecycle rules
- enforce claim requirements
- enforce assignment rules
- validate relationships
- coordinate exports, backups, and reporting

### Storage Layer

Responsibilities:

- encapsulate SQLite schema and migrations
- configure WAL mode and busy timeout
- execute transactional writes
- persist both current state and events

### Report And Export Layer

Responsibilities:

- generate JSON, CSV, TXT, and Markdown exports
- serve the local HTML report views
- read from the same application boundary as the CLI so behavior stays consistent

This separation is important even in v1 because it keeps open the future path of swapping direct SQLite access for a daemon or remote service without redesigning the CLI contract.

## Validation Rules

- `title` is the only required field at task creation.
- `domain` and `project` are always optional for tasks.
- Tasks may remain unclassified even while active.
- Projects must belong to exactly one domain.
- Domains must not belong to another domain or project.
- Task parent/child hierarchy is separate from project/domain classification.
- Claims are exclusive and task-only.
- Direct work-session time tracking is task-only.
- Default assignee inheritance at task creation is allowed when classification is explicit.
- Non-interactive reassignment decisions during reclassification must be explicit.
- Hard deletion is exceptional and not part of normal workflow.

## Deletion And Preservation

Normal workflow should preserve records rather than remove them.

Expected behavior:

- use terminal statuses such as `completed` or `cancelled`
- hide terminal records from default views unless explicitly requested
- allow hard deletion only for exceptional technical reasons through an explicit admin-style path

There is no separate `archived` field in v1 because terminal status already captures the required lifecycle state.

## Cross-Platform Support And Distribution

V1 is officially cross-platform:

- macOS
- Linux
- Windows

Distribution requirements:

- first-party standalone binaries
- Homebrew support on macOS
- first-party Linux package-manager support via Debian and RPM packages
- winget support on Windows

Additional package-manager channels such as Scoop or Chocolatey may be added later, but are not required for v1.

## Testing Strategy

### Unit Tests

Cover:

- lifecycle rules
- claim rules
- assignment inheritance and prompt requirements
- relationship validation
- time tracking math
- exit-code mapping

### Integration Tests

Cover:

- schema migration
- transactional claim and update behavior
- concurrent claim races
- event history integrity
- export generation
- backup and restore fidelity
- WAL-mode behavior under modest contention

### CLI Contract Tests

Cover:

- stable JSON output
- deterministic exit codes
- non-interactive command behavior
- human-readable output for core workflows

Critical scenario tests:

- two agents race to claim the same task and only one succeeds
- an expired claim is cleared and a second actor claims the task
- an existing task is reclassified into a domain or project with a default assignee in interactive and non-interactive modes
- backup and restore preserve UUIDs, history, links, and actor references
- the local HTML report reflects the same filtered task state as CLI queries

## Future-Proofing Constraints

V1 is intentionally local-first, but it must not block future remote or multi-user evolution.

Required forward-compatible choices:

- durable UUIDs across all entity types
- explicit actor records for humans and agents
- append-only event history
- a storage or service boundary above raw SQLite calls
- CLI semantics that can survive a future backend swap

Likely future directions enabled by this design:

- multi-machine sync or remote service backend
- authenticated multi-human collaboration
- stronger actor identity verification
- richer query language and reusable report definitions
- TUI
- browser-based create and edit flows
- custom markdown templates
- custom relationship types

## Open Decisions Deferred To Implementation Planning

- exact CLI command names and flag shapes
- exact JSON schema documents and exit-code number assignments
- exact HTML server framework
- exact SQLite schema details and migration tooling
- exact repo-reference payload shape
- exact packaging pipeline and release automation
