package app

import (
	"context"
	"database/sql"
	"fmt"

	taskdb "github.com/H4ZM47/improved-invention/internal/db"
)

// TaskManager provides the service-layer task workflows used by commands.
type TaskManager struct {
	DB              *sql.DB
	HumanName       string
	CurrentActorRef string
}

// Create creates a new task with low-friction title-only capture.
func (m TaskManager) Create(ctx context.Context, req CreateTaskRequest) (TaskRecord, error) {
	actorID, err := m.resolveCurrentActorID(ctx)
	if err != nil {
		return TaskRecord{}, err
	}

	task, err := taskdb.CreateTask(ctx, m.DB, taskdb.TaskCreateInput{
		Title:       req.Title,
		Description: req.Description,
		ActorID:     actorID,
	})
	if err != nil {
		return TaskRecord{}, err
	}

	return toTaskRecord(task), nil
}

// List returns the default task listing.
func (m TaskManager) List(ctx context.Context, _ ListTasksRequest) ([]TaskRecord, error) {
	tasks, err := taskdb.ListTasks(ctx, m.DB)
	if err != nil {
		return nil, err
	}

	records := make([]TaskRecord, 0, len(tasks))
	for _, task := range tasks {
		records = append(records, toTaskRecord(task))
	}
	return records, nil
}

// Show resolves a single task.
func (m TaskManager) Show(ctx context.Context, req ShowTaskRequest) (TaskRecord, error) {
	task, err := taskdb.FindTask(ctx, m.DB, req.Reference)
	if err != nil {
		return TaskRecord{}, fmt.Errorf("find task %q: %w", req.Reference, err)
	}

	return toTaskRecord(task), nil
}

// Update applies task field and lifecycle updates.
func (m TaskManager) Update(ctx context.Context, req UpdateTaskRequest) (TaskRecord, error) {
	actorID, err := m.resolveCurrentActorID(ctx)
	if err != nil {
		return TaskRecord{}, err
	}

	task, err := taskdb.UpdateTask(ctx, m.DB, taskdb.TaskUpdateInput{
		Reference:   req.Reference,
		Title:       req.Title,
		Description: req.Description,
		Tags:        req.Tags,
		DomainRef:   req.DomainRef,
		ProjectRef:  req.ProjectRef,
		DueAt:       req.DueAt,
		Status:      req.Status,
		ActorID:     actorID,
	})
	if err != nil {
		return TaskRecord{}, err
	}

	return toTaskRecord(task), nil
}

// Claim acquires an exclusive claim on a task for the current actor.
func (m TaskManager) Claim(ctx context.Context, req ClaimTaskRequest) (ClaimRecord, error) {
	actorID, err := m.resolveCurrentActorID(ctx)
	if err != nil {
		return ClaimRecord{}, err
	}
	if actorID == nil {
		return ClaimRecord{}, fmt.Errorf("missing current actor for claim")
	}

	claim, err := taskdb.AcquireClaim(ctx, m.DB, taskdb.ClaimAcquireInput{
		TaskReference: req.Reference,
		ActorID:       *actorID,
		Lease:         req.Lease,
	})
	if err != nil {
		return ClaimRecord{}, err
	}

	return toClaimRecord(claim), nil
}

// RenewClaim extends an existing claim held by the current actor.
func (m TaskManager) RenewClaim(ctx context.Context, req RenewClaimRequest) (ClaimRecord, error) {
	actorID, err := m.resolveCurrentActorID(ctx)
	if err != nil {
		return ClaimRecord{}, err
	}
	if actorID == nil {
		return ClaimRecord{}, fmt.Errorf("missing current actor for claim renewal")
	}

	claim, err := taskdb.RenewClaim(ctx, m.DB, taskdb.ClaimMutationInput{
		TaskReference: req.Reference,
		ActorID:       *actorID,
		Lease:         req.Lease,
	})
	if err != nil {
		return ClaimRecord{}, err
	}

	return toClaimRecord(claim), nil
}

// ReleaseClaim releases an existing claim held by the current actor.
func (m TaskManager) ReleaseClaim(ctx context.Context, req ReleaseClaimRequest) error {
	actorID, err := m.resolveCurrentActorID(ctx)
	if err != nil {
		return err
	}
	if actorID == nil {
		return fmt.Errorf("missing current actor for claim release")
	}

	_, err = taskdb.ReleaseClaim(ctx, m.DB, taskdb.ClaimMutationInput{
		TaskReference: req.Reference,
		ActorID:       *actorID,
	})
	return err
}

// Unlock releases any current claim on a task for exceptional recovery.
func (m TaskManager) Unlock(ctx context.Context, req UnlockTaskRequest) error {
	actorID, err := m.resolveCurrentActorID(ctx)
	if err != nil {
		return err
	}
	if actorID == nil {
		return fmt.Errorf("missing current actor for manual unlock")
	}

	_, err = taskdb.UnlockClaim(ctx, m.DB, taskdb.ClaimMutationInput{
		TaskReference: req.Reference,
		ActorID:       *actorID,
	})
	return err
}

func (m TaskManager) resolveCurrentActorID(ctx context.Context) (*int64, error) {
	actorManager := ActorManager{
		DB:        m.DB,
		HumanName: m.HumanName,
	}

	if m.CurrentActorRef != "" {
		if _, err := taskdb.ParseAgentIdentity(m.CurrentActorRef); err == nil {
			actor, err := actorManager.GetOrCreateAgentActor(ctx, m.CurrentActorRef)
			if err != nil {
				return nil, err
			}
			return lookupActorID(ctx, m.DB, actor.Handle)
		}
	}

	human, err := actorManager.BootstrapConfiguredHumanActor(ctx)
	if err != nil {
		return nil, err
	}
	return lookupActorID(ctx, m.DB, human.Handle)
}

func lookupActorID(ctx context.Context, db *sql.DB, handle string) (*int64, error) {
	actor, err := taskdb.FindActor(ctx, db, handle)
	if err != nil {
		return nil, err
	}
	return &actor.ID, nil
}

func toTaskRecord(task taskdb.Task) TaskRecord {
	return TaskRecord{
		Handle:          task.Handle,
		UUID:            task.UUID,
		Title:           task.Title,
		Description:     task.Description,
		Status:          task.Status,
		DomainID:        task.DomainUUID,
		ProjectID:       task.ProjectUUID,
		AssigneeActorID: task.AssigneeUUID,
		DueAt:           task.DueAt,
		Tags:            task.Tags,
		CreatedAt:       task.CreatedAt,
		UpdatedAt:       task.UpdatedAt,
		ClosedAt:        task.ClosedAt,
	}
}

func toClaimRecord(claim taskdb.Claim) ClaimRecord {
	return ClaimRecord{
		TaskHandle:  claim.TaskHandle,
		ActorHandle: claim.ActorHandle,
		Status:      "active",
	}
}
