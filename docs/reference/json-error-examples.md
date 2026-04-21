# JSON Error Examples

Date: 2026-04-21
Status: Initial v1 JSON error examples

## Contract Shape

Failed machine-readable responses use a stable top-level envelope:

```json
{
  "ok": false,
  "command": "task show",
  "error": {
    "code": "ENTITY_NOT_FOUND",
    "exit_code": 20,
    "message": "task TASK-1042 was not found",
    "details": {}
  }
}
```

## Envelope Rules

- `ok` is always `false` for failures.
- `command` is the canonical command path as executed.
- `error.code` is the stable symbolic failure class.
- `error.exit_code` is the matching process exit code.
- `error.message` is human-readable and may evolve, but should remain concise.
- `error.details` is structured context useful to humans and agents.

## Example: Missing Task

Command:

```text
task show TASK-404 --json
```

Exit code:

```text
20
```

Payload:

```json
{
  "ok": false,
  "command": "task show",
  "error": {
    "code": "ENTITY_NOT_FOUND",
    "exit_code": 20,
    "message": "task TASK-404 was not found",
    "details": {
      "entity": "task",
      "reference": "TASK-404"
    }
  }
}
```

## Example: Claim Conflict

Command:

```text
task claim TASK-1042 --actor codex:agent-9 --json
```

Exit code:

```text
31
```

Payload:

```json
{
  "ok": false,
  "command": "task claim",
  "error": {
    "code": "CLAIM_CONFLICT",
    "exit_code": 31,
    "message": "task TASK-1042 is already claimed",
    "details": {
      "task_handle": "TASK-1042",
      "claim_holder": {
        "provider": "codex",
        "external_id": "agent-7",
        "handle": "ACT-203"
      },
      "expires_at": "2026-04-22T03:05:00Z"
    }
  }
}
```

## Example: Assignment Decision Required

Command:

```text
task update TASK-1042 --project PROJ-18 --no-input --json
```

Exit code:

```text
44
```

Payload:

```json
{
  "ok": false,
  "command": "task update",
  "error": {
    "code": "ASSIGNMENT_DECISION_REQUIRED",
    "exit_code": 44,
    "message": "changing project requires an explicit assignee decision",
    "details": {
      "task_handle": "TASK-1042",
      "project_handle": "PROJ-18",
      "choices": [
        "--accept-default-assignee",
        "--assignee <actor-ref>",
        "--keep-assignee"
      ]
    }
  }
}
```

## Example: Invalid Status Transition

Command:

```text
task close TASK-1042 --status backlog --json
```

Exit code:

```text
40
```

Payload:

```json
{
  "ok": false,
  "command": "task close",
  "error": {
    "code": "INVALID_STATUS_TRANSITION",
    "exit_code": 40,
    "message": "cannot close task TASK-1042 into status backlog",
    "details": {
      "from": "active",
      "requested_to": "backlog",
      "allowed": [
        "completed",
        "cancelled"
      ]
    }
  }
}
```

## Example: Database Busy

Command:

```text
task update TASK-1042 --title "New title" --json
```

Exit code:

```text
81
```

Payload:

```json
{
  "ok": false,
  "command": "task update",
  "error": {
    "code": "DATABASE_BUSY",
    "exit_code": 81,
    "message": "database remained locked past the configured busy timeout",
    "details": {
      "timeout_ms": 5000
    }
  }
}
```
