package export

import (
	"fmt"
	"strings"
)

// EncodeTXT serializes a Bundle as a plain-text task listing.
//
// Each task renders as a short block:
//
//	TASK-1042  Write initial CLI contract reference
//	  status: active    assignee: ACT-203
//	  tags: cli, contracts
//	  due: 2026-04-30T00:00:00Z    updated: 2026-04-21T03:12:00Z
//
// Blocks are separated by a single blank line. Unknown or empty fields are
// omitted from the body lines but the handle/title header is always present.
// Output ends with a single trailing newline.
func EncodeTXT(bundle Bundle) []byte {
	var b strings.Builder

	if len(bundle.Tasks) == 0 {
		b.WriteString("(no tasks)\n")
		return []byte(b.String())
	}

	for i, task := range bundle.Tasks {
		if i > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "%s  %s\n", task.Handle, task.Title)

		line := "  status: " + task.Status
		if task.AssigneeActorID != nil && *task.AssigneeActorID != "" {
			line += "    assignee: " + *task.AssigneeActorID
		}
		b.WriteString(line + "\n")

		if len(task.Tags) > 0 {
			fmt.Fprintf(&b, "  tags: %s\n", strings.Join(task.Tags, ", "))
		}
		if task.MilestoneHandle != nil && *task.MilestoneHandle != "" {
			fmt.Fprintf(&b, "  milestone: %s\n", *task.MilestoneHandle)
		}

		meta := "  updated: " + task.UpdatedAt
		if task.DueAt != nil && *task.DueAt != "" {
			meta = "  due: " + *task.DueAt + "    updated: " + task.UpdatedAt
		}
		if task.ClosedAt != nil && *task.ClosedAt != "" {
			meta += "    closed: " + *task.ClosedAt
		}
		b.WriteString(meta + "\n")

		if task.Description != "" {
			for _, descLine := range strings.Split(task.Description, "\n") {
				fmt.Fprintf(&b, "  %s\n", descLine)
			}
		}
	}

	return []byte(b.String())
}
