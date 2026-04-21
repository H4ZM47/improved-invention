package cli

import (
	"bytes"
	"os"
	"path/filepath"
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
