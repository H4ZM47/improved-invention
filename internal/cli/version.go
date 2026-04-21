package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

func newVersionCommand(build BuildInfo, opts *GlobalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show build information",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if opts.JSON {
				payload := map[string]any{
					"ok":      true,
					"command": "task version",
					"data": map[string]string{
						"version": build.Version,
						"commit":  build.Commit,
						"date":    build.Date,
					},
					"meta": map[string]any{},
				}

				encoder := json.NewEncoder(cmd.OutOrStdout())
				encoder.SetIndent("", "  ")
				return encoder.Encode(payload)
			}

			_, err := fmt.Fprintf(
				cmd.OutOrStdout(),
				"version=%s commit=%s date=%s\n",
				build.Version,
				build.Commit,
				build.Date,
			)
			return err
		},
	}
}
