package app

import (
	"context"
	"database/sql"
	"fmt"

	taskdb "github.com/H4ZM47/improved-invention/internal/db"
)

// ActorManager provides the service-layer actor workflows used by commands.
type ActorManager struct {
	DB        *sql.DB
	HumanName string
}

// BootstrapConfiguredHumanActor ensures the configured local human actor exists.
func (m ActorManager) BootstrapConfiguredHumanActor(ctx context.Context) (ActorRecord, error) {
	actor, err := taskdb.EnsureHumanActor(ctx, m.DB, m.HumanName)
	if err != nil {
		return ActorRecord{}, err
	}
	return toActorRecord(actor), nil
}

// ParseAgentIdentity validates and decomposes a provider-scoped agent reference.
func (m ActorManager) ParseAgentIdentity(raw string) (AgentIdentityRecord, error) {
	identity, err := taskdb.ParseAgentIdentity(raw)
	if err != nil {
		return AgentIdentityRecord{}, err
	}

	return AgentIdentityRecord{
		Provider:   identity.Provider,
		ExternalID: identity.ExternalID,
	}, nil
}

// GetOrCreateAgentActor resolves or creates an agent actor on first use.
func (m ActorManager) GetOrCreateAgentActor(ctx context.Context, raw string) (ActorRecord, error) {
	identity, err := taskdb.ParseAgentIdentity(raw)
	if err != nil {
		return ActorRecord{}, err
	}

	actor, err := taskdb.GetOrCreateAgentActor(ctx, m.DB, identity)
	if err != nil {
		return ActorRecord{}, err
	}
	return toActorRecord(actor), nil
}

// List returns all known actor records.
func (m ActorManager) List(ctx context.Context, _ ListActorsRequest) ([]ActorRecord, error) {
	actors, err := taskdb.ListActors(ctx, m.DB)
	if err != nil {
		return nil, err
	}

	records := make([]ActorRecord, 0, len(actors))
	for _, actor := range actors {
		records = append(records, toActorRecord(actor))
	}
	return records, nil
}

// Show resolves a single actor by handle, uuid, or identity reference.
func (m ActorManager) Show(ctx context.Context, req ShowActorRequest) (ActorRecord, error) {
	actor, err := taskdb.FindActor(ctx, m.DB, req.Reference)
	if err != nil {
		return ActorRecord{}, fmt.Errorf("find actor %q: %w", req.Reference, err)
	}

	return toActorRecord(actor), nil
}

func toActorRecord(actor taskdb.Actor) ActorRecord {
	label := actor.DisplayName
	if label == "" {
		label = actor.ExternalID
	}

	return ActorRecord{
		Handle:      actor.Handle,
		Kind:        actor.Kind,
		Label:       label,
		UUID:        actor.UUID,
		Provider:    actor.Provider,
		ExternalID:  actor.ExternalID,
		DisplayName: actor.DisplayName,
	}
}
