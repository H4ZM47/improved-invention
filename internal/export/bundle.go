package export

import "github.com/H4ZM47/task-cli/internal/app"

// DocumentVersion identifies the export document schema version.
//
// Bumping this is a breaking change to consumers; additions should be made
// backward-compatibly whenever possible.
const DocumentVersion = 1

// Bundle is the input to every export serializer.
//
// Callers are responsible for ordering (slices are emitted in the given order)
// and for populating only the entity collections that are in scope for their
// command. Empty slices are represented as empty collections in output, not
// omitted, so structure is predictable for downstream automation.
type Bundle struct {
	Tasks         []app.TaskRecord
	Domains       []app.DomainRecord
	Projects      []app.ProjectRecord
	Actors        []app.ActorRecord
	Links         []app.LinkRecord
	Relationships []app.RelationshipRecord
}
