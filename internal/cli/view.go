package cli

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/H4ZM47/task-cli/internal/app"
	taskconfig "github.com/H4ZM47/task-cli/internal/config"
	taskdb "github.com/H4ZM47/task-cli/internal/db"
	"github.com/spf13/cobra"
)

func newViewCommand(opts *GlobalOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "view",
		Short: "Manage saved views",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(
		newViewCreateCommand(opts),
		newViewUpdateCommand(opts),
		newViewListCommand(opts),
		newViewShowCommand(opts),
		newViewApplyCommand(opts),
		newViewDeleteCommand(opts),
	)
	return cmd
}

type viewFilterFlags struct {
	statuses  []string
	tags      []string
	domain    string
	project   string
	assignee  string
	dueBefore string
	dueAfter  string
	search    string
}

func (f *viewFilterFlags) bind(cmd *cobra.Command) {
	cmd.Flags().StringArrayVar(&f.statuses, "status", nil, "Filter by task status; repeat to allow multiple statuses")
	cmd.Flags().StringArrayVar(&f.tags, "tag", nil, "Filter by task tag; repeat to require multiple tags")
	cmd.Flags().StringVar(&f.domain, "domain", "", "Filter by task domain reference")
	cmd.Flags().StringVar(&f.project, "project", "", "Filter by task project reference")
	cmd.Flags().StringVar(&f.assignee, "assignee", "", "Filter by task assignee reference")
	cmd.Flags().StringVar(&f.dueBefore, "due-before", "", "Filter to tasks due on or before the RFC3339 timestamp")
	cmd.Flags().StringVar(&f.dueAfter, "due-after", "", "Filter to tasks due on or after the RFC3339 timestamp")
	cmd.Flags().StringVar(&f.search, "search", "", "Filter by case-insensitive search across title and description")
}

func (f viewFilterFlags) toSavedViewFilters() (app.SavedViewFilters, error) {
	normalizedDueBefore, err := normalizeRFC3339IfSet("due-before", f.dueBefore)
	if err != nil {
		return app.SavedViewFilters{}, err
	}
	normalizedDueAfter, err := normalizeRFC3339IfSet("due-after", f.dueAfter)
	if err != nil {
		return app.SavedViewFilters{}, err
	}

	return app.SavedViewFilters{
		Statuses:    f.statuses,
		Tags:        f.tags,
		DomainRef:   f.domain,
		ProjectRef:  f.project,
		AssigneeRef: f.assignee,
		DueBefore:   normalizedDueBefore,
		DueAfter:    normalizedDueAfter,
		Search:      f.search,
	}, nil
}

func normalizeRFC3339IfSet(name, value string) (string, error) {
	if value == "" {
		return "", nil
	}
	return normalizeRFC3339Flag(name, value, true)
}

func newViewCreateCommand(opts *GlobalOptions) *cobra.Command {
	var filters viewFilterFlags

	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a saved view",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			db, manager, err := viewManagerFromOptions(ctx, opts)
			if err != nil {
				return err
			}
			defer db.Close()

			savedFilters, err := filters.toSavedViewFilters()
			if err != nil {
				return err
			}

			view, err := manager.Create(ctx, app.CreateViewRequest{
				Name:    args[0],
				Filters: savedFilters,
			})
			if err != nil {
				return err
			}

			return emitViewPayload(cmd, opts, "task view create", view)
		},
	}
	filters.bind(cmd)
	return cmd
}

func newViewUpdateCommand(opts *GlobalOptions) *cobra.Command {
	var filters viewFilterFlags
	var newName string

	cmd := &cobra.Command{
		Use:   "update <name>",
		Short: "Update a saved view",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			db, manager, err := viewManagerFromOptions(ctx, opts)
			if err != nil {
				return err
			}
			defer db.Close()

			savedFilters, err := filters.toSavedViewFilters()
			if err != nil {
				return err
			}

			view, err := manager.Update(ctx, app.UpdateViewRequest{
				Name:    args[0],
				NewName: newName,
				Filters: savedFilters,
			})
			if err != nil {
				return err
			}

			return emitViewPayload(cmd, opts, "task view update", view)
		},
	}
	filters.bind(cmd)
	cmd.Flags().StringVar(&newName, "rename", "", "Rename the saved view to the provided name")
	return cmd
}

func newViewListCommand(opts *GlobalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List saved views",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()

			db, manager, err := viewManagerFromOptions(ctx, opts)
			if err != nil {
				return err
			}
			defer db.Close()

			views, err := manager.List(ctx, app.ListViewsRequest{})
			if err != nil {
				return err
			}

			if opts.JSON {
				return writeJSON(cmd, map[string]any{
					"ok":      true,
					"command": "task view list",
					"data":    map[string]any{"items": views},
					"meta":    map[string]any{"count": len(views)},
				})
			}

			for _, v := range views {
				if _, err := fmt.Fprintln(cmd.OutOrStdout(), v.Name); err != nil {
					return err
				}
			}
			return nil
		},
	}
}

func newViewShowCommand(opts *GlobalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "show <name>",
		Short: "Show a saved view's stored filters",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			db, manager, err := viewManagerFromOptions(ctx, opts)
			if err != nil {
				return err
			}
			defer db.Close()

			view, err := manager.Show(ctx, app.ShowViewRequest{Name: args[0]})
			if err != nil {
				return err
			}

			return emitViewPayload(cmd, opts, "task view show", view)
		},
	}
}

func newViewApplyCommand(opts *GlobalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "apply <name>",
		Short: "List tasks matching a saved view",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			cfg, db, err := openViewDB(ctx, opts)
			if err != nil {
				return err
			}
			defer db.Close()

			view, err := app.ViewManager{DB: db}.Show(ctx, app.ShowViewRequest{Name: args[0]})
			if err != nil {
				return err
			}

			tasks, err := app.TaskManager{DB: db, HumanName: cfg.HumanName}.List(ctx, view.Filters.ToListTasksRequest())
			if err != nil {
				return err
			}

			if opts.JSON {
				return writeJSON(cmd, map[string]any{
					"ok":      true,
					"command": "task view apply",
					"data":    map[string]any{"items": tasks},
					"meta": map[string]any{
						"count": len(tasks),
						"view":  view.Name,
					},
				})
			}

			for _, task := range tasks {
				if _, err := fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\n", task.Handle, task.Status, task.Title); err != nil {
					return err
				}
			}
			return nil
		},
	}
}

func newViewDeleteCommand(opts *GlobalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete a saved view",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			db, manager, err := viewManagerFromOptions(ctx, opts)
			if err != nil {
				return err
			}
			defer db.Close()

			if err := manager.Delete(ctx, app.DeleteViewRequest{Name: args[0]}); err != nil {
				return err
			}

			if opts.JSON {
				return writeJSON(cmd, map[string]any{
					"ok":      true,
					"command": "task view delete",
					"data":    map[string]any{"name": args[0]},
					"meta":    map[string]any{},
				})
			}
			return nil
		},
	}
}

func emitViewPayload(cmd *cobra.Command, opts *GlobalOptions, commandName string, view app.ViewRecord) error {
	if opts.JSON {
		return writeJSON(cmd, map[string]any{
			"ok":      true,
			"command": commandName,
			"data":    map[string]any{"view": view},
			"meta":    map[string]any{},
		})
	}

	_, err := fmt.Fprintf(cmd.OutOrStdout(), "%s\n", view.Name)
	return err
}

func openViewDB(ctx context.Context, opts *GlobalOptions) (taskconfig.Resolved, *sql.DB, error) {
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

func viewManagerFromOptions(ctx context.Context, opts *GlobalOptions) (*sql.DB, app.ViewManager, error) {
	_, db, err := openViewDB(ctx, opts)
	if err != nil {
		return nil, app.ViewManager{}, err
	}
	return db, app.ViewManager{DB: db}, nil
}
