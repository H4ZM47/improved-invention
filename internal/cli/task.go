package cli

import (
	"bufio"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/H4ZM47/improved-invention/internal/app"
	taskconfig "github.com/H4ZM47/improved-invention/internal/config"
	taskdb "github.com/H4ZM47/improved-invention/internal/db"
	"github.com/spf13/cobra"
)

func newTaskCreateCommand(opts *GlobalOptions) *cobra.Command {
	var description string
	var tags string
	var domain string
	var project string
	var assignee string
	var dueAt string

	cmd := &cobra.Command{
		Use:   "create <title>",
		Short: "Create a task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, db, manager, err := taskManagerFromOptions(cmd.Context(), opts)
			if err != nil {
				return err
			}
			defer db.Close()

			task, err := manager.Create(cmd.Context(), app.CreateTaskRequest{
				Title:       args[0],
				Description: description,
				Tags:        splitCSV(tags),
				DomainRef:   optionalString(cmd, "domain", domain),
				ProjectRef:  optionalString(cmd, "project", project),
				AssigneeRef: optionalString(cmd, "assignee", assignee),
				DueAt:       optionalString(cmd, "due-at", dueAt),
			})
			if err != nil {
				return err
			}

			if opts.JSON {
				return writeJSON(cmd, map[string]any{
					"ok":      true,
					"command": "task create",
					"data": map[string]any{
						"task": task,
					},
					"meta": map[string]any{
						"claimed": false,
					},
				})
			}

			_, err = fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\n", task.Handle, task.Status, task.Title)
			return err
		},
	}

	cmd.Flags().StringVar(&description, "description", "", "Set the task description")
	cmd.Flags().StringVar(&tags, "tags", "", "Set comma-separated task tags")
	cmd.Flags().StringVar(&domain, "domain", "", "Set the task domain reference")
	cmd.Flags().StringVar(&project, "project", "", "Set the task project reference")
	cmd.Flags().StringVar(&assignee, "assignee", "", "Set the task assignee")
	cmd.Flags().StringVar(&dueAt, "due-at", "", "Set the task due timestamp")
	return cmd
}

func newTaskListCommand(opts *GlobalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List tasks",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, db, manager, err := taskManagerFromOptions(cmd.Context(), opts)
			if err != nil {
				return err
			}
			defer db.Close()

			items, err := manager.List(cmd.Context(), app.ListTasksRequest{})
			if err != nil {
				return err
			}

			if opts.JSON {
				return writeJSON(cmd, map[string]any{
					"ok":      true,
					"command": "task list",
					"data": map[string]any{
						"items": items,
					},
					"meta": map[string]any{
						"count": len(items),
					},
				})
			}

			for _, item := range items {
				if _, err := fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\n", item.Handle, item.Status, item.Title); err != nil {
					return err
				}
			}
			return nil
		},
	}
}

func newTaskShowCommand(opts *GlobalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "show <task-ref>",
		Short: "Show task detail",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, db, manager, err := taskManagerFromOptions(cmd.Context(), opts)
			if err != nil {
				return err
			}
			defer db.Close()

			task, err := manager.Show(cmd.Context(), app.ShowTaskRequest{Reference: args[0]})
			if err != nil {
				return err
			}

			if opts.JSON {
				return writeJSON(cmd, map[string]any{
					"ok":      true,
					"command": "task show",
					"data": map[string]any{
						"task": task,
					},
					"meta": map[string]any{},
				})
			}

			_, err = fmt.Fprintf(
				cmd.OutOrStdout(),
				"handle=%s\nstatus=%s\ntitle=%s\ndescription=%s\n",
				task.Handle,
				task.Status,
				task.Title,
				task.Description,
			)
			return err
		},
	}
}

func newTaskUpdateCommand(opts *GlobalOptions) *cobra.Command {
	var title string
	var description string
	var tags string
	var domain string
	var project string
	var assignee string
	var dueAt string
	var status string
	var acceptDefaultAssignee bool
	var keepAssignee bool

	cmd := &cobra.Command{
		Use:   "update <task-ref>",
		Short: "Update task fields",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, db, manager, err := taskManagerFromOptions(cmd.Context(), opts)
			if err != nil {
				return err
			}
			defer db.Close()

			if keepAssignee && acceptDefaultAssignee {
				return fmt.Errorf("task update allows only one of --keep-assignee or --accept-default-assignee")
			}
			if cmd.Flags().Changed("assignee") && (keepAssignee || acceptDefaultAssignee) {
				return fmt.Errorf("task update allows only one explicit assignee decision")
			}

			req := app.UpdateTaskRequest{Reference: args[0]}
			if cmd.Flags().Changed("title") {
				req.Title = &title
			}
			if cmd.Flags().Changed("description") {
				req.Description = &description
			}
			if cmd.Flags().Changed("tags") {
				values := splitCSV(tags)
				req.Tags = &values
			}
			if cmd.Flags().Changed("domain") {
				req.DomainRef = &domain
			}
			if cmd.Flags().Changed("project") {
				req.ProjectRef = &project
			}
			if cmd.Flags().Changed("assignee") {
				req.AssigneeRef = &assignee
			}
			if cmd.Flags().Changed("due-at") {
				req.DueAt = &dueAt
			}
			if cmd.Flags().Changed("status") {
				req.Status = &status
			}
			req.AcceptDefaultAssignee = acceptDefaultAssignee
			req.KeepAssignee = keepAssignee

			if req.Title == nil && req.Description == nil && req.Tags == nil && req.DomainRef == nil && req.ProjectRef == nil && req.AssigneeRef == nil && req.DueAt == nil && req.Status == nil && !req.AcceptDefaultAssignee && !req.KeepAssignee {
				return fmt.Errorf("task update requires at least one changed field")
			}

			task, err := manager.Update(cmd.Context(), req)
			var decisionErr *app.AssignmentDecisionRequiredError
			if err != nil && errors.As(err, &decisionErr) {
				if opts.NoInput {
					return err
				}

				promptedAssignee, promptedKeep, promptErr := promptAssigneeDecision(cmd.ErrOrStderr(), cmd.InOrStdin(), *decisionErr)
				if promptErr != nil {
					return promptErr
				}

				req.KeepAssignee = promptedKeep
				req.AcceptDefaultAssignee = !promptedKeep && promptedAssignee == nil
				req.AssigneeRef = promptedAssignee
				task, err = manager.Update(cmd.Context(), req)
			}
			if err != nil {
				return err
			}

			if opts.JSON {
				return writeJSON(cmd, map[string]any{
					"ok":      true,
					"command": "task update",
					"data": map[string]any{
						"task": task,
					},
					"meta": map[string]any{},
				})
			}

			_, err = fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\n", task.Handle, task.Status, task.Title)
			return err
		},
	}

	cmd.Flags().StringVar(&title, "title", "", "Set the task title")
	cmd.Flags().StringVar(&description, "description", "", "Set the task description")
	cmd.Flags().StringVar(&tags, "tags", "", "Set comma-separated task tags")
	cmd.Flags().StringVar(&domain, "domain", "", "Set the task domain reference")
	cmd.Flags().StringVar(&project, "project", "", "Set the task project reference")
	cmd.Flags().StringVar(&assignee, "assignee", "", "Set the task assignee")
	cmd.Flags().StringVar(&dueAt, "due-at", "", "Set the task due timestamp")
	cmd.Flags().StringVar(&status, "status", "", "Set the task status")
	cmd.Flags().BoolVar(&acceptDefaultAssignee, "accept-default-assignee", false, "Accept the inherited default assignee during reclassification")
	cmd.Flags().BoolVar(&keepAssignee, "keep-assignee", false, "Keep the current assignee during reclassification")

	return cmd
}

func newTaskClaimCommand(opts *GlobalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "claim <task-ref>",
		Short: "Acquire a task claim",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, db, manager, err := taskManagerFromOptions(cmd.Context(), opts)
			if err != nil {
				return err
			}
			defer db.Close()

			claim, err := manager.Claim(cmd.Context(), app.ClaimTaskRequest{
				Reference: args[0],
				Lease:     cfg.ClaimLease,
			})
			if err != nil {
				return err
			}

			if opts.JSON {
				return writeJSON(cmd, map[string]any{
					"ok":      true,
					"command": "task claim",
					"data": map[string]any{
						"claim": claim,
					},
					"meta": map[string]any{},
				})
			}

			_, err = fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\n", claim.TaskHandle, claim.ActorHandle, claim.Status)
			return err
		},
	}
}

func newTaskRenewCommand(opts *GlobalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "renew <task-ref>",
		Short: "Renew an active task claim",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, db, manager, err := taskManagerFromOptions(cmd.Context(), opts)
			if err != nil {
				return err
			}
			defer db.Close()

			claim, err := manager.RenewClaim(cmd.Context(), app.RenewClaimRequest{
				Reference: args[0],
				Lease:     cfg.ClaimLease,
			})
			if err != nil {
				return err
			}

			if opts.JSON {
				return writeJSON(cmd, map[string]any{
					"ok":      true,
					"command": "task renew",
					"data": map[string]any{
						"claim": claim,
					},
					"meta": map[string]any{},
				})
			}

			_, err = fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\n", claim.TaskHandle, claim.ActorHandle, claim.Status)
			return err
		},
	}
}

func newTaskStartCommand(opts *GlobalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "start <task-ref>",
		Short: "Start task time tracking",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, db, manager, err := taskManagerFromOptions(cmd.Context(), opts)
			if err != nil {
				return err
			}
			defer db.Close()

			session, err := manager.StartSession(cmd.Context(), app.StartTaskSessionRequest{
				Reference: args[0],
			})
			if err != nil {
				return err
			}

			if opts.JSON {
				return writeJSON(cmd, map[string]any{
					"ok":      true,
					"command": "task start",
					"data": map[string]any{
						"session": session,
					},
					"meta": map[string]any{},
				})
			}

			_, err = fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%d\n", session.TaskHandle, session.State, session.ElapsedSecond)
			return err
		},
	}
}

func newTaskPauseCommand(opts *GlobalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "pause <task-ref>",
		Short: "Pause task time tracking",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, db, manager, err := taskManagerFromOptions(cmd.Context(), opts)
			if err != nil {
				return err
			}
			defer db.Close()

			session, err := manager.PauseSession(cmd.Context(), app.PauseTaskSessionRequest{
				Reference: args[0],
			})
			if err != nil {
				return err
			}

			if opts.JSON {
				return writeJSON(cmd, map[string]any{
					"ok":      true,
					"command": "task pause",
					"data": map[string]any{
						"session": session,
					},
					"meta": map[string]any{},
				})
			}

			_, err = fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%d\n", session.TaskHandle, session.State, session.ElapsedSecond)
			return err
		},
	}
}

func newTaskResumeCommand(opts *GlobalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "resume <task-ref>",
		Short: "Resume task time tracking",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, db, manager, err := taskManagerFromOptions(cmd.Context(), opts)
			if err != nil {
				return err
			}
			defer db.Close()

			session, err := manager.ResumeSession(cmd.Context(), app.ResumeTaskSessionRequest{
				Reference: args[0],
			})
			if err != nil {
				return err
			}

			if opts.JSON {
				return writeJSON(cmd, map[string]any{
					"ok":      true,
					"command": "task resume",
					"data": map[string]any{
						"session": session,
					},
					"meta": map[string]any{},
				})
			}

			_, err = fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%d\n", session.TaskHandle, session.State, session.ElapsedSecond)
			return err
		},
	}
}

func newTaskReleaseCommand(opts *GlobalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "release <task-ref>",
		Short: "Release an active task claim",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, db, manager, err := taskManagerFromOptions(cmd.Context(), opts)
			if err != nil {
				return err
			}
			defer db.Close()

			if err := manager.ReleaseClaim(cmd.Context(), app.ReleaseClaimRequest{Reference: args[0]}); err != nil {
				return err
			}

			if opts.JSON {
				return writeJSON(cmd, map[string]any{
					"ok":      true,
					"command": "task release",
					"data":    map[string]any{},
					"meta":    map[string]any{},
				})
			}

			_, err = fmt.Fprintf(cmd.OutOrStdout(), "%s\treleased\n", args[0])
			return err
		},
	}
}

func newTaskUnlockCommand(opts *GlobalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "unlock <task-ref>",
		Short: "Manually unlock a task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, db, manager, err := taskManagerFromOptions(cmd.Context(), opts)
			if err != nil {
				return err
			}
			defer db.Close()

			if err := manager.Unlock(cmd.Context(), app.UnlockTaskRequest{Reference: args[0]}); err != nil {
				return err
			}

			if opts.JSON {
				return writeJSON(cmd, map[string]any{
					"ok":      true,
					"command": "task unlock",
					"data":    map[string]any{},
					"meta":    map[string]any{},
				})
			}

			_, err = fmt.Fprintf(cmd.OutOrStdout(), "%s\tunlocked\n", args[0])
			return err
		},
	}
}

func newTaskCloseCommand(opts *GlobalOptions) *cobra.Command {
	var status string

	cmd := &cobra.Command{
		Use:   "close <task-ref>",
		Short: "Close or reopen a task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if status == "" {
				status = "completed"
			}

			_, db, manager, err := taskManagerFromOptions(cmd.Context(), opts)
			if err != nil {
				return err
			}
			defer db.Close()

			task, err := manager.Update(cmd.Context(), app.UpdateTaskRequest{
				Reference: args[0],
				Status:    &status,
			})
			if err != nil {
				return err
			}

			if opts.JSON {
				return writeJSON(cmd, map[string]any{
					"ok":      true,
					"command": "task close",
					"data": map[string]any{
						"task": task,
					},
					"meta": map[string]any{},
				})
			}

			_, err = fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\n", task.Handle, task.Status, task.Title)
			return err
		},
	}

	cmd.Flags().StringVar(&status, "status", "", "Set the terminal or reopened task status")
	return cmd
}

func taskManagerFromOptions(ctx context.Context, opts *GlobalOptions) (taskconfig.Resolved, *sql.DB, app.TaskManager, error) {
	cfg, err := taskconfig.Resolve(taskconfig.Options{
		DBPathOverride: opts.DBPath,
		ActorOverride:  opts.Actor,
	})
	if err != nil {
		return taskconfig.Resolved{}, nil, app.TaskManager{}, err
	}

	db, err := taskdb.Open(ctx, cfg)
	if err != nil {
		return taskconfig.Resolved{}, nil, app.TaskManager{}, err
	}

	return cfg, db, app.TaskManager{
		DB:              db,
		HumanName:       cfg.HumanName,
		CurrentActorRef: cfg.Actor,
	}, nil
}

func splitCSV(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return []string{}
	}

	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			values = append(values, trimmed)
		}
	}
	return values
}

func promptAssigneeDecision(out io.Writer, in io.Reader, decision app.AssignmentDecisionRequiredError) (*string, bool, error) {
	target := "classification"
	if decision.ProjectHandle != nil {
		target = "project " + *decision.ProjectHandle
	} else if decision.DomainHandle != nil {
		target = "domain " + *decision.DomainHandle
	}

	defaultLabel := "the default assignee"
	if decision.DefaultAssigneeHandle != nil {
		defaultLabel = *decision.DefaultAssigneeHandle
	}

	if _, err := fmt.Fprintf(out, "Changing %s requires an assignee decision.\n[d] accept default (%s)\n[k] keep current assignee\n[a] enter a different assignee\nChoice: ", target, defaultLabel); err != nil {
		return nil, false, err
	}

	reader := bufio.NewReader(in)
	choice, err := reader.ReadString('\n')
	if err != nil {
		return nil, false, err
	}

	switch strings.ToLower(strings.TrimSpace(choice)) {
	case "d", "default", "":
		return nil, false, nil
	case "k", "keep":
		return nil, true, nil
	case "a", "assignee":
		if _, err := fmt.Fprint(out, "Assignee: "); err != nil {
			return nil, false, err
		}
		value, err := reader.ReadString('\n')
		if err != nil {
			return nil, false, err
		}
		trimmed := strings.TrimSpace(value)
		return &trimmed, false, nil
	default:
		return nil, false, fmt.Errorf("invalid assignee decision %q", strings.TrimSpace(choice))
	}
}
