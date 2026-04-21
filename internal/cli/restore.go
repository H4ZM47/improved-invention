package cli

import (
	"fmt"

	taskconfig "github.com/H4ZM47/grind/internal/config"
	taskdb "github.com/H4ZM47/grind/internal/db"
	"github.com/spf13/cobra"
)

func newRestoreCommand(opts *GlobalOptions) *cobra.Command {
	cmd := newGroupCommand("restore", "Restore from a full-fidelity backup")
	cmd.AddCommand(newRestoreApplyCommand(opts))
	return cmd
}

func newRestoreApplyCommand(opts *GlobalOptions) *cobra.Command {
	var input string
	var force bool

	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Restore a portable full-fidelity backup artifact",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if input == "" {
				return fmt.Errorf("restore input path is required")
			}

			cfg, err := taskconfig.Resolve(taskconfig.Options{
				DBPathOverride: opts.DBPath,
				ActorOverride:  opts.Actor,
			})
			if err != nil {
				return err
			}

			if err := taskdb.RestoreDatabase(cmd.Context(), input, cfg, force); err != nil {
				return err
			}

			if opts.JSON {
				return writeJSON(cmd, map[string]any{
					"ok":      true,
					"command": "grind restore apply",
					"data": map[string]any{
						"input_path": input,
						"db_path":    cfg.DBPath,
					},
					"meta": map[string]any{
						"force": force,
					},
				})
			}

			_, err = fmt.Fprintf(cmd.OutOrStdout(), "Restored full-fidelity backup from %s into %s\n", input, cfg.DBPath)
			return err
		},
	}

	cmd.Flags().StringVarP(&input, "input", "i", "", "Read the backup artifact from this path")
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite the existing target database if it exists")
	return cmd
}
