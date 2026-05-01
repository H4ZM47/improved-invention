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
	taskManager := app.TaskManager{DB: db, HumanName: "alex"}
	task, err := taskManager.Create(context.Background(), app.CreateTaskRequest{
		Title:        "Write CLI contract",
		Tags:         []string{"cli"},
		DomainRef:    &domain.Handle,
		MilestoneRef: &milestone.Handle,
	})
	if err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	otherTask, err := taskManager.Create(context.Background(), app.CreateTaskRequest{
		Title: "Review exported links",
	})
	if err != nil {
		t.Fatalf("CreateTask(other) error = %v", err)
	}
	if _, err := (app.LinkManager{DB: db, HumanName: "alex"}).Create(context.Background(), app.CreateLinkRequest{
		TaskRef: task.Handle,
		Type:    "url",
		Target:  "https://example.com/spec",
	}); err != nil {
		t.Fatalf("CreateLink() error = %v", err)
	}
	if _, err := (app.RelationshipManager{DB: db, HumanName: "alex"}).Create(context.Background(), app.CreateRelationshipRequest{
		SourceTaskRef: task.Handle,
		Type:          "blocks",
		TargetTaskRef: otherTask.Handle,
	}); err != nil {
		t.Fatalf("CreateRelationship() error = %v", err)
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
		Links []struct {
			Type   string `json:"type"`
			Target string `json:"target"`
		} `json:"links"`
		Relationships []struct {
			Type       string `json:"type"`
			SourceTask string `json:"source_task"`
			TargetTask string `json:"target_task"`
		} `json:"relationships"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &doc); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\noutput: %s", err, stdout.String())
	}
	if doc.Version != 1 {
		t.Fatalf("doc.Version = %d, want 1", doc.Version)
	}
	if len(doc.Tasks) != 2 || doc.Tasks[0].Title != "Review exported links" || doc.Tasks[1].Title != "Write CLI contract" {
		t.Fatalf("doc.Tasks = %+v, want exported tasks ordered by most recent update", doc.Tasks)
	}
	if got, want := doc.Tasks[1].MilestoneHandle, "MILE-1"; got != want {
		t.Fatalf("doc.Tasks[1].MilestoneHandle = %q, want %q", got, want)
	}
	if len(doc.Domains) != 1 || doc.Domains[0].Name != "Work" {
		t.Fatalf("doc.Domains = %+v, want one domain named 'Work'", doc.Domains)
	}
	if len(doc.Milestones) != 1 || doc.Milestones[0].Name != "v1.0.2" {
		t.Fatalf("doc.Milestones = %+v, want one milestone named 'v1.0.2'", doc.Milestones)
	}
	if len(doc.Links) != 1 || doc.Links[0].Type != "url" || doc.Links[0].Target != "https://example.com/spec" {
		t.Fatalf("doc.Links = %+v, want one exported url link", doc.Links)
	}
	if len(doc.Relationships) != 1 || doc.Relationships[0].Type != "blocks" {
		t.Fatalf("doc.Relationships = %+v, want one exported blocks relationship", doc.Relationships)
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
