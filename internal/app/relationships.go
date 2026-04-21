package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"

	taskdb "github.com/H4ZM47/grind/internal/db"
)

// RelationshipManager provides service-layer task relationship workflows.
type RelationshipManager struct {
	DB              *sql.DB
	HumanName       string
	CurrentActorRef string
}

// LinkManager provides service-layer task external-link workflows.
type LinkManager struct {
	DB              *sql.DB
	HumanName       string
	CurrentActorRef string
}

// Create creates a normalized relationship between two tasks.
func (m RelationshipManager) Create(ctx context.Context, req CreateRelationshipRequest) (RelationshipRecord, error) {
	actorID, err := m.currentActorID(ctx)
	if err != nil {
		return RelationshipRecord{}, err
	}

	normalizedType, sourceRef, targetRef, err := normalizeRelationship(req.Type, req.SourceTaskRef, req.TargetTaskRef)
	if err != nil {
		return RelationshipRecord{}, err
	}

	relationship, err := taskdb.CreateRelationship(ctx, m.DB, taskdb.RelationshipCreateInput{
		RelationshipType:    normalizedType,
		SourceTaskReference: sourceRef,
		TargetTaskReference: targetRef,
		ActorID:             actorID,
	})
	if err != nil {
		return RelationshipRecord{}, err
	}

	return toRelationshipRecord(relationship), nil
}

// List lists relationships touching a task.
func (m RelationshipManager) List(ctx context.Context, req ListRelationshipsRequest) ([]RelationshipRecord, error) {
	relationships, err := taskdb.ListRelationshipsForTask(ctx, m.DB, req.TaskRef)
	if err != nil {
		return nil, err
	}

	records := make([]RelationshipRecord, 0, len(relationships))
	for _, relationship := range relationships {
		records = append(records, toRelationshipRecord(relationship))
	}
	return records, nil
}

// Remove removes a normalized relationship between two tasks.
func (m RelationshipManager) Remove(ctx context.Context, req RemoveRelationshipRequest) error {
	actorID, err := m.currentActorID(ctx)
	if err != nil {
		return err
	}

	normalizedType, sourceRef, targetRef, err := normalizeRelationship(req.Type, req.SourceTaskRef, req.TargetTaskRef)
	if err != nil {
		return err
	}

	_, err = taskdb.RemoveRelationship(ctx, m.DB, taskdb.RelationshipRemoveInput{
		RelationshipType:    normalizedType,
		SourceTaskReference: sourceRef,
		TargetTaskReference: targetRef,
		ActorID:             actorID,
	})
	return err
}

// Create stores a task-scoped external link.
func (m LinkManager) Create(ctx context.Context, req CreateLinkRequest) (LinkRecord, error) {
	actorID, err := m.currentActorID(ctx)
	if err != nil {
		return LinkRecord{}, err
	}

	metadataJSON, err := json.Marshal(req.Metadata)
	if err != nil {
		return LinkRecord{}, err
	}

	link, err := taskdb.CreateExternalLink(ctx, m.DB, taskdb.TaskExternalLinkCreateInput{
		TaskReference: req.TaskRef,
		LinkType:      req.Type,
		Target:        req.Target,
		Label:         req.Label,
		MetadataJSON:  string(metadataJSON),
		ActorID:       actorID,
	})
	if err != nil {
		return LinkRecord{}, err
	}

	return toLinkRecord(link), nil
}

// AttachCurrentRepoContext explicitly links the current repo/worktree context to a task.
func (m LinkManager) AttachCurrentRepoContext(ctx context.Context, req AttachCurrentRepoContextRequest) (AttachCurrentRepoContextResult, error) {
	existing, err := m.List(ctx, ListLinksRequest{TaskRef: req.TaskRef})
	if err != nil {
		return AttachCurrentRepoContextResult{}, err
	}

	result := AttachCurrentRepoContextResult{}
	if req.LinkRepo {
		repoTarget := req.Context.RepoTarget()
		if link, ok := findMatchingLink(existing, taskdb.LinkTypeRepo, repoTarget); ok {
			linkCopy := link
			result.RepoLink = &linkCopy
		} else {
			link, err := m.Create(ctx, CreateLinkRequest{
				TaskRef: req.TaskRef,
				Type:    taskdb.LinkTypeRepo,
				Target:  repoTarget,
				Label:   "Current repository",
				Metadata: map[string]string{
					"repo_root":  req.Context.RepoRoot,
					"git_dir":    req.Context.GitDir,
					"remote_url": req.Context.RemoteURL,
				},
			})
			if err != nil {
				return AttachCurrentRepoContextResult{}, err
			}
			result.RepoLink = &link
		}
	}

	worktreeTarget := req.Context.WorktreeTarget()
	if link, ok := findMatchingLink(existing, taskdb.LinkTypeWorktree, worktreeTarget); ok {
		result.WorktreeLink = link
	} else {
		link, err := m.Create(ctx, CreateLinkRequest{
			TaskRef: req.TaskRef,
			Type:    taskdb.LinkTypeWorktree,
			Target:  worktreeTarget,
			Label:   "Current worktree",
			Metadata: map[string]string{
				"repo_root":  req.Context.RepoRoot,
				"git_dir":    req.Context.GitDir,
				"remote_url": req.Context.RemoteURL,
			},
		})
		if err != nil {
			return AttachCurrentRepoContextResult{}, err
		}
		result.WorktreeLink = link
	}

	return result, nil
}

// List lists task-scoped external links.
func (m LinkManager) List(ctx context.Context, req ListLinksRequest) ([]LinkRecord, error) {
	links, err := taskdb.ListExternalLinksForTask(ctx, m.DB, req.TaskRef)
	if err != nil {
		return nil, err
	}

	records := make([]LinkRecord, 0, len(links))
	for _, link := range links {
		records = append(records, toLinkRecord(link))
	}
	return records, nil
}

// Remove removes a task-scoped external link by UUID or target descriptor.
func (m LinkManager) Remove(ctx context.Context, req RemoveLinkRequest) error {
	actorID, err := m.currentActorID(ctx)
	if err != nil {
		return err
	}

	if req.LinkRef == "" {
		return sql.ErrNoRows
	}

	_, err = taskdb.RemoveExternalLink(ctx, m.DB, taskdb.TaskExternalLinkRemoveInput{
		TaskReference: req.TaskRef,
		LinkUUID:      req.LinkRef,
		ActorID:       actorID,
	})
	return err
}

func (m RelationshipManager) currentActorID(ctx context.Context) (*int64, error) {
	return TaskManager(m).resolveCurrentActorID(ctx)
}

func (m LinkManager) currentActorID(ctx context.Context) (*int64, error) {
	return TaskManager(m).resolveCurrentActorID(ctx)
}

func toRelationshipRecord(relationship taskdb.Relationship) RelationshipRecord {
	return RelationshipRecord{
		UUID:       relationship.UUID,
		Type:       relationshipTypeForCLI(relationship.RelationshipType),
		SourceTask: relationship.SourceTaskHandle,
		TargetTask: relationship.TargetTaskHandle,
		CreatedAt:  relationship.CreatedAt,
	}
}

func toLinkRecord(link taskdb.ExternalLink) LinkRecord {
	metadata := map[string]string{}
	if strings.TrimSpace(link.MetadataJSON) != "" {
		_ = json.Unmarshal([]byte(link.MetadataJSON), &metadata)
	}
	return LinkRecord{
		UUID:      link.UUID,
		TaskID:    link.TaskHandle,
		Type:      link.LinkType,
		Target:    link.Target,
		Label:     link.Label,
		Metadata:  metadata,
		CreatedAt: link.CreatedAt,
	}
}

func findMatchingLink(links []LinkRecord, linkType string, target string) (LinkRecord, bool) {
	for _, link := range links {
		if link.Type == linkType && link.Target == target {
			return link, true
		}
	}
	return LinkRecord{}, false
}

func normalizeRelationship(rawType string, sourceTaskRef string, targetTaskRef string) (normalizedType string, normalizedSource string, normalizedTarget string, err error) {
	switch strings.ToLower(strings.TrimSpace(rawType)) {
	case "parent_of", "parent", "child_of", "child":
		normalizedType = "parent_child"
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(rawType)), "child") {
			return normalizedType, targetTaskRef, sourceTaskRef, nil
		}
		return normalizedType, sourceTaskRef, targetTaskRef, nil
	case "blocks", "blocked_by":
		normalizedType = "blocks"
		if strings.EqualFold(strings.TrimSpace(rawType), "blocked_by") {
			return normalizedType, targetTaskRef, sourceTaskRef, nil
		}
		return normalizedType, sourceTaskRef, targetTaskRef, nil
	case "related_to", "related":
		return "related_to", sourceTaskRef, targetTaskRef, nil
	case "sibling_of", "sibling":
		return "sibling_of", sourceTaskRef, targetTaskRef, nil
	case "duplicate_of", "duplicate":
		return "duplicate_of", sourceTaskRef, targetTaskRef, nil
	case "supersedes", "supersede":
		return "supersedes", sourceTaskRef, targetTaskRef, nil
	default:
		return "", "", "", taskdb.ErrInvalidRelationshipType
	}
}

func relationshipTypeForCLI(value string) string {
	switch value {
	case "parent_child":
		return "parent_of"
	case "blocks":
		return "blocks"
	default:
		return value
	}
}
