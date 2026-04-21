package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrClaimConflict = errors.New("claim conflict")
	ErrClaimRequired = errors.New("claim required")
	ErrClaimNotHeld  = errors.New("claim not held by actor")
	ErrClaimExpired  = errors.New("claim expired")
)

// Claim is the database-backed representation of an active or historical claim.
type Claim struct {
	UUID        string  `json:"uuid"`
	TaskHandle  string  `json:"task_handle"`
	TaskUUID    string  `json:"task_uuid"`
	ActorHandle string  `json:"actor_handle"`
	ActorUUID   string  `json:"actor_uuid"`
	ClaimedAt   string  `json:"claimed_at"`
	ExpiresAt   string  `json:"expires_at"`
	RenewedAt   *string `json:"renewed_at"`
	ReleasedAt  *string `json:"released_at"`
}

type ClaimAcquireInput struct {
	TaskReference string
	ActorID       int64
	Lease         time.Duration
}

type ClaimMutationInput struct {
	TaskReference string
	ActorID       int64
	Lease         time.Duration
}

func AcquireClaim(ctx context.Context, db *sql.DB, input ClaimAcquireInput) (Claim, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return Claim{}, fmt.Errorf("begin claim transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	task, err := findTaskTx(ctx, tx, input.TaskReference)
	if err != nil {
		return Claim{}, err
	}

	if err := expireClaimsTx(ctx, tx, task.ID); err != nil {
		return Claim{}, err
	}

	if _, err := currentClaimTx(ctx, tx, task.ID); err == nil {
		return Claim{}, ErrClaimConflict
	} else if !errors.Is(err, sql.ErrNoRows) {
		return Claim{}, err
	}

	expiresAt := time.Now().UTC().Add(input.Lease).Format(time.RFC3339)
	claimUUID := uuid.NewString()
	result, err := tx.ExecContext(ctx, `
		INSERT INTO claims(uuid, task_id, actor_id, expires_at)
		VALUES (?, ?, ?, ?)
	`, claimUUID, task.ID, input.ActorID, expiresAt)
	if err != nil {
		if isClaimConflictError(err) {
			return Claim{}, ErrClaimConflict
		}
		return Claim{}, fmt.Errorf("insert claim: %w", err)
	}

	claimID, err := result.LastInsertId()
	if err != nil {
		return Claim{}, fmt.Errorf("read claim insert id: %w", err)
	}

	if err := appendEventTx(ctx, tx, eventInput{
		EntityType: "task",
		EntityUUID: task.UUID,
		ActorID:    &input.ActorID,
		EventType:  "claim_acquired",
		Payload: map[string]any{
			"task_handle": task.Handle,
			"expires_at":  expiresAt,
		},
	}); err != nil {
		return Claim{}, err
	}

	claim, err := findClaimByIDTx(ctx, tx, claimID)
	if err != nil {
		return Claim{}, err
	}

	if err := tx.Commit(); err != nil {
		return Claim{}, fmt.Errorf("commit claim acquisition: %w", err)
	}

	return claim, nil
}

func ReleaseClaim(ctx context.Context, db *sql.DB, input ClaimMutationInput) (Claim, error) {
	return mutateClaim(ctx, db, input, "claim_released", false)
}

func RenewClaim(ctx context.Context, db *sql.DB, input ClaimMutationInput) (Claim, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return Claim{}, fmt.Errorf("begin renew claim transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	task, err := findTaskTx(ctx, tx, input.TaskReference)
	if err != nil {
		return Claim{}, err
	}
	if err := expireClaimsTx(ctx, tx, task.ID); err != nil {
		return Claim{}, err
	}

	current, err := currentClaimTx(ctx, tx, task.ID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Claim{}, ErrClaimRequired
		}
		return Claim{}, err
	}
	if current.ActorUUID != "" {
		var currentActorID int64
		if err := tx.QueryRowContext(ctx, `SELECT id FROM actors WHERE uuid = ?`, current.ActorUUID).Scan(&currentActorID); err != nil {
			return Claim{}, err
		}
		if currentActorID != input.ActorID {
			return Claim{}, ErrClaimNotHeld
		}
	}

	newExpiry := time.Now().UTC().Add(input.Lease).Format(time.RFC3339)
	if _, err := tx.ExecContext(ctx, `
		UPDATE claims
		SET expires_at = ?, renewed_at = CURRENT_TIMESTAMP
		WHERE uuid = ?
	`, newExpiry, current.UUID); err != nil {
		return Claim{}, fmt.Errorf("renew claim: %w", err)
	}

	if err := appendEventTx(ctx, tx, eventInput{
		EntityType: "task",
		EntityUUID: task.UUID,
		ActorID:    &input.ActorID,
		EventType:  "claim_renewed",
		Payload: map[string]any{
			"task_handle": task.Handle,
			"expires_at":  newExpiry,
		},
	}); err != nil {
		return Claim{}, err
	}

	claim, err := currentClaimTx(ctx, tx, task.ID)
	if err != nil {
		return Claim{}, err
	}
	if err := tx.Commit(); err != nil {
		return Claim{}, fmt.Errorf("commit claim renewal: %w", err)
	}
	return claim, nil
}

func UnlockClaim(ctx context.Context, db *sql.DB, input ClaimMutationInput) (Claim, error) {
	return mutateClaim(ctx, db, input, "unlock", true)
}

func RequireClaimHeld(ctx context.Context, tx *sql.Tx, taskID int64, actorID int64) error {
	if err := expireClaimsTx(ctx, tx, taskID); err != nil {
		return err
	}

	current, err := currentClaimTx(ctx, tx, taskID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrClaimRequired
		}
		return err
	}

	var currentActorID int64
	if err := tx.QueryRowContext(ctx, `SELECT id FROM actors WHERE uuid = ?`, current.ActorUUID).Scan(&currentActorID); err != nil {
		return fmt.Errorf("resolve current claim actor: %w", err)
	}
	if currentActorID != actorID {
		return ErrClaimNotHeld
	}

	return nil
}

func mutateClaim(ctx context.Context, db *sql.DB, input ClaimMutationInput, eventType string, ignoreHolder bool) (Claim, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return Claim{}, fmt.Errorf("begin claim mutation transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	task, err := findTaskTx(ctx, tx, input.TaskReference)
	if err != nil {
		return Claim{}, err
	}
	if err := expireClaimsTx(ctx, tx, task.ID); err != nil {
		return Claim{}, err
	}

	current, err := currentClaimTx(ctx, tx, task.ID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Claim{}, ErrClaimRequired
		}
		return Claim{}, err
	}

	if !ignoreHolder {
		var currentActorID int64
		if err := tx.QueryRowContext(ctx, `SELECT id FROM actors WHERE uuid = ?`, current.ActorUUID).Scan(&currentActorID); err != nil {
			return Claim{}, err
		}
		if currentActorID != input.ActorID {
			return Claim{}, ErrClaimNotHeld
		}
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE claims
		SET released_at = CURRENT_TIMESTAMP, release_reason = ?
		WHERE uuid = ?
	`, eventType, current.UUID); err != nil {
		return Claim{}, fmt.Errorf("release claim: %w", err)
	}

	if err := appendEventTx(ctx, tx, eventInput{
		EntityType: "task",
		EntityUUID: task.UUID,
		ActorID:    &input.ActorID,
		EventType:  eventType,
		Payload: map[string]any{
			"task_handle": task.Handle,
		},
	}); err != nil {
		return Claim{}, err
	}

	claim, err := findClaimByUUIDTx(ctx, tx, current.UUID)
	if err != nil {
		return Claim{}, err
	}

	if err := tx.Commit(); err != nil {
		return Claim{}, fmt.Errorf("commit claim mutation: %w", err)
	}

	return claim, nil
}

func expireClaimsTx(ctx context.Context, tx *sql.Tx, taskID int64) error {
	if _, err := tx.ExecContext(ctx, `
		UPDATE claims
		SET released_at = CURRENT_TIMESTAMP, release_reason = 'expired'
		WHERE task_id = ? AND released_at IS NULL AND expires_at <= ?
	`, taskID, time.Now().UTC().Format(time.RFC3339)); err != nil {
		return fmt.Errorf("expire stale claims: %w", err)
	}
	return nil
}

func isClaimConflictError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "UNIQUE constraint failed: claims.task_id") ||
		strings.Contains(msg, "claims_one_open_claim_per_task_idx")
}

func currentClaimTx(ctx context.Context, tx *sql.Tx, taskID int64) (Claim, error) {
	return queryClaim(
		ctx,
		tx,
		`
			SELECT c.uuid, t.handle, t.uuid, a.handle, a.uuid, c.claimed_at, c.expires_at, c.renewed_at, c.released_at
			FROM claims c
			JOIN tasks t ON t.id = c.task_id
			JOIN actors a ON a.id = c.actor_id
			WHERE c.task_id = ? AND c.released_at IS NULL
			ORDER BY c.id DESC
			LIMIT 1
		`,
		taskID,
	)
}

func findClaimByIDTx(ctx context.Context, tx *sql.Tx, id int64) (Claim, error) {
	return queryClaim(
		ctx,
		tx,
		`
			SELECT c.uuid, t.handle, t.uuid, a.handle, a.uuid, c.claimed_at, c.expires_at, c.renewed_at, c.released_at
			FROM claims c
			JOIN tasks t ON t.id = c.task_id
			JOIN actors a ON a.id = c.actor_id
			WHERE c.id = ?
		`,
		id,
	)
}

func findClaimByUUIDTx(ctx context.Context, tx *sql.Tx, claimUUID string) (Claim, error) {
	return queryClaim(
		ctx,
		tx,
		`
			SELECT c.uuid, t.handle, t.uuid, a.handle, a.uuid, c.claimed_at, c.expires_at, c.renewed_at, c.released_at
			FROM claims c
			JOIN tasks t ON t.id = c.task_id
			JOIN actors a ON a.id = c.actor_id
			WHERE c.uuid = ?
		`,
		claimUUID,
	)
}

type claimRowScanner interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func queryClaim(ctx context.Context, runner claimRowScanner, query string, args ...any) (Claim, error) {
	row := runner.QueryRowContext(ctx, query, args...)
	return scanClaim(row)
}

func scanClaim(scanner interface{ Scan(...any) error }) (Claim, error) {
	var claim Claim
	var renewedAt sql.NullString
	var releasedAt sql.NullString
	if err := scanner.Scan(
		&claim.UUID,
		&claim.TaskHandle,
		&claim.TaskUUID,
		&claim.ActorHandle,
		&claim.ActorUUID,
		&claim.ClaimedAt,
		&claim.ExpiresAt,
		&renewedAt,
		&releasedAt,
	); err != nil {
		return Claim{}, err
	}

	claim.RenewedAt = nullableStringFromNull(renewedAt)
	claim.ReleasedAt = nullableStringFromNull(releasedAt)
	return claim, nil
}
