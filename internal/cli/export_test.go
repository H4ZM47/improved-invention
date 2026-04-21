package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/H4ZM47/grind/internal/app"
	taskconfig "github.com/H4ZM47/grind/internal/config"
	taskdb "github.com/H4ZM47/grind/internal/db"
)

func seedExportFixtures(t *testing.T) string {
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
	domain, err := (app.DomainManager{DB: db, HumanName: "alex"}).Create(context.Background(), app.CreateDomainRequest{Name: "Work"})
	if err != nil {
		t.Fatalf("CreateDomain() error = %v", err)
	}
	milestone, err := (app.MilestoneManager{DB: db, HumanName: "alex"}).Create(context.Background(), app.CreateMilestoneRequest{Name: "v1.0.2"})
	if err != nil {
		t.Fatalf("CreateMilestone() error = %v", err)
	}
	if _, err := (app.TaskManager{DB: db, HumanName: "alex"}).Create(context.Background(), app.CreateTaskRequest{
		Title:        "Write CLI contract",
		Tags:         []string{"cli"},
		DomainRef:    &domain.Handle,
		MilestoneRef: &milestone.Handle,
	}); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}

	return dbPath
}

func TestExportJSONToStdoutIncludesSeededTask(t *testing.T) {
	t.Parallel()

	dbPath := seedExportFixtures(t)

	opts := &GlobalOptions{DBPath: dbPath}
	cmd := newExportCommand(opts)
	cmd.SetArgs([]string{"json"})
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var doc struct {
		Version int `json:"version"`
		Tasks   []struct {
			Title           string `json:"title"`
			MilestoneHandle string `json:"milestone_handle"`
		} `json:"tasks"`
		Domains []struct {
			Name string `json:"name"`
		} `json:"domains"`
		Milestones []struct {
			Name string `json:"name"`
		} `json:"milestones"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &doc); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\noutput: %s", err, stdout.String())
	}
	if doc.Version != 1 {
		t.Fatalf("doc.Version = %d, want 1", doc.Version)
	}
	if len(doc.Tasks) != 1 || doc.Tasks[0].Title != "Write CLI contract" {
		t.Fatalf("doc.Tasks = %+v, want one task with title 'Write CLI contract'", doc.Tasks)
	}
	if got, want := doc.Tasks[0].MilestoneHandle, "MILE-1"; got != want {
		t.Fatalf("doc.Tasks[0].MilestoneHandle = %q, want %q", got, want)
	}
	if len(doc.Domains) != 1 || doc.Domains[0].Name != "Work" {
		t.Fatalf("doc.Domains = %+v, want one domain named 'Work'", doc.Domains)
	}
	if len(doc.Milestones) != 1 || doc.Milestones[0].Name != "v1.0.2" {
		t.Fatalf("doc.Milestones = %+v, want one milestone named 'v1.0.2'", doc.Milestones)
	}
}

func TestExportCSVWritesToOutputFile(t *testing.T) {
	t.Parallel()

	dbPath := seedExportFixtures(t)
	outPath := filepath.Join(t.TempDir(), "tasks.csv")

	opts := &GlobalOptions{DBPath: dbPath}
	cmd := newExportCommand(opts)
	cmd.SetArgs([]string{"csv", "--output", outPath})
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if stdout.Len() != 0 {
		t.Fatalf("stdout should be empty when --output is set, got %q", stdout.String())
	}

	contents, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v", err)
	}
	if !strings.Contains(string(contents), "Write CLI contract") {
		t.Fatalf("csv output missing seeded task title:\n%s", contents)
	}
}

func TestExportTXTWritesToStdout(t *testing.T) {
	t.Parallel()

	dbPath := seedExportFixtures(t)

	opts := &GlobalOptions{DBPath: dbPath}
	cmd := newExportCommand(opts)
	cmd.SetArgs([]string{"txt"})
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if !strings.Contains(stdout.String(), "Write CLI contract") {
		t.Fatalf("txt output missing seeded task title:\n%s", stdout.String())
	}
}

func TestExportMarkdownWritesToStdout(t *testing.T) {
	t.Parallel()

	dbPath := seedExportFixtures(t)

	opts := &GlobalOptions{DBPath: dbPath}
	cmd := newExportCommand(opts)
	cmd.SetArgs([]string{"markdown"})
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if !strings.Contains(stdout.String(), "Write CLI contract") {
		t.Fatalf("markdown output missing seeded task title:\n%s", stdout.String())
	}
}
