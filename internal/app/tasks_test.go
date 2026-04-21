package app

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	taskdb "github.com/H4ZM47/improved-invention/internal/db"
)

func TestTaskManagerCreateAndUpdateLifecycle(t *testing.T) {
	t.Parallel()

	db := openActorManagerTestDB(t)
	manager := TaskManager{
		DB:        db,
		HumanName: "alex",
	}

	created, err := manager.Create(context.Background(), CreateTaskRequest{
		Title: "Write task manager",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if got, want := created.Status, "backlog"; got != want {
		t.Fatalf("created.Status = %q, want %q", got, want)
	}

	if _, err := manager.Claim(context.Background(), ClaimTaskRequest{
		Reference: created.Handle,
		Lease:     time.Hour,
	}); err != nil {
		t.Fatalf("Claim() error = %v", err)
	}

	active := "active"
	updated, err := manager.Update(context.Background(), UpdateTaskRequest{
		Reference: created.Handle,
		Status:    &active,
	})
	if err != nil {
		t.Fatalf("Update(active) error = %v", err)
	}

	if got, want := updated.Status, "active"; got != want {
		t.Fatalf("updated.Status = %q, want %q", got, want)
	}

	completed := "completed"
	closed, err := manager.Update(context.Background(), UpdateTaskRequest{
		Reference: created.Handle,
		Status:    &completed,
	})
	if err != nil {
		t.Fatalf("Update(completed) error = %v", err)
	}

	if closed.ClosedAt == nil {
		t.Fatal("closed.ClosedAt = nil, want terminal timestamp")
	}
}

func TestTaskManagerUpdateRequiresClaim(t *testing.T) {
	t.Parallel()

	db := openActorManagerTestDB(t)
	manager := TaskManager{
		DB:        db,
		HumanName: "alex",
	}

	created, err := manager.Create(context.Background(), CreateTaskRequest{
		Title: "Guard updates with claim",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	active := "active"
	_, err = manager.Update(context.Background(), UpdateTaskRequest{
		Reference: created.Handle,
		Status:    &active,
	})
	if !errors.Is(err, taskdb.ErrClaimRequired) {
		t.Fatalf("Update() error = %v, want ErrClaimRequired", err)
	}
}

func TestTaskManagerCreateInheritsMostSpecificDefaultAssignee(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openActorManagerTestDB(t)

	actorManager := ActorManager{
		DB:        db,
		HumanName: "alex",
	}
	projectManager := ProjectManager{
		DB:        db,
		HumanName: "alex",
	}
	domainManager := DomainManager{
		DB:        db,
		HumanName: "alex",
	}
	taskManager := TaskManager{
		DB:        db,
		HumanName: "alex",
	}

	human, err := actorManager.BootstrapConfiguredHumanActor(ctx)
	if err != nil {
		t.Fatalf("BootstrapConfiguredHumanActor() error = %v", err)
	}
	agent, err := actorManager.GetOrCreateAgentActor(ctx, "codex:agent-7")
	if err != nil {
		t.Fatalf("GetOrCreateAgentActor() error = %v", err)
	}

	domain, err := domainManager.Create(ctx, CreateDomainRequest{
		Name:               "Work",
		DefaultAssigneeRef: &human.Handle,
	})
	if err != nil {
		t.Fatalf("CreateDomain() error = %v", err)
	}
	project, err := projectManager.Create(ctx, CreateProjectRequest{
		Name:               "Task CLI",
		DomainRef:          domain.Handle,
		DefaultAssigneeRef: &agent.Handle,
	})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}

	task, err := taskManager.Create(ctx, CreateTaskRequest{
		Title:      "Implement inheritance",
		ProjectRef: &project.Handle,
	})
	if err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}

	if task.AssigneeActorID == nil {
		t.Fatal("task.AssigneeActorID = nil, want inherited project default")
	}
	if got, want := *task.AssigneeActorID, agent.UUID; got != want {
		t.Fatalf("task.AssigneeActorID = %q, want %q", got, want)
	}
	if task.DomainID == nil || *task.DomainID != domain.UUID {
		t.Fatalf("task.DomainID = %v, want %q", task.DomainID, domain.UUID)
	}
}

func TestTaskManagerUpdateRequiresAssigneeDecisionForReclassification(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openActorManagerTestDB(t)

	actorManager := ActorManager{
		DB:        db,
		HumanName: "alex",
	}
	domainManager := DomainManager{
		DB:        db,
		HumanName: "alex",
	}
	taskManager := TaskManager{
		DB:        db,
		HumanName: "alex",
	}

	human, err := actorManager.BootstrapConfiguredHumanActor(ctx)
	if err != nil {
		t.Fatalf("BootstrapConfiguredHumanActor() error = %v", err)
	}
	agent, err := actorManager.GetOrCreateAgentActor(ctx, "codex:agent-9")
	if err != nil {
		t.Fatalf("GetOrCreateAgentActor() error = %v", err)
	}

	home, err := domainManager.Create(ctx, CreateDomainRequest{
		Name:               "Home",
		DefaultAssigneeRef: &human.Handle,
	})
	if err != nil {
		t.Fatalf("CreateDomain(home) error = %v", err)
	}
	work, err := domainManager.Create(ctx, CreateDomainRequest{
		Name:               "Work",
		DefaultAssigneeRef: &agent.Handle,
	})
	if err != nil {
		t.Fatalf("CreateDomain(work) error = %v", err)
	}

	task, err := taskManager.Create(ctx, CreateTaskRequest{
		Title:     "Reclassify me",
		DomainRef: &home.Handle,
	})
	if err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}

	if _, err := taskManager.Claim(ctx, ClaimTaskRequest{
		Reference: task.Handle,
		Lease:     time.Hour,
	}); err != nil {
		t.Fatalf("Claim() error = %v", err)
	}

	_, err = taskManager.Update(ctx, UpdateTaskRequest{
		Reference: task.Handle,
		DomainRef: &work.Handle,
	})
	var decisionErr *AssignmentDecisionRequiredError
	if !errors.As(err, &decisionErr) {
		t.Fatalf("Update() error = %v, want AssignmentDecisionRequiredError", err)
	}

	updated, err := taskManager.Update(ctx, UpdateTaskRequest{
		Reference:             task.Handle,
		DomainRef:             &work.Handle,
		AcceptDefaultAssignee: true,
	})
	if err != nil {
		t.Fatalf("Update(accept default) error = %v", err)
	}

	if updated.AssigneeActorID == nil || *updated.AssigneeActorID != agent.UUID {
		t.Fatalf("updated.AssigneeActorID = %v, want %q", updated.AssigneeActorID, agent.UUID)
	}
}

func TestTaskManagerCloseAutoClosesActiveSession(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openActorManagerTestDB(t)
	manager := TaskManager{
		DB:        db,
		HumanName: "alex",
	}

	task, err := manager.Create(ctx, CreateTaskRequest{
		Title: "Close session with task",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if _, err := manager.Claim(ctx, ClaimTaskRequest{
		Reference: task.Handle,
		Lease:     time.Hour,
	}); err != nil {
		t.Fatalf("Claim() error = %v", err)
	}

	if _, err := manager.StartSession(ctx, StartTaskSessionRequest{
		Reference: task.Handle,
	}); err != nil {
		t.Fatalf("StartSession() error = %v", err)
	}

	active := "active"
	if _, err := manager.Update(ctx, UpdateTaskRequest{
		Reference: task.Handle,
		Status:    &active,
	}); err != nil {
		t.Fatalf("Update(active) error = %v", err)
	}

	completed := "completed"
	if _, err := manager.Update(ctx, UpdateTaskRequest{
		Reference: task.Handle,
		Status:    &completed,
	}); err != nil {
		t.Fatalf("Update(completed) error = %v", err)
	}

	assertTaskEventCount(t, db, task.UUID, "session_closed", 1)
}

func TestTaskManagerAddAndEditManualTime(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openActorManagerTestDB(t)
	manager := TaskManager{
		DB:        db,
		HumanName: "alex",
	}

	task, err := manager.Create(ctx, CreateTaskRequest{
		Title: "Record manual time",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if _, err := manager.Claim(ctx, ClaimTaskRequest{
		Reference: task.Handle,
		Lease:     time.Hour,
	}); err != nil {
		t.Fatalf("Claim() error = %v", err)
	}

	startedAt := time.Date(2026, time.April, 21, 9, 0, 0, 0, time.UTC)
	entry, err := manager.AddManualTime(ctx, AddManualTimeRequest{
		Reference: task.Handle,
		Duration:  45 * time.Minute,
		StartedAt: &startedAt,
		Note:      "Imported from notes",
	})
	if err != nil {
		t.Fatalf("AddManualTime() error = %v", err)
	}
	if entry.EntryID == "" {
		t.Fatal("entry.EntryID = empty, want generated manual entry id")
	}
	if got, want := entry.StartedAt, startedAt.Format(time.RFC3339); got != want {
		t.Fatalf("entry.StartedAt = %q, want %q", got, want)
	}

	updatedDuration := 75 * time.Minute
	updatedNote := "Corrected import"
	updated, err := manager.EditManualTime(ctx, EditManualTimeRequest{
		Reference: task.Handle,
		EntryID:   entry.EntryID,
		Duration:  &updatedDuration,
		Note:      &updatedNote,
	})
	if err != nil {
		t.Fatalf("EditManualTime() error = %v", err)
	}
	if got, want := updated.DurationSecond, int64(updatedDuration/time.Second); got != want {
		t.Fatalf("updated.DurationSecond = %d, want %d", got, want)
	}
	if got, want := updated.Note, updatedNote; got != want {
		t.Fatalf("updated.Note = %q, want %q", got, want)
	}

	total, err := taskdb.DeriveManualTaskTime(ctx, db, task.Handle)
	if err != nil {
		t.Fatalf("DeriveManualTaskTime() error = %v", err)
	}
	if got, want := total, updatedDuration; got != want {
		t.Fatalf("DeriveManualTaskTime() = %s, want %s", got, want)
	}
}

func TestTaskManagerAddManualTimeRequiresClaim(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openActorManagerTestDB(t)
	manager := TaskManager{
		DB:        db,
		HumanName: "alex",
	}

	task, err := manager.Create(ctx, CreateTaskRequest{
		Title: "Claim-gated manual time",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	startedAt := time.Date(2026, time.April, 21, 9, 0, 0, 0, time.UTC)
	_, err = manager.AddManualTime(ctx, AddManualTimeRequest{
		Reference: task.Handle,
		Duration:  30 * time.Minute,
		StartedAt: &startedAt,
	})
	if !errors.Is(err, taskdb.ErrClaimRequired) {
		t.Fatalf("AddManualTime() error = %v, want ErrClaimRequired", err)
	}
}

func TestTaskManagerListAppliesFiltersAndSearch(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openActorManagerTestDB(t)

	actorManager := ActorManager{
		DB:        db,
		HumanName: "alex",
	}
	domainManager := DomainManager{
		DB:        db,
		HumanName: "alex",
	}
	projectManager := ProjectManager{
		DB:        db,
		HumanName: "alex",
	}
	taskManager := TaskManager{
		DB:        db,
		HumanName: "alex",
	}

	human, err := actorManager.BootstrapConfiguredHumanActor(ctx)
	if err != nil {
		t.Fatalf("BootstrapConfiguredHumanActor() error = %v", err)
	}
	domain, err := domainManager.Create(ctx, CreateDomainRequest{Name: "Work"})
	if err != nil {
		t.Fatalf("CreateDomain() error = %v", err)
	}
	project, err := projectManager.Create(ctx, CreateProjectRequest{
		Name:      "Task CLI",
		DomainRef: domain.Handle,
	})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}

	dueAt := "2026-04-21T12:00:00Z"
	task, err := taskManager.Create(ctx, CreateTaskRequest{
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
	if _, err := taskManager.Claim(ctx, ClaimTaskRequest{
		Reference: task.Handle,
		Lease:     time.Hour,
	}); err != nil {
		t.Fatalf("Claim() error = %v", err)
	}
	active := "active"
	if _, err := taskManager.Update(ctx, UpdateTaskRequest{
		Reference: task.Handle,
		Status:    &active,
	}); err != nil {
		t.Fatalf("Update(active) error = %v", err)
	}

	_, err = taskManager.Create(ctx, CreateTaskRequest{
		Title:       "Write docs",
		Description: "General docs work",
		Tags:        []string{"docs"},
	})
	if err != nil {
		t.Fatalf("CreateTask(other) error = %v", err)
	}

	items, err := taskManager.List(ctx, ListTasksRequest{
		Statuses:    []string{"active"},
		DomainRef:   &domain.Handle,
		ProjectRef:  &project.Handle,
		AssigneeRef: &human.Handle,
		DueBefore:   stringRef("2026-04-21T23:59:59Z"),
		DueAfter:    stringRef("2026-04-21T00:00:00Z"),
		Tags:        []string{"cli"},
		Search:      "contract",
	})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if got, want := len(items), 1; got != want {
		t.Fatalf("len(items) = %d, want %d", got, want)
	}
	if got, want := items[0].Handle, task.Handle; got != want {
		t.Fatalf("items[0].Handle = %q, want %q", got, want)
	}
}

func stringRef(value string) *string {
	return &value
}

func assertTaskEventCount(t *testing.T, db *sql.DB, taskUUID string, eventType string, want int) {
	t.Helper()

	var got int
	if err := db.QueryRow(`
		SELECT count(*)
		FROM events
		WHERE entity_type = 'task' AND entity_uuid = ? AND event_type = ?
	`, taskUUID, eventType).Scan(&got); err != nil {
		t.Fatalf("count task events failed: %v", err)
	}

	if got != want {
		t.Fatalf("count(%s) = %d, want %d", eventType, got, want)
	}
}
