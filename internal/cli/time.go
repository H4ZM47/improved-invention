package cli

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/H4ZM47/grind/internal/app"
	"github.com/spf13/cobra"
)

func newTimeCommand(opts *GlobalOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "time",
		Short: "Manage manual time entries",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(
		newTimeStartCommand(opts),
		newTimePauseCommand(opts),
		newTimeResumeCommand(opts),
		newRetiredTimeAddCommand(),
		newTimeEditCommand(opts),
	)

	return cmd
}

func newTimeStartCommand(opts *GlobalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "start <task-ref>",
		Short: "Start task time tracking",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, db, manager, err := taskManagerFromOptions(cmd.Context(), opts)
			if err != nil {
				return err
			}
			defer func() {
				_ = db.Close()
			}()

			session, err := manager.StartSession(cmd.Context(), app.StartTaskSessionRequest{
				Reference: args[0],
			})
			if err != nil {
				return err
			}

			if opts.JSON {
				return writeJSON(cmd, map[string]any{
					"ok":      true,
					"command": "grind time start",
					"data": map[string]any{
						"session": session,
					},
					"meta": map[string]any{},
				})
			}

			_, err = fmt.Fprintf(cmd.OutOrStdout(), "task=%s\tstate=%s\telapsed_seconds=%d\n", session.TaskHandle, session.State, session.ElapsedSecond)
			return err
		},
	}
}

func newTimePauseCommand(opts *GlobalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "pause <task-ref>",
		Short: "Pause task time tracking",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, db, manager, err := taskManagerFromOptions(cmd.Context(), opts)
			if err != nil {
				return err
			}
			defer func() {
				_ = db.Close()
			}()

			session, err := manager.PauseSession(cmd.Context(), app.PauseTaskSessionRequest{
				Reference: args[0],
			})
			if err != nil {
				return err
			}

			if opts.JSON {
				return writeJSON(cmd, map[string]any{
					"ok":      true,
					"command": "grind time pause",
					"data": map[string]any{
						"session": session,
					},
					"meta": map[string]any{},
				})
			}

			_, err = fmt.Fprintf(cmd.OutOrStdout(), "task=%s\tstate=%s\telapsed_seconds=%d\n", session.TaskHandle, session.State, session.ElapsedSecond)
			return err
		},
	}
}

func newTimeResumeCommand(opts *GlobalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "resume <task-ref>",
		Short: "Resume task time tracking",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, db, manager, err := taskManagerFromOptions(cmd.Context(), opts)
			if err != nil {
				return err
			}
			defer func() {
				_ = db.Close()
			}()

			session, err := manager.ResumeSession(cmd.Context(), app.ResumeTaskSessionRequest{
				Reference: args[0],
			})
			if err != nil {
				return err
			}

			if opts.JSON {
				return writeJSON(cmd, map[string]any{
					"ok":      true,
					"command": "grind time resume",
					"data": map[string]any{
						"session": session,
					},
					"meta": map[string]any{},
				})
			}

			_, err = fmt.Fprintf(cmd.OutOrStdout(), "task=%s\tstate=%s\telapsed_seconds=%d\n", session.TaskHandle, session.State, session.ElapsedSecond)
			return err
		},
	}
}

func newRetiredTimeAddCommand() *cobra.Command {
	return &cobra.Command{
		Use:    "add <task-ref>",
		Short:  "Retired: use grind time edit",
		Hidden: true,
		Args:   cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return fmt.Errorf("`grind time add %s` was removed; use `grind time edit %s`", args[0], args[0])
		},
	}
}

func newTimeEditCommand(opts *GlobalOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "edit <task-ref>",
		Short: "Interactively edit or add a manual time entry on a claimed task",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 2 {
				return fmt.Errorf("`grind time edit %s %s` was removed; use `grind time edit %s`", args[0], args[1], args[0])
			}
			if opts.NoInput {
				return fmt.Errorf("`grind time edit` is interactive-only; omit `--no-input`")
			}

			_, db, manager, err := taskManagerFromOptions(cmd.Context(), opts)
			if err != nil {
				return err
			}
			defer func() {
				_ = db.Close()
			}()

			entries, err := manager.ListManualTime(cmd.Context(), app.ListManualTimeRequest{Reference: args[0]})
			if err != nil {
				return err
			}

			reader := bufio.NewReader(cmd.InOrStdin())
			selection, err := promptManualTimeSelection(cmd.ErrOrStderr(), reader, args[0], entries)
			if err != nil {
				return err
			}

			var entry app.ManualTimeEntryRecord
			switch {
			case selection.create:
				startedAt, duration, note, err := promptNewManualTimeValues(cmd.ErrOrStderr(), reader)
				if err != nil {
					return err
				}
				entry, err = manager.AddManualTime(cmd.Context(), app.AddManualTimeRequest{
					Reference: args[0],
					StartedAt: &startedAt,
					Duration:  duration,
					Note:      note,
				})
				if err != nil {
					return err
				}
			default:
				req, noChanges, err := promptEditManualTimeValues(cmd.ErrOrStderr(), reader, args[0], selection.entry)
				if err != nil {
					return err
				}
				if noChanges {
					if opts.JSON {
						return writeJSON(cmd, map[string]any{
							"ok":      true,
							"command": "grind time edit",
							"data": map[string]any{
								"entry": selection.entry,
							},
							"meta": map[string]any{
								"changed": false,
							},
						})
					}
					_, err = fmt.Fprintln(cmd.OutOrStdout(), "No changes applied.")
					return err
				}
				entry, err = manager.EditManualTime(cmd.Context(), req)
				if err != nil {
					return err
				}
			}

			if err != nil {
				return err
			}

			meta := map[string]any{}
			if selection.create {
				meta["mode"] = "created"
			} else {
				meta["mode"] = "edited"
			}

			if opts.JSON {
				return writeJSON(cmd, map[string]any{
					"ok":      true,
					"command": "grind time edit",
					"data": map[string]any{
						"entry": entry,
					},
					"meta": meta,
				})
			}

			if selection.create {
				_, err = fmt.Fprintf(cmd.OutOrStdout(), "Created %s\t%s\t%d\t%s\n", entry.TaskHandle, entry.EntryID, entry.DurationSecond, entry.StartedAt)
			} else {
				_, err = fmt.Fprintf(cmd.OutOrStdout(), "Updated %s\t%s\t%d\t%s\n", entry.TaskHandle, entry.EntryID, entry.DurationSecond, entry.StartedAt)
			}
			if err != nil {
				return err
			}
			return nil
		},
	}

	return cmd
}

type manualTimeSelection struct {
	create bool
	entry  app.ManualTimeEntryRecord
}

func promptManualTimeSelection(out io.Writer, reader *bufio.Reader, taskRef string, entries []app.ManualTimeEntryRecord) (manualTimeSelection, error) {
	if len(entries) == 0 {
		if _, err := fmt.Fprintf(out, "No manual time entries exist for %s. Creating a new entry.\n", taskRef); err != nil {
			return manualTimeSelection{}, err
		}
		return manualTimeSelection{create: true}, nil
	}

	if _, err := fmt.Fprintf(out, "Manual time entries for %s:\n", taskRef); err != nil {
		return manualTimeSelection{}, err
	}
	for index, entry := range entries {
		if _, err := fmt.Fprintf(out, "  %d) %s  %s  %s\n", index+1, entry.StartedAt, formatDurationSeconds(entry.DurationSecond), entry.Note); err != nil {
			return manualTimeSelection{}, err
		}
	}
	if _, err := fmt.Fprint(out, "  n) Add a new entry\nSelect an entry to edit or `n` to add a new one: "); err != nil {
		return manualTimeSelection{}, err
	}

	choice, err := reader.ReadString('\n')
	if err != nil {
		return manualTimeSelection{}, err
	}
	choice = strings.TrimSpace(choice)
	if strings.EqualFold(choice, "n") || strings.EqualFold(choice, "new") {
		return manualTimeSelection{create: true}, nil
	}

	index, err := strconv.Atoi(choice)
	if err != nil || index < 1 || index > len(entries) {
		return manualTimeSelection{}, fmt.Errorf("select a numbered entry or `n`")
	}
	return manualTimeSelection{entry: entries[index-1]}, nil
}

func promptNewManualTimeValues(out io.Writer, reader *bufio.Reader) (time.Time, time.Duration, string, error) {
	startedAtRaw, err := promptRequiredLine(out, reader, "Started at (RFC3339): ")
	if err != nil {
		return time.Time{}, 0, "", err
	}
	startedAt, err := time.Parse(time.RFC3339, startedAtRaw)
	if err != nil {
		return time.Time{}, 0, "", fmt.Errorf("parse started_at: %w", err)
	}

	durationRaw, err := promptRequiredLine(out, reader, "Duration (Go duration, for example 45m): ")
	if err != nil {
		return time.Time{}, 0, "", err
	}
	duration, err := time.ParseDuration(durationRaw)
	if err != nil {
		return time.Time{}, 0, "", fmt.Errorf("parse duration: %w", err)
	}

	note, err := promptOptionalLine(out, reader, "Note (optional): ")
	if err != nil {
		return time.Time{}, 0, "", err
	}
	return startedAt, duration, note, nil
}

func promptEditManualTimeValues(out io.Writer, reader *bufio.Reader, taskRef string, entry app.ManualTimeEntryRecord) (app.EditManualTimeRequest, bool, error) {
	if _, err := fmt.Fprintf(out, "Editing %s on %s\n", entry.EntryID, taskRef); err != nil {
		return app.EditManualTimeRequest{}, false, err
	}

	startedAtRaw, err := promptOptionalLine(out, reader, fmt.Sprintf("Started at [%s]: ", entry.StartedAt))
	if err != nil {
		return app.EditManualTimeRequest{}, false, err
	}
	durationRaw, err := promptOptionalLine(out, reader, fmt.Sprintf("Duration [%s]: ", formatDurationSeconds(entry.DurationSecond)))
	if err != nil {
		return app.EditManualTimeRequest{}, false, err
	}
	noteRaw, err := promptOptionalLine(out, reader, fmt.Sprintf("Note [%s]: ", entry.Note))
	if err != nil {
		return app.EditManualTimeRequest{}, false, err
	}

	req := app.EditManualTimeRequest{
		Reference: taskRef,
		EntryID:   entry.EntryID,
	}

	if strings.TrimSpace(startedAtRaw) != "" {
		startedAt, err := time.Parse(time.RFC3339, strings.TrimSpace(startedAtRaw))
		if err != nil {
			return app.EditManualTimeRequest{}, false, fmt.Errorf("parse started_at: %w", err)
		}
		req.StartedAt = &startedAt
	}
	if strings.TrimSpace(durationRaw) != "" {
		duration, err := time.ParseDuration(strings.TrimSpace(durationRaw))
		if err != nil {
			return app.EditManualTimeRequest{}, false, fmt.Errorf("parse duration: %w", err)
		}
		req.Duration = &duration
	}
	if strings.TrimSpace(noteRaw) != "" {
		note := strings.TrimSpace(noteRaw)
		req.Note = &note
	}

	noChanges := req.StartedAt == nil && req.Duration == nil && req.Note == nil
	return req, noChanges, nil
}

func promptRequiredLine(out io.Writer, reader *bufio.Reader, prompt string) (string, error) {
	if _, err := fmt.Fprint(out, prompt); err != nil {
		return "", err
	}
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return "", fmt.Errorf("value is required")
	}
	return line, nil
}

func promptOptionalLine(out io.Writer, reader *bufio.Reader, prompt string) (string, error) {
	if _, err := fmt.Fprint(out, prompt); err != nil {
		return "", err
	}
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

func formatDurationSeconds(seconds int64) string {
	return (time.Duration(seconds) * time.Second).String()
}
