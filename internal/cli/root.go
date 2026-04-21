package cli

import "github.com/spf13/cobra"

// BuildInfo carries binary metadata injected at build time.
type BuildInfo struct {
	Version string
	Commit  string
	Date    string
}

// GlobalOptions are shared across all commands.
type GlobalOptions struct {
	JSON      bool
	NoInput   bool
	DBPath    string
	Actor     string
	Quiet     bool
	Agents    bool
	AgentHelp bool
	Version   bool
	Config    bool
}

// Execute runs the Grind root command.
func Execute(build BuildInfo) error {
	cmd, _ := newRootCommandWithOptions(build)
	return cmd.Execute()
}

// NewRootCommand constructs the root command tree.
func NewRootCommand(build BuildInfo) *cobra.Command {
	cmd, _ := newRootCommandWithOptions(build)
	return cmd
}

func newRootCommandWithOptions(build BuildInfo) (*cobra.Command, *GlobalOptions) {
	opts := &GlobalOptions{}

	cmd := &cobra.Command{
		Use:           "grind",
		Short:         "Local-first task management for humans and AI agents",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if opts.Agents || opts.AgentHelp {
				return writeAgentInstructions(cmd.OutOrStdout(), opts.JSON)
			}
			if opts.Version {
				return writeVersionInfo(cmd.OutOrStdout(), build, opts.JSON)
			}
			if opts.Config {
				return writeConfigInfo(cmd.OutOrStdout(), opts)
			}
			return cmd.Help()
		},
	}

	flags := cmd.PersistentFlags()
	flags.BoolVar(&opts.JSON, "json", false, "Emit machine-readable JSON output")
	flags.BoolVar(&opts.NoInput, "no-input", false, "Disable prompts and require explicit non-interactive behavior")
	flags.StringVar(&opts.DBPath, "db", "", "Override the resolved database path")
	flags.StringVar(&opts.Actor, "actor", "", "Override the acting human or agent reference")
	flags.BoolVar(&opts.Quiet, "quiet", false, "Suppress non-essential human-oriented output")
	flags.BoolVar(&opts.Version, "version", false, "Show build information and exit")
	flags.BoolVar(&opts.Config, "config", false, "Show resolved runtime configuration and exit")
	flags.BoolVar(&opts.Agents, "agents", false, "Show the built-in guide for agent-safe Grind usage")
	flags.BoolVar(&opts.AgentHelp, "agent-help", false, "Alias for --agents")

	cmd.AddCommand(
		newTaskCreateCommand(opts),
		newTaskListCommand(opts),
		newTaskShowCommand(opts),
		newTaskUpdateCommand(opts),
		newClaimCommand(opts),
		newLinkCommand(opts),
		newLinkRepoCommand(opts),
		newTimeCommand(opts),
		newTaskOpenCommand(opts),
		newTaskCloseCommand(opts),
		newTaskCancelCommand(opts),
		newProjectCommand(opts),
		newDomainCommand(opts),
		newMilestoneCommand(opts),
		newActorCommand(opts),
		newViewCommand(opts),
		newExportCommand(opts),
		newServeCommand(opts),
		newBackupCommand(opts),
		newRestoreCommand(opts),
		newRetiredConfigCommand(),
		newRetiredVersionCommand(),
		newRetiredAgentsCommand(),
		newRetiredAgentdocsCommand(),
		newRetiredRenewCommand(),
		newRetiredReleaseCommand(),
		newRetiredUnlockCommand(),
		newRetiredStartCommand(),
		newRetiredPauseCommand(),
		newRetiredResumeCommand(),
		newRetiredReportCommand(),
		newRetiredRelationshipCommand(),
	)

	return cmd, opts
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
