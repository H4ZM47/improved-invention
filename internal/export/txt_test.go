package export

import (
	"strings"
	"testing"

	"github.com/H4ZM47/improved-invention/internal/app"
)

func TestEncodeTXTEmptyBundle(t *testing.T) {
	t.Parallel()

	got := string(EncodeTXT(Bundle{}))
	if got != "(no tasks)\n" {
		t.Fatalf("EncodeTXT() empty bundle = %q", got)
	}
}

func TestEncodeTXTTaskBlock(t *testing.T) {
	t.Parallel()

	got := string(EncodeTXT(Bundle{Tasks: []app.TaskRecord{sampleTask()}}))

	wantLines := []string{
		"TASK-1042  Write initial CLI contract reference",
		"  status: active    assignee: actor-uuid",
		"  tags: cli, contracts",
		"  updated: 2026-04-21T03:12:00Z",
		"  Capture JSON envelopes.",
	}
	for _, line := range wantLines {
		if !strings.Contains(got, line) {
			t.Fatalf("missing line %q in output:\n%s", line, got)
		}
	}
	if !strings.HasSuffix(got, "\n") {
		t.Fatalf("output must end with newline, got:\n%s", got)
	}
}

func TestEncodeTXTSeparatesMultipleTasks(t *testing.T) {
	t.Parallel()

	bundle := Bundle{Tasks: []app.TaskRecord{
		{Handle: "TASK-1", Title: "one", Status: "backlog", UpdatedAt: "2026-04-21T00:00:00Z"},
		{Handle: "TASK-2", Title: "two", Status: "backlog", UpdatedAt: "2026-04-21T00:00:00Z"},
	}}
	got := string(EncodeTXT(bundle))
	if !strings.Contains(got, "TASK-1  one\n") || !strings.Contains(got, "TASK-2  two\n") {
		t.Fatalf("expected both task headers, got:\n%s", got)
	}
	if !strings.Contains(got, "\n\nTASK-2") {
		t.Fatalf("expected blank line between blocks, got:\n%s", got)
	}
}
