package cli

import (
	"fmt"
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
		newTimeAddCommand(opts),
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

func newTimeAddCommand(opts *GlobalOptions) *cobra.Command {
	var durationRaw string
	var startedAtRaw string
	var note string

	cmd := &cobra.Command{
		Use:   "add <task-ref>",
		Short: "Add a manual time entry to a claimed task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, db, manager, err := taskManagerFromOptions(cmd.Context(), opts)
			if err != nil {
				return err
			}
			defer db.Close()

			duration, err := time.ParseDuration(durationRaw)
			if err != nil {
				return fmt.Errorf("parse --duration: %w", err)
			}
			startedAt, err := time.Parse(time.RFC3339, startedAtRaw)
			if err != nil {
				return fmt.Errorf("parse --started-at: %w", err)
			}

			entry, err := manager.AddManualTime(cmd.Context(), app.AddManualTimeRequest{
				Reference: args[0],
				Duration:  duration,
				StartedAt: &startedAt,
				Note:      note,
			})
			if err != nil {
				return err
			}

			if opts.JSON {
				return writeJSON(cmd, map[string]any{
					"ok":      true,
					"command": "grind time add",
					"data": map[string]any{
						"entry": entry,
					},
					"meta": map[string]any{},
				})
			}

			_, err = fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%d\t%s\n", entry.TaskHandle, entry.EntryID, entry.DurationSecond, entry.StartedAt)
			return err
		},
	}

	cmd.Flags().StringVar(&durationRaw, "duration", "", "Set the manual duration using Go duration syntax, for example 45m")
	cmd.Flags().StringVar(&startedAtRaw, "started-at", "", "Set the manual start timestamp in RFC3339 format")
	cmd.Flags().StringVar(&note, "note", "", "Set an optional note for the manual time entry")
	_ = cmd.MarkFlagRequired("duration")
	_ = cmd.MarkFlagRequired("started-at")
	return cmd
}

func newTimeEditCommand(opts *GlobalOptions) *cobra.Command {
	var durationRaw string
	var startedAtRaw string
	var note string

	cmd := &cobra.Command{
		Use:   "edit <task-ref> <entry-id>",
		Short: "Edit a manual time entry on a claimed task",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, db, manager, err := taskManagerFromOptions(cmd.Context(), opts)
			if err != nil {
				return err
			}
			defer db.Close()

			req := app.EditManualTimeRequest{
				Reference: args[0],
				EntryID:   args[1],
			}
			if cmd.Flags().Changed("duration") {
				duration, err := time.ParseDuration(durationRaw)
				if err != nil {
					return fmt.Errorf("parse --duration: %w", err)
				}
				req.Duration = &duration
			}
			if cmd.Flags().Changed("started-at") {
				startedAt, err := time.Parse(time.RFC3339, startedAtRaw)
				if err != nil {
					return fmt.Errorf("parse --started-at: %w", err)
				}
				req.StartedAt = &startedAt
			}
			if cmd.Flags().Changed("note") {
				req.Note = &note
			}
			if req.Duration == nil && req.StartedAt == nil && req.Note == nil {
				return fmt.Errorf("grind time edit requires at least one changed field")
			}

			entry, err := manager.EditManualTime(cmd.Context(), req)
			if err != nil {
				return err
			}

			if opts.JSON {
				return writeJSON(cmd, map[string]any{
					"ok":      true,
					"command": "grind time edit",
					"data": map[string]any{
						"entry": entry,
					},
					"meta": map[string]any{},
				})
			}

			_, err = fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%d\t%s\n", entry.TaskHandle, entry.EntryID, entry.DurationSecond, entry.StartedAt)
			return err
		},
	}

	cmd.Flags().StringVar(&durationRaw, "duration", "", "Update the manual duration using Go duration syntax, for example 1h15m")
	cmd.Flags().StringVar(&startedAtRaw, "started-at", "", "Update the manual start timestamp in RFC3339 format")
	cmd.Flags().StringVar(&note, "note", "", "Update the note on the manual time entry")
	return cmd
}
