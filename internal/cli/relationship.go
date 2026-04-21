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

func newRelationshipCommand(opts *GlobalOptions) *cobra.Command {
	cmd := newGroupCommand("relationship", "Manage task relationships")
	cmd.AddCommand(
		newRelationshipAddCommand(opts),
		newRelationshipListCommand(opts),
		newRelationshipRemoveCommand(opts),
	)
	return cmd
}

func newRelationshipAddCommand(opts *GlobalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "add <type> <source-task-ref> <target-task-ref>",
		Short: "Create a relationship between two tasks",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, db, manager, err := relationshipManagerFromOptions(cmd.Context(), opts)
			if err != nil {
				return err
			}
			defer db.Close()

			relationship, err := manager.Create(cmd.Context(), app.CreateRelationshipRequest{
				Type:          args[0],
				SourceTaskRef: args[1],
				TargetTaskRef: args[2],
			})
			if err != nil {
				return err
			}

			if opts.JSON {
				return writeJSON(cmd, map[string]any{
					"ok":      true,
					"command": "grind relationship add",
					"data": map[string]any{
						"relationship": relationship,
					},
					"meta": map[string]any{},
				})
			}

			_, err = fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\t%s\n", relationship.UUID, relationship.Type, relationship.SourceTask, relationship.TargetTask)
			return err
		},
	}
}

func newRelationshipListCommand(opts *GlobalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "list <task-ref>",
		Short: "List relationships touching a task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, db, manager, err := relationshipManagerFromOptions(cmd.Context(), opts)
			if err != nil {
				return err
			}
			defer db.Close()

			items, err := manager.List(cmd.Context(), app.ListRelationshipsRequest{TaskRef: args[0]})
			if err != nil {
				return err
			}

			if opts.JSON {
				return writeJSON(cmd, map[string]any{
					"ok":      true,
					"command": "grind relationship list",
					"data": map[string]any{
						"items": items,
					},
					"meta": map[string]any{
						"count": len(items),
					},
				})
			}

			for _, item := range items {
				if _, err := fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\t%s\n", item.UUID, item.Type, item.SourceTask, item.TargetTask); err != nil {
					return err
				}
			}
			return nil
		},
	}
}

func newRelationshipRemoveCommand(opts *GlobalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "remove <type> <source-task-ref> <target-task-ref>",
		Short: "Remove a relationship between two tasks",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, db, manager, err := relationshipManagerFromOptions(cmd.Context(), opts)
			if err != nil {
				return err
			}
			defer db.Close()

			if err := manager.Remove(cmd.Context(), app.RemoveRelationshipRequest{
				Type:          args[0],
				SourceTaskRef: args[1],
				TargetTaskRef: args[2],
			}); err != nil {
				return err
			}

			if opts.JSON {
				return writeJSON(cmd, map[string]any{
					"ok":      true,
					"command": "grind relationship remove",
					"data":    map[string]any{},
					"meta":    map[string]any{},
				})
			}

			_, err = fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\tremoved\n", args[0], args[1], args[2])
			return err
		},
	}
}

func relationshipManagerFromOptions(ctx context.Context, opts *GlobalOptions) (taskconfig.Resolved, *sql.DB, app.RelationshipManager, error) {
	cfg, err := taskconfig.Resolve(taskconfig.Options{
		DBPathOverride: opts.DBPath,
		ActorOverride:  opts.Actor,
	})
	if err != nil {
		return taskconfig.Resolved{}, nil, app.RelationshipManager{}, err
	}

	db, err := taskdb.Open(ctx, cfg)
	if err != nil {
		return taskconfig.Resolved{}, nil, app.RelationshipManager{}, err
	}

	return cfg, db, app.RelationshipManager{
		DB:              db,
		HumanName:       cfg.HumanName,
		CurrentActorRef: cfg.Actor,
	}, nil
}
