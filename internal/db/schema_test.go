package db

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	taskconfig "github.com/H4ZM47/task-cli/internal/config"
)

func TestSchemaEnforcesProjectDomainContainment(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)

	if _, err := db.Exec(`
		INSERT INTO projects(uuid, handle, name, domain_id)
		VALUES ('proj-uuid', 'PROJ-1', 'CLI Project', 999)
	`); err == nil {
		t.Fatal("insert project without domain succeeded, want foreign key failure")
	}
}

func TestSchemaEnforcesActorIdentityUniqueness(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)

	if _, err := db.Exec(`
		INSERT INTO actors(uuid, handle, kind, provider, external_id, display_name)
		VALUES ('actor-1', 'ACT-1', 'agent', 'codex', 'agent-7', 'Agent Seven')
	`); err != nil {
		t.Fatalf("first actor insert failed: %v", err)
	}

	if _, err := db.Exec(`
		INSERT INTO actors(uuid, handle, kind, provider, external_id, display_name)
		VALUES ('actor-2', 'ACT-2', 'agent', 'codex', 'agent-7', 'Duplicate Agent Seven')
	`); err == nil {
		t.Fatal("duplicate actor identity insert succeeded, want unique failure")
	}
}

func TestSchemaEnforcesOneOpenClaimPerTask(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)
	domainID := insertDomain(t, db, "dom-1", "DOM-1", "Work")
	_ = domainID
	actorID := insertActor(t, db, "actor-1", "ACT-1", "agent", "codex", "agent-7")
	taskID := insertTask(t, db, "task-1", "TASK-1", "Write schema", nil, nil)

	if _, err := db.Exec(`
		INSERT INTO claims(uuid, task_id, actor_id, expires_at)
		VALUES ('claim-1', ?, ?, '2030-01-01T00:00:00Z')
	`, taskID, actorID); err != nil {
		t.Fatalf("first claim insert failed: %v", err)
	}

	if _, err := db.Exec(`
		INSERT INTO claims(uuid, task_id, actor_id, expires_at)
		VALUES ('claim-2', ?, ?, '2030-01-02T00:00:00Z')
	`, taskID, actorID); err == nil {
		t.Fatal("second open claim insert succeeded, want unique failure")
	}
}

func TestSchemaEnforcesOneParentPerChild(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)
	parentOne := insertTask(t, db, "task-parent-1", "TASK-1", "Parent one", nil, nil)
	parentTwo := insertTask(t, db, "task-parent-2", "TASK-2", "Parent two", nil, nil)
	child := insertTask(t, db, "task-child", "TASK-3", "Child", nil, nil)

	if _, err := db.Exec(`
		INSERT INTO relationships(uuid, source_task_id, target_task_id, relationship_type)
		VALUES ('rel-1', ?, ?, 'parent_child')
	`, parentOne, child); err != nil {
		t.Fatalf("first parent relationship insert failed: %v", err)
	}

	if _, err := db.Exec(`
		INSERT INTO relationships(uuid, source_task_id, target_task_id, relationship_type)
		VALUES ('rel-2', ?, ?, 'parent_child')
	`, parentTwo, child); err == nil {
		t.Fatal("second parent relationship insert succeeded, want unique failure")
	}
}

func TestSchemaSeedsHandleSequences(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)

	rows, err := db.Query(`SELECT entity_type, next_value FROM handle_sequences ORDER BY entity_type`)
	if err != nil {
		t.Fatalf("query handle_sequences failed: %v", err)
	}
	defer rows.Close()

	got := map[string]int{}
	for rows.Next() {
		var entity string
		var next int
		if err := rows.Scan(&entity, &next); err != nil {
			t.Fatalf("scan handle_sequences row failed: %v", err)
		}
		got[entity] = next
	}

	want := map[string]int{
		"actor":   1,
		"domain":  1,
		"project": 1,
		"task":    1,
	}
	if len(got) != len(want) {
		t.Fatalf("len(got) = %d, want %d", len(got), len(want))
	}
	for entity, next := range want {
		if got[entity] != next {
			t.Fatalf("handle_sequences[%q] = %d, want %d", entity, got[entity], next)
		}
	}
}

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()

	cfg := taskconfig.Resolved{
		DBPath:      filepath.Join(t.TempDir(), "task.db"),
		BusyTimeout: 5 * time.Second,
	}

	db, err := Open(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	t.Cleanup(func() {
		_ = db.Close()
	})

	return db
}

func insertDomain(t *testing.T, db *sql.DB, uuid string, handle string, name string) int64 {
	t.Helper()

	result, err := db.Exec(`
		INSERT INTO domains(uuid, handle, name)
		VALUES (?, ?, ?)
	`, uuid, handle, name)
	if err != nil {
		t.Fatalf("insert domain failed: %v", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("LastInsertId() for domain failed: %v", err)
	}

	return id
}

func insertActor(t *testing.T, db *sql.DB, uuid string, handle string, kind string, provider string, externalID string) int64 {
	t.Helper()

	var providerValue any = provider
	if provider == "" {
		providerValue = nil
	}

	result, err := db.Exec(`
		INSERT INTO actors(uuid, handle, kind, provider, external_id, display_name)
		VALUES (?, ?, ?, ?, ?, ?)
	`, uuid, handle, kind, providerValue, externalID, handle)
	if err != nil {
		t.Fatalf("insert actor failed: %v", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("LastInsertId() for actor failed: %v", err)
	}

	return id
}

func insertTask(t *testing.T, db *sql.DB, uuid string, handle string, title string, domainID any, projectID any) int64 {
	t.Helper()

	result, err := db.Exec(`
		INSERT INTO tasks(uuid, handle, title, domain_id, project_id)
		VALUES (?, ?, ?, ?, ?)
	`, uuid, handle, title, domainID, projectID)
	if err != nil {
		t.Fatalf("insert task failed: %v", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("LastInsertId() for task failed: %v", err)
	}

	return id
}
