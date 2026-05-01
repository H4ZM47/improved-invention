package cli

import (
	"bytes"
	"context"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/H4ZM47/grind/internal/app"
	taskconfig "github.com/H4ZM47/grind/internal/config"
	taskdb "github.com/H4ZM47/grind/internal/db"
)

func TestReportServeCommandBootstrapsServer(t *testing.T) {
	t.Parallel()

	dbPath := seedReportServeFixtures(t)
	addr := reserveTestAddr(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	root := NewRootCommand(BuildInfo{})
	root.SetContext(ctx)
	root.SetArgs([]string{"--db", dbPath, "serve", "--addr", addr})
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)

	done := make(chan error, 1)
	go func() {
		done <- root.Execute()
	}()

	reportURL := "http://" + addr + "/tasks"
	waitForReportServer(t, reportURL)

	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Execute() error = %v; stdout=%q stderr=%q", err, stdout.String(), stderr.String())
		}
	case <-time.After(5 * time.Second):
		t.Fatal("serve command did not exit after context cancellation")
	}

	if !strings.Contains(stdout.String(), "Serving read-only report at http://"+addr) {
		t.Fatalf("stdout = %q, want report URL bootstrap message", stdout.String())
	}
}

func seedReportServeFixtures(t *testing.T) string {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "task.db")
	cfg := taskconfig.Resolved{DBPath: dbPath, BusyTimeout: 5 * time.Second}

	db, err := taskdb.Open(context.Background(), cfg)
	if err != nil {
		t.Fatalf("taskdb.Open() error = %v", err)
	}
	defer func() {
		_ = db.Close()
	}()

	if _, err := (app.ActorManager{DB: db, HumanName: "alex"}).BootstrapConfiguredHumanActor(context.Background()); err != nil {
		t.Fatalf("BootstrapConfiguredHumanActor() error = %v", err)
	}
	if _, err := (app.TaskManager{DB: db, HumanName: "alex"}).Create(context.Background(), app.CreateTaskRequest{
		Title: "Serve report fixtures",
		Tags:  []string{"report"},
	}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	return dbPath
}

func reserveTestAddr(t *testing.T) string {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen() error = %v", err)
	}
	addr := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatalf("listener.Close() error = %v", err)
	}
	return addr
}

func waitForReportServer(t *testing.T, reportURL string) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		res, err := http.Get(reportURL)
		if err == nil {
			_ = res.Body.Close()
			if res.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("report server at %s did not become ready in time", reportURL)
}
