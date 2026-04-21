# CLI Surface

Date: 2026-04-21
Status: Current v1 command-group definition

## Scope

This document defines the top-level Grind command groups and cross-cutting naming rules for v1.

It intentionally does not lock:

- JSON payload schemas
- exact exit code assignments
- full flag sets for every leaf command

Those are follow-on contract issues built on top of this command map.

## Executable Name

The v1 executable name is:

```text
grind
```

Examples in docs, scripts, and tests should use `grind ...` as the canonical command form.

## Command Design Rules

- The root command owns the primary task workflow because the executable itself is already named `grind`.
- Additional top-level commands are nouns that map to first-class product concepts or platform functions.
- Leaf commands are verbs such as `create`, `list`, `show`, `update`, `open`, `close`, `cancel`, or `export`.
- Purely informational root affordances should use `--option` flags rather than noun-style commands.
- Commands should favor explicit flags over positional magic when non-interactive automation is expected.
- A command that mutates state must be runnable without prompts.
- Human prompts are additive behavior, not the only path to completing work.

## Root Informational Flags

The canonical root-level informational surfaces are:

- `grind --version`
- `grind --config`
- `grind --agents`
- `grind --agent-help`

These are treated as option-style entrypoints rather than command groups because they expose root metadata, configuration, or built-in documentation rather than a first-class resource workflow.

## Top-Level Command Groups

### Root Task Commands

Primary task operations live directly under the root command rather than under a nested `task` group.

Expected responsibility:

- create tasks
- show task detail
- list tasks
- update fields
- change status
- claim, renew, release, and unlock through a nested claim namespace
- manage task links
- manage task time tracking

Representative shape:

```text
grind create ...
grind list ...
grind show <task-ref>
grind update <task-ref> ...
grind claim acquire <task-ref>
```

Note:

- task lifecycle, the `claim` namespace, the `time` namespace, and link operations stay near the root because the executable already names the primary entity.

### `project`

Project record management.

Expected responsibility:

- create projects
- show project detail
- list projects
- update project fields
- open, close, or cancel projects
- manage default assignee settings

### `domain`

Domain record management.

Expected responsibility:

- create domains
- show domain detail
- list domains
- update domain fields
- open, close, or cancel domains
- manage default assignee settings

### `actor`

Actor inspection and human-configuration workflows.

Expected responsibility:

- inspect actors
- list configured humans and observed agents
- show actor detail for assignment and claim history

Note:

- agents are still created implicitly on first use; `actor` exists for visibility and inspection, not to force pre-registration.

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

- saved views are first-class reusable query definitions, not just ad hoc flags on `grind list`.

### `export`

Structured and human-readable export workflows.

Expected responsibility:

- export tasks to JSON
- export tasks to CSV
- export tasks to TXT
- export tasks to built-in Markdown

### `backup`

Full-fidelity backup creation.

Expected responsibility:

- create a portable backup artifact that preserves UUIDs, history, links, and actor references

### `restore`

Full-fidelity restore operations.

Expected responsibility:

- restore a previously created backup artifact into a local database

### `serve`

Read-only local HTML reporting.

Expected responsibility:

- start the local report server
- select bind address or port
- choose filters or initial view parameters

## Global Flags

Every top-level command group should be designed to support a consistent shared set of global flags. The final set can expand, but v1 should reserve these concepts now:

- `--json`: emit machine-readable output
- `--no-input`: disable prompts and require explicit non-interactive behavior
- `--db <path>`: override the resolved database path
- `--actor <ref>`: identify the acting human or agent when explicit override is needed
- `--quiet`: suppress non-essential human-oriented output
- `--version`: show build information and exit
- `--config`: show resolved runtime configuration and exit
- `--agents`: show the built-in agent operating guide and exit
- `--agent-help`: alias for `--agents`

## Reference Style

Commands that target existing records should accept stable human-usable references first and UUIDs second.

Preferred reference order:

1. handle such as `TASK-1042`
2. UUID

This keeps normal use human-friendly while preserving durable machine references.

## Example Tree

```text
grind
  create
  list
  show
  update
  open
  close
  cancel
  claim
    acquire
    renew
    release
    unlock
  time
    start
    pause
    resume
    add
    edit
  project
    create
    list
    show
    update
    open
    close
    cancel
  domain
    create
    list
    show
    update
    open
    close
    cancel
  milestone
    create
    list
    show
    update
    open
    close
    cancel
  actor
    list
    show
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
  backup
    create
  restore
  serve
  link-repo
```

## Naming Consequences

This command map intentionally keeps:

- task-specific workflows at the root
- other entity management at the top level
- portability features under `backup`, `restore`, `export`, and `serve`
- machine configuration as root-level flags instead of noun commands

That split should make the CLI easier for both humans and agents to scan, script, and document.
