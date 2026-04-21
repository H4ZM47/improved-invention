package config

import (
	"fmt"
	"os"
	"os/user"
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
	CurrentUser    func() (*user.User, error)
}

// Resolved is the runtime configuration computed for the current process.
type Resolved struct {
	DataDir     string
	DBPath      string
	Actor       string
	HumanName   string
	BusyTimeout time.Duration
	ClaimLease  time.Duration
	SourceOrder []string
}

// Resolve computes Grind runtime configuration from explicit overrides,
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

	currentUser := opts.CurrentUser
	if currentUser == nil {
		currentUser = user.Current
	}

	configDir, err := userConfigDir()
	if err != nil {
		return Resolved{}, fmt.Errorf("resolve user config dir: %w", err)
	}

	dataDir := filepath.Join(configDir, "grind")
	if envValue := envStringMulti(lookupEnv, "GRIND_DATA_DIR", "TASK_DATA_DIR"); envValue != "" {
		dataDir = envValue
	}

	dbPath := filepath.Join(dataDir, "grind.db")
	switch {
	case opts.DBPathOverride != "":
		dbPath = opts.DBPathOverride
	case envStringMulti(lookupEnv, "GRIND_DB_PATH", "TASK_DB_PATH") != "":
		dbPath = envStringMulti(lookupEnv, "GRIND_DB_PATH", "TASK_DB_PATH")
	}

	actor := envStringMulti(lookupEnv, "GRIND_ACTOR", "TASK_ACTOR")
	if opts.ActorOverride != "" {
		actor = opts.ActorOverride
	}

	humanName := envStringMulti(lookupEnv, "GRIND_HUMAN_NAME", "TASK_HUMAN_NAME")
	if humanName == "" {
		current, err := currentUser()
		if err == nil && current != nil && current.Username != "" {
			humanName = current.Username
		}
	}
	if humanName == "" {
		humanName = "local-human"
	}

	busyTimeout := defaultBusyTimeout
	if raw := envStringMulti(lookupEnv, "GRIND_BUSY_TIMEOUT_MS", "TASK_BUSY_TIMEOUT_MS"); raw != "" {
		ms, err := strconv.Atoi(raw)
		if err != nil || ms <= 0 {
			return Resolved{}, fmt.Errorf("parse GRIND_BUSY_TIMEOUT_MS: %q is invalid", raw)
		}
		busyTimeout = time.Duration(ms) * time.Millisecond
	}

	claimLease := defaultClaimLease
	if raw := envStringMulti(lookupEnv, "GRIND_CLAIM_LEASE_HOURS", "TASK_CLAIM_LEASE_HOURS"); raw != "" {
		hours, err := strconv.Atoi(raw)
		if err != nil || hours <= 0 {
			return Resolved{}, fmt.Errorf("parse GRIND_CLAIM_LEASE_HOURS: %q is invalid", raw)
		}
		claimLease = time.Duration(hours) * time.Hour
	}

	return Resolved{
		DataDir:     dataDir,
		DBPath:      dbPath,
		Actor:       actor,
		HumanName:   humanName,
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

func envStringMulti(lookup func(string) (string, bool), keys ...string) string {
	for _, key := range keys {
		if value := envString(lookup, key); value != "" {
			return value
		}
	}
	return ""
}
