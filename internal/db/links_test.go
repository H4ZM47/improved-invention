package db

import (
	"context"
	"errors"
	"testing"
)

func TestCreateListAndRemoveTaskExternalLink(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)
	insertTask(t, db, "task-1", "TASK-1", "Task", nil, nil)

	created, err := CreateExternalLink(context.Background(), db, TaskExternalLinkCreateInput{
		TaskReference: "TASK-1",
		LinkType:      LinkTypeRepo,
		Target:        "https://github.com/H4ZM47/task-cli",
		Label:         "origin",
		MetadataJSON:  `{"branch":"main"}`,
	})
	if err != nil {
		t.Fatalf("CreateExternalLink() error = %v", err)
	}

	if got, want := created.TaskHandle, "TASK-1"; got != want {
		t.Fatalf("TaskHandle = %q, want %q", got, want)
	}
	if got, want := created.LinkType, LinkTypeRepo; got != want {
		t.Fatalf("LinkType = %q, want %q", got, want)
	}

	got, err := ListExternalLinksForTask(context.Background(), db, "TASK-1")
	if err != nil {
		t.Fatalf("ListExternalLinksForTask() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len(ListTaskExternalLinks()) = %d, want 1", len(got))
	}
	if got[0].UUID != created.UUID {
		t.Fatalf("listed link UUID = %q, want %q", got[0].UUID, created.UUID)
	}

	removed, err := RemoveExternalLink(context.Background(), db, TaskExternalLinkRemoveInput{
		TaskReference: "TASK-1",
		LinkUUID:      created.UUID,
	})
	if err != nil {
		t.Fatalf("RemoveExternalLink() error = %v", err)
	}
	if removed.UUID != created.UUID {
		t.Fatalf("removed link UUID = %q, want %q", removed.UUID, created.UUID)
	}

	got, err = ListExternalLinksForTask(context.Background(), db, "TASK-1")
	if err != nil {
		t.Fatalf("ListExternalLinksForTask() after remove error = %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("len(ListTaskExternalLinks()) after remove = %d, want 0", len(got))
	}
}

func TestCreateTaskExternalLinkRejectsInvalidType(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)
	insertTask(t, db, "task-1", "TASK-1", "Task", nil, nil)

	_, err := CreateExternalLink(context.Background(), db, TaskExternalLinkCreateInput{
		TaskReference: "TASK-1",
		LinkType:      "ticket",
		Target:        "https://example.com",
	})
	if !errors.Is(err, ErrInvalidLinkType) {
		t.Fatalf("CreateExternalLink() error = %v, want ErrInvalidLinkType", err)
	}
}

func TestCreateTaskExternalLinkRejectsInvalidMetadataJSON(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)
	insertTask(t, db, "task-1", "TASK-1", "Task", nil, nil)

	_, err := CreateExternalLink(context.Background(), db, TaskExternalLinkCreateInput{
		TaskReference: "TASK-1",
		LinkType:      LinkTypeFile,
		Target:        "/tmp/spec.md",
		MetadataJSON:  "{invalid",
	})
	if err == nil {
		t.Fatal("CreateExternalLink() error = nil, want invalid metadata failure")
	}
}

func TestCreateTaskExternalLinkEnforcesPerTaskUniqueness(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)
	insertTask(t, db, "task-1", "TASK-1", "Task", nil, nil)

	input := TaskExternalLinkCreateInput{
		TaskReference: "TASK-1",
		LinkType:      LinkTypeWorktree,
		Target:        "/Users/alex/task",
	}
	if _, err := CreateExternalLink(context.Background(), db, input); err != nil {
		t.Fatalf("first CreateExternalLink() error = %v", err)
	}

	if _, err := CreateExternalLink(context.Background(), db, input); err == nil {
		t.Fatal("second CreateExternalLink() succeeded, want uniqueness failure")
	}
}
