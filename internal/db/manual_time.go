package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	manualTimeEventAdded  = "manual_time_added"
	manualTimeEventEdited = "manual_time_edited"
)

var (
	ErrInvalidManualTimeEntry   = errors.New("invalid manual time entry")
	ErrInvalidManualTimeHistory = errors.New("invalid manual time history")
	ErrManualTimeEntryNotFound  = errors.New("manual time entry not found")
	ErrManualTimeEditEmpty      = errors.New("manual time edit requires changes")
)

// ManualTimeEntry is the current materialized view of one manual task time record.
type ManualTimeEntry struct {
	EntryID    string
	TaskUUID   string
	TaskHandle string
	StartedAt  time.Time
	Duration   time.Duration
	Note       string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// ManualTimeEntryCreateInput describes the fields required to add a manual time entry.
type ManualTimeEntryCreateInput struct {
	TaskReference string
	ActorID       int64
	StartedAt     time.Time
	Duration      time.Duration
	Note          string
}

// ManualTimeEntryEditInput describes a partial edit to an existing manual time entry.
type ManualTimeEntryEditInput struct {
	TaskReference string
	EntryID       string
	ActorID       int64
	StartedAt     *time.Time
	Duration      *time.Duration
	Note          *string
}

// CreateManualTimeEntry records a manual_time_added event for a claimed task.
func CreateManualTimeEntry(ctx context.Context, db *sql.DB, input ManualTimeEntryCreateInput) (ManualTimeEntry, error) {
	startedAt, durationMS, err := normalizeManualTimeValues(input.StartedAt, input.Duration)
	if err != nil {
		return ManualTimeEntry{}, err
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return ManualTimeEntry{}, fmt.Errorf("begin create manual time transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	task, err := findTaskTx(ctx, tx, input.TaskReference)
	if err != nil {
		return ManualTimeEntry{}, err
	}
	if err := RequireClaimHeld(ctx, tx, task.ID, input.ActorID); err != nil {
		return ManualTimeEntry{}, err
	}

	entryID := uuid.NewString()
	if err := appendEventTx(ctx, tx, eventInput{
		EntityType: "task",
		EntityUUID: task.UUID,
		ActorID:    &input.ActorID,
		EventType:  manualTimeEventAdded,
		Payload: map[string]any{
			"task_handle":  task.Handle,
			"entry_id":     entryID,
			"started_at":   startedAt.Format(time.RFC3339),
			"duration_ms":  durationMS,
			"note":         input.Note,
			"recorded_via": "manual_time",
		},
	}); err != nil {
		return ManualTimeEntry{}, err
	}

	entries, err := listManualTimeEntriesTx(ctx, tx, task.UUID, task.Handle)
	if err != nil {
		return ManualTimeEntry{}, err
	}

	entry, err := findManualTimeEntry(entries, entryID)
	if err != nil {
		return ManualTimeEntry{}, err
	}

	if err := tx.Commit(); err != nil {
		return ManualTimeEntry{}, fmt.Errorf("commit create manual time transaction: %w", err)
	}

	return entry, nil
}

// EditManualTimeEntry records a manual_time_edited event for a claimed task.
func EditManualTimeEntry(ctx context.Context, db *sql.DB, input ManualTimeEntryEditInput) (ManualTimeEntry, error) {
	if input.StartedAt == nil && input.Duration == nil && input.Note == nil {
		return ManualTimeEntry{}, ErrManualTimeEditEmpty
	}
	if input.EntryID == "" {
		return ManualTimeEntry{}, fmt.Errorf("%w: missing entry id", ErrInvalidManualTimeEntry)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return ManualTimeEntry{}, fmt.Errorf("begin edit manual time transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	task, err := findTaskTx(ctx, tx, input.TaskReference)
	if err != nil {
		return ManualTimeEntry{}, err
	}
	if err := RequireClaimHeld(ctx, tx, task.ID, input.ActorID); err != nil {
		return ManualTimeEntry{}, err
	}

	entries, err := listManualTimeEntriesTx(ctx, tx, task.UUID, task.Handle)
	if err != nil {
		return ManualTimeEntry{}, err
	}

	current, err := findManualTimeEntry(entries, input.EntryID)
	if err != nil {
		return ManualTimeEntry{}, err
	}

	next := current
	if input.StartedAt != nil {
		next.StartedAt = input.StartedAt.UTC()
	}
	if input.Duration != nil {
		next.Duration = *input.Duration
	}
	if input.Note != nil {
		next.Note = *input.Note
	}

	startedAt, durationMS, err := normalizeManualTimeValues(next.StartedAt, next.Duration)
	if err != nil {
		return ManualTimeEntry{}, err
	}

	if err := appendEventTx(ctx, tx, eventInput{
		EntityType: "task",
		EntityUUID: task.UUID,
		ActorID:    &input.ActorID,
		EventType:  manualTimeEventEdited,
		Payload: map[string]any{
			"task_handle":          task.Handle,
			"entry_id":             current.EntryID,
			"started_at":           startedAt.Format(time.RFC3339),
			"duration_ms":          durationMS,
			"note":                 next.Note,
			"previous_started_at":  current.StartedAt.UTC().Format(time.RFC3339),
			"previous_duration_ms": current.Duration.Milliseconds(),
			"previous_note":        current.Note,
			"recorded_via":         "manual_time",
		},
	}); err != nil {
		return ManualTimeEntry{}, err
	}

	entries, err = listManualTimeEntriesTx(ctx, tx, task.UUID, task.Handle)
	if err != nil {
		return ManualTimeEntry{}, err
	}

	entry, err := findManualTimeEntry(entries, current.EntryID)
	if err != nil {
		return ManualTimeEntry{}, err
	}

	if err := tx.Commit(); err != nil {
		return ManualTimeEntry{}, fmt.Errorf("commit edit manual time transaction: %w", err)
	}

	return entry, nil
}

// ListManualTimeEntries returns the materialized current manual entries for a task.
func ListManualTimeEntries(ctx context.Context, db *sql.DB, taskReference string) ([]ManualTimeEntry, error) {
	task, err := FindTask(ctx, db, taskReference)
	if err != nil {
		return nil, err
	}
	return listManualTimeEntriesTx(ctx, db, task.UUID, task.Handle)
}

// DeriveManualTaskTime replays manual time events and returns the current total.
func DeriveManualTaskTime(ctx context.Context, db *sql.DB, taskReference string) (time.Duration, error) {
	entries, err := ListManualTimeEntries(ctx, db, taskReference)
	if err != nil {
		return 0, err
	}

	total := time.Duration(0)
	for _, entry := range entries {
		total += entry.Duration
	}

	return total, nil
}

type manualTimeEventRecord struct {
	EventType  string
	PayloadRaw string
	OccurredAt time.Time
}

type manualTimeEventPayload struct {
	EntryID            string `json:"entry_id"`
	StartedAt          string `json:"started_at"`
	DurationMS         int64  `json:"duration_ms"`
	Note               string `json:"note"`
	PreviousStartedAt  string `json:"previous_started_at"`
	PreviousDurationMS int64  `json:"previous_duration_ms"`
	PreviousNote       string `json:"previous_note"`
}

type manualTimeEventQuerier interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func listManualTimeEntriesTx(ctx context.Context, runner manualTimeEventQuerier, taskUUID string, taskHandle string) ([]ManualTimeEntry, error) {
	events, err := loadManualTimeEvents(ctx, runner, taskUUID)
	if err != nil {
		return nil, err
	}
	return replayManualTimeEntries(taskUUID, taskHandle, events)
}

func loadManualTimeEvents(ctx context.Context, runner manualTimeEventQuerier, taskUUID string) ([]manualTimeEventRecord, error) {
	rows, err := runner.QueryContext(ctx, `
		SELECT event_type, payload_json, occurred_at
		FROM events
		WHERE entity_type = 'task'
		  AND entity_uuid = ?
		  AND event_type IN (?, ?)
		ORDER BY datetime(occurred_at) ASC, id ASC
	`, taskUUID, manualTimeEventAdded, manualTimeEventEdited)
	if err != nil {
		return nil, fmt.Errorf("load manual time events: %w", err)
	}
	defer rows.Close()

	var events []manualTimeEventRecord
	for rows.Next() {
		var event manualTimeEventRecord
		var rawOccurredAt string
		if err := rows.Scan(&event.EventType, &event.PayloadRaw, &rawOccurredAt); err != nil {
			return nil, fmt.Errorf("scan manual time event: %w", err)
		}
		event.OccurredAt, err = parseEventTime(rawOccurredAt)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidManualTimeHistory, err)
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate manual time events: %w", err)
	}

	return events, nil
}

func replayManualTimeEntries(taskUUID string, taskHandle string, events []manualTimeEventRecord) ([]ManualTimeEntry, error) {
	entries := make(map[string]ManualTimeEntry, len(events))

	for _, event := range events {
		var payload manualTimeEventPayload
		if err := json.Unmarshal([]byte(event.PayloadRaw), &payload); err != nil {
			return nil, fmt.Errorf("%w: decode payload: %v", ErrInvalidManualTimeHistory, err)
		}

		startedAt, duration, err := manualTimeValuesFromPayload(payload)
		if err != nil {
			return nil, err
		}

		switch event.EventType {
		case manualTimeEventAdded:
			if payload.EntryID == "" {
				return nil, fmt.Errorf("%w: add event missing entry id", ErrInvalidManualTimeHistory)
			}
			if _, exists := entries[payload.EntryID]; exists {
				return nil, fmt.Errorf("%w: duplicate entry %s", ErrInvalidManualTimeHistory, payload.EntryID)
			}
			entries[payload.EntryID] = ManualTimeEntry{
				EntryID:    payload.EntryID,
				TaskUUID:   taskUUID,
				TaskHandle: taskHandle,
				StartedAt:  startedAt,
				Duration:   duration,
				Note:       payload.Note,
				CreatedAt:  event.OccurredAt,
				UpdatedAt:  event.OccurredAt,
			}
		case manualTimeEventEdited:
			current, exists := entries[payload.EntryID]
			if !exists {
				return nil, fmt.Errorf("%w: edit before add for %s", ErrInvalidManualTimeHistory, payload.EntryID)
			}
			current.StartedAt = startedAt
			current.Duration = duration
			current.Note = payload.Note
			current.UpdatedAt = event.OccurredAt
			entries[payload.EntryID] = current
		default:
			return nil, fmt.Errorf("%w: unsupported event type %s", ErrInvalidManualTimeHistory, event.EventType)
		}
	}

	result := make([]ManualTimeEntry, 0, len(entries))
	for _, entry := range entries {
		result = append(result, entry)
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].StartedAt.Equal(result[j].StartedAt) {
			return result[i].EntryID < result[j].EntryID
		}
		return result[i].StartedAt.Before(result[j].StartedAt)
	})

	return result, nil
}

func findManualTimeEntry(entries []ManualTimeEntry, entryID string) (ManualTimeEntry, error) {
	for _, entry := range entries {
		if entry.EntryID == entryID {
			return entry, nil
		}
	}
	return ManualTimeEntry{}, ErrManualTimeEntryNotFound
}

func normalizeManualTimeValues(startedAt time.Time, duration time.Duration) (time.Time, int64, error) {
	if startedAt.IsZero() {
		return time.Time{}, 0, fmt.Errorf("%w: started_at is required", ErrInvalidManualTimeEntry)
	}
	if duration <= 0 {
		return time.Time{}, 0, fmt.Errorf("%w: duration must be positive", ErrInvalidManualTimeEntry)
	}

	normalizedStart := startedAt.UTC().Round(0)
	durationMS := duration.Milliseconds()
	if durationMS <= 0 {
		return time.Time{}, 0, fmt.Errorf("%w: duration must be at least 1ms", ErrInvalidManualTimeEntry)
	}

	return normalizedStart, durationMS, nil
}

func manualTimeValuesFromPayload(payload manualTimeEventPayload) (time.Time, time.Duration, error) {
	if strings.TrimSpace(payload.EntryID) == "" {
		return time.Time{}, 0, fmt.Errorf("%w: missing entry id", ErrInvalidManualTimeHistory)
	}

	startedAt, err := parseEventTime(payload.StartedAt)
	if err != nil {
		return time.Time{}, 0, fmt.Errorf("%w: invalid started_at: %v", ErrInvalidManualTimeHistory, err)
	}
	if payload.DurationMS <= 0 {
		return time.Time{}, 0, fmt.Errorf("%w: duration must be positive", ErrInvalidManualTimeHistory)
	}

	return startedAt, time.Duration(payload.DurationMS) * time.Millisecond, nil
}
