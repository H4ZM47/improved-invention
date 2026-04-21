package testutil

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	taskconfig "github.com/H4ZM47/grind/internal/config"
	taskdb "github.com/H4ZM47/grind/internal/db"
)

// OpenSQLiteDB opens a real migrated SQLite database in a temp directory.
func OpenSQLiteDB(t *testing.T) *sql.DB {
	t.Helper()
	return OpenSQLiteDBAtPath(t, filepath.Join(t.TempDir(), "task.db"))
}

// OpenSQLiteDBAtPath opens a real migrated SQLite database at an explicit path.
func OpenSQLiteDBAtPath(t *testing.T, dbPath string) *sql.DB {
	t.Helper()

	cfg := taskconfig.Resolved{
		DBPath:      dbPath,
		BusyTimeout: 5 * time.Second,
		HumanName:   "alex",
	}

	db, err := taskdb.Open(context.Background(), cfg)
	if err != nil {
		t.Fatalf("taskdb.Open(%q) error = %v", dbPath, err)
	}

	t.Cleanup(func() {
		_ = db.Close()
	})

	return db
}

// ResolvedConfig returns the standard test runtime configuration.
func ResolvedConfig(dbPath string) taskconfig.Resolved {
	return taskconfig.Resolved{
		DBPath:      dbPath,
		BusyTimeout: 5 * time.Second,
		HumanName:   "alex",
	}
}
