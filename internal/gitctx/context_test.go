package gitctx

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestDetectReturnsRepoAndRemoteContext(t *testing.T) {
	t.Parallel()

	repoDir := initGitRepo(t)

	current, err := Detect(context.Background(), repoDir)
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	canonicalRepoDir, err := filepath.EvalSymlinks(repoDir)
	if err != nil {
		t.Fatalf("EvalSymlinks(repoDir) error = %v", err)
	}
	if got, want := current.RepoRoot, canonicalRepoDir; got != want {
		t.Fatalf("RepoRoot = %q, want %q", got, want)
	}
	if got, want := current.WorktreeRoot, canonicalRepoDir; got != want {
		t.Fatalf("WorktreeRoot = %q, want %q", got, want)
	}
	if got, want := current.RemoteURL, "https://github.com/H4ZM47/improved-invention.git"; got != want {
		t.Fatalf("RemoteURL = %q, want %q", got, want)
	}
	if got, want := current.RepoTarget(), "https://github.com/H4ZM47/improved-invention.git"; got != want {
		t.Fatalf("RepoTarget() = %q, want %q", got, want)
	}
}

func TestDetectReturnsErrNotGitRepoOutsideRepository(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if _, err := Detect(context.Background(), dir); err != ErrNotGitRepo {
		t.Fatalf("Detect() error = %v, want ErrNotGitRepo", err)
	}
}

func initGitRepo(t *testing.T) string {
	t.Helper()

	repoDir := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatalf("mkdir repo failed: %v", err)
	}

	for _, args := range [][]string{
		{"git", "init", "-q", repoDir},
		{"git", "-C", repoDir, "config", "user.name", "Codex"},
		{"git", "-C", repoDir, "config", "user.email", "codex@example.com"},
		{"git", "-C", repoDir, "remote", "add", "origin", "https://github.com/H4ZM47/improved-invention.git"},
	} {
		if out, err := exec.Command(args[0], args[1:]...).CombinedOutput(); err != nil {
			t.Fatalf("%v failed: %v\n%s", args, err, string(out))
		}
	}

	return repoDir
}
