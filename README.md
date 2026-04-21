# Grind

Local-first task management for humans and AI agents.

[GitHub Releases](https://github.com/H4ZM47/grind/releases) • [CLI Contract](docs/reference/cli-contract.md) • [Human Quickstart](docs/guides/human-cli-quickstart.md) • [Agent Integration](docs/guides/agent-integration.md)

## Why it exists

Most task tools assume either:

- a human clicking around in a UI
- or an automation scraping output that was never meant to be stable

Grind is designed for a different environment: one local machine, one human user, and many short-lived agents sharing the same source of truth. It gives both humans and agents the same interface over a local SQLite database.

## What you get

| Capability | What it means |
| --- | --- |
| Low-friction capture | `title` is the only required field for a new task |
| Agent-safe contract | `--json`, `--no-input`, and deterministic exit codes are first-class |
| Exclusive claims | only one worker mutates a task at a time |
| First-class structure | tasks, domains, projects, milestones, actors, links, and saved views |
| Event history | all meaningful changes are recorded in an append-only log |
| Read and portability paths | JSON, CSV, TXT, Markdown, local HTML reports, backup, and restore |
| Local-first storage | SQLite today, with a clean path to a future daemon or remote backend |

## Install

Release assets are published on [GitHub Releases](https://github.com/H4ZM47/grind/releases).

Each release includes:

- standalone archives for macOS, Linux, and Windows
- Linux `.deb` and `.rpm` packages
- `checksums.txt`
- a tap-ready Homebrew formula
- winget manifests

### Quick install from a release archive

1. Download the asset for your platform from the latest release.
2. Extract it.
3. Put `grind` on your `PATH`.

Example:

```sh
grind --version
grind --config
```

### Homebrew

Grind is published through the `H4ZM47/grind` tap.

Recommended install flow:

```sh
brew tap H4ZM47/grind
brew install grind
```

Direct install also works:

```sh
brew install H4ZM47/grind/grind
```

### Package-manager outputs

The repo also generates release metadata for:

- Homebrew via the `H4ZM47/grind` tap
- winget via release-ready manifests
- Debian and RPM package artifacts

Those artifacts are attached to the GitHub release as packaging inputs, not hidden in CI.

## Quick start

### Human flow

Capture a task:

```sh
grind create "Draft release notes"
```

Claim it before mutating it:

```sh
grind claim acquire TASK-1
grind update TASK-1 --status active
```

Track progress:

```sh
grind time start TASK-1
grind time pause TASK-1
grind time resume TASK-1
grind close TASK-1
```

Find work again later:

```sh
grind list --status backlog --tag docs
grind list --search release
grind view create docs-backlog --status backlog --tag docs
grind view apply docs-backlog
```

### Agent flow

Agents should always use explicit non-interactive calls:

```sh
grind list --status backlog --json --actor codex:agent-7
grind claim acquire TASK-1 --json --no-input --actor codex:agent-7
grind update TASK-1 --status active --json --no-input --actor codex:agent-7
```

That contract is documented in:

- [CLI Contract](docs/reference/cli-contract.md)
- [JSON Success Examples](docs/reference/json-success-examples.md)
- [JSON Error Examples](docs/reference/json-error-examples.md)
- [Exit Codes](docs/reference/exit-codes.md)

Built-in agent instructions are also available directly from the CLI:

```sh
grind --agents
```

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

The root command is `grind`.

Common commands:

- `grind create`, `grind list`, `grind show`, `grind update`
- `grind open`, `grind close`, `grind cancel`
- `grind claim acquire|renew|release|unlock`
- `grind time start|pause|resume`, `grind time add`, `grind time edit`
- `grind domain ...`, `grind project ...`, `grind actor ...`
- `grind milestone ...`, `grind view ...`, `grind export ...`, `grind serve`
- `grind link ...`, `grind link-repo`
- `grind backup create`, `grind restore`
- `grind --config`, `grind --version`, `grind --agents`

Global flags:

- `--json`
- `--no-input`
- `--db <path>`
- `--actor <ref>`
- `--quiet`

For the full tree, see [CLI Surface](docs/reference/cli-surface.md).

## Reporting, export, and backup

Grind gives you three different read/portability paths:

| Tool | Use it when |
| --- | --- |
| `grind export ...` | you want JSON, CSV, TXT, or Markdown output |
| `grind serve` | you want a local read-only browser view |
| `grind backup create` / `grind restore` | you want full-fidelity move or recovery |

Examples:

```sh
grind export json --output tasks.json
grind serve --addr 127.0.0.1:8080
grind backup create --output grind-backup.sqlite
```

## Local repo awareness

When you run the CLI inside a git repo or worktree, it can help without mutating task data implicitly.

- `grind list --here` filters to tasks linked to the current repo/worktree
- `grind link-repo TASK-1` explicitly records the current repo/worktree context

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
HOMEBREW_TAP_DIR=/path/to/homebrew-grind ./scripts/sync-homebrew-tap.sh
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

- [Grind Design](docs/superpowers/specs/2026-04-20-grind-design.md)
- [V1 Implementation Stack](docs/engineering/v1-implementation-stack.md)
- [Testing Guide](docs/engineering/testing.md)

## License

[MIT](LICENSE)
