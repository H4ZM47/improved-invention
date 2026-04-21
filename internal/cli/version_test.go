package cli

import (
	"bytes"
	"testing"
)

func TestVersionCommandTextOutput(t *testing.T) {
	t.Parallel()

	opts := &GlobalOptions{}
	build := BuildInfo{
		Version: "1.2.3",
		Commit:  "abc123",
		Date:    "2026-04-21",
	}

	cmd := newVersionCommand(build, opts)
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	got := stdout.String()
	want := "version=1.2.3 commit=abc123 date=2026-04-21\n"
	if got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
}

func TestVersionCommandJSONOutput(t *testing.T) {
	t.Parallel()

	opts := &GlobalOptions{JSON: true}
	build := BuildInfo{
		Version: "1.2.3",
		Commit:  "abc123",
		Date:    "2026-04-21",
	}

	cmd := newVersionCommand(build, opts)
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	want := "{\n  \"command\": \"grind version\",\n  \"data\": {\n    \"commit\": \"abc123\",\n    \"date\": \"2026-04-21\",\n    \"version\": \"1.2.3\"\n  },\n  \"meta\": {},\n  \"ok\": true\n}\n"
	if stdout.String() != want {
		t.Fatalf("stdout = %q, want %q", stdout.String(), want)
	}
}
