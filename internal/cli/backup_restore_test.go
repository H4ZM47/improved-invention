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
)

func TestBackupCreateWritesArtifactToOutputPath(t *testing.T) {
	t.Parallel()

	dbPath := seedBackupRestoreFixtures(t)
	outputPath := filepath.Join(t.TempDir(), "task-backup.sqlite")

	root := NewRootCommand(BuildInfo{})
	root.SetArgs([]string{"--db", dbPath, "--json", "backup", "create", "--output", outputPath})
	var stdout bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stdout)

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v\noutput: %s", err, stdout.String())
	}

	var payload struct {
		OK      bool   `json:"ok"`
		Command string `json:"command"`
		Data    struct {
			OutputPath string `json:"output_path"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\noutput: %s", err, stdout.String())
	}
	if !payload.OK {
		t.Fatal("payload.OK = false, want true")
	}
	if got, want := payload.Command, "grind backup create"; got != want {
		t.Fatalf("payload.Command = %q, want %q", got, want)
	}
	if got, want := payload.Data.OutputPath, outputPath; got != want {
		t.Fatalf("payload.Data.OutputPath = %q, want %q", got, want)
	}
}

func TestRestoreApplyRestoresArtifactIntoTargetDatabase(t *testing.T) {
	t.Parallel()

	sourceDBPath := seedBackupRestoreFixtures(t)
	backupPath := filepath.Join(t.TempDir(), "task-backup.sqlite")
	targetPath := filepath.Join(t.TempDir(), "restored-task.db")

	backupRoot := NewRootCommand(BuildInfo{})
	backupRoot.SetArgs([]string{"--db", sourceDBPath, "backup", "create", "--output", backupPath})
	var backupOutput bytes.Buffer
	backupRoot.SetOut(&backupOutput)
	backupRoot.SetErr(&backupOutput)
	if err := backupRoot.Execute(); err != nil {
		t.Fatalf("backup Execute() error = %v\noutput: %s", err, backupOutput.String())
	}

	restoreRoot := NewRootCommand(BuildInfo{})
	restoreRoot.SetArgs([]string{"--db", targetPath, "--json", "restore", "apply", "--input", backupPath})
	var restoreOutput bytes.Buffer
	restoreRoot.SetOut(&restoreOutput)
	restoreRoot.SetErr(&restoreOutput)
	if err := restoreRoot.Execute(); err != nil {
		t.Fatalf("restore Execute() error = %v\noutput: %s", err, restoreOutput.String())
	}

	var payload struct {
		OK      bool   `json:"ok"`
		Command string `json:"command"`
		Data    struct {
			DBPath string `json:"db_path"`
		} `json:"data"`
	}
	if err := json.Unmarshal(restoreOutput.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\noutput: %s", err, restoreOutput.String())
	}
	if !payload.OK {
		t.Fatal("payload.OK = false, want true")
	}
	if got, want := payload.Command, "grind restore apply"; got != want {
		t.Fatalf("payload.Command = %q, want %q", got, want)
	}
	if got, want := payload.Data.DBPath, targetPath; got != want {
		t.Fatalf("payload.Data.DBPath = %q, want %q", got, want)
	}

	cfg := taskconfig.Resolved{
		DBPath:      targetPath,
		BusyTimeout: 5 * time.Second,
		HumanName:   "alex",
	}
	db, err := taskdb.Open(context.Background(), cfg)
	if err != nil {
		t.Fatalf("taskdb.Open(restored) error = %v", err)
	}
	defer db.Close()

	task, err := taskdb.FindTask(context.Background(), db, "TASK-1")
	if err != nil {
		t.Fatalf("FindTask(TASK-1) error = %v", err)
	}
	if got, want := task.Title, "Back up the CLI"; got != want {
		t.Fatalf("task.Title = %q, want %q", got, want)
	}
}

func TestRestoreApplyRequiresForceToOverwriteExistingDatabase(t *testing.T) {
	t.Parallel()

	sourceDBPath := seedBackupRestoreFixtures(t)
	backupPath := filepath.Join(t.TempDir(), "task-backup.sqlite")
	targetPath := filepath.Join(t.TempDir(), "restored-task.db")

	backupRoot := NewRootCommand(BuildInfo{})
	backupRoot.SetArgs([]string{"--db", sourceDBPath, "backup", "create", "--output", backupPath})
	var backupOutput bytes.Buffer
	backupRoot.SetOut(&backupOutput)
	backupRoot.SetErr(&backupOutput)
	if err := backupRoot.Execute(); err != nil {
		t.Fatalf("backup Execute() error = %v\noutput: %s", err, backupOutput.String())
	}

	targetSeed := seedBackupRestoreFixturesAtPath(t, targetPath)
	_ = targetSeed

	restoreRoot := NewRootCommand(BuildInfo{})
	restoreRoot.SetArgs([]string{"--db", targetPath, "restore", "apply", "--input", backupPath})
	var restoreOutput bytes.Buffer
	restoreRoot.SetOut(&restoreOutput)
	restoreRoot.SetErr(&restoreOutput)
	err := restoreRoot.Execute()
	if err == nil {
		t.Fatal("restore Execute() error = nil, want overwrite guard failure")
	}
	if !strings.Contains(err.Error(), "--force") {
		t.Fatalf("restore error = %v, want overwrite guidance", err)
	}
}

func seedBackupRestoreFixtures(t *testing.T) string {
	t.Helper()
	return seedBackupRestoreFixturesAtPath(t, filepath.Join(t.TempDir(), "task.db"))
}

func seedBackupRestoreFixturesAtPath(t *testing.T, dbPath string) string {
	t.Helper()

	cfg := taskconfig.Resolved{
		DBPath:      dbPath,
		BusyTimeout: 5 * time.Second,
		HumanName:   "alex",
	}
	db, err := taskdb.Open(context.Background(), cfg)
	if err != nil {
		t.Fatalf("taskdb.Open() error = %v", err)
	}
	defer db.Close()

	actorManager := app.ActorManager{DB: db, HumanName: "alex"}
	human, err := actorManager.BootstrapConfiguredHumanActor(context.Background())
	if err != nil {
		t.Fatalf("BootstrapConfiguredHumanActor() error = %v", err)
	}

	domain, err := (app.DomainManager{DB: db, HumanName: "alex"}).Create(context.Background(), app.CreateDomainRequest{Name: "Work"})
	if err != nil {
		t.Fatalf("CreateDomain() error = %v", err)
	}
	project, err := (app.ProjectManager{DB: db, HumanName: "alex"}).Create(context.Background(), app.CreateProjectRequest{
		Name:      "Grind",
		DomainRef: domain.Handle,
	})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	if _, err := (app.TaskManager{DB: db, HumanName: "alex"}).Create(context.Background(), app.CreateTaskRequest{
		Title:       "Back up the CLI",
		Description: "Preserve everything",
		DomainRef:   &domain.Handle,
		ProjectRef:  &project.Handle,
		AssigneeRef: &human.Handle,
		Tags:        []string{"backup"},
	}); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}

	return dbPath
}
