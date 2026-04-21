package db

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	taskconfig "github.com/H4ZM47/improved-invention/internal/config"
)

func TestOpenAppliesRuntimePragmas(t *testing.T) {
	t.Parallel()

	cfg := taskconfig.Resolved{
		DBPath:      filepath.Join(t.TempDir(), "task.db"),
		BusyTimeout: 7 * time.Second,
	}

	db, err := Open(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	assertPragmaInt(t, db, "busy_timeout", 7000)
	assertPragmaInt(t, db, "foreign_keys", 1)
	assertPragmaText(t, db, "journal_mode", "wal")
	assertTableExists(t, db, "tasks")
	assertTableExists(t, db, "projects")
	assertTableExists(t, db, "domains")
	assertTableExists(t, db, "actors")
	assertTableExists(t, db, "claims")
	assertTableExists(t, db, "relationships")
	assertTableExists(t, db, "external_links")
	assertTableExists(t, db, "events")
	assertTableExists(t, db, "saved_views")
	assertTableExists(t, db, "handle_sequences")
}

func assertPragmaInt(t *testing.T, db *sql.DB, pragma string, want int) {
	t.Helper()

	var got int
	query := "PRAGMA " + pragma + ";"
	if err := db.QueryRow(query).Scan(&got); err != nil {
		t.Fatalf("QueryRow(%q) error = %v", query, err)
	}

	if got != want {
		t.Fatalf("%s = %d, want %d", pragma, got, want)
	}
}

func assertPragmaText(t *testing.T, db *sql.DB, pragma string, want string) {
	t.Helper()

	var got string
	query := "PRAGMA " + pragma + ";"
	if err := db.QueryRow(query).Scan(&got); err != nil {
		t.Fatalf("QueryRow(%q) error = %v", query, err)
	}

	if got != want {
		t.Fatalf("%s = %q, want %q", pragma, got, want)
	}
}

func assertTableExists(t *testing.T, db *sql.DB, name string) {
	t.Helper()

	var got string
	if err := db.QueryRow(
		`SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?`,
		name,
	).Scan(&got); err != nil {
		t.Fatalf("table %s missing: %v", name, err)
	}

	if got != name {
		t.Fatalf("table lookup returned %q, want %q", got, name)
	}
}
