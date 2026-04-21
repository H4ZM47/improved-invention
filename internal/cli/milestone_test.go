package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/H4ZM47/grind/internal/app"
	taskconfig "github.com/H4ZM47/grind/internal/config"
	taskdb "github.com/H4ZM47/grind/internal/db"
)

func TestMilestoneLifecycleCommandsJSON(t *testing.T) {
	t.Parallel()

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

	opts := &GlobalOptions{DBPath: dbPath, Actor: "alex", JSON: true}

	create := newMilestoneCreateCommand(opts)
	create.SetArgs([]string{"v1.0.2", "--description", "Ship milestone support"})
	var createOut bytes.Buffer
	create.SetOut(&createOut)
	create.SetErr(&createOut)
	if err := create.Execute(); err != nil {
		t.Fatalf("milestone create Execute() error = %v", err)
	}

	var created struct {
		Data struct {
			Milestone app.MilestoneRecord `json:"milestone"`
		} `json:"data"`
	}
	if err := json.Unmarshal(createOut.Bytes(), &created); err != nil {
		t.Fatalf("unmarshal milestone create output: %v", err)
	}
	if got, want := created.Data.Milestone.Handle, "MILE-1"; got != want {
		t.Fatalf("created milestone handle = %q, want %q", got, want)
	}

	cancel := newMilestoneCancelCommand(opts)
	cancel.SetArgs([]string{created.Data.Milestone.Handle})
	var cancelOut bytes.Buffer
	cancel.SetOut(&cancelOut)
	cancel.SetErr(&cancelOut)
	if err := cancel.Execute(); err != nil {
		t.Fatalf("milestone cancel Execute() error = %v", err)
	}

	open := newMilestoneOpenCommand(opts)
	open.SetArgs([]string{created.Data.Milestone.Handle})
	var openOut bytes.Buffer
	open.SetOut(&openOut)
	open.SetErr(&openOut)
	if err := open.Execute(); err != nil {
		t.Fatalf("milestone open Execute() error = %v", err)
	}

	show := newMilestoneShowCommand(opts)
	show.SetArgs([]string{created.Data.Milestone.Handle})
	var showOut bytes.Buffer
	show.SetOut(&showOut)
	show.SetErr(&showOut)
	if err := show.Execute(); err != nil {
		t.Fatalf("milestone show Execute() error = %v", err)
	}

	var shown struct {
		Data struct {
			Milestone app.MilestoneRecord `json:"milestone"`
		} `json:"data"`
	}
	if err := json.Unmarshal(showOut.Bytes(), &shown); err != nil {
		t.Fatalf("unmarshal milestone show output: %v", err)
	}
	if got, want := shown.Data.Milestone.Status, "backlog"; got != want {
		t.Fatalf("shown milestone status = %q, want %q", got, want)
	}
}

func TestTaskListJSONSupportsMilestoneFilter(t *testing.T) {
	t.Parallel()

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

	milestoneOne, err := (app.MilestoneManager{DB: db, HumanName: "alex"}).Create(context.Background(), app.CreateMilestoneRequest{Name: "v1.0.1"})
	if err != nil {
		t.Fatalf("CreateMilestone(v1.0.1) error = %v", err)
	}
	milestoneTwo, err := (app.MilestoneManager{DB: db, HumanName: "alex"}).Create(context.Background(), app.CreateMilestoneRequest{Name: "v1.0.2"})
	if err != nil {
		t.Fatalf("CreateMilestone(v1.0.2) error = %v", err)
	}

	taskManager := app.TaskManager{DB: db, HumanName: "alex"}
	if _, err := taskManager.Create(context.Background(), app.CreateTaskRequest{
		Title:        "Included task",
		MilestoneRef: &milestoneOne.Handle,
	}); err != nil {
		t.Fatalf("CreateTask(included) error = %v", err)
	}
	if _, err := taskManager.Create(context.Background(), app.CreateTaskRequest{
		Title:        "Excluded task",
		MilestoneRef: &milestoneTwo.Handle,
	}); err != nil {
		t.Fatalf("CreateTask(excluded) error = %v", err)
	}

	opts := &GlobalOptions{DBPath: dbPath, Actor: "alex", JSON: true}
	cmd := newTaskListCommand(opts)
	cmd.SetArgs([]string{"--milestone", milestoneOne.Handle})
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("task list Execute() error = %v", err)
	}

	var payload struct {
		Data struct {
			Items []app.TaskRecord `json:"items"`
		} `json:"data"`
		Meta struct {
			Count   int `json:"count"`
			Filters struct {
				Milestone string `json:"milestone"`
			} `json:"filters"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal task list output: %v", err)
	}
	if got, want := payload.Meta.Count, 1; got != want {
		t.Fatalf("payload.Meta.Count = %d, want %d", got, want)
	}
	if got, want := payload.Meta.Filters.Milestone, milestoneOne.Handle; got != want {
		t.Fatalf("payload.Meta.Filters.Milestone = %q, want %q", got, want)
	}
	if got, want := payload.Data.Items[0].Title, "Included task"; got != want {
		t.Fatalf("payload.Data.Items[0].Title = %q, want %q", got, want)
	}
	if payload.Data.Items[0].MilestoneHandle == nil || *payload.Data.Items[0].MilestoneHandle != milestoneOne.Handle {
		t.Fatalf("payload.Data.Items[0].MilestoneHandle = %v, want %q", payload.Data.Items[0].MilestoneHandle, milestoneOne.Handle)
	}
}
