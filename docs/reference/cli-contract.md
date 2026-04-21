# CLI Contract

Date: 2026-04-21
Status: Initial v1 contract reference

## Purpose

This document is the initial machine-usable contract for Task CLI v1. It complements the CLI surface and design spec by locking the behavior that automation and agents should rely on from the command line.

See also:

- [CLI Surface](cli-surface.md)
- [JSON Success Examples](json-success-examples.md)
- [JSON Error Examples](json-error-examples.md)
- [Exit Codes](exit-codes.md)

## Contract Guarantees

- Every state-changing command must have a fully non-interactive path.
- `--json` output is a stable interface, not a best-effort debug mode.
- Exit codes are deterministic and granular.
- Human-readable text and machine-readable JSON must describe the same outcome.
- Commands should accept human-friendly handles first and UUIDs second.

## Global Behavior

### Non-Interactive Mode

`--no-input` means:

- never prompt
- fail with an explicit error if the command requires a human decision
- do not mutate additional fields implicitly to “help”

### JSON Mode

`--json` means:

- emit one JSON object to stdout
- do not mix progress chatter into stdout
- send any non-contract diagnostics to stderr
- keep top-level envelope shape stable across releases

### Stable References

Existing records should be addressable by:

1. handle, such as `TASK-1042`
2. UUID

## Success Envelope

Successful responses use:

```json
{
  "ok": true,
  "command": "task create",
  "data": {},
  "meta": {}
}
```

Field rules:

- `ok`: always `true`
- `command`: canonical executed command path
- `data`: primary response content
- `meta`: secondary context such as filters, count, or output format

## Error Envelope

Failed responses use:

```json
{
  "ok": false,
  "command": "task update",
  "error": {
    "code": "CLAIM_REQUIRED",
    "exit_code": 30,
    "message": "task TASK-1042 must be claimed before update",
    "details": {}
  }
}
```

Field rules:

- `ok`: always `false`
- `error.code`: stable symbolic error class
- `error.exit_code`: exact process exit code
- `error.message`: concise human-readable explanation
- `error.details`: structured context for automation

## Representative Commands

### Create

```text
task create "Write CLI contract" --json
```

### Read

```text
task show TASK-1042 --json
task list --status active --tag cli --json
```

### Mutate

```text
task claim TASK-1042 --actor codex:agent-7 --json
task update TASK-1042 --title "Revised title" --no-input --json
```

### Configuration And Introspection

```text
task config show --json
task version --json
```

## Compatibility Policy

- Adding new fields inside `data` or `meta` is allowed when existing fields keep the same meaning.
- Renaming or deleting top-level envelope fields is not allowed in v1.
- Reusing an existing exit code for a different failure class is not allowed.
- Human-readable text may improve, but JSON structure and symbolic error codes are the durability target.

## Immediate Follow-On Implementation

The next implementation steps after publishing this contract are:

- wire `--json` output helpers into command handlers
- map command failures onto the published exit-code table
- add contract tests that lock JSON envelopes and exit codes in CI
