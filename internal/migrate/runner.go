package migrate

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"
)

// Migration is a discovered SQL migration asset.
type Migration struct {
	Name string
	SQL  string
}

// Discover returns sorted SQL migrations from the provided filesystem.
func Discover(fsys fs.FS) ([]Migration, error) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, fmt.Errorf("read migrations: %w", err)
	}

	migrations := make([]Migration, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || path.Ext(entry.Name()) != ".sql" {
			continue
		}

		body, err := fs.ReadFile(fsys, entry.Name())
		if err != nil {
			return nil, fmt.Errorf("read migration %s: %w", entry.Name(), err)
		}

		migrations = append(migrations, Migration{
			Name: entry.Name(),
			SQL:  string(body),
		})
	}

	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Name < migrations[j].Name
	})

	return migrations, nil
}

// Run ensures the migration tracking table exists and applies any missing migrations.
func Run(ctx context.Context, db *sql.DB, fsys fs.FS) error {
	if _, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			name TEXT PRIMARY KEY,
			applied_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
	`); err != nil {
		return fmt.Errorf("ensure schema_migrations table: %w", err)
	}

	migrations, err := Discover(fsys)
	if err != nil {
		return err
	}

	for _, migration := range migrations {
		applied, err := isApplied(ctx, db, migration.Name)
		if err != nil {
			return err
		}
		if applied {
			continue
		}

		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin migration %s: %w", migration.Name, err)
		}

		if strings.TrimSpace(migration.SQL) != "" {
			if _, err := tx.ExecContext(ctx, migration.SQL); err != nil {
				_ = tx.Rollback()
				return fmt.Errorf("apply migration %s: %w", migration.Name, err)
			}
		}

		if _, err := tx.ExecContext(
			ctx,
			`INSERT INTO schema_migrations(name) VALUES (?)`,
			migration.Name,
		); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("record migration %s: %w", migration.Name, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %s: %w", migration.Name, err)
		}
	}

	return nil
}

func isApplied(ctx context.Context, db *sql.DB, name string) (bool, error) {
	var count int
	if err := db.QueryRowContext(
		ctx,
		`SELECT COUNT(1) FROM schema_migrations WHERE name = ?`,
		name,
	).Scan(&count); err != nil {
		return false, fmt.Errorf("check migration %s: %w", name, err)
	}

	return count > 0, nil
}
