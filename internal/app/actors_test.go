package app

import (
	"context"
	"database/sql"
	"testing"

	"github.com/H4ZM47/improved-invention/internal/testutil"
)

func TestActorManagerBootstrapsHumanAndCreatesAgent(t *testing.T) {
	t.Parallel()

	db := openActorManagerTestDB(t)
	manager := ActorManager{
		DB:        db,
		HumanName: "alex",
	}

	human, err := manager.BootstrapConfiguredHumanActor(context.Background())
	if err != nil {
		t.Fatalf("BootstrapConfiguredHumanActor() error = %v", err)
	}

	if got, want := human.Kind, "human"; got != want {
		t.Fatalf("human.Kind = %q, want %q", got, want)
	}

	agent, err := manager.GetOrCreateAgentActor(context.Background(), "codex:agent-7")
	if err != nil {
		t.Fatalf("GetOrCreateAgentActor() error = %v", err)
	}

	if got, want := agent.Provider, "codex"; got != want {
		t.Fatalf("agent.Provider = %q, want %q", got, want)
	}
}

func openActorManagerTestDB(t *testing.T) *sql.DB {
	t.Helper()
	return testutil.OpenSQLiteDB(t)
}
