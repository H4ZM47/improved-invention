package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

var (
	// ErrDomainProjectConstraint indicates invalid domain/project containment.
	ErrDomainProjectConstraint = errors.New("domain/project constraint")
)

// Domain is the database-backed representation of a domain record.
type Domain struct {
	ID                    int64
	UUID                  string
	Handle                string
	Name                  string
	Description           string
	Status                string
	DefaultAssigneeUUID   *string
	DefaultAssigneeHandle *string
	AssigneeUUID          *string
	AssigneeHandle        *string
	DueAt                 *string
	Tags                  []string
	CreatedAt             string
	UpdatedAt             string
	ClosedAt              *string
}

// Project is the database-backed representation of a project record.
type Project struct {
	ID                    int64
	UUID                  string
	Handle                string
	Name                  string
	Description           string
	Status                string
	DomainUUID            string
	DomainHandle          string
	DefaultAssigneeUUID   *string
	DefaultAssigneeHandle *string
	AssigneeUUID          *string
	AssigneeHandle        *string
	DueAt                 *string
	Tags                  []string
	CreatedAt             string
	UpdatedAt             string
	ClosedAt              *string
}

// DomainCreateInput contains mutable fields for domain creation.
type DomainCreateInput struct {
	Name               string
	Description        string
	DefaultAssigneeRef *string
	AssigneeRef        *string
	DueAt              *string
	Tags               []string
	ActorID            *int64
}

// DomainUpdateInput contains mutable fields for domain updates.
type DomainUpdateInput struct {
	Reference          string
	Name               *string
	Description        *string
	DefaultAssigneeRef *string
	AssigneeRef        *string
	DueAt              *string
	Tags               *[]string
	Status             *string
	ActorID            *int64
}

// ProjectCreateInput contains mutable fields for project creation.
type ProjectCreateInput struct {
	Name               string
	Description        string
	DomainRef          string
	DefaultAssigneeRef *string
	AssigneeRef        *string
	DueAt              *string
	Tags               []string
	ActorID            *int64
}

// ProjectUpdateInput contains mutable fields for project updates.
type ProjectUpdateInput struct {
	Reference          string
	Name               *string
	Description        *string
	DomainRef          *string
	DefaultAssigneeRef *string
	AssigneeRef        *string
	DueAt              *string
	Tags               *[]string
	Status             *string
	ActorID            *int64
}

// CreateDomain inserts a new domain and corresponding event.
func CreateDomain(ctx context.Context, db *sql.DB, input DomainCreateInput) (Domain, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return Domain{}, fmt.Errorf("begin create domain transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	handle, err := nextHandle(ctx, tx, "domain", "DOM")
	if err != nil {
		return Domain{}, err
	}

	defaultAssigneeID, err := resolveNullableActorIDTx(ctx, tx, input.DefaultAssigneeRef)
	if err != nil {
		return Domain{}, err
	}
	assigneeID, err := resolveNullableActorIDTx(ctx, tx, input.AssigneeRef)
	if err != nil {
		return Domain{}, err
	}

	tagsJSON, err := json.Marshal(input.Tags)
	if err != nil {
		return Domain{}, fmt.Errorf("marshal domain tags: %w", err)
	}

	result, err := tx.ExecContext(ctx, `
		INSERT INTO domains(uuid, handle, name, description, status, default_assignee_actor_id, assignee_actor_id, due_at, tags)
		VALUES (?, ?, ?, ?, 'backlog', ?, ?, ?, ?)
	`, uuid.NewString(), handle, input.Name, input.Description, defaultAssigneeID, assigneeID, nullableValue(input.DueAt), string(tagsJSON))
	if err != nil {
		return Domain{}, fmt.Errorf("insert domain: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return Domain{}, fmt.Errorf("read domain insert id: %w", err)
	}

	domain, err := findDomainByIDTx(ctx, tx, id)
	if err != nil {
		return Domain{}, err
	}

	if err := appendEventTx(ctx, tx, eventInput{
		EntityType: "domain",
		EntityUUID: domain.UUID,
		ActorID:    input.ActorID,
		EventType:  "create",
		Payload: map[string]any{
			"name":   domain.Name,
			"status": domain.Status,
		},
	}); err != nil {
		return Domain{}, err
	}

	if err := tx.Commit(); err != nil {
		return Domain{}, fmt.Errorf("commit create domain transaction: %w", err)
	}

	return domain, nil
}

// ListDomains returns domains ordered by most recently updated first.
func ListDomains(ctx context.Context, db *sql.DB) ([]Domain, error) {
	rows, err := db.QueryContext(ctx, domainSelectQuery+` ORDER BY d.updated_at DESC, d.id DESC`)
	if err != nil {
		return nil, fmt.Errorf("list domains: %w", err)
	}
	defer rows.Close()

	var domains []Domain
	for rows.Next() {
		domain, err := scanDomain(rows)
		if err != nil {
			return nil, err
		}
		domains = append(domains, domain)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate domains: %w", err)
	}

	return domains, nil
}

// FindDomain resolves a domain by handle or UUID.
func FindDomain(ctx context.Context, db *sql.DB, reference string) (Domain, error) {
	return queryDomain(ctx, db, domainSelectQuery+` WHERE d.handle = ? OR d.uuid = ?`, reference, reference)
}

// UpdateDomain mutates domain fields and records an event.
func UpdateDomain(ctx context.Context, db *sql.DB, input DomainUpdateInput) (Domain, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return Domain{}, fmt.Errorf("begin update domain transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	current, err := findDomainTx(ctx, tx, input.Reference)
	if err != nil {
		return Domain{}, err
	}

	next := current
	if input.Name != nil {
		next.Name = *input.Name
	}
	if input.Description != nil {
		next.Description = *input.Description
	}
	if input.Status != nil {
		next.Status = *input.Status
	}
	if input.Tags != nil {
		next.Tags = *input.Tags
	}
	if input.DueAt != nil {
		next.DueAt = nullableStringPointer(*input.DueAt)
	}

	defaultAssigneeID, err := currentNullableActorID(ctx, tx, current.DefaultAssigneeUUID)
	if err != nil {
		return Domain{}, err
	}
	if input.DefaultAssigneeRef != nil {
		defaultAssigneeID, err = resolveNullableActorIDTx(ctx, tx, input.DefaultAssigneeRef)
		if err != nil {
			return Domain{}, err
		}
	}

	assigneeID, err := currentNullableActorID(ctx, tx, current.AssigneeUUID)
	if err != nil {
		return Domain{}, err
	}
	if input.AssigneeRef != nil {
		assigneeID, err = resolveNullableActorIDTx(ctx, tx, input.AssigneeRef)
		if err != nil {
			return Domain{}, err
		}
	}

	if err := validateLifecycleTransition(current.Status, next.Status); err != nil {
		return Domain{}, err
	}

	tagsJSON, err := json.Marshal(next.Tags)
	if err != nil {
		return Domain{}, fmt.Errorf("marshal domain tags: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE domains
		SET name = ?, description = ?, status = ?, default_assignee_actor_id = ?, assignee_actor_id = ?, due_at = ?, tags = ?,
		    updated_at = CURRENT_TIMESTAMP,
		    closed_at = CASE
		      WHEN ? IN ('completed', 'cancelled') THEN COALESCE(closed_at, CURRENT_TIMESTAMP)
		      ELSE NULL
		    END
		WHERE id = ?
	`, next.Name, next.Description, next.Status, defaultAssigneeID, assigneeID, nullableValue(next.DueAt), string(tagsJSON), next.Status, current.ID); err != nil {
		return Domain{}, fmt.Errorf("update domain: %w", err)
	}

	updated, err := findDomainByIDTx(ctx, tx, current.ID)
	if err != nil {
		return Domain{}, err
	}

	eventType := "update"
	payload := map[string]any{
		"name":                   updated.Name,
		"description":            updated.Description,
		"default_assignee_actor": updated.DefaultAssigneeUUID,
		"assignee_actor":         updated.AssigneeUUID,
		"due_at":                 updated.DueAt,
		"tags":                   updated.Tags,
	}
	if current.Status != updated.Status {
		eventType = "status_change"
		payload = map[string]any{
			"from": current.Status,
			"to":   updated.Status,
		}
	}

	if err := appendEventTx(ctx, tx, eventInput{
		EntityType: "domain",
		EntityUUID: updated.UUID,
		ActorID:    input.ActorID,
		EventType:  eventType,
		Payload:    payload,
	}); err != nil {
		return Domain{}, err
	}

	if err := tx.Commit(); err != nil {
		return Domain{}, fmt.Errorf("commit update domain transaction: %w", err)
	}

	return updated, nil
}

// CreateProject inserts a new project and corresponding event.
func CreateProject(ctx context.Context, db *sql.DB, input ProjectCreateInput) (Project, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return Project{}, fmt.Errorf("begin create project transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	handle, err := nextHandle(ctx, tx, "project", "PROJ")
	if err != nil {
		return Project{}, err
	}

	domain, err := findDomainTx(ctx, tx, input.DomainRef)
	if err != nil {
		return Project{}, fmt.Errorf("%w: project domain %q not found", ErrDomainProjectConstraint, input.DomainRef)
	}

	defaultAssigneeID, err := resolveNullableActorIDTx(ctx, tx, input.DefaultAssigneeRef)
	if err != nil {
		return Project{}, err
	}
	assigneeID, err := resolveNullableActorIDTx(ctx, tx, input.AssigneeRef)
	if err != nil {
		return Project{}, err
	}

	tagsJSON, err := json.Marshal(input.Tags)
	if err != nil {
		return Project{}, fmt.Errorf("marshal project tags: %w", err)
	}

	result, err := tx.ExecContext(ctx, `
		INSERT INTO projects(uuid, handle, name, description, status, domain_id, default_assignee_actor_id, assignee_actor_id, due_at, tags)
		VALUES (?, ?, ?, ?, 'backlog', ?, ?, ?, ?, ?)
	`, uuid.NewString(), handle, input.Name, input.Description, domain.ID, defaultAssigneeID, assigneeID, nullableValue(input.DueAt), string(tagsJSON))
	if err != nil {
		return Project{}, fmt.Errorf("insert project: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return Project{}, fmt.Errorf("read project insert id: %w", err)
	}

	project, err := findProjectByIDTx(ctx, tx, id)
	if err != nil {
		return Project{}, err
	}

	if err := appendEventTx(ctx, tx, eventInput{
		EntityType: "project",
		EntityUUID: project.UUID,
		ActorID:    input.ActorID,
		EventType:  "create",
		Payload: map[string]any{
			"name":      project.Name,
			"status":    project.Status,
			"domain_id": project.DomainUUID,
		},
	}); err != nil {
		return Project{}, err
	}

	if err := tx.Commit(); err != nil {
		return Project{}, fmt.Errorf("commit create project transaction: %w", err)
	}

	return project, nil
}

// ListProjects returns projects ordered by most recently updated first.
func ListProjects(ctx context.Context, db *sql.DB) ([]Project, error) {
	rows, err := db.QueryContext(ctx, projectSelectQuery+` ORDER BY p.updated_at DESC, p.id DESC`)
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	defer rows.Close()

	var projects []Project
	for rows.Next() {
		project, err := scanProject(rows)
		if err != nil {
			return nil, err
		}
		projects = append(projects, project)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate projects: %w", err)
	}

	return projects, nil
}

// FindProject resolves a project by handle or UUID.
func FindProject(ctx context.Context, db *sql.DB, reference string) (Project, error) {
	return queryProject(ctx, db, projectSelectQuery+` WHERE p.handle = ? OR p.uuid = ?`, reference, reference)
}

// UpdateProject mutates project fields and records an event.
func UpdateProject(ctx context.Context, db *sql.DB, input ProjectUpdateInput) (Project, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return Project{}, fmt.Errorf("begin update project transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	current, err := findProjectTx(ctx, tx, input.Reference)
	if err != nil {
		return Project{}, err
	}

	next := current
	if input.Name != nil {
		next.Name = *input.Name
	}
	if input.Description != nil {
		next.Description = *input.Description
	}
	if input.Status != nil {
		next.Status = *input.Status
	}
	if input.Tags != nil {
		next.Tags = *input.Tags
	}
	if input.DueAt != nil {
		next.DueAt = nullableStringPointer(*input.DueAt)
	}

	var domainID int64
	if err := tx.QueryRowContext(ctx, `SELECT id FROM domains WHERE uuid = ?`, current.DomainUUID).Scan(&domainID); err != nil {
		return Project{}, fmt.Errorf("resolve current project domain: %w", err)
	}
	if input.DomainRef != nil {
		domain, err := findDomainTx(ctx, tx, *input.DomainRef)
		if err != nil {
			return Project{}, fmt.Errorf("%w: project domain %q not found", ErrDomainProjectConstraint, *input.DomainRef)
		}
		domainID = domain.ID
	}

	defaultAssigneeID, err := currentNullableActorID(ctx, tx, current.DefaultAssigneeUUID)
	if err != nil {
		return Project{}, err
	}
	if input.DefaultAssigneeRef != nil {
		defaultAssigneeID, err = resolveNullableActorIDTx(ctx, tx, input.DefaultAssigneeRef)
		if err != nil {
			return Project{}, err
		}
	}

	assigneeID, err := currentNullableActorID(ctx, tx, current.AssigneeUUID)
	if err != nil {
		return Project{}, err
	}
	if input.AssigneeRef != nil {
		assigneeID, err = resolveNullableActorIDTx(ctx, tx, input.AssigneeRef)
		if err != nil {
			return Project{}, err
		}
	}

	if err := validateLifecycleTransition(current.Status, next.Status); err != nil {
		return Project{}, err
	}

	tagsJSON, err := json.Marshal(next.Tags)
	if err != nil {
		return Project{}, fmt.Errorf("marshal project tags: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE projects
		SET name = ?, description = ?, status = ?, domain_id = ?, default_assignee_actor_id = ?, assignee_actor_id = ?, due_at = ?, tags = ?,
		    updated_at = CURRENT_TIMESTAMP,
		    closed_at = CASE
		      WHEN ? IN ('completed', 'cancelled') THEN COALESCE(closed_at, CURRENT_TIMESTAMP)
		      ELSE NULL
		    END
		WHERE id = ?
	`, next.Name, next.Description, next.Status, domainID, defaultAssigneeID, assigneeID, nullableValue(next.DueAt), string(tagsJSON), next.Status, current.ID); err != nil {
		return Project{}, fmt.Errorf("update project: %w", err)
	}

	updated, err := findProjectByIDTx(ctx, tx, current.ID)
	if err != nil {
		return Project{}, err
	}

	eventType := "update"
	payload := map[string]any{
		"name":                   updated.Name,
		"description":            updated.Description,
		"domain_id":              updated.DomainUUID,
		"default_assignee_actor": updated.DefaultAssigneeUUID,
		"assignee_actor":         updated.AssigneeUUID,
		"due_at":                 updated.DueAt,
		"tags":                   updated.Tags,
	}
	if current.Status != updated.Status {
		eventType = "status_change"
		payload = map[string]any{
			"from": current.Status,
			"to":   updated.Status,
		}
	}

	if err := appendEventTx(ctx, tx, eventInput{
		EntityType: "project",
		EntityUUID: updated.UUID,
		ActorID:    input.ActorID,
		EventType:  eventType,
		Payload:    payload,
	}); err != nil {
		return Project{}, err
	}

	if err := tx.Commit(); err != nil {
		return Project{}, fmt.Errorf("commit update project transaction: %w", err)
	}

	return updated, nil
}

const domainSelectQuery = `
	SELECT d.id, d.uuid, d.handle, d.name, d.description, d.status,
	       def.uuid, def.handle, ass.uuid, ass.handle, d.due_at, d.tags, d.created_at, d.updated_at, d.closed_at
	FROM domains d
	LEFT JOIN actors def ON def.id = d.default_assignee_actor_id
	LEFT JOIN actors ass ON ass.id = d.assignee_actor_id
`

const projectSelectQuery = `
	SELECT p.id, p.uuid, p.handle, p.name, p.description, p.status,
	       d.uuid, d.handle, def.uuid, def.handle, ass.uuid, ass.handle, p.due_at, p.tags, p.created_at, p.updated_at, p.closed_at
	FROM projects p
	INNER JOIN domains d ON d.id = p.domain_id
	LEFT JOIN actors def ON def.id = p.default_assignee_actor_id
	LEFT JOIN actors ass ON ass.id = p.assignee_actor_id
`

func findDomainTx(ctx context.Context, tx *sql.Tx, reference string) (Domain, error) {
	return queryDomain(ctx, tx, domainSelectQuery+` WHERE d.handle = ? OR d.uuid = ?`, reference, reference)
}

func findDomainByIDTx(ctx context.Context, tx *sql.Tx, id int64) (Domain, error) {
	return queryDomain(ctx, tx, domainSelectQuery+` WHERE d.id = ?`, id)
}

func findProjectTx(ctx context.Context, tx *sql.Tx, reference string) (Project, error) {
	return queryProject(ctx, tx, projectSelectQuery+` WHERE p.handle = ? OR p.uuid = ?`, reference, reference)
}

func findProjectByIDTx(ctx context.Context, tx *sql.Tx, id int64) (Project, error) {
	return queryProject(ctx, tx, projectSelectQuery+` WHERE p.id = ?`, id)
}

type entityRowScanner interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func queryDomain(ctx context.Context, runner entityRowScanner, query string, args ...any) (Domain, error) {
	return scanDomain(runner.QueryRowContext(ctx, query, args...))
}

func queryProject(ctx context.Context, runner entityRowScanner, query string, args ...any) (Project, error) {
	return scanProject(runner.QueryRowContext(ctx, query, args...))
}

func scanDomain(scanner interface{ Scan(...any) error }) (Domain, error) {
	var domain Domain
	var defaultAssigneeUUID sql.NullString
	var defaultAssigneeHandle sql.NullString
	var assigneeUUID sql.NullString
	var assigneeHandle sql.NullString
	var dueAt sql.NullString
	var tagsJSON string
	var closedAt sql.NullString

	if err := scanner.Scan(
		&domain.ID,
		&domain.UUID,
		&domain.Handle,
		&domain.Name,
		&domain.Description,
		&domain.Status,
		&defaultAssigneeUUID,
		&defaultAssigneeHandle,
		&assigneeUUID,
		&assigneeHandle,
		&dueAt,
		&tagsJSON,
		&domain.CreatedAt,
		&domain.UpdatedAt,
		&closedAt,
	); err != nil {
		return Domain{}, err
	}

	if tagsJSON == "" {
		tagsJSON = "[]"
	}
	if err := json.Unmarshal([]byte(tagsJSON), &domain.Tags); err != nil {
		return Domain{}, fmt.Errorf("unmarshal domain tags: %w", err)
	}

	domain.DefaultAssigneeUUID = nullableStringFromNull(defaultAssigneeUUID)
	domain.DefaultAssigneeHandle = nullableStringFromNull(defaultAssigneeHandle)
	domain.AssigneeUUID = nullableStringFromNull(assigneeUUID)
	domain.AssigneeHandle = nullableStringFromNull(assigneeHandle)
	domain.DueAt = nullableStringFromNull(dueAt)
	domain.ClosedAt = nullableStringFromNull(closedAt)
	return domain, nil
}

func scanProject(scanner interface{ Scan(...any) error }) (Project, error) {
	var project Project
	var defaultAssigneeUUID sql.NullString
	var defaultAssigneeHandle sql.NullString
	var assigneeUUID sql.NullString
	var assigneeHandle sql.NullString
	var dueAt sql.NullString
	var tagsJSON string
	var closedAt sql.NullString

	if err := scanner.Scan(
		&project.ID,
		&project.UUID,
		&project.Handle,
		&project.Name,
		&project.Description,
		&project.Status,
		&project.DomainUUID,
		&project.DomainHandle,
		&defaultAssigneeUUID,
		&defaultAssigneeHandle,
		&assigneeUUID,
		&assigneeHandle,
		&dueAt,
		&tagsJSON,
		&project.CreatedAt,
		&project.UpdatedAt,
		&closedAt,
	); err != nil {
		return Project{}, err
	}

	if tagsJSON == "" {
		tagsJSON = "[]"
	}
	if err := json.Unmarshal([]byte(tagsJSON), &project.Tags); err != nil {
		return Project{}, fmt.Errorf("unmarshal project tags: %w", err)
	}

	project.DefaultAssigneeUUID = nullableStringFromNull(defaultAssigneeUUID)
	project.DefaultAssigneeHandle = nullableStringFromNull(defaultAssigneeHandle)
	project.AssigneeUUID = nullableStringFromNull(assigneeUUID)
	project.AssigneeHandle = nullableStringFromNull(assigneeHandle)
	project.DueAt = nullableStringFromNull(dueAt)
	project.ClosedAt = nullableStringFromNull(closedAt)
	return project, nil
}

func resolveNullableActorIDTx(ctx context.Context, tx *sql.Tx, reference *string) (any, error) {
	if reference == nil {
		return nil, nil
	}
	if *reference == "" {
		return nil, nil
	}

	actor, err := findActorReferenceTx(ctx, tx, *reference)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("actor reference %q not found", *reference)
		}
		return nil, err
	}

	return actor.ID, nil
}

func currentNullableActorID(ctx context.Context, tx *sql.Tx, actorUUID *string) (any, error) {
	if actorUUID == nil {
		return nil, nil
	}

	var id int64
	if err := tx.QueryRowContext(ctx, `SELECT id FROM actors WHERE uuid = ?`, *actorUUID).Scan(&id); err != nil {
		return nil, fmt.Errorf("resolve actor id for uuid %q: %w", *actorUUID, err)
	}
	return id, nil
}

func findActorReferenceTx(ctx context.Context, tx *sql.Tx, reference string) (Actor, error) {
	if identity, err := ParseAgentIdentity(reference); err == nil {
		return queryActor(
			ctx,
			tx,
			`
				SELECT id, uuid, handle, kind, ifnull(provider, ''), external_id, display_name,
				       first_seen_at, last_seen_at, created_at, updated_at
				FROM actors
				WHERE kind = ? AND ifnull(provider, '') = ? AND external_id = ?
			`,
			"agent",
			identity.Provider,
			identity.ExternalID,
		)
	}

	actor, err := queryActor(
		ctx,
		tx,
		`
			SELECT id, uuid, handle, kind, ifnull(provider, ''), external_id, display_name,
			       first_seen_at, last_seen_at, created_at, updated_at
			FROM actors
			WHERE handle = ? OR uuid = ?
		`,
		reference,
		reference,
	)
	if err == nil {
		return actor, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return Actor{}, err
	}

	return queryActor(
		ctx,
		tx,
		`
			SELECT id, uuid, handle, kind, ifnull(provider, ''), external_id, display_name,
			       first_seen_at, last_seen_at, created_at, updated_at
			FROM actors
			WHERE kind = 'human' AND external_id = ?
		`,
		reference,
	)
}

func nullableValue(value *string) any {
	if value == nil || *value == "" {
		return nil
	}
	return *value
}
