package db

import (
	"context"
	"database/sql"
	"testing"
)

func TestCreateAndFindTaskPreserveMilestoneAssignment(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)
	milestone, err := CreateMilestone(context.Background(), db, MilestoneCreateInput{Name: "v1.0.2"})
	if err != nil {
		t.Fatalf("CreateMilestone() error = %v", err)
	}

	task, err := CreateTask(context.Background(), db, TaskCreateInput{
		Title:        "Ship milestone persistence",
		MilestoneRef: &milestone.Handle,
	})
	if err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	if task.MilestoneUUID == nil || *task.MilestoneUUID != milestone.UUID {
		t.Fatalf("task.MilestoneUUID = %v, want %q", task.MilestoneUUID, milestone.UUID)
	}
	if task.MilestoneHandle == nil || *task.MilestoneHandle != milestone.Handle {
		t.Fatalf("task.MilestoneHandle = %v, want %q", task.MilestoneHandle, milestone.Handle)
	}

	found, err := FindTask(context.Background(), db, task.Handle)
	if err != nil {
		t.Fatalf("FindTask() error = %v", err)
	}
	if found.MilestoneHandle == nil || *found.MilestoneHandle != milestone.Handle {
		t.Fatalf("found.MilestoneHandle = %v, want %q", found.MilestoneHandle, milestone.Handle)
	}
}

func TestUpdateTaskCanSetAndClearMilestoneAssignment(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)
	actorID := insertActor(t, db, "actor-1", "ACT-1", "human", "", "alex")
	milestone, err := CreateMilestone(context.Background(), db, MilestoneCreateInput{Name: "v1.0.2"})
	if err != nil {
		t.Fatalf("CreateMilestone() error = %v", err)
	}
	task, err := CreateTask(context.Background(), db, TaskCreateInput{
		Title: "Dogfood milestone assignment",
	})
	if err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	insertOpenClaim(t, db, task.ID, actorID)

	status := "active"
	updated, err := UpdateTask(context.Background(), db, TaskUpdateInput{
		Reference:    task.Handle,
		Status:       &status,
		MilestoneRef: &milestone.Handle,
		ActorID:      &actorID,
	})
	if err != nil {
		t.Fatalf("UpdateTask(set milestone) error = %v", err)
	}
	if updated.MilestoneHandle == nil || *updated.MilestoneHandle != milestone.Handle {
		t.Fatalf("updated.MilestoneHandle = %v, want %q", updated.MilestoneHandle, milestone.Handle)
	}

	clear := ""
	cleared, err := UpdateTask(context.Background(), db, TaskUpdateInput{
		Reference:    task.Handle,
		MilestoneRef: &clear,
		ActorID:      &actorID,
	})
	if err != nil {
		t.Fatalf("UpdateTask(clear milestone) error = %v", err)
	}
	if cleared.MilestoneUUID != nil || cleared.MilestoneHandle != nil {
		t.Fatalf("cleared milestone = (%v, %v), want nil", cleared.MilestoneUUID, cleared.MilestoneHandle)
	}
}

func TestListTasksSupportsMilestoneFilter(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)
	milestoneOne, err := CreateMilestone(context.Background(), db, MilestoneCreateInput{Name: "v1.0.1"})
	if err != nil {
		t.Fatalf("CreateMilestone(milestoneOne) error = %v", err)
	}
	milestoneTwo, err := CreateMilestone(context.Background(), db, MilestoneCreateInput{Name: "v1.0.2"})
	if err != nil {
		t.Fatalf("CreateMilestone(milestoneTwo) error = %v", err)
	}

	if _, err := CreateTask(context.Background(), db, TaskCreateInput{
		Title:        "Included task",
		MilestoneRef: &milestoneOne.Handle,
	}); err != nil {
		t.Fatalf("CreateTask(included) error = %v", err)
	}
	if _, err := CreateTask(context.Background(), db, TaskCreateInput{
		Title:        "Excluded task",
		MilestoneRef: &milestoneTwo.Handle,
	}); err != nil {
		t.Fatalf("CreateTask(excluded) error = %v", err)
	}

	items, err := ListTasks(context.Background(), db, TaskListQuery{
		MilestoneRef: &milestoneOne.Handle,
	})
	if err != nil {
		t.Fatalf("ListTasks() error = %v", err)
	}
	if got, want := len(items), 1; got != want {
		t.Fatalf("len(items) = %d, want %d", got, want)
	}
	if items[0].MilestoneHandle == nil || *items[0].MilestoneHandle != milestoneOne.Handle {
		t.Fatalf("items[0].MilestoneHandle = %v, want %q", items[0].MilestoneHandle, milestoneOne.Handle)
	}
}

func insertOpenClaim(t *testing.T, db *sql.DB, taskID int64, actorID int64) {
	t.Helper()

	if _, err := db.Exec(`
		INSERT INTO claims(uuid, task_id, actor_id, expires_at)
		VALUES ('claim-'+hex(randomblob(8)), ?, ?, '2030-01-01T00:00:00Z')
	`, taskID, actorID); err != nil {
		t.Fatalf("insert open claim failed: %v", err)
	}
}
