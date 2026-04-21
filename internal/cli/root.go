package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// BuildInfo carries binary metadata injected at build time.
type BuildInfo struct {
	Version string
	Commit  string
	Date    string
}

// GlobalOptions are shared across all commands.
type GlobalOptions struct {
	JSON    bool
	NoInput bool
	DBPath  string
	Actor   string
	Quiet   bool
}

// Execute runs the Task CLI root command.
func Execute(build BuildInfo) error {
	return NewRootCommand(build).Execute()
}

// NewRootCommand constructs the root command tree.
func NewRootCommand(build BuildInfo) *cobra.Command {
	opts := &GlobalOptions{}

	cmd := &cobra.Command{
		Use:           "task",
		Short:         "Local-first task management for humans and AI agents",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	flags := cmd.PersistentFlags()
	flags.BoolVar(&opts.JSON, "json", false, "Emit machine-readable JSON output")
	flags.BoolVar(&opts.NoInput, "no-input", false, "Disable prompts and require explicit non-interactive behavior")
	flags.StringVar(&opts.DBPath, "db", "", "Override the resolved database path")
	flags.StringVar(&opts.Actor, "actor", "", "Override the acting human or agent reference")
	flags.BoolVar(&opts.Quiet, "quiet", false, "Suppress non-essential human-oriented output")

	cmd.AddCommand(
		newTaskCreateCommand(opts),
		newTaskListCommand(opts),
		newTaskShowCommand(opts),
		newTaskUpdateCommand(opts),
		newTaskClaimCommand(opts),
		newTaskRenewCommand(opts),
		newTaskReleaseCommand(opts),
		newTaskUnlockCommand(opts),
		newStubCommand("start", "Start task time tracking"),
		newStubCommand("pause", "Pause task time tracking"),
		newStubCommand("resume", "Resume task time tracking"),
		newTaskCloseCommand(opts),
		newProjectCommand(opts),
		newDomainCommand(opts),
		newActorCommand(opts),
		newGroupCommand("view", "Manage saved views"),
		newGroupCommand("export", "Export task data"),
		newGroupCommand("report", "Serve read-only reports"),
		newGroupCommand("backup", "Create full-fidelity backups"),
		newGroupCommand("restore", "Restore from a full-fidelity backup"),
		newConfigCommand(opts),
		newVersionCommand(build, opts),
	)

	return cmd
}

func newGroupCommand(use string, short string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	return cmd
}

func newStubCommand(use string, short string) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return fmt.Errorf("%s is not implemented yet", cmd.CommandPath())
		},
	}
}
