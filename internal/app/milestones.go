package app

import (
	"context"
	"database/sql"

	taskdb "github.com/H4ZM47/grind/internal/db"
)

// MilestoneManager provides service-layer milestone workflows used by commands.
type MilestoneManager struct {
	DB              *sql.DB
	HumanName       string
	CurrentActorRef string
}

// Create inserts a new milestone.
func (m MilestoneManager) Create(ctx context.Context, req CreateMilestoneRequest) (MilestoneRecord, error) {
	actorID, err := TaskManager(m).resolveCurrentActorID(ctx)
	if err != nil {
		return MilestoneRecord{}, err
	}

	milestone, err := taskdb.CreateMilestone(ctx, m.DB, taskdb.MilestoneCreateInput{
		Name:        req.Name,
		Description: req.Description,
		DueAt:       req.DueAt,
		ActorID:     actorID,
	})
	if err != nil {
		return MilestoneRecord{}, err
	}

	return toMilestoneRecord(milestone), nil
}

// List returns the current milestone set.
func (m MilestoneManager) List(ctx context.Context, _ ListMilestonesRequest) ([]MilestoneRecord, error) {
	milestones, err := taskdb.ListMilestones(ctx, m.DB)
	if err != nil {
		return nil, err
	}

	records := make([]MilestoneRecord, 0, len(milestones))
	for _, milestone := range milestones {
		records = append(records, toMilestoneRecord(milestone))
	}
	return records, nil
}

// Show resolves one milestone by handle or UUID.
func (m MilestoneManager) Show(ctx context.Context, req ShowMilestoneRequest) (MilestoneRecord, error) {
	milestone, err := taskdb.FindMilestone(ctx, m.DB, req.Reference)
	if err != nil {
		return MilestoneRecord{}, err
	}
	return toMilestoneRecord(milestone), nil
}

// Update mutates a milestone.
func (m MilestoneManager) Update(ctx context.Context, req UpdateMilestoneRequest) (MilestoneRecord, error) {
	actorID, err := TaskManager(m).resolveCurrentActorID(ctx)
	if err != nil {
		return MilestoneRecord{}, err
	}

	milestone, err := taskdb.UpdateMilestone(ctx, m.DB, taskdb.MilestoneUpdateInput{
		Reference:   req.Reference,
		Name:        req.Name,
		Description: req.Description,
		DueAt:       req.DueAt,
		Status:      req.Status,
		ActorID:     actorID,
	})
	if err != nil {
		return MilestoneRecord{}, err
	}

	return toMilestoneRecord(milestone), nil
}

func toMilestoneRecord(milestone taskdb.Milestone) MilestoneRecord {
	return MilestoneRecord{
		Handle:      milestone.Handle,
		UUID:        milestone.UUID,
		Name:        milestone.Name,
		Description: milestone.Description,
		Status:      milestone.Status,
		DueAt:       milestone.DueAt,
		CreatedAt:   milestone.CreatedAt,
		UpdatedAt:   milestone.UpdatedAt,
		ClosedAt:    milestone.ClosedAt,
	}
}
