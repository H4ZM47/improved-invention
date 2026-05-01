package db

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"
)

func TestTaskSessionLifecycleRecordsEvents(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)
	actorID := insertActor(t, db, "actor-1", "ACT-1", "agent", "codex", "agent-1")
	insertTask(t, db, "task-1", "TASK-1", "Track session", nil, nil)

	if _, err := AcquireClaim(context.Background(), db, ClaimAcquireInput{
		TaskReference: "TASK-1",
		ActorID:       actorID,
		Lease:         time.Hour,
	}); err != nil {
		t.Fatalf("AcquireClaim() error = %v", err)
	}

	for _, step := range []struct {
		name string
		run  func(context.Context, *sql.DB, SessionEventInput) error
	}{
		{name: "start", run: StartTaskSession},
		{name: "pause", run: PauseTaskSession},
		{name: "resume", run: ResumeTaskSession},
		{name: "close", run: CloseTaskSession},
	} {
		if err := step.run(context.Background(), db, SessionEventInput{
			TaskReference: "TASK-1",
			ActorID:       actorID,
		}); err != nil {
			t.Fatalf("%s session event error = %v", step.name, err)
		}
	}

	rows, err := db.Query(`
		SELECT event_type
		FROM events
		WHERE entity_type = 'task' AND entity_uuid = 'task-1' AND event_type LIKE 'session_%'
		ORDER BY id ASC
	`)
	if err != nil {
		t.Fatalf("query session events failed: %v", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	var got []string
	for rows.Next() {
		var eventType string
		if err := rows.Scan(&eventType); err != nil {
			t.Fatalf("scan session event failed: %v", err)
		}
		got = append(got, eventType)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate session events failed: %v", err)
	}

	want := []string{"session_started", "session_paused", "session_resumed", "session_closed"}
	if len(got) != len(want) {
		t.Fatalf("len(session events) = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("session event %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestTaskSessionEventsRequireClaim(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)
	actorID := insertActor(t, db, "actor-1", "ACT-1", "agent", "codex", "agent-1")
	insertTask(t, db, "task-1", "TASK-1", "Track session", nil, nil)

	err := StartTaskSession(context.Background(), db, SessionEventInput{
		TaskReference: "TASK-1",
		ActorID:       actorID,
	})
	if !errors.Is(err, ErrClaimRequired) {
		t.Fatalf("StartTaskSession() error = %v, want ErrClaimRequired", err)
	}
}

func TestStartAndResumeRejectClosedTasks(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)
	actorID := insertActor(t, db, "actor-1", "ACT-1", "agent", "codex", "agent-1")
	taskID := insertTask(t, db, "task-1", "TASK-1", "Closed task", nil, nil)

	if _, err := db.Exec(`UPDATE tasks SET status = 'completed', closed_at = CURRENT_TIMESTAMP WHERE id = ?`, taskID); err != nil {
		t.Fatalf("mark task completed failed: %v", err)
	}
	if _, err := db.Exec(`
		INSERT INTO claims(uuid, task_id, actor_id, expires_at)
		VALUES ('claim-1', ?, ?, ?)
	`, taskID, actorID, time.Now().UTC().Add(time.Hour).Format(time.RFC3339)); err != nil {
		t.Fatalf("seed claim failed: %v", err)
	}

	if err := StartTaskSession(context.Background(), db, SessionEventInput{
		TaskReference: "TASK-1",
		ActorID:       actorID,
	}); !errors.Is(err, ErrSessionOnClosedTask) {
		t.Fatalf("StartTaskSession() error = %v, want ErrSessionOnClosedTask", err)
	}

	if _, err := db.Exec(`
		INSERT INTO events(uuid, entity_type, entity_uuid, actor_id, event_type, payload_json, occurred_at)
		VALUES
		  ('evt-1', 'task', 'task-1', ?, 'session_started', '{}', '2026-04-20 10:00:00'),
		  ('evt-2', 'task', 'task-1', ?, 'session_paused', '{}', '2026-04-20 10:10:00')
	`, actorID, actorID); err != nil {
		t.Fatalf("seed paused session history failed: %v", err)
	}

	if err := ResumeTaskSession(context.Background(), db, SessionEventInput{
		TaskReference: "TASK-1",
		ActorID:       actorID,
	}); !errors.Is(err, ErrSessionOnClosedTask) {
		t.Fatalf("ResumeTaskSession() error = %v, want ErrSessionOnClosedTask", err)
	}
}

func TestDeriveTaskElapsedAcrossMultipleSessionIntervals(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)
	actorID := insertActor(t, db, "actor-1", "ACT-1", "agent", "codex", "agent-1")
	insertTask(t, db, "task-1", "TASK-1", "Track session", nil, nil)

	if _, err := db.Exec(`
		INSERT INTO events(uuid, entity_type, entity_uuid, actor_id, event_type, payload_json, occurred_at)
		VALUES
		  ('evt-1', 'task', 'task-1', ?, 'session_started', '{}', '2026-04-20 10:00:00'),
		  ('evt-2', 'task', 'task-1', ?, 'session_paused', '{}', '2026-04-20 10:15:00'),
		  ('evt-3', 'task', 'task-1', ?, 'session_resumed', '{}', '2026-04-20 10:30:00'),
		  ('evt-4', 'task', 'task-1', ?, 'session_closed', '{}', '2026-04-20 10:50:00'),
		  ('evt-5', 'task', 'task-1', ?, 'session_started', '{}', '2026-04-20 11:00:00'),
		  ('evt-6', 'task', 'task-1', ?, 'session_closed', '{}', '2026-04-20 11:10:00')
	`, actorID, actorID, actorID, actorID, actorID, actorID); err != nil {
		t.Fatalf("seed session history failed: %v", err)
	}

	got, err := DeriveElapsedTaskTime(context.Background(), db, "TASK-1")
	if err != nil {
		t.Fatalf("DeriveElapsedTaskTime() error = %v", err)
	}

	want := 45 * time.Minute
	if got != want {
		t.Fatalf("DeriveElapsedTaskTime() = %s, want %s", got, want)
	}
}

func TestDeriveTaskElapsedIncludesActiveSessionThroughNow(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)
	actorID := insertActor(t, db, "actor-1", "ACT-1", "agent", "codex", "agent-1")
	insertTask(t, db, "task-1", "TASK-1", "Track session", nil, nil)

	if _, err := db.Exec(`
		INSERT INTO events(uuid, entity_type, entity_uuid, actor_id, event_type, payload_json, occurred_at)
		VALUES
		  ('evt-1', 'task', 'task-1', ?, 'session_started', '{}', '2026-04-20 10:00:00'),
		  ('evt-2', 'task', 'task-1', ?, 'session_paused', '{}', '2026-04-20 10:15:00'),
		  ('evt-3', 'task', 'task-1', ?, 'session_resumed', '{}', '2026-04-20 10:30:00')
	`, actorID, actorID, actorID); err != nil {
		t.Fatalf("seed active session history failed: %v", err)
	}

	originalNow := sessionNow
	sessionNow = func() time.Time {
		return time.Date(2026, time.April, 20, 11, 0, 0, 0, time.UTC)
	}
	t.Cleanup(func() {
		sessionNow = originalNow
	})

	got, err := DeriveElapsedTaskTime(context.Background(), db, "TASK-1")
	if err != nil {
		t.Fatalf("DeriveElapsedTaskTime() error = %v", err)
	}

	want := 45 * time.Minute
	if got != want {
		t.Fatalf("DeriveElapsedTaskTime() = %s, want %s", got, want)
	}
}

func TestDeriveTaskElapsedRejectsInvalidEventSequences(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)
	actorID := insertActor(t, db, "actor-1", "ACT-1", "agent", "codex", "agent-1")
	insertTask(t, db, "task-1", "TASK-1", "Track session", nil, nil)

	if _, err := db.Exec(`
		INSERT INTO events(uuid, entity_type, entity_uuid, actor_id, event_type, payload_json, occurred_at)
		VALUES ('evt-1', 'task', 'task-1', ?, 'session_paused', '{}', '2026-04-20 10:15:00')
	`, actorID); err != nil {
		t.Fatalf("seed invalid session history failed: %v", err)
	}

	_, err := DeriveElapsedTaskTime(context.Background(), db, "TASK-1")
	if !errors.Is(err, ErrInvalidSessionHistory) {
		t.Fatalf("DeriveElapsedTaskTime() error = %v, want ErrInvalidSessionHistory", err)
	}
}
