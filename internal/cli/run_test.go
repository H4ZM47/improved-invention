package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunVersionJSONMatchesGolden(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := Run(BuildInfo{
		Version: "1.2.3",
		Commit:  "abc123",
		Date:    "2026-04-21",
	}, []string{"--json", "version"}, &stdout, &stderr)

	if got, want := exitCode, 0; got != want {
		t.Fatalf("exitCode = %d, want %d", got, want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	assertGoldenFile(t, "version_success.json.golden", stdout.String())
}

func TestRunViewApplyMissingJSONMatchesGolden(t *testing.T) {
	t.Parallel()

	dbPath := seedViewFixtures(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := Run(BuildInfo{}, []string{"--db", dbPath, "--json", "view", "apply", "Missing"}, &stdout, &stderr)

	if got, want := exitCode, 60; got != want {
		t.Fatalf("exitCode = %d, want %d", got, want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	assertGoldenFile(t, "view_apply_missing_error.json.golden", stdout.String())
}

func TestRunInvalidArgsJSONMatchesGolden(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := Run(BuildInfo{}, []string{"--json", "bogus"}, &stdout, &stderr)

	if got, want := exitCode, 10; got != want {
		t.Fatalf("exitCode = %d, want %d", got, want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	assertGoldenFile(t, "invalid_args_error.json.golden", stdout.String())
}

func TestRunAssignmentDecisionRequiredNonJSONUsesExitCode(t *testing.T) {
	t.Parallel()

	dbPath, taskHandle, domainHandle := seedReclassificationScenario(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := Run(
		BuildInfo{},
		[]string{"--db", dbPath, "--actor", "alex", "--no-input", "update", taskHandle, "--domain", domainHandle},
		&stdout,
		&stderr,
	)

	if got, want := exitCode, 44; got != want {
		t.Fatalf("exitCode = %d, want %d", got, want)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if got := stderr.String(); got == "" || !containsAll(got, []string{"explicit assignee decision"}) {
		t.Fatalf("stderr = %q, want assignment decision message", got)
	}
}

func TestRunProjectCreateMissingDomainUsesInvalidArgsExitCode(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := Run(BuildInfo{}, []string{"--json", "project", "create", "Demo"}, &stdout, &stderr)
	if got, want := exitCode, 10; got != want {
		t.Fatalf("exitCode = %d, want %d; stdout=%s", got, want, stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	if got := stdout.String(); !containsAll(got, []string{`"code": "INVALID_ARGS"`, `required flag(s)`, `domain`}) {
		t.Fatalf("stdout = %q, want invalid args payload", got)
	}
}

func TestRunShowMissingSanitizesEntityNotFoundMessage(t *testing.T) {
	t.Parallel()

	dbPath := seedViewFixtures(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := Run(BuildInfo{}, []string{"--db", dbPath, "--json", "show", "TASK-404"}, &stdout, &stderr)
	if got, want := exitCode, 20; got != want {
		t.Fatalf("exitCode = %d, want %d; stdout=%s", got, want, stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	if got := stdout.String(); !containsAll(got, []string{`"code": "ENTITY_NOT_FOUND"`, `task \"TASK-404\" was not found`}) {
		t.Fatalf("stdout = %q, want sanitized entity message", got)
	}
	if strings.Contains(stdout.String(), "sql: no rows in result set") {
		t.Fatalf("stdout leaked raw sql details: %q", stdout.String())
	}
}

func assertGoldenFile(t *testing.T, name, got string) {
	t.Helper()

	path := filepath.Join("..", "..", "testdata", "cli", name)
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v", path, err)
	}
	if got != string(want) {
		t.Fatalf("golden mismatch for %s\n--- got ---\n%s\n--- want ---\n%s", name, got, string(want))
	}
}

func containsAll(haystack string, needles []string) bool {
	for _, needle := range needles {
		if !bytes.Contains([]byte(haystack), []byte(needle)) {
			return false
		}
	}
	return true
}
