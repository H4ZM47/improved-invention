package db

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestAcquireClaimRejectsConcurrentClaim(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)
	actorOne := insertActor(t, db, "actor-1", "ACT-1", "agent", "codex", "agent-1")
	actorTwo := insertActor(t, db, "actor-2", "ACT-2", "agent", "codex", "agent-2")
	insertTask(t, db, "task-1", "TASK-1", "Claim me", nil, nil)

	if _, err := AcquireClaim(context.Background(), db, ClaimAcquireInput{
		TaskReference: "TASK-1",
		ActorID:       actorOne,
		Lease:         time.Hour,
	}); err != nil {
		t.Fatalf("first AcquireClaim() error = %v", err)
	}

	_, err := AcquireClaim(context.Background(), db, ClaimAcquireInput{
		TaskReference: "TASK-1",
		ActorID:       actorTwo,
		Lease:         time.Hour,
	})
	if !errors.Is(err, ErrClaimConflict) {
		t.Fatalf("second AcquireClaim() error = %v, want ErrClaimConflict", err)
	}
}

func TestReleaseClaimAllowsReacquire(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)
	actorOne := insertActor(t, db, "actor-1", "ACT-1", "agent", "codex", "agent-1")
	actorTwo := insertActor(t, db, "actor-2", "ACT-2", "agent", "codex", "agent-2")
	insertTask(t, db, "task-1", "TASK-1", "Claim me", nil, nil)

	if _, err := AcquireClaim(context.Background(), db, ClaimAcquireInput{
		TaskReference: "TASK-1",
		ActorID:       actorOne,
		Lease:         time.Hour,
	}); err != nil {
		t.Fatalf("AcquireClaim() error = %v", err)
	}

	if _, err := ReleaseClaim(context.Background(), db, ClaimMutationInput{
		TaskReference: "TASK-1",
		ActorID:       actorOne,
	}); err != nil {
		t.Fatalf("ReleaseClaim() error = %v", err)
	}

	if _, err := AcquireClaim(context.Background(), db, ClaimAcquireInput{
		TaskReference: "TASK-1",
		ActorID:       actorTwo,
		Lease:         time.Hour,
	}); err != nil {
		t.Fatalf("reacquire after release error = %v", err)
	}
}

func TestExpiredClaimIsClearedBeforeReacquire(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)
	actorOne := insertActor(t, db, "actor-1", "ACT-1", "agent", "codex", "agent-1")
	actorTwo := insertActor(t, db, "actor-2", "ACT-2", "agent", "codex", "agent-2")
	taskID := insertTask(t, db, "task-1", "TASK-1", "Claim me", nil, nil)

	if _, err := db.Exec(`
		INSERT INTO claims(uuid, task_id, actor_id, expires_at)
		VALUES ('claim-1', ?, ?, ?)
	`, taskID, actorOne, time.Now().UTC().Add(-time.Hour).Format(time.RFC3339)); err != nil {
		t.Fatalf("seed expired claim failed: %v", err)
	}

	if _, err := AcquireClaim(context.Background(), db, ClaimAcquireInput{
		TaskReference: "TASK-1",
		ActorID:       actorTwo,
		Lease:         time.Hour,
	}); err != nil {
		t.Fatalf("AcquireClaim() after expiry error = %v", err)
	}
}

func TestAcquireClaimConcurrentRaceReturnsOneWinnerAndOneConflict(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)
	actorOne := insertActor(t, db, "actor-1", "ACT-1", "agent", "codex", "agent-1")
	actorTwo := insertActor(t, db, "actor-2", "ACT-2", "agent", "codex", "agent-2")
	insertTask(t, db, "task-1", "TASK-1", "Claim me concurrently", nil, nil)

	start := make(chan struct{})
	type result struct {
		actorID int64
		err     error
	}
	results := make(chan result, 2)
	var wg sync.WaitGroup

	run := func(actorID int64) {
		defer wg.Done()
		<-start
		_, err := AcquireClaim(context.Background(), db, ClaimAcquireInput{
			TaskReference: "TASK-1",
			ActorID:       actorID,
			Lease:         time.Hour,
		})
		results <- result{actorID: actorID, err: err}
	}

	wg.Add(2)
	go run(actorOne)
	go run(actorTwo)
	close(start)
	wg.Wait()
	close(results)

	var successCount int
	var conflictCount int
	for result := range results {
		switch {
		case result.err == nil:
			successCount++
		case errors.Is(result.err, ErrClaimConflict):
			conflictCount++
		default:
			t.Fatalf("AcquireClaim(actor=%d) error = %v, want nil or ErrClaimConflict", result.actorID, result.err)
		}
	}

	if got, want := successCount, 1; got != want {
		t.Fatalf("successCount = %d, want %d", got, want)
	}
	if got, want := conflictCount, 1; got != want {
		t.Fatalf("conflictCount = %d, want %d", got, want)
	}
}
