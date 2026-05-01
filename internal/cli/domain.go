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

func newDomainCommand(opts *GlobalOptions) *cobra.Command {
	cmd := newGroupCommand("domain", "Manage domains")
	cmd.AddCommand(
		newDomainCreateCommand(opts),
		newDomainListCommand(opts),
		newDomainShowCommand(opts),
		newDomainUpdateCommand(opts),
		newDomainOpenCommand(opts),
		newDomainCloseCommand(opts),
		newDomainCancelCommand(opts),
	)
	return cmd
}

func newDomainCreateCommand(opts *GlobalOptions) *cobra.Command {
	var description string
	var defaultAssignee string
	var assignee string
	var dueAt string
	var tags string

	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a domain",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, db, manager, err := domainManagerFromOptions(cmd.Context(), opts)
			if err != nil {
				return err
			}
			defer db.Close()

			domain, err := manager.Create(cmd.Context(), app.CreateDomainRequest{
				Name:               args[0],
				Description:        description,
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
					"command": "grind domain create",
					"data": map[string]any{
						"domain": domain,
					},
					"meta": map[string]any{},
				})
			}

			_, err = fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\n", domain.Handle, domain.Status, domain.Name)
			return err
		},
	}

	cmd.Flags().StringVar(&description, "description", "", "Set the domain description")
	cmd.Flags().StringVar(&defaultAssignee, "default-assignee", "", "Set the domain default assignee")
	cmd.Flags().StringVar(&assignee, "assignee", "", "Set the domain assignee")
	cmd.Flags().StringVar(&dueAt, "due-at", "", "Set the domain due timestamp")
	cmd.Flags().StringVar(&tags, "tags", "", "Set comma-separated domain tags")
	return cmd
}

func newDomainListCommand(opts *GlobalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List domains",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, db, manager, err := domainManagerFromOptions(cmd.Context(), opts)
			if err != nil {
				return err
			}
			defer db.Close()

			items, err := manager.List(cmd.Context(), app.ListDomainsRequest{})
			if err != nil {
				return err
			}

			if opts.JSON {
				return writeJSON(cmd, map[string]any{
					"ok":      true,
					"command": "grind domain list",
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

func newDomainShowCommand(opts *GlobalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "show <domain-ref>",
		Short: "Show domain detail",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, db, manager, err := domainManagerFromOptions(cmd.Context(), opts)
			if err != nil {
				return err
			}
			defer db.Close()

			domain, err := manager.Show(cmd.Context(), app.ShowDomainRequest{Reference: args[0]})
			if err != nil {
				return err
			}

			if opts.JSON {
				return writeJSON(cmd, map[string]any{
					"ok":      true,
					"command": "grind domain show",
					"data": map[string]any{
						"domain": domain,
					},
					"meta": map[string]any{},
				})
			}

			_, err = fmt.Fprintf(cmd.OutOrStdout(), "handle=%s\nstatus=%s\nname=%s\ndescription=%s\n", domain.Handle, domain.Status, domain.Name, domain.Description)
			return err
		},
	}
}

func newDomainUpdateCommand(opts *GlobalOptions) *cobra.Command {
	var name string
	var description string
	var defaultAssignee string
	var assignee string
	var dueAt string
	var tags string
	var status string

	cmd := &cobra.Command{
		Use:   "update <domain-ref>",
		Short: "Update a domain",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, db, manager, err := domainManagerFromOptions(cmd.Context(), opts)
			if err != nil {
				return err
			}
			defer db.Close()

			req := app.UpdateDomainRequest{Reference: args[0]}
			if cmd.Flags().Changed("name") {
				req.Name = &name
			}
			if cmd.Flags().Changed("description") {
				req.Description = &description
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

			if req.Name == nil && req.Description == nil && req.DefaultAssigneeRef == nil && req.AssigneeRef == nil && req.DueAt == nil && req.Tags == nil && req.Status == nil {
				return fmt.Errorf("grind domain update requires at least one changed field")
			}

			domain, err := manager.Update(cmd.Context(), req)
			if err != nil {
				return err
			}

			if opts.JSON {
				return writeJSON(cmd, map[string]any{
					"ok":      true,
					"command": "grind domain update",
					"data": map[string]any{
						"domain": domain,
					},
					"meta": map[string]any{},
				})
			}

			_, err = fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\n", domain.Handle, domain.Status, domain.Name)
			return err
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Set the domain name")
	cmd.Flags().StringVar(&description, "description", "", "Set the domain description")
	cmd.Flags().StringVar(&defaultAssignee, "default-assignee", "", "Set the domain default assignee")
	cmd.Flags().StringVar(&assignee, "assignee", "", "Set the domain assignee")
	cmd.Flags().StringVar(&dueAt, "due-at", "", "Set the domain due timestamp")
	cmd.Flags().StringVar(&tags, "tags", "", "Set comma-separated domain tags")
	cmd.Flags().StringVar(&status, "status", "", "Set the domain status")
	return cmd
}

func newDomainOpenCommand(opts *GlobalOptions) *cobra.Command {
	return newDomainStatusCommand(opts, "open", "Reopen a domain", taskdb.StatusBacklog)
}

func newDomainCloseCommand(opts *GlobalOptions) *cobra.Command {
	return newDomainStatusCommand(opts, "close", "Close a domain", taskdb.StatusCompleted)
}

func newDomainCancelCommand(opts *GlobalOptions) *cobra.Command {
	return newDomainStatusCommand(opts, "cancel", "Cancel a domain", taskdb.StatusCancelled)
}

func newDomainStatusCommand(opts *GlobalOptions, use, short, statusValue string) *cobra.Command {
	return newLifecycleStatusCommand(opts, lifecycleStatusCommandConfig{
		Use:             use,
		Short:           short,
		RefName:         "domain-ref",
		CommandName:     "grind domain " + use,
		StatusValue:     statusValue,
		RetiredFlagHelp: "Retired domain lifecycle flag; use the explicit open, close, or cancel verbs",
		MigrationError:  domainLifecycleMigrationError,
		Run: func(cmd *cobra.Command, reference string, status string) (lifecycleStatusResult, error) {
			_, db, manager, err := domainManagerFromOptions(cmd.Context(), opts)
			if err != nil {
				return lifecycleStatusResult{}, err
			}
			defer db.Close()

			domain, err := manager.Update(cmd.Context(), app.UpdateDomainRequest{
				Reference: reference,
				Status:    &status,
			})
			if err != nil {
				return lifecycleStatusResult{}, err
			}
			return lifecycleStatusResult{
				DataKey: "domain",
				Data:    domain,
				Line:    fmt.Sprintf("%s\t%s\t%s\n", domain.Handle, domain.Status, domain.Name),
			}, nil
		},
	})
}

func domainLifecycleMigrationError(use, handle, status string) error {
	switch status {
	case taskdb.StatusBacklog:
		return fmt.Errorf("`grind domain close %s --status backlog` was removed; use `grind domain open %s`", handle, handle)
	case taskdb.StatusCompleted:
		return fmt.Errorf("`grind domain close %s --status completed` was removed; use `grind domain close %s`", handle, handle)
	case taskdb.StatusCancelled:
		return fmt.Errorf("`grind domain close %s --status cancelled` was removed; use `grind domain cancel %s`", handle, handle)
	default:
		return fmt.Errorf("the `--status` flag was removed from `grind domain %s`; use `grind domain open`, `grind domain close`, or `grind domain cancel`", use)
	}
}

func domainManagerFromOptions(ctx context.Context, opts *GlobalOptions) (taskconfig.Resolved, *sql.DB, app.DomainManager, error) {
	cfg, err := taskconfig.Resolve(taskconfig.Options{
		DBPathOverride: opts.DBPath,
		ActorOverride:  opts.Actor,
	})
	if err != nil {
		return taskconfig.Resolved{}, nil, app.DomainManager{}, err
	}

	db, err := taskdb.Open(ctx, cfg)
	if err != nil {
		return taskconfig.Resolved{}, nil, app.DomainManager{}, err
	}

	return cfg, db, app.DomainManager{
		DB:              db,
		HumanName:       cfg.HumanName,
		CurrentActorRef: cfg.Actor,
	}, nil
}

func optionalString(cmd *cobra.Command, flagName string, value string) *string {
	if !cmd.Flags().Changed(flagName) {
		return nil
	}
	return &value
}
