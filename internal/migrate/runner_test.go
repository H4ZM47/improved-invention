package migrate

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
	"testing/fstest"
)

func TestDiscoverSortsSQLFiles(t *testing.T) {
	t.Parallel()

	got, err := Discover(fstest.MapFS{
		"002_second.sql": {Data: []byte("SELECT 2;")},
		"001_first.sql":  {Data: []byte("SELECT 1;")},
		"README.md":      {Data: []byte("ignore")},
	})
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2", len(got))
	}

	if got[0].Name != "001_first.sql" || got[1].Name != "002_second.sql" {
		t.Fatalf("unexpected order: %#v", got)
	}
}

func TestRunAppliesMigrationsOnce(t *testing.T) {
	t.Parallel()

	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "task.db"))
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	fsys := fstest.MapFS{
		"001_create_notes.sql": {
			Data: []byte(`CREATE TABLE notes (id INTEGER PRIMARY KEY, body TEXT NOT NULL);`),
		},
	}

	if err := Run(context.Background(), db, fsys); err != nil {
		t.Fatalf("first Run() error = %v", err)
	}

	if err := Run(context.Background(), db, fsys); err != nil {
		t.Fatalf("second Run() error = %v", err)
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(1) FROM schema_migrations`).Scan(&count); err != nil {
		t.Fatalf("count schema_migrations error = %v", err)
	}

	if count != 1 {
		t.Fatalf("schema_migrations count = %d, want 1", count)
	}
}
