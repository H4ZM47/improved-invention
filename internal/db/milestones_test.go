package db

import (
	"context"
	"testing"
)

func TestCreateFindListAndUpdateMilestone(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)

	created, err := CreateMilestone(context.Background(), db, MilestoneCreateInput{
		Name:        "v1.0.2",
		Description: "First milestone",
		DueAt:       stringPointer("2026-05-01T12:00:00Z"),
	})
	if err != nil {
		t.Fatalf("CreateMilestone() error = %v", err)
	}
	if got, want := created.Handle, "MILE-1"; got != want {
		t.Fatalf("created.Handle = %q, want %q", got, want)
	}
	if got, want := created.Status, "backlog"; got != want {
		t.Fatalf("created.Status = %q, want %q", got, want)
	}

	found, err := FindMilestone(context.Background(), db, created.Handle)
	if err != nil {
		t.Fatalf("FindMilestone(handle) error = %v", err)
	}
	if got, want := found.UUID, created.UUID; got != want {
		t.Fatalf("found.UUID = %q, want %q", got, want)
	}

	items, err := ListMilestones(context.Background(), db)
	if err != nil {
		t.Fatalf("ListMilestones() error = %v", err)
	}
	if got, want := len(items), 1; got != want {
		t.Fatalf("len(items) = %d, want %d", got, want)
	}
	if got, want := items[0].Handle, created.Handle; got != want {
		t.Fatalf("items[0].Handle = %q, want %q", got, want)
	}

	name := "v1.0.2 dogfood"
	description := "Updated milestone"
	status := "completed"
	dueAt := "2026-05-02T09:30:00Z"
	updated, err := UpdateMilestone(context.Background(), db, MilestoneUpdateInput{
		Reference:   created.Handle,
		Name:        &name,
		Description: &description,
		Status:      &status,
		DueAt:       &dueAt,
	})
	if err != nil {
		t.Fatalf("UpdateMilestone() error = %v", err)
	}
	if got, want := updated.Name, name; got != want {
		t.Fatalf("updated.Name = %q, want %q", got, want)
	}
	if got, want := updated.Status, status; got != want {
		t.Fatalf("updated.Status = %q, want %q", got, want)
	}
	if updated.ClosedAt == nil {
		t.Fatal("updated.ClosedAt = nil, want milestone close timestamp")
	}
}

func TestCreateMilestoneAdvancesHandleSequence(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)

	first, err := CreateMilestone(context.Background(), db, MilestoneCreateInput{Name: "v1.0.1"})
	if err != nil {
		t.Fatalf("CreateMilestone(first) error = %v", err)
	}
	second, err := CreateMilestone(context.Background(), db, MilestoneCreateInput{Name: "v1.0.2"})
	if err != nil {
		t.Fatalf("CreateMilestone(second) error = %v", err)
	}

	if got, want := first.Handle, "MILE-1"; got != want {
		t.Fatalf("first.Handle = %q, want %q", got, want)
	}
	if got, want := second.Handle, "MILE-2"; got != want {
		t.Fatalf("second.Handle = %q, want %q", got, want)
	}
}
