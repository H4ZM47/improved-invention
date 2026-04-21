package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newVersionCommand(build BuildInfo) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show build information",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, _ []string) {
			fmt.Fprintf(
				cmd.OutOrStdout(),
				"version=%s commit=%s date=%s\n",
				build.Version,
				build.Commit,
				build.Date,
			)
		},
	}
}
