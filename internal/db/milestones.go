package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
)

// Milestone is the database-backed representation of a milestone record.
type Milestone struct {
	ID          int64
	UUID        string
	Handle      string
	Name        string
	Description string
	Status      string
	DueAt       *string
	CreatedAt   string
	UpdatedAt   string
	ClosedAt    *string
}

// MilestoneCreateInput contains mutable fields for milestone creation.
type MilestoneCreateInput struct {
	Name        string
	Description string
	DueAt       *string
	ActorID     *int64
}

// MilestoneUpdateInput contains mutable fields for milestone updates.
type MilestoneUpdateInput struct {
	Reference   string
	Name        *string
	Description *string
	DueAt       *string
	Status      *string
	ActorID     *int64
}

// CreateMilestone inserts a new milestone and corresponding event.
func CreateMilestone(ctx context.Context, db *sql.DB, input MilestoneCreateInput) (Milestone, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return Milestone{}, fmt.Errorf("begin create milestone transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	handle, err := nextHandle(ctx, tx, "milestone", "MILE")
	if err != nil {
		return Milestone{}, err
	}

	result, err := tx.ExecContext(ctx, `
		INSERT INTO milestones(uuid, handle, name, description, status, due_at)
		VALUES (?, ?, ?, ?, 'backlog', ?)
	`, uuid.NewString(), handle, input.Name, input.Description, nullableValue(input.DueAt))
	if err != nil {
		return Milestone{}, fmt.Errorf("insert milestone: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return Milestone{}, fmt.Errorf("read milestone insert id: %w", err)
	}

	milestone, err := findMilestoneByIDTx(ctx, tx, id)
	if err != nil {
		return Milestone{}, err
	}

	if err := appendEventTx(ctx, tx, eventInput{
		EntityType: "milestone",
		EntityUUID: milestone.UUID,
		ActorID:    input.ActorID,
		EventType:  "create",
		Payload: map[string]any{
			"name":   milestone.Name,
			"status": milestone.Status,
			"due_at": milestone.DueAt,
		},
	}); err != nil {
		return Milestone{}, err
	}

	if err := tx.Commit(); err != nil {
		return Milestone{}, fmt.Errorf("commit create milestone transaction: %w", err)
	}

	return milestone, nil
}

// ListMilestones returns milestones ordered by most recently updated first.
func ListMilestones(ctx context.Context, db *sql.DB) ([]Milestone, error) {
	rows, err := db.QueryContext(ctx, milestoneSelectQuery+` ORDER BY m.updated_at DESC, m.id DESC`)
	if err != nil {
		return nil, fmt.Errorf("list milestones: %w", err)
	}
	defer rows.Close()

	var milestones []Milestone
	for rows.Next() {
		milestone, err := scanMilestone(rows)
		if err != nil {
			return nil, err
		}
		milestones = append(milestones, milestone)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate milestones: %w", err)
	}

	return milestones, nil
}

// FindMilestone resolves a milestone by handle or UUID.
func FindMilestone(ctx context.Context, db *sql.DB, reference string) (Milestone, error) {
	return queryMilestone(ctx, db, milestoneSelectQuery+` WHERE m.handle = ? OR m.uuid = ?`, reference, reference)
}

// UpdateMilestone mutates milestone fields and records an event.
func UpdateMilestone(ctx context.Context, db *sql.DB, input MilestoneUpdateInput) (Milestone, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return Milestone{}, fmt.Errorf("begin update milestone transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	current, err := findMilestoneTx(ctx, tx, input.Reference)
	if err != nil {
		return Milestone{}, err
	}

	next := current
	if input.Name != nil {
		next.Name = *input.Name
	}
	if input.Description != nil {
		next.Description = *input.Description
	}
	if input.DueAt != nil {
		next.DueAt = nullableStringPointer(*input.DueAt)
	}
	if input.Status != nil {
		next.Status = *input.Status
	}

	if err := validateLifecycleTransition(current.Status, next.Status); err != nil {
		return Milestone{}, err
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE milestones
		SET name = ?, description = ?, status = ?, due_at = ?,
		    updated_at = CURRENT_TIMESTAMP,
		    closed_at = CASE
		      WHEN ? IN ('completed', 'cancelled') THEN COALESCE(closed_at, CURRENT_TIMESTAMP)
		      ELSE NULL
		    END
		WHERE id = ?
	`, next.Name, next.Description, next.Status, nullableValue(next.DueAt), next.Status, current.ID); err != nil {
		return Milestone{}, fmt.Errorf("update milestone: %w", err)
	}

	updated, err := findMilestoneByIDTx(ctx, tx, current.ID)
	if err != nil {
		return Milestone{}, err
	}

	eventType := "update"
	payload := map[string]any{
		"name":        updated.Name,
		"description": updated.Description,
		"due_at":      updated.DueAt,
	}
	if current.Status != updated.Status {
		eventType = "status_change"
		payload = map[string]any{
			"from": current.Status,
			"to":   updated.Status,
		}
	}

	if err := appendEventTx(ctx, tx, eventInput{
		EntityType: "milestone",
		EntityUUID: updated.UUID,
		ActorID:    input.ActorID,
		EventType:  eventType,
		Payload:    payload,
	}); err != nil {
		return Milestone{}, err
	}

	if err := tx.Commit(); err != nil {
		return Milestone{}, fmt.Errorf("commit update milestone transaction: %w", err)
	}

	return updated, nil
}

const milestoneSelectQuery = `
	SELECT m.id, m.uuid, m.handle, m.name, m.description, m.status,
	       m.due_at, m.created_at, m.updated_at, m.closed_at
	FROM milestones m
`

func findMilestoneTx(ctx context.Context, tx *sql.Tx, reference string) (Milestone, error) {
	return queryMilestone(ctx, tx, milestoneSelectQuery+` WHERE m.handle = ? OR m.uuid = ?`, reference, reference)
}

func findMilestoneByIDTx(ctx context.Context, tx *sql.Tx, id int64) (Milestone, error) {
	return queryMilestone(ctx, tx, milestoneSelectQuery+` WHERE m.id = ?`, id)
}

func queryMilestone(ctx context.Context, runner entityRowScanner, query string, args ...any) (Milestone, error) {
	return scanMilestone(runner.QueryRowContext(ctx, query, args...))
}

func scanMilestone(scanner interface{ Scan(...any) error }) (Milestone, error) {
	var milestone Milestone
	var dueAt sql.NullString
	var closedAt sql.NullString

	if err := scanner.Scan(
		&milestone.ID,
		&milestone.UUID,
		&milestone.Handle,
		&milestone.Name,
		&milestone.Description,
		&milestone.Status,
		&dueAt,
		&milestone.CreatedAt,
		&milestone.UpdatedAt,
		&closedAt,
	); err != nil {
		return Milestone{}, err
	}

	milestone.DueAt = nullableStringFromNull(dueAt)
	milestone.ClosedAt = nullableStringFromNull(closedAt)
	return milestone, nil
}

func resolveNullableMilestoneIDTx(ctx context.Context, tx *sql.Tx, reference *string) (any, *string, *string, error) {
	if reference == nil {
		return nil, nil, nil, nil
	}
	if *reference == "" {
		return nil, nil, nil, nil
	}

	milestone, err := findMilestoneTx(ctx, tx, *reference)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil, nil, fmt.Errorf("milestone reference %q not found", *reference)
		}
		return nil, nil, nil, err
	}

	return milestone.ID, &milestone.UUID, &milestone.Handle, nil
}
