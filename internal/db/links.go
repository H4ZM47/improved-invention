package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

const (
	LinkTypeFile       = "file"
	LinkTypeURL        = "url"
	LinkTypeRepo       = "repo"
	LinkTypeWorktree   = "worktree"
	LinkTypeObsidian   = "obsidian"
	LinkTypeOther      = "other"
	LinkTargetTask     = "task"
	LinkTargetExternal = "external"
)

var (
	// ErrInvalidLinkType indicates an unsupported external link type.
	ErrInvalidLinkType = errors.New("invalid external link type")
)

// ExternalLink is the database-backed representation of a task-scoped external link.
type ExternalLink struct {
	ID           int64
	UUID         string
	TaskUUID     string
	TaskHandle   string
	LinkType     string
	Target       string
	Label        string
	MetadataJSON string
	CreatedAt    string
}

// TaskExternalLinkCreateInput describes the fields needed to create a task-scoped external link.
type TaskExternalLinkCreateInput struct {
	TaskReference string
	LinkType      string
	Target        string
	Label         string
	MetadataJSON  string
	ActorID       *int64
}

// TaskExternalLinkRemoveInput describes the identifying fields for link removal.
type TaskExternalLinkRemoveInput struct {
	TaskReference string
	LinkUUID      string
	ActorID       *int64
}

// CreateExternalLink inserts a task-scoped external link and records an event on the task.
func CreateExternalLink(ctx context.Context, db *sql.DB, input TaskExternalLinkCreateInput) (ExternalLink, error) {
	if err := validateLinkType(input.LinkType); err != nil {
		return ExternalLink{}, err
	}

	metadataJSON := input.MetadataJSON
	if metadataJSON == "" {
		metadataJSON = "{}"
	}
	if !json.Valid([]byte(metadataJSON)) {
		return ExternalLink{}, fmt.Errorf("invalid metadata json")
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return ExternalLink{}, fmt.Errorf("begin create external link transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	task, err := findTaskTx(ctx, tx, input.TaskReference)
	if err != nil {
		return ExternalLink{}, err
	}

	result, err := tx.ExecContext(ctx, `
		INSERT INTO links(uuid, source_task_id, link_type, target_kind, target_value, label, metadata_json)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, uuid.NewString(), task.ID, input.LinkType, LinkTargetExternal, input.Target, input.Label, metadataJSON)
	if err != nil {
		return ExternalLink{}, fmt.Errorf("insert external link: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return ExternalLink{}, fmt.Errorf("read external link insert id: %w", err)
	}

	link, err := findExternalLinkByIDTx(ctx, tx, id)
	if err != nil {
		return ExternalLink{}, err
	}

	if err := appendEventTx(ctx, tx, eventInput{
		EntityType: "task",
		EntityUUID: task.UUID,
		ActorID:    input.ActorID,
		EventType:  "external_link_add",
		Payload: map[string]any{
			"link_uuid":     link.UUID,
			"link_type":     link.LinkType,
			"target":        link.Target,
			"label":         link.Label,
			"metadata_json": link.MetadataJSON,
		},
	}); err != nil {
		return ExternalLink{}, err
	}

	if err := tx.Commit(); err != nil {
		return ExternalLink{}, fmt.Errorf("commit create external link transaction: %w", err)
	}

	return link, nil
}

// ListExternalLinksForTask returns task-scoped links ordered by most recently created first.
func ListExternalLinksForTask(ctx context.Context, db *sql.DB, taskReference string) ([]ExternalLink, error) {
	task, err := FindTask(ctx, db, taskReference)
	if err != nil {
		return nil, err
	}

	rows, err := db.QueryContext(ctx, externalLinkSelectQuery+`
		WHERE l.source_task_id = ? AND l.target_kind = ?
		ORDER BY l.created_at DESC, l.id DESC
	`, task.ID, LinkTargetExternal)
	if err != nil {
		return nil, fmt.Errorf("list external links: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	var links []ExternalLink
	for rows.Next() {
		link, err := scanExternalLink(rows)
		if err != nil {
			return nil, err
		}
		links = append(links, link)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate external links: %w", err)
	}

	return links, nil
}

// ListExternalLinks returns every external task link ordered by most recently created first.
func ListExternalLinks(ctx context.Context, db *sql.DB) ([]ExternalLink, error) {
	rows, err := db.QueryContext(ctx, externalLinkSelectQuery+`
		WHERE l.target_kind = ?
		ORDER BY l.created_at DESC, l.id DESC
	`, LinkTargetExternal)
	if err != nil {
		return nil, fmt.Errorf("list external links: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	var links []ExternalLink
	for rows.Next() {
		link, err := scanExternalLink(rows)
		if err != nil {
			return nil, err
		}
		links = append(links, link)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate external links: %w", err)
	}

	return links, nil
}

// RemoveExternalLink deletes a task-scoped link and records an event on the task.
func RemoveExternalLink(ctx context.Context, db *sql.DB, input TaskExternalLinkRemoveInput) (ExternalLink, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return ExternalLink{}, fmt.Errorf("begin remove external link transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	task, err := findTaskTx(ctx, tx, input.TaskReference)
	if err != nil {
		return ExternalLink{}, err
	}

	link, err := queryExternalLink(ctx, tx, externalLinkSelectQuery+`
		WHERE l.source_task_id = ? AND l.target_kind = ? AND l.uuid = ?
	`, task.ID, LinkTargetExternal, input.LinkUUID)
	if err != nil {
		return ExternalLink{}, err
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM links WHERE id = ?`, link.ID); err != nil {
		return ExternalLink{}, fmt.Errorf("delete external link: %w", err)
	}

	if err := appendEventTx(ctx, tx, eventInput{
		EntityType: "task",
		EntityUUID: task.UUID,
		ActorID:    input.ActorID,
		EventType:  "external_link_remove",
		Payload: map[string]any{
			"link_uuid":     link.UUID,
			"link_type":     link.LinkType,
			"target":        link.Target,
			"label":         link.Label,
			"metadata_json": link.MetadataJSON,
		},
	}); err != nil {
		return ExternalLink{}, err
	}

	if err := tx.Commit(); err != nil {
		return ExternalLink{}, fmt.Errorf("commit remove external link transaction: %w", err)
	}

	return link, nil
}

const externalLinkSelectQuery = `
	SELECT l.id, l.uuid, t.uuid, t.handle, l.link_type, l.target_value, l.label, l.metadata_json, l.created_at
	FROM links l
	INNER JOIN tasks t ON t.id = l.source_task_id
`

type externalLinkRowScanner interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func findExternalLinkByIDTx(ctx context.Context, tx *sql.Tx, id int64) (ExternalLink, error) {
	return queryExternalLink(ctx, tx, externalLinkSelectQuery+` WHERE l.id = ?`, id)
}

func queryExternalLink(ctx context.Context, runner externalLinkRowScanner, query string, args ...any) (ExternalLink, error) {
	return scanExternalLink(runner.QueryRowContext(ctx, query, args...))
}

func scanExternalLink(scanner interface{ Scan(...any) error }) (ExternalLink, error) {
	var link ExternalLink
	if err := scanner.Scan(
		&link.ID,
		&link.UUID,
		&link.TaskUUID,
		&link.TaskHandle,
		&link.LinkType,
		&link.Target,
		&link.Label,
		&link.MetadataJSON,
		&link.CreatedAt,
	); err != nil {
		return ExternalLink{}, err
	}

	return link, nil
}

func validateLinkType(value string) error {
	switch value {
	case LinkTypeFile,
		LinkTypeURL,
		LinkTypeRepo,
		LinkTypeWorktree,
		LinkTypeObsidian,
		LinkTypeOther:
		return nil
	default:
		return fmt.Errorf("%w: %s", ErrInvalidLinkType, value)
	}
}
