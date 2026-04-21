# Claim and Operator Procedures

This guide covers the operational rules around claims, stale locks, and manual intervention.

## Claim Model

Claims are exclusive task locks.

- one task can have at most one active claim
- normal state-changing task updates require a valid claim
- claims expire automatically after the configured lease duration
- the default lease is 24 hours
- renewal is explicit, not automatic

Inspect the current lease setting:

```sh
task config show --json
```

## Normal Claim Lifecycle

Acquire a claim:

```sh
task claim TASK-1 --actor codex:agent-7 --no-input --json
```

Renew a claim before expiry:

```sh
task renew TASK-1 --actor codex:agent-7 --no-input --json
```

Release the claim when work is complete or handed off:

```sh
task release TASK-1 --actor codex:agent-7 --no-input --json
```

## When To Use Manual Unlock

Use `task unlock` only when the normal holder cannot release the task:

- the agent process crashed
- the human or agent is gone and the lease cannot wait
- a stale lock is blocking urgent reassignment
- you are repairing an operational mistake

Command:

```sh
task unlock TASK-1 --json
```

Manual unlock is intentionally separate from normal release because it overrides the current holder.

## Recommended Operator Checks

Before unlocking:

1. inspect the task and current state
2. confirm the holder is actually stale or unavailable
3. check whether the claim is close to natural expiry
4. prefer `renew` or `release` if the current holder is still healthy

Useful commands:

```sh
task show TASK-1 --json
task actor show ACT-2 --json
task list --status active --json
```

## Expiry Behavior

Claims are not meant to block work forever.

- expired claims are cleared automatically when the task is reopened by the CLI
- operators do not need to unlock a claim that has already aged out
- renewal must be intentional, which helps prevent silent stale-lock extension

If you see a conflict on a claim that should have expired, re-run the command against the same DB path and inspect the task state before escalating.

## Human And Agent Responsibilities

Humans:

- claim before mutating shared work
- release claims promptly when they stop working
- use manual unlock sparingly

Agents:

- always pass explicit `--actor`
- use `--no-input --json`
- treat claim conflicts as expected workflow outcomes
- renew long-running claims intentionally

## Incident Pattern: Stuck Claim

Recommended response:

1. `task show TASK-1 --json`
2. decide whether the holder is still active
3. if active, wait or coordinate
4. if stale but recoverable, let the holder release or renew
5. if stale and blocking, `task unlock TASK-1 --json`
6. new worker claims the task normally

That keeps the audit history clear and avoids bypassing the claim system for convenience.
