package app

import "context"

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
	Handle string
	Title  string
	Status string
}

type ProjectRecord struct {
	Handle string
	Name   string
	Status string
}

type DomainRecord struct {
	Handle string
	Name   string
	Status string
}

type ActorRecord struct {
	Handle string
	Kind   string
	Label  string
}

type ViewRecord struct {
	Name string
}

type ClaimRecord struct {
	TaskHandle  string
	ActorHandle string
	Status      string
}

type CreateTaskRequest struct{}
type ListTasksRequest struct{}
type ShowTaskRequest struct{}
type UpdateTaskRequest struct{}
type ClaimTaskRequest struct{}
type RenewClaimRequest struct{}
type ReleaseClaimRequest struct{}
type UnlockTaskRequest struct{}

type CreateProjectRequest struct{}
type ListProjectsRequest struct{}
type ShowProjectRequest struct{}
type UpdateProjectRequest struct{}

type CreateDomainRequest struct{}
type ListDomainsRequest struct{}
type ShowDomainRequest struct{}
type UpdateDomainRequest struct{}

type ListActorsRequest struct{}
type ShowActorRequest struct{}

type CreateViewRequest struct{}
type ListViewsRequest struct{}
type ShowViewRequest struct{}
type UpdateViewRequest struct{}
type DeleteViewRequest struct{}
