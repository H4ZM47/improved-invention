package cli

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/H4ZM47/grind/internal/app"
	taskconfig "github.com/H4ZM47/grind/internal/config"
	taskdb "github.com/H4ZM47/grind/internal/db"
	"github.com/spf13/cobra"
)

func newMilestoneCommand(opts *GlobalOptions) *cobra.Command {
	cmd := newGroupCommand("milestone", "Manage milestones")
	cmd.AddCommand(
		newMilestoneCreateCommand(opts),
		newMilestoneListCommand(opts),
		newMilestoneShowCommand(opts),
		newMilestoneUpdateCommand(opts),
		newMilestoneOpenCommand(opts),
		newMilestoneCloseCommand(opts),
		newMilestoneCancelCommand(opts),
	)
	return cmd
}

func newMilestoneCreateCommand(opts *GlobalOptions) *cobra.Command {
	var description string
	var dueAt string

	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a milestone",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, db, manager, err := milestoneManagerFromOptions(cmd.Context(), opts)
			if err != nil {
				return err
			}
			defer db.Close()

			milestone, err := manager.Create(cmd.Context(), app.CreateMilestoneRequest{
				Name:        args[0],
				Description: description,
				DueAt:       optionalString(cmd, "due-at", dueAt),
			})
			if err != nil {
				return err
			}

			if opts.JSON {
				return writeJSON(cmd, map[string]any{
					"ok":      true,
					"command": "grind milestone create",
					"data": map[string]any{
						"milestone": milestone,
					},
					"meta": map[string]any{},
				})
			}

			_, err = fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\n", milestone.Handle, milestone.Status, milestone.Name)
			return err
		},
	}

	cmd.Flags().StringVar(&description, "description", "", "Set the milestone description")
	cmd.Flags().StringVar(&dueAt, "due-at", "", "Set the milestone due timestamp")
	return cmd
}

func newMilestoneListCommand(opts *GlobalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List milestones",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, db, manager, err := milestoneManagerFromOptions(cmd.Context(), opts)
			if err != nil {
				return err
			}
			defer db.Close()

			items, err := manager.List(cmd.Context(), app.ListMilestonesRequest{})
			if err != nil {
				return err
			}

			if opts.JSON {
				return writeJSON(cmd, map[string]any{
					"ok":      true,
					"command": "grind milestone list",
					"data": map[string]any{
						"items": items,
					},
					"meta": map[string]any{
						"count": len(items),
					},
				})
			}

			for _, item := range items {
				if _, err := fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\n", item.Handle, item.Status, item.Name); err != nil {
					return err
				}
			}
			return nil
		},
	}
}

func newMilestoneShowCommand(opts *GlobalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "show <milestone-ref>",
		Short: "Show milestone detail",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, db, manager, err := milestoneManagerFromOptions(cmd.Context(), opts)
			if err != nil {
				return err
			}
			defer db.Close()

			milestone, err := manager.Show(cmd.Context(), app.ShowMilestoneRequest{Reference: args[0]})
			if err != nil {
				return err
			}

			if opts.JSON {
				return writeJSON(cmd, map[string]any{
					"ok":      true,
					"command": "grind milestone show",
					"data": map[string]any{
						"milestone": milestone,
					},
					"meta": map[string]any{},
				})
			}

			_, err = fmt.Fprintf(cmd.OutOrStdout(), "handle=%s\nstatus=%s\nname=%s\ndescription=%s\n", milestone.Handle, milestone.Status, milestone.Name, milestone.Description)
			return err
		},
	}
}

func newMilestoneUpdateCommand(opts *GlobalOptions) *cobra.Command {
	var name string
	var description string
	var dueAt string
	var status string

	cmd := &cobra.Command{
		Use:   "update <milestone-ref>",
		Short: "Update a milestone",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, db, manager, err := milestoneManagerFromOptions(cmd.Context(), opts)
			if err != nil {
				return err
			}
			defer db.Close()

			req := app.UpdateMilestoneRequest{Reference: args[0]}
			if cmd.Flags().Changed("name") {
				req.Name = &name
			}
			if cmd.Flags().Changed("description") {
				req.Description = &description
			}
			if cmd.Flags().Changed("due-at") {
				req.DueAt = &dueAt
			}
			if cmd.Flags().Changed("status") {
				req.Status = &status
			}

			if req.Name == nil && req.Description == nil && req.DueAt == nil && req.Status == nil {
				return fmt.Errorf("grind milestone update requires at least one changed field")
			}

			milestone, err := manager.Update(cmd.Context(), req)
			if err != nil {
				return err
			}

			if opts.JSON {
				return writeJSON(cmd, map[string]any{
					"ok":      true,
					"command": "grind milestone update",
					"data": map[string]any{
						"milestone": milestone,
					},
					"meta": map[string]any{},
				})
			}

			_, err = fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\n", milestone.Handle, milestone.Status, milestone.Name)
			return err
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Set the milestone name")
	cmd.Flags().StringVar(&description, "description", "", "Set the milestone description")
	cmd.Flags().StringVar(&dueAt, "due-at", "", "Set the milestone due timestamp")
	cmd.Flags().StringVar(&status, "status", "", "Set the milestone status")
	return cmd
}

func newMilestoneOpenCommand(opts *GlobalOptions) *cobra.Command {
	return newMilestoneStatusCommand(opts, "open", "Reopen a milestone", taskdb.StatusBacklog)
}

func newMilestoneCloseCommand(opts *GlobalOptions) *cobra.Command {
	return newMilestoneStatusCommand(opts, "close", "Close a milestone", taskdb.StatusCompleted)
}

func newMilestoneCancelCommand(opts *GlobalOptions) *cobra.Command {
	return newMilestoneStatusCommand(opts, "cancel", "Cancel a milestone", taskdb.StatusCancelled)
}

func newMilestoneStatusCommand(opts *GlobalOptions, use, short, status string) *cobra.Command {
	return newLifecycleStatusCommand(opts, lifecycleStatusCommandConfig{
		Use:             use,
		Short:           short,
		RefName:         "milestone-ref",
		CommandName:     "grind milestone " + use,
		StatusValue:     status,
		RetiredFlagHelp: "Retired milestone lifecycle flag; use the explicit open, close, or cancel verbs",
		MigrationError:  milestoneLifecycleMigrationError,
		Run: func(cmd *cobra.Command, reference string, status string) (lifecycleStatusResult, error) {
			_, db, manager, err := milestoneManagerFromOptions(cmd.Context(), opts)
			if err != nil {
				return lifecycleStatusResult{}, err
			}
			defer db.Close()

			milestone, err := manager.Update(cmd.Context(), app.UpdateMilestoneRequest{
				Reference: reference,
				Status:    &status,
			})
			if err != nil {
				return lifecycleStatusResult{}, err
			}
			return lifecycleStatusResult{
				DataKey: "milestone",
				Data:    milestone,
				Line:    fmt.Sprintf("%s\t%s\t%s\n", milestone.Handle, milestone.Status, milestone.Name),
			}, nil
		},
	})
}

func milestoneLifecycleMigrationError(use, handle, status string) error {
	switch status {
	case taskdb.StatusBacklog:
		return fmt.Errorf("`grind milestone close %s --status backlog` was removed; use `grind milestone open %s`", handle, handle)
	case taskdb.StatusCompleted:
		return fmt.Errorf("`grind milestone close %s --status completed` was removed; use `grind milestone close %s`", handle, handle)
	case taskdb.StatusCancelled:
		return fmt.Errorf("`grind milestone close %s --status cancelled` was removed; use `grind milestone cancel %s`", handle, handle)
	default:
		return fmt.Errorf("the `--status` flag was removed from `grind milestone %s`; use `grind milestone open`, `grind milestone close`, or `grind milestone cancel`", use)
	}
}

func milestoneManagerFromOptions(ctx context.Context, opts *GlobalOptions) (taskconfig.Resolved, *sql.DB, app.MilestoneManager, error) {
	cfg, err := taskconfig.Resolve(taskconfig.Options{
		DBPathOverride: opts.DBPath,
		ActorOverride:  opts.Actor,
	})
	if err != nil {
		return taskconfig.Resolved{}, nil, app.MilestoneManager{}, err
	}

	db, err := taskdb.Open(ctx, cfg)
	if err != nil {
		return taskconfig.Resolved{}, nil, app.MilestoneManager{}, err
	}

	return cfg, db, app.MilestoneManager{
		DB:              db,
		HumanName:       cfg.HumanName,
		CurrentActorRef: cfg.Actor,
	}, nil
}
