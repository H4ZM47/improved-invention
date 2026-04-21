package cli

import (
	"context"
	"database/sql"
	"fmt"

	taskconfig "github.com/H4ZM47/task-cli/internal/config"
	taskdb "github.com/H4ZM47/task-cli/internal/db"
	"github.com/spf13/cobra"
)

func newBackupCommand(opts *GlobalOptions) *cobra.Command {
	cmd := newGroupCommand("backup", "Create full-fidelity backups")
	cmd.AddCommand(newBackupCreateCommand(opts))
	return cmd
}

func newBackupCreateCommand(opts *GlobalOptions) *cobra.Command {
	var output string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a portable full-fidelity backup artifact",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if output == "" {
				return fmt.Errorf("backup output path is required")
			}

			cfg, db, err := openBackupDB(cmd.Context(), opts)
			if err != nil {
				return err
			}
			defer db.Close()

			if err := taskdb.BackupDatabase(cmd.Context(), db, output); err != nil {
				return err
			}

			if opts.JSON {
				return writeJSON(cmd, map[string]any{
					"ok":      true,
					"command": "task backup create",
					"data": map[string]any{
						"output_path": output,
					},
					"meta": map[string]any{
						"db_path": cfg.DBPath,
					},
				})
			}

			_, err = fmt.Fprintf(cmd.OutOrStdout(), "Created full-fidelity backup at %s\n", output)
			return err
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", "", "Write the backup artifact to this path")
	return cmd
}

func openBackupDB(ctx context.Context, opts *GlobalOptions) (taskconfig.Resolved, *sql.DB, error) {
	cfg, err := taskconfig.Resolve(taskconfig.Options{
		DBPathOverride: opts.DBPath,
		ActorOverride:  opts.Actor,
	})
	if err != nil {
		return taskconfig.Resolved{}, nil, err
	}
	db, err := taskdb.Open(ctx, cfg)
	if err != nil {
		return taskconfig.Resolved{}, nil, err
	}
	return cfg, db, nil
}
