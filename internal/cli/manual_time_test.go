package cli

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/H4ZM47/improved-invention/internal/app"
	taskconfig "github.com/H4ZM47/improved-invention/internal/config"
	taskdb "github.com/H4ZM47/improved-invention/internal/db"
	"github.com/spf13/cobra"
)

func TestTaskTimeAddCommandEndToEnd(t *testing.T) {
	t.Parallel()

	dbPath, taskHandle := seedClaimedTaskForManualTimeCLI(t)
	requireCommandPath(t, NewRootCommand(BuildInfo{}), "time", "add")

	startedAt := time.Date(2026, time.April, 21, 9, 0, 0, 0, time.UTC)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	root := NewRootCommand(BuildInfo{})
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{
		"--db", dbPath,
		"--actor", "alex",
		"--json",
		"time", "add", taskHandle,
		"--started-at", startedAt.Format(time.RFC3339),
		"--duration", "45m",
		"--note", "Imported from calendar",
	})

	if err := root.Execute(); err != nil {
		t.Fatalf("task time add Execute() error = %v; stderr=%q", err, stderr.String())
	}

	var payload struct {
		OK      bool   `json:"ok"`
		Command string `json:"command"`
		Data    struct {
			Entry struct {
				EntryID         string `json:"entry_id"`
				TaskHandle      string `json:"task_handle"`
				StartedAt       string `json:"started_at"`
				DurationSeconds int64  `json:"duration_seconds"`
				Note            string `json:"note"`
			} `json:"entry"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal(stdout) error = %v; stdout=%q", err, stdout.String())
	}

	if !payload.OK {
		t.Fatal("payload.OK = false, want true")
	}
	if got, want := payload.Command, "task time add"; got != want {
		t.Fatalf("payload.Command = %q, want %q", got, want)
	}
	if payload.Data.Entry.EntryID == "" {
		t.Fatal("payload.Data.Entry.EntryID = empty, want generated entry ID")
	}
	if got, want := payload.Data.Entry.TaskHandle, taskHandle; got != want {
		t.Fatalf("payload.Data.Entry.TaskHandle = %q, want %q", got, want)
	}
	if got, want := payload.Data.Entry.StartedAt, startedAt.Format(time.RFC3339); got != want {
		t.Fatalf("payload.Data.Entry.StartedAt = %q, want %q", got, want)
	}
	if got, want := payload.Data.Entry.DurationSeconds, int64((45*time.Minute)/time.Second); got != want {
		t.Fatalf("payload.Data.Entry.DurationSeconds = %d, want %d", got, want)
	}
	if got, want := payload.Data.Entry.Note, "Imported from calendar"; got != want {
		t.Fatalf("payload.Data.Entry.Note = %q, want %q", got, want)
	}

	eventPayload := queryLatestManualTimeEventPayload(t, dbPath, taskHandle, "manual_time_added")
	if got, want := eventPayload["entry_id"].(string), payload.Data.Entry.EntryID; got != want {
		t.Fatalf("event payload entry_id = %q, want %q", got, want)
	}
	if got, want := eventPayload["started_at"].(string), startedAt.Format(time.RFC3339); got != want {
		t.Fatalf("event payload started_at = %q, want %q", got, want)
	}
	if got, want := int64(eventPayload["duration_ms"].(float64)), int64((45*time.Minute)/time.Millisecond); got != want {
		t.Fatalf("event payload duration_ms = %d, want %d", got, want)
	}
	if got, want := eventPayload["note"].(string), "Imported from calendar"; got != want {
		t.Fatalf("event payload note = %q, want %q", got, want)
	}
}

func TestTaskTimeEditCommandEndToEnd(t *testing.T) {
	t.Parallel()

	dbPath, taskHandle := seedClaimedTaskForManualTimeCLI(t)
	requireCommandPath(t, NewRootCommand(BuildInfo{}), "time", "edit")

	entryID := createManualTimeEntryViaCLI(t, dbPath, taskHandle, time.Date(2026, time.April, 21, 9, 0, 0, 0, time.UTC), "30m", "Initial import")

	updatedStart := time.Date(2026, time.April, 21, 10, 15, 0, 0, time.UTC)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	root := NewRootCommand(BuildInfo{})
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{
		"--db", dbPath,
		"--actor", "alex",
		"--json",
		"time", "edit", taskHandle, entryID,
		"--started-at", updatedStart.Format(time.RFC3339),
		"--duration", "75m",
		"--note", "Corrected import",
	})

	if err := root.Execute(); err != nil {
		t.Fatalf("task time edit Execute() error = %v; stderr=%q", err, stderr.String())
	}

	var payload struct {
		OK      bool   `json:"ok"`
		Command string `json:"command"`
		Data    struct {
			Entry struct {
				EntryID         string `json:"entry_id"`
				TaskHandle      string `json:"task_handle"`
				StartedAt       string `json:"started_at"`
				DurationSeconds int64  `json:"duration_seconds"`
				Note            string `json:"note"`
			} `json:"entry"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal(stdout) error = %v; stdout=%q", err, stdout.String())
	}

	if !payload.OK {
		t.Fatal("payload.OK = false, want true")
	}
	if got, want := payload.Command, "task time edit"; got != want {
		t.Fatalf("payload.Command = %q, want %q", got, want)
	}
	if got, want := payload.Data.Entry.EntryID, entryID; got != want {
		t.Fatalf("payload.Data.Entry.EntryID = %q, want %q", got, want)
	}
	if got, want := payload.Data.Entry.TaskHandle, taskHandle; got != want {
		t.Fatalf("payload.Data.Entry.TaskHandle = %q, want %q", got, want)
	}
	if got, want := payload.Data.Entry.StartedAt, updatedStart.Format(time.RFC3339); got != want {
		t.Fatalf("payload.Data.Entry.StartedAt = %q, want %q", got, want)
	}
	if got, want := payload.Data.Entry.DurationSeconds, int64((75*time.Minute)/time.Second); got != want {
		t.Fatalf("payload.Data.Entry.DurationSeconds = %d, want %d", got, want)
	}
	if got, want := payload.Data.Entry.Note, "Corrected import"; got != want {
		t.Fatalf("payload.Data.Entry.Note = %q, want %q", got, want)
	}

	eventPayload := queryLatestManualTimeEventPayload(t, dbPath, taskHandle, "manual_time_edited")
	if got, want := eventPayload["entry_id"].(string), entryID; got != want {
		t.Fatalf("event payload entry_id = %q, want %q", got, want)
	}
	if got, want := eventPayload["started_at"].(string), updatedStart.Format(time.RFC3339); got != want {
		t.Fatalf("event payload started_at = %q, want %q", got, want)
	}
	if got, want := int64(eventPayload["duration_ms"].(float64)), int64((75*time.Minute)/time.Millisecond); got != want {
		t.Fatalf("event payload duration_ms = %d, want %d", got, want)
	}
	if got, want := eventPayload["note"].(string), "Corrected import"; got != want {
		t.Fatalf("event payload note = %q, want %q", got, want)
	}
	if got, want := int64(eventPayload["previous_duration_ms"].(float64)), int64((30*time.Minute)/time.Millisecond); got != want {
		t.Fatalf("event payload previous_duration_ms = %d, want %d", got, want)
	}
	if got, want := eventPayload["previous_note"].(string), "Initial import"; got != want {
		t.Fatalf("event payload previous_note = %q, want %q", got, want)
	}
}

func createManualTimeEntryViaCLI(t *testing.T, dbPath string, taskHandle string, startedAt time.Time, duration string, note string) string {
	t.Helper()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	root := NewRootCommand(BuildInfo{})
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{
		"--db", dbPath,
		"--actor", "alex",
		"--json",
		"time", "add", taskHandle,
		"--started-at", startedAt.Format(time.RFC3339),
		"--duration", duration,
		"--note", note,
	})

	if err := root.Execute(); err != nil {
		t.Fatalf("task time add Execute() error = %v; stderr=%q", err, stderr.String())
	}

	var payload struct {
		Data struct {
			Entry struct {
				EntryID string `json:"entry_id"`
			} `json:"entry"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal(stdout) error = %v; stdout=%q", err, stdout.String())
	}
	if payload.Data.Entry.EntryID == "" {
		t.Fatal("payload.Data.Entry.EntryID = empty, want generated entry ID")
	}

	return payload.Data.Entry.EntryID
}

func seedClaimedTaskForManualTimeCLI(t *testing.T) (dbPath string, taskHandle string) {
	t.Helper()

	dbPath = filepath.Join(t.TempDir(), "task.db")
	cfg := taskconfig.Resolved{
		DBPath:      dbPath,
		BusyTimeout: 5 * time.Second,
	}

	db, err := taskdb.Open(context.Background(), cfg)
	if err != nil {
		t.Fatalf("taskdb.Open() error = %v", err)
	}
	defer db.Close()

	manager := app.TaskManager{
		DB:        db,
		HumanName: "alex",
	}
	task, err := manager.Create(context.Background(), app.CreateTaskRequest{
		Title: "Track manual time from CLI",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, err := manager.Claim(context.Background(), app.ClaimTaskRequest{
		Reference: task.Handle,
		Lease:     time.Hour,
	}); err != nil {
		t.Fatalf("Claim() error = %v", err)
	}

	return dbPath, task.Handle
}

func requireCommandPath(t *testing.T, root *cobra.Command, names ...string) *cobra.Command {
	t.Helper()

	current := root
	for _, name := range names {
		next := findNamedSubcommand(current, name)
		if next == nil {
			t.Skipf("command path %v not present on this branch yet", names)
		}
		current = next
	}

	return current
}

func findNamedSubcommand(root *cobra.Command, name string) *cobra.Command {
	for _, cmd := range root.Commands() {
		if cmd.Name() == name {
			return cmd
		}
	}
	return nil
}

func queryLatestManualTimeEventPayload(t *testing.T, dbPath string, taskHandle string, eventType string) map[string]any {
	t.Helper()

	cfg := taskconfig.Resolved{
		DBPath:      dbPath,
		BusyTimeout: 5 * time.Second,
	}
	db, err := taskdb.Open(context.Background(), cfg)
	if err != nil {
		t.Fatalf("taskdb.Open() error = %v", err)
	}
	defer db.Close()

	var taskUUID string
	if err := db.QueryRow(`SELECT uuid FROM tasks WHERE handle = ?`, taskHandle).Scan(&taskUUID); err != nil {
		t.Fatalf("lookup task uuid failed: %v", err)
	}

	var payloadJSON string
	err = db.QueryRow(`
		SELECT payload_json
		FROM events
		WHERE entity_type = 'task' AND entity_uuid = ? AND event_type = ?
		ORDER BY id DESC
		LIMIT 1
	`, taskUUID, eventType).Scan(&payloadJSON)
	if err != nil {
		if err == sql.ErrNoRows {
			t.Fatalf("no %s event found for %s", eventType, taskHandle)
		}
		t.Fatalf("query event payload failed: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(payloadJSON), &payload); err != nil {
		t.Fatalf("json.Unmarshal(payload) error = %v", err)
	}

	return payload
}
