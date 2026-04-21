package config

import (
	"path/filepath"
	"testing"
	"time"
)

func TestResolveDefaults(t *testing.T) {
	t.Parallel()

	cfg, err := Resolve(Options{
		LookupEnv: func(string) (string, bool) {
			return "", false
		},
		UserConfigDir: func() (string, error) {
			return "/tmp/task-home", nil
		},
	})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	if got, want := cfg.DataDir, "/tmp/task-home/task"; got != want {
		t.Fatalf("DataDir = %q, want %q", got, want)
	}

	if got, want := cfg.DBPath, filepath.Join("/tmp/task-home/task", "task.db"); got != want {
		t.Fatalf("DBPath = %q, want %q", got, want)
	}

	if got, want := cfg.BusyTimeout, 5*time.Second; got != want {
		t.Fatalf("BusyTimeout = %v, want %v", got, want)
	}

	if got, want := cfg.ClaimLease, 24*time.Hour; got != want {
		t.Fatalf("ClaimLease = %v, want %v", got, want)
	}
}

func TestResolvePrefersOverrides(t *testing.T) {
	t.Parallel()

	cfg, err := Resolve(Options{
		DBPathOverride: "/tmp/custom.db",
		ActorOverride:  "codex:agent-1",
		LookupEnv: func(key string) (string, bool) {
			switch key {
			case "TASK_DB_PATH":
				return "/tmp/env.db", true
			case "TASK_ACTOR":
				return "codex:env-agent", true
			default:
				return "", false
			}
		},
		UserConfigDir: func() (string, error) {
			return "/tmp/task-home", nil
		},
	})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	if got, want := cfg.DBPath, "/tmp/custom.db"; got != want {
		t.Fatalf("DBPath = %q, want %q", got, want)
	}

	if got, want := cfg.Actor, "codex:agent-1"; got != want {
		t.Fatalf("Actor = %q, want %q", got, want)
	}
}

func TestResolveParsesEnvironmentRuntimeSettings(t *testing.T) {
	t.Parallel()

	cfg, err := Resolve(Options{
		LookupEnv: func(key string) (string, bool) {
			switch key {
			case "TASK_DATA_DIR":
				return "/tmp/task-data", true
			case "TASK_BUSY_TIMEOUT_MS":
				return "9000", true
			case "TASK_CLAIM_LEASE_HOURS":
				return "12", true
			default:
				return "", false
			}
		},
		UserConfigDir: func() (string, error) {
			return "/tmp/task-home", nil
		},
	})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	if got, want := cfg.DataDir, "/tmp/task-data"; got != want {
		t.Fatalf("DataDir = %q, want %q", got, want)
	}

	if got, want := cfg.DBPath, "/tmp/task-data/task.db"; got != want {
		t.Fatalf("DBPath = %q, want %q", got, want)
	}

	if got, want := cfg.BusyTimeout, 9*time.Second; got != want {
		t.Fatalf("BusyTimeout = %v, want %v", got, want)
	}

	if got, want := cfg.ClaimLease, 12*time.Hour; got != want {
		t.Fatalf("ClaimLease = %v, want %v", got, want)
	}
}

func TestResolveRejectsInvalidBusyTimeout(t *testing.T) {
	t.Parallel()

	_, err := Resolve(Options{
		LookupEnv: func(key string) (string, bool) {
			if key == "TASK_BUSY_TIMEOUT_MS" {
				return "zero", true
			}
			return "", false
		},
		UserConfigDir: func() (string, error) {
			return "/tmp/task-home", nil
		},
	})
	if err == nil {
		t.Fatal("Resolve() error = nil, want parse failure")
	}
}
