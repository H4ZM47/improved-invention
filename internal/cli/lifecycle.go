package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

type lifecycleStatusResult struct {
	DataKey string
	Data    any
	Line    string
}

type lifecycleStatusCommandConfig struct {
	Use             string
	Short           string
	RefName         string
	CommandName     string
	StatusValue     string
	RetiredFlagHelp string
	MigrationError  func(use, reference, status string) error
	Run             func(cmd *cobra.Command, reference string, status string) (lifecycleStatusResult, error)
}

func newLifecycleStatusCommand(opts *GlobalOptions, cfg lifecycleStatusCommandConfig) *cobra.Command {
	var retiredStatus string

	cmd := &cobra.Command{
		Use:   cfg.Use + " <" + cfg.RefName + ">",
		Short: cfg.Short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().Changed("status") {
				return cfg.MigrationError(cfg.Use, args[0], retiredStatus)
			}

			result, err := cfg.Run(cmd, args[0], cfg.StatusValue)
			if err != nil {
				return err
			}

			if result.DataKey == "" {
				return fmt.Errorf("missing lifecycle result data key")
			}

			if opts.JSON {
				return writeJSON(cmd, map[string]any{
					"ok":      true,
					"command": cfg.CommandName,
					"data": map[string]any{
						result.DataKey: result.Data,
					},
					"meta": map[string]any{},
				})
			}

			_, err = fmt.Fprint(cmd.OutOrStdout(), result.Line)
			return err
		},
	}

	cmd.Flags().StringVar(&retiredStatus, "status", "", cfg.RetiredFlagHelp)
	_ = cmd.Flags().MarkHidden("status")
	return cmd
}
