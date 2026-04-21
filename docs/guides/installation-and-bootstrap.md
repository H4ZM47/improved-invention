# Installation and Bootstrap

This guide covers the supported ways to run Task CLI today and the minimum bootstrap needed to start using a local database safely.

## Current Installation Paths

Package-manager distribution is still being finished. Today, the supported paths are:

1. build from source
2. run a release archive produced by CI

### Build From Source

Requirements:

- Go 1.26 or newer
- a writable local data directory for the SQLite database

Build the local binary:

```sh
make build
./dist/task version
```

Or run the CLI directly from source during development:

```sh
go run ./cmd/task version
```

### Use A Release Archive

Release archives are built by `scripts/build-release.sh` and bundled as standalone binaries for Linux, macOS, and Windows.

Local example:

```sh
GOOS=darwin GOARCH=arm64 VERSION=dev COMMIT=$(git rev-parse --short HEAD) DATE=$(date -u +%FT%TZ) \
  ./scripts/build-release.sh
```

That produces an archive under `dist/releases/` containing:

- the `task` binary
- `LICENSE`
- `README.md`

## Runtime Configuration

Task CLI resolves configuration in this order:

1. CLI flags
2. environment variables
3. OS defaults

Inspect the resolved runtime configuration:

```sh
task config show --json
```

Supported environment variables:

- `TASK_DATA_DIR`: override the app data directory
- `TASK_DB_PATH`: override the SQLite database path directly
- `TASK_ACTOR`: set the acting human or agent reference
- `TASK_HUMAN_NAME`: set the configured local human name
- `TASK_BUSY_TIMEOUT_MS`: override SQLite busy timeout
- `TASK_CLAIM_LEASE_HOURS`: override the default claim lease duration

Default locations:

- macOS: `~/Library/Application Support/task/task.db`
- Linux: `$XDG_CONFIG_HOME/task/task.db` or `~/.config/task/task.db`
- Windows: the OS user config directory plus `task\\task.db`

## Bootstrap A Fresh Database

The database and schema are created automatically on first use. A minimal bootstrap flow is:

```sh
task config show
task create "First task"
task list
```

You can also bootstrap in a project-local sandbox by pointing the CLI at an explicit database path:

```sh
task create "Local sandbox task" --db ./.task-dev.db
```

## Identity Defaults

Task CLI distinguishes the configured local human from ephemeral agent actors.

- human-oriented commands usually rely on the resolved `TASK_HUMAN_NAME`
- agents should pass an explicit `--actor` such as `codex:agent-7`
- agent actors are created implicitly the first time they claim or mutate work

Example:

```sh
task claim TASK-1 --actor codex:agent-7 --json
```

## First Sanity Checks

These commands confirm that the installation is usable:

```sh
task version --json
task config show --json
task create "Installation smoke test" --json
task list --json
```

If those pass, the local binary, DB path, migrations, and JSON contract are all working.
