package cli

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/H4ZM47/grind/internal/app"
	taskconfig "github.com/H4ZM47/grind/internal/config"
	taskdb "github.com/H4ZM47/grind/internal/db"
	"github.com/H4ZM47/grind/internal/gitctx"
)

func TestLinkAttachCurrentRepoCommandJSONCreatesRepoAndWorktreeLinks(t *testing.T) {
	dbPath, taskHandle := seedClaimedTaskForRepoContextCLI(t)
	repoDir := initCLIRepo(t, "https://github.com/H4ZM47/grind.git")
	current := detectRepoContext(t, repoDir)

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	withWorkingDirectory(t, repoDir, func() {
		root := NewRootCommand(BuildInfo{})
		root.SetOut(&stdout)
		root.SetErr(&stderr)
		root.SetArgs([]string{
			"--db", dbPath,
			"--actor", "alex",
			"--json",
			"link-repo", taskHandle,
		})

		if err := root.Execute(); err != nil {
			t.Fatalf("grind link-repo Execute() error = %v; stderr=%q", err, stderr.String())
		}
	})

	var payload struct {
		OK      bool   `json:"ok"`
		Command string `json:"command"`
		Data    struct {
			RepoLink *struct {
				Type     string            `json:"type"`
				Target   string            `json:"target"`
				Metadata map[string]string `json:"metadata"`
			} `json:"repo_link"`
			WorktreeLink struct {
				Type     string            `json:"type"`
				Target   string            `json:"target"`
				Metadata map[string]string `json:"metadata"`
			} `json:"worktree_link"`
		} `json:"data"`
		Meta struct {
			RepoTarget     string `json:"repo_target"`
			WorktreeTarget string `json:"worktree_target"`
		} `json:"meta"`
	}

	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal(stdout) error = %v; stdout=%q", err, stdout.String())
	}

	if !payload.OK {
		t.Fatal("payload.OK = false, want true")
	}
	if got, want := payload.Command, "grind link-repo"; got != want {
		t.Fatalf("payload.Command = %q, want %q", got, want)
	}
	if payload.Data.RepoLink == nil {
		t.Fatal("payload.Data.RepoLink = nil, want explicit repo link")
	}
	if got, want := payload.Data.RepoLink.Type, taskdb.LinkTypeRepo; got != want {
		t.Fatalf("payload.Data.RepoLink.Type = %q, want %q", got, want)
	}
	if got, want := payload.Data.RepoLink.Target, current.RepoTarget(); got != want {
		t.Fatalf("payload.Data.RepoLink.Target = %q, want %q", got, want)
	}
	if got, want := payload.Data.WorktreeLink.Type, taskdb.LinkTypeWorktree; got != want {
		t.Fatalf("payload.Data.WorktreeLink.Type = %q, want %q", got, want)
	}
	if got, want := payload.Data.WorktreeLink.Target, current.WorktreeTarget(); got != want {
		t.Fatalf("payload.Data.WorktreeLink.Target = %q, want %q", got, want)
	}
	if got, want := payload.Meta.RepoTarget, current.RepoTarget(); got != want {
		t.Fatalf("payload.Meta.RepoTarget = %q, want %q", got, want)
	}
	if got, want := payload.Meta.WorktreeTarget, current.WorktreeTarget(); got != want {
		t.Fatalf("payload.Meta.WorktreeTarget = %q, want %q", got, want)
	}
	if got, want := payload.Data.RepoLink.Metadata["repo_root"], current.RepoRoot; got != want {
		t.Fatalf("payload.Data.RepoLink.Metadata[repo_root] = %q, want %q", got, want)
	}
	if got, want := payload.Data.WorktreeLink.Metadata["repo_root"], current.RepoRoot; got != want {
		t.Fatalf("payload.Data.WorktreeLink.Metadata[repo_root] = %q, want %q", got, want)
	}

	links := listExternalLinksForTask(t, dbPath, taskHandle)
	if got, want := len(links), 2; got != want {
		t.Fatalf("len(links) = %d, want %d", got, want)
	}

	targetsByType := map[string]string{}
	for _, link := range links {
		targetsByType[link.LinkType] = link.Target
	}
	if got, want := targetsByType[taskdb.LinkTypeRepo], current.RepoTarget(); got != want {
		t.Fatalf("repo link target = %q, want %q", got, want)
	}
	if got, want := targetsByType[taskdb.LinkTypeWorktree], current.WorktreeTarget(); got != want {
		t.Fatalf("worktree link target = %q, want %q", got, want)
	}
}

func TestTaskListHereJSONScopesToCurrentRepoContextWithoutMutation(t *testing.T) {
	currentRepoDir := initCLIRepo(t, "https://github.com/H4ZM47/grind.git")
	otherRepoDir := initCLIRepo(t, "https://github.com/example/other.git")
	current := detectRepoContext(t, currentRepoDir)
	other := detectRepoContext(t, otherRepoDir)

	dbPath, repoOnlyHandle, worktreeOnlyHandle, unrelatedHandle := seedRepoContextListScenario(
		t,
		current.RepoTarget(),
		current.WorktreeTarget(),
		other.RepoTarget(),
		other.WorktreeTarget(),
	)

	beforeCount := countExternalLinks(t, dbPath)
	beforeRepoOnly := len(listExternalLinksForTask(t, dbPath, repoOnlyHandle))
	beforeWorktreeOnly := len(listExternalLinksForTask(t, dbPath, worktreeOnlyHandle))
	beforeUnrelated := len(listExternalLinksForTask(t, dbPath, unrelatedHandle))

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	withWorkingDirectory(t, currentRepoDir, func() {
		root := NewRootCommand(BuildInfo{})
		root.SetOut(&stdout)
		root.SetErr(&stderr)
		root.SetArgs([]string{
			"--db", dbPath,
			"--actor", "alex",
			"--json",
			"list",
			"--here",
		})

		if err := root.Execute(); err != nil {
			t.Fatalf("grind list --here Execute() error = %v; stderr=%q", err, stderr.String())
		}
	})

	var payload struct {
		OK      bool   `json:"ok"`
		Command string `json:"command"`
		Data    struct {
			Items []struct {
				Handle string `json:"handle"`
			} `json:"items"`
		} `json:"data"`
		Meta struct {
			Count   int `json:"count"`
			Filters struct {
				Here bool `json:"here"`
			} `json:"filters"`
		} `json:"meta"`
	}

	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal(stdout) error = %v; stdout=%q", err, stdout.String())
	}

	if !payload.OK {
		t.Fatal("payload.OK = false, want true")
	}
	if got, want := payload.Command, "grind list"; got != want {
		t.Fatalf("payload.Command = %q, want %q", got, want)
	}
	if !payload.Meta.Filters.Here {
		t.Fatal("payload.Meta.Filters.Here = false, want true")
	}
	if got, want := payload.Meta.Count, 2; got != want {
		t.Fatalf("payload.Meta.Count = %d, want %d", got, want)
	}

	handles := make([]string, 0, len(payload.Data.Items))
	for _, item := range payload.Data.Items {
		handles = append(handles, item.Handle)
	}
	slices.Sort(handles)

	wantHandles := []string{repoOnlyHandle, worktreeOnlyHandle}
	slices.Sort(wantHandles)
	if !slices.Equal(handles, wantHandles) {
		t.Fatalf("grind list --here handles = %v, want %v", handles, wantHandles)
	}
	for _, handle := range handles {
		if handle == unrelatedHandle {
			t.Fatalf("grind list --here included unrelated handle %q", unrelatedHandle)
		}
	}

	if got, want := countExternalLinks(t, dbPath), beforeCount; got != want {
		t.Fatalf("countExternalLinks() after list = %d, want %d", got, want)
	}
	if got, want := len(listExternalLinksForTask(t, dbPath, repoOnlyHandle)), beforeRepoOnly; got != want {
		t.Fatalf("repo-only grind link count after list = %d, want %d", got, want)
	}
	if got, want := len(listExternalLinksForTask(t, dbPath, worktreeOnlyHandle)), beforeWorktreeOnly; got != want {
		t.Fatalf("worktree-only grind link count after list = %d, want %d", got, want)
	}
	if got, want := len(listExternalLinksForTask(t, dbPath, unrelatedHandle)), beforeUnrelated; got != want {
		t.Fatalf("unrelated grind link count after list = %d, want %d", got, want)
	}
}

func seedClaimedTaskForRepoContextCLI(t *testing.T) (dbPath string, taskHandle string) {
	t.Helper()

	dbPath = filepath.Join(t.TempDir(), "task.db")
	db := openRepoContextTestDB(t, dbPath)
	defer db.Close()

	manager := app.TaskManager{
		DB:        db,
		HumanName: "alex",
	}
	task, err := manager.Create(context.Background(), app.CreateTaskRequest{
		Title: "Attach current repo context",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, err := manager.Claim(context.Background(), app.ClaimTaskRequest{
		Reference: task.Handle,
		Lease:     time.Hour,
	}); err != nil {
		t.Fatalf("Claim() error = %v", err)
	}

	return dbPath, task.Handle
}

func seedRepoContextListScenario(
	t *testing.T,
	currentRepoTarget string,
	currentWorktreeTarget string,
	otherRepoTarget string,
	otherWorktreeTarget string,
) (dbPath string, repoOnlyHandle string, worktreeOnlyHandle string, unrelatedHandle string) {
	t.Helper()

	dbPath = filepath.Join(t.TempDir(), "task.db")
	db := openRepoContextTestDB(t, dbPath)
	defer db.Close()

	manager := app.TaskManager{
		DB:        db,
		HumanName: "alex",
	}

	repoOnly, err := manager.Create(context.Background(), app.CreateTaskRequest{
		Title: "Repo scoped task",
	})
	if err != nil {
		t.Fatalf("Create(repoOnly) error = %v", err)
	}
	worktreeOnly, err := manager.Create(context.Background(), app.CreateTaskRequest{
		Title: "Worktree scoped task",
	})
	if err != nil {
		t.Fatalf("Create(worktreeOnly) error = %v", err)
	}
	unrelated, err := manager.Create(context.Background(), app.CreateTaskRequest{
		Title: "Unrelated repo task",
	})
	if err != nil {
		t.Fatalf("Create(unrelated) error = %v", err)
	}

	for _, input := range []taskdb.TaskExternalLinkCreateInput{
		{
			TaskReference: repoOnly.Handle,
			LinkType:      taskdb.LinkTypeRepo,
			Target:        currentRepoTarget,
			Label:         "Current repository",
			MetadataJSON:  "{}",
		},
		{
			TaskReference: worktreeOnly.Handle,
			LinkType:      taskdb.LinkTypeWorktree,
			Target:        currentWorktreeTarget,
			Label:         "Current worktree",
			MetadataJSON:  "{}",
		},
		{
			TaskReference: unrelated.Handle,
			LinkType:      taskdb.LinkTypeRepo,
			Target:        otherRepoTarget,
			Label:         "Other repository",
			MetadataJSON:  "{}",
		},
		{
			TaskReference: unrelated.Handle,
			LinkType:      taskdb.LinkTypeWorktree,
			Target:        otherWorktreeTarget,
			Label:         "Other worktree",
			MetadataJSON:  "{}",
		},
	} {
		if _, err := taskdb.CreateExternalLink(context.Background(), db, input); err != nil {
			t.Fatalf("CreateExternalLink(%s, %s) error = %v", input.TaskReference, input.LinkType, err)
		}
	}

	return dbPath, repoOnly.Handle, worktreeOnly.Handle, unrelated.Handle
}

func openRepoContextTestDB(t *testing.T, dbPath string) *sql.DB {
	t.Helper()

	cfg := taskconfig.Resolved{
		DBPath:      dbPath,
		BusyTimeout: 5 * time.Second,
	}

	db, err := taskdb.Open(context.Background(), cfg)
	if err != nil {
		t.Fatalf("taskdb.Open() error = %v", err)
	}
	return db
}

func listExternalLinksForTask(t *testing.T, dbPath string, taskHandle string) []taskdb.ExternalLink {
	t.Helper()

	db := openRepoContextTestDB(t, dbPath)
	defer db.Close()

	links, err := taskdb.ListExternalLinksForTask(context.Background(), db, taskHandle)
	if err != nil {
		t.Fatalf("ListExternalLinksForTask(%s) error = %v", taskHandle, err)
	}
	return links
}

func countExternalLinks(t *testing.T, dbPath string) int {
	t.Helper()

	db := openRepoContextTestDB(t, dbPath)
	defer db.Close()

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM links WHERE target_kind = 'external'`).Scan(&count); err != nil {
		t.Fatalf("count external links failed: %v", err)
	}
	return count
}

func withWorkingDirectory(t *testing.T, dir string, fn func()) {
	t.Helper()

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd() error = %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("os.Chdir(%q) error = %v", dir, err)
	}
	defer func() {
		if err := os.Chdir(cwd); err != nil {
			t.Fatalf("restore cwd to %q error = %v", cwd, err)
		}
	}()

	fn()
}

func initCLIRepo(t *testing.T, remoteURL string) string {
	t.Helper()

	repoDir := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v", repoDir, err)
	}

	for _, args := range [][]string{
		{"git", "init", "-q", repoDir},
		{"git", "-C", repoDir, "config", "user.name", "Codex"},
		{"git", "-C", repoDir, "config", "user.email", "codex@example.com"},
		{"git", "-C", repoDir, "remote", "add", "origin", remoteURL},
	} {
		if output, err := exec.Command(args[0], args[1:]...).CombinedOutput(); err != nil {
			t.Fatalf("%v failed: %v\n%s", args, err, string(output))
		}
	}

	return repoDir
}

func detectRepoContext(t *testing.T, repoDir string) gitctx.Context {
	t.Helper()

	current, err := gitctx.Detect(context.Background(), repoDir)
	if err != nil {
		t.Fatalf("gitctx.Detect(%q) error = %v", repoDir, err)
	}
	return current
}
