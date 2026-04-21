package app

import (
	"context"
	"time"
)

// Services groups the service-layer interfaces that CLI command handlers call.
type Services interface {
	Tasks() TaskService
	Projects() ProjectService
	Domains() DomainService
	Actors() ActorService
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
	Name string `json:"name"`
}

type ClaimRecord struct {
	TaskHandle  string `json:"task_handle"`
	ActorHandle string `json:"actor_handle"`
	Status      string `json:"status"`
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

type ListTasksRequest struct{}

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

type CreateViewRequest struct{}
type ListViewsRequest struct{}
type ShowViewRequest struct{}
type UpdateViewRequest struct{}
type DeleteViewRequest struct{}
