package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

const (
	RelationshipTypeParentChild = "parent_child"
	RelationshipTypeBlocks      = "blocks"
	RelationshipTypeRelatedTo   = "related_to"
	RelationshipTypeSiblingOf   = "sibling_of"
	RelationshipTypeDuplicateOf = "duplicate_of"
	RelationshipTypeSupersedes  = "supersedes"
)

var (
	// ErrInvalidRelationshipType indicates an unsupported relationship type.
	ErrInvalidRelationshipType = errors.New("invalid relationship type")
	// ErrSelfRelationship indicates a task link targeting the same task.
	ErrSelfRelationship = errors.New("self relationship")
)

// Relationship is the database-backed representation of a task relationship.
type Relationship struct {
	ID               int64
	UUID             string
	SourceTaskUUID   string
	SourceTaskHandle string
	TargetTaskUUID   string
	TargetTaskHandle string
	RelationshipType string
	CreatedAt        string
}

// RelationshipCreateInput describes the fields needed to create a relationship.
type RelationshipCreateInput struct {
	SourceTaskReference string
	TargetTaskReference string
	RelationshipType    string
	ActorID             *int64
}

// RelationshipRemoveInput describes the identifying fields for relationship removal.
type RelationshipRemoveInput struct {
	SourceTaskReference string
	TargetTaskReference string
	RelationshipType    string
	ActorID             *int64
}

// CreateRelationship inserts a task relationship and records an event on the source task.
func CreateRelationship(ctx context.Context, db *sql.DB, input RelationshipCreateInput) (Relationship, error) {
	if err := validateRelationshipType(input.RelationshipType); err != nil {
		return Relationship{}, err
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return Relationship{}, fmt.Errorf("begin create relationship transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	source, err := findTaskTx(ctx, tx, input.SourceTaskReference)
	if err != nil {
		return Relationship{}, err
	}
	target, err := findTaskTx(ctx, tx, input.TargetTaskReference)
	if err != nil {
		return Relationship{}, err
	}
	if source.ID == target.ID {
		return Relationship{}, fmt.Errorf("%w: a task link cannot target the same task", ErrSelfRelationship)
	}

	result, err := tx.ExecContext(ctx, `
		INSERT INTO links(uuid, source_task_id, link_type, target_kind, target_task_id, metadata_json)
		VALUES (?, ?, ?, ?, ?, '{}')
	`, uuid.NewString(), source.ID, input.RelationshipType, LinkTargetTask, target.ID)
	if err != nil {
		return Relationship{}, fmt.Errorf("insert relationship: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return Relationship{}, fmt.Errorf("read relationship insert id: %w", err)
	}

	relationship, err := findRelationshipByIDTx(ctx, tx, id)
	if err != nil {
		return Relationship{}, err
	}

	if err := appendEventTx(ctx, tx, eventInput{
		EntityType: "task",
		EntityUUID: source.UUID,
		ActorID:    input.ActorID,
		EventType:  "relationship_add",
		Payload: map[string]any{
			"relationship_uuid": relationship.UUID,
			"relationship_type": relationship.RelationshipType,
			"source_task_id":    relationship.SourceTaskUUID,
			"target_task_id":    relationship.TargetTaskUUID,
		},
	}); err != nil {
		return Relationship{}, err
	}

	if err := tx.Commit(); err != nil {
		return Relationship{}, fmt.Errorf("commit create relationship transaction: %w", err)
	}

	return relationship, nil
}

// ListRelationshipsForTask returns every relationship touching the referenced task.
func ListRelationshipsForTask(ctx context.Context, db *sql.DB, taskReference string) ([]Relationship, error) {
	task, err := FindTask(ctx, db, taskReference)
	if err != nil {
		return nil, err
	}

	rows, err := db.QueryContext(ctx, relationshipSelectQuery+`
		WHERE r.target_kind = ? AND (r.source_task_id = ? OR r.target_task_id = ?)
		ORDER BY r.created_at DESC, r.id DESC
	`, LinkTargetTask, task.ID, task.ID)
	if err != nil {
		return nil, fmt.Errorf("list relationships: %w", err)
	}
	defer rows.Close()

	var relationships []Relationship
	for rows.Next() {
		relationship, err := scanRelationship(rows)
		if err != nil {
			return nil, err
		}
		relationships = append(relationships, relationship)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate relationships: %w", err)
	}

	return relationships, nil
}

// RemoveRelationship deletes a relationship and records an event on the source task.
func RemoveRelationship(ctx context.Context, db *sql.DB, input RelationshipRemoveInput) (Relationship, error) {
	if err := validateRelationshipType(input.RelationshipType); err != nil {
		return Relationship{}, err
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return Relationship{}, fmt.Errorf("begin remove relationship transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	source, err := findTaskTx(ctx, tx, input.SourceTaskReference)
	if err != nil {
		return Relationship{}, err
	}
	target, err := findTaskTx(ctx, tx, input.TargetTaskReference)
	if err != nil {
		return Relationship{}, err
	}

	relationship, err := queryRelationship(ctx, tx, relationshipSelectQuery+`
		WHERE r.target_kind = ? AND r.source_task_id = ? AND r.target_task_id = ? AND r.link_type = ?
	`, LinkTargetTask, source.ID, target.ID, input.RelationshipType)
	if err != nil {
		return Relationship{}, err
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM links WHERE id = ?`, relationship.ID); err != nil {
		return Relationship{}, fmt.Errorf("delete relationship: %w", err)
	}

	if err := appendEventTx(ctx, tx, eventInput{
		EntityType: "task",
		EntityUUID: source.UUID,
		ActorID:    input.ActorID,
		EventType:  "relationship_remove",
		Payload: map[string]any{
			"relationship_uuid": relationship.UUID,
			"relationship_type": relationship.RelationshipType,
			"source_task_id":    relationship.SourceTaskUUID,
			"target_task_id":    relationship.TargetTaskUUID,
		},
	}); err != nil {
		return Relationship{}, err
	}

	if err := tx.Commit(); err != nil {
		return Relationship{}, fmt.Errorf("commit remove relationship transaction: %w", err)
	}

	return relationship, nil
}

const relationshipSelectQuery = `
	SELECT r.id, r.uuid, source.uuid, source.handle, target.uuid, target.handle, r.link_type, r.created_at
	FROM links r
	INNER JOIN tasks source ON source.id = r.source_task_id
	INNER JOIN tasks target ON target.id = r.target_task_id
`

type relationshipRowScanner interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func findRelationshipByIDTx(ctx context.Context, tx *sql.Tx, id int64) (Relationship, error) {
	return queryRelationship(ctx, tx, relationshipSelectQuery+` WHERE r.id = ?`, id)
}

func queryRelationship(ctx context.Context, runner relationshipRowScanner, query string, args ...any) (Relationship, error) {
	return scanRelationship(runner.QueryRowContext(ctx, query, args...))
}

func scanRelationship(scanner interface{ Scan(...any) error }) (Relationship, error) {
	var relationship Relationship
	if err := scanner.Scan(
		&relationship.ID,
		&relationship.UUID,
		&relationship.SourceTaskUUID,
		&relationship.SourceTaskHandle,
		&relationship.TargetTaskUUID,
		&relationship.TargetTaskHandle,
		&relationship.RelationshipType,
		&relationship.CreatedAt,
	); err != nil {
		return Relationship{}, err
	}

	return relationship, nil
}

func validateRelationshipType(value string) error {
	switch value {
	case RelationshipTypeParentChild,
		RelationshipTypeBlocks,
		RelationshipTypeRelatedTo,
		RelationshipTypeSiblingOf,
		RelationshipTypeDuplicateOf,
		RelationshipTypeSupersedes:
		return nil
	default:
		return fmt.Errorf("%w: %s", ErrInvalidRelationshipType, value)
	}
}
