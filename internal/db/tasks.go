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

// Task is the database-backed representation of a task record.
type Task struct {
	ID              int64
	UUID            string
	Handle          string
	Title           string
	Description     string
	Status          string
	DomainUUID      *string
	ProjectUUID     *string
	MilestoneUUID   *string
	MilestoneHandle *string
	AssigneeUUID    *string
	DueAt           *string
	Tags            []string
	CreatedAt       string
	UpdatedAt       string
	ClosedAt        *string
}

// TaskCreateInput describes the minimal fields required to create a task.
type TaskCreateInput struct {
	Title        string
	Description  string
	Tags         []string
	DomainRef    *string
	ProjectRef   *string
	MilestoneRef *string
	AssigneeRef  *string
	DueAt        *string
	ActorID      *int64
}

// TaskUpdateInput describes mutable task fields.
type TaskUpdateInput struct {
	Reference    string
	Title        *string
	Description  *string
	Tags         *[]string
	DomainRef    *string
	ProjectRef   *string
	MilestoneRef *string
	AssigneeRef  *string
	DueAt        *string
	Status       *string
	ActorID      *int64
}

// TaskListQuery describes the supported task-list filters in v1.
type TaskListQuery struct {
	Statuses       []string
	DomainRef      *string
	ProjectRef     *string
	MilestoneRef   *string
	AssigneeRef    *string
	DueBefore      *string
	DueAfter       *string
	Tags           []string
	Search         string
	RepoTarget     *string
	WorktreeTarget *string
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

	classification, err := resolveTaskClassificationTx(ctx, tx, nil, nil, input.DomainRef, input.ProjectRef)
	if err != nil {
		return Task{}, err
	}
	assigneeID, err := resolveNullableActorIDTx(ctx, tx, input.AssigneeRef)
	if err != nil {
		return Task{}, err
	}
	milestoneID, _, _, err := resolveNullableMilestoneIDTx(ctx, tx, input.MilestoneRef)
	if err != nil {
		return Task{}, err
	}
	tagsJSON, err := json.Marshal(input.Tags)
	if err != nil {
		return Task{}, fmt.Errorf("marshal task tags: %w", err)
	}

	taskUUID := uuid.NewString()
	result, err := tx.ExecContext(ctx, `
		INSERT INTO tasks(uuid, handle, title, description, status, domain_id, project_id, milestone_id, assignee_actor_id, due_at, tags)
		VALUES (?, ?, ?, ?, 'backlog', ?, ?, ?, ?, ?, ?)
	`, taskUUID, handle, input.Title, input.Description, classification.domainID, classification.projectID, milestoneID, assigneeID, nullableValue(input.DueAt), string(tagsJSON))
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
			"title":        task.Title,
			"status":       task.Status,
			"domain_id":    task.DomainUUID,
			"project_id":   task.ProjectUUID,
			"milestone_id": task.MilestoneUUID,
			"assignee_id":  task.AssigneeUUID,
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
func ListTasks(ctx context.Context, db *sql.DB, query TaskListQuery) ([]Task, error) {
	statement, args := buildTaskListQuery(query)

	rows, err := db.QueryContext(ctx, statement, args...)
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

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

func buildTaskListQuery(query TaskListQuery) (string, []any) {
	statement := taskSelectQuery
	where := make([]string, 0, 8)
	args := make([]any, 0, 16)

	statuses := compactStrings(query.Statuses)
	if len(statuses) > 0 {
		placeholders := make([]string, 0, len(statuses))
		for _, status := range statuses {
			placeholders = append(placeholders, "?")
			args = append(args, status)
		}
		where = append(where, "t.status IN ("+strings.Join(placeholders, ", ")+")")
	}

	if ref := trimmedPointer(query.DomainRef); ref != nil {
		where = append(where, "(d.handle = ? OR d.uuid = ?)")
		args = append(args, *ref, *ref)
	}
	if ref := trimmedPointer(query.ProjectRef); ref != nil {
		where = append(where, "(p.handle = ? OR p.uuid = ?)")
		args = append(args, *ref, *ref)
	}
	if ref := trimmedPointer(query.MilestoneRef); ref != nil {
		where = append(where, "(m.handle = ? OR m.uuid = ?)")
		args = append(args, *ref, *ref)
	}
	if ref := trimmedPointer(query.AssigneeRef); ref != nil {
		where = append(where, "(a.handle = ? OR a.uuid = ?)")
		args = append(args, *ref, *ref)
	}
	if ref := trimmedPointer(query.DueBefore); ref != nil {
		where = append(where, "t.due_at IS NOT NULL AND t.due_at <= ?")
		args = append(args, *ref)
	}
	if ref := trimmedPointer(query.DueAfter); ref != nil {
		where = append(where, "t.due_at IS NOT NULL AND t.due_at >= ?")
		args = append(args, *ref)
	}

	for _, tag := range compactStrings(query.Tags) {
		where = append(where, "EXISTS (SELECT 1 FROM json_each(t.tags) WHERE json_each.value = ?)")
		args = append(args, tag)
	}

	if search := strings.TrimSpace(query.Search); search != "" {
		pattern := "%" + strings.ToLower(search) + "%"
		where = append(where, "(LOWER(t.title) LIKE ? OR LOWER(t.description) LIKE ?)")
		args = append(args, pattern, pattern)
	}

	repoContextClauses := make([]string, 0, 2)
	if ref := trimmedPointer(query.RepoTarget); ref != nil {
		repoContextClauses = append(repoContextClauses, "EXISTS (SELECT 1 FROM links l WHERE l.source_task_id = t.id AND l.target_kind = 'external' AND l.link_type = 'repo' AND l.target_value = ?)")
		args = append(args, *ref)
	}
	if ref := trimmedPointer(query.WorktreeTarget); ref != nil {
		repoContextClauses = append(repoContextClauses, "EXISTS (SELECT 1 FROM links l WHERE l.source_task_id = t.id AND l.target_kind = 'external' AND l.link_type = 'worktree' AND l.target_value = ?)")
		args = append(args, *ref)
	}
	if len(repoContextClauses) > 0 {
		where = append(where, "("+strings.Join(repoContextClauses, " OR ")+")")
	}

	if len(where) > 0 {
		statement += " WHERE " + strings.Join(where, " AND ")
	}
	statement += " ORDER BY t.updated_at DESC, t.id DESC"

	return statement, args
}

func compactStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	result := make([]string, 0, len(values))
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func trimmedPointer(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
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
	if input.DueAt != nil {
		nextTask.DueAt = nullableStringPointer(*input.DueAt)
	}
	if input.Status != nil {
		nextTask.Status = *input.Status
	}
	if input.DomainRef != nil || input.ProjectRef != nil {
		classification, err := resolveTaskClassificationTx(ctx, tx, current.DomainUUID, current.ProjectUUID, input.DomainRef, input.ProjectRef)
		if err != nil {
			return Task{}, err
		}
		nextTask.DomainUUID = classification.domainUUID
		nextTask.ProjectUUID = classification.projectUUID
	}
	milestoneID, err := resolveEntityIDForUUID(ctx, tx, "milestones", current.MilestoneUUID)
	if err != nil {
		return Task{}, err
	}
	if input.MilestoneRef != nil {
		var milestoneUUID *string
		var milestoneHandle *string
		milestoneID, milestoneUUID, milestoneHandle, err = resolveNullableMilestoneIDTx(ctx, tx, input.MilestoneRef)
		if err != nil {
			return Task{}, err
		}
		nextTask.MilestoneUUID = milestoneUUID
		nextTask.MilestoneHandle = milestoneHandle
	}
	assigneeID, err := currentNullableActorID(ctx, tx, current.AssigneeUUID)
	if err != nil {
		return Task{}, err
	}
	if input.AssigneeRef != nil {
		assigneeID, err = resolveNullableActorIDTx(ctx, tx, input.AssigneeRef)
		if err != nil {
			return Task{}, err
		}
	}

	if err := validateLifecycleTransition(current.Status, nextTask.Status); err != nil {
		return Task{}, err
	}

	if input.Status != nil && isClosedStatus(*input.Status) {
		if err := closeTaskSessionIfActiveTx(ctx, tx, current, *input.ActorID); err != nil {
			return Task{}, err
		}
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
		SET title = ?, description = ?, status = ?, domain_id = ?, project_id = ?, milestone_id = ?, assignee_actor_id = ?, due_at = ?, tags = ?,
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
		milestoneID,
		assigneeID,
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
		"title":        updated.Title,
		"description":  updated.Description,
		"tags":         updated.Tags,
		"domain_id":    updated.DomainUUID,
		"project_id":   updated.ProjectUUID,
		"milestone_id": updated.MilestoneUUID,
		"assignee_id":  updated.AssigneeUUID,
		"due_at":       updated.DueAt,
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

func validateLifecycleTransition(from string, to string) error {
	if from == to {
		return nil
	}

	switch from {
	case StatusBacklog:
		if to == StatusActive || to == StatusPaused || to == StatusBlocked || to == StatusCompleted || to == StatusCancelled {
			return nil
		}
	case StatusActive, StatusPaused, StatusBlocked:
		if to == StatusBacklog || to == StatusActive || to == StatusPaused || to == StatusBlocked || to == StatusCompleted || to == StatusCancelled {
			return nil
		}
	case StatusCompleted, StatusCancelled:
		if to == StatusBacklog || to == StatusActive || to == StatusPaused || to == StatusBlocked {
			return nil
		}
	}

	return fmt.Errorf("invalid task status transition from %s to %s", from, to)
}

func isClosedStatus(status string) bool {
	return status == StatusCompleted || status == StatusCancelled
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
	       d.uuid, p.uuid, m.uuid, m.handle, a.uuid, t.due_at, t.tags, t.created_at, t.updated_at, t.closed_at
	FROM tasks t
	LEFT JOIN domains d ON d.id = t.domain_id
	LEFT JOIN projects p ON p.id = t.project_id
	LEFT JOIN milestones m ON m.id = t.milestone_id
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
	var milestoneUUID sql.NullString
	var milestoneHandle sql.NullString
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
		&milestoneUUID,
		&milestoneHandle,
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
	task.MilestoneUUID = nullableStringFromNull(milestoneUUID)
	task.MilestoneHandle = nullableStringFromNull(milestoneHandle)
	task.AssigneeUUID = nullableStringFromNull(assigneeUUID)
	task.DueAt = nullableStringFromNull(dueAt)
	task.ClosedAt = nullableStringFromNull(closedAt)

	return task, nil
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

type taskClassification struct {
	domainID    any
	projectID   any
	domainUUID  *string
	projectUUID *string
}

func resolveTaskClassificationTx(ctx context.Context, tx *sql.Tx, currentDomainUUID *string, currentProjectUUID *string, domainRef *string, projectRef *string) (taskClassification, error) {
	result := taskClassification{
		domainUUID:  currentDomainUUID,
		projectUUID: currentProjectUUID,
	}

	if domainRef != nil {
		if *domainRef == "" {
			result.domainUUID = nil
		} else {
			domain, err := findDomainTx(ctx, tx, *domainRef)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return taskClassification{}, fmt.Errorf("domains reference %q not found", *domainRef)
				}
				return taskClassification{}, err
			}
			result.domainUUID = &domain.UUID
		}
	}

	if projectRef != nil {
		if *projectRef == "" {
			result.projectUUID = nil
		} else {
			project, err := findProjectTx(ctx, tx, *projectRef)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return taskClassification{}, fmt.Errorf("projects reference %q not found", *projectRef)
				}
				return taskClassification{}, err
			}
			result.projectUUID = &project.UUID
		}
	}

	if result.projectUUID != nil {
		project, err := queryProject(ctx, tx, projectSelectQuery+` WHERE p.uuid = ?`, *result.projectUUID)
		if err != nil {
			return taskClassification{}, fmt.Errorf("resolve grind project domain: %w", err)
		}

		if result.domainUUID == nil {
			result.domainUUID = &project.DomainUUID
		} else if *result.domainUUID != project.DomainUUID {
			return taskClassification{}, fmt.Errorf("%w: project %s belongs to domain %s", ErrDomainProjectConstraint, project.Handle, project.DomainHandle)
		}
	}

	domainID, err := resolveEntityIDForUUID(ctx, tx, "domains", result.domainUUID)
	if err != nil {
		return taskClassification{}, err
	}
	projectID, err := resolveEntityIDForUUID(ctx, tx, "projects", result.projectUUID)
	if err != nil {
		return taskClassification{}, err
	}

	result.domainID = domainID
	result.projectID = projectID
	return result, nil
}

func nullableStringPointer(value string) *string {
	if value == "" {
		return nil
	}
	v := value
	return &v
}
