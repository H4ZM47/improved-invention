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

func newActorCommand(opts *GlobalOptions) *cobra.Command {
	cmd := newGroupCommand("actor", "Inspect and configure actors")
	cmd.AddCommand(
		newActorListCommand(opts),
		newActorShowCommand(opts),
	)
	return cmd
}

func newActorListCommand(opts *GlobalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List known actors",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, db, manager, err := actorManagerFromOptions(cmd.Context(), opts)
			if err != nil {
				return err
			}
			defer db.Close()

			if _, err := manager.BootstrapConfiguredHumanActor(cmd.Context()); err != nil {
				return err
			}

			actors, err := manager.List(cmd.Context(), app.ListActorsRequest{})
			if err != nil {
				return err
			}

			if opts.JSON {
				payload := map[string]any{
					"ok":      true,
					"command": "task actor list",
					"data": map[string]any{
						"items": actors,
					},
					"meta": map[string]any{
						"count":      len(actors),
						"human_name": cfg.HumanName,
					},
				}
				return writeJSON(cmd, payload)
			}

			for _, actor := range actors {
				if _, err := fmt.Fprintf(
					cmd.OutOrStdout(),
					"%s\t%s\t%s\n",
					actor.Handle,
					actor.Kind,
					actor.Label,
				); err != nil {
					return err
				}
			}

			return nil
		},
	}
}

func newActorShowCommand(opts *GlobalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "show <actor-ref>",
		Short: "Show actor detail",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, db, manager, err := actorManagerFromOptions(cmd.Context(), opts)
			if err != nil {
				return err
			}
			defer db.Close()

			actor, err := manager.Show(cmd.Context(), app.ShowActorRequest{
				Reference: args[0],
			})
			if err != nil {
				return err
			}

			if opts.JSON {
				return writeJSON(cmd, map[string]any{
					"ok":      true,
					"command": "task actor show",
					"data": map[string]any{
						"actor": actor,
					},
					"meta": map[string]any{},
				})
			}

			_, err = fmt.Fprintf(
				cmd.OutOrStdout(),
				"handle=%s\nkind=%s\nlabel=%s\nuuid=%s\nprovider=%s\nexternal_id=%s\ndisplay_name=%s\n",
				actor.Handle,
				actor.Kind,
				actor.Label,
				actor.UUID,
				actor.Provider,
				actor.ExternalID,
				actor.DisplayName,
			)
			return err
		},
	}
}

func actorManagerFromOptions(ctx context.Context, opts *GlobalOptions) (taskconfig.Resolved, *sql.DB, app.ActorManager, error) {
	cfg, err := taskconfig.Resolve(taskconfig.Options{
		DBPathOverride: opts.DBPath,
		ActorOverride:  opts.Actor,
	})
	if err != nil {
		return taskconfig.Resolved{}, nil, app.ActorManager{}, err
	}

	db, err := taskdb.Open(ctx, cfg)
	if err != nil {
		return taskconfig.Resolved{}, nil, app.ActorManager{}, err
	}

	return cfg, db, app.ActorManager{
		DB:        db,
		HumanName: cfg.HumanName,
	}, nil
}

func writeJSON(cmd *cobra.Command, payload map[string]any) error {
	return writeJSONTo(cmd.OutOrStdout(), payload)
}
