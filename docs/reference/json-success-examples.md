# JSON Success Examples

Date: 2026-04-21
Status: Initial v1 JSON success examples

## Contract Shape

Successful machine-readable responses use a stable top-level envelope:

```json
{
  "ok": true,
  "command": "grind create",
  "data": {},
  "meta": {}
}
```

## Envelope Rules

- `ok` is always `true` for successful responses.
- `command` is the canonical command path as executed.
- `data` contains the command-specific result payload.
- `meta` contains non-primary metadata such as pagination, filters, or export format.
- Success payloads should avoid unstable fields such as random IDs or transient timestamps unless the command itself is creating or returning them as business data.

## Example: `grind create --json`

```json
{
  "ok": true,
  "command": "grind create",
  "data": {
    "task": {
      "uuid": "9e04c26b-fb8a-4f96-a3f5-6544d07d7757",
      "handle": "TASK-1042",
      "title": "Write initial CLI contract reference",
      "description": "",
      "status": "backlog",
      "domain_id": null,
      "project_id": null,
      "assignee_actor_id": null,
      "due_at": null,
      "tags": [],
      "created_at": "2026-04-21T03:00:00Z",
      "updated_at": "2026-04-21T03:00:00Z",
      "closed_at": null
    }
  },
  "meta": {
    "claimed": false
  }
}
```

## Example: `grind show TASK-1042 --json`

```json
{
  "ok": true,
  "command": "grind show",
  "data": {
    "task": {
      "uuid": "9e04c26b-fb8a-4f96-a3f5-6544d07d7757",
      "handle": "TASK-1042",
      "title": "Write initial CLI contract reference",
      "status": "active",
      "description": "Capture JSON envelopes and exit-code mapping.",
      "domain_id": null,
      "project_id": null,
      "assignee_actor_id": "3d67c947-0d8d-4d88-a2b1-1243ed1e8b4c",
      "due_at": null,
      "tags": [
        "cli",
        "contracts"
      ],
      "created_at": "2026-04-21T03:00:00Z",
      "updated_at": "2026-04-21T03:12:00Z",
      "closed_at": null
    },
    "claim": {
      "actor": {
        "kind": "agent",
        "provider": "codex",
        "external_id": "agent-7",
        "handle": "ACT-203"
      },
      "claimed_at": "2026-04-21T03:05:00Z",
      "expires_at": "2026-04-22T03:05:00Z"
    }
  },
  "meta": {}
}
```

## Example: `grind list --json`

```json
{
  "ok": true,
  "command": "grind list",
  "data": {
    "items": [
      {
        "handle": "TASK-1042",
        "title": "Write initial CLI contract reference",
        "status": "active",
        "assignee_actor_id": "3d67c947-0d8d-4d88-a2b1-1243ed1e8b4c",
        "updated_at": "2026-04-21T03:12:00Z"
      },
      {
        "handle": "TASK-1043",
        "title": "Document granular exit codes",
        "status": "backlog",
        "assignee_actor_id": null,
        "updated_at": "2026-04-21T03:09:00Z"
      }
    ]
  },
  "meta": {
    "count": 2,
    "filters": {
      "status": [
        "backlog",
        "active"
      ],
      "tags": [
        "cli"
      ]
    }
  }
}
```

## Example: `view show inbox --json`

```json
{
  "ok": true,
  "command": "view show",
  "data": {
    "view": {
      "name": "inbox",
      "entity": "task",
      "filters": {
        "status": [
          "backlog"
        ]
      }
    }
  },
  "meta": {}
}
```

## Example: `grind --version --json`

```json
{
  "ok": true,
  "command": "grind --version",
  "data": {
    "version": "dev",
    "commit": "unknown",
    "date": "unknown"
  },
  "meta": {}
}
```
