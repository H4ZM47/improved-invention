package cli

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/H4ZM47/improved-invention/internal/app"
	taskconfig "github.com/H4ZM47/improved-invention/internal/config"
	taskdb "github.com/H4ZM47/improved-invention/internal/db"
	"github.com/spf13/cobra"
)

func newProjectCommand(opts *GlobalOptions) *cobra.Command {
	cmd := newGroupCommand("project", "Manage projects")
	cmd.AddCommand(
		newProjectCreateCommand(opts),
		newProjectListCommand(opts),
		newProjectShowCommand(opts),
		newProjectUpdateCommand(opts),
		newProjectCloseCommand(opts),
	)
	return cmd
}

func newProjectCreateCommand(opts *GlobalOptions) *cobra.Command {
	var description string
	var domain string
	var defaultAssignee string
	var assignee string
	var dueAt string
	var tags string

	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if domain == "" {
				return fmt.Errorf("task project create requires --domain")
			}

			_, db, manager, err := projectManagerFromOptions(cmd.Context(), opts)
			if err != nil {
				return err
			}
			defer db.Close()

			project, err := manager.Create(cmd.Context(), app.CreateProjectRequest{
				Name:               args[0],
				Description:        description,
				DomainRef:          domain,
				DefaultAssigneeRef: optionalString(cmd, "default-assignee", defaultAssignee),
				AssigneeRef:        optionalString(cmd, "assignee", assignee),
				DueAt:              optionalString(cmd, "due-at", dueAt),
				Tags:               splitCSV(tags),
			})
			if err != nil {
				return err
			}

			if opts.JSON {
				return writeJSON(cmd, map[string]any{
					"ok":      true,
					"command": "task project create",
					"data": map[string]any{
						"project": project,
					},
					"meta": map[string]any{},
				})
			}

			_, err = fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\n", project.Handle, project.Status, project.Name)
			return err
		},
	}

	cmd.Flags().StringVar(&description, "description", "", "Set the project description")
	cmd.Flags().StringVar(&domain, "domain", "", "Set the project domain reference")
	cmd.Flags().StringVar(&defaultAssignee, "default-assignee", "", "Set the project default assignee")
	cmd.Flags().StringVar(&assignee, "assignee", "", "Set the project assignee")
	cmd.Flags().StringVar(&dueAt, "due-at", "", "Set the project due timestamp")
	cmd.Flags().StringVar(&tags, "tags", "", "Set comma-separated project tags")
	return cmd
}

func newProjectListCommand(opts *GlobalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List projects",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, db, manager, err := projectManagerFromOptions(cmd.Context(), opts)
			if err != nil {
				return err
			}
			defer db.Close()

			items, err := manager.List(cmd.Context(), app.ListProjectsRequest{})
			if err != nil {
				return err
			}

			if opts.JSON {
				return writeJSON(cmd, map[string]any{
					"ok":      true,
					"command": "task project list",
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

func newProjectShowCommand(opts *GlobalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "show <project-ref>",
		Short: "Show project detail",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, db, manager, err := projectManagerFromOptions(cmd.Context(), opts)
			if err != nil {
				return err
			}
			defer db.Close()

			project, err := manager.Show(cmd.Context(), app.ShowProjectRequest{Reference: args[0]})
			if err != nil {
				return err
			}

			if opts.JSON {
				return writeJSON(cmd, map[string]any{
					"ok":      true,
					"command": "task project show",
					"data": map[string]any{
						"project": project,
					},
					"meta": map[string]any{},
				})
			}

			_, err = fmt.Fprintf(cmd.OutOrStdout(), "handle=%s\nstatus=%s\nname=%s\ndescription=%s\ndomain_id=%s\n", project.Handle, project.Status, project.Name, project.Description, project.DomainID)
			return err
		},
	}
}

func newProjectUpdateCommand(opts *GlobalOptions) *cobra.Command {
	var name string
	var description string
	var domain string
	var defaultAssignee string
	var assignee string
	var dueAt string
	var tags string
	var status string

	cmd := &cobra.Command{
		Use:   "update <project-ref>",
		Short: "Update a project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, db, manager, err := projectManagerFromOptions(cmd.Context(), opts)
			if err != nil {
				return err
			}
			defer db.Close()

			req := app.UpdateProjectRequest{Reference: args[0]}
			if cmd.Flags().Changed("name") {
				req.Name = &name
			}
			if cmd.Flags().Changed("description") {
				req.Description = &description
			}
			if cmd.Flags().Changed("domain") {
				req.DomainRef = &domain
			}
			if cmd.Flags().Changed("default-assignee") {
				req.DefaultAssigneeRef = &defaultAssignee
			}
			if cmd.Flags().Changed("assignee") {
				req.AssigneeRef = &assignee
			}
			if cmd.Flags().Changed("due-at") {
				req.DueAt = &dueAt
			}
			if cmd.Flags().Changed("tags") {
				values := splitCSV(tags)
				req.Tags = &values
			}
			if cmd.Flags().Changed("status") {
				req.Status = &status
			}

			if req.Name == nil && req.Description == nil && req.DomainRef == nil && req.DefaultAssigneeRef == nil && req.AssigneeRef == nil && req.DueAt == nil && req.Tags == nil && req.Status == nil {
				return fmt.Errorf("task project update requires at least one changed field")
			}

			project, err := manager.Update(cmd.Context(), req)
			if err != nil {
				return err
			}

			if opts.JSON {
				return writeJSON(cmd, map[string]any{
					"ok":      true,
					"command": "task project update",
					"data": map[string]any{
						"project": project,
					},
					"meta": map[string]any{},
				})
			}

			_, err = fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\n", project.Handle, project.Status, project.Name)
			return err
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Set the project name")
	cmd.Flags().StringVar(&description, "description", "", "Set the project description")
	cmd.Flags().StringVar(&domain, "domain", "", "Set the project domain reference")
	cmd.Flags().StringVar(&defaultAssignee, "default-assignee", "", "Set the project default assignee")
	cmd.Flags().StringVar(&assignee, "assignee", "", "Set the project assignee")
	cmd.Flags().StringVar(&dueAt, "due-at", "", "Set the project due timestamp")
	cmd.Flags().StringVar(&tags, "tags", "", "Set comma-separated project tags")
	cmd.Flags().StringVar(&status, "status", "", "Set the project status")
	return cmd
}

func newProjectCloseCommand(opts *GlobalOptions) *cobra.Command {
	var status string

	cmd := &cobra.Command{
		Use:   "close <project-ref>",
		Short: "Close or reopen a project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if status == "" {
				status = "completed"
			}

			_, db, manager, err := projectManagerFromOptions(cmd.Context(), opts)
			if err != nil {
				return err
			}
			defer db.Close()

			project, err := manager.Update(cmd.Context(), app.UpdateProjectRequest{
				Reference: args[0],
				Status:    &status,
			})
			if err != nil {
				return err
			}

			if opts.JSON {
				return writeJSON(cmd, map[string]any{
					"ok":      true,
					"command": "task project close",
					"data": map[string]any{
						"project": project,
					},
					"meta": map[string]any{},
				})
			}

			_, err = fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\n", project.Handle, project.Status, project.Name)
			return err
		},
	}

	cmd.Flags().StringVar(&status, "status", "", "Set the terminal or reopened project status")
	return cmd
}

func projectManagerFromOptions(ctx context.Context, opts *GlobalOptions) (taskconfig.Resolved, *sql.DB, app.ProjectManager, error) {
	cfg, err := taskconfig.Resolve(taskconfig.Options{
		DBPathOverride: opts.DBPath,
		ActorOverride:  opts.Actor,
	})
	if err != nil {
		return taskconfig.Resolved{}, nil, app.ProjectManager{}, err
	}

	db, err := taskdb.Open(ctx, cfg)
	if err != nil {
		return taskconfig.Resolved{}, nil, app.ProjectManager{}, err
	}

	return cfg, db, app.ProjectManager{
		DB:              db,
		HumanName:       cfg.HumanName,
		CurrentActorRef: cfg.Actor,
	}, nil
}
