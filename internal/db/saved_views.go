package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// ErrSavedViewNotFound is returned when a named saved view does not exist.
var ErrSavedViewNotFound = errors.New("saved view not found")

// ErrSavedViewNameTaken is returned when a create or update conflicts with an existing name.
var ErrSavedViewNameTaken = errors.New("saved view name is already in use")

// SavedView is the database representation of a named saved list query.
type SavedView struct {
	ID          int64
	UUID        string
	Name        string
	EntityType  string
	FiltersJSON string
	CreatedAt   string
	UpdatedAt   string
}

// SavedViewInput carries the user-facing fields shared between create and update.
type SavedViewInput struct {
	Name        string
	EntityType  string
	FiltersJSON string
}

const savedViewSelectColumns = `id, uuid, name, entity_type, filters_json, created_at, updated_at`

// CreateSavedView inserts a new saved view.
func CreateSavedView(ctx context.Context, db *sql.DB, input SavedViewInput) (SavedView, error) {
	normalized, err := normalizeSavedViewInput(input)
	if err != nil {
		return SavedView{}, err
	}

	res, err := db.ExecContext(ctx, `
		INSERT INTO saved_views(uuid, name, entity_type, filters_json)
		VALUES (?, ?, ?, ?)
	`, uuid.NewString(), normalized.Name, normalized.EntityType, normalized.FiltersJSON)
	if err != nil {
		if isSavedViewNameConflict(err) {
			return SavedView{}, fmt.Errorf("%w: %q", ErrSavedViewNameTaken, normalized.Name)
		}
		return SavedView{}, fmt.Errorf("insert saved view: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return SavedView{}, fmt.Errorf("read saved view insert id: %w", err)
	}

	return findSavedViewByID(ctx, db, id)
}

// ListSavedViews returns saved views ordered alphabetically by name.
func ListSavedViews(ctx context.Context, db *sql.DB) ([]SavedView, error) {
	rows, err := db.QueryContext(ctx, `SELECT `+savedViewSelectColumns+` FROM saved_views ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("list saved views: %w", err)
	}
	defer rows.Close()

	var views []SavedView
	for rows.Next() {
		view, err := scanSavedView(rows)
		if err != nil {
			return nil, err
		}
		views = append(views, view)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate saved views: %w", err)
	}
	return views, nil
}

// FindSavedView resolves a saved view by name.
func FindSavedView(ctx context.Context, db *sql.DB, name string) (SavedView, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return SavedView{}, fmt.Errorf("%w: name is required", ErrSavedViewNotFound)
	}

	row := db.QueryRowContext(ctx, `SELECT `+savedViewSelectColumns+` FROM saved_views WHERE name = ?`, trimmed)
	view, err := scanSavedView(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return SavedView{}, fmt.Errorf("%w: %q", ErrSavedViewNotFound, trimmed)
		}
		return SavedView{}, err
	}
	return view, nil
}

// UpdateSavedView replaces the filters of an existing saved view.
//
// The lookup happens by current name; the new name (if different) takes effect
// atomically with the filters.
func UpdateSavedView(ctx context.Context, db *sql.DB, currentName string, input SavedViewInput) (SavedView, error) {
	trimmedCurrent := strings.TrimSpace(currentName)
	if trimmedCurrent == "" {
		return SavedView{}, fmt.Errorf("%w: name is required", ErrSavedViewNotFound)
	}

	normalized, err := normalizeSavedViewInput(input)
	if err != nil {
		return SavedView{}, err
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return SavedView{}, fmt.Errorf("begin update saved view transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var id int64
	if err := tx.QueryRowContext(ctx, `SELECT id FROM saved_views WHERE name = ?`, trimmedCurrent).Scan(&id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return SavedView{}, fmt.Errorf("%w: %q", ErrSavedViewNotFound, trimmedCurrent)
		}
		return SavedView{}, fmt.Errorf("find saved view: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE saved_views
		SET name = ?, entity_type = ?, filters_json = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, normalized.Name, normalized.EntityType, normalized.FiltersJSON, id); err != nil {
		if isSavedViewNameConflict(err) {
			return SavedView{}, fmt.Errorf("%w: %q", ErrSavedViewNameTaken, normalized.Name)
		}
		return SavedView{}, fmt.Errorf("update saved view: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return SavedView{}, fmt.Errorf("commit update saved view transaction: %w", err)
	}

	return findSavedViewByID(ctx, db, id)
}

// DeleteSavedView removes a saved view by name.
func DeleteSavedView(ctx context.Context, db *sql.DB, name string) error {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return fmt.Errorf("%w: name is required", ErrSavedViewNotFound)
	}

	res, err := db.ExecContext(ctx, `DELETE FROM saved_views WHERE name = ?`, trimmed)
	if err != nil {
		return fmt.Errorf("delete saved view: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("read saved view delete rows: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("%w: %q", ErrSavedViewNotFound, trimmed)
	}
	return nil
}

func findSavedViewByID(ctx context.Context, db *sql.DB, id int64) (SavedView, error) {
	row := db.QueryRowContext(ctx, `SELECT `+savedViewSelectColumns+` FROM saved_views WHERE id = ?`, id)
	return scanSavedView(row)
}

func scanSavedView(scanner interface{ Scan(...any) error }) (SavedView, error) {
	var v SavedView
	if err := scanner.Scan(&v.ID, &v.UUID, &v.Name, &v.EntityType, &v.FiltersJSON, &v.CreatedAt, &v.UpdatedAt); err != nil {
		return SavedView{}, err
	}
	return v, nil
}

func normalizeSavedViewInput(input SavedViewInput) (SavedViewInput, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return SavedViewInput{}, errors.New("saved view name is required")
	}
	entityType := strings.TrimSpace(input.EntityType)
	if entityType == "" {
		entityType = "task"
	}

	filters := strings.TrimSpace(input.FiltersJSON)
	if filters == "" {
		filters = "{}"
	}
	if !json.Valid([]byte(filters)) {
		return SavedViewInput{}, errors.New("saved view filters must be valid JSON")
	}

	return SavedViewInput{Name: name, EntityType: entityType, FiltersJSON: filters}, nil
}

func isSavedViewNameConflict(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed: saved_views.name")
}
