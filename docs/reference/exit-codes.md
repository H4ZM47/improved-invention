# Exit Codes

Date: 2026-04-21
Status: Initial v1 exit-code map

## Purpose

Task CLI v1 uses explicit, granular exit codes so humans, scripts, and agents can distinguish validation failures, lock conflicts, missing records, storage faults, and internal errors without scraping stderr text.

The exit-code map is part of the CLI contract. Human-readable messages may improve over time, but the semantic meaning of these codes should remain stable once released.

## Rules

- `0` is the only success exit code.
- Non-zero exit codes map to one specific failure class.
- A single failure should not return a generic “something went wrong” code if a more specific class exists.
- JSON error output must echo the same numeric exit code and symbolic error code.

## Exit-Code Table

| Exit code | Symbolic code | Meaning |
| --- | --- | --- |
| `0` | `OK` | Command completed successfully. |
| `10` | `INVALID_ARGS` | Command-line arguments or flags are malformed. |
| `11` | `VALIDATION_ERROR` | Input was structurally valid CLI syntax but failed business validation. |
| `12` | `OUTPUT_MODE_UNSUPPORTED` | Requested output mode is not supported for the command. |
| `13` | `INTERACTIVE_INPUT_REQUIRED` | Command needs a human decision and `--no-input` prevented prompting. |
| `20` | `ENTITY_NOT_FOUND` | Requested task, project, domain, actor, or view could not be found. |
| `21` | `REFERENCE_AMBIGUOUS` | The provided handle or lookup matched more than one record. |
| `22` | `NO_RESULTS` | A read/query command completed successfully but returned no matching rows in a mode that treats emptiness as exceptional. |
| `30` | `CLAIM_REQUIRED` | A mutating task operation requires an active claim and none was present. |
| `31` | `CLAIM_CONFLICT` | Another actor already holds the active claim. |
| `32` | `CLAIM_EXPIRED` | The referenced claim is no longer valid because its lease expired. |
| `33` | `CLAIM_NOT_HELD_BY_ACTOR` | The caller tried to renew or release a claim it does not hold. |
| `34` | `CLAIM_RENEWAL_NOT_ALLOWED` | Renewal was requested for a claim that cannot be renewed in its current state. |
| `35` | `UNLOCK_NOT_ALLOWED` | Manual unlock was attempted without satisfying the recovery policy. |
| `40` | `INVALID_STATUS_TRANSITION` | Requested lifecycle transition is not allowed. |
| `41` | `INVALID_RELATIONSHIP` | Relationship type or endpoints violate v1 rules. |
| `42` | `HIERARCHY_CONFLICT` | Parent-child or sibling linkage would violate one-parent hierarchy rules. |
| `43` | `DUPLICATE_LINK` | Relationship or external link already exists. |
| `44` | `ASSIGNMENT_DECISION_REQUIRED` | Reclassification would trigger default-assignee behavior and the caller did not choose how to handle it. |
| `45` | `DOMAIN_PROJECT_CONSTRAINT` | Domain/project containment rules were violated. |
| `50` | `ACTOR_NOT_FOUND` | Referenced actor does not exist. |
| `51` | `ACTOR_IDENTITY_INVALID` | Actor identity format is invalid, such as a malformed agent provider ID. |
| `60` | `VIEW_NOT_FOUND` | Requested saved view does not exist. |
| `61` | `FILTER_INVALID` | Filter syntax or filter fields are invalid for the command. |
| `70` | `EXPORT_FAILED` | Export rendering or output writing failed. |
| `71` | `REPORT_SERVER_FAILED` | The local HTML report server could not start or serve correctly. |
| `72` | `BACKUP_FAILED` | Backup creation failed. |
| `73` | `RESTORE_FAILED` | Restore failed or the artifact was invalid. |
| `80` | `DATABASE_UNAVAILABLE` | Database path, permissions, or initialization prevented opening the DB. |
| `81` | `DATABASE_BUSY` | SQLite could not obtain the necessary lock within the configured timeout. |
| `82` | `MIGRATION_FAILED` | Schema migration failed. |
| `90` | `INTERNAL_ERROR` | Unexpected internal failure with no more specific mapped class. |

## Guidance

### Read Commands

Typical read commands should use:

- `0` for successful reads with results
- `22` only when the command mode explicitly treats empty result sets as exceptional
- `20` when a direct lookup target such as `TASK-1042` does not exist

### Mutating Commands

Mutating commands should prefer domain-specific failure classes:

- claim/lock failures in the `30` range
- lifecycle and structure failures in the `40` range
- actor and view lookup failures in the `50` and `60` ranges

### Storage And Runtime Failures

Storage failures should not be collapsed into `90` when the CLI can distinguish them:

- DB open/bootstrap errors -> `80`
- busy timeout / lock starvation -> `81`
- migration failures -> `82`

## Example JSON Pairing

If a command exits with code `31`, the JSON error payload should contain:

```json
{
  "ok": false,
  "error": {
    "code": "CLAIM_CONFLICT",
    "exit_code": 31,
    "message": "task TASK-1042 is already claimed by codex:agent-7"
  }
}
```
