package cli

import (
	"fmt"
	"io"
)

const agentInstructionsText = `Grind agent instructions

Core rules:
- always pass --json
- pass --no-input for state-changing commands
- always pass an explicit --actor like codex:agent-7
- branch on error.code and exit codes, not human text
- claim a task before mutating it
- never assume repo-aware helpers attach metadata implicitly

Baseline:
  grind <command> ... --actor codex:agent-7 --no-input --json

Safe loop:
  1. grind --config --json --actor codex:agent-7
  2. grind list --status backlog --json --actor codex:agent-7
  3. grind claim acquire TASK-1 --json --no-input --actor codex:agent-7
  4. grind update TASK-1 --status active --json --no-input --actor codex:agent-7
  5. grind show TASK-1 --json --actor codex:agent-7
  6. grind close TASK-1 --json --no-input --actor codex:agent-7
  7. grind claim release TASK-1 --json --no-input --actor codex:agent-7

Useful commands:
- grind list --here --json --actor codex:agent-7
- grind link-repo TASK-1 --json --no-input --actor codex:agent-7
- agents should not create manual time entries; grind time edit is interactive-only for humans
- grind link add TASK-1 blocks TASK-2 --json --no-input --actor codex:agent-7

Reclassification safety:
- when changing --domain or --project in non-interactive mode, also choose one of:
  --accept-default-assignee
  --assignee <actor-ref>
  --keep-assignee

Full guide:
- docs/guides/agent-integration.md

Built-in aliases:
- grind --agents
- grind --agent-help
`

func writeAgentInstructions(out io.Writer, asJSON bool) error {
	if asJSON {
		return writeJSONTo(out, map[string]any{
			"ok":      true,
			"command": "grind --agents",
			"data": map[string]any{
				"title":   "Grind agent instructions",
				"content": agentInstructionsText,
				"aliases": []string{"--agents", "--agent-help"},
				"guide":   "docs/guides/agent-integration.md",
			},
			"meta": map[string]any{},
		})
	}

	_, err := fmt.Fprint(out, agentInstructionsText)
	return err
}
