package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

// Task is the database-backed representation of a task record.
type Task struct {
	ID           int64
	UUID         string
	Handle       string
	Title        string
	Description  string
	Status       string
	DomainUUID   *string
	ProjectUUID  *string
	AssigneeUUID *string
	DueAt        *string
	Tags         []string
	CreatedAt    string
	UpdatedAt    string
	ClosedAt     *string
}

// TaskCreateInput describes the minimal fields required to create a task.
type TaskCreateInput struct {
	Title       string
	Description string
	ActorID     *int64
}

// TaskUpdateInput describes mutable task fields.
type TaskUpdateInput struct {
	Reference   string
	Title       *string
	Description *string
	Tags        *[]string
	DomainRef   *string
	ProjectRef  *string
	DueAt       *string
	Status      *string
	ActorID     *int64
}

// CreateTask inserts a new backlog task and its corresponding event.
func CreateTask(ctx context.Context, db *sql.DB, input TaskCreateInput) (Task, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return Task{}, fmt.Errorf("begin create task transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	handle, err := nextHandle(ctx, tx, "task", "TASK")
	if err != nil {
		return Task{}, err
	}

	taskUUID := uuid.NewString()
	result, err := tx.ExecContext(ctx, `
		INSERT INTO tasks(uuid, handle, title, description, status, tags)
		VALUES (?, ?, ?, ?, 'backlog', '[]')
	`, taskUUID, handle, input.Title, input.Description)
	if err != nil {
		return Task{}, fmt.Errorf("insert task: %w", err)
	}

	taskID, err := result.LastInsertId()
	if err != nil {
		return Task{}, fmt.Errorf("read task insert id: %w", err)
	}

	task, err := findTaskByIDTx(ctx, tx, taskID)
	if err != nil {
		return Task{}, err
	}

	if err := appendEventTx(ctx, tx, eventInput{
		EntityType: "task",
		EntityUUID: task.UUID,
		ActorID:    input.ActorID,
		EventType:  "create",
		Payload: map[string]any{
			"title":  task.Title,
			"status": task.Status,
		},
	}); err != nil {
		return Task{}, err
	}

	if err := tx.Commit(); err != nil {
		return Task{}, fmt.Errorf("commit create task transaction: %w", err)
	}

	return task, nil
}

// ListTasks returns tasks ordered by most recently updated first.
func ListTasks(ctx context.Context, db *sql.DB) ([]Task, error) {
	rows, err := db.QueryContext(ctx, taskSelectQuery+` ORDER BY t.updated_at DESC, t.id DESC`)
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	defer rows.Close()

	var tasks []Task
	for rows.Next() {
		task, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tasks: %w", err)
	}

	return tasks, nil
}

// FindTask resolves a task by handle or UUID.
func FindTask(ctx context.Context, db *sql.DB, reference string) (Task, error) {
	return queryTask(ctx, db, taskSelectQuery+` WHERE t.handle = ? OR t.uuid = ?`, reference, reference)
}

// UpdateTask mutates task fields and records an update or status_change event.
func UpdateTask(ctx context.Context, db *sql.DB, input TaskUpdateInput) (Task, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return Task{}, fmt.Errorf("begin update task transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	current, err := findTaskTx(ctx, tx, input.Reference)
	if err != nil {
		return Task{}, err
	}
	if input.ActorID == nil {
		return Task{}, ErrClaimRequired
	}
	if err := RequireClaimHeld(ctx, tx, current.ID, *input.ActorID); err != nil {
		return Task{}, err
	}

	nextTask := current

	if input.Title != nil {
		nextTask.Title = *input.Title
	}
	if input.Description != nil {
		nextTask.Description = *input.Description
	}
	if input.Tags != nil {
		nextTask.Tags = *input.Tags
	}
	if input.DomainRef != nil {
		nextTask.DomainUUID, err = resolveNullableEntityUUID(ctx, tx, "domains", *input.DomainRef)
		if err != nil {
			return Task{}, err
		}
	}
	if input.ProjectRef != nil {
		nextTask.ProjectUUID, err = resolveNullableEntityUUID(ctx, tx, "projects", *input.ProjectRef)
		if err != nil {
			return Task{}, err
		}
	}
	if input.DueAt != nil {
		nextTask.DueAt = nullableStringPointer(*input.DueAt)
	}
	if input.Status != nil {
		nextTask.Status = *input.Status
	}

	if err := validateTaskTransition(current.Status, nextTask.Status); err != nil {
		return Task{}, err
	}

	if isClosedStatus(nextTask.Status) {
		now := "CURRENT_TIMESTAMP"
		_ = now
		if current.ClosedAt == nil {
			value := "__set_by_sql__"
			nextTask.ClosedAt = &value
		}
	} else {
		nextTask.ClosedAt = nil
	}

	tagsJSON, err := json.Marshal(nextTask.Tags)
	if err != nil {
		return Task{}, fmt.Errorf("marshal task tags: %w", err)
	}

	domainID, err := resolveEntityIDForUUID(ctx, tx, "domains", nextTask.DomainUUID)
	if err != nil {
		return Task{}, err
	}
	projectID, err := resolveEntityIDForUUID(ctx, tx, "projects", nextTask.ProjectUUID)
	if err != nil {
		return Task{}, err
	}

	query := `
		UPDATE tasks
		SET title = ?, description = ?, status = ?, domain_id = ?, project_id = ?, due_at = ?, tags = ?,
		    updated_at = CURRENT_TIMESTAMP,
		    closed_at = CASE
		      WHEN ? IN ('completed', 'cancelled') THEN COALESCE(closed_at, CURRENT_TIMESTAMP)
		      ELSE NULL
		    END
		WHERE id = ?
	`

	if _, err := tx.ExecContext(
		ctx,
		query,
		nextTask.Title,
		nextTask.Description,
		nextTask.Status,
		domainID,
		projectID,
		nextTask.DueAt,
		string(tagsJSON),
		nextTask.Status,
		current.ID,
	); err != nil {
		return Task{}, fmt.Errorf("update task: %w", err)
	}

	updated, err := findTaskByIDTx(ctx, tx, current.ID)
	if err != nil {
		return Task{}, err
	}

	eventType := "update"
	payload := map[string]any{
		"title":       updated.Title,
		"description": updated.Description,
		"tags":        updated.Tags,
		"domain_id":   updated.DomainUUID,
		"project_id":  updated.ProjectUUID,
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
		EntityType: "task",
		EntityUUID: updated.UUID,
		ActorID:    input.ActorID,
		EventType:  eventType,
		Payload:    payload,
	}); err != nil {
		return Task{}, err
	}

	if err := tx.Commit(); err != nil {
		return Task{}, fmt.Errorf("commit update task transaction: %w", err)
	}

	return updated, nil
}

func validateTaskTransition(from string, to string) error {
	if from == to {
		return nil
	}

	switch from {
	case "backlog":
		if to == "active" || to == "paused" || to == "blocked" {
			return nil
		}
	case "active", "paused", "blocked":
		if to == "backlog" || to == "active" || to == "paused" || to == "blocked" || to == "completed" || to == "cancelled" {
			return nil
		}
	case "completed", "cancelled":
		if to == "active" || to == "paused" || to == "blocked" {
			return nil
		}
	}

	return fmt.Errorf("invalid task status transition from %s to %s", from, to)
}

func isClosedStatus(status string) bool {
	return status == "completed" || status == "cancelled"
}

type eventInput struct {
	EntityType string
	EntityUUID string
	ActorID    *int64
	EventType  string
	Payload    map[string]any
}

func appendEventTx(ctx context.Context, tx *sql.Tx, input eventInput) error {
	payloadJSON, err := json.Marshal(input.Payload)
	if err != nil {
		return fmt.Errorf("marshal event payload: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO events(uuid, entity_type, entity_uuid, actor_id, event_type, payload_json)
		VALUES (?, ?, ?, ?, ?, ?)
	`, uuid.NewString(), input.EntityType, input.EntityUUID, input.ActorID, input.EventType, string(payloadJSON))
	if err != nil {
		return fmt.Errorf("insert event: %w", err)
	}

	return nil
}

const taskSelectQuery = `
	SELECT t.id, t.uuid, t.handle, t.title, t.description, t.status,
	       d.uuid, p.uuid, a.uuid, t.due_at, t.tags, t.created_at, t.updated_at, t.closed_at
	FROM tasks t
	LEFT JOIN domains d ON d.id = t.domain_id
	LEFT JOIN projects p ON p.id = t.project_id
	LEFT JOIN actors a ON a.id = t.assignee_actor_id
`

func findTaskTx(ctx context.Context, tx *sql.Tx, reference string) (Task, error) {
	return queryTask(ctx, tx, taskSelectQuery+` WHERE t.handle = ? OR t.uuid = ?`, reference, reference)
}

func findTaskByIDTx(ctx context.Context, tx *sql.Tx, id int64) (Task, error) {
	return queryTask(ctx, tx, taskSelectQuery+` WHERE t.id = ?`, id)
}

type taskRowScanner interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func queryTask(ctx context.Context, runner taskRowScanner, query string, args ...any) (Task, error) {
	return scanTask(runner.QueryRowContext(ctx, query, args...))
}

func scanTask(scanner interface{ Scan(...any) error }) (Task, error) {
	var task Task
	var domainUUID sql.NullString
	var projectUUID sql.NullString
	var assigneeUUID sql.NullString
	var dueAt sql.NullString
	var tagsJSON string
	var closedAt sql.NullString

	if err := scanner.Scan(
		&task.ID,
		&task.UUID,
		&task.Handle,
		&task.Title,
		&task.Description,
		&task.Status,
		&domainUUID,
		&projectUUID,
		&assigneeUUID,
		&dueAt,
		&tagsJSON,
		&task.CreatedAt,
		&task.UpdatedAt,
		&closedAt,
	); err != nil {
		return Task{}, err
	}

	if tagsJSON == "" {
		tagsJSON = "[]"
	}
	if err := json.Unmarshal([]byte(tagsJSON), &task.Tags); err != nil {
		return Task{}, fmt.Errorf("unmarshal task tags: %w", err)
	}

	task.DomainUUID = nullableStringFromNull(domainUUID)
	task.ProjectUUID = nullableStringFromNull(projectUUID)
	task.AssigneeUUID = nullableStringFromNull(assigneeUUID)
	task.DueAt = nullableStringFromNull(dueAt)
	task.ClosedAt = nullableStringFromNull(closedAt)

	return task, nil
}

func resolveNullableEntityUUID(ctx context.Context, tx *sql.Tx, table string, reference string) (*string, error) {
	if reference == "" {
		return nil, nil
	}

	var resolved string
	query := fmt.Sprintf(`SELECT uuid FROM %s WHERE handle = ? OR uuid = ?`, table)
	if err := tx.QueryRowContext(ctx, query, reference, reference).Scan(&resolved); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%s reference %q not found", table, reference)
		}
		return nil, fmt.Errorf("resolve %s reference %q: %w", table, reference, err)
	}

	return &resolved, nil
}

func resolveEntityIDForUUID(ctx context.Context, tx *sql.Tx, table string, value *string) (any, error) {
	if value == nil {
		return nil, nil
	}

	var id int64
	query := fmt.Sprintf(`SELECT id FROM %s WHERE uuid = ?`, table)
	if err := tx.QueryRowContext(ctx, query, *value).Scan(&id); err != nil {
		return nil, fmt.Errorf("resolve %s id for uuid %q: %w", table, *value, err)
	}

	return id, nil
}

func nullableStringFromNull(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	v := value.String
	return &v
}

func nullableStringPointer(value string) *string {
	if value == "" {
		return nil
	}
	v := value
	return &v
}
