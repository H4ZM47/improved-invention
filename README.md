# Task CLI

Local-first task management for humans and AI agents.

[GitHub Releases](https://github.com/H4ZM47/task-cli/releases) • [CLI Contract](docs/reference/cli-contract.md) • [Human Quickstart](docs/guides/human-cli-quickstart.md) • [Agent Integration](docs/guides/agent-integration.md)

## Why it exists

Most task tools assume either:

- a human clicking around in a UI
- or an automation scraping output that was never meant to be stable

Task CLI is designed for a different environment: one local machine, one human user, and many short-lived agents sharing the same source of truth. It gives both humans and agents the same interface over a local SQLite database.

## What you get

| Capability | What it means |
| --- | --- |
| Low-friction capture | `title` is the only required field for a new task |
| Agent-safe contract | `--json`, `--no-input`, and deterministic exit codes are first-class |
| Exclusive claims | only one worker mutates a task at a time |
| First-class structure | tasks, domains, projects, actors, links, relationships, and saved views |
| Event history | all meaningful changes are recorded in an append-only log |
| Read and portability paths | JSON, CSV, TXT, Markdown, local HTML reports, backup, and restore |
| Local-first storage | SQLite today, with a clean path to a future daemon or remote backend |

## Install

Release assets are published on [GitHub Releases](https://github.com/H4ZM47/task-cli/releases).

Each release includes:

- standalone archives for macOS, Linux, and Windows
- Linux `.deb` and `.rpm` packages
- `checksums.txt`
- a tap-ready Homebrew formula
- winget manifests

### Quick install from a release archive

1. Download the asset for your platform from the latest release.
2. Extract it.
3. Put `task` on your `PATH`.

Example:

```sh
task version
task config show
```

### Package-manager outputs

The repo also generates release metadata for:

- Homebrew via a tap-ready formula
- winget via release-ready manifests
- Debian and RPM package artifacts

Those artifacts are attached to the GitHub release as packaging inputs, not hidden in CI.

## Quick start

### Human flow

Capture a task:

```sh
task create "Draft release notes"
```

Claim it before mutating it:

```sh
task claim TASK-1
task update TASK-1 --status active
```

Track progress:

```sh
task start TASK-1
task pause TASK-1
task resume TASK-1
task close TASK-1 --status completed
```

Find work again later:

```sh
task list --status backlog --tag docs
task list --search release
task view create docs-backlog --status backlog --tag docs
task view apply docs-backlog
```

### Agent flow

Agents should always use explicit non-interactive calls:

```sh
task list --status backlog --json --actor codex:agent-7
task claim TASK-1 --json --no-input --actor codex:agent-7
task update TASK-1 --status active --json --no-input --actor codex:agent-7
```

That contract is documented in:

- [CLI Contract](docs/reference/cli-contract.md)
- [JSON Success Examples](docs/reference/json-success-examples.md)
- [JSON Error Examples](docs/reference/json-error-examples.md)
- [Exit Codes](docs/reference/exit-codes.md)

## Core concepts

### Tasks

Tasks are the primary work entity.

- `title` is required
- new tasks start in `backlog`
- normal state-changing updates require an active claim

### Claims

Claims are exclusive locks.

- one task, one active claim
- claims expire automatically after the lease duration
- renewals are explicit
- manual unlock exists for operator intervention

### Domains and projects

Domains and projects are first-class records, not just tags.

- each project belongs to exactly one domain
- tasks may remain unclassified
- default assignees can be inherited when classification is set explicitly

### Actors

Actors represent humans and agents.

- humans are configured locally
- agents are created implicitly from structured self-reported IDs like `codex:agent-7`

## Commands at a glance

The root command is `task`.

Common commands:

- `task create`, `task list`, `task show`, `task update`
- `task claim`, `task renew`, `task release`, `task unlock`
- `task start`, `task pause`, `task resume`, `task time add`, `task time edit`, `task close`
- `task domain ...`, `task project ...`, `task actor ...`
- `task relationship ...`, `task link ...`
- `task view ...`, `task export ...`, `task report serve`
- `task backup create`, `task restore apply`
- `task config show`, `task version`

Global flags:

- `--json`
- `--no-input`
- `--db <path>`
- `--actor <ref>`
- `--quiet`

For the full tree, see [CLI Surface](docs/reference/cli-surface.md).

## Reporting, export, and backup

Task CLI gives you three different read/portability paths:

| Tool | Use it when |
| --- | --- |
| `task export ...` | you want JSON, CSV, TXT, or Markdown output |
| `task report serve` | you want a local read-only browser view |
| `task backup create` / `task restore apply` | you want full-fidelity move or recovery |

Examples:

```sh
task export json --output tasks.json
task report serve --addr 127.0.0.1:8080
task backup create --output task-backup.sqlite
```

## Local repo awareness

When you run the CLI inside a git repo or worktree, it can help without mutating task data implicitly.

- `task list --here` filters to tasks linked to the current repo/worktree
- `task link attach-current-repo TASK-1` explicitly records the current repo/worktree context

That keeps repo-aware workflows useful for humans and agents without hidden writes.

## Development

Requirements:

- Go 1.26+

Main commands:

```sh
make fmt
make lint
make test
make build
```

### Release artifact generation

Build all release artifacts locally:

```sh
./scripts/build-packaging-artifacts.sh
```

That script:

- builds release archives into `dist/releases/`
- writes `dist/releases/checksums.txt`
- builds Linux `.deb` and `.rpm` packages into `dist/packages/`
- generates a tap-ready Homebrew formula into `dist/metadata/homebrew-tap/`
- generates winget manifests into `dist/metadata/winget/`

To sync the generated Homebrew formula into a tap checkout:

```sh
HOMEBREW_TAP_DIR=/path/to/homebrew-task-cli ./scripts/sync-homebrew-tap.sh
```

## Documentation

### Getting started

- [Installation and Bootstrap](docs/guides/installation-and-bootstrap.md)
- [Human CLI Quickstart](docs/guides/human-cli-quickstart.md)
- [Agent Integration](docs/guides/agent-integration.md)
- [Claim and Operator Procedures](docs/guides/claim-and-operator-procedures.md)
- [Backup, Export, and Report Usage](docs/guides/backup-export-report.md)

### Reference

- [CLI Surface](docs/reference/cli-surface.md)
- [CLI Contract](docs/reference/cli-contract.md)
- [JSON Success Examples](docs/reference/json-success-examples.md)
- [JSON Error Examples](docs/reference/json-error-examples.md)
- [Exit Codes](docs/reference/exit-codes.md)

### Design and implementation

- [Task CLI Design](docs/superpowers/specs/2026-04-20-task-cli-design.md)
- [V1 Implementation Stack](docs/engineering/v1-implementation-stack.md)
- [Testing Guide](docs/engineering/testing.md)

## License

[MIT](LICENSE)
