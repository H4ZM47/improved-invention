package app

import (
	"context"
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
