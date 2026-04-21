# Human CLI Quickstart

This guide shows the shortest path from a blank database to a normal human workflow.

## 1. Capture A Task

Only the title is required:

```sh
grind create "Draft v1 launch notes"
```

Optional fields can be set during capture:

```sh
grind create "Draft v1 launch notes" \
  --description "Use standard Markdown for the outline." \
  --tags docs,launch \
  --due-at 2026-04-25T17:00:00Z
```

New tasks start in `backlog` and are unclaimed.

## 2. Find Work

List all tasks:

```sh
grind list
```

Filter by common fields:

```sh
grind list --status backlog --tag docs
grind list --assignee alex --project grind
grind list --search launch
```

If the current shell is inside a linked git repo or worktree, you can filter to local context:

```sh
grind list --here
```

## 3. Claim Before Mutating

Task updates are claim-gated. Claim the task before changing it:

```sh
grind claim acquire TASK-1
grind update TASK-1 --status active
```

Release it when you are done:

```sh
grind claim release TASK-1
```

If you need to keep a long-running claim alive:

```sh
grind claim renew TASK-1
```

## 4. Track Progress

Start, pause, and resume active work:

```sh
grind time start TASK-1
grind time pause TASK-1
grind time resume TASK-1
```

Add manual time when you need to correct or supplement the session history:

```sh
grind time add TASK-1 \
  --started-at 2026-04-21T10:00:00Z \
  --duration 45m \
  --note "Drafted launch summary"
```

Edit a manual entry by entry ID:

```sh
grind time edit TASK-1 ENTRY-ID --duration 1h --note "Expanded scope"
```

## 5. Organize Tasks

Create domains and projects:

```sh
grind domain create "work"
grind project create "grind" --domain work
```

Classify an existing task:

```sh
grind update TASK-1 --domain work --project grind
```

If reclassification would inherit a default assignee, interactive mode prompts. In non-interactive mode you must choose explicitly.

## 6. Add Context

Attach external links:

```sh
grind link add TASK-1 url https://example.com/spec
grind link-repo TASK-1
```

Create task relationships:

```sh
grind link add TASK-1 blocks TASK-2
grind link list TASK-1
```

## 7. Save Useful Views

Saved views let you reuse common filters:

```sh
grind view create docs-backlog --status backlog --tag docs
grind view list
grind view apply docs-backlog
```

You can later inspect or update the stored filters:

```sh
grind view show docs-backlog
grind view update docs-backlog --status active
```

## 8. Close Work

Mark work complete or cancelled:

```sh
grind close TASK-1
grind cancel TASK-2
```
`grind close` also closes any active or paused time session on that task.

## 9. Switch To JSON When Needed

Human-readable output is the default, but every core workflow also supports machine-readable output:

```sh
grind show TASK-1 --json
grind list --status active --json
```

That makes it easy to move between ad hoc terminal usage and automation without learning a second interface.
