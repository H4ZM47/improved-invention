package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
)

func TestListTasksOrdersByMostRecentlyUpdatedFirst(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)
	insertTask(t, db, "task-1", "TASK-1", "Older", nil, nil)
	insertTask(t, db, "task-2", "TASK-2", "Newer", nil, nil)

	if _, err := db.Exec(`
		UPDATE tasks
		SET updated_at = CASE handle
			WHEN 'TASK-1' THEN '2026-04-21 09:00:00'
			WHEN 'TASK-2' THEN '2026-04-21 10:00:00'
		END
	`); err != nil {
		t.Fatalf("seed updated_at values failed: %v", err)
	}

	items, err := ListTasks(context.Background(), db, TaskListQuery{})
	if err != nil {
		t.Fatalf("ListTasks() error = %v", err)
	}
	if got, want := len(items), 2; got != want {
		t.Fatalf("len(items) = %d, want %d", got, want)
	}
	if got, want := items[0].Handle, "TASK-2"; got != want {
		t.Fatalf("items[0].Handle = %q, want %q", got, want)
	}
	if got, want := items[1].Handle, "TASK-1"; got != want {
		t.Fatalf("items[1].Handle = %q, want %q", got, want)
	}
}

func TestListTasksSupportsFieldTagAndSearchFilters(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)
	domainID := insertDomain(t, db, "domain-1", "DOM-1", "Work")
	projectID := insertProjectFixture(t, db, "project-1", "PROJ-1", "Task CLI", domainID)
	assigneeID := insertActor(t, db, "actor-1", "ACT-1", "human", "", "alex")

	insertTask(t, db, "task-1", "TASK-1", "Write CLI contract", domainID, projectID)
	updateTaskFixture(t, db, "TASK-1", taskFixtureUpdate{
		Description: "Document the shared list command",
		Status:      "active",
		AssigneeID:  &assigneeID,
		DueAt:       stringPointer("2026-04-21T12:00:00Z"),
		Tags:        []string{"cli", "contract"},
	})

	insertTask(t, db, "task-2", "TASK-2", "Write docs", nil, nil)
	updateTaskFixture(t, db, "TASK-2", taskFixtureUpdate{
		Description: "General docs task",
		Status:      "backlog",
		DueAt:       stringPointer("2026-04-22T12:00:00Z"),
		Tags:        []string{"docs"},
	})

	items, err := ListTasks(context.Background(), db, TaskListQuery{
		Statuses:    []string{"active"},
		DomainRef:   stringPointer("DOM-1"),
		ProjectRef:  stringPointer("PROJ-1"),
		AssigneeRef: stringPointer("ACT-1"),
		DueBefore:   stringPointer("2026-04-21T23:59:59Z"),
		DueAfter:    stringPointer("2026-04-21T00:00:00Z"),
		Tags:        []string{"cli"},
		Search:      "contract",
	})
	if err != nil {
		t.Fatalf("ListTasks() error = %v", err)
	}
	if got, want := len(items), 1; got != want {
		t.Fatalf("len(items) = %d, want %d", got, want)
	}
	if got, want := items[0].Handle, "TASK-1"; got != want {
		t.Fatalf("items[0].Handle = %q, want %q", got, want)
	}
}

func TestListTasksSupportsRepoAndWorktreeContextFilters(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)
	attachedID := insertTask(t, db, "task-1", "TASK-1", "Attached task", nil, nil)
	insertTask(t, db, "task-2", "TASK-2", "Unattached task", nil, nil)

	if _, err := db.Exec(`
		INSERT INTO external_links(uuid, task_id, link_type, target, label)
		VALUES
		  ('link-1', ?, 'repo', 'https://github.com/H4ZM47/task-cli.git', 'repo'),
		  ('link-2', ?, 'worktree', '/Users/alex/task', 'worktree')
	`, attachedID, attachedID); err != nil {
		t.Fatalf("seed external links failed: %v", err)
	}

	items, err := ListTasks(context.Background(), db, TaskListQuery{
		RepoTarget:     stringPointer("https://github.com/H4ZM47/task-cli.git"),
		WorktreeTarget: stringPointer("/Users/alex/task"),
	})
	if err != nil {
		t.Fatalf("ListTasks() error = %v", err)
	}
	if got, want := len(items), 1; got != want {
		t.Fatalf("len(items) = %d, want %d", got, want)
	}
	if got, want := items[0].Handle, "TASK-1"; got != want {
		t.Fatalf("items[0].Handle = %q, want %q", got, want)
	}
}

type taskFixtureUpdate struct {
	Description string
	Status      string
	AssigneeID  *int64
	DueAt       *string
	Tags        []string
}

func insertProjectFixture(t *testing.T, db *sql.DB, uuid string, handle string, name string, domainID int64) int64 {
	t.Helper()

	result, err := db.Exec(`
		INSERT INTO projects(uuid, handle, name, domain_id)
		VALUES (?, ?, ?, ?)
	`, uuid, handle, name, domainID)
	if err != nil {
		t.Fatalf("insert project failed: %v", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("LastInsertId() for project failed: %v", err)
	}

	return id
}

func updateTaskFixture(t *testing.T, db *sql.DB, handle string, update taskFixtureUpdate) {
	t.Helper()

	tagsJSON, err := json.Marshal(update.Tags)
	if err != nil {
		t.Fatalf("json.Marshal(tags) error = %v", err)
	}

	if _, err := db.Exec(`
		UPDATE tasks
		SET description = ?, status = ?, assignee_actor_id = ?, due_at = ?, tags = ?
		WHERE handle = ?
	`, update.Description, update.Status, nullableInt64(update.AssigneeID), nullableStringAny(update.DueAt), string(tagsJSON), handle); err != nil {
		t.Fatalf("update task fixture failed: %v", err)
	}
}

func nullableInt64(value *int64) any {
	if value == nil {
		return nil
	}
	return *value
}

func stringPointer(value string) *string {
	return &value
}

func nullableStringAny(value *string) any {
	if value == nil {
		return nil
	}
	return *value
}
