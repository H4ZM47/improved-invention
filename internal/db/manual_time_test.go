package db

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestCreateManualTimeEntryRecordsEventAndDerivesDuration(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)
	actorID := insertActor(t, db, "actor-1", "ACT-1", "agent", "codex", "agent-1")
	insertTask(t, db, "task-1", "TASK-1", "Track manual time", nil, nil)

	if _, err := AcquireClaim(context.Background(), db, ClaimAcquireInput{
		TaskReference: "TASK-1",
		ActorID:       actorID,
		Lease:         time.Hour,
	}); err != nil {
		t.Fatalf("AcquireClaim() error = %v", err)
	}

	startedAt := time.Date(2026, time.April, 19, 9, 30, 0, 0, time.UTC)
	entry, err := CreateManualTimeEntry(context.Background(), db, ManualTimeEntryCreateInput{
		TaskReference: "TASK-1",
		ActorID:       actorID,
		StartedAt:     startedAt,
		Duration:      95 * time.Minute,
		Note:          "Imported from notes",
	})
	if err != nil {
		t.Fatalf("CreateManualTimeEntry() error = %v", err)
	}

	if entry.EntryID == "" {
		t.Fatal("CreateManualTimeEntry() returned empty entry id")
	}
	if got, want := entry.StartedAt, startedAt; !got.Equal(want) {
		t.Fatalf("StartedAt = %s, want %s", got, want)
	}
	if got, want := entry.Duration, 95*time.Minute; got != want {
		t.Fatalf("Duration = %s, want %s", got, want)
	}
	if got, want := entry.Note, "Imported from notes"; got != want {
		t.Fatalf("Note = %q, want %q", got, want)
	}

	var eventType string
	var payloadJSON string
	if err := db.QueryRow(`
		SELECT event_type, payload_json
		FROM events
		WHERE entity_type = 'task' AND entity_uuid = 'task-1'
		ORDER BY id DESC
		LIMIT 1
	`).Scan(&eventType, &payloadJSON); err != nil {
		t.Fatalf("query manual time event failed: %v", err)
	}

	if got, want := eventType, manualTimeEventAdded; got != want {
		t.Fatalf("event_type = %q, want %q", got, want)
	}
	for _, fragment := range []string{entry.EntryID, "Imported from notes", "duration_ms", "entry_id"} {
		if !strings.Contains(payloadJSON, fragment) {
			t.Fatalf("payload_json = %s, missing %q", payloadJSON, fragment)
		}
	}

	total, err := DeriveManualTaskTime(context.Background(), db, "TASK-1")
	if err != nil {
		t.Fatalf("DeriveManualTaskTime() error = %v", err)
	}
	if got, want := total, 95*time.Minute; got != want {
		t.Fatalf("DeriveManualTaskTime() = %s, want %s", got, want)
	}
}

func TestCreateManualTimeEntryRequiresClaim(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)
	actorID := insertActor(t, db, "actor-1", "ACT-1", "agent", "codex", "agent-1")
	insertTask(t, db, "task-1", "TASK-1", "Track manual time", nil, nil)

	_, err := CreateManualTimeEntry(context.Background(), db, ManualTimeEntryCreateInput{
		TaskReference: "TASK-1",
		ActorID:       actorID,
		StartedAt:     time.Date(2026, time.April, 19, 9, 30, 0, 0, time.UTC),
		Duration:      30 * time.Minute,
	})
	if !errors.Is(err, ErrClaimRequired) {
		t.Fatalf("CreateManualTimeEntry() error = %v, want ErrClaimRequired", err)
	}
}

func TestEditManualTimeEntryPreservesAuditHistoryAndUpdatesTotals(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)
	actorID := insertActor(t, db, "actor-1", "ACT-1", "agent", "codex", "agent-1")
	insertTask(t, db, "task-1", "TASK-1", "Track manual time", nil, nil)

	if _, err := AcquireClaim(context.Background(), db, ClaimAcquireInput{
		TaskReference: "TASK-1",
		ActorID:       actorID,
		Lease:         time.Hour,
	}); err != nil {
		t.Fatalf("AcquireClaim() error = %v", err)
	}

	created, err := CreateManualTimeEntry(context.Background(), db, ManualTimeEntryCreateInput{
		TaskReference: "TASK-1",
		ActorID:       actorID,
		StartedAt:     time.Date(2026, time.April, 19, 9, 30, 0, 0, time.UTC),
		Duration:      45 * time.Minute,
		Note:          "Initial import",
	})
	if err != nil {
		t.Fatalf("CreateManualTimeEntry() error = %v", err)
	}

	updatedStart := time.Date(2026, time.April, 19, 10, 0, 0, 0, time.UTC)
	updatedDuration := 75 * time.Minute
	updatedNote := "Corrected import"
	updated, err := EditManualTimeEntry(context.Background(), db, ManualTimeEntryEditInput{
		TaskReference: "TASK-1",
		EntryID:       created.EntryID,
		ActorID:       actorID,
		StartedAt:     &updatedStart,
		Duration:      &updatedDuration,
		Note:          &updatedNote,
	})
	if err != nil {
		t.Fatalf("EditManualTimeEntry() error = %v", err)
	}

	if got, want := updated.EntryID, created.EntryID; got != want {
		t.Fatalf("EntryID = %q, want %q", got, want)
	}
	if got, want := updated.StartedAt, updatedStart; !got.Equal(want) {
		t.Fatalf("StartedAt = %s, want %s", got, want)
	}
	if got, want := updated.Duration, updatedDuration; got != want {
		t.Fatalf("Duration = %s, want %s", got, want)
	}
	if got, want := updated.Note, updatedNote; got != want {
		t.Fatalf("Note = %q, want %q", got, want)
	}
	if updated.UpdatedAt.Before(updated.CreatedAt) {
		t.Fatalf("UpdatedAt = %s, want on or after CreatedAt %s", updated.UpdatedAt, updated.CreatedAt)
	}

	entries, err := ListManualTimeEntries(context.Background(), db, "TASK-1")
	if err != nil {
		t.Fatalf("ListManualTimeEntries() error = %v", err)
	}
	if got, want := len(entries), 1; got != want {
		t.Fatalf("len(ListManualTimeEntries()) = %d, want %d", got, want)
	}

	var events []string
	rows, err := db.Query(`
		SELECT event_type
		FROM events
		WHERE entity_type = 'task' AND entity_uuid = 'task-1' AND event_type LIKE 'manual_time_%'
		ORDER BY id ASC
	`)
	if err != nil {
		t.Fatalf("query manual time history failed: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var eventType string
		if err := rows.Scan(&eventType); err != nil {
			t.Fatalf("scan manual time history failed: %v", err)
		}
		events = append(events, eventType)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate manual time history failed: %v", err)
	}

	wantEvents := []string{manualTimeEventAdded, manualTimeEventEdited}
	if len(events) != len(wantEvents) {
		t.Fatalf("len(events) = %d, want %d (%v)", len(events), len(wantEvents), events)
	}
	for i := range wantEvents {
		if events[i] != wantEvents[i] {
			t.Fatalf("events[%d] = %q, want %q", i, events[i], wantEvents[i])
		}
	}

	total, err := DeriveManualTaskTime(context.Background(), db, "TASK-1")
	if err != nil {
		t.Fatalf("DeriveManualTaskTime() error = %v", err)
	}
	if got, want := total, updatedDuration; got != want {
		t.Fatalf("DeriveManualTaskTime() = %s, want %s", got, want)
	}
}

func TestEditManualTimeEntryRequiresExistingEntry(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)
	actorID := insertActor(t, db, "actor-1", "ACT-1", "agent", "codex", "agent-1")
	insertTask(t, db, "task-1", "TASK-1", "Track manual time", nil, nil)

	if _, err := AcquireClaim(context.Background(), db, ClaimAcquireInput{
		TaskReference: "TASK-1",
		ActorID:       actorID,
		Lease:         time.Hour,
	}); err != nil {
		t.Fatalf("AcquireClaim() error = %v", err)
	}

	updatedDuration := 20 * time.Minute
	_, err := EditManualTimeEntry(context.Background(), db, ManualTimeEntryEditInput{
		TaskReference: "TASK-1",
		EntryID:       "missing-entry",
		ActorID:       actorID,
		Duration:      &updatedDuration,
	})
	if !errors.Is(err, ErrManualTimeEntryNotFound) {
		t.Fatalf("EditManualTimeEntry() error = %v, want ErrManualTimeEntryNotFound", err)
	}
}

func TestDeriveManualTaskTimeRejectsInvalidHistory(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)
	insertTask(t, db, "task-1", "TASK-1", "Track manual time", nil, nil)

	if _, err := db.Exec(`
		INSERT INTO events(uuid, entity_type, entity_uuid, event_type, payload_json, occurred_at)
		VALUES ('evt-1', 'task', 'task-1', ?, ?, '2026-04-20 10:00:00')
		`, manualTimeEventEdited, `{"entry_id":"entry-1","started_at":"2026-04-19T09:00:00Z","duration_ms":1800000,"note":"bad history"}`); err != nil {
		t.Fatalf("seed invalid manual time history failed: %v", err)
	}

	_, err := DeriveManualTaskTime(context.Background(), db, "TASK-1")
	if !errors.Is(err, ErrInvalidManualTimeHistory) {
		t.Fatalf("DeriveManualTaskTime() error = %v, want ErrInvalidManualTimeHistory", err)
	}
}

func TestListManualTimeEntriesOrdersByStartedAt(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)
	insertTask(t, db, "task-1", "TASK-1", "Track manual time", nil, nil)

	if _, err := db.Exec(`
		INSERT INTO events(uuid, entity_type, entity_uuid, event_type, payload_json, occurred_at)
		VALUES
		  ('evt-1', 'task', 'task-1', ?, ?, '2026-04-20 12:00:00'),
		  ('evt-2', 'task', 'task-1', ?, ?, '2026-04-20 12:05:00')
	`,
		manualTimeEventAdded, `{"entry_id":"entry-b","started_at":"2026-04-19T11:00:00Z","duration_ms":1800000,"note":"later"}`,
		manualTimeEventAdded, `{"entry_id":"entry-a","started_at":"2026-04-19T09:00:00Z","duration_ms":900000,"note":"earlier"}`,
	); err != nil {
		t.Fatalf("seed manual time entries failed: %v", err)
	}

	entries, err := ListManualTimeEntries(context.Background(), db, "TASK-1")
	if err != nil {
		t.Fatalf("ListManualTimeEntries() error = %v", err)
	}
	if got, want := len(entries), 2; got != want {
		t.Fatalf("len(entries) = %d, want %d", got, want)
	}
	if got, want := entries[0].EntryID, "entry-a"; got != want {
		t.Fatalf("entries[0].EntryID = %q, want %q", got, want)
	}
	if got, want := entries[1].EntryID, "entry-b"; got != want {
		t.Fatalf("entries[1].EntryID = %q, want %q", got, want)
	}
}
