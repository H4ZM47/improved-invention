package app

import (
	"context"
	"database/sql"

	taskdb "github.com/H4ZM47/improved-invention/internal/db"
)

// DomainManager provides service-layer domain workflows used by commands.
type DomainManager struct {
	DB              *sql.DB
	HumanName       string
	CurrentActorRef string
}

// Create inserts a new domain.
func (m DomainManager) Create(ctx context.Context, req CreateDomainRequest) (DomainRecord, error) {
	actorID, err := m.currentActorID(ctx)
	if err != nil {
		return DomainRecord{}, err
	}

	domain, err := taskdb.CreateDomain(ctx, m.DB, taskdb.DomainCreateInput{
		Name:               req.Name,
		Description:        req.Description,
		DefaultAssigneeRef: req.DefaultAssigneeRef,
		AssigneeRef:        req.AssigneeRef,
		DueAt:              req.DueAt,
		Tags:               req.Tags,
		ActorID:            actorID,
	})
	if err != nil {
		return DomainRecord{}, err
	}

	return toDomainRecord(domain), nil
}

// List returns the current domain set.
func (m DomainManager) List(ctx context.Context, _ ListDomainsRequest) ([]DomainRecord, error) {
	domains, err := taskdb.ListDomains(ctx, m.DB)
	if err != nil {
		return nil, err
	}

	records := make([]DomainRecord, 0, len(domains))
	for _, domain := range domains {
		records = append(records, toDomainRecord(domain))
	}
	return records, nil
}

// Show resolves one domain by handle or UUID.
func (m DomainManager) Show(ctx context.Context, req ShowDomainRequest) (DomainRecord, error) {
	domain, err := taskdb.FindDomain(ctx, m.DB, req.Reference)
	if err != nil {
		return DomainRecord{}, err
	}
	return toDomainRecord(domain), nil
}

// Update mutates a domain.
func (m DomainManager) Update(ctx context.Context, req UpdateDomainRequest) (DomainRecord, error) {
	actorID, err := m.currentActorID(ctx)
	if err != nil {
		return DomainRecord{}, err
	}

	domain, err := taskdb.UpdateDomain(ctx, m.DB, taskdb.DomainUpdateInput{
		Reference:          req.Reference,
		Name:               req.Name,
		Description:        req.Description,
		DefaultAssigneeRef: req.DefaultAssigneeRef,
		AssigneeRef:        req.AssigneeRef,
		DueAt:              req.DueAt,
		Tags:               req.Tags,
		Status:             req.Status,
		ActorID:            actorID,
	})
	if err != nil {
		return DomainRecord{}, err
	}

	return toDomainRecord(domain), nil
}

// ProjectManager provides service-layer project workflows used by commands.
type ProjectManager struct {
	DB              *sql.DB
	HumanName       string
	CurrentActorRef string
}

// Create inserts a new project.
func (m ProjectManager) Create(ctx context.Context, req CreateProjectRequest) (ProjectRecord, error) {
	actorID, err := m.currentActorID(ctx)
	if err != nil {
		return ProjectRecord{}, err
	}

	project, err := taskdb.CreateProject(ctx, m.DB, taskdb.ProjectCreateInput{
		Name:               req.Name,
		Description:        req.Description,
		DomainRef:          req.DomainRef,
		DefaultAssigneeRef: req.DefaultAssigneeRef,
		AssigneeRef:        req.AssigneeRef,
		DueAt:              req.DueAt,
		Tags:               req.Tags,
		ActorID:            actorID,
	})
	if err != nil {
		return ProjectRecord{}, err
	}

	return toProjectRecord(project), nil
}

// List returns the current project set.
func (m ProjectManager) List(ctx context.Context, _ ListProjectsRequest) ([]ProjectRecord, error) {
	projects, err := taskdb.ListProjects(ctx, m.DB)
	if err != nil {
		return nil, err
	}

	records := make([]ProjectRecord, 0, len(projects))
	for _, project := range projects {
		records = append(records, toProjectRecord(project))
	}
	return records, nil
}

// Show resolves one project by handle or UUID.
func (m ProjectManager) Show(ctx context.Context, req ShowProjectRequest) (ProjectRecord, error) {
	project, err := taskdb.FindProject(ctx, m.DB, req.Reference)
	if err != nil {
		return ProjectRecord{}, err
	}
	return toProjectRecord(project), nil
}

// Update mutates a project.
func (m ProjectManager) Update(ctx context.Context, req UpdateProjectRequest) (ProjectRecord, error) {
	actorID, err := m.currentActorID(ctx)
	if err != nil {
		return ProjectRecord{}, err
	}

	project, err := taskdb.UpdateProject(ctx, m.DB, taskdb.ProjectUpdateInput{
		Reference:          req.Reference,
		Name:               req.Name,
		Description:        req.Description,
		DomainRef:          req.DomainRef,
		DefaultAssigneeRef: req.DefaultAssigneeRef,
		AssigneeRef:        req.AssigneeRef,
		DueAt:              req.DueAt,
		Tags:               req.Tags,
		Status:             req.Status,
		ActorID:            actorID,
	})
	if err != nil {
		return ProjectRecord{}, err
	}

	return toProjectRecord(project), nil
}

func (m DomainManager) currentActorID(ctx context.Context) (*int64, error) {
	return TaskManager(m).resolveCurrentActorID(ctx)
}

func (m ProjectManager) currentActorID(ctx context.Context) (*int64, error) {
	return TaskManager(m).resolveCurrentActorID(ctx)
}

func toDomainRecord(domain taskdb.Domain) DomainRecord {
	return DomainRecord{
		Handle:                 domain.Handle,
		UUID:                   domain.UUID,
		Name:                   domain.Name,
		Description:            domain.Description,
		Status:                 domain.Status,
		DefaultAssigneeActorID: domain.DefaultAssigneeUUID,
		AssigneeActorID:        domain.AssigneeUUID,
		DueAt:                  domain.DueAt,
		Tags:                   domain.Tags,
		CreatedAt:              domain.CreatedAt,
		UpdatedAt:              domain.UpdatedAt,
		ClosedAt:               domain.ClosedAt,
	}
}

func toProjectRecord(project taskdb.Project) ProjectRecord {
	return ProjectRecord{
		Handle:                 project.Handle,
		UUID:                   project.UUID,
		Name:                   project.Name,
		Description:            project.Description,
		Status:                 project.Status,
		DomainID:               project.DomainUUID,
		DefaultAssigneeActorID: project.DefaultAssigneeUUID,
		AssigneeActorID:        project.AssigneeUUID,
		DueAt:                  project.DueAt,
		Tags:                   project.Tags,
		CreatedAt:              project.CreatedAt,
		UpdatedAt:              project.UpdatedAt,
		ClosedAt:               project.ClosedAt,
	}
}
