package report

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/H4ZM47/improved-invention/internal/app"
	taskconfig "github.com/H4ZM47/improved-invention/internal/config"
	taskdb "github.com/H4ZM47/improved-invention/internal/db"
)

func TestServerTaskListHTMLRendersFilteredTasks(t *testing.T) {
	t.Parallel()

	server, _ := seedReportServer(t)
	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()

	res, err := http.Get(httpServer.URL + "/tasks?search=contract&tag=cli&status=active")
	if err != nil {
		t.Fatalf("http.Get() error = %v", err)
	}
	defer res.Body.Close()

	if got, want := res.StatusCode, http.StatusOK; got != want {
		t.Fatalf("StatusCode = %d, want %d", got, want)
	}

	body := readBody(t, res)
	if !strings.Contains(body, "Write CLI contract") {
		t.Fatalf("task list HTML missing filtered task:\n%s", body)
	}
	if strings.Contains(body, "Routine docs") {
		t.Fatalf("task list HTML included non-matching task:\n%s", body)
	}
	if !strings.Contains(body, `value="contract"`) {
		t.Fatalf("task list HTML did not preserve search input:\n%s", body)
	}
}

func TestServerAPITasksReturnsFilteredJSON(t *testing.T) {
	t.Parallel()

	server, _ := seedReportServer(t)
	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()

	res, err := http.Get(httpServer.URL + "/api/tasks?tag=cli&status=active")
	if err != nil {
		t.Fatalf("http.Get() error = %v", err)
	}
	defer res.Body.Close()

	if got, want := res.StatusCode, http.StatusOK; got != want {
		t.Fatalf("StatusCode = %d, want %d", got, want)
	}

	var payload struct {
		OK      bool   `json:"ok"`
		Command string `json:"command"`
		Data    struct {
			Items []struct {
				Title string `json:"title"`
			} `json:"items"`
		} `json:"data"`
		Meta struct {
			Count   int `json:"count"`
			Filters struct {
				Statuses []string `json:"status"`
				Tags     []string `json:"tags"`
			} `json:"filters"`
		} `json:"meta"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if !payload.OK {
		t.Fatal("payload.OK = false, want true")
	}
	if got, want := payload.Command, "task report api tasks"; got != want {
		t.Fatalf("payload.Command = %q, want %q", got, want)
	}
	if got, want := payload.Meta.Count, 1; got != want {
		t.Fatalf("payload.Meta.Count = %d, want %d", got, want)
	}
	if got, want := payload.Data.Items[0].Title, "Write CLI contract"; got != want {
		t.Fatalf("payload.Data.Items[0].Title = %q, want %q", got, want)
	}
	if got, want := payload.Meta.Filters.Statuses[0], "active"; got != want {
		t.Fatalf("payload.Meta.Filters.Statuses[0] = %q, want %q", got, want)
	}
	if got, want := payload.Meta.Filters.Tags[0], "cli"; got != want {
		t.Fatalf("payload.Meta.Filters.Tags[0] = %q, want %q", got, want)
	}
}

func TestServerTaskDetailHTMLRendersLinksAndRelationships(t *testing.T) {
	t.Parallel()

	server, taskHandle := seedReportServer(t)
	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()

	res, err := http.Get(httpServer.URL + "/tasks/" + taskHandle)
	if err != nil {
		t.Fatalf("http.Get() error = %v", err)
	}
	defer res.Body.Close()

	if got, want := res.StatusCode, http.StatusOK; got != want {
		t.Fatalf("StatusCode = %d, want %d", got, want)
	}

	body := readBody(t, res)
	for _, want := range []string{
		"Write CLI contract",
		"Document list filters",
		"https://example.com/spec",
		"blocks",
		"TASK-2",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("task detail HTML missing %q:\n%s", want, body)
		}
	}
}

func TestServerRemainsReadOnly(t *testing.T) {
	t.Parallel()

	server, taskHandle := seedReportServer(t)
	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()

	for _, path := range []string{"/tasks", "/tasks/" + taskHandle, "/api/tasks"} {
		req, err := http.NewRequest(http.MethodPost, httpServer.URL+path, strings.NewReader(`{}`))
		if err != nil {
			t.Fatalf("http.NewRequest() error = %v", err)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Do(%s) error = %v", path, err)
		}
		res.Body.Close()

		if got, want := res.StatusCode, http.StatusMethodNotAllowed; got != want {
			t.Fatalf("StatusCode for %s = %d, want %d", path, got, want)
		}
	}
}

func seedReportServer(t *testing.T) (*Server, string) {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "task.db")
	cfg := taskconfig.Resolved{DBPath: dbPath, BusyTimeout: 5 * time.Second}

	db, err := taskdb.Open(context.Background(), cfg)
	if err != nil {
		t.Fatalf("taskdb.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	actorManager := app.ActorManager{DB: db, HumanName: "alex"}
	if _, err := actorManager.BootstrapConfiguredHumanActor(context.Background()); err != nil {
		t.Fatalf("BootstrapConfiguredHumanActor() error = %v", err)
	}

	taskManager := app.TaskManager{DB: db, HumanName: "alex"}
	linkManager := app.LinkManager{DB: db, HumanName: "alex"}
	relationshipManager := app.RelationshipManager{DB: db, HumanName: "alex"}

	mainTask, err := taskManager.Create(context.Background(), app.CreateTaskRequest{
		Title:       "Write CLI contract",
		Description: "Document list filters",
		Tags:        []string{"cli", "contract"},
	})
	if err != nil {
		t.Fatalf("Create(mainTask) error = %v", err)
	}
	otherTask, err := taskManager.Create(context.Background(), app.CreateTaskRequest{
		Title:       "Routine docs",
		Description: "General docs work",
		Tags:        []string{"docs"},
	})
	if err != nil {
		t.Fatalf("Create(otherTask) error = %v", err)
	}
	if _, err := taskManager.Claim(context.Background(), app.ClaimTaskRequest{
		Reference: mainTask.Handle,
		Lease:     time.Hour,
	}); err != nil {
		t.Fatalf("Claim() error = %v", err)
	}
	active := "active"
	if _, err := taskManager.Update(context.Background(), app.UpdateTaskRequest{
		Reference: mainTask.Handle,
		Status:    &active,
	}); err != nil {
		t.Fatalf("Update(active) error = %v", err)
	}
	if _, err := linkManager.Create(context.Background(), app.CreateLinkRequest{
		TaskRef: mainTask.Handle,
		Type:    "url",
		Target:  "https://example.com/spec",
		Label:   "Spec",
	}); err != nil {
		t.Fatalf("CreateLink() error = %v", err)
	}
	if _, err := relationshipManager.Create(context.Background(), app.CreateRelationshipRequest{
		Type:          "blocks",
		SourceTaskRef: mainTask.Handle,
		TargetTaskRef: otherTask.Handle,
	}); err != nil {
		t.Fatalf("CreateRelationship() error = %v", err)
	}

	server, err := NewServer(Dependencies{
		Tasks:         taskManager,
		Links:         linkManager,
		Relationships: relationshipManager,
	})
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	return server, mainTask.Handle
}

func readBody(t *testing.T, res *http.Response) string {
	t.Helper()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("io.ReadAll() error = %v", err)
	}
	return string(body)
}
