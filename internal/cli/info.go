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

func newRetiredRenewCommand() *cobra.Command {
	return &cobra.Command{
		Use:    "renew <task-ref>",
		Short:  "Retired: use grind claim renew",
		Args:   cobra.ExactArgs(1),
		Hidden: true,
		RunE: func(_ *cobra.Command, args []string) error {
			return fmt.Errorf("`grind renew %s` was removed; use `grind claim renew %s`", args[0], args[0])
		},
	}
}

func newRetiredReleaseCommand() *cobra.Command {
	return &cobra.Command{
		Use:    "release <task-ref>",
		Short:  "Retired: use grind claim release",
		Args:   cobra.ExactArgs(1),
		Hidden: true,
		RunE: func(_ *cobra.Command, args []string) error {
			return fmt.Errorf("`grind release %s` was removed; use `grind claim release %s`", args[0], args[0])
		},
	}
}

func newRetiredUnlockCommand() *cobra.Command {
	return &cobra.Command{
		Use:    "unlock <task-ref>",
		Short:  "Retired: use grind claim unlock",
		Args:   cobra.ExactArgs(1),
		Hidden: true,
		RunE: func(_ *cobra.Command, args []string) error {
			return fmt.Errorf("`grind unlock %s` was removed; use `grind claim unlock %s`", args[0], args[0])
		},
	}
}

func newRetiredStartCommand() *cobra.Command {
	return &cobra.Command{
		Use:    "start <task-ref>",
		Short:  "Retired: use grind time start",
		Args:   cobra.ExactArgs(1),
		Hidden: true,
		RunE: func(_ *cobra.Command, args []string) error {
			return fmt.Errorf("`grind start %s` was removed; use `grind time start %s`", args[0], args[0])
		},
	}
}

func newRetiredPauseCommand() *cobra.Command {
	return &cobra.Command{
		Use:    "pause <task-ref>",
		Short:  "Retired: use grind time pause",
		Args:   cobra.ExactArgs(1),
		Hidden: true,
		RunE: func(_ *cobra.Command, args []string) error {
			return fmt.Errorf("`grind pause %s` was removed; use `grind time pause %s`", args[0], args[0])
		},
	}
}

func newRetiredResumeCommand() *cobra.Command {
	return &cobra.Command{
		Use:    "resume <task-ref>",
		Short:  "Retired: use grind time resume",
		Args:   cobra.ExactArgs(1),
		Hidden: true,
		RunE: func(_ *cobra.Command, args []string) error {
			return fmt.Errorf("`grind resume %s` was removed; use `grind time resume %s`", args[0], args[0])
		},
	}
}

func newRetiredReportCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "report",
		Short:  "Retired: use grind serve",
		Hidden: true,
		RunE: func(_ *cobra.Command, _ []string) error {
			return fmt.Errorf("`grind report serve` was removed; use `grind serve`")
		},
	}
	cmd.AddCommand(&cobra.Command{
		Use:    "serve",
		Short:  "Retired: use grind serve",
		Args:   cobra.NoArgs,
		Hidden: true,
		RunE: func(_ *cobra.Command, _ []string) error {
			return fmt.Errorf("`grind report serve` was removed; use `grind serve`")
		},
	})
	return cmd
}

func newRetiredRelationshipCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "relationship",
		Short:  "Retired: use grind link",
		Hidden: true,
		RunE: func(_ *cobra.Command, _ []string) error {
			return fmt.Errorf("`grind relationship` was removed; use `grind link`")
		},
	}
	cmd.AddCommand(
		&cobra.Command{
			Use:    "add",
			Short:  "Retired: use grind link add",
			Hidden: true,
			RunE: func(_ *cobra.Command, _ []string) error {
				return fmt.Errorf("`grind relationship add` was removed; use `grind link add`")
			},
		},
		&cobra.Command{
			Use:    "list",
			Short:  "Retired: use grind link list",
			Hidden: true,
			RunE: func(_ *cobra.Command, _ []string) error {
				return fmt.Errorf("`grind relationship list` was removed; use `grind link list`")
			},
		},
		&cobra.Command{
			Use:    "remove",
			Short:  "Retired: use grind link remove",
			Hidden: true,
			RunE: func(_ *cobra.Command, _ []string) error {
				return fmt.Errorf("`grind relationship remove` was removed; use `grind link remove`")
			},
		},
	)
	return cmd
}
