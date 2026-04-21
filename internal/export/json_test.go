package export

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/H4ZM47/improved-invention/internal/app"
)

func TestEncodeJSONEmptyBundleIsDeterministic(t *testing.T) {
	t.Parallel()

	got, err := EncodeJSON(Bundle{})
	if err != nil {
		t.Fatalf("EncodeJSON() error = %v", err)
	}

	want := `{
  "version": 1,
  "tasks": [],
  "domains": [],
  "projects": [],
  "actors": [],
  "links": [],
  "relationships": []
}
`
	if string(got) != want {
		t.Fatalf("EncodeJSON() empty bundle mismatch\n got: %q\nwant: %q", got, want)
	}
}

func TestEncodeJSONPreservesTaskFields(t *testing.T) {
	t.Parallel()

	bundle := Bundle{
		Tasks: []app.TaskRecord{sampleTask()},
	}

	got, err := EncodeJSON(bundle)
	if err != nil {
		t.Fatalf("EncodeJSON() error = %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(got, &decoded); err != nil {
		t.Fatalf("decode result: %v", err)
	}

	tasks, ok := decoded["tasks"].([]any)
	if !ok || len(tasks) != 1 {
		t.Fatalf("tasks shape = %v, want one entry", decoded["tasks"])
	}
	task := tasks[0].(map[string]any)
	if task["handle"] != "TASK-1042" {
		t.Fatalf("task.handle = %v, want TASK-1042", task["handle"])
	}
	if task["tags"].([]any)[0] != "cli" {
		t.Fatalf("task.tags = %v, want [cli, contracts]", task["tags"])
	}
}

func TestEncodeJSONNilSlicesBecomeEmpty(t *testing.T) {
	t.Parallel()

	bundle := Bundle{
		Tasks: []app.TaskRecord{{
			Handle: "TASK-1",
			UUID:   "uuid-1",
			Title:  "x",
			Status: "backlog",
			// Tags is nil on purpose
			CreatedAt: "2026-04-21T00:00:00Z",
			UpdatedAt: "2026-04-21T00:00:00Z",
		}},
	}

	got, err := EncodeJSON(bundle)
	if err != nil {
		t.Fatalf("EncodeJSON() error = %v", err)
	}
	if !strings.Contains(string(got), `"tags": []`) {
		t.Fatalf("expected empty tags array in output, got:\n%s", got)
	}
	if strings.Contains(string(got), "null") {
		// pointer fields may legitimately be null; only reject the slice ones
		if strings.Contains(string(got), `"tags": null`) {
			t.Fatalf("tags should not be null:\n%s", got)
		}
	}
}

func sampleTask() app.TaskRecord {
	assignee := "actor-uuid"
	return app.TaskRecord{
		Handle:          "TASK-1042",
		UUID:            "9e04c26b-fb8a-4f96-a3f5-6544d07d7757",
		Title:           "Write initial CLI contract reference",
		Description:     "Capture JSON envelopes.",
		Status:          "active",
		AssigneeActorID: &assignee,
		Tags:            []string{"cli", "contracts"},
		CreatedAt:       "2026-04-21T03:00:00Z",
		UpdatedAt:       "2026-04-21T03:12:00Z",
	}
}
