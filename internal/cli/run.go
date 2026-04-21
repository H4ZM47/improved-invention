package cli

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/H4ZM47/improved-invention/internal/app"
	taskdb "github.com/H4ZM47/improved-invention/internal/db"
	"github.com/H4ZM47/improved-invention/internal/gitctx"
	"github.com/spf13/cobra"
)

type failureClass struct {
	Code     string
	ExitCode int
	Message  string
	Details  map[string]any
}

// Run executes the CLI with centralized exit-code and JSON error handling.
func Run(build BuildInfo, args []string, stdout io.Writer, stderr io.Writer) int {
	root, opts := newRootCommandWithOptions(build)
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.SetArgs(args)

	cmd, err := root.ExecuteC()
	if err == nil {
		return 0
	}

	commandName := "task"
	if cmd != nil && cmd.CommandPath() != "" {
		commandName = cmd.CommandPath()
	}

	jsonRequested := opts.JSON || argsContainFlag(args, "--json")
	failure := classifyFailure(err, commandName)

	if jsonRequested {
		_ = writeJSONTo(stdout, map[string]any{
			"ok":      false,
			"command": commandName,
			"error": map[string]any{
				"code":      failure.Code,
				"exit_code": failure.ExitCode,
				"message":   failure.Message,
				"details":   failure.Details,
			},
		})
		return failure.ExitCode
	}

	_, _ = fmt.Fprintln(stderr, failure.Message)
	return failure.ExitCode
}

func classifyFailure(err error, commandName string) failureClass {
	failure := failureClass{
		Code:     "INTERNAL_ERROR",
		ExitCode: 90,
		Message:  err.Error(),
		Details:  map[string]any{},
	}

	switch {
	case isInvalidArgsError(err):
		failure.Code, failure.ExitCode = "INVALID_ARGS", 10
	case isAssignmentDecisionError(err):
		failure.Code, failure.ExitCode = "ASSIGNMENT_DECISION_REQUIRED", 44
		failure.Details = assignmentDecisionDetails(err)
	case errors.Is(err, taskdb.ErrClaimRequired):
		failure.Code, failure.ExitCode = "CLAIM_REQUIRED", 30
	case errors.Is(err, taskdb.ErrClaimConflict):
		failure.Code, failure.ExitCode = "CLAIM_CONFLICT", 31
	case errors.Is(err, taskdb.ErrClaimExpired):
		failure.Code, failure.ExitCode = "CLAIM_EXPIRED", 32
	case errors.Is(err, taskdb.ErrClaimNotHeld):
		failure.Code, failure.ExitCode = "CLAIM_NOT_HELD_BY_ACTOR", 33
	case errors.Is(err, taskdb.ErrDomainProjectConstraint):
		failure.Code, failure.ExitCode = "DOMAIN_PROJECT_CONSTRAINT", 45
	case errors.Is(err, taskdb.ErrSavedViewNotFound):
		failure.Code, failure.ExitCode = "VIEW_NOT_FOUND", 60
	case errors.Is(err, taskdb.ErrInvalidRelationshipType), errors.Is(err, taskdb.ErrInvalidLinkType):
		failure.Code, failure.ExitCode = "INVALID_RELATIONSHIP", 41
	case errors.Is(err, gitctx.ErrNotGitRepo):
		failure.Code, failure.ExitCode = "VALIDATION_ERROR", 11
	case isFilterError(err):
		failure.Code, failure.ExitCode = "FILTER_INVALID", 61
	case isEntityNotFoundError(err):
		failure.Code, failure.ExitCode = "ENTITY_NOT_FOUND", 20
	case strings.HasPrefix(commandName, "task backup"):
		failure.Code, failure.ExitCode = "BACKUP_FAILED", 72
	case strings.HasPrefix(commandName, "task restore"):
		failure.Code, failure.ExitCode = "RESTORE_FAILED", 73
	case strings.HasPrefix(commandName, "task report"):
		failure.Code, failure.ExitCode = "REPORT_SERVER_FAILED", 71
	case strings.HasPrefix(commandName, "task export"):
		failure.Code, failure.ExitCode = "EXPORT_FAILED", 70
	case isMigrationError(err):
		failure.Code, failure.ExitCode = "MIGRATION_FAILED", 82
	case isDatabaseUnavailableError(err):
		failure.Code, failure.ExitCode = "DATABASE_UNAVAILABLE", 80
	case isDatabaseBusyError(err):
		failure.Code, failure.ExitCode = "DATABASE_BUSY", 81
	case isValidationError(err):
		failure.Code, failure.ExitCode = "VALIDATION_ERROR", 11
	}

	return failure
}

func isInvalidArgsError(err error) bool {
	if err == nil {
		return false
	}
	var cobraErr *cobra.Command
	_ = cobraErr
	msg := err.Error()
	return strings.Contains(msg, "unknown command") ||
		strings.Contains(msg, "unknown flag") ||
		strings.Contains(msg, "accepts ") ||
		strings.Contains(msg, "requires at least") ||
		strings.Contains(msg, "requires exactly") ||
		strings.Contains(msg, "argument")
}

func isAssignmentDecisionError(err error) bool {
	var decisionErr *app.AssignmentDecisionRequiredError
	return errors.As(err, &decisionErr)
}

func assignmentDecisionDetails(err error) map[string]any {
	var decisionErr *app.AssignmentDecisionRequiredError
	if !errors.As(err, &decisionErr) {
		return map[string]any{}
	}

	details := map[string]any{
		"choices": []string{"--accept-default-assignee", "--assignee <actor-ref>", "--keep-assignee"},
	}
	if decisionErr.TaskHandle != "" {
		details["task_handle"] = decisionErr.TaskHandle
	}
	if decisionErr.DomainHandle != nil {
		details["domain_handle"] = *decisionErr.DomainHandle
	}
	if decisionErr.ProjectHandle != nil {
		details["project_handle"] = *decisionErr.ProjectHandle
	}
	if decisionErr.DefaultAssigneeHandle != nil {
		details["default_assignee_handle"] = *decisionErr.DefaultAssigneeHandle
	}
	return details
}

func isEntityNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, sql.ErrNoRows) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, " not found") || strings.HasSuffix(msg, "not found")
}

func isFilterError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "invalid --status value") ||
		strings.Contains(msg, "parse --due-") ||
		strings.Contains(msg, "saved view filters must be valid JSON")
}

func isDatabaseUnavailableError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "open sqlite database") ||
		strings.Contains(msg, "ping sqlite database") ||
		strings.Contains(msg, "create database directory") ||
		strings.Contains(msg, "resolve user config dir")
}

func isMigrationError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "run migrations:")
}

func isDatabaseBusyError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "database is locked") || strings.Contains(msg, "busy timeout")
}

func isValidationError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "requires at least one changed field") ||
		strings.Contains(msg, "requires changes") ||
		strings.Contains(msg, "is required") ||
		strings.Contains(msg, "must differ") ||
		strings.Contains(msg, "already exists") ||
		strings.Contains(msg, "allows only one")
}

func argsContainFlag(args []string, flag string) bool {
	for _, arg := range args {
		if arg == flag || strings.HasPrefix(arg, flag+"=") {
			return true
		}
	}
	return false
}

func writeJSONTo(out io.Writer, payload map[string]any) error {
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	return encoder.Encode(payload)
}
