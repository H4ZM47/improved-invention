package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	taskdb "github.com/H4ZM47/improved-invention/internal/db"
)

// TaskManager provides the service-layer task workflows used by commands.
type TaskManager struct {
	DB              *sql.DB
	HumanName       string
	CurrentActorRef string
}

// AssignmentDecisionRequiredError signals that a reclassification needs an explicit assignee choice.
type AssignmentDecisionRequiredError struct {
	TaskHandle            string
	DomainHandle          *string
	ProjectHandle         *string
	DefaultAssigneeHandle *string
}

func (e *AssignmentDecisionRequiredError) Error() string {
	return "changing classification requires an explicit assignee decision"
}

// Create creates a new task with low-friction title-only capture.
func (m TaskManager) Create(ctx context.Context, req CreateTaskRequest) (TaskRecord, error) {
	actorID, err := m.resolveCurrentActorID(ctx)
	if err != nil {
		return TaskRecord{}, err
	}

	classification, err := m.resolveClassification(ctx, nil, req.DomainRef, req.ProjectRef)
	if err != nil {
		return TaskRecord{}, err
	}

	assigneeRef := req.AssigneeRef
	if assigneeRef == nil && classification.defaultAssigneeHandle != nil {
		assigneeRef = classification.defaultAssigneeHandle
	}

	task, err := taskdb.CreateTask(ctx, m.DB, taskdb.TaskCreateInput{
		Title:       req.Title,
		Description: req.Description,
		Tags:        req.Tags,
		DomainRef:   classification.domainRef,
		ProjectRef:  classification.projectRef,
		AssigneeRef: assigneeRef,
		DueAt:       req.DueAt,
		ActorID:     actorID,
	})
	if err != nil {
		return TaskRecord{}, err
	}

	return toTaskRecord(task), nil
}

// List returns the task listing for the provided filters.
func (m TaskManager) List(ctx context.Context, req ListTasksRequest) ([]TaskRecord, error) {
	tasks, err := taskdb.ListTasks(ctx, m.DB, taskdb.TaskListQuery{
		Statuses:       req.Statuses,
		DomainRef:      req.DomainRef,
		ProjectRef:     req.ProjectRef,
		AssigneeRef:    req.AssigneeRef,
		DueBefore:      req.DueBefore,
		DueAfter:       req.DueAfter,
		Tags:           req.Tags,
		Search:         req.Search,
		RepoTarget:     req.RepoTarget,
		WorktreeTarget: req.WorktreeTarget,
	})
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

	current, err := taskdb.FindTask(ctx, m.DB, req.Reference)
	if err != nil {
		return TaskRecord{}, fmt.Errorf("find task %q: %w", req.Reference, err)
	}

	classification, err := m.resolveClassification(ctx, &current, req.DomainRef, req.ProjectRef)
	if err != nil {
		return TaskRecord{}, err
	}

	assigneeRef := req.AssigneeRef
	classificationChanged := changedStringPointer(current.DomainUUID, classification.domainUUID) || changedStringPointer(current.ProjectUUID, classification.projectUUID)
	defaultWouldChangeAssignee := classification.defaultAssigneeUUID != nil && changedStringPointer(current.AssigneeUUID, classification.defaultAssigneeUUID)
	if assigneeRef == nil && classificationChanged && defaultWouldChangeAssignee {
		switch {
		case req.AcceptDefaultAssignee:
			assigneeRef = classification.defaultAssigneeHandle
		case req.KeepAssignee:
			// keep the current assignee unchanged
		default:
			return TaskRecord{}, &AssignmentDecisionRequiredError{
				TaskHandle:            current.Handle,
				DomainHandle:          classification.domainHandle,
				ProjectHandle:         classification.projectHandle,
				DefaultAssigneeHandle: classification.defaultAssigneeHandle,
			}
		}
	} else if assigneeRef == nil && req.AcceptDefaultAssignee && classification.defaultAssigneeHandle != nil {
		assigneeRef = classification.defaultAssigneeHandle
	}

	if req.Status != nil && isTerminalStatus(*req.Status) && actorID != nil {
		err := taskdb.CloseTaskSession(ctx, m.DB, taskdb.SessionEventInput{
			TaskReference: req.Reference,
			ActorID:       *actorID,
		})
		if err != nil && !errors.Is(err, taskdb.ErrSessionNotActive) {
			return TaskRecord{}, err
		}
	}

	task, err := taskdb.UpdateTask(ctx, m.DB, taskdb.TaskUpdateInput{
		Reference:   req.Reference,
		Title:       req.Title,
		Description: req.Description,
		Tags:        req.Tags,
		DomainRef:   classification.domainRef,
		ProjectRef:  classification.projectRef,
		AssigneeRef: assigneeRef,
		DueAt:       req.DueAt,
		Status:      req.Status,
		ActorID:     actorID,
	})
	if err != nil {
		return TaskRecord{}, err
	}

	return toTaskRecord(task), nil
}

// StartSession records the beginning of active work on a task.
func (m TaskManager) StartSession(ctx context.Context, req StartTaskSessionRequest) (TaskSessionRecord, error) {
	return m.recordSessionEvent(ctx, req.Reference, "active", taskdb.StartTaskSession)
}

// PauseSession records a pause in active work on a task.
func (m TaskManager) PauseSession(ctx context.Context, req PauseTaskSessionRequest) (TaskSessionRecord, error) {
	return m.recordSessionEvent(ctx, req.Reference, "paused", taskdb.PauseTaskSession)
}

// ResumeSession records the resumption of active work on a task.
func (m TaskManager) ResumeSession(ctx context.Context, req ResumeTaskSessionRequest) (TaskSessionRecord, error) {
	return m.recordSessionEvent(ctx, req.Reference, "active", taskdb.ResumeTaskSession)
}

// AddManualTime records an explicit backfilled time entry on a claimed task.
func (m TaskManager) AddManualTime(ctx context.Context, req AddManualTimeRequest) (ManualTimeEntryRecord, error) {
	actorID, err := m.resolveCurrentActorID(ctx)
	if err != nil {
		return ManualTimeEntryRecord{}, err
	}
	if actorID == nil {
		return ManualTimeEntryRecord{}, fmt.Errorf("missing current actor for manual time entry")
	}
	if req.StartedAt == nil {
		return ManualTimeEntryRecord{}, fmt.Errorf("manual time entry requires started_at")
	}

	entry, err := taskdb.CreateManualTimeEntry(ctx, m.DB, taskdb.ManualTimeEntryCreateInput{
		TaskReference: req.Reference,
		ActorID:       *actorID,
		StartedAt:     *req.StartedAt,
		Duration:      req.Duration,
		Note:          req.Note,
	})
	if err != nil {
		return ManualTimeEntryRecord{}, err
	}

	return toManualTimeEntryRecord(entry), nil
}

// EditManualTime updates an explicit manual time entry on a claimed task.
func (m TaskManager) EditManualTime(ctx context.Context, req EditManualTimeRequest) (ManualTimeEntryRecord, error) {
	actorID, err := m.resolveCurrentActorID(ctx)
	if err != nil {
		return ManualTimeEntryRecord{}, err
	}
	if actorID == nil {
		return ManualTimeEntryRecord{}, fmt.Errorf("missing current actor for manual time entry")
	}

	entry, err := taskdb.EditManualTimeEntry(ctx, m.DB, taskdb.ManualTimeEntryEditInput{
		TaskReference: req.Reference,
		EntryID:       req.EntryID,
		ActorID:       *actorID,
		StartedAt:     req.StartedAt,
		Duration:      req.Duration,
		Note:          req.Note,
	})
	if err != nil {
		return ManualTimeEntryRecord{}, err
	}

	return toManualTimeEntryRecord(entry), nil
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

func toManualTimeEntryRecord(entry taskdb.ManualTimeEntry) ManualTimeEntryRecord {
	return ManualTimeEntryRecord{
		EntryID:        entry.EntryID,
		TaskHandle:     entry.TaskHandle,
		DurationSecond: int64(entry.Duration / time.Second),
		StartedAt:      entry.StartedAt.UTC().Format(time.RFC3339),
		Note:           entry.Note,
	}
}

func (m TaskManager) recordSessionEvent(ctx context.Context, reference string, state string, run func(context.Context, *sql.DB, taskdb.SessionEventInput) error) (TaskSessionRecord, error) {
	actorID, err := m.resolveCurrentActorID(ctx)
	if err != nil {
		return TaskSessionRecord{}, err
	}
	if actorID == nil {
		return TaskSessionRecord{}, fmt.Errorf("missing current actor for task session event")
	}

	if err := run(ctx, m.DB, taskdb.SessionEventInput{
		TaskReference: reference,
		ActorID:       *actorID,
	}); err != nil {
		return TaskSessionRecord{}, err
	}

	task, err := taskdb.FindTask(ctx, m.DB, reference)
	if err != nil {
		return TaskSessionRecord{}, fmt.Errorf("find task %q after session event: %w", reference, err)
	}
	elapsed, err := taskdb.DeriveElapsedTaskTime(ctx, m.DB, reference)
	if err != nil {
		return TaskSessionRecord{}, err
	}

	return TaskSessionRecord{
		TaskHandle:    task.Handle,
		State:         state,
		ElapsedSecond: int64(elapsed / time.Second),
	}, nil
}

type classificationPreview struct {
	domainRef             *string
	projectRef            *string
	domainUUID            *string
	projectUUID           *string
	domainHandle          *string
	projectHandle         *string
	defaultAssigneeUUID   *string
	defaultAssigneeHandle *string
}

func (m TaskManager) resolveClassification(ctx context.Context, current *taskdb.Task, requestedDomainRef *string, requestedProjectRef *string) (classificationPreview, error) {
	var targetDomain *taskdb.Domain
	var targetProject *taskdb.Project

	if current != nil && current.DomainUUID != nil {
		domain, err := taskdb.FindDomain(ctx, m.DB, *current.DomainUUID)
		if err != nil {
			return classificationPreview{}, fmt.Errorf("find current domain %q: %w", *current.DomainUUID, err)
		}
		targetDomain = &domain
	}
	if current != nil && current.ProjectUUID != nil {
		project, err := taskdb.FindProject(ctx, m.DB, *current.ProjectUUID)
		if err != nil {
			return classificationPreview{}, fmt.Errorf("find current project %q: %w", *current.ProjectUUID, err)
		}
		targetProject = &project
	}

	if requestedDomainRef != nil {
		if *requestedDomainRef == "" {
			targetDomain = nil
		} else {
			domain, err := taskdb.FindDomain(ctx, m.DB, *requestedDomainRef)
			if err != nil {
				return classificationPreview{}, fmt.Errorf("find domain %q: %w", *requestedDomainRef, err)
			}
			targetDomain = &domain
		}
	}
	if requestedProjectRef != nil {
		if *requestedProjectRef == "" {
			targetProject = nil
		} else {
			project, err := taskdb.FindProject(ctx, m.DB, *requestedProjectRef)
			if err != nil {
				return classificationPreview{}, fmt.Errorf("find project %q: %w", *requestedProjectRef, err)
			}
			targetProject = &project
		}
	}

	if targetProject != nil {
		projectDomain, err := taskdb.FindDomain(ctx, m.DB, targetProject.DomainUUID)
		if err != nil {
			return classificationPreview{}, fmt.Errorf("find project domain %q: %w", targetProject.DomainUUID, err)
		}
		if targetDomain == nil {
			targetDomain = &projectDomain
		} else if targetDomain.UUID != targetProject.DomainUUID {
			return classificationPreview{}, fmt.Errorf("%w: project %s belongs to domain %s", taskdb.ErrDomainProjectConstraint, targetProject.Handle, projectDomain.Handle)
		}
	}

	result := classificationPreview{
		domainUUID:    recordStringPointer(targetDomain, func(v *taskdb.Domain) string { return v.UUID }),
		projectUUID:   recordStringPointer(targetProject, func(v *taskdb.Project) string { return v.UUID }),
		domainHandle:  recordStringPointer(targetDomain, func(v *taskdb.Domain) string { return v.Handle }),
		projectHandle: recordStringPointer(targetProject, func(v *taskdb.Project) string { return v.Handle }),
	}
	if targetProject != nil && targetProject.DefaultAssigneeUUID != nil {
		result.defaultAssigneeUUID = targetProject.DefaultAssigneeUUID
		result.defaultAssigneeHandle = targetProject.DefaultAssigneeHandle
	} else if targetDomain != nil {
		result.defaultAssigneeUUID = targetDomain.DefaultAssigneeUUID
		result.defaultAssigneeHandle = targetDomain.DefaultAssigneeHandle
	}

	if requestedDomainRef != nil || requestedProjectRef != nil || (current == nil && (targetDomain != nil || targetProject != nil)) {
		result.domainRef = handleUpdateReference(targetDomain)
		result.projectRef = handleProjectUpdateReference(targetProject, requestedProjectRef, current)
	}

	return result, nil
}

func changedStringPointer(current *string, next *string) bool {
	switch {
	case current == nil && next == nil:
		return false
	case current == nil || next == nil:
		return true
	default:
		return *current != *next
	}
}

func recordStringPointer[T any](value *T, get func(*T) string) *string {
	if value == nil {
		return nil
	}
	v := get(value)
	return &v
}

func handleUpdateReference(domain *taskdb.Domain) *string {
	if domain == nil {
		empty := ""
		return &empty
	}
	handle := domain.Handle
	return &handle
}

func handleProjectUpdateReference(project *taskdb.Project, requestedProjectRef *string, current *taskdb.Task) *string {
	if requestedProjectRef == nil && current != nil {
		return nil
	}
	if requestedProjectRef == nil && current == nil && project == nil {
		return nil
	}
	if project == nil {
		empty := ""
		return &empty
	}
	handle := project.Handle
	return &handle
}

func isTerminalStatus(status string) bool {
	return status == "completed" || status == "cancelled"
}

func toClaimRecord(claim taskdb.Claim) ClaimRecord {
	return ClaimRecord{
		TaskHandle:  claim.TaskHandle,
		ActorHandle: claim.ActorHandle,
		Status:      "active",
	}
}
