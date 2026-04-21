package cli

import (
	"encoding/json"
	"fmt"

	taskconfig "github.com/H4ZM47/task-cli/internal/config"
	"github.com/spf13/cobra"
)

func newConfigCommand(opts *GlobalOptions) *cobra.Command {
	cmd := newGroupCommand("config", "Inspect local configuration")
	cmd.AddCommand(newConfigShowCommand(opts))
	return cmd
}

func newConfigShowCommand(opts *GlobalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Show resolved runtime configuration",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := taskconfig.Resolve(taskconfig.Options{
				DBPathOverride: opts.DBPath,
				ActorOverride:  opts.Actor,
			})
			if err != nil {
				return err
			}

			if opts.JSON {
				payload := map[string]any{
					"ok":      true,
					"command": "task config show",
					"data": map[string]any{
						"config": map[string]any{
							"data_dir":        cfg.DataDir,
							"db_path":         cfg.DBPath,
							"actor":           cfg.Actor,
							"human_name":      cfg.HumanName,
							"busy_timeout_ms": cfg.BusyTimeout.Milliseconds(),
							"claim_lease_h":   int64(cfg.ClaimLease.Hours()),
							"source_order":    cfg.SourceOrder,
						},
					},
					"meta": map[string]any{},
				}

				encoder := json.NewEncoder(cmd.OutOrStdout())
				encoder.SetIndent("", "  ")
				return encoder.Encode(payload)
			}

			_, err = fmt.Fprintf(
				cmd.OutOrStdout(),
				"data_dir=%s\ndb_path=%s\nactor=%s\nhuman_name=%s\nbusy_timeout_ms=%d\nclaim_lease_h=%d\n",
				cfg.DataDir,
				cfg.DBPath,
				cfg.Actor,
				cfg.HumanName,
				cfg.BusyTimeout.Milliseconds(),
				int64(cfg.ClaimLease.Hours()),
			)
			return err
		},
	}
}
