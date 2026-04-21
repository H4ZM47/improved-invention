package export

import (
	"encoding/csv"
	"strings"
	"testing"

	"github.com/H4ZM47/improved-invention/internal/app"
)

func TestEncodeCSVHeaderOnlyForEmptyBundle(t *testing.T) {
	t.Parallel()

	got, err := EncodeCSV(Bundle{})
	if err != nil {
		t.Fatalf("EncodeCSV() error = %v", err)
	}

	want := "handle,uuid,title,description,status,domain_id,project_id,assignee_actor_id,due_at,tags,created_at,updated_at,closed_at\n"
	if string(got) != want {
		t.Fatalf("EncodeCSV() empty bundle mismatch\n got: %q\nwant: %q", got, want)
	}
}

func TestEncodeCSVTaskRow(t *testing.T) {
	t.Parallel()

	bundle := Bundle{Tasks: []app.TaskRecord{sampleTask()}}

	got, err := EncodeCSV(bundle)
	if err != nil {
		t.Fatalf("EncodeCSV() error = %v", err)
	}

	records, err := csv.NewReader(strings.NewReader(string(got))).ReadAll()
	if err != nil {
		t.Fatalf("parse csv: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected header+1 row, got %d rows", len(records))
	}
	row := records[1]
	if row[0] != "TASK-1042" {
		t.Fatalf("row[handle] = %q, want TASK-1042", row[0])
	}
	if row[7] != "actor-uuid" {
		t.Fatalf("row[assignee_actor_id] = %q, want actor-uuid", row[7])
	}
	if row[9] != "cli|contracts" {
		t.Fatalf("row[tags] = %q, want cli|contracts", row[9])
	}
}

func TestEncodeCSVNilPointersRenderEmpty(t *testing.T) {
	t.Parallel()

	bundle := Bundle{Tasks: []app.TaskRecord{{
		Handle:    "TASK-9",
		UUID:      "u",
		Title:     "minimal",
		Status:    "backlog",
		CreatedAt: "2026-04-21T00:00:00Z",
		UpdatedAt: "2026-04-21T00:00:00Z",
	}}}
	got, err := EncodeCSV(bundle)
	if err != nil {
		t.Fatalf("EncodeCSV() error = %v", err)
	}
	records, _ := csv.NewReader(strings.NewReader(string(got))).ReadAll()
	row := records[1]
	for _, i := range []int{5, 6, 7, 8, 12} { // domain_id, project_id, assignee_actor_id, due_at, closed_at
		if row[i] != "" {
			t.Fatalf("row[%d] = %q, want empty", i, row[i])
		}
	}
}

func TestEncodeCSVEscapesEmbeddedDelimiters(t *testing.T) {
	t.Parallel()

	bundle := Bundle{Tasks: []app.TaskRecord{{
		Handle:      "TASK-1",
		UUID:        "u",
		Title:       `needs, quoting "here"`,
		Description: "line1\nline2",
		Status:      "active",
		Tags:        []string{"a", "b"},
		CreatedAt:   "2026-04-21T00:00:00Z",
		UpdatedAt:   "2026-04-21T00:00:00Z",
	}}}
	got, err := EncodeCSV(bundle)
	if err != nil {
		t.Fatalf("EncodeCSV() error = %v", err)
	}
	records, err := csv.NewReader(strings.NewReader(string(got))).ReadAll()
	if err != nil {
		t.Fatalf("parse csv: %v", err)
	}
	row := records[1]
	if row[2] != `needs, quoting "here"` {
		t.Fatalf("title round-trip failed: %q", row[2])
	}
	if row[3] != "line1\nline2" {
		t.Fatalf("description round-trip failed: %q", row[3])
	}
}
