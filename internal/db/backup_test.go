package db_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/H4ZM47/grind/internal/app"
	taskconfig "github.com/H4ZM47/grind/internal/config"
	taskdb "github.com/H4ZM47/grind/internal/db"
)

func TestBackupDatabaseCreatesPortableArtifact(t *testing.T) {
	t.Parallel()

	sourceDBPath, expected := seedBackupFixtureDB(t)
	backupPath := filepath.Join(t.TempDir(), "task-backup.sqlite")

	sourceCfg := taskconfig.Resolved{DBPath: sourceDBPath, BusyTimeout: 5 * time.Second}
	sourceDB, err := taskdb.Open(context.Background(), sourceCfg)
	if err != nil {
		t.Fatalf("Open(source) error = %v", err)
	}
	defer sourceDB.Close()

	if err := taskdb.BackupDatabase(context.Background(), sourceDB, backupPath); err != nil {
		t.Fatalf("BackupDatabase() error = %v", err)
	}

	backupCfg := taskconfig.Resolved{DBPath: backupPath, BusyTimeout: 5 * time.Second}
	backupDB, err := taskdb.Open(context.Background(), backupCfg)
	if err != nil {
		t.Fatalf("Open(backup) error = %v", err)
	}
	defer backupDB.Close()

	assertBackupFixturePreserved(t, backupDB, expected)
}

func TestRestoreDatabasePreservesIdentityAndHandleSequences(t *testing.T) {
	t.Parallel()

	sourceDBPath, expected := seedBackupFixtureDB(t)
	backupPath := filepath.Join(t.TempDir(), "task-backup.sqlite")
	targetPath := filepath.Join(t.TempDir(), "restored-task.db")

	sourceCfg := taskconfig.Resolved{DBPath: sourceDBPath, BusyTimeout: 5 * time.Second}
	sourceDB, err := taskdb.Open(context.Background(), sourceCfg)
	if err != nil {
		t.Fatalf("Open(source) error = %v", err)
	}
	if err := taskdb.BackupDatabase(context.Background(), sourceDB, backupPath); err != nil {
		t.Fatalf("BackupDatabase() error = %v", err)
	}
	if err := sourceDB.Close(); err != nil {
		t.Fatalf("sourceDB.Close() error = %v", err)
	}

	targetCfg := taskconfig.Resolved{
		DBPath:      targetPath,
		BusyTimeout: 5 * time.Second,
		HumanName:   "alex",
	}
	if err := taskdb.RestoreDatabase(context.Background(), backupPath, targetCfg, false); err != nil {
		t.Fatalf("RestoreDatabase() error = %v", err)
	}

	targetDB, err := taskdb.Open(context.Background(), targetCfg)
	if err != nil {
		t.Fatalf("Open(restored) error = %v", err)
	}
	defer targetDB.Close()

	assertBackupFixturePreserved(t, targetDB, expected)

	nextTask, err := (app.TaskManager{DB: targetDB, HumanName: "alex"}).Create(context.Background(), app.CreateTaskRequest{
		Title: "Post-restore task",
	})
	if err != nil {
		t.Fatalf("Create(post-restore) error = %v", err)
	}
	if got, want := nextTask.Handle, "TASK-3"; got != want {
		t.Fatalf("post-restore handle = %q, want %q", got, want)
	}
}

func TestOpenExpiresStaleClaims(t *testing.T) {
	t.Parallel()

	cfg := taskconfig.Resolved{
		DBPath:      filepath.Join(t.TempDir(), "task.db"),
		BusyTimeout: 5 * time.Second,
	}

	db, err := taskdb.Open(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	actorID := insertActor(t, db, "actor-1", "ACT-1", "human", "", "alex")
	taskID := insertTask(t, db, "task-1", "TASK-1", "Write backup tests", nil, nil)
	if _, err := db.Exec(`
		INSERT INTO claims(uuid, task_id, actor_id, expires_at)
		VALUES ('claim-1', ?, ?, '2000-01-01T00:00:00Z')
	`, taskID, actorID); err != nil {
		t.Fatalf("insert stale claim failed: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("db.Close() error = %v", err)
	}

	reopened, err := taskdb.Open(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Open(reopened) error = %v", err)
	}
	defer reopened.Close()

	var releasedAt string
	var reason string
	if err := reopened.QueryRow(`
		SELECT released_at, release_reason
		FROM claims
		WHERE uuid = 'claim-1'
	`).Scan(&releasedAt, &reason); err != nil {
		t.Fatalf("QueryRow(stale claim) error = %v", err)
	}
	if releasedAt == "" {
		t.Fatal("released_at = empty, want stale claim to be expired on open")
	}
	if got, want := reason, "expired"; got != want {
		t.Fatalf("release_reason = %q, want %q", got, want)
	}
}

type backupFixtureExpectation struct {
	DomainHandle  string
	MilestoneHandle string
	ProjectHandle string
	TaskOneHandle string
	TaskOneUUID   string
	TaskTwoHandle string
	TaskTwoUUID   string
	ActorHandle   string
	SavedViewName string
	EventCount    int
}

func seedBackupFixtureDB(t *testing.T) (string, backupFixtureExpectation) {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "source-task.db")
	cfg := taskconfig.Resolved{
		DBPath:      dbPath,
		BusyTimeout: 5 * time.Second,
		HumanName:   "alex",
	}
	db, err := taskdb.Open(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer db.Close()

	actorManager := app.ActorManager{DB: db, HumanName: "alex"}
	human, err := actorManager.BootstrapConfiguredHumanActor(context.Background())
	if err != nil {
		t.Fatalf("BootstrapConfiguredHumanActor() error = %v", err)
	}

	domain, err := (app.DomainManager{DB: db, HumanName: "alex"}).Create(context.Background(), app.CreateDomainRequest{
		Name: "Work",
	})
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

	taskManager := app.TaskManager{DB: db, HumanName: "alex"}
	taskOne, err := taskManager.Create(context.Background(), app.CreateTaskRequest{
		Title:       "Back up the CLI",
		Description: "Preserve everything",
		DomainRef:   &domain.Handle,
		ProjectRef:  &project.Handle,
		AssigneeRef: &human.Handle,
		Tags:        []string{"backup", "portable"},
	})
	if err != nil {
		t.Fatalf("Create(taskOne) error = %v", err)
	}
	taskTwo, err := taskManager.Create(context.Background(), app.CreateTaskRequest{
		Title: "Restore the CLI",
		Tags:  []string{"restore"},
	})
	if err != nil {
		t.Fatalf("Create(taskTwo) error = %v", err)
	}

	if _, err := taskManager.Claim(context.Background(), app.ClaimTaskRequest{
		Reference: taskOne.Handle,
		Lease:     24 * time.Hour,
	}); err != nil {
		t.Fatalf("Claim() error = %v", err)
	}
	active := "active"
	if _, err := taskManager.Update(context.Background(), app.UpdateTaskRequest{
		Reference: taskOne.Handle,
		Status:    &active,
	}); err != nil {
		t.Fatalf("Update(active) error = %v", err)
	}

	if _, err := (app.LinkManager{DB: db, HumanName: "alex"}).Create(context.Background(), app.CreateLinkRequest{
		TaskRef: taskOne.Handle,
		Type:    "url",
		Target:  "https://example.com/backup-spec",
		Label:   "Spec",
	}); err != nil {
		t.Fatalf("CreateLink() error = %v", err)
	}
	if _, err := (app.RelationshipManager{DB: db, HumanName: "alex"}).Create(context.Background(), app.CreateRelationshipRequest{
		Type:          "blocks",
		SourceTaskRef: taskOne.Handle,
		TargetTaskRef: taskTwo.Handle,
	}); err != nil {
		t.Fatalf("CreateRelationship() error = %v", err)
	}
	if _, err := (app.ViewManager{DB: db}).Create(context.Background(), app.CreateViewRequest{
		Name: "Open Backup Work",
		Filters: app.SavedViewFilters{
			Statuses: []string{"active"},
			Tags:     []string{"backup"},
		},
	}); err != nil {
		t.Fatalf("CreateView() error = %v", err)
	}
	milestone, err := taskdb.CreateMilestone(context.Background(), db, taskdb.MilestoneCreateInput{
		Name:        "v1.0.1",
		Description: "Portable backup milestone",
	})
	if err != nil {
		t.Fatalf("CreateMilestone() error = %v", err)
	}
	if _, err := db.Exec(`UPDATE tasks SET milestone_id = ? WHERE uuid = ?`, milestone.ID, taskOne.UUID); err != nil {
		t.Fatalf("attach milestone to taskOne failed: %v", err)
	}

	var eventCount int
	if err := db.QueryRow(`SELECT COUNT(1) FROM events`).Scan(&eventCount); err != nil {
		t.Fatalf("QueryRow(events count) error = %v", err)
	}

	return dbPath, backupFixtureExpectation{
		DomainHandle:    domain.Handle,
		MilestoneHandle: milestone.Handle,
		ProjectHandle:   project.Handle,
		TaskOneHandle:   taskOne.Handle,
		TaskOneUUID:     taskOne.UUID,
		TaskTwoHandle:   taskTwo.Handle,
		TaskTwoUUID:     taskTwo.UUID,
		ActorHandle:     human.Handle,
		SavedViewName:   "Open Backup Work",
		EventCount:      eventCount,
	}
}

func assertBackupFixturePreserved(t *testing.T, db *sql.DB, expected backupFixtureExpectation) {
	t.Helper()

	taskOne, err := taskdb.FindTask(context.Background(), db, expected.TaskOneHandle)
	if err != nil {
		t.Fatalf("FindTask(taskOne) error = %v", err)
	}
	if got, want := taskOne.UUID, expected.TaskOneUUID; got != want {
		t.Fatalf("taskOne.UUID = %q, want %q", got, want)
	}
	if taskOne.MilestoneHandle == nil {
		t.Fatal("taskOne.MilestoneHandle = nil, want preserved milestone handle")
	}
	if got, want := *taskOne.MilestoneHandle, expected.MilestoneHandle; got != want {
		t.Fatalf("taskOne.MilestoneHandle = %q, want %q", got, want)
	}

	taskTwo, err := taskdb.FindTask(context.Background(), db, expected.TaskTwoHandle)
	if err != nil {
		t.Fatalf("FindTask(taskTwo) error = %v", err)
	}
	if got, want := taskTwo.UUID, expected.TaskTwoUUID; got != want {
		t.Fatalf("taskTwo.UUID = %q, want %q", got, want)
	}

	if _, err := taskdb.FindDomain(context.Background(), db, expected.DomainHandle); err != nil {
		t.Fatalf("FindDomain() error = %v", err)
	}
	if _, err := taskdb.FindMilestone(context.Background(), db, expected.MilestoneHandle); err != nil {
		t.Fatalf("FindMilestone() error = %v", err)
	}
	if _, err := taskdb.FindProject(context.Background(), db, expected.ProjectHandle); err != nil {
		t.Fatalf("FindProject() error = %v", err)
	}
	if _, err := taskdb.FindActor(context.Background(), db, expected.ActorHandle); err != nil {
		t.Fatalf("FindActor() error = %v", err)
	}
	if _, err := taskdb.FindSavedView(context.Background(), db, expected.SavedViewName); err != nil {
		t.Fatalf("FindSavedView() error = %v", err)
	}

	links, err := taskdb.ListExternalLinksForTask(context.Background(), db, expected.TaskOneHandle)
	if err != nil {
		t.Fatalf("ListExternalLinksForTask() error = %v", err)
	}
	if got, want := len(links), 1; got != want {
		t.Fatalf("len(links) = %d, want %d", got, want)
	}

	relationships, err := taskdb.ListRelationshipsForTask(context.Background(), db, expected.TaskOneHandle)
	if err != nil {
		t.Fatalf("ListRelationshipsForTask() error = %v", err)
	}
	if got, want := len(relationships), 1; got != want {
		t.Fatalf("len(relationships) = %d, want %d", got, want)
	}

	var eventCount int
	if err := db.QueryRow(`SELECT COUNT(1) FROM events`).Scan(&eventCount); err != nil {
		t.Fatalf("QueryRow(events count) error = %v", err)
	}
	if got, want := eventCount, expected.EventCount; got != want {
		t.Fatalf("eventCount = %d, want %d", got, want)
	}

	var migrationCount int
	if err := db.QueryRow(`SELECT COUNT(1) FROM schema_migrations`).Scan(&migrationCount); err != nil {
		t.Fatalf("QueryRow(schema_migrations count) error = %v", err)
	}
	if migrationCount == 0 {
		t.Fatal("schema_migrations count = 0, want applied migrations preserved")
	}
}

func insertActor(t *testing.T, db *sql.DB, uuid string, handle string, kind string, provider string, externalID string) int64 {
	t.Helper()

	var providerValue any = provider
	if provider == "" {
		providerValue = nil
	}

	result, err := db.Exec(`
		INSERT INTO actors(uuid, handle, kind, provider, external_id, display_name)
		VALUES (?, ?, ?, ?, ?, ?)
	`, uuid, handle, kind, providerValue, externalID, handle)
	if err != nil {
		t.Fatalf("insert actor failed: %v", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("LastInsertId() for actor failed: %v", err)
	}
	return id
}

func insertTask(t *testing.T, db *sql.DB, uuid string, handle string, title string, domainID any, projectID any) int64 {
	t.Helper()

	result, err := db.Exec(`
		INSERT INTO tasks(uuid, handle, title, domain_id, project_id)
		VALUES (?, ?, ?, ?, ?)
	`, uuid, handle, title, domainID, projectID)
	if err != nil {
		t.Fatalf("insert task failed: %v", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("LastInsertId() for task failed: %v", err)
	}
	return id
}
