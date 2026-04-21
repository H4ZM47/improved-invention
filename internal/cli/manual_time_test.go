package cli

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/H4ZM47/grind/internal/app"
	taskconfig "github.com/H4ZM47/grind/internal/config"
	taskdb "github.com/H4ZM47/grind/internal/db"
	"github.com/spf13/cobra"
)

func TestTaskTimeEditCreatesEntryInteractively(t *testing.T) {
	t.Parallel()

	dbPath, taskHandle := seedClaimedTaskForManualTimeCLI(t)
	requireCommandPath(t, NewRootCommand(BuildInfo{}), "time", "edit")

	startedAt := time.Date(2026, time.April, 21, 9, 0, 0, 0, time.UTC)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	root := NewRootCommand(BuildInfo{})
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetIn(strings.NewReader(startedAt.Format(time.RFC3339) + "\n45m\nImported from calendar\n"))
	root.SetArgs([]string{
		"--db", dbPath,
		"--actor", "alex",
		"--json",
		"time", "edit", taskHandle,
	})

	if err := root.Execute(); err != nil {
		t.Fatalf("grind time edit Execute() error = %v; stderr=%q", err, stderr.String())
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
		Meta struct {
			Mode string `json:"mode"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal(stdout) error = %v; stdout=%q", err, stdout.String())
	}

	if !payload.OK {
		t.Fatal("payload.OK = false, want true")
	}
	if got, want := payload.Command, "grind time edit"; got != want {
		t.Fatalf("payload.Command = %q, want %q", got, want)
	}
	if got, want := payload.Meta.Mode, "created"; got != want {
		t.Fatalf("payload.Meta.Mode = %q, want %q", got, want)
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
}

func TestTaskTimeEditUpdatesEntryInteractively(t *testing.T) {
	t.Parallel()

	dbPath, taskHandle := seedClaimedTaskForManualTimeCLI(t)
	requireCommandPath(t, NewRootCommand(BuildInfo{}), "time", "edit")

	entryID := createManualTimeEntryViaManager(t, dbPath, taskHandle, time.Date(2026, time.April, 21, 9, 0, 0, 0, time.UTC), 30*time.Minute, "Initial import")
	updatedStart := time.Date(2026, time.April, 21, 10, 15, 0, 0, time.UTC)

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	root := NewRootCommand(BuildInfo{})
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetIn(strings.NewReader("1\n" + updatedStart.Format(time.RFC3339) + "\n75m\nCorrected import\n"))
	root.SetArgs([]string{
		"--db", dbPath,
		"--actor", "alex",
		"--json",
		"time", "edit", taskHandle,
	})

	if err := root.Execute(); err != nil {
		t.Fatalf("grind time edit Execute() error = %v; stderr=%q", err, stderr.String())
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
		Meta struct {
			Mode string `json:"mode"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal(stdout) error = %v; stdout=%q", err, stdout.String())
	}

	if !payload.OK {
		t.Fatal("payload.OK = false, want true")
	}
	if got, want := payload.Meta.Mode, "edited"; got != want {
		t.Fatalf("payload.Meta.Mode = %q, want %q", got, want)
	}
	if got, want := payload.Data.Entry.EntryID, entryID; got != want {
		t.Fatalf("payload.Data.Entry.EntryID = %q, want %q", got, want)
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
}

func TestTaskTimeEditRejectsNoInput(t *testing.T) {
	t.Parallel()

	dbPath, taskHandle := seedClaimedTaskForManualTimeCLI(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := Run(BuildInfo{}, []string{"--db", dbPath, "--actor", "alex", "--json", "--no-input", "time", "edit", taskHandle}, &stdout, &stderr)
	if got, want := exitCode, 11; got != want {
		t.Fatalf("exitCode = %d, want %d; stdout=%s", got, want, stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	if got := stdout.String(); !containsAll(got, []string{`"code": "VALIDATION_ERROR"`, "`grind time edit` is interactive-only"}) {
		t.Fatalf("stdout = %q, want interactive-only validation message", got)
	}
}

func TestRetiredTimeAddReturnsGuidance(t *testing.T) {
	t.Parallel()

	dbPath, taskHandle := seedClaimedTaskForManualTimeCLI(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := Run(BuildInfo{}, []string{"--db", dbPath, "--json", "time", "add", taskHandle}, &stdout, &stderr)
	if got, want := exitCode, 10; got != want {
		t.Fatalf("exitCode = %d, want %d; stdout=%s", got, want, stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	if got := stdout.String(); !containsAll(got, []string{`"code": "INVALID_ARGS"`, "`grind time add TASK-1` was removed", "`grind time edit TASK-1`"}) {
		t.Fatalf("stdout = %q, want migration guidance", got)
	}
}

func createManualTimeEntryViaManager(t *testing.T, dbPath string, taskHandle string, startedAt time.Time, duration time.Duration, note string) string {
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

	manager := app.TaskManager{
		DB:        db,
		HumanName: "alex",
	}
	entry, err := manager.AddManualTime(context.Background(), app.AddManualTimeRequest{
		Reference: taskHandle,
		StartedAt: &startedAt,
		Duration:  duration,
		Note:      note,
	})
	if err != nil {
		t.Fatalf("AddManualTime() error = %v", err)
	}
	return entry.EntryID
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
