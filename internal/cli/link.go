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
	cmd := newGroupCommand("link", "Manage task external links")
	cmd.AddCommand(
		newLinkAddCommand(opts),
		newLinkAttachCurrentRepoCommand(opts),
		newLinkListCommand(opts),
		newLinkRemoveCommand(opts),
	)
	return cmd
}

func newLinkAttachCurrentRepoCommand(opts *GlobalOptions) *cobra.Command {
	var noRepo bool

	cmd := &cobra.Command{
		Use:   "attach-current-repo <task-ref>",
		Short: "Explicitly attach the current repo/worktree context to a task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, db, manager, err := linkManagerFromOptions(cmd.Context(), opts)
			if err != nil {
				return err
			}
			defer db.Close()

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
					"command": "grind link attach-current-repo",
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
		Use:   "add <task-ref> <type> <target>",
		Short: "Create an external link for a task",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, db, manager, err := linkManagerFromOptions(cmd.Context(), opts)
			if err != nil {
				return err
			}
			defer db.Close()

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

			_, err = fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\n", link.UUID, link.Type, link.Target)
			return err
		},
	}

	cmd.Flags().StringVar(&label, "label", "", "Set a human-friendly label for the link")
	return cmd
}

func newLinkListCommand(opts *GlobalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "list <task-ref>",
		Short: "List external links for a task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, db, manager, err := linkManagerFromOptions(cmd.Context(), opts)
			if err != nil {
				return err
			}
			defer db.Close()

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
				if _, err := fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\n", item.UUID, item.Type, item.Target); err != nil {
					return err
				}
			}
			return nil
		},
	}
}

func newLinkRemoveCommand(opts *GlobalOptions) *cobra.Command {
	var linkType string
	var target string

	cmd := &cobra.Command{
		Use:   "remove <task-ref> <link-ref>",
		Short: "Remove an external link from a task",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, db, manager, err := linkManagerFromOptions(cmd.Context(), opts)
			if err != nil {
				return err
			}
			defer db.Close()

			req := app.RemoveLinkRequest{
				TaskRef: args[0],
				LinkRef: args[1],
			}
			if cmd.Flags().Changed("type") {
				req.Type = &linkType
			}
			if cmd.Flags().Changed("target") {
				req.Target = &target
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

			_, err = fmt.Fprintf(cmd.OutOrStdout(), "%s\tremoved\n", args[1])
			return err
		},
	}

	cmd.Flags().StringVar(&linkType, "type", "", "Constrain removal to a specific link type")
	cmd.Flags().StringVar(&target, "target", "", "Constrain removal to a specific link target")
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
