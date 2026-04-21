package cli

import (
	"fmt"
	"io"

	taskconfig "github.com/H4ZM47/grind/internal/config"
	"github.com/spf13/cobra"
)

func writeVersionInfo(out io.Writer, build BuildInfo, asJSON bool) error {
	if asJSON {
		return writeJSONTo(out, map[string]any{
			"ok":      true,
			"command": "grind --version",
			"data": map[string]string{
				"version": build.Version,
				"commit":  build.Commit,
				"date":    build.Date,
			},
			"meta": map[string]any{},
		})
	}

	_, err := fmt.Fprintf(out, "version=%s commit=%s date=%s\n", build.Version, build.Commit, build.Date)
	return err
}

func writeConfigInfo(out io.Writer, opts *GlobalOptions) error {
	cfg, err := taskconfig.Resolve(taskconfig.Options{
		DBPathOverride: opts.DBPath,
		ActorOverride:  opts.Actor,
	})
	if err != nil {
		return err
	}

	effectiveActor := cfg.Actor
	if effectiveActor == "" {
		effectiveActor = cfg.HumanName
	}

	if opts.JSON {
		return writeJSONTo(out, map[string]any{
			"ok":      true,
			"command": "grind --config",
			"data": map[string]any{
				"config": map[string]any{
					"data_dir":        cfg.DataDir,
					"db_path":         cfg.DBPath,
					"actor":           cfg.Actor,
					"effective_actor": effectiveActor,
					"human_name":      cfg.HumanName,
					"busy_timeout_ms": cfg.BusyTimeout.Milliseconds(),
					"claim_lease_h":   int64(cfg.ClaimLease.Hours()),
					"source_order":    cfg.SourceOrder,
				},
			},
			"meta": map[string]any{},
		})
	}

	_, err = fmt.Fprintf(
		out,
		"data_dir=%s\ndb_path=%s\nactor=%s\neffective_actor=%s\nhuman_name=%s\nbusy_timeout_ms=%d\nclaim_lease_h=%d\n",
		cfg.DataDir,
		cfg.DBPath,
		cfg.Actor,
		effectiveActor,
		cfg.HumanName,
		cfg.BusyTimeout.Milliseconds(),
		int64(cfg.ClaimLease.Hours()),
	)
	return err
}

func newRetiredVersionCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "version",
		Short:  "Retired: use --version",
		Args:   cobra.NoArgs,
		Hidden: true,
		RunE: func(_ *cobra.Command, _ []string) error {
			return fmt.Errorf("`grind version` was removed; use `grind --version`")
		},
	}
	return cmd
}

func newRetiredConfigCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "config",
		Short:  "Retired: use --config",
		Hidden: true,
		RunE: func(_ *cobra.Command, _ []string) error {
			return fmt.Errorf("`grind config show` was removed; use `grind --config`")
		},
	}
	cmd.AddCommand(&cobra.Command{
		Use:    "show",
		Short:  "Retired: use --config",
		Args:   cobra.NoArgs,
		Hidden: true,
		RunE: func(_ *cobra.Command, _ []string) error {
			return fmt.Errorf("`grind config show` was removed; use `grind --config`")
		},
	})
	return cmd
}

func newRetiredAgentsCommand() *cobra.Command {
	return &cobra.Command{
		Use:    "agents",
		Short:  "Retired: use --agents",
		Hidden: true,
		RunE: func(_ *cobra.Command, _ []string) error {
			return fmt.Errorf("`grind agents` was removed; use `grind --agents`")
		},
	}
}

func newRetiredAgentdocsCommand() *cobra.Command {
	return &cobra.Command{
		Use:    "agentdocs",
		Short:  "Retired: use --agents",
		Hidden: true,
		RunE: func(_ *cobra.Command, _ []string) error {
			return fmt.Errorf("`grind agentdocs` was removed; use `grind --agents`")
		},
	}
}
