package cli

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestActorListJSONBootstrapsHumanActor(t *testing.T) {
	t.Parallel()

	opts := &GlobalOptions{
		JSON:   true,
		DBPath: t.TempDir() + "/task.db",
	}

	cmd := newActorListCommand(opts)
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var payload struct {
		OK   bool `json:"ok"`
		Data struct {
			Items []struct {
				Kind string `json:"kind"`
			} `json:"items"`
		} `json:"data"`
	}

	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if !payload.OK {
		t.Fatal("payload.OK = false, want true")
	}

	if len(payload.Data.Items) == 0 {
		t.Fatal("actor list returned no items, want configured human actor")
	}

	if got, want := payload.Data.Items[0].Kind, "human"; got != want {
		t.Fatalf("first actor kind = %q, want %q", got, want)
	}
}
