package cli

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/H4ZM47/improved-invention/internal/app"
	taskconfig "github.com/H4ZM47/improved-invention/internal/config"
	taskdb "github.com/H4ZM47/improved-invention/internal/db"
)

func TestTaskUpdateNoInputRequiresExplicitAssigneeDecision(t *testing.T) {
	t.Parallel()

	dbPath, taskHandle, domainHandle := seedReclassificationScenario(t)

	opts := &GlobalOptions{
		DBPath:  dbPath,
		Actor:   "alex",
		NoInput: true,
	}
	cmd := newTaskUpdateCommand(opts)
	cmd.SetArgs([]string{taskHandle, "--domain", domainHandle})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want assignment decision failure")
	}
	if !strings.Contains(err.Error(), "explicit assignee decision") {
		t.Fatalf("Execute() error = %v, want assignment decision message", err)
	}
}

func TestTaskUpdateInteractivePromptCanKeepAssignee(t *testing.T) {
	t.Parallel()

	dbPath, taskHandle, domainHandle := seedReclassificationScenario(t)

	opts := &GlobalOptions{
		DBPath: dbPath,
		Actor:  "alex",
	}
	cmd := newTaskUpdateCommand(opts)
	cmd.SetArgs([]string{taskHandle, "--domain", domainHandle})
	cmd.SetIn(strings.NewReader("k\n"))
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
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

	task, err := taskdb.FindTask(context.Background(), db, taskHandle)
	if err != nil {
		t.Fatalf("FindTask() error = %v", err)
	}
	if task.AssigneeUUID == nil {
		t.Fatal("task.AssigneeUUID = nil, want kept assignee")
	}

	homeActor, err := taskdb.FindActor(context.Background(), db, "alex")
	if err != nil {
		t.Fatalf("FindActor(alex) error = %v", err)
	}
	if got, want := *task.AssigneeUUID, homeActor.UUID; got != want {
		t.Fatalf("task.AssigneeUUID = %q, want %q", got, want)
	}
	if !strings.Contains(stderr.String(), "requires an assignee decision") {
		t.Fatalf("stderr = %q, want prompt text", stderr.String())
	}
}

func seedReclassificationScenario(t *testing.T) (dbPath string, taskHandle string, targetDomainHandle string) {
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

	actorManager := app.ActorManager{
		DB:        db,
		HumanName: "alex",
	}
	domainManager := app.DomainManager{
		DB:        db,
		HumanName: "alex",
	}
	taskManager := app.TaskManager{
		DB:        db,
		HumanName: "alex",
	}

	human, err := actorManager.BootstrapConfiguredHumanActor(context.Background())
	if err != nil {
		t.Fatalf("BootstrapConfiguredHumanActor() error = %v", err)
	}
	agent, err := actorManager.GetOrCreateAgentActor(context.Background(), "codex:agent-17")
	if err != nil {
		t.Fatalf("GetOrCreateAgentActor() error = %v", err)
	}

	home, err := domainManager.Create(context.Background(), app.CreateDomainRequest{
		Name:               "Home",
		DefaultAssigneeRef: &human.Handle,
	})
	if err != nil {
		t.Fatalf("CreateDomain(home) error = %v", err)
	}
	work, err := domainManager.Create(context.Background(), app.CreateDomainRequest{
		Name:               "Work",
		DefaultAssigneeRef: &agent.Handle,
	})
	if err != nil {
		t.Fatalf("CreateDomain(work) error = %v", err)
	}

	task, err := taskManager.Create(context.Background(), app.CreateTaskRequest{
		Title:     "Interactive reclassification",
		DomainRef: &home.Handle,
	})
	if err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	if _, err := taskManager.Claim(context.Background(), app.ClaimTaskRequest{
		Reference: task.Handle,
		Lease:     time.Hour,
	}); err != nil {
		t.Fatalf("Claim() error = %v", err)
	}

	return dbPath, task.Handle, work.Handle
}
