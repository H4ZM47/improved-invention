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
