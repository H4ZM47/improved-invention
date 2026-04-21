# Human CLI Quickstart

This guide shows the shortest path from a blank database to a normal human workflow.

## 1. Capture A Task

Only the title is required:

```sh
task create "Draft v1 launch notes"
```

Optional fields can be set during capture:

```sh
task create "Draft v1 launch notes" \
  --description "Use standard Markdown for the outline." \
  --tags docs,launch \
  --due-at 2026-04-25T17:00:00Z
```

New tasks start in `backlog` and are unclaimed.

## 2. Find Work

List all tasks:

```sh
task list
```

Filter by common fields:

```sh
task list --status backlog --tag docs
task list --assignee alex --project task-cli
task list --search launch
```

If the current shell is inside a linked git repo or worktree, you can filter to local context:

```sh
task list --here
```

## 3. Claim Before Mutating

Task updates are claim-gated. Claim the task before changing it:

```sh
task claim TASK-1
task update TASK-1 --status active
```

Release it when you are done:

```sh
task release TASK-1
```

If you need to keep a long-running claim alive:

```sh
task renew TASK-1
```

## 4. Track Progress

Start, pause, and resume active work:

```sh
task start TASK-1
task pause TASK-1
task resume TASK-1
```

Add manual time when you need to correct or supplement the session history:

```sh
task time add TASK-1 \
  --started-at 2026-04-21T10:00:00Z \
  --duration 45m \
  --note "Drafted launch summary"
```

Edit a manual entry by entry ID:

```sh
task time edit TASK-1 ENTRY-ID --duration 1h --note "Expanded scope"
```

## 5. Organize Tasks

Create domains and projects:

```sh
task domain create "work"
task project create "task-cli" --domain work
```

Classify an existing task:

```sh
task update TASK-1 --domain work --project task-cli
```

If reclassification would inherit a default assignee, interactive mode prompts. In non-interactive mode you must choose explicitly.

## 6. Add Context

Attach external links:

```sh
task link add TASK-1 spec https://example.com/spec
task link attach-current-repo TASK-1
```

Create task relationships:

```sh
task relationship add blocks TASK-1 TASK-2
task relationship list TASK-1
```

## 7. Save Useful Views

Saved views let you reuse common filters:

```sh
task view create docs-backlog --status backlog --tag docs
task view list
task view apply docs-backlog
```

You can later inspect or update the stored filters:

```sh
task view show docs-backlog
task view update docs-backlog --status active
```

## 8. Close Work

Mark work complete or cancelled:

```sh
task close TASK-1 --status completed
task close TASK-2 --status cancelled
```

`task close` also closes any active or paused time session on that task.

## 9. Switch To JSON When Needed

Human-readable output is the default, but every core workflow also supports machine-readable output:

```sh
task show TASK-1 --json
task list --status active --json
```

That makes it easy to move between ad hoc terminal usage and automation without learning a second interface.
