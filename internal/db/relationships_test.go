package db

import (
	"context"
	"errors"
	"testing"
)

func TestCreateListAndRemoveRelationship(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)
	sourceID := insertTask(t, db, "task-source", "TASK-1", "Source", nil, nil)
	_ = sourceID
	insertTask(t, db, "task-target", "TASK-2", "Target", nil, nil)

	created, err := CreateRelationship(context.Background(), db, RelationshipCreateInput{
		SourceTaskReference: "TASK-1",
		TargetTaskReference: "TASK-2",
		RelationshipType:    RelationshipTypeBlocks,
	})
	if err != nil {
		t.Fatalf("CreateRelationship() error = %v", err)
	}

	if got, want := created.SourceTaskHandle, "TASK-1"; got != want {
		t.Fatalf("SourceTaskHandle = %q, want %q", got, want)
	}
	if got, want := created.TargetTaskHandle, "TASK-2"; got != want {
		t.Fatalf("TargetTaskHandle = %q, want %q", got, want)
	}
	if got, want := created.RelationshipType, RelationshipTypeBlocks; got != want {
		t.Fatalf("RelationshipType = %q, want %q", got, want)
	}

	got, err := ListRelationshipsForTask(context.Background(), db, "TASK-1")
	if err != nil {
		t.Fatalf("ListRelationshipsForTask() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len(ListRelationships()) = %d, want 1", len(got))
	}
	if got[0].UUID != created.UUID {
		t.Fatalf("listed relationship UUID = %q, want %q", got[0].UUID, created.UUID)
	}

	removed, err := RemoveRelationship(context.Background(), db, RelationshipRemoveInput{
		SourceTaskReference: "TASK-1",
		TargetTaskReference: "TASK-2",
		RelationshipType:    RelationshipTypeBlocks,
	})
	if err != nil {
		t.Fatalf("RemoveRelationship() error = %v", err)
	}
	if removed.UUID != created.UUID {
		t.Fatalf("removed relationship UUID = %q, want %q", removed.UUID, created.UUID)
	}

	got, err = ListRelationshipsForTask(context.Background(), db, "TASK-1")
	if err != nil {
		t.Fatalf("ListRelationshipsForTask() after remove error = %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("len(ListRelationships()) after remove = %d, want 0", len(got))
	}
}

func TestCreateRelationshipRejectsInvalidType(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)
	insertTask(t, db, "task-source", "TASK-1", "Source", nil, nil)
	insertTask(t, db, "task-target", "TASK-2", "Target", nil, nil)

	_, err := CreateRelationship(context.Background(), db, RelationshipCreateInput{
		SourceTaskReference: "TASK-1",
		TargetTaskReference: "TASK-2",
		RelationshipType:    "depends_on",
	})
	if !errors.Is(err, ErrInvalidRelationshipType) {
		t.Fatalf("CreateRelationship() error = %v, want ErrInvalidRelationshipType", err)
	}
}

func TestCreateRelationshipEnforcesOneParentHierarchy(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)
	insertTask(t, db, "task-parent-1", "TASK-1", "Parent 1", nil, nil)
	insertTask(t, db, "task-parent-2", "TASK-2", "Parent 2", nil, nil)
	insertTask(t, db, "task-child", "TASK-3", "Child", nil, nil)

	if _, err := CreateRelationship(context.Background(), db, RelationshipCreateInput{
		SourceTaskReference: "TASK-1",
		TargetTaskReference: "TASK-3",
		RelationshipType:    RelationshipTypeParentChild,
	}); err != nil {
		t.Fatalf("first CreateRelationship() error = %v", err)
	}

	if _, err := CreateRelationship(context.Background(), db, RelationshipCreateInput{
		SourceTaskReference: "TASK-2",
		TargetTaskReference: "TASK-3",
		RelationshipType:    RelationshipTypeParentChild,
	}); err == nil {
		t.Fatal("second CreateRelationship() succeeded, want constraint failure")
	}
}

func TestCreateRelationshipRejectsSelfLink(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)
	insertTask(t, db, "task-source", "TASK-1", "Source", nil, nil)

	_, err := CreateRelationship(context.Background(), db, RelationshipCreateInput{
		SourceTaskReference: "TASK-1",
		TargetTaskReference: "TASK-1",
		RelationshipType:    RelationshipTypeBlocks,
	})
	if !errors.Is(err, ErrSelfRelationship) {
		t.Fatalf("CreateRelationship() error = %v, want ErrSelfRelationship", err)
	}
}
