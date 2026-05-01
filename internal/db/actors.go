package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// Actor is the database representation of an actor record.
type Actor struct {
	ID          int64
	UUID        string
	Handle      string
	Kind        string
	Provider    string
	ExternalID  string
	DisplayName string
	FirstSeenAt string
	LastSeenAt  string
	CreatedAt   string
	UpdatedAt   string
}

// AgentIdentity is a parsed provider-scoped agent identifier.
type AgentIdentity struct {
	Provider   string
	ExternalID string
}

// ParseAgentIdentity parses values such as codex:agent-7.
func ParseAgentIdentity(raw string) (AgentIdentity, error) {
	provider, externalID, found := strings.Cut(strings.TrimSpace(raw), ":")
	if !found || strings.TrimSpace(provider) == "" || strings.TrimSpace(externalID) == "" {
		return AgentIdentity{}, fmt.Errorf("invalid agent identity %q", raw)
	}

	return AgentIdentity{
		Provider:   strings.TrimSpace(provider),
		ExternalID: strings.TrimSpace(externalID),
	}, nil
}

// EnsureHumanActor loads or creates the configured human actor and refreshes its timestamps.
func EnsureHumanActor(ctx context.Context, db *sql.DB, humanName string) (Actor, error) {
	return ensureActor(ctx, db, actorUpsertInput{
		Kind:        "human",
		Provider:    "",
		ExternalID:  humanName,
		DisplayName: humanName,
	})
}

// GetOrCreateAgentActor loads or creates an agent actor and refreshes its timestamps.
func GetOrCreateAgentActor(ctx context.Context, db *sql.DB, identity AgentIdentity) (Actor, error) {
	return ensureActor(ctx, db, actorUpsertInput{
		Kind:        "agent",
		Provider:    identity.Provider,
		ExternalID:  identity.ExternalID,
		DisplayName: identity.Provider + ":" + identity.ExternalID,
	})
}

// ListActors returns actors ordered for deterministic presentation.
func ListActors(ctx context.Context, db *sql.DB) ([]Actor, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT id, uuid, handle, kind, ifnull(provider, ''), external_id, display_name,
		       first_seen_at, last_seen_at, created_at, updated_at
		FROM actors
		ORDER BY kind, external_id, handle
	`)
	if err != nil {
		return nil, fmt.Errorf("list actors: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	var actors []Actor
	for rows.Next() {
		actor, err := scanActor(rows)
		if err != nil {
			return nil, err
		}
		actors = append(actors, actor)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate actors: %w", err)
	}

	return actors, nil
}

// FindActor resolves an actor by handle, UUID, provider-scoped ID, or human external ID.
func FindActor(ctx context.Context, db *sql.DB, reference string) (Actor, error) {
	if identity, err := ParseAgentIdentity(reference); err == nil {
		return findActorByIdentity(ctx, db, "agent", identity.Provider, identity.ExternalID)
	}

	actor, err := findActorByHandleOrUUID(ctx, db, reference)
	if err == nil {
		return actor, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return Actor{}, err
	}

	return findActorByIdentity(ctx, db, "human", "", reference)
}

type actorUpsertInput struct {
	Kind        string
	Provider    string
	ExternalID  string
	DisplayName string
}

func ensureActor(ctx context.Context, db *sql.DB, input actorUpsertInput) (Actor, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return Actor{}, fmt.Errorf("begin actor transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	actor, err := findActorTx(ctx, tx, input.Kind, input.Provider, input.ExternalID)
	if err == nil {
		if _, err := tx.ExecContext(ctx, `
			UPDATE actors
			SET display_name = ?, last_seen_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
			WHERE id = ?
		`, input.DisplayName, actor.ID); err != nil {
			return Actor{}, fmt.Errorf("update actor timestamps: %w", err)
		}

		actor, err = findActorTx(ctx, tx, input.Kind, input.Provider, input.ExternalID)
		if err != nil {
			return Actor{}, err
		}
		if err := tx.Commit(); err != nil {
			return Actor{}, fmt.Errorf("commit existing actor update: %w", err)
		}
		return actor, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return Actor{}, err
	}

	handle, err := nextHandle(ctx, tx, "actor", "ACT")
	if err != nil {
		return Actor{}, err
	}

	providerValue := any(nil)
	if input.Provider != "" {
		providerValue = input.Provider
	}

	actorUUID := uuid.NewString()
	result, err := tx.ExecContext(ctx, `
		INSERT INTO actors(uuid, handle, kind, provider, external_id, display_name)
		VALUES (?, ?, ?, ?, ?, ?)
	`, actorUUID, handle, input.Kind, providerValue, input.ExternalID, input.DisplayName)
	if err != nil {
		return Actor{}, fmt.Errorf("insert actor: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return Actor{}, fmt.Errorf("read actor insert id: %w", err)
	}

	actor, err = findActorByIDTx(ctx, tx, id)
	if err != nil {
		return Actor{}, err
	}

	if err := tx.Commit(); err != nil {
		return Actor{}, fmt.Errorf("commit actor insert: %w", err)
	}

	return actor, nil
}

func nextHandle(ctx context.Context, tx *sql.Tx, entityType string, prefix string) (string, error) {
	var nextValue int64
	if err := tx.QueryRowContext(
		ctx,
		`SELECT next_value FROM handle_sequences WHERE entity_type = ?`,
		entityType,
	).Scan(&nextValue); err != nil {
		return "", fmt.Errorf("read handle sequence %s: %w", entityType, err)
	}

	if _, err := tx.ExecContext(
		ctx,
		`UPDATE handle_sequences SET next_value = next_value + 1 WHERE entity_type = ?`,
		entityType,
	); err != nil {
		return "", fmt.Errorf("advance handle sequence %s: %w", entityType, err)
	}

	return fmt.Sprintf("%s-%d", prefix, nextValue), nil
}

func findActorByIdentity(ctx context.Context, db *sql.DB, kind string, provider string, externalID string) (Actor, error) {
	return queryActor(
		ctx,
		db,
		`
			SELECT id, uuid, handle, kind, ifnull(provider, ''), external_id, display_name,
			       first_seen_at, last_seen_at, created_at, updated_at
			FROM actors
			WHERE kind = ? AND ifnull(provider, '') = ? AND external_id = ?
		`,
		kind,
		provider,
		externalID,
	)
}

func findActorByHandleOrUUID(ctx context.Context, db *sql.DB, reference string) (Actor, error) {
	return queryActor(
		ctx,
		db,
		`
			SELECT id, uuid, handle, kind, ifnull(provider, ''), external_id, display_name,
			       first_seen_at, last_seen_at, created_at, updated_at
			FROM actors
			WHERE handle = ? OR uuid = ?
		`,
		reference,
		reference,
	)
}

func findActorTx(ctx context.Context, tx *sql.Tx, kind string, provider string, externalID string) (Actor, error) {
	return queryActor(
		ctx,
		tx,
		`
			SELECT id, uuid, handle, kind, ifnull(provider, ''), external_id, display_name,
			       first_seen_at, last_seen_at, created_at, updated_at
			FROM actors
			WHERE kind = ? AND ifnull(provider, '') = ? AND external_id = ?
		`,
		kind,
		provider,
		externalID,
	)
}

func findActorByIDTx(ctx context.Context, tx *sql.Tx, id int64) (Actor, error) {
	return queryActor(
		ctx,
		tx,
		`
			SELECT id, uuid, handle, kind, ifnull(provider, ''), external_id, display_name,
			       first_seen_at, last_seen_at, created_at, updated_at
			FROM actors
			WHERE id = ?
		`,
		id,
	)
}

type rowScanner interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func queryActor(ctx context.Context, runner rowScanner, query string, args ...any) (Actor, error) {
	row := runner.QueryRowContext(ctx, query, args...)
	return scanActor(row)
}

type scanner interface {
	Scan(...any) error
}

func scanActor(row scanner) (Actor, error) {
	var actor Actor
	if err := row.Scan(
		&actor.ID,
		&actor.UUID,
		&actor.Handle,
		&actor.Kind,
		&actor.Provider,
		&actor.ExternalID,
		&actor.DisplayName,
		&actor.FirstSeenAt,
		&actor.LastSeenAt,
		&actor.CreatedAt,
		&actor.UpdatedAt,
	); err != nil {
		return Actor{}, err
	}

	return actor, nil
}
