package cli

import (
	"context"
	"database/sql"
	"fmt"
	"os"

	"github.com/H4ZM47/grind/internal/app"
	taskconfig "github.com/H4ZM47/grind/internal/config"
	taskdb "github.com/H4ZM47/grind/internal/db"
	"github.com/H4ZM47/grind/internal/gitctx"
	"github.com/spf13/cobra"
)

func newLinkCommand(opts *GlobalOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "link",
		Short: "Manage task connections",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) >= 1 && args[0] == "attach-current-repo" {
				return fmt.Errorf("`grind link attach-current-repo` was removed; use `grind link-repo`")
			}
			return cmd.Help()
		},
	}
	cmd.AddCommand(
		newLinkAddCommand(opts),
		newLinkListCommand(opts),
		newLinkRemoveCommand(opts),
	)
	return cmd
}

func newLinkRepoCommand(opts *GlobalOptions) *cobra.Command {
	var noRepo bool

	cmd := &cobra.Command{
		Use:   "link-repo <task-ref>",
		Short: "Explicitly attach the current repo/worktree context to a task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, db, manager, err := linkManagerFromOptions(cmd.Context(), opts)
			if err != nil {
				return err
			}
			defer func() {
				_ = db.Close()
			}()

			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			current, err := gitctx.Detect(cmd.Context(), cwd)
			if err != nil {
				return err
			}

			result, err := manager.AttachCurrentRepoContext(cmd.Context(), app.AttachCurrentRepoContextRequest{
				TaskRef:  args[0],
				Context:  current,
				LinkRepo: !noRepo,
			})
			if err != nil {
				return err
			}

			if opts.JSON {
				return writeJSON(cmd, map[string]any{
					"ok":      true,
					"command": "grind link-repo",
					"data": map[string]any{
						"repo_link":     result.RepoLink,
						"worktree_link": result.WorktreeLink,
					},
					"meta": map[string]any{
						"repo_target":     current.RepoTarget(),
						"worktree_target": current.WorktreeTarget(),
					},
				})
			}

			if result.RepoLink != nil {
				if _, err := fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\n", result.RepoLink.UUID, result.RepoLink.Type, result.RepoLink.Target); err != nil {
					return err
				}
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\n", result.WorktreeLink.UUID, result.WorktreeLink.Type, result.WorktreeLink.Target)
			return err
		},
	}

	cmd.Flags().BoolVar(&noRepo, "no-repo-link", false, "Attach only the current worktree link and skip the repo link")
	return cmd
}

func newLinkAddCommand(opts *GlobalOptions) *cobra.Command {
	var label string

	cmd := &cobra.Command{
		Use:   "add <source> <relationship> <target>",
		Short: "Create a typed link to a task or external resource",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, db, manager, err := linkManagerFromOptions(cmd.Context(), opts)
			if err != nil {
				return err
			}
			defer func() {
				_ = db.Close()
			}()

			link, err := manager.Create(cmd.Context(), app.CreateLinkRequest{
				TaskRef: args[0],
				Type:    args[1],
				Target:  args[2],
				Label:   label,
			})
			if err != nil {
				return err
			}

			if opts.JSON {
				return writeJSON(cmd, map[string]any{
					"ok":      true,
					"command": "grind link add",
					"data": map[string]any{
						"link": link,
					},
					"meta": map[string]any{},
				})
			}

			_, err = fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\t%s\n", link.UUID, link.Type, link.TargetKind, link.Target)
			return err
		},
	}

	cmd.Flags().StringVar(&label, "label", "", "Set a human-friendly label for the link")
	cmd.Long = "Create a typed link.\n\nTask links use the conversational order `source relationship target`, for example `grind link add TASK-1 blocks TASK-2`.\nExternal links use the same order, for example `grind link add TASK-1 url https://example.com/spec`."
	return cmd
}

func newLinkListCommand(opts *GlobalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "list <task-ref>",
		Short: "List task and external links touching a task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, db, manager, err := linkManagerFromOptions(cmd.Context(), opts)
			if err != nil {
				return err
			}
			defer func() {
				_ = db.Close()
			}()

			items, err := manager.List(cmd.Context(), app.ListLinksRequest{TaskRef: args[0]})
			if err != nil {
				return err
			}

			if opts.JSON {
				return writeJSON(cmd, map[string]any{
					"ok":      true,
					"command": "grind link list",
					"data": map[string]any{
						"items": items,
					},
					"meta": map[string]any{
						"count": len(items),
					},
				})
			}

			for _, item := range items {
				if _, err := fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\t%s\n", item.UUID, item.Type, item.TargetKind, item.Target); err != nil {
					return err
				}
			}
			return nil
		},
	}
}

func newLinkRemoveCommand(opts *GlobalOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove <source> <relationship> <target>",
		Short: "Remove a typed link from a task",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, db, manager, err := linkManagerFromOptions(cmd.Context(), opts)
			if err != nil {
				return err
			}
			defer func() {
				_ = db.Close()
			}()

			req := app.RemoveLinkRequest{
				TaskRef: args[0],
				Type:    &args[1],
				Target:  &args[2],
			}

			if err := manager.Remove(cmd.Context(), req); err != nil {
				return err
			}

			if opts.JSON {
				return writeJSON(cmd, map[string]any{
					"ok":      true,
					"command": "grind link remove",
					"data":    map[string]any{},
					"meta":    map[string]any{},
				})
			}

			_, err = fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\tremoved\n", args[0], args[1], args[2])
			return err
		},
	}
	return cmd
}

func linkManagerFromOptions(ctx context.Context, opts *GlobalOptions) (taskconfig.Resolved, *sql.DB, app.LinkManager, error) {
	cfg, err := taskconfig.Resolve(taskconfig.Options{
		DBPathOverride: opts.DBPath,
		ActorOverride:  opts.Actor,
	})
	if err != nil {
		return taskconfig.Resolved{}, nil, app.LinkManager{}, err
	}

	db, err := taskdb.Open(ctx, cfg)
	if err != nil {
		return taskconfig.Resolved{}, nil, app.LinkManager{}, err
	}

	return cfg, db, app.LinkManager{
		DB:              db,
		HumanName:       cfg.HumanName,
		CurrentActorRef: cfg.Actor,
	}, nil
}
