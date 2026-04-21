package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	taskconfig "github.com/H4ZM47/task-cli/internal/config"
)

// BackupDatabase writes a consistent full-fidelity SQLite artifact to outputPath.
func BackupDatabase(ctx context.Context, db *sql.DB, outputPath string) error {
	outputPath = strings.TrimSpace(outputPath)
	if outputPath == "" {
		return errors.New("backup output path is required")
	}
	if _, err := os.Stat(outputPath); err == nil {
		return fmt.Errorf("backup output %s already exists", outputPath)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("stat backup output %s: %w", outputPath, err)
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return fmt.Errorf("create backup directory: %w", err)
	}

	quoted := "'" + strings.ReplaceAll(outputPath, "'", "''") + "'"
	if _, err := db.ExecContext(ctx, "VACUUM INTO "+quoted); err != nil {
		return fmt.Errorf("vacuum into backup artifact: %w", err)
	}
	return nil
}

// RestoreDatabase replaces cfg.DBPath with the backup artifact at inputPath.
func RestoreDatabase(ctx context.Context, inputPath string, cfg taskconfig.Resolved, force bool) error {
	inputPath = strings.TrimSpace(inputPath)
	if inputPath == "" {
		return errors.New("restore input path is required")
	}
	targetPath := strings.TrimSpace(cfg.DBPath)
	if targetPath == "" {
		return errors.New("restore target database path is required")
	}

	inputAbs, err := filepath.Abs(inputPath)
	if err != nil {
		return fmt.Errorf("resolve restore input path: %w", err)
	}
	targetAbs, err := filepath.Abs(targetPath)
	if err != nil {
		return fmt.Errorf("resolve restore target path: %w", err)
	}
	if inputAbs == targetAbs {
		return errors.New("restore input and target database paths must differ")
	}

	if err := validateBackupArtifact(ctx, inputAbs); err != nil {
		return err
	}

	if _, err := os.Stat(targetAbs); err == nil && !force {
		return fmt.Errorf("restore target %s already exists; rerun with --force to overwrite", targetAbs)
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("stat restore target %s: %w", targetAbs, err)
	}

	if err := os.MkdirAll(filepath.Dir(targetAbs), 0o755); err != nil {
		return fmt.Errorf("create restore target directory: %w", err)
	}

	tmpPath := targetAbs + ".restore-tmp"
	if err := removeSQLiteArtifacts(tmpPath); err != nil {
		return err
	}
	if err := copyFile(inputAbs, tmpPath); err != nil {
		return err
	}
	defer func() {
		_ = removeSQLiteArtifacts(tmpPath)
	}()

	if err := removeSQLiteArtifacts(targetAbs); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, targetAbs); err != nil {
		return fmt.Errorf("move restored database into place: %w", err)
	}

	opened, err := Open(ctx, cfg)
	if err != nil {
		return fmt.Errorf("open restored database: %w", err)
	}
	if err := opened.Close(); err != nil {
		return fmt.Errorf("close restored database: %w", err)
	}

	return nil
}

func validateBackupArtifact(ctx context.Context, path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("backup artifact %s does not exist", path)
		}
		return fmt.Errorf("stat backup artifact %s: %w", path, err)
	}
	if info.IsDir() {
		return fmt.Errorf("backup artifact %s is a directory", path)
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return fmt.Errorf("open backup artifact: %w", err)
	}
	defer db.Close()

	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("ping backup artifact: %w", err)
	}

	requiredTables := []string{
		"actors",
		"domains",
		"projects",
		"tasks",
		"claims",
		"relationships",
		"external_links",
		"events",
		"saved_views",
		"handle_sequences",
	}
	for _, table := range requiredTables {
		var got string
		if err := db.QueryRowContext(
			ctx,
			`SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?`,
			table,
		).Scan(&got); err != nil {
			return fmt.Errorf("validate backup artifact table %s: %w", table, err)
		}
	}

	return nil
}

func removeSQLiteArtifacts(path string) error {
	for _, candidate := range []string{path, path + "-wal", path + "-shm"} {
		if err := os.Remove(candidate); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove %s: %w", candidate, err)
		}
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open restore input %s: %w", src, err)
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("create restore target %s: %w", dst, err)
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("copy backup artifact to %s: %w", dst, err)
	}
	if err := out.Sync(); err != nil {
		return fmt.Errorf("sync restore target %s: %w", dst, err)
	}
	return nil
}
