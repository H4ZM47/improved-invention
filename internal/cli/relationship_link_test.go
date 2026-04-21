package cli

import (
	"bytes"
	"context"
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

func TestRelationshipCommandsEndToEnd(t *testing.T) {
	t.Parallel()

	dbPath, leftTask, rightTask := seedTwoClaimedTasks(t)
	opts := &GlobalOptions{
		DBPath: dbPath,
		Actor:  "alex",
	}

	add := newRelationshipAddCommand(opts)
	add.SetArgs([]string{"blocks", leftTask, rightTask})
	if err := add.Execute(); err != nil {
		t.Fatalf("relationship add Execute() error = %v", err)
	}

	list := newRelationshipListCommand(opts)
	list.SetArgs([]string{leftTask})
	var stdout bytes.Buffer
	list.SetOut(&stdout)
	list.SetErr(&stdout)
	if err := list.Execute(); err != nil {
		t.Fatalf("relationship list Execute() error = %v", err)
	}
	if !strings.Contains(stdout.String(), "blocks") {
		t.Fatalf("relationship list output = %q, want blocks row", stdout.String())
	}

	remove := newRelationshipRemoveCommand(opts)
	remove.SetArgs([]string{"blocks", leftTask, rightTask})
	if err := remove.Execute(); err != nil {
		t.Fatalf("relationship remove Execute() error = %v", err)
	}
}

func TestRelationshipChildAliasUsesSourceAsParent(t *testing.T) {
	t.Parallel()

	dbPath, parentTask, childTask := seedTwoClaimedTasks(t)
	opts := &GlobalOptions{
		DBPath: dbPath,
		Actor:  "alex",
		JSON:   true,
	}

	add := newRelationshipAddCommand(opts)
	add.SetArgs([]string{"child", parentTask, childTask})
	var stdout bytes.Buffer
	add.SetOut(&stdout)
	add.SetErr(&stdout)
	if err := add.Execute(); err != nil {
		t.Fatalf("relationship add child Execute() error = %v", err)
	}

	var payload struct {
		Data struct {
			Relationship struct {
				Type       string `json:"type"`
				SourceTask string `json:"source_task"`
				TargetTask string `json:"target_task"`
			} `json:"relationship"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v; stdout=%q", err, stdout.String())
	}
	if got, want := payload.Data.Relationship.Type, "parent_of"; got != want {
		t.Fatalf("relationship.Type = %q, want %q", got, want)
	}
	if got, want := payload.Data.Relationship.SourceTask, parentTask; got != want {
		t.Fatalf("relationship.SourceTask = %q, want %q", got, want)
	}
	if got, want := payload.Data.Relationship.TargetTask, childTask; got != want {
		t.Fatalf("relationship.TargetTask = %q, want %q", got, want)
	}
}

func TestLinkCommandsEndToEnd(t *testing.T) {
	t.Parallel()

	dbPath, taskHandle, _ := seedTwoClaimedTasks(t)
	opts := &GlobalOptions{
		DBPath: dbPath,
		Actor:  "alex",
	}

	add := newLinkAddCommand(opts)
	add.SetArgs([]string{taskHandle, "url", "https://example.com/spec"})
	if err := add.Execute(); err != nil {
		t.Fatalf("link add Execute() error = %v", err)
	}

	list := newLinkListCommand(opts)
	list.SetArgs([]string{taskHandle})
	var stdout bytes.Buffer
	list.SetOut(&stdout)
	list.SetErr(&stdout)
	if err := list.Execute(); err != nil {
		t.Fatalf("link list Execute() error = %v", err)
	}
	if !strings.Contains(stdout.String(), "https://example.com/spec") {
		t.Fatalf("link list output = %q, want created url", stdout.String())
	}

	cfg := taskconfig.Resolved{
		DBPath:      dbPath,
		BusyTimeout: 5 * time.Second,
	}
	db, err := taskdb.Open(context.Background(), cfg)
	if err != nil {
		t.Fatalf("taskdb.Open() error = %v", err)
	}
	defer db.Close()

	links, err := taskdb.ListExternalLinksForTask(context.Background(), db, taskHandle)
	if err != nil {
		t.Fatalf("ListExternalLinksForTask() error = %v", err)
	}
	if len(links) != 1 {
		t.Fatalf("len(links) = %d, want 1", len(links))
	}

	remove := newLinkRemoveCommand(opts)
	remove.SetArgs([]string{taskHandle, links[0].UUID})
	if err := remove.Execute(); err != nil {
		t.Fatalf("link remove Execute() error = %v", err)
	}
}

func TestTaskSessionCommandsEndToEnd(t *testing.T) {
	t.Parallel()

	dbPath, taskHandle, _ := seedTwoClaimedTasks(t)
	opts := &GlobalOptions{
		DBPath: dbPath,
		Actor:  "alex",
	}

	for index, cmdFactory := range []func(*GlobalOptions) *cobra.Command{
		newTaskStartCommand,
		newTaskPauseCommand,
		newTaskResumeCommand,
	} {
		cmd := cmdFactory(opts)
		cmd.SetArgs([]string{taskHandle})
		var stdout bytes.Buffer
		cmd.SetOut(&stdout)
		cmd.SetErr(&stdout)
		if err := cmd.Execute(); err != nil {
			t.Fatalf("session command Execute() error = %v", err)
		}
		if got := stdout.String(); !strings.Contains(got, "elapsed_seconds=") || !strings.Contains(got, "task="+taskHandle) {
			t.Fatalf("session command %d output = %q, want labeled fields", index, got)
		}
	}
}

func seedTwoClaimedTasks(t *testing.T) (dbPath string, firstTask string, secondTask string) {
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
	first, err := manager.Create(context.Background(), app.CreateTaskRequest{
		Title: "Left task",
	})
	if err != nil {
		t.Fatalf("Create(left) error = %v", err)
	}
	second, err := manager.Create(context.Background(), app.CreateTaskRequest{
		Title: "Right task",
	})
	if err != nil {
		t.Fatalf("Create(right) error = %v", err)
	}

	for _, handle := range []string{first.Handle, second.Handle} {
		if _, err := manager.Claim(context.Background(), app.ClaimTaskRequest{
			Reference: handle,
			Lease:     time.Hour,
		}); err != nil {
			t.Fatalf("Claim(%s) error = %v", handle, err)
		}
	}

	return dbPath, first.Handle, second.Handle
}
