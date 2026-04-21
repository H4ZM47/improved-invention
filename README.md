# improved-invention

Local-first task management CLI for humans and AI agents, sharing a single SQLite database as the v1 source of truth.

GitHub: [H4ZM47/improved-invention](https://github.com/H4ZM47/improved-invention)

## Status

Core v1 workflows are in place on `main`: task CRUD, claims, domains/projects, links, relationships, time tracking, saved views, exports, backup/restore, and the read-only report server. Remaining release-hardening work is focused on package-manager distribution.

## What it is

Task CLI (`task`) is a cross-platform, CLI-first utility for capturing, reviewing, and managing project work on a single machine shared by one human and many ephemeral AI agents.

Key properties:

- Low-friction capture (title is the only required field).
- Deterministic non-interactive automation via `--no-input` and `--json`.
- Exclusive claim locking so only one worker mutates a task at a time.
- First-class tasks, projects, domains, actors, and an append-only event log.
- Built-in JSON, CSV, TXT, and Markdown export, plus a read-only local HTML report server.
- A stable path from local SQLite today to a future daemon or remote backend.

## Architecture

- Language: Go (module `github.com/H4ZM47/improved-invention`)
- CLI framework: [`spf13/cobra`](https://github.com/spf13/cobra)
- Storage: `database/sql` with [`modernc.org/sqlite`](https://pkg.go.dev/modernc.org/sqlite) (pure-Go, no CGO)
- Migrations: plain `.sql` files under [migrations/](migrations/), embedded via `go:embed` and executed by `internal/migrate`
- Reporting/export: standard library (`encoding/json`, `encoding/csv`, `net/http`, `html/template`)

Layout:

```text
cmd/task/         process startup (pending)
internal/cli/     Cobra command wiring and output adapters
internal/app/     service-layer orchestration
internal/db/      SQLite access and transactions
internal/config/  runtime configuration resolution
internal/migrate/ embedded SQL migration runner
internal/export/  export renderers
internal/report/  read-only HTML report server
migrations/       embedded SQL migrations
testdata/         fixtures
```

## Command surface

The root command `task` hosts the primary task workflow directly. Additional groups cover adjacent concerns:

- `task create | list | show | update | claim | renew | release | unlock | start | pause | resume | close`
- `task relationship ...`, `task link ...`
- `task project ...`, `task domain ...`, `task actor ...`
- `task view | export | report | backup | restore`
- `task config show`, `task version`

Global flags: `--json`, `--no-input`, `--db <path>`, `--actor <ref>`, `--quiet`.

See [CLI Surface](docs/reference/cli-surface.md) for the full tree and [CLI Contract](docs/reference/cli-contract.md) for the JSON envelope and compatibility guarantees.

## Development

Requires Go 1.26+.

```sh
make fmt    # gofmt / goimports via scripts/fmt.sh
make lint   # golangci-lint via scripts/lint.sh
make test   # go test ./... via scripts/test.sh
make build  # build dist/task via scripts/build.sh
```

Packaging and release metadata:

```sh
./scripts/build-packaging-artifacts.sh
```

That single script:

- builds the portable release archives into `dist/releases/`
- writes `dist/releases/checksums.txt`
- builds Linux `.deb` and `.rpm` artifacts into `dist/packages/`
- generates a tap-ready Homebrew formula into `dist/metadata/homebrew-tap/Formula/task-cli.rb`
- generates winget manifests into `dist/metadata/winget/`

To copy the generated formula into a Homebrew tap checkout:

```sh
HOMEBREW_TAP_DIR=/path/to/homebrew-task-cli ./scripts/sync-homebrew-tap.sh
```

## Design and reference docs

Planning:

- [Task CLI Design](docs/superpowers/specs/2026-04-20-task-cli-design.md)
- [V1 Implementation Stack](docs/engineering/v1-implementation-stack.md)

Reference:

- [CLI Surface](docs/reference/cli-surface.md)
- [CLI Contract](docs/reference/cli-contract.md)
- [JSON Success Examples](docs/reference/json-success-examples.md)
- [JSON Error Examples](docs/reference/json-error-examples.md)
- [Exit Codes](docs/reference/exit-codes.md)

Guides:

- [Installation and Bootstrap](docs/guides/installation-and-bootstrap.md)
- [Human CLI Quickstart](docs/guides/human-cli-quickstart.md)
- [Agent Integration](docs/guides/agent-integration.md)
- [Claim and Operator Procedures](docs/guides/claim-and-operator-procedures.md)
- [Backup, Export, and Report Usage](docs/guides/backup-export-report.md)

## License

See [LICENSE](LICENSE).
