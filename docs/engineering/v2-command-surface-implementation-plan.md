# V2 Command Surface Implementation Plan

Date: 2026-04-21
Status: Approved implementation plan derived from the V2 command-surface review backlog in Grind.

## Goal

Implement the approved V2 command-surface redesign so the CLI is shorter, more consistent, and more human-friendly without leaving overlapping command concepts in place.

The implementation should be a hard cut. Deprecated V1 command forms should not linger as long-term aliases. If a removed form is intercepted at all, it should fail with a direct migration message to the new canonical form.

## Final Approved Surface

### Root informational flags

- `grind --version`
- `grind --config`
- `grind --agents`
- `grind --agent-help`

### Task lifecycle

- `grind open <task>`
- `grind close <task>`
- `grind cancel <task>`

Tasks remain the executable work item type.

### Claim lifecycle

- `grind claim acquire <task>`
- `grind claim renew <task>`
- `grind claim release <task>`
- `grind claim unlock <task>`

### Task time tracking

- `grind time start <task>`
- `grind time pause <task>`
- `grind time resume <task>`

Manual time remains under `grind time`, but the human-facing editing flow is centered on:

- `grind time edit`

The interactive `grind time edit` flow should show existing entries for a task, allow selection of an existing entry to edit, and allow creation of a new manual entry from the same interactive flow. There should be no separate agent-safe non-interactive creation path for manual time entry.

### Project and domain lifecycle

Projects and domains are structural containers, not executable work items.

- `grind project open <project>`
- `grind project close <project>`
- `grind project cancel <project>`
- `grind domain open <domain>`
- `grind domain close <domain>`
- `grind domain cancel <domain>`

Projects and domains do not get claim locks and do not get live time tracking.

### Restore and local HTML server

- `grind restore`
- `grind serve`

Keep file exports under:

- `grind export ...`

### Universal link model

`link` becomes the only CLI concept for both task-to-task relationships and task-to-external-resource references.

Canonical mutation shape:

- `grind link add <type> <source> <target>`
- `grind link list <source>`
- `grind link remove <type> <source> <target>`

Repo helper rename:

- `grind link-repo`

The `grind relationship ...` surface should be removed.

### Description entry

- `--description` always means literal text
- `--description-file` always means file contents
- interactive `grind create "Task"` should offer a prompt or editor for optional description authoring
- `grind create "Task" --no-input` should create without a description unless another rule explicitly requires one

## Superseded Review Items

These reviewed ideas were superseded by later final decisions and should not be implemented:

- `TASK-24`: superseded by the split decision to use `grind restore` and `grind serve` while keeping `grind export`
- `TASK-26`: superseded by the final task lifecycle verbs `open`, `close`, `cancel`

The implementation should follow the final approved tasks, not the superseded ones.

## Implementation Order

### Phase 1: Parser and command tree reshaping

Implement the top-level command reshaping first:

- add `open` and `cancel` task commands
- convert claim operations into a nested `claim` namespace
- move live time controls under `time`
- add `project open|close|cancel`
- add `domain open|close|cancel`
- collapse `restore apply` to `restore`
- collapse `report serve` to `serve`
- add `link-repo`
- remove `relationship` command registration

During this phase, keep the internal application/storage interfaces working through thin adapters if needed, but make the command tree reflect the final public surface immediately.

### Phase 2: Internal lifecycle and routing cleanup

After the command tree is in place, normalize lifecycle routing so the new verbs map cleanly to the right status transitions:

- task `open` should reopen to the non-terminal baseline state used by the product
- task `close` should map to the normal completed terminal path
- task `cancel` should map to the cancelled terminal path
- project/domain `open|close|cancel` should map to the same lifecycle concepts as tasks, while preserving the fact that they remain structural entities

This phase should also remove V1 `close --status ...` dependencies from the public command handlers.

### Phase 3: Claim and time refactor

Refactor the root claim and time command handlers:

- move claim logic behind `claim acquire|renew|release|unlock`
- move live session logic behind `time start|pause|resume`
- preserve agent-safe JSON output and exit codes
- keep manual time editing interactive-first and human-oriented

Manual time entry should remain intentionally limited for agents. Do not invent a new agent-safe manual time creation API in this phase.

### Phase 4: Universal link unification

This is the highest-risk phase and should be treated as its own coherent slice.

Required changes:

- merge the public concepts of `relationship` and external `link`
- make `link add` accept both task-to-task and task-to-resource link types
- make `link list` emit a stable `target_kind`
- document the two target shapes clearly in help output
- keep validation split internally if needed, but present one user-facing concept

This phase should also introduce the short helper:

- `grind link-repo <task>`

### Phase 5: Description UX improvements

Extend create/update flows to support:

- `--description <text>`
- `--description-file <path>`
- interactive authoring for humans when appropriate

Design constraints:

- do not infer file paths from bare `--description` values
- do not auto-prompt in `--no-input`
- keep agent-safe behavior deterministic

### Phase 6: Documentation, contract, and dogfooding

Update all outward-facing materials:

- README
- CLI surface reference
- CLI contract reference
- install/bootstrap guide
- human quickstart
- agent guide
- Homebrew smoke checks
- JSON success and error examples

Then dogfood the full redesigned surface with Grind itself and capture follow-up bugs before release.

## Data and Output Requirements

### JSON command naming

Every renamed command must emit the new canonical command string in JSON payloads. Do not preserve old command names in `command` fields once the hard cut lands.

### Link output shape

Unified link output must distinguish target categories explicitly. Add a field such as:

- `target_kind: "task"`
- `target_kind: "external"`

Do not rely on consumers to infer target kind from the link type string alone.

### Error handling

Removed V1 forms should fail with direct migration guidance. Examples:

- use `grind claim acquire` instead of `grind claim`
- use `grind time start` instead of `grind start`
- use `grind serve` instead of `grind report serve`

The exact exit class should stay consistent with existing invalid-argument handling.

## Testing Plan

### CLI contract tests

Add or update contract tests for:

- new command names and JSON `command` fields
- removed V1 forms producing clear migration errors
- claim lifecycle under the nested `claim` namespace
- live time lifecycle under the `time` namespace
- `serve` and `restore` root-level forms
- `link-repo`

### Behavioral tests

Add or update integration coverage for:

- task/project/domain `open|close|cancel`
- universal link handling across both task and external targets
- `link list` target-kind output
- interactive description authoring behavior
- `--description-file` behavior

### Dogfood checks

Run representative flows as both:

- a human user
- an explicit agent actor

Focus specifically on:

- help discoverability
- command length and readability
- removal guidance for old forms
- JSON stability for agents

## Suggested Execution Slices

To reduce risk, implement this in slices:

1. Root and lifecycle renames
2. Claim and live-time namespace reshaping
3. Description UX improvements
4. Universal link unification
5. Documentation and dogfood pass

The universal link slice should not be mixed casually into unrelated command renames. It is the most conceptually invasive change in the plan.

