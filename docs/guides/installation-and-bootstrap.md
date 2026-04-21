# Installation and Bootstrap

This guide covers the supported ways to run Grind today and the minimum bootstrap needed to start using a local database safely.

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
./dist/grind version
```

Or run the CLI directly from source during development:

```sh
go run ./cmd/grind version
```

### Use A Release Archive

Release archives are built by `scripts/build-release.sh` and bundled as standalone binaries for Linux, macOS, and Windows.

Local example:

```sh
GOOS=darwin GOARCH=arm64 VERSION=dev COMMIT=$(git rev-parse --short HEAD) DATE=$(date -u +%FT%TZ) \
  ./scripts/build-release.sh
```

That produces an archive under `dist/releases/` containing:

- the `grind` binary
- `LICENSE`
- `README.md`

## Runtime Configuration

Grind resolves configuration in this order:

1. CLI flags
2. environment variables
3. OS defaults

Inspect the resolved runtime configuration:

```sh
grind config show --json
```

Supported environment variables:

- `GRIND_DATA_DIR`: override the app data directory
- `GRIND_DB_PATH`: override the SQLite database path directly
- `GRIND_ACTOR`: set the acting human or agent reference
- `GRIND_HUMAN_NAME`: set the configured local human name
- `GRIND_BUSY_TIMEOUT_MS`: override SQLite busy timeout
- `GRIND_CLAIM_LEASE_HOURS`: override the default claim lease duration
- legacy `TASK_*` environment variables are still accepted during the rename transition

Default locations:

- macOS: `~/Library/Application Support/grind/grind.db`
- Linux: `$XDG_CONFIG_HOME/grind/grind.db` or `~/.config/grind/grind.db`
- Windows: the OS user config directory plus `grind\\grind.db`

## Bootstrap A Fresh Database

The database and schema are created automatically on first use. A minimal bootstrap flow is:

```sh
grind config show
grind create "First task"
grind list
```

You can also bootstrap in a project-local sandbox by pointing the CLI at an explicit database path:

```sh
grind create "Local sandbox task" --db ./.task-dev.db
```

## Identity Defaults

Grind distinguishes the configured local human from ephemeral agent actors.

- human-oriented commands usually rely on the resolved `GRIND_HUMAN_NAME`
- agents should pass an explicit `--actor` such as `codex:agent-7`
- agent actors are created implicitly the first time they claim or mutate work

Example:

```sh
grind claim TASK-1 --actor codex:agent-7 --json
```

## First Sanity Checks

These commands confirm that the installation is usable:

```sh
grind version --json
grind config show --json
grind create "Installation smoke test" --json
grind list --json
```

If those pass, the local binary, DB path, migrations, and JSON contract are all working.
