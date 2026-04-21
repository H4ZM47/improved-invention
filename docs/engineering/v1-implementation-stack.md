# V1 Implementation Stack

Date: 2026-04-20
Status: Accepted for v1

## Decision

Grind v1 will be implemented in Go as a single-binary CLI application with a layered architecture:

- Go for the application runtime and build toolchain
- `cobra` for command parsing and help generation
- `database/sql` with `modernc.org/sqlite` for the local SQLite backend
- embedded SQL migrations executed by an internal migration runner
- standard-library serializers and servers for JSON, CSV, TXT, Markdown, and local HTML reporting
- standard-library `testing` as the baseline test harness

This choice is optimized for the v1 constraints in the approved design spec:

- cross-platform distribution with standalone binaries
- low operational overhead for humans and agents
- reliable local SQLite access
- simple packaging for macOS, Linux, and Windows
- a stable path from local SQLite today to a future daemon or remote backend later

## Why Go

Go is the best fit for v1 because it combines:

- easy cross-platform static-style distribution
- strong standard-library support for CLI, HTTP, templating, JSON, CSV, and testing
- a straightforward concurrency model for short transactional workflows
- low startup cost for agent-driven command execution
- a small runtime surface compared with Node.js or Python distributions

The most important practical advantage is packaging. A local-first CLI with SQLite, JSON contracts, and a read-only report server benefits from shipping as one predictable binary instead of a runtime plus dependency graph.

## Selected Components

### Language and Toolchain

- Language: Go
- Baseline policy: pin to the current stable Go release series in `go.mod`
- Module mode: single-module repository for v1

### CLI Framework

- Framework: `github.com/spf13/cobra`

Reasons:

- widely understood command/subcommand model
- good help and shell completion support
- simple composition for nested command groups
- low risk for a CLI that needs both human-friendly and machine-safe usage

### Storage

- Database API: `database/sql`
- SQLite driver: `modernc.org/sqlite`

Reasons:

- keeps the service/storage boundary close to plain SQL
- avoids CGO as a hard requirement for local development and cross-platform binary builds
- works well with embedded migrations and transactional command handlers

### Migrations

- Format: plain `.sql` migration files
- Execution: internal migration runner with embedded assets via `go:embed`

Reasons:

- schema changes stay readable and reviewable
- migrations remain usable whether the backend stays SQLite or is later replaced
- the CLI can bootstrap a fresh database without external tooling

### Reporting and Export

- JSON: `encoding/json`
- CSV: `encoding/csv`
- TXT and Markdown: standard string/buffer rendering in the application layer
- HTML report server: `net/http` plus `html/template`

Reasons:

- v1 reporting is intentionally read-only and lightweight
- the standard library is sufficient for the required output formats
- fewer dependencies make the machine-readable contract easier to keep stable

### Testing

- Baseline: standard-library `testing`
- Style: table-driven unit tests plus SQLite-backed integration tests

Reasons:

- enough power for the approved v1 scope
- no need to commit to a larger assertion or mocking framework before the core APIs exist

### Release and Packaging Direction

- Build artifacts: standalone binaries per target platform
- Release orchestration target: `goreleaser`

`goreleaser` is part of the implementation direction because it aligns well with:

- multi-platform binary production
- Homebrew publication
- archive generation for Linux and Windows
- a clean future path to package-manager metadata automation

The exact release workflow is deferred to the packaging issues, but the stack should assume a single-binary release shape from the start.

## Rejected Alternatives

### Node.js and TypeScript

Pros:

- fast CLI iteration
- rich ecosystem
- familiar developer experience

Why not chosen:

- weaker standalone distribution story for a cross-platform SQLite CLI
- more packaging complexity for agents and end users
- more moving parts for native SQLite behavior and installer paths

### Rust

Pros:

- strong binaries
- excellent performance
- explicit type system

Why not chosen:

- slower iteration for a product still defining its workflow surface
- steeper contributor overhead for what is mostly transactional CLI and storage work
- less leverage from batteries-included standard library for the HTML/reporting side

### Python

Pros:

- fast prototyping
- simple SQLite access

Why not chosen:

- weak standalone distribution story relative to Go
- higher risk of environment drift across platforms
- less attractive for a durable agent-safe CLI product

## Resulting Repository Shape

The first scaffold should assume a structure close to:

```text
cmd/grind/
internal/app/
internal/cli/
internal/config/
internal/db/
internal/migrate/
internal/report/
internal/export/
internal/task/
migrations/
testdata/
```

Guidance:

- `cmd/grind` owns process startup only
- `internal/cli` owns Cobra command wiring and output adapters
- `internal/app` owns service-layer orchestration
- `internal/db` owns SQLite access and transactions
- feature packages such as `internal/task` own domain logic behind the application boundary

## Consequences

What this decision optimizes:

- straightforward local bootstrap
- quick agent invocation
- minimal release footprint
- low-friction SQLite integration

What this decision makes less likely:

- shipping a plugin-heavy JavaScript CLI
- relying on CGO-bound SQLite as the default path
- using a thick web framework for the v1 HTML report server

## Follow-On Work Unblocked By This Decision

- repo layout and module scaffold
- root command bootstrap
- lint, test, and build entrypoints
- config and database bootstrap
- release automation planning
