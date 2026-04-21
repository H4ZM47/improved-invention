package export

import (
	"fmt"
	"strings"

	"github.com/H4ZM47/task-cli/internal/app"
)

// EncodeMarkdown serializes a Bundle as the built-in v1 Markdown format.
//
// Layout:
//
//	# Task Export
//
//	| Metric  | Count |
//	| ------- | ----- |
//	| Tasks   | 3     |
//	...
//
//	## Tasks
//
//	- [ ] `TASK-1042` Write initial CLI contract reference — `active`
//	      - tags: cli, contracts
//	      - assignee: actor-uuid
//	      - due: 2026-04-30T00:00:00Z
//
// Custom templates are out of scope for v1; the shape is intentionally
// opinionated and stable.
func EncodeMarkdown(bundle Bundle) []byte {
	var b strings.Builder
	b.WriteString("# Task Export\n\n")
	writeMarkdownSummary(&b, bundle)

	b.WriteString("\n## Tasks\n\n")
	if len(bundle.Tasks) == 0 {
		b.WriteString("_No tasks._\n")
		return []byte(b.String())
	}
	for _, task := range bundle.Tasks {
		writeMarkdownTask(&b, task)
	}
	return []byte(b.String())
}

func writeMarkdownSummary(b *strings.Builder, bundle Bundle) {
	b.WriteString("| Metric | Count |\n")
	b.WriteString("| ------ | ----- |\n")
	fmt.Fprintf(b, "| Tasks | %d |\n", len(bundle.Tasks))
	fmt.Fprintf(b, "| Domains | %d |\n", len(bundle.Domains))
	fmt.Fprintf(b, "| Projects | %d |\n", len(bundle.Projects))
	fmt.Fprintf(b, "| Actors | %d |\n", len(bundle.Actors))
	fmt.Fprintf(b, "| Links | %d |\n", len(bundle.Links))
	fmt.Fprintf(b, "| Relationships | %d |\n", len(bundle.Relationships))
}

func writeMarkdownTask(b *strings.Builder, task app.TaskRecord) {
	check := "[ ]"
	if task.Status == "completed" {
		check = "[x]"
	}
	fmt.Fprintf(b, "- %s `%s` %s — `%s`\n", check, task.Handle, task.Title, task.Status)
	if len(task.Tags) > 0 {
		fmt.Fprintf(b, "    - tags: %s\n", strings.Join(task.Tags, ", "))
	}
	if task.AssigneeActorID != nil && *task.AssigneeActorID != "" {
		fmt.Fprintf(b, "    - assignee: %s\n", *task.AssigneeActorID)
	}
	if task.DueAt != nil && *task.DueAt != "" {
		fmt.Fprintf(b, "    - due: %s\n", *task.DueAt)
	}
	if task.ClosedAt != nil && *task.ClosedAt != "" {
		fmt.Fprintf(b, "    - closed: %s\n", *task.ClosedAt)
	}
	if task.Description != "" {
		b.WriteString("\n")
		for _, line := range strings.Split(task.Description, "\n") {
			fmt.Fprintf(b, "    %s\n", line)
		}
	}
}
