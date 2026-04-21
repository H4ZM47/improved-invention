package app

import (
	"context"
	"time"

	"github.com/H4ZM47/grind/internal/gitctx"
)

// Services groups the service-layer interfaces that CLI command handlers call.
type Services interface {
	Tasks() TaskService
	Projects() ProjectService
	Domains() DomainService
	Actors() ActorService
	Relationships() RelationshipService
	Links() LinkService
	Views() ViewService
	Runtime() RuntimeService
}

// RuntimeService exposes process-level inspection commands.
type RuntimeService interface {
	ShowConfig(context.Context, ShowConfigRequest) (ShowConfigResult, error)
}

// TaskService defines task-specific workflows exposed to the CLI.
type TaskService interface {
	Create(context.Context, CreateTaskRequest) (TaskRecord, error)
	List(context.Context, ListTasksRequest) ([]TaskRecord, error)
	Show(context.Context, ShowTaskRequest) (TaskRecord, error)
	Update(context.Context, UpdateTaskRequest) (TaskRecord, error)
	StartSession(context.Context, StartTaskSessionRequest) (TaskSessionRecord, error)
	PauseSession(context.Context, PauseTaskSessionRequest) (TaskSessionRecord, error)
	ResumeSession(context.Context, ResumeTaskSessionRequest) (TaskSessionRecord, error)
	AddManualTime(context.Context, AddManualTimeRequest) (ManualTimeEntryRecord, error)
	EditManualTime(context.Context, EditManualTimeRequest) (ManualTimeEntryRecord, error)
	Claim(context.Context, ClaimTaskRequest) (ClaimRecord, error)
	RenewClaim(context.Context, RenewClaimRequest) (ClaimRecord, error)
	ReleaseClaim(context.Context, ReleaseClaimRequest) error
	Unlock(context.Context, UnlockTaskRequest) error
}

// ProjectService defines project workflows exposed to the CLI.
type ProjectService interface {
	Create(context.Context, CreateProjectRequest) (ProjectRecord, error)
	List(context.Context, ListProjectsRequest) ([]ProjectRecord, error)
	Show(context.Context, ShowProjectRequest) (ProjectRecord, error)
	Update(context.Context, UpdateProjectRequest) (ProjectRecord, error)
}

// DomainService defines domain workflows exposed to the CLI.
type DomainService interface {
	Create(context.Context, CreateDomainRequest) (DomainRecord, error)
	List(context.Context, ListDomainsRequest) ([]DomainRecord, error)
	Show(context.Context, ShowDomainRequest) (DomainRecord, error)
	Update(context.Context, UpdateDomainRequest) (DomainRecord, error)
}

// ActorService defines actor inspection and human configuration workflows.
type ActorService interface {
	List(context.Context, ListActorsRequest) ([]ActorRecord, error)
	Show(context.Context, ShowActorRequest) (ActorRecord, error)
	BootstrapConfiguredHumanActor(context.Context) (ActorRecord, error)
	GetOrCreateAgentActor(context.Context, string) (ActorRecord, error)
	ParseAgentIdentity(string) (AgentIdentityRecord, error)
}

// RelationshipService defines task relationship workflows.
type RelationshipService interface {
	Create(context.Context, CreateRelationshipRequest) (RelationshipRecord, error)
	List(context.Context, ListRelationshipsRequest) ([]RelationshipRecord, error)
	Remove(context.Context, RemoveRelationshipRequest) error
}

// LinkService defines task external-link workflows.
type LinkService interface {
	Create(context.Context, CreateLinkRequest) (LinkRecord, error)
	AttachCurrentRepoContext(context.Context, AttachCurrentRepoContextRequest) (AttachCurrentRepoContextResult, error)
	List(context.Context, ListLinksRequest) ([]LinkRecord, error)
	Remove(context.Context, RemoveLinkRequest) error
}

// ViewService defines saved-view workflows.
type ViewService interface {
	Create(context.Context, CreateViewRequest) (ViewRecord, error)
	List(context.Context, ListViewsRequest) ([]ViewRecord, error)
	Show(context.Context, ShowViewRequest) (ViewRecord, error)
	Update(context.Context, UpdateViewRequest) (ViewRecord, error)
	Delete(context.Context, DeleteViewRequest) error
}

type ShowConfigRequest struct{}

type ShowConfigResult struct {
	DBPath      string
	Actor       string
	BusyTimeout string
	ClaimLease  string
}

type TaskRecord struct {
	Handle          string   `json:"handle"`
	UUID            string   `json:"uuid"`
	Title           string   `json:"title"`
	Description     string   `json:"description"`
	Status          string   `json:"status"`
	DomainID        *string  `json:"domain_id"`
	ProjectID       *string  `json:"project_id"`
	AssigneeActorID *string  `json:"assignee_actor_id"`
	DueAt           *string  `json:"due_at"`
	Tags            []string `json:"tags"`
	CreatedAt       string   `json:"created_at"`
	UpdatedAt       string   `json:"updated_at"`
	ClosedAt        *string  `json:"closed_at"`
}

type ProjectRecord struct {
	Handle                 string   `json:"handle"`
	UUID                   string   `json:"uuid"`
	Name                   string   `json:"name"`
	Description            string   `json:"description"`
	Status                 string   `json:"status"`
	DomainID               string   `json:"domain_id"`
	DefaultAssigneeActorID *string  `json:"default_assignee_actor_id"`
	AssigneeActorID        *string  `json:"assignee_actor_id"`
	DueAt                  *string  `json:"due_at"`
	Tags                   []string `json:"tags"`
	CreatedAt              string   `json:"created_at"`
	UpdatedAt              string   `json:"updated_at"`
	ClosedAt               *string  `json:"closed_at"`
}

type DomainRecord struct {
	Handle                 string   `json:"handle"`
	UUID                   string   `json:"uuid"`
	Name                   string   `json:"name"`
	Description            string   `json:"description"`
	Status                 string   `json:"status"`
	DefaultAssigneeActorID *string  `json:"default_assignee_actor_id"`
	AssigneeActorID        *string  `json:"assignee_actor_id"`
	DueAt                  *string  `json:"due_at"`
	Tags                   []string `json:"tags"`
	CreatedAt              string   `json:"created_at"`
	UpdatedAt              string   `json:"updated_at"`
	ClosedAt               *string  `json:"closed_at"`
}

type ActorRecord struct {
	Handle      string `json:"handle"`
	Kind        string `json:"kind"`
	Label       string `json:"label"`
	UUID        string `json:"uuid"`
	Provider    string `json:"provider"`
	ExternalID  string `json:"external_id"`
	DisplayName string `json:"display_name"`
}

type AgentIdentityRecord struct {
	Provider   string `json:"provider"`
	ExternalID string `json:"external_id"`
}

type ViewRecord struct {
	Name       string           `json:"name"`
	UUID       string           `json:"uuid"`
	EntityType string           `json:"entity_type"`
	Filters    SavedViewFilters `json:"filters"`
	CreatedAt  string           `json:"created_at"`
	UpdatedAt  string           `json:"updated_at"`
}

// SavedViewFilters is the JSON-serializable filter payload stored with a saved view.
//
// Fields mirror ListTasksRequest but use zero values (empty string, nil slice)
// to mean "no filter" so saved payloads round-trip cleanly through JSON.
type SavedViewFilters struct {
	Statuses    []string `json:"statuses,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	DomainRef   string   `json:"domain_ref,omitempty"`
	ProjectRef  string   `json:"project_ref,omitempty"`
	AssigneeRef string   `json:"assignee_ref,omitempty"`
	DueBefore   string   `json:"due_before,omitempty"`
	DueAfter    string   `json:"due_after,omitempty"`
	Search      string   `json:"search,omitempty"`
}

type ClaimRecord struct {
	TaskHandle  string `json:"task_handle"`
	ActorHandle string `json:"actor_handle"`
	Status      string `json:"status"`
}

type RelationshipRecord struct {
	UUID       string `json:"uuid"`
	Type       string `json:"type"`
	SourceTask string `json:"source_task"`
	TargetTask string `json:"target_task"`
	CreatedAt  string `json:"created_at"`
}

type LinkRecord struct {
	UUID      string            `json:"uuid"`
	TaskID    string            `json:"task_id"`
	Type      string            `json:"type"`
	Target    string            `json:"target"`
	Label     string            `json:"label"`
	Metadata  map[string]string `json:"metadata"`
	CreatedAt string            `json:"created_at"`
}

type TaskSessionRecord struct {
	TaskHandle    string `json:"task_handle"`
	State         string `json:"state"`
	ElapsedSecond int64  `json:"elapsed_seconds"`
}

type ManualTimeEntryRecord struct {
	EntryID        string `json:"entry_id"`
	TaskHandle     string `json:"task_handle"`
	DurationSecond int64  `json:"duration_seconds"`
	StartedAt      string `json:"started_at"`
	Note           string `json:"note"`
}

type CreateTaskRequest struct {
	Title       string
	Description string
	Tags        []string
	DomainRef   *string
	ProjectRef  *string
	AssigneeRef *string
	DueAt       *string
}

type ListTasksRequest struct {
	Statuses       []string
	DomainRef      *string
	ProjectRef     *string
	AssigneeRef    *string
	DueBefore      *string
	DueAfter       *string
	Tags           []string
	Search         string
	RepoTarget     *string
	WorktreeTarget *string
}

type ShowTaskRequest struct {
	Reference string
}

type UpdateTaskRequest struct {
	Reference             string
	Title                 *string
	Description           *string
	Tags                  *[]string
	DomainRef             *string
	ProjectRef            *string
	AssigneeRef           *string
	DueAt                 *string
	Status                *string
	AcceptDefaultAssignee bool
	KeepAssignee          bool
}
type ClaimTaskRequest struct {
	Reference string
	Lease     time.Duration
}
type StartTaskSessionRequest struct {
	Reference string
	At        *time.Time
}
type PauseTaskSessionRequest struct {
	Reference string
	At        *time.Time
}
type ResumeTaskSessionRequest struct {
	Reference string
	At        *time.Time
}
type AddManualTimeRequest struct {
	Reference string
	Duration  time.Duration
	StartedAt *time.Time
	Note      string
}
type EditManualTimeRequest struct {
	Reference string
	EntryID   string
	Duration  *time.Duration
	StartedAt *time.Time
	Note      *string
}
type RenewClaimRequest struct {
	Reference string
	Lease     time.Duration
}
type ReleaseClaimRequest struct {
	Reference string
}
type UnlockTaskRequest struct {
	Reference string
}

type CreateRelationshipRequest struct {
	Type          string
	SourceTaskRef string
	TargetTaskRef string
	ActorID       *int64
}

type ListRelationshipsRequest struct {
	TaskRef string
}

type RemoveRelationshipRequest struct {
	Type          string
	SourceTaskRef string
	TargetTaskRef string
	ActorID       *int64
}

type CreateLinkRequest struct {
	TaskRef  string
	Type     string
	Target   string
	Label    string
	Metadata map[string]string
	ActorID  *int64
}

type ListLinksRequest struct {
	TaskRef string
}

type RemoveLinkRequest struct {
	TaskRef string
	LinkRef string
	Type    *string
	Target  *string
	ActorID *int64
}

type AttachCurrentRepoContextRequest struct {
	TaskRef  string
	Context  gitctx.Context
	LinkRepo bool
}

type AttachCurrentRepoContextResult struct {
	RepoLink     *LinkRecord `json:"repo_link"`
	WorktreeLink LinkRecord  `json:"worktree_link"`
}

type CreateProjectRequest struct {
	Name               string
	Description        string
	DomainRef          string
	DefaultAssigneeRef *string
	AssigneeRef        *string
	DueAt              *string
	Tags               []string
}
type ListProjectsRequest struct{}
type ShowProjectRequest struct {
	Reference string
}
type UpdateProjectRequest struct {
	Reference          string
	Name               *string
	Description        *string
	DomainRef          *string
	DefaultAssigneeRef *string
	AssigneeRef        *string
	DueAt              *string
	Tags               *[]string
	Status             *string
}

type CreateDomainRequest struct {
	Name               string
	Description        string
	DefaultAssigneeRef *string
	AssigneeRef        *string
	DueAt              *string
	Tags               []string
}
type ListDomainsRequest struct{}
type ShowDomainRequest struct {
	Reference string
}
type UpdateDomainRequest struct {
	Reference          string
	Name               *string
	Description        *string
	DefaultAssigneeRef *string
	AssigneeRef        *string
	DueAt              *string
	Tags               *[]string
	Status             *string
}

type ListActorsRequest struct{}
type ShowActorRequest struct {
	Reference string
}

type CreateViewRequest struct {
	Name    string
	Filters SavedViewFilters
}

type ListViewsRequest struct{}

type ShowViewRequest struct {
	Name string
}

type UpdateViewRequest struct {
	Name    string
	NewName string
	Filters SavedViewFilters
}

type DeleteViewRequest struct {
	Name string
}
