# Backup, Export, and Report Usage

Grind supports three different read and portability paths:

- `backup`: full-fidelity database copies for disaster recovery or machine moves
- `export`: portable task snapshots in JSON, CSV, TXT, or Markdown
- `report`: a read-only local HTML and JSON server for browsing the current DB

## Full-Fidelity Backups

Create a backup artifact:

```sh
grind backup create --output ./grind-backup.sqlite --json
```

What a backup preserves:

- UUIDs and human handles
- tasks, projects, domains, actors
- events, relationships, and external links
- saved views
- claim state as stored in the source database

Use backups when you want a restorable copy of the entire system, not just a report.

## Restore A Backup

Restore into a target database:

```sh
grind restore apply --input ./grind-backup.sqlite --db ./restored-grind.db --json
```

Overwrite an existing target database:

```sh
grind restore apply --input ./grind-backup.sqlite --db ./grind.db --force --json
```

Operational notes:

- restore is a whole-database action, not a selective import
- restore preserves IDs, history, and links
- stale claims are cleaned up on first open according to the current lease rules

## Structured Exports

Export the current database contents as JSON:

```sh
grind export json --output ./tasks.json
```

Other supported formats:

```sh
grind export csv --output ./tasks.csv
grind export txt --output ./tasks.txt
grind export markdown --output ./tasks.md
```

If `--output` is omitted, export content is written to stdout.

Use exports when you need:

- a portable snapshot for analysis
- a Markdown or TXT summary for humans
- CSV for spreadsheets
- JSON for downstream tooling

## Local Report Server

Start the read-only report server:

```sh
grind report serve --addr 127.0.0.1:8080
```

Key routes:

- `/tasks`: HTML grind list
- `/tasks/{task-ref}`: HTML task detail
- `/api/tasks`: JSON list endpoint

The server only allows `GET` and `HEAD`. Mutating requests return `405 Method Not Allowed`.

## Filtering The Report

The report server accepts the same core list filters through query parameters:

- `status`
- `tag`
- `domain`
- `project`
- `assignee`
- `due-before`
- `due-after`
- `search`

Examples:

```text
http://127.0.0.1:8080/tasks?status=active&tag=docs
http://127.0.0.1:8080/api/tasks?search=launch
```

## Choosing The Right Tool

Use `backup` when:

- you need a restorable full copy
- you are moving to a new machine
- you want disaster-recovery coverage

Use `export` when:

- you need a derived artifact for another tool or human reader
- you do not need to restore the exact DB later

Use `report` when:

- you want a local read-only browser view over the live database
- you need to share a local filtered view without teaching someone the CLI first
