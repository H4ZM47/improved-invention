package cli

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/H4ZM47/task-cli/internal/app"
	taskconfig "github.com/H4ZM47/task-cli/internal/config"
	taskdb "github.com/H4ZM47/task-cli/internal/db"
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

	for _, cmdFactory := range []func(*GlobalOptions) *cobra.Command{
		newTaskStartCommand,
		newTaskPauseCommand,
		newTaskResumeCommand,
	} {
		cmd := cmdFactory(opts)
		cmd.SetArgs([]string{taskHandle})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("session command Execute() error = %v", err)
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
