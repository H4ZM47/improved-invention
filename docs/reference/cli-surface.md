# CLI Surface

Date: 2026-04-20
Status: Initial v1 command-group definition

## Scope

This document defines the top-level Task CLI command groups and cross-cutting naming rules for v1.

It intentionally does not lock:

- JSON payload schemas
- exact exit code assignments
- full flag sets for every leaf command

Those are follow-on contract issues built on top of this command map.

## Executable Name

The v1 executable name is:

```text
task
```

Examples in docs, scripts, and tests should use `task ...` as the canonical command form.

## Command Design Rules

- The root command owns the primary task workflow because the executable itself is already named `task`.
- Additional top-level commands are nouns that map to first-class product concepts or platform functions.
- Leaf commands are verbs such as `create`, `list`, `show`, `update`, `claim`, or `export`.
- Commands should favor explicit flags over positional magic when non-interactive automation is expected.
- A command that mutates state must be runnable without prompts.
- Human prompts are additive behavior, not the only path to completing work.

## Top-Level Command Groups

### Root Task Commands

Primary task operations live directly under the root command rather than under a nested `task` group.

Expected responsibility:

- create tasks
- show task detail
- list tasks
- update fields
- change status
- claim, renew, release, and unlock
- manage task relationships
- manage task links
- manage task time tracking

Representative shape:

```text
task create ...
task list ...
task show <task-ref>
task update <task-ref> ...
task claim <task-ref> ...
```

Note:

- task lifecycle, claims, relationships, links, and time tracking stay at the root because the executable already names the primary entity.

### `project`

Project record management.

Expected responsibility:

- create projects
- show project detail
- list projects
- update project fields
- close or reopen projects
- manage default assignee settings

### `domain`

Domain record management.

Expected responsibility:

- create domains
- show domain detail
- list domains
- update domain fields
- close or reopen domains
- manage default assignee settings

### `actor`

Actor inspection and human-configuration workflows.

Expected responsibility:

- inspect actors
- list configured humans and observed agents
- configure or update the local human actor
- show actor detail for assignment and claim history

Note:

- agents are still created implicitly on first use; `actor` exists for visibility and human configuration, not to force pre-registration.

### `view`

Saved-view management.

Expected responsibility:

- create saved views
- list saved views
- show saved view definitions
- update saved views
- delete saved views
- apply saved views to task listings

Rationale:

- saved views are first-class reusable query definitions, not just ad hoc flags on `task list`.

### `export`

Structured and human-readable export workflows.

Expected responsibility:

- export tasks to JSON
- export tasks to CSV
- export tasks to TXT
- export tasks to built-in Markdown

### `report`

Read-only local HTML reporting.

Expected responsibility:

- start the local report server
- select bind address or port
- choose filters or initial view parameters

### `backup`

Full-fidelity backup creation.

Expected responsibility:

- create a portable backup artifact that preserves UUIDs, history, links, and actor references

### `restore`

Full-fidelity restore operations.

Expected responsibility:

- restore a previously created backup artifact into a local database

### `config`

Local machine configuration and environment inspection.

Expected responsibility:

- inspect resolved database path
- inspect current actor configuration
- inspect current runtime paths and defaults

Rationale:

- keeping config inspection separate avoids overloading entity commands with machine-level concerns.

## Global Flags

Every top-level command group should be designed to support a consistent shared set of global flags. The final set can expand, but v1 should reserve these concepts now:

- `--json`: emit machine-readable output
- `--no-input`: disable prompts and require explicit non-interactive behavior
- `--db <path>`: override the resolved database path
- `--actor <ref>`: identify the acting human or agent when explicit override is needed
- `--quiet`: suppress non-essential human-oriented output

## Reference Style

Commands that target existing records should accept stable human-usable references first and UUIDs second.

Preferred reference order:

1. handle such as `TASK-1042`
2. UUID

This keeps normal use human-friendly while preserving durable machine references.

## Example Tree

```text
task
  create
  list
  show
  update
  claim
  renew
  release
  unlock
  start
  pause
  resume
  close
  project
    create
    list
    show
    update
    close
  domain
    create
    list
    show
    update
    close
  actor
    list
    show
    configure-human
  view
    create
    list
    show
    update
    delete
    apply
  export
    json
    csv
    txt
    markdown
  report
    serve
  backup
    create
  restore
    run
  config
    show
```

## Naming Consequences

This command map intentionally keeps:

- task-specific workflows at the root
- other entity management at the top level
- portability features under `backup`, `restore`, and `export`
- machine configuration under `config`

That split should make the CLI easier for both humans and agents to scan, script, and document.
