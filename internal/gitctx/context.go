package gitctx

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

var ErrNotGitRepo = errors.New("not inside a git repository")

// Context describes the current git repository and worktree context.
type Context struct {
	RepoRoot     string
	WorktreeRoot string
	GitDir       string
	RemoteURL    string
}

// RepoTarget returns the canonical repo target for explicit repo links.
func (c Context) RepoTarget() string {
	if strings.TrimSpace(c.RemoteURL) != "" {
		return c.RemoteURL
	}
	return c.RepoRoot
}

// WorktreeTarget returns the canonical worktree target for explicit worktree links.
func (c Context) WorktreeTarget() string {
	return c.WorktreeRoot
}

// Detect resolves git repository and worktree context from the provided cwd.
func Detect(ctx context.Context, cwd string) (Context, error) {
	repoRoot, err := gitOutput(ctx, cwd, "rev-parse", "--show-toplevel")
	if err != nil {
		return Context{}, err
	}

	gitDir, err := gitOutput(ctx, cwd, "rev-parse", "--absolute-git-dir")
	if err != nil {
		return Context{}, err
	}

	remoteURL, err := gitOutputOptional(ctx, cwd, "remote", "get-url", "origin")
	if err != nil {
		return Context{}, err
	}

	return Context{
		RepoRoot:     repoRoot,
		WorktreeRoot: repoRoot,
		GitDir:       gitDir,
		RemoteURL:    remoteURL,
	}, nil
}

func gitOutput(ctx context.Context, cwd string, args ...string) (string, error) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", cwd}, args...)...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if strings.Contains(stderr.String(), "not a git repository") {
			return "", ErrNotGitRepo
		}
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}

	return strings.TrimSpace(stdout.String()), nil
}

func gitOutputOptional(ctx context.Context, cwd string, args ...string) (string, error) {
	value, err := gitOutput(ctx, cwd, args...)
	if err != nil {
		var exitErr *exec.ExitError
		if errors.Is(err, ErrNotGitRepo) {
			return "", err
		}
		if errors.As(err, &exitErr) {
			return "", nil
		}
		if strings.Contains(err.Error(), "No such remote") {
			return "", nil
		}
		return "", err
	}
	return value, nil
}
