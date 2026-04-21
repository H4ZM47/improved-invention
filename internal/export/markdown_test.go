package export

import (
	"strings"
	"testing"

	"github.com/H4ZM47/improved-invention/internal/app"
)

func TestEncodeMarkdownEmptyBundle(t *testing.T) {
	t.Parallel()

	got := string(EncodeMarkdown(Bundle{}))
	if !strings.Contains(got, "# Task Export") {
		t.Fatalf("missing heading in empty bundle output:\n%s", got)
	}
	if !strings.Contains(got, "| Tasks | 0 |") {
		t.Fatalf("missing zero-count summary row:\n%s", got)
	}
	if !strings.Contains(got, "_No tasks._") {
		t.Fatalf("missing empty-tasks placeholder:\n%s", got)
	}
}

func TestEncodeMarkdownRendersTaskCheckbox(t *testing.T) {
	t.Parallel()

	bundle := Bundle{Tasks: []app.TaskRecord{sampleTask()}}
	got := string(EncodeMarkdown(bundle))

	if !strings.Contains(got, "- [ ] `TASK-1042` Write initial CLI contract reference — `active`") {
		t.Fatalf("expected unchecked task line, got:\n%s", got)
	}
	if !strings.Contains(got, "- tags: cli, contracts") {
		t.Fatalf("expected tag list, got:\n%s", got)
	}
	if !strings.Contains(got, "- assignee: actor-uuid") {
		t.Fatalf("expected assignee, got:\n%s", got)
	}
}

func TestEncodeMarkdownCompletedTaskIsChecked(t *testing.T) {
	t.Parallel()

	done := "2026-04-21T04:00:00Z"
	task := sampleTask()
	task.Status = "completed"
	task.ClosedAt = &done

	got := string(EncodeMarkdown(Bundle{Tasks: []app.TaskRecord{task}}))
	if !strings.Contains(got, "- [x] `TASK-1042`") {
		t.Fatalf("completed task should render with [x], got:\n%s", got)
	}
	if !strings.Contains(got, "- closed: 2026-04-21T04:00:00Z") {
		t.Fatalf("completed task should show closed_at, got:\n%s", got)
	}
}

func TestEncodeMarkdownSummaryCountsAllEntities(t *testing.T) {
	t.Parallel()

	bundle := Bundle{
		Tasks:    []app.TaskRecord{sampleTask(), sampleTask()},
		Domains:  []app.DomainRecord{{Handle: "DOM-1"}},
		Projects: []app.ProjectRecord{{Handle: "PRJ-1"}, {Handle: "PRJ-2"}, {Handle: "PRJ-3"}},
	}
	got := string(EncodeMarkdown(bundle))
	for _, want := range []string{
		"| Tasks | 2 |",
		"| Domains | 1 |",
		"| Projects | 3 |",
		"| Actors | 0 |",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing summary row %q in output:\n%s", want, got)
		}
	}
}
