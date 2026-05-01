package cli

import (
	"bufio"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/H4ZM47/grind/internal/app"
	taskconfig "github.com/H4ZM47/grind/internal/config"
	taskdb "github.com/H4ZM47/grind/internal/db"
	"github.com/H4ZM47/grind/internal/gitctx"
	"github.com/spf13/cobra"
)

func newTaskCreateCommand(opts *GlobalOptions) *cobra.Command {
	var description string
	var descriptionFile string
	var tags string
	var domain string
	var project string
	var milestone string
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
			defer func() {
				_ = db.Close()
			}()

			resolvedDescription, err := resolveDescriptionInput(
				cmd,
				opts,
				description,
				descriptionFile,
				descriptionPromptConfig{
					Enabled: true,
					Title:   args[0],
				},
			)
			if err != nil {
				return err
			}

			task, err := manager.Create(cmd.Context(), app.CreateTaskRequest{
				Title:        args[0],
				Description:  resolvedDescription,
				Tags:         splitCSV(tags),
				DomainRef:    optionalString(cmd, "domain", domain),
				ProjectRef:   optionalString(cmd, "project", project),
				MilestoneRef: optionalString(cmd, "milestone", milestone),
				AssigneeRef:  optionalString(cmd, "assignee", assignee),
				DueAt:        optionalString(cmd, "due-at", dueAt),
			})
			if err != nil {
				return err
			}

			if opts.JSON {
				return writeJSON(cmd, map[string]any{
					"ok":      true,
					"command": "grind create",
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
	cmd.Flags().StringVar(&descriptionFile, "description-file", "", "Read the task description from a file path")
	cmd.Flags().StringVar(&tags, "tags", "", "Set comma-separated task tags")
	cmd.Flags().StringVar(&domain, "domain", "", "Set the task domain reference")
	cmd.Flags().StringVar(&project, "project", "", "Set the task project reference")
	cmd.Flags().StringVar(&milestone, "milestone", "", "Set the task milestone reference")
	cmd.Flags().StringVar(&assignee, "assignee", "", "Set the task assignee")
	cmd.Flags().StringVar(&dueAt, "due-at", "", "Set the task due timestamp")
	return cmd
}

func newTaskListCommand(opts *GlobalOptions) *cobra.Command {
	var statuses []string
	var tags []string
	var domain string
	var project string
	var milestone string
	var assignee string
	var dueBefore string
	var dueAfter string
	var search string
	var here bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List tasks",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, db, manager, err := taskManagerFromOptions(cmd.Context(), opts)
			if err != nil {
				return err
			}
			defer func() {
				_ = db.Close()
			}()

			if err := validateTaskListStatuses(statuses); err != nil {
				return err
			}
			normalizedDueBefore, err := normalizeRFC3339Flag("due-before", dueBefore, cmd.Flags().Changed("due-before"))
			if err != nil {
				return err
			}
			normalizedDueAfter, err := normalizeRFC3339Flag("due-after", dueAfter, cmd.Flags().Changed("due-after"))
			if err != nil {
				return err
			}
			if normalizedDueBefore != "" {
				dueBefore = normalizedDueBefore
			}
			if normalizedDueAfter != "" {
				dueAfter = normalizedDueAfter
			}

			var repoTarget *string
			var worktreeTarget *string
			if here {
				cwd, err := os.Getwd()
				if err != nil {
					return err
				}
				current, err := gitctx.Detect(cmd.Context(), cwd)
				if err != nil {
					return err
				}
				repoValue := current.RepoTarget()
				worktreeValue := current.WorktreeTarget()
				repoTarget = &repoValue
				worktreeTarget = &worktreeValue
			}

			items, err := manager.List(cmd.Context(), app.ListTasksRequest{
				Statuses:       statuses,
				DomainRef:      optionalString(cmd, "domain", domain),
				ProjectRef:     optionalString(cmd, "project", project),
				MilestoneRef:   optionalString(cmd, "milestone", milestone),
				AssigneeRef:    optionalString(cmd, "assignee", assignee),
				DueBefore:      optionalString(cmd, "due-before", dueBefore),
				DueAfter:       optionalString(cmd, "due-after", dueAfter),
				Tags:           tags,
				Search:         search,
				RepoTarget:     repoTarget,
				WorktreeTarget: worktreeTarget,
			})
			if err != nil {
				return err
			}

			if opts.JSON {
				filters := taskListFiltersMeta(statuses, tags, domain, project, milestone, assignee, dueBefore, dueAfter, search)
				if here {
					filters["here"] = true
				}
				return writeJSON(cmd, map[string]any{
					"ok":      true,
					"command": "grind list",
					"data": map[string]any{
						"items": items,
					},
					"meta": map[string]any{
						"count":   len(items),
						"filters": filters,
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

	cmd.Flags().StringArrayVar(&statuses, "status", nil, "Filter by task status; repeat to allow multiple statuses")
	cmd.Flags().StringArrayVar(&tags, "tag", nil, "Filter by task tag; repeat to require multiple tags")
	cmd.Flags().StringVar(&domain, "domain", "", "Filter by domain reference")
	cmd.Flags().StringVar(&project, "project", "", "Filter by project reference")
	cmd.Flags().StringVar(&milestone, "milestone", "", "Filter by milestone reference")
	cmd.Flags().StringVar(&assignee, "assignee", "", "Filter by task assignee reference")
	cmd.Flags().StringVar(&dueBefore, "due-before", "", "Filter to tasks due on or before the RFC3339 timestamp")
	cmd.Flags().StringVar(&dueAfter, "due-after", "", "Filter to tasks due on or after the RFC3339 timestamp")
	cmd.Flags().StringVar(&search, "search", "", "Filter by case-insensitive search across title and description")
	cmd.Flags().BoolVar(&here, "here", false, "Filter to tasks linked to the current repo or worktree context")

	return cmd
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
			defer func() {
				_ = db.Close()
			}()

			task, err := manager.Show(cmd.Context(), app.ShowTaskRequest{Reference: args[0]})
			if err != nil {
				return err
			}

			if opts.JSON {
				return writeJSON(cmd, map[string]any{
					"ok":      true,
					"command": "grind show",
					"data": map[string]any{
						"task": task,
					},
					"meta": map[string]any{},
				})
			}

			_, err = fmt.Fprintf(
				cmd.OutOrStdout(),
				"handle=%s\nstatus=%s\ntitle=%s\ndescription=%s\nmilestone=%s\n",
				task.Handle,
				task.Status,
				task.Title,
				task.Description,
				stringOrEmpty(task.MilestoneHandle),
			)
			return err
		},
	}
}

func newTaskUpdateCommand(opts *GlobalOptions) *cobra.Command {
	var title string
	var description string
	var descriptionFile string
	var tags string
	var domain string
	var project string
	var milestone string
	var assignee string
	var dueAt string
	var status string
	var clearMilestone bool
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
			defer func() {
				_ = db.Close()
			}()

			if keepAssignee && acceptDefaultAssignee {
				return fmt.Errorf("grind update allows only one of --keep-assignee or --accept-default-assignee")
			}
			if cmd.Flags().Changed("assignee") && (keepAssignee || acceptDefaultAssignee) {
				return fmt.Errorf("grind update allows only one explicit assignee decision")
			}
			if clearMilestone && cmd.Flags().Changed("milestone") {
				return fmt.Errorf("grind update allows only one of --milestone or --clear-milestone")
			}

			resolvedDescription, err := resolveDescriptionInput(
				cmd,
				opts,
				description,
				descriptionFile,
				descriptionPromptConfig{},
			)
			if err != nil {
				return err
			}

			req := app.UpdateTaskRequest{Reference: args[0]}
			if cmd.Flags().Changed("title") {
				req.Title = &title
			}
			if cmd.Flags().Changed("description") || cmd.Flags().Changed("description-file") {
				req.Description = &resolvedDescription
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
			if cmd.Flags().Changed("milestone") {
				req.MilestoneRef = &milestone
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
			req.ClearMilestone = clearMilestone
			req.AcceptDefaultAssignee = acceptDefaultAssignee
			req.KeepAssignee = keepAssignee

			if req.Title == nil && req.Description == nil && req.Tags == nil && req.DomainRef == nil && req.ProjectRef == nil && req.MilestoneRef == nil && req.AssigneeRef == nil && req.DueAt == nil && req.Status == nil && !req.ClearMilestone && !req.AcceptDefaultAssignee && !req.KeepAssignee {
				return fmt.Errorf("grind update requires at least one changed field")
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
					"command": "grind update",
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
	cmd.Flags().StringVar(&descriptionFile, "description-file", "", "Read the task description from a file path")
	cmd.Flags().StringVar(&tags, "tags", "", "Set comma-separated task tags")
	cmd.Flags().StringVar(&domain, "domain", "", "Set the task domain reference")
	cmd.Flags().StringVar(&project, "project", "", "Set the task project reference")
	cmd.Flags().StringVar(&milestone, "milestone", "", "Set the task milestone reference")
	cmd.Flags().BoolVar(&clearMilestone, "clear-milestone", false, "Remove the task milestone assignment")
	cmd.Flags().StringVar(&assignee, "assignee", "", "Set the task assignee")
	cmd.Flags().StringVar(&dueAt, "due-at", "", "Set the task due timestamp")
	cmd.Flags().StringVar(&status, "status", "", "Set the task status")
	cmd.Flags().BoolVar(&acceptDefaultAssignee, "accept-default-assignee", false, "Accept the inherited default assignee during reclassification")
	cmd.Flags().BoolVar(&keepAssignee, "keep-assignee", false, "Keep the current assignee during reclassification")

	return cmd
}

func newClaimCommand(opts *GlobalOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "claim",
		Short: "Manage task claims",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 {
				return fmt.Errorf("`grind claim %s` was removed; use `grind claim acquire %s`", args[0], args[0])
			}
			return cmd.Help()
		},
	}
	cmd.AddCommand(
		newClaimAcquireCommand(opts),
		newClaimRenewCommand(opts),
		newClaimReleaseCommand(opts),
		newClaimUnlockCommand(opts),
	)
	return cmd
}

func newClaimAcquireCommand(opts *GlobalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "acquire <task-ref>",
		Short: "Claim a task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, db, manager, err := taskManagerFromOptions(cmd.Context(), opts)
			if err != nil {
				return err
			}
			defer func() {
				_ = db.Close()
			}()

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
					"command": "grind claim acquire",
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

func newClaimRenewCommand(opts *GlobalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "renew <task-ref>",
		Short: "Renew an active task claim",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, db, manager, err := taskManagerFromOptions(cmd.Context(), opts)
			if err != nil {
				return err
			}
			defer func() {
				_ = db.Close()
			}()

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
					"command": "grind claim renew",
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

func newClaimReleaseCommand(opts *GlobalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "release <task-ref>",
		Short: "Release an active task claim",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, db, manager, err := taskManagerFromOptions(cmd.Context(), opts)
			if err != nil {
				return err
			}
			defer func() {
				_ = db.Close()
			}()

			if err := manager.ReleaseClaim(cmd.Context(), app.ReleaseClaimRequest{Reference: args[0]}); err != nil {
				return err
			}

			if opts.JSON {
				return writeJSON(cmd, map[string]any{
					"ok":      true,
					"command": "grind claim release",
					"data":    map[string]any{},
					"meta":    map[string]any{},
				})
			}

			_, err = fmt.Fprintf(cmd.OutOrStdout(), "%s\treleased\n", args[0])
			return err
		},
	}
}

func newClaimUnlockCommand(opts *GlobalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "unlock <task-ref>",
		Short: "Manually unlock a task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, db, manager, err := taskManagerFromOptions(cmd.Context(), opts)
			if err != nil {
				return err
			}
			defer func() {
				_ = db.Close()
			}()

			if err := manager.Unlock(cmd.Context(), app.UnlockTaskRequest{Reference: args[0]}); err != nil {
				return err
			}

			if opts.JSON {
				return writeJSON(cmd, map[string]any{
					"ok":      true,
					"command": "grind claim unlock",
					"data":    map[string]any{},
					"meta":    map[string]any{},
				})
			}

			_, err = fmt.Fprintf(cmd.OutOrStdout(), "%s\tunlocked\n", args[0])
			return err
		},
	}
}

func newTaskOpenCommand(opts *GlobalOptions) *cobra.Command {
	return newTaskStatusCommand(opts, "open", "Reopen a task", taskdb.StatusBacklog)
}

func newTaskCloseCommand(opts *GlobalOptions) *cobra.Command {
	return newTaskStatusCommand(opts, "close", "Close a task", taskdb.StatusCompleted)
}

func newTaskCancelCommand(opts *GlobalOptions) *cobra.Command {
	return newTaskStatusCommand(opts, "cancel", "Cancel a task", taskdb.StatusCancelled)
}

func newTaskStatusCommand(opts *GlobalOptions, use, short, statusValue string) *cobra.Command {
	return newLifecycleStatusCommand(opts, lifecycleStatusCommandConfig{
		Use:             use,
		Short:           short,
		RefName:         "task-ref",
		CommandName:     "grind " + use,
		StatusValue:     statusValue,
		RetiredFlagHelp: "Retired task lifecycle flag; use the explicit open, close, or cancel verbs",
		MigrationError:  taskLifecycleMigrationError,
		Run: func(cmd *cobra.Command, reference string, status string) (lifecycleStatusResult, error) {
			_, db, manager, err := taskManagerFromOptions(cmd.Context(), opts)
			if err != nil {
				return lifecycleStatusResult{}, err
			}
			defer func() {
				_ = db.Close()
			}()

			task, err := manager.Update(cmd.Context(), app.UpdateTaskRequest{
				Reference: reference,
				Status:    &status,
			})
			if err != nil {
				return lifecycleStatusResult{}, err
			}
			return lifecycleStatusResult{
				DataKey: "task",
				Data:    task,
				Line:    fmt.Sprintf("%s\t%s\t%s\n", task.Handle, task.Status, task.Title),
			}, nil
		},
	})
}

func taskLifecycleMigrationError(use, handle, status string) error {
	switch status {
	case taskdb.StatusBacklog:
		return fmt.Errorf("`grind close %s --status backlog` was removed; use `grind open %s`", handle, handle)
	case taskdb.StatusCompleted:
		if use != "close" {
			return fmt.Errorf("`grind close %s --status completed` was removed; use `grind close %s`", handle, handle)
		}
		return fmt.Errorf("`grind %s %s --status completed` was removed; use `grind close %s`", use, handle, handle)
	case taskdb.StatusCancelled:
		return fmt.Errorf("`grind close %s --status cancelled` was removed; use `grind cancel %s`", handle, handle)
	default:
		return fmt.Errorf("the `--status` flag was removed from `grind %s`; use `grind open`, `grind close`, or `grind cancel`", use)
	}
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

type descriptionPromptConfig struct {
	Enabled bool
	Title   string
}

func resolveDescriptionInput(cmd *cobra.Command, opts *GlobalOptions, literal string, filePath string, prompt descriptionPromptConfig) (string, error) {
	descriptionFlagChanged := cmd.Flags().Changed("description")
	descriptionFileChanged := cmd.Flags().Changed("description-file")

	switch {
	case descriptionFlagChanged && descriptionFileChanged:
		return "", fmt.Errorf("use only one of --description or --description-file")
	case descriptionFileChanged:
		return readDescriptionFile(filePath)
	case descriptionFlagChanged:
		return literal, nil
	case prompt.Enabled && shouldPromptForDescription(cmd, opts):
		return promptTaskDescription(cmd.ErrOrStderr(), cmd.InOrStdin(), prompt.Title)
	default:
		return literal, nil
	}
}

func readDescriptionFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read --description-file %s: %w", path, err)
	}
	return string(data), nil
}

func shouldPromptForDescription(cmd *cobra.Command, opts *GlobalOptions) bool {
	if opts.NoInput {
		return false
	}

	inputFile, ok := cmd.InOrStdin().(*os.File)
	if !ok {
		return false
	}
	outputFile, ok := cmd.ErrOrStderr().(*os.File)
	if !ok {
		return false
	}

	inputInfo, err := inputFile.Stat()
	if err != nil || (inputInfo.Mode()&os.ModeCharDevice) == 0 {
		return false
	}
	outputInfo, err := outputFile.Stat()
	if err != nil || (outputInfo.Mode()&os.ModeCharDevice) == 0 {
		return false
	}

	return true
}

func promptTaskDescription(out io.Writer, in io.Reader, title string) (string, error) {
	if _, err := fmt.Fprintf(out, "Add a description for %q? Press Enter to skip.\nDescription (finish with an empty line):\n", title); err != nil {
		return "", err
	}

	reader := bufio.NewReader(in)
	lines := make([]string, 0, 4)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				trimmed := strings.TrimRight(line, "\r\n")
				if trimmed != "" {
					lines = append(lines, trimmed)
				}
				break
			}
			return "", err
		}

		trimmed := strings.TrimRight(line, "\r\n")
		if trimmed == "" {
			break
		}
		lines = append(lines, trimmed)
	}

	return strings.Join(lines, "\n"), nil
}

func validateTaskListStatuses(statuses []string) error {
	for _, status := range statuses {
		trimmed := strings.TrimSpace(status)
		if trimmed == "" {
			continue
		}
		if !taskdb.IsValidStatus(trimmed) {
			return fmt.Errorf("invalid --status value %q", trimmed)
		}
	}
	return nil
}

func normalizeRFC3339Flag(name string, value string, changed bool) (string, error) {
	if !changed {
		return "", nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return "", fmt.Errorf("parse --%s: %w", name, err)
	}
	return parsed.UTC().Format(time.RFC3339), nil
}

func taskListFiltersMeta(statuses []string, tags []string, domain string, project string, milestone string, assignee string, dueBefore string, dueAfter string, search string) map[string]any {
	filters := map[string]any{}

	if values := splitNonEmpty(statuses); len(values) > 0 {
		filters["status"] = values
	}
	if values := splitNonEmpty(tags); len(values) > 0 {
		filters["tags"] = values
	}
	if strings.TrimSpace(domain) != "" {
		filters["domain"] = strings.TrimSpace(domain)
	}
	if strings.TrimSpace(project) != "" {
		filters["project"] = strings.TrimSpace(project)
	}
	if strings.TrimSpace(milestone) != "" {
		filters["milestone"] = strings.TrimSpace(milestone)
	}
	if strings.TrimSpace(assignee) != "" {
		filters["assignee"] = strings.TrimSpace(assignee)
	}
	if strings.TrimSpace(dueBefore) != "" {
		filters["due_before"] = strings.TrimSpace(dueBefore)
	}
	if strings.TrimSpace(dueAfter) != "" {
		filters["due_after"] = strings.TrimSpace(dueAfter)
	}
	if strings.TrimSpace(search) != "" {
		filters["search"] = strings.TrimSpace(search)
	}

	return filters
}

func splitNonEmpty(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func stringOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
