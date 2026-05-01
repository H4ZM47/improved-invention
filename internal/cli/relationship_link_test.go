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

func TestLinkAddTaskRelationshipUsesTaskTargetKind(t *testing.T) {
	t.Parallel()

	dbPath, parentTask, childTask := seedTwoClaimedTasks(t)
	opts := &GlobalOptions{
		DBPath: dbPath,
		Actor:  "alex",
		JSON:   true,
	}

	add := newLinkAddCommand(opts)
	add.SetArgs([]string{parentTask, "child", childTask})
	var stdout bytes.Buffer
	add.SetOut(&stdout)
	add.SetErr(&stdout)
	if err := add.Execute(); err != nil {
		t.Fatalf("link add child Execute() error = %v", err)
	}

	var payload struct {
		Data struct {
			Link struct {
				Type       string `json:"type"`
				SourceTask string `json:"source_task"`
				TargetKind string `json:"target_kind"`
				Target     string `json:"target"`
			} `json:"link"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v; stdout=%q", err, stdout.String())
	}
	if got, want := payload.Data.Link.Type, "parent_of"; got != want {
		t.Fatalf("link.Type = %q, want %q", got, want)
	}
	if got, want := payload.Data.Link.SourceTask, parentTask; got != want {
		t.Fatalf("link.SourceTask = %q, want %q", got, want)
	}
	if got, want := payload.Data.Link.TargetKind, "task"; got != want {
		t.Fatalf("link.TargetKind = %q, want %q", got, want)
	}
	if got, want := payload.Data.Link.Target, childTask; got != want {
		t.Fatalf("link.Target = %q, want %q", got, want)
	}
}

func TestLinkCommandsListAndRemoveTaskAndExternalLinks(t *testing.T) {
	t.Parallel()

	dbPath, taskHandle, otherTask := seedTwoClaimedTasks(t)
	opts := &GlobalOptions{
		DBPath: dbPath,
		Actor:  "alex",
	}

	add := newLinkAddCommand(opts)
	add.SetArgs([]string{taskHandle, "url", "https://example.com/spec"})
	if err := add.Execute(); err != nil {
		t.Fatalf("link add Execute() error = %v", err)
	}
	add.SetArgs([]string{taskHandle, "blocks", otherTask})
	if err := add.Execute(); err != nil {
		t.Fatalf("link add relationship Execute() error = %v", err)
	}

	list := newLinkListCommand(opts)
	list.SetArgs([]string{taskHandle})
	var stdout bytes.Buffer
	list.SetOut(&stdout)
	list.SetErr(&stdout)
	if err := list.Execute(); err != nil {
		t.Fatalf("link list Execute() error = %v", err)
	}
	if !strings.Contains(stdout.String(), "https://example.com/spec") || !strings.Contains(stdout.String(), "\texternal\t") {
		t.Fatalf("link list output = %q, want created external link with target kind", stdout.String())
	}
	if !strings.Contains(stdout.String(), otherTask) || !strings.Contains(stdout.String(), "\ttask\t") {
		t.Fatalf("link list output = %q, want created task link with target kind", stdout.String())
	}

	cfg := taskconfig.Resolved{
		DBPath:      dbPath,
		BusyTimeout: 5 * time.Second,
	}
	db, err := taskdb.Open(context.Background(), cfg)
	if err != nil {
		t.Fatalf("taskdb.Open() error = %v", err)
	}
	defer func() {
		_ = db.Close()
	}()

	links, err := taskdb.ListExternalLinksForTask(context.Background(), db, taskHandle)
	if err != nil {
		t.Fatalf("ListExternalLinksForTask() error = %v", err)
	}
	if len(links) != 1 {
		t.Fatalf("len(links) = %d, want 1 external link", len(links))
	}

	remove := newLinkRemoveCommand(opts)
	remove.SetArgs([]string{taskHandle, "url", "https://example.com/spec"})
	if err := remove.Execute(); err != nil {
		t.Fatalf("link remove Execute() error = %v", err)
	}
	remove.SetArgs([]string{taskHandle, "blocks", otherTask})
	if err := remove.Execute(); err != nil {
		t.Fatalf("link remove relationship Execute() error = %v", err)
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
		newTimeStartCommand,
		newTimePauseCommand,
		newTimeResumeCommand,
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
	defer func() {
		_ = db.Close()
	}()

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
