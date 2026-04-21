package db

import (
	"context"
	"database/sql"
	"errors"
	"testing"
)

func TestParseAgentIdentity(t *testing.T) {
	t.Parallel()

	identity, err := ParseAgentIdentity("codex:agent-7")
	if err != nil {
		t.Fatalf("ParseAgentIdentity() error = %v", err)
	}

	if got, want := identity.Provider, "codex"; got != want {
		t.Fatalf("Provider = %q, want %q", got, want)
	}

	if got, want := identity.ExternalID, "agent-7"; got != want {
		t.Fatalf("ExternalID = %q, want %q", got, want)
	}
}

func TestParseAgentIdentityRejectsInvalidValues(t *testing.T) {
	t.Parallel()

	for _, raw := range []string{"", "codex", "codex:", ":agent-7"} {
		if _, err := ParseAgentIdentity(raw); err == nil {
			t.Fatalf("ParseAgentIdentity(%q) error = nil, want invalid identity failure", raw)
		}
	}
}

func TestEnsureHumanActorIsStable(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)

	first, err := EnsureHumanActor(context.Background(), db, "alex")
	if err != nil {
		t.Fatalf("first EnsureHumanActor() error = %v", err)
	}

	second, err := EnsureHumanActor(context.Background(), db, "alex")
	if err != nil {
		t.Fatalf("second EnsureHumanActor() error = %v", err)
	}

	if first.ID != second.ID {
		t.Fatalf("actor IDs differ: %d vs %d", first.ID, second.ID)
	}

	if first.Handle != second.Handle {
		t.Fatalf("actor handles differ: %q vs %q", first.Handle, second.Handle)
	}
}

func TestGetOrCreateAgentActorCreatesOnce(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)

	identity := AgentIdentity{Provider: "codex", ExternalID: "agent-7"}
	first, err := GetOrCreateAgentActor(context.Background(), db, identity)
	if err != nil {
		t.Fatalf("first GetOrCreateAgentActor() error = %v", err)
	}

	second, err := GetOrCreateAgentActor(context.Background(), db, identity)
	if err != nil {
		t.Fatalf("second GetOrCreateAgentActor() error = %v", err)
	}

	if first.ID != second.ID {
		t.Fatalf("actor IDs differ: %d vs %d", first.ID, second.ID)
	}
}

func TestFindActorSupportsHumanAndAgentReferences(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)

	human, err := EnsureHumanActor(context.Background(), db, "alex")
	if err != nil {
		t.Fatalf("EnsureHumanActor() error = %v", err)
	}

	agent, err := GetOrCreateAgentActor(context.Background(), db, AgentIdentity{
		Provider:   "codex",
		ExternalID: "agent-7",
	})
	if err != nil {
		t.Fatalf("GetOrCreateAgentActor() error = %v", err)
	}

	gotHuman, err := FindActor(context.Background(), db, "alex")
	if err != nil {
		t.Fatalf("FindActor(human) error = %v", err)
	}
	if gotHuman.ID != human.ID {
		t.Fatalf("human actor ID = %d, want %d", gotHuman.ID, human.ID)
	}

	gotAgent, err := FindActor(context.Background(), db, "codex:agent-7")
	if err != nil {
		t.Fatalf("FindActor(agent) error = %v", err)
	}
	if gotAgent.ID != agent.ID {
		t.Fatalf("agent actor ID = %d, want %d", gotAgent.ID, agent.ID)
	}

	_, err = FindActor(context.Background(), db, "ACT-404")
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("FindActor(missing) error = %v, want sql.ErrNoRows", err)
	}
}
