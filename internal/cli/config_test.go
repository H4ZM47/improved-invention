package cli

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestNewConfigCommandJSONOutput(t *testing.T) {
	t.Parallel()

	opts := &GlobalOptions{
		JSON:   true,
		DBPath: "/tmp/task.db",
		Actor:  "codex:agent-7",
	}

	cmd := newConfigShowCommand(opts)
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var payload struct {
		OK      bool   `json:"ok"`
		Command string `json:"command"`
		Data    struct {
			Config struct {
				DBPath string `json:"db_path"`
				Actor  string `json:"actor"`
			} `json:"config"`
		} `json:"data"`
	}

	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if !payload.OK {
		t.Fatal("payload.OK = false, want true")
	}

	if got, want := payload.Command, "grind config show"; got != want {
		t.Fatalf("Command = %q, want %q", got, want)
	}

	if got, want := payload.Data.Config.DBPath, "/tmp/task.db"; got != want {
		t.Fatalf("DBPath = %q, want %q", got, want)
	}

	if got, want := payload.Data.Config.Actor, "codex:agent-7"; got != want {
		t.Fatalf("Actor = %q, want %q", got, want)
	}
}
