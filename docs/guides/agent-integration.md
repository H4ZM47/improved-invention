# Agent Guide

Grind is designed so agents can use the same interface as humans without screen scraping, prompt handling, or hidden side effects.

This guide is the practical operating manual for agents that read, claim, update, and complete work in Grind.

Related references:

- [CLI Contract](../reference/cli-contract.md)
- [Exit Codes](../reference/exit-codes.md)
- [JSON Success Examples](../reference/json-success-examples.md)
- [JSON Error Examples](../reference/json-error-examples.md)
- [CLI Surface](../reference/cli-surface.md)

Built-in entrypoint:

- `grind --agents`
- `grind --agent-help`

Do not use:

- `grind agents`
- `grind agentdocs`

## Core Rules

Treat these as hard requirements for agent-driven usage:

- always pass `--json`
- always pass `--no-input` for state-changing commands
- always pass an explicit `--actor`
- branch on exit codes and `error.code`, not human text
- claim a task before changing it
- never assume repo-aware helpers attach metadata implicitly

Baseline shape:

```sh
grind <command> ... --actor codex:agent-7 --no-input --json
```

For read-only commands, `--no-input` is optional but harmless.

## Identity

Use a structured self-reported agent identity:

- `codex:agent-7`
- `claude:batch-3`
- `cursor:worker-12`

Example:

```sh
grind list --status backlog --json --actor codex:agent-7
```

Agent actors are created implicitly on first claim or mutation if they do not already exist.

## Startup Checklist

When an agent starts operating against a Grind database:

1. Confirm the target database and runtime config.
2. Read candidate work.
3. Claim one task before mutating it.
4. Move it into `active` if work is beginning immediately.
5. Add links, notes, or time entries as work proceeds.
6. Close and release when done.

Minimal startup example:

```sh
grind --config --json --actor codex:agent-7
grind list --status backlog --json --actor codex:agent-7
grind claim TASK-1 --json --no-input --actor codex:agent-7
grind update TASK-1 --status active --json --no-input --actor codex:agent-7
```

## Reading Work

### List candidate tasks

```sh
grind list --status backlog --json --actor codex:agent-7
grind list --status active --assignee ACT-1 --json --actor codex:agent-7
grind list --tag docs --search release --json --actor codex:agent-7
```

### Use saved views

```sh
grind view list --json --actor codex:agent-7
grind view apply ready-docs --json --actor codex:agent-7
```

### Inspect one task

```sh
grind show TASK-1 --json --actor codex:agent-7
```

## Claims And Safe Mutation

Claims are exclusive task locks. A normal task update should fail unless the current actor holds the active claim.

### Claim a task

```sh
grind claim TASK-1 --json --no-input --actor codex:agent-7
```

### Update after claiming

```sh
grind update TASK-1 \
  --title "Rewrite release notes" \
  --status active \
  --json \
  --no-input \
  --actor codex:agent-7
```

### Renew a long-running claim

```sh
grind renew TASK-1 --json --no-input --actor codex:agent-7
```

### Release a task when work is done or handed off

```sh
grind release TASK-1 --json --no-input --actor codex:agent-7
```

### Manual unlock

`unlock` is an operator action, not the normal completion path:

```sh
grind unlock TASK-1 --json --no-input --actor codex:agent-7
```

## Status Model

Task lifecycle in v1:

- `backlog`
- `active`
- `paused`
- `blocked`
- `completed`
- `cancelled`

Typical agent path:

```sh
grind claim TASK-1 --json --no-input --actor codex:agent-7
grind update TASK-1 --status active --json --no-input --actor codex:agent-7
grind close TASK-1 --status completed --json --no-input --actor codex:agent-7
grind release TASK-1 --json --no-input --actor codex:agent-7
```

If a task is blocked:

```sh
grind update TASK-1 --status blocked --json --no-input --actor codex:agent-7
```

## Reclassification And Assignee Safety

Grind intentionally refuses silent assignee changes in non-interactive mode.

If a task is moved into a domain or project with a default assignee, the agent must choose one of:

- `--accept-default-assignee`
- `--assignee <actor-ref>`
- `--keep-assignee`

Example:

```sh
grind update TASK-1 \
  --project PROJ-1 \
  --keep-assignee \
  --json \
  --no-input \
  --actor codex:agent-7
```

Without one of those flags, the command fails with `ASSIGNMENT_DECISION_REQUIRED`.

## Time Tracking

### Event-derived session tracking

Use this when the agent is actively working:

```sh
grind start TASK-1 --json --no-input --actor codex:agent-7
grind pause TASK-1 --json --no-input --actor codex:agent-7
grind resume TASK-1 --json --no-input --actor codex:agent-7
```

### Manual time entries

Use this for backfill or explicitly recorded work:

```sh
grind time add TASK-1 \
  --started-at 2026-04-21T10:00:00Z \
  --duration 30m \
  --note "Investigated report rendering" \
  --json \
  --no-input \
  --actor codex:agent-7
```

```sh
grind time edit TASK-1 <entry-id> \
  --duration 45m \
  --json \
  --no-input \
  --actor codex:agent-7
```

## Links And Relationships

### Add supporting links

```sh
grind link add TASK-1 url https://example.com/spec \
  --label "Spec" \
  --json \
  --no-input \
  --actor codex:agent-7
```

```sh
grind link add TASK-1 file /tmp/build.log \
  --label "Build log" \
  --json \
  --no-input \
  --actor codex:agent-7
```

### Add task relationships

Use the relationship aliases in their intuitive direction:

```sh
grind relationship add parent TASK-1 TASK-2 --json --no-input --actor codex:agent-7
grind relationship add child TASK-1 TASK-3 --json --no-input --actor codex:agent-7
grind relationship add blocks TASK-1 TASK-4 --json --no-input --actor codex:agent-7
grind relationship add related_to TASK-1 TASK-5 --json --no-input --actor codex:agent-7
```

## Repo-Aware Helpers

Repo/worktree helpers are explicit. They help agents use current git context without causing hidden writes.

### Read tasks linked to the current repo or worktree

```sh
grind list --here --json --actor codex:agent-7
```

### Attach the current repo/worktree to a task

```sh
grind link attach-current-repo TASK-1 --json --no-input --actor codex:agent-7
```

This attaches repo/worktree metadata only when the agent asks for it.

## Export, Reporting, And Backup

### Export machine-usable data

```sh
grind export json --output tasks.json --json --actor codex:agent-7
```

### Start the local read-only report server

```sh
grind report serve --addr 127.0.0.1:8080 --json --actor codex:agent-7
```

### Full backup and restore

```sh
grind backup create --output grind-backup.sqlite --json --actor codex:agent-7
grind restore apply --input grind-backup.sqlite --force --json --actor codex:agent-7
```

## Failure Handling

Agents should branch on:

- process exit code
- `error.code`
- `error.details`

Do not branch on `error.message` unless you are producing logs for humans.

Common outcomes:

| `error.code` | What it means | Typical response |
| --- | --- | --- |
| `INVALID_ARGS` | command shape or required flags are wrong | fix invocation and retry |
| `VALIDATION_ERROR` | the request is well-formed but invalid | correct the input |
| `ENTITY_NOT_FOUND` | task, actor, project, or other record is missing | stop or re-resolve reference |
| `VIEW_NOT_FOUND` | saved view does not exist | use a different view or create it |
| `CLAIM_REQUIRED` | mutation needs an active claim | claim the task first |
| `CLAIM_CONFLICT` | another actor already holds the task | skip, retry later, or choose another task |
| `CLAIM_NOT_HELD_BY_ACTOR` | current actor is not the claim holder | do not mutate the task |
| `ASSIGNMENT_DECISION_REQUIRED` | non-interactive reclassification would change assignee | choose `--accept-default-assignee`, `--assignee`, or `--keep-assignee` |
| `FILTER_INVALID` | list/view filter values are invalid | correct the filter |
| `INVALID_RELATIONSHIP` | relationship or link type is unsupported | use a supported type |
| `DATABASE_BUSY` | SQLite is locked or busy | retry with backoff |

## Recommended Retry Policy

Safe to retry automatically:

- `DATABASE_BUSY`
- some `CLAIM_CONFLICT` scenarios, if the agent is polling for work
- transient report/export/backup failures caused by local file contention

Do not blindly retry:

- `INVALID_ARGS`
- `VALIDATION_ERROR`
- `ENTITY_NOT_FOUND`
- `ASSIGNMENT_DECISION_REQUIRED`
- `CLAIM_NOT_HELD_BY_ACTOR`

## Minimal Agent Loop

This is the smallest safe work loop for an autonomous agent:

```sh
grind list --status backlog --json --actor codex:agent-7
grind claim TASK-1 --json --no-input --actor codex:agent-7
grind update TASK-1 --status active --json --no-input --actor codex:agent-7
grind link attach-current-repo TASK-1 --json --no-input --actor codex:agent-7
grind close TASK-1 --status completed --json --no-input --actor codex:agent-7
grind release TASK-1 --json --no-input --actor codex:agent-7
```

## Practical Advice

- Prefer handles like `TASK-1` over UUIDs unless the workflow already has UUIDs.
- Log the full JSON response for failed mutations.
- Use `config show --json` at startup if the database path or actor identity matters.
- Keep one task claimed at a time unless the surrounding workflow explicitly requires more.
- Attach repo context explicitly when working inside a git checkout so later `--here` queries stay useful.
