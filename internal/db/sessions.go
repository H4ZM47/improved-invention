package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

var (
	ErrSessionActive         = errors.New("session already active")
	ErrSessionNotActive      = errors.New("session not active")
	ErrSessionNotPaused      = errors.New("session not paused")
	ErrSessionOnClosedTask   = errors.New("session cannot start on closed task")
	ErrInvalidSessionHistory = errors.New("invalid session history")
)

var sessionNow = time.Now

// SessionEventInput identifies the task and actor for a session event.
type SessionEventInput struct {
	TaskReference string
	ActorID       int64
}

// StartTaskSession records a session_started event for a claimed task.
func StartTaskSession(ctx context.Context, db *sql.DB, input SessionEventInput) error {
	return recordTaskSessionEvent(ctx, db, input, "session_started")
}

// PauseTaskSession records a session_paused event for a claimed task.
func PauseTaskSession(ctx context.Context, db *sql.DB, input SessionEventInput) error {
	return recordTaskSessionEvent(ctx, db, input, "session_paused")
}

// ResumeTaskSession records a session_resumed event for a claimed task.
func ResumeTaskSession(ctx context.Context, db *sql.DB, input SessionEventInput) error {
	return recordTaskSessionEvent(ctx, db, input, "session_resumed")
}

// CloseTaskSession records a session_closed event for a claimed task.
func CloseTaskSession(ctx context.Context, db *sql.DB, input SessionEventInput) error {
	return recordTaskSessionEvent(ctx, db, input, "session_closed")
}

// DeriveElapsedTaskTime replays task session events and returns total elapsed work time.
func DeriveElapsedTaskTime(ctx context.Context, db *sql.DB, taskReference string) (time.Duration, error) {
	task, err := FindTask(ctx, db, taskReference)
	if err != nil {
		return 0, err
	}

	events, err := loadTaskSessionEvents(ctx, db, task.UUID)
	if err != nil {
		return 0, err
	}

	total, _, _, err := replayTaskSessionEvents(events, sessionNow().UTC())
	if err != nil {
		return 0, err
	}

	return total, nil
}

type taskSessionEvent struct {
	EventType  string
	OccurredAt time.Time
}

type taskSessionState string

const (
	taskSessionIdle   taskSessionState = "idle"
	taskSessionActive taskSessionState = "active"
	taskSessionPaused taskSessionState = "paused"
)

func recordTaskSessionEvent(ctx context.Context, db *sql.DB, input SessionEventInput, eventType string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin %s transaction: %w", eventType, err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	task, err := findTaskTx(ctx, tx, input.TaskReference)
	if err != nil {
		return err
	}
	if err := RequireClaimHeld(ctx, tx, task.ID, input.ActorID); err != nil {
		return err
	}

	events, err := loadTaskSessionEvents(ctx, tx, task.UUID)
	if err != nil {
		return err
	}

	_, state, _, err := replayTaskSessionEvents(events, sessionNow().UTC())
	if err != nil {
		return err
	}

	if err := validateSessionEventTransition(task.Status, state, eventType); err != nil {
		return err
	}

	if err := appendEventTx(ctx, tx, eventInput{
		EntityType: "task",
		EntityUUID: task.UUID,
		ActorID:    &input.ActorID,
		EventType:  eventType,
		Payload: map[string]any{
			"task_handle": task.Handle,
			"status":      task.Status,
		},
	}); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit %s transaction: %w", eventType, err)
	}

	return nil
}

type sessionEventQuerier interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func loadTaskSessionEvents(ctx context.Context, runner sessionEventQuerier, taskUUID string) ([]taskSessionEvent, error) {
	rows, err := runner.QueryContext(ctx, `
		SELECT event_type, occurred_at
		FROM events
		WHERE entity_type = 'task'
		  AND entity_uuid = ?
		  AND event_type IN ('session_started', 'session_paused', 'session_resumed', 'session_closed')
		ORDER BY datetime(occurred_at) ASC, id ASC
	`, taskUUID)
	if err != nil {
		return nil, fmt.Errorf("load task session events: %w", err)
	}
	defer rows.Close()

	var events []taskSessionEvent
	for rows.Next() {
		var rawOccurredAt string
		var event taskSessionEvent
		if err := rows.Scan(&event.EventType, &rawOccurredAt); err != nil {
			return nil, fmt.Errorf("scan task session event: %w", err)
		}
		event.OccurredAt, err = parseEventTime(rawOccurredAt)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate task session events: %w", err)
	}

	return events, nil
}

func replayTaskSessionEvents(events []taskSessionEvent, now time.Time) (time.Duration, taskSessionState, *time.Time, error) {
	total := time.Duration(0)
	state := taskSessionIdle
	var startedAt *time.Time

	for _, event := range events {
		switch event.EventType {
		case "session_started", "session_resumed":
			if state == taskSessionActive {
				return 0, "", nil, fmt.Errorf("%w: %s while active", ErrInvalidSessionHistory, event.EventType)
			}
			if event.EventType == "session_resumed" && state != taskSessionPaused {
				return 0, "", nil, fmt.Errorf("%w: resume without pause", ErrInvalidSessionHistory)
			}
			start := event.OccurredAt.UTC()
			startedAt = &start
			state = taskSessionActive
		case "session_paused", "session_closed":
			if state == taskSessionIdle {
				return 0, "", nil, fmt.Errorf("%w: %s without active session", ErrInvalidSessionHistory, event.EventType)
			}
			if startedAt == nil {
				return 0, "", nil, fmt.Errorf("%w: missing session start", ErrInvalidSessionHistory)
			}
			if event.OccurredAt.Before(*startedAt) {
				return 0, "", nil, fmt.Errorf("%w: event time moved backwards", ErrInvalidSessionHistory)
			}
			total += event.OccurredAt.Sub(*startedAt)
			startedAt = nil
			if event.EventType == "session_paused" {
				state = taskSessionPaused
			} else {
				state = taskSessionIdle
			}
		default:
			return 0, "", nil, fmt.Errorf("%w: unsupported event type %s", ErrInvalidSessionHistory, event.EventType)
		}
	}

	if state == taskSessionActive && startedAt != nil {
		if now.Before(*startedAt) {
			return 0, "", nil, fmt.Errorf("%w: current time predates active session", ErrInvalidSessionHistory)
		}
		total += now.Sub(*startedAt)
	}

	return total, state, startedAt, nil
}

func validateSessionEventTransition(taskStatus string, state taskSessionState, eventType string) error {
	switch eventType {
	case "session_started":
		if isClosedStatus(taskStatus) {
			return ErrSessionOnClosedTask
		}
		if state != taskSessionIdle {
			return ErrSessionActive
		}
		return nil
	case "session_paused":
		if state != taskSessionActive {
			return ErrSessionNotActive
		}
		return nil
	case "session_resumed":
		if isClosedStatus(taskStatus) {
			return ErrSessionOnClosedTask
		}
		if state == taskSessionActive {
			return ErrSessionActive
		}
		if state != taskSessionPaused {
			return ErrSessionNotPaused
		}
		return nil
	case "session_closed":
		if state == taskSessionIdle {
			return ErrSessionNotActive
		}
		return nil
	default:
		return fmt.Errorf("unsupported session event %q", eventType)
	}
}

func parseEventTime(value string) (time.Time, error) {
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05"} {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return parsed.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("parse event time %q: %w", value, ErrInvalidSessionHistory)
}
