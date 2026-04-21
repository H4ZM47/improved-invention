package app

import (
	"context"
	"testing"
)

func TestDomainAndProjectManagersCreateUpdateAndClose(t *testing.T) {
	t.Parallel()

	db := openActorManagerTestDB(t)
	domainManager := DomainManager{
		DB:        db,
		HumanName: "alex",
	}
	projectManager := ProjectManager{
		DB:        db,
		HumanName: "alex",
	}

	domain, err := domainManager.Create(context.Background(), CreateDomainRequest{
		Name:               "Work",
		Description:        "Primary work domain",
		DefaultAssigneeRef: stringPointer("alex"),
		Tags:               []string{"ops"},
	})
	if err != nil {
		t.Fatalf("CreateDomain() error = %v", err)
	}

	if got, want := domain.Handle, "DOM-1"; got != want {
		t.Fatalf("domain.Handle = %q, want %q", got, want)
	}
	if domain.DefaultAssigneeActorID == nil {
		t.Fatal("domain.DefaultAssigneeActorID = nil, want configured human UUID")
	}

	project, err := projectManager.Create(context.Background(), CreateProjectRequest{
		Name:      "Grind",
		DomainRef: domain.Handle,
	})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}

	if got, want := project.DomainID, domain.UUID; got != want {
		t.Fatalf("project.DomainID = %q, want %q", got, want)
	}

	blocked := "blocked"
	updatedProject, err := projectManager.Update(context.Background(), UpdateProjectRequest{
		Reference: project.Handle,
		Status:    &blocked,
	})
	if err != nil {
		t.Fatalf("UpdateProject() error = %v", err)
	}
	if got, want := updatedProject.Status, "blocked"; got != want {
		t.Fatalf("updatedProject.Status = %q, want %q", got, want)
	}

	active := "active"
	if _, err := domainManager.Update(context.Background(), UpdateDomainRequest{
		Reference: domain.Handle,
		Status:    &active,
	}); err != nil {
		t.Fatalf("UpdateDomain(active) error = %v", err)
	}

	cancelled := "cancelled"
	closedDomain, err := domainManager.Update(context.Background(), UpdateDomainRequest{
		Reference: domain.Handle,
		Status:    &cancelled,
	})
	if err != nil {
		t.Fatalf("UpdateDomain() close error = %v", err)
	}
	if closedDomain.ClosedAt == nil {
		t.Fatal("closedDomain.ClosedAt = nil, want terminal timestamp")
	}

	backlog := "backlog"
	reopenedDomain, err := domainManager.Update(context.Background(), UpdateDomainRequest{
		Reference: domain.Handle,
		Status:    &backlog,
	})
	if err != nil {
		t.Fatalf("UpdateDomain(reopen) error = %v", err)
	}
	if got, want := reopenedDomain.Status, "backlog"; got != want {
		t.Fatalf("reopenedDomain.Status = %q, want %q", got, want)
	}

	completed := "completed"
	closedProject, err := projectManager.Update(context.Background(), UpdateProjectRequest{
		Reference: project.Handle,
		Status:    &completed,
	})
	if err != nil {
		t.Fatalf("UpdateProject(completed) error = %v", err)
	}
	if closedProject.ClosedAt == nil {
		t.Fatal("closedProject.ClosedAt = nil, want terminal timestamp")
	}

	reopenedProject, err := projectManager.Update(context.Background(), UpdateProjectRequest{
		Reference: project.Handle,
		Status:    &backlog,
	})
	if err != nil {
		t.Fatalf("UpdateProject(reopen) error = %v", err)
	}
	if got, want := reopenedProject.Status, "backlog"; got != want {
		t.Fatalf("reopenedProject.Status = %q, want %q", got, want)
	}
}

func stringPointer(value string) *string {
	return &value
}
