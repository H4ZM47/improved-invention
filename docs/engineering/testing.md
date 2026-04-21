# Testing

Grind uses three test layers:

- unit-style package tests for business rules and command behavior
- SQLite-backed integration tests that open a real migrated database
- contract-style snapshot tests for machine-readable CLI output

## Conventions

- prefer `t.Parallel()` for package tests unless they mutate shared process state
- use [`internal/testutil/sqlite.go`](/Users/alex/task/internal/testutil/sqlite.go) to open real migrated SQLite databases for tests
- keep helper setup close to the package under test unless it is reused across packages
- assert the machine-readable payload shape directly for CLI JSON responses
- add regression tests for every bug fixed in claim lifecycle, restore, filtering, or CLI contract paths

## Local Checks

The standard local verification path is:

```bash
make test
make build
make lint
```

These commands are the same ones CI runs.

## SQLite Integration Tests

Integration-style tests should:

- call the real database bootstrap path through `taskdb.Open`
- exercise migrations, pragmas, transactions, and persistence behavior
- use temp directories and isolated database paths
- avoid depending on ordering across tests

The current shared helper returns a real SQLite database with migrations applied:

```go
db := testutil.OpenSQLiteDB(t)
```

Use `OpenSQLiteDBAtPath` when a test needs to reopen the same database file across multiple phases, such as backup/restore or claim-expiry-on-open scenarios.

## Contract Tests

When a command exposes stable JSON for agents, add a contract test that verifies:

- the command name
- the top-level `ok`, `data`, and `meta` envelope shape
- any documented filter or summary metadata
- deterministic field names and success semantics

Exit-code golden coverage remains a separate hardening lane and should be added alongside the final process exit-code mapping.
