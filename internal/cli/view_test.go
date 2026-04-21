package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/H4ZM47/task-cli/internal/app"
	taskconfig "github.com/H4ZM47/task-cli/internal/config"
	taskdb "github.com/H4ZM47/task-cli/internal/db"
)

func seedViewFixtures(t *testing.T) string {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "task.db")
	cfg := taskconfig.Resolved{DBPath: dbPath, BusyTimeout: 5 * time.Second}

	db, err := taskdb.Open(context.Background(), cfg)
	if err != nil {
		t.Fatalf("taskdb.Open() error = %v", err)
	}
	defer db.Close()

	if _, err := (app.ActorManager{DB: db, HumanName: "alex"}).BootstrapConfiguredHumanActor(context.Background()); err != nil {
		t.Fatalf("BootstrapConfiguredHumanActor() error = %v", err)
	}
	taskMgr := app.TaskManager{DB: db, HumanName: "alex"}
	if _, err := taskMgr.Create(context.Background(), app.CreateTaskRequest{
		Title: "Urgent fix",
		Tags:  []string{"urgent"},
	}); err != nil {
		t.Fatalf("CreateTask(Urgent) error = %v", err)
	}
	if _, err := taskMgr.Create(context.Background(), app.CreateTaskRequest{
		Title: "Routine chore",
		Tags:  []string{"routine"},
	}); err != nil {
		t.Fatalf("CreateTask(Routine) error = %v", err)
	}

	return dbPath
}

func runViewSubcommand(t *testing.T, dbPath string, jsonOutput bool, args ...string) string {
	t.Helper()

	opts := &GlobalOptions{DBPath: dbPath, JSON: jsonOutput}
	cmd := newViewCommand(opts)
	cmd.SetArgs(args)
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute(%v) error = %v\noutput: %s", args, err, stdout.String())
	}
	return stdout.String()
}

func TestViewCreateListApplyDeleteLifecycle(t *testing.T) {
	t.Parallel()

	dbPath := seedViewFixtures(t)

	runViewSubcommand(t, dbPath, false, "create", "Urgent Tasks", "--tag", "urgent")

	listOut := runViewSubcommand(t, dbPath, false, "list")
	if !strings.Contains(listOut, "Urgent Tasks") {
		t.Fatalf("list output missing view name:\n%s", listOut)
	}

	applyJSON := runViewSubcommand(t, dbPath, true, "apply", "Urgent Tasks")
	var payload struct {
		OK   bool `json:"ok"`
		Data struct {
			Items []struct {
				Title string   `json:"title"`
				Tags  []string `json:"tags"`
			} `json:"items"`
		} `json:"data"`
		Meta struct {
			Count int    `json:"count"`
			View  string `json:"view"`
		} `json:"meta"`
	}
	if err := json.Unmarshal([]byte(applyJSON), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\noutput: %s", err, applyJSON)
	}
	if !payload.OK {
		t.Fatalf("payload.OK = false\noutput: %s", applyJSON)
	}
	if payload.Meta.Count != 1 {
		t.Fatalf("payload.Meta.Count = %d, want 1\noutput: %s", payload.Meta.Count, applyJSON)
	}
	if payload.Data.Items[0].Title != "Urgent fix" {
		t.Fatalf("applied item title = %q, want 'Urgent fix'", payload.Data.Items[0].Title)
	}
	if payload.Meta.View != "Urgent Tasks" {
		t.Fatalf("payload.Meta.View = %q, want 'Urgent Tasks'", payload.Meta.View)
	}

	runViewSubcommand(t, dbPath, false, "delete", "Urgent Tasks")

	listAfter := runViewSubcommand(t, dbPath, false, "list")
	if strings.Contains(listAfter, "Urgent Tasks") {
		t.Fatalf("list output still contains deleted view:\n%s", listAfter)
	}
}

func TestViewUpdateReplacesFiltersAndCanRename(t *testing.T) {
	t.Parallel()

	dbPath := seedViewFixtures(t)

	runViewSubcommand(t, dbPath, false, "create", "Triage", "--tag", "urgent")
	runViewSubcommand(t, dbPath, false, "update", "Triage", "--tag", "routine", "--rename", "Chores")

	showJSON := runViewSubcommand(t, dbPath, true, "show", "Chores")
	var payload struct {
		Data struct {
			View struct {
				Name    string `json:"name"`
				Filters struct {
					Tags []string `json:"tags"`
				} `json:"filters"`
			} `json:"view"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(showJSON), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\noutput: %s", err, showJSON)
	}
	if payload.Data.View.Name != "Chores" {
		t.Fatalf("view.Name = %q, want 'Chores'", payload.Data.View.Name)
	}
	if len(payload.Data.View.Filters.Tags) != 1 || payload.Data.View.Filters.Tags[0] != "routine" {
		t.Fatalf("view.Filters.Tags = %v, want [routine]", payload.Data.View.Filters.Tags)
	}
}

func TestViewApplyMissingViewReturnsError(t *testing.T) {
	t.Parallel()

	dbPath := seedViewFixtures(t)

	opts := &GlobalOptions{DBPath: dbPath}
	cmd := newViewCommand(opts)
	cmd.SetArgs([]string{"apply", "Missing"})
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() expected error for missing view")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("error = %v, want it to mention 'not found'", err)
	}
}
