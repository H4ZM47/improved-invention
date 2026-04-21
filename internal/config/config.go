package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

const (
	defaultBusyTimeout = 5 * time.Second
	defaultClaimLease  = 24 * time.Hour
)

// Options describes external inputs used during config resolution.
type Options struct {
	DBPathOverride string
	ActorOverride  string
	LookupEnv      func(string) (string, bool)
	UserConfigDir  func() (string, error)
}

// Resolved is the runtime configuration computed for the current process.
type Resolved struct {
	DataDir     string
	DBPath      string
	Actor       string
	BusyTimeout time.Duration
	ClaimLease  time.Duration
	SourceOrder []string
}

// Resolve computes Task CLI runtime configuration from explicit overrides,
// environment variables, and OS-specific defaults.
func Resolve(opts Options) (Resolved, error) {
	lookupEnv := opts.LookupEnv
	if lookupEnv == nil {
		lookupEnv = os.LookupEnv
	}

	userConfigDir := opts.UserConfigDir
	if userConfigDir == nil {
		userConfigDir = os.UserConfigDir
	}

	configDir, err := userConfigDir()
	if err != nil {
		return Resolved{}, fmt.Errorf("resolve user config dir: %w", err)
	}

	dataDir := filepath.Join(configDir, "task")
	if envValue, ok := lookupEnv("TASK_DATA_DIR"); ok && envValue != "" {
		dataDir = envValue
	}

	dbPath := filepath.Join(dataDir, "task.db")
	switch {
	case opts.DBPathOverride != "":
		dbPath = opts.DBPathOverride
	case envString(lookupEnv, "TASK_DB_PATH") != "":
		dbPath = envString(lookupEnv, "TASK_DB_PATH")
	}

	actor := envString(lookupEnv, "TASK_ACTOR")
	if opts.ActorOverride != "" {
		actor = opts.ActorOverride
	}

	busyTimeout := defaultBusyTimeout
	if raw := envString(lookupEnv, "TASK_BUSY_TIMEOUT_MS"); raw != "" {
		ms, err := strconv.Atoi(raw)
		if err != nil || ms <= 0 {
			return Resolved{}, fmt.Errorf("parse TASK_BUSY_TIMEOUT_MS: %q is invalid", raw)
		}
		busyTimeout = time.Duration(ms) * time.Millisecond
	}

	claimLease := defaultClaimLease
	if raw := envString(lookupEnv, "TASK_CLAIM_LEASE_HOURS"); raw != "" {
		hours, err := strconv.Atoi(raw)
		if err != nil || hours <= 0 {
			return Resolved{}, fmt.Errorf("parse TASK_CLAIM_LEASE_HOURS: %q is invalid", raw)
		}
		claimLease = time.Duration(hours) * time.Hour
	}

	return Resolved{
		DataDir:     dataDir,
		DBPath:      dbPath,
		Actor:       actor,
		BusyTimeout: busyTimeout,
		ClaimLease:  claimLease,
		SourceOrder: []string{
			"cli overrides",
			"environment variables",
			"os-specific defaults",
		},
	}, nil
}

func envString(lookup func(string) (string, bool), key string) string {
	value, ok := lookup(key)
	if !ok {
		return ""
	}
	return value
}
