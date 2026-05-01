package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/H4ZM47/grind/internal/app"
	taskconfig "github.com/H4ZM47/grind/internal/config"
	taskdb "github.com/H4ZM47/grind/internal/db"
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
	defer func() {
		_ = db.Close()
	}()

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

func TestTaskListJSONIncludesFiltersAndSearch(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "task.db")
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

	actorManager := app.ActorManager{
		DB:        db,
		HumanName: "alex",
	}
	domainManager := app.DomainManager{
		DB:        db,
		HumanName: "alex",
	}
	projectManager := app.ProjectManager{
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
	domain, err := domainManager.Create(context.Background(), app.CreateDomainRequest{Name: "Work"})
	if err != nil {
		t.Fatalf("CreateDomain() error = %v", err)
	}
	project, err := projectManager.Create(context.Background(), app.CreateProjectRequest{
		Name:      "Grind",
		DomainRef: domain.Handle,
	})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}

	dueAt := "2026-04-21T12:00:00Z"
	task, err := taskManager.Create(context.Background(), app.CreateTaskRequest{
		Title:       "Write CLI contract",
		Description: "Document list filters",
		Tags:        []string{"cli", "contract"},
		DomainRef:   &domain.Handle,
		ProjectRef:  &project.Handle,
		AssigneeRef: &human.Handle,
		DueAt:       &dueAt,
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
	active := "active"
	if _, err := taskManager.Update(context.Background(), app.UpdateTaskRequest{
		Reference: task.Handle,
		Status:    &active,
	}); err != nil {
		t.Fatalf("Update(active) error = %v", err)
	}

	if _, err := taskManager.Create(context.Background(), app.CreateTaskRequest{
		Title:       "Write docs",
		Description: "General docs work",
		Tags:        []string{"docs"},
	}); err != nil {
		t.Fatalf("CreateTask(other) error = %v", err)
	}

	opts := &GlobalOptions{
		DBPath: dbPath,
		Actor:  "alex",
		JSON:   true,
	}
	cmd := newTaskListCommand(opts)
	cmd.SetArgs([]string{
		"--status", "active",
		"--domain", domain.Handle,
		"--project", project.Handle,
		"--assignee", human.Handle,
		"--due-before", "2026-04-21T23:59:59Z",
		"--due-after", "2026-04-21T00:00:00Z",
		"--tag", "cli",
		"--search", "contract",
	})
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var payload struct {
		Command string `json:"command"`
		Data    struct {
			Items []struct {
				Handle string `json:"handle"`
			} `json:"items"`
		} `json:"data"`
		Meta struct {
			Count   int `json:"count"`
			Filters struct {
				Status   []string `json:"status"`
				Tags     []string `json:"tags"`
				Domain   string   `json:"domain"`
				Project  string   `json:"project"`
				Assignee string   `json:"assignee"`
				Search   string   `json:"search"`
			} `json:"filters"`
		} `json:"meta"`
	}

	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v; stdout=%q", err, stdout.String())
	}
	if got, want := payload.Command, "grind list"; got != want {
		t.Fatalf("payload.Command = %q, want %q", got, want)
	}
	if got, want := payload.Meta.Count, 1; got != want {
		t.Fatalf("payload.Meta.Count = %d, want %d", got, want)
	}
	if got, want := len(payload.Data.Items), 1; got != want {
		t.Fatalf("len(payload.Data.Items) = %d, want %d", got, want)
	}
	if got, want := payload.Data.Items[0].Handle, task.Handle; got != want {
		t.Fatalf("payload.Data.Items[0].Handle = %q, want %q", got, want)
	}
	if got, want := payload.Meta.Filters.Status[0], "active"; got != want {
		t.Fatalf("payload.Meta.Filters.Status[0] = %q, want %q", got, want)
	}
	if got, want := payload.Meta.Filters.Tags[0], "cli"; got != want {
		t.Fatalf("payload.Meta.Filters.Tags[0] = %q, want %q", got, want)
	}
	if got, want := payload.Meta.Filters.Domain, domain.Handle; got != want {
		t.Fatalf("payload.Meta.Filters.Domain = %q, want %q", got, want)
	}
	if got, want := payload.Meta.Filters.Project, project.Handle; got != want {
		t.Fatalf("payload.Meta.Filters.Project = %q, want %q", got, want)
	}
	if got, want := payload.Meta.Filters.Assignee, human.Handle; got != want {
		t.Fatalf("payload.Meta.Filters.Assignee = %q, want %q", got, want)
	}
	if got, want := payload.Meta.Filters.Search, "contract"; got != want {
		t.Fatalf("payload.Meta.Filters.Search = %q, want %q", got, want)
	}
}

func TestTaskCreateDescriptionFileUsesContents(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "task.db")
	descriptionPath := filepath.Join(t.TempDir(), "description.md")
	wantDescription := "line one\nline two"
	if err := os.WriteFile(descriptionPath, []byte(wantDescription), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	opts := &GlobalOptions{
		DBPath: dbPath,
		Actor:  "alex",
		JSON:   true,
	}
	cmd := newTaskCreateCommand(opts)
	cmd.SetArgs([]string{"Description file task", "--description-file", descriptionPath})
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var payload struct {
		Data struct {
			Task struct {
				Description string `json:"description"`
			} `json:"task"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v; stdout=%q", err, stdout.String())
	}
	if got, want := payload.Data.Task.Description, wantDescription; got != want {
		t.Fatalf("payload.Data.Task.Description = %q, want %q", got, want)
	}
}

func TestTaskUpdateDescriptionFileUsesContents(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "task.db")
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
	task, err := manager.Create(context.Background(), app.CreateTaskRequest{
		Title: "Update description file task",
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

	descriptionPath := filepath.Join(t.TempDir(), "updated-description.md")
	wantDescription := "updated\nfrom file"
	if err := os.WriteFile(descriptionPath, []byte(wantDescription), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	opts := &GlobalOptions{
		DBPath: dbPath,
		Actor:  "alex",
		JSON:   true,
	}
	cmd := newTaskUpdateCommand(opts)
	cmd.SetArgs([]string{task.Handle, "--description-file", descriptionPath})
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var payload struct {
		Data struct {
			Task struct {
				Description string `json:"description"`
			} `json:"task"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v; stdout=%q", err, stdout.String())
	}
	if got, want := payload.Data.Task.Description, wantDescription; got != want {
		t.Fatalf("payload.Data.Task.Description = %q, want %q", got, want)
	}
}

func TestPromptTaskDescriptionSupportsSkipAndMultiline(t *testing.T) {
	t.Parallel()

	got, err := promptTaskDescription(io.Discard, strings.NewReader("\n"), "Prompt task")
	if err != nil {
		t.Fatalf("promptTaskDescription(skip) error = %v", err)
	}
	if got != "" {
		t.Fatalf("promptTaskDescription(skip) = %q, want empty", got)
	}

	got, err = promptTaskDescription(io.Discard, strings.NewReader("line one\nline two\n\n"), "Prompt task")
	if err != nil {
		t.Fatalf("promptTaskDescription(multiline) error = %v", err)
	}
	if want := "line one\nline two"; got != want {
		t.Fatalf("promptTaskDescription(multiline) = %q, want %q", got, want)
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
	defer func() {
		_ = db.Close()
	}()

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
