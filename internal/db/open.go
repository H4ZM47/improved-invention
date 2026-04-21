package db

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	taskconfig "github.com/H4ZM47/task-cli/internal/config"
	"github.com/H4ZM47/task-cli/internal/migrate"
	migrationsfs "github.com/H4ZM47/task-cli/migrations"
	_ "modernc.org/sqlite"
)

// Open initializes the SQLite database from resolved runtime configuration.
func Open(ctx context.Context, cfg taskconfig.Resolved) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(cfg.DBPath), 0o755); err != nil {
		return nil, fmt.Errorf("create database directory: %w", err)
	}

	db, err := sql.Open("sqlite", cfg.DBPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite database: %w", err)
	}

	if err := applyRuntimePragmas(ctx, db, cfg); err != nil {
		_ = db.Close()
		return nil, err
	}

	if err := migrate.Run(ctx, db, migrationsfs.Files); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("run migrations: %w", err)
	}
	if err := expireAllStaleClaims(ctx, db); err != nil {
		_ = db.Close()
		return nil, err
	}

	return db, nil
}

func applyRuntimePragmas(ctx context.Context, db *sql.DB, cfg taskconfig.Resolved) error {
	statements := []string{
		fmt.Sprintf("PRAGMA busy_timeout = %d;", cfg.BusyTimeout.Milliseconds()),
		"PRAGMA journal_mode = WAL;",
		"PRAGMA foreign_keys = ON;",
	}

	for _, statement := range statements {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("apply sqlite pragma %q: %w", statement, err)
		}
	}

	return nil
}

func expireAllStaleClaims(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, `
		UPDATE claims
		SET released_at = CURRENT_TIMESTAMP, release_reason = 'expired'
		WHERE released_at IS NULL AND expires_at <= ?
	`, time.Now().UTC().Format(time.RFC3339)); err != nil {
		return fmt.Errorf("expire stale claims on open: %w", err)
	}
	return nil
}
