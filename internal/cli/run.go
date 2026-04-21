package cli

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"

	"github.com/H4ZM47/grind/internal/app"
	taskdb "github.com/H4ZM47/grind/internal/db"
	"github.com/H4ZM47/grind/internal/gitctx"
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
	applyRootFlagOverridesFromArgs(opts, args)
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.SetArgs(args)

	jsonRequested := opts.JSON || argsContainFlag(args, "--json")
	if argsContainFlag(args, "--version") {
		if err := writeVersionInfo(stdout, build, jsonRequested); err != nil {
			return writeFailure(stdout, stderr, jsonRequested, "grind --version", err)
		}
		return 0
	}
	if argsContainFlag(args, "--config") {
		if err := writeConfigInfo(stdout, opts); err != nil {
			return writeFailure(stdout, stderr, jsonRequested, "grind --config", err)
		}
		return 0
	}
	if argsContainFlag(args, "--agents") || argsContainFlag(args, "--agent-help") {
		if err := writeAgentInstructions(stdout, jsonRequested); err != nil {
			return writeFailure(stdout, stderr, jsonRequested, "grind --agents", err)
		}
		return 0
	}

	cmd, err := root.ExecuteC()
	if err == nil {
		return 0
	}

	commandName := "grind"
	if cmd != nil && cmd.CommandPath() != "" {
		commandName = cmd.CommandPath()
	}

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

func writeFailure(stdout io.Writer, stderr io.Writer, jsonRequested bool, commandName string, err error) int {
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
	} else {
		_, _ = fmt.Fprintln(stderr, failure.Message)
	}
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
		failure.Message = normalizeEntityNotFoundMessage(err, commandName)
	case strings.HasPrefix(commandName, "grind backup"):
		failure.Code, failure.ExitCode = "BACKUP_FAILED", 72
	case strings.HasPrefix(commandName, "grind restore"):
		failure.Code, failure.ExitCode = "RESTORE_FAILED", 73
	case strings.HasPrefix(commandName, "grind serve"), strings.HasPrefix(commandName, "grind report"):
		failure.Code, failure.ExitCode = "REPORT_SERVER_FAILED", 71
	case strings.HasPrefix(commandName, "grind export"):
		failure.Code, failure.ExitCode = "EXPORT_FAILED", 70
	case isMigrationError(err):
		failure.Code, failure.ExitCode = "MIGRATION_FAILED", 82
	case isDatabaseBusyError(err):
		failure.Code, failure.ExitCode = "DATABASE_BUSY", 81
	case isDatabaseUnavailableError(err):
		failure.Code, failure.ExitCode = "DATABASE_UNAVAILABLE", 80
	case isValidationError(err):
		failure.Code, failure.ExitCode = "VALIDATION_ERROR", 11
	}

	failure.Message = normalizeFailureMessage(failure.Code, err, commandName)

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
		strings.Contains(msg, "required flag(s)") ||
		strings.Contains(msg, "accepts ") ||
		strings.Contains(msg, "requires at least") ||
		strings.Contains(msg, "requires exactly") ||
		strings.Contains(msg, "requires --") ||
		strings.Contains(msg, "was removed; use") ||
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

var findEntityNoRowsPattern = regexp.MustCompile(`^find ([a-z_ ]+) "([^"]+)": sql: no rows in result set$`)

func normalizeEntityNotFoundMessage(err error, commandName string) string {
	if err == nil {
		return "requested record was not found"
	}

	msg := strings.TrimSpace(err.Error())
	if matches := findEntityNoRowsPattern.FindStringSubmatch(msg); len(matches) == 3 {
		return fmt.Sprintf("%s %q was not found", strings.TrimSpace(matches[1]), matches[2])
	}

	if errors.Is(err, taskdb.ErrSavedViewNotFound) {
		if idx := strings.LastIndex(msg, ":"); idx >= 0 && idx+1 < len(msg) {
			return fmt.Sprintf("saved view %s was not found", strings.TrimSpace(msg[idx+1:]))
		}
		return "saved view was not found"
	}

	switch {
	case strings.HasPrefix(commandName, "grind link remove"):
		return "link was not found"
	case strings.HasPrefix(commandName, "grind relationship remove"):
		return "relationship was not found"
	}

	if errors.Is(err, sql.ErrNoRows) {
		return "requested record was not found"
	}

	if lower := strings.ToLower(msg); strings.Contains(lower, "not found") {
		return msg
	}

	return "requested record was not found"
}

func normalizeFailureMessage(code string, err error, commandName string) string {
	switch code {
	case "INVALID_ARGS":
		return normalizeInvalidArgsMessage(err)
	case "ASSIGNMENT_DECISION_REQUIRED":
		return err.Error()
	case "CLAIM_REQUIRED":
		return "this action requires an active claim on the task; claim it first"
	case "CLAIM_CONFLICT":
		return "this task is already claimed by another actor"
	case "CLAIM_EXPIRED":
		return "the active claim has expired; claim the task again"
	case "CLAIM_NOT_HELD_BY_ACTOR":
		return "you do not hold the active claim on this task"
	case "DOMAIN_PROJECT_CONSTRAINT":
		return normalizeDomainProjectConstraintMessage(err)
	case "VIEW_NOT_FOUND", "ENTITY_NOT_FOUND":
		return normalizeEntityNotFoundMessage(err, commandName)
	case "INVALID_RELATIONSHIP":
		return normalizeInvalidRelationshipMessage(err)
	case "FILTER_INVALID":
		return normalizeFilterMessage(err)
	case "VALIDATION_ERROR":
		return normalizeValidationMessage(err, commandName)
	case "BACKUP_FAILED":
		return normalizeBackupRestoreMessage(err, "backup")
	case "RESTORE_FAILED":
		return normalizeBackupRestoreMessage(err, "restore")
	case "REPORT_SERVER_FAILED":
		return normalizeReportMessage(err)
	case "EXPORT_FAILED":
		return normalizeExportMessage(err)
	case "MIGRATION_FAILED":
		return "the Grind database schema could not be prepared; inspect the migration error and try again"
	case "DATABASE_UNAVAILABLE":
		return normalizeDatabaseUnavailableMessage(err)
	case "DATABASE_BUSY":
		return "the Grind database is busy or locked; wait a moment and try again"
	case "INTERNAL_ERROR":
		return normalizeInternalErrorMessage(err)
	default:
		if err == nil {
			return "the command failed"
		}
		return err.Error()
	}
}

func normalizeInvalidArgsMessage(err error) string {
	if err == nil {
		return "the command arguments are invalid"
	}
	return err.Error()
}

func normalizeDomainProjectConstraintMessage(err error) string {
	if err == nil {
		return "the selected project and domain do not form a valid combination"
	}
	msg := strings.TrimSpace(err.Error())
	msg = strings.TrimPrefix(msg, taskdb.ErrDomainProjectConstraint.Error()+":")
	msg = strings.TrimSpace(msg)
	if msg == "" {
		return "the selected project and domain do not form a valid combination"
	}
	if strings.Contains(msg, "project domain") && strings.Contains(msg, "not found") {
		return msg
	}
	if strings.Contains(msg, "belongs to domain") {
		return msg
	}
	return msg
}

func normalizeInvalidRelationshipMessage(err error) string {
	if err == nil {
		return "the requested relationship or link type is not supported"
	}
	msg := strings.TrimSpace(err.Error())
	supportedTaskTypes := strings.Join(sortedKeys([]string{
		"parent", "child", "blocks", "blocked_by", "related", "sibling", "duplicate", "supersedes",
	}), ", ")
	supportedResourceTypes := strings.Join(sortedKeys([]string{
		taskdb.LinkTypeFile, taskdb.LinkTypeURL, taskdb.LinkTypeRepo, taskdb.LinkTypeWorktree, taskdb.LinkTypeObsidian, taskdb.LinkTypeOther,
	}), ", ")
	switch {
	case errors.Is(err, taskdb.ErrInvalidRelationshipType):
		return fmt.Sprintf("unsupported link type; task links use one of: %s; resource links use one of: %s", supportedTaskTypes, supportedResourceTypes)
	case errors.Is(err, taskdb.ErrInvalidLinkType):
		return fmt.Sprintf("unsupported link type; task links use one of: %s; resource links use one of: %s", supportedTaskTypes, supportedResourceTypes)
	default:
		return msg
	}
}

func normalizeFilterMessage(err error) string {
	if err == nil {
		return "one or more filters are invalid"
	}
	msg := strings.TrimSpace(err.Error())
	switch {
	case strings.Contains(msg, "invalid --status value"):
		return msg + "; use one of: backlog, active, paused, blocked, completed, cancelled"
	case strings.Contains(msg, "parse --due-before"):
		return "`--due-before` must be a valid RFC3339 timestamp"
	case strings.Contains(msg, "parse --due-after"):
		return "`--due-after` must be a valid RFC3339 timestamp"
	case strings.Contains(msg, "saved view filters must be valid JSON"):
		return "saved view filters must be valid JSON"
	default:
		return msg
	}
}

func normalizeValidationMessage(err error, commandName string) string {
	if err == nil {
		return "the command input is invalid"
	}
	msg := strings.TrimSpace(err.Error())

	switch {
	case errors.Is(err, gitctx.ErrNotGitRepo):
		return "this command must be run inside a git repository"
	case strings.Contains(msg, "read --description-file"):
		return msg
	case strings.Contains(msg, "invalid task status transition from "):
		matches := regexp.MustCompile(`invalid task status transition from ([^ ]+) to ([^ ]+)`).FindStringSubmatch(msg)
		if len(matches) == 3 {
			return fmt.Sprintf("cannot change status directly from %s to %s", matches[1], matches[2])
		}
	case errors.Is(err, taskdb.ErrSessionActive):
		return "a task session is already active"
	case errors.Is(err, taskdb.ErrSessionNotActive):
		return "there is no active task session to update"
	case errors.Is(err, taskdb.ErrSessionNotPaused):
		return "the task session is not paused"
	case errors.Is(err, taskdb.ErrSessionOnClosedTask):
		return "time tracking cannot start or resume on a closed task"
	case errors.Is(err, taskdb.ErrInvalidManualTimeEntry):
		return strings.TrimSpace(strings.TrimPrefix(msg, taskdb.ErrInvalidManualTimeEntry.Error()+":"))
	case errors.Is(err, taskdb.ErrManualTimeEditEmpty):
		return "manual time edit requires at least one changed field"
	case errors.Is(err, taskdb.ErrManualTimeEntryNotFound):
		return "manual time entry was not found"
	case strings.Contains(msg, "parse GRIND_BUSY_TIMEOUT_MS"):
		return "GRIND_BUSY_TIMEOUT_MS must be a positive integer number of milliseconds"
	case strings.Contains(msg, "parse GRIND_CLAIM_LEASE_HOURS"):
		return "GRIND_CLAIM_LEASE_HOURS must be a positive integer number of hours"
	}

	return msg
}

func normalizeBackupRestoreMessage(err error, mode string) string {
	if err == nil {
		return fmt.Sprintf("%s failed", mode)
	}
	msg := strings.TrimSpace(err.Error())
	switch {
	case strings.Contains(msg, "output path is required"):
		return "you must provide an output path"
	case strings.Contains(msg, "input path is required"):
		return "you must provide an input path"
	case strings.Contains(msg, "target database path is required"):
		return "the target database path is required"
	case strings.Contains(msg, "must differ"):
		return "the restore input and target database paths must be different"
	default:
		return msg
	}
}

func normalizeReportMessage(err error) string {
	if err == nil {
		return "the report server failed"
	}
	msg := strings.TrimSpace(err.Error())
	if strings.Contains(msg, "not found") {
		return msg
	}
	if strings.Contains(msg, "build report server") {
		return "the report server could not be prepared"
	}
	if strings.Contains(msg, "start report server on") {
		return msg
	}
	if strings.Contains(msg, "serve report server") {
		return "the report server stopped unexpectedly"
	}
	return msg
}

func normalizeExportMessage(err error) string {
	if err == nil {
		return "the export failed"
	}
	return strings.TrimSpace(err.Error())
}

func normalizeDatabaseUnavailableMessage(err error) string {
	if err == nil {
		return "the Grind database is unavailable"
	}
	msg := strings.TrimSpace(err.Error())
	switch {
	case strings.Contains(msg, "resolve user config dir"):
		return "could not resolve the local Grind config directory"
	case strings.Contains(msg, "create database directory"):
		return "could not create the local Grind data directory"
	case strings.Contains(msg, "open sqlite database"):
		return "could not open the Grind database"
	case strings.Contains(msg, "ping sqlite database"):
		return "could not connect to the Grind database"
	default:
		return msg
	}
}

func normalizeInternalErrorMessage(err error) string {
	if err == nil {
		return "the command failed unexpectedly"
	}
	msg := strings.TrimSpace(err.Error())
	switch {
	case strings.Contains(msg, "UNIQUE constraint failed: links.target_task_id"):
		return "that task already has a parent"
	case strings.Contains(msg, "UNIQUE constraint failed: saved_views.name"):
		return "a saved view with that name already exists"
	case strings.Contains(msg, "UNIQUE constraint failed: claims.task_id"):
		return "this task is already claimed by another actor"
	case strings.Contains(msg, "invalid metadata json"):
		return "link metadata must be valid JSON"
	case strings.Contains(msg, "begin ") && strings.Contains(msg, " transaction"):
		return "the database operation could not start"
	case strings.Contains(msg, "commit ") && strings.Contains(msg, " transaction"):
		return "the database operation could not be completed"
	default:
		return msg
	}
}

func sortedKeys(values []string) []string {
	out := append([]string(nil), values...)
	sort.Strings(out)
	return out
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
		strings.Contains(msg, "read --description-file") ||
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

func applyRootFlagOverridesFromArgs(opts *GlobalOptions, args []string) {
	if opts == nil {
		return
	}

	for index := 0; index < len(args); index++ {
		arg := args[index]

		switch {
		case arg == "--json":
			opts.JSON = true
		case arg == "--no-input":
			opts.NoInput = true
		case arg == "--quiet":
			opts.Quiet = true
		case arg == "--agents":
			opts.Agents = true
		case arg == "--agent-help":
			opts.AgentHelp = true
		case arg == "--version":
			opts.Version = true
		case arg == "--config":
			opts.Config = true
		case arg == "--db" && index+1 < len(args):
			index++
			opts.DBPath = args[index]
		case strings.HasPrefix(arg, "--db="):
			opts.DBPath = strings.TrimPrefix(arg, "--db=")
		case arg == "--actor" && index+1 < len(args):
			index++
			opts.Actor = args[index]
		case strings.HasPrefix(arg, "--actor="):
			opts.Actor = strings.TrimPrefix(arg, "--actor=")
		}
	}
}

func writeJSONTo(out io.Writer, payload map[string]any) error {
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	return encoder.Encode(payload)
}
