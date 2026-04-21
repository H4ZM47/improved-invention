package export

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"strings"
)

// csvTaskHeader is the stable column layout for the task CSV export.
//
// Column names match the JSON field names so the two formats round-trip
// cleanly for the fields CSV can represent.
var csvTaskHeader = []string{
	"handle",
	"uuid",
	"title",
	"description",
	"status",
	"domain_id",
	"project_id",
	"milestone_id",
	"milestone_handle",
	"assignee_actor_id",
	"due_at",
	"tags",
	"created_at",
	"updated_at",
	"closed_at",
}

// EncodeCSV serializes the tasks in a Bundle as CSV.
//
// CSV is scoped to the primary task table. Other entity collections in the
// Bundle are deliberately not emitted because mixing tables in a single CSV
// stream is hostile to standard CSV consumers. Use JSON for a full-bundle
// export.
//
// Tags are joined with `|` into a single cell. Absent (nil pointer) fields
// are written as empty strings, keeping column count stable.
func EncodeCSV(bundle Bundle) ([]byte, error) {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	if err := w.Write(csvTaskHeader); err != nil {
		return nil, fmt.Errorf("write csv header: %w", err)
	}

	for _, task := range bundle.Tasks {
		row := []string{
			task.Handle,
			task.UUID,
			task.Title,
			task.Description,
			task.Status,
			derefString(task.DomainID),
			derefString(task.ProjectID),
			derefString(task.MilestoneID),
			derefString(task.MilestoneHandle),
			derefString(task.AssigneeActorID),
			derefString(task.DueAt),
			strings.Join(task.Tags, "|"),
			task.CreatedAt,
			task.UpdatedAt,
			derefString(task.ClosedAt),
		}
		if err := w.Write(row); err != nil {
			return nil, fmt.Errorf("write csv row: %w", err)
		}
	}

	w.Flush()
	if err := w.Error(); err != nil {
		return nil, fmt.Errorf("flush csv: %w", err)
	}
	return buf.Bytes(), nil
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
