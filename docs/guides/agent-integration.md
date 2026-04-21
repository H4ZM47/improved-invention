# Agent Integration

Task CLI v1 is designed so agents can use the same product surface as humans without scraping text output or relying on prompts.

## Contract Rules

Agents should treat these behaviors as mandatory:

- pass `--json` for machine-readable output
- pass `--no-input` for state-changing commands
- pass an explicit `--actor` identity
- treat exit codes as deterministic
- consume the JSON envelope rather than parsing human text

Reference docs:

- [CLI Contract](../reference/cli-contract.md)
- [Exit Codes](../reference/exit-codes.md)
- [JSON Success Examples](../reference/json-success-examples.md)
- [JSON Error Examples](../reference/json-error-examples.md)

## Actor Identity

Use a structured self-reported agent reference such as:

- `codex:agent-7`
- `claude:batch-3`

Example:

```sh
task list --json --actor codex:agent-7
```

The actor record is created implicitly on first claim or mutation if it does not already exist.

## Recommended Global Flags

For non-interactive automation, use this baseline shape:

```sh
task <command> ... --actor codex:agent-7 --no-input --json
```

Use `--db` when the process must target a specific workspace-local or test database.

## Common Agent Flows

### Discover Candidate Work

```sh
task list --status backlog --tag docs --json --actor codex:agent-7
task view apply ready-docs --json --actor codex:agent-7
```

### Claim Before Mutation

```sh
task claim TASK-1 --actor codex:agent-7 --no-input --json
task update TASK-1 --status active --actor codex:agent-7 --no-input --json
```

If another worker already holds the task, expect a lock-conflict style error instead of a silent overwrite.

### Add Work Evidence

```sh
task time add TASK-1 \
  --started-at 2026-04-21T10:00:00Z \
  --duration 30m \
  --note "Investigated report rendering" \
  --actor codex:agent-7 \
  --no-input \
  --json
```

```sh
task link add TASK-1 repo https://github.com/H4ZM47/task-cli \
  --actor codex:agent-7 \
  --no-input \
  --json
```

### Finish Or Hand Off

```sh
task close TASK-1 --status completed --actor codex:agent-7 --no-input --json
task release TASK-1 --actor codex:agent-7 --no-input --json
```

## Non-Interactive Reclassification

Task CLI intentionally avoids implicit data mutation in `--no-input` mode.

If an update would reclassify a task into a domain or project with a default assignee, the agent must choose one of:

- `--accept-default-assignee`
- `--assignee <actor-ref>`
- `--keep-assignee`

Example:

```sh
task update TASK-1 \
  --project task-cli \
  --keep-assignee \
  --actor codex:agent-7 \
  --no-input \
  --json
```

Without an explicit choice, the command fails instead of silently changing `assignee`.

## Error Handling

Agents should branch on:

- process exit code
- `error.code`
- structured `error.details`

Do not rely on matching `error.message` strings.

Typical outcomes to handle explicitly:

- missing task or view
- claim required
- claim conflict
- invalid lifecycle transition
- validation failure
- assignment decision required

## Repo-Aware Helpers

Repo/worktree support is helper-only, not implicit mutation.

- use `task list --here` to read tasks linked to the current repo or worktree
- use `task link attach-current-repo TASK-1` to attach the current repo context explicitly

This keeps local context useful without surprising background writes.
