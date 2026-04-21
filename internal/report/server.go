package report

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/H4ZM47/grind/internal/app"
)

// Server serves read-only HTML and JSON task reports.
type Server struct {
	tasks         app.TaskManager
	links         app.LinkManager
	relationships app.RelationshipManager
	templates     *template.Template
}

// Dependencies groups the app-layer managers used by the report server.
type Dependencies struct {
	Tasks         app.TaskManager
	Links         app.LinkManager
	Relationships app.RelationshipManager
}

// NewServer constructs a read-only report server over the shared app boundary.
func NewServer(deps Dependencies) (*Server, error) {
	tmpl, err := template.New("report").Funcs(template.FuncMap{
		"joined": strings.Join,
		"query":  encodeFilterQuery,
	}).Parse(layoutTemplate + taskListTemplate + taskDetailTemplate)
	if err != nil {
		return nil, fmt.Errorf("parse report templates: %w", err)
	}

	return &Server{
		tasks:         deps.Tasks,
		links:         deps.Links,
		relationships: deps.Relationships,
		templates:     tmpl,
	}, nil
}

// Handler returns the HTTP handler for the read-only report server.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleRoot)
	mux.HandleFunc("/tasks", s.handleTaskList)
	mux.HandleFunc("/tasks/", s.handleTaskDetail)
	mux.HandleFunc("/api/tasks", s.handleAPITasks)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "report server is read-only", http.StatusMethodNotAllowed)
			return
		}
		mux.ServeHTTP(w, r)
	})
}

type filterState struct {
	Statuses  []string `json:"status"`
	Tags      []string `json:"tags"`
	Domain    string   `json:"domain,omitempty"`
	Project   string   `json:"project,omitempty"`
	Milestone string   `json:"milestone,omitempty"`
	Assignee  string   `json:"assignee,omitempty"`
	DueBefore string   `json:"due_before,omitempty"`
	DueAfter  string   `json:"due_after,omitempty"`
	Search    string   `json:"search,omitempty"`
}

type taskListPageData struct {
	Title      string
	Tasks      []app.TaskRecord
	Filters    filterState
	StatusText string
	TagText    string
	Generated  string
}

type taskDetailPageData struct {
	Title         string
	Task          app.TaskRecord
	Links         []app.LinkRecord
	Relationships []app.RelationshipRecord
	BackURL       string
	Generated     string
}

func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	target := "/tasks"
	if raw := r.URL.RawQuery; raw != "" {
		target += "?" + raw
	}
	http.Redirect(w, r, target, http.StatusTemporaryRedirect)
}

func (s *Server) handleTaskList(w http.ResponseWriter, r *http.Request) {
	req, filters := parseListFilters(r.URL.Query())
	tasks, err := s.tasks.List(r.Context(), req)
	if err != nil {
		http.Error(w, fmt.Sprintf("list tasks: %v", err), http.StatusInternalServerError)
		return
	}

	data := taskListPageData{
		Title:      "Task Report",
		Tasks:      tasks,
		Filters:    filters,
		StatusText: strings.Join(filters.Statuses, ", "),
		TagText:    strings.Join(filters.Tags, ", "),
		Generated:  time.Now().Format(time.RFC3339),
	}
	if err := s.renderHTML(w, "task-list", data); err != nil {
		http.Error(w, fmt.Sprintf("render grind list: %v", err), http.StatusInternalServerError)
	}
}

func (s *Server) handleTaskDetail(w http.ResponseWriter, r *http.Request) {
	ref := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/tasks/"))
	if ref == "" {
		http.NotFound(w, r)
		return
	}

	task, err := s.tasks.Show(r.Context(), app.ShowTaskRequest{Reference: ref})
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, context.Canceled) {
			status = http.StatusRequestTimeout
		}
		if strings.Contains(strings.ToLower(err.Error()), "not found") {
			status = http.StatusNotFound
		}
		http.Error(w, err.Error(), status)
		return
	}

	links, err := s.links.List(r.Context(), app.ListLinksRequest{TaskRef: ref})
	if err != nil {
		http.Error(w, fmt.Sprintf("list task links: %v", err), http.StatusInternalServerError)
		return
	}
	externalLinks := make([]app.LinkRecord, 0, len(links))
	for _, link := range links {
		if link.TargetKind != "external" {
			continue
		}
		externalLinks = append(externalLinks, link)
	}
	relationships, err := s.relationships.List(r.Context(), app.ListRelationshipsRequest{TaskRef: ref})
	if err != nil {
		http.Error(w, fmt.Sprintf("list task relationships: %v", err), http.StatusInternalServerError)
		return
	}

	data := taskDetailPageData{
		Title:         task.Title,
		Task:          task,
		Links:         externalLinks,
		Relationships: relationships,
		BackURL:       "/tasks",
		Generated:     time.Now().Format(time.RFC3339),
	}
	if err := s.renderHTML(w, "task-detail", data); err != nil {
		http.Error(w, fmt.Sprintf("render task detail: %v", err), http.StatusInternalServerError)
	}
}

func (s *Server) handleAPITasks(w http.ResponseWriter, r *http.Request) {
	req, filters := parseListFilters(r.URL.Query())
	tasks, err := s.tasks.List(r.Context(), req)
	if err != nil {
		http.Error(w, fmt.Sprintf("list tasks: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	payload := map[string]any{
		"ok":      true,
		"command": "grind serve api tasks",
		"data":    map[string]any{"items": tasks},
		"meta": map[string]any{
			"count":   len(tasks),
			"filters": filters,
		},
	}
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	_ = encoder.Encode(payload)
}

func (s *Server) renderHTML(w http.ResponseWriter, name string, data any) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return s.templates.ExecuteTemplate(w, name, data)
}

func parseListFilters(values url.Values) (app.ListTasksRequest, filterState) {
	statuses := splitFilterValues(values["status"])
	tags := splitFilterValues(values["tag"])
	filters := filterState{
		Statuses:  statuses,
		Tags:      tags,
		Domain:    strings.TrimSpace(values.Get("domain")),
		Project:   strings.TrimSpace(values.Get("project")),
		Milestone: strings.TrimSpace(values.Get("milestone")),
		Assignee:  strings.TrimSpace(values.Get("assignee")),
		DueBefore: strings.TrimSpace(values.Get("due-before")),
		DueAfter:  strings.TrimSpace(values.Get("due-after")),
		Search:    strings.TrimSpace(values.Get("search")),
	}

	req := app.ListTasksRequest{
		Statuses: statuses,
		Tags:     tags,
		Search:   filters.Search,
	}
	req.DomainRef = stringPtrIfSet(filters.Domain)
	req.ProjectRef = stringPtrIfSet(filters.Project)
	req.MilestoneRef = stringPtrIfSet(filters.Milestone)
	req.AssigneeRef = stringPtrIfSet(filters.Assignee)
	req.DueBefore = stringPtrIfSet(filters.DueBefore)
	req.DueAfter = stringPtrIfSet(filters.DueAfter)
	return req, filters
}

func splitFilterValues(raw []string) []string {
	if len(raw) == 0 {
		return nil
	}

	var values []string
	for _, item := range raw {
		for _, part := range strings.Split(item, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			values = append(values, part)
		}
	}
	if len(values) == 0 {
		return nil
	}
	return values
}

func stringPtrIfSet(value string) *string {
	if value == "" {
		return nil
	}
	copy := value
	return &copy
}

func encodeFilterQuery(filters filterState) string {
	values := url.Values{}
	for _, status := range filters.Statuses {
		values.Add("status", status)
	}
	for _, tag := range filters.Tags {
		values.Add("tag", tag)
	}
	if filters.Domain != "" {
		values.Set("domain", filters.Domain)
	}
	if filters.Project != "" {
		values.Set("project", filters.Project)
	}
	if filters.Milestone != "" {
		values.Set("milestone", filters.Milestone)
	}
	if filters.Assignee != "" {
		values.Set("assignee", filters.Assignee)
	}
	if filters.DueBefore != "" {
		values.Set("due-before", filters.DueBefore)
	}
	if filters.DueAfter != "" {
		values.Set("due-after", filters.DueAfter)
	}
	if filters.Search != "" {
		values.Set("search", filters.Search)
	}
	return values.Encode()
}

const layoutTemplate = `
{{define "page-start"}}
<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>{{.Title}}</title>
  <style>
    :root {
      color-scheme: light;
      --bg: #f4efe5;
      --surface: #fffaf2;
      --ink: #20201c;
      --muted: #706c61;
      --line: #d6cdb8;
      --accent: #ab4e24;
      --accent-soft: #f2dccf;
      --chip: #e7ddca;
      --shadow: rgba(32, 32, 28, 0.08);
      font-family: ui-sans-serif, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
    }
    * { box-sizing: border-box; }
    body {
      margin: 0;
      background:
        radial-gradient(circle at top left, rgba(171, 78, 36, 0.12), transparent 24rem),
        linear-gradient(180deg, #fbf7ee 0%, var(--bg) 100%);
      color: var(--ink);
    }
    main {
      width: min(1080px, calc(100vw - 2rem));
      margin: 0 auto;
      padding: 2rem 0 3rem;
    }
    .hero, .panel {
      background: var(--surface);
      border: 1px solid var(--line);
      border-radius: 18px;
      box-shadow: 0 12px 28px var(--shadow);
    }
    .hero {
      padding: 1.5rem;
      margin-bottom: 1rem;
    }
    .hero h1 {
      margin: 0 0 0.35rem;
      font-size: clamp(1.7rem, 4vw, 2.5rem);
      line-height: 1;
    }
    .hero p, .muted {
      margin: 0;
      color: var(--muted);
    }
    .panel { padding: 1.25rem; margin-bottom: 1rem; }
    .grid {
      display: grid;
      gap: 0.75rem;
      grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
    }
    label {
      display: grid;
      gap: 0.35rem;
      font-size: 0.95rem;
      color: var(--muted);
    }
    input {
      width: 100%;
      padding: 0.7rem 0.8rem;
      border: 1px solid var(--line);
      border-radius: 12px;
      background: #fff;
      color: var(--ink);
    }
    .actions {
      display: flex;
      gap: 0.75rem;
      align-items: center;
      margin-top: 1rem;
      flex-wrap: wrap;
    }
    .button, button {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      min-height: 2.5rem;
      padding: 0 1rem;
      border-radius: 999px;
      border: 1px solid var(--accent);
      background: var(--accent);
      color: #fff;
      text-decoration: none;
      font-weight: 600;
      cursor: pointer;
    }
    .button.secondary {
      color: var(--accent);
      background: transparent;
    }
    table {
      width: 100%;
      border-collapse: collapse;
    }
    th, td {
      text-align: left;
      padding: 0.85rem 0.65rem;
      border-bottom: 1px solid var(--line);
      vertical-align: top;
    }
    th {
      font-size: 0.84rem;
      color: var(--muted);
      text-transform: uppercase;
      letter-spacing: 0.04em;
    }
    a { color: var(--accent); }
    .chips {
      display: flex;
      flex-wrap: wrap;
      gap: 0.4rem;
    }
    .chip {
      display: inline-flex;
      align-items: center;
      min-height: 1.8rem;
      padding: 0 0.7rem;
      border-radius: 999px;
      background: var(--chip);
      color: var(--ink);
      font-size: 0.85rem;
    }
    .summary-grid {
      display: grid;
      gap: 0.8rem;
      grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
    }
    .summary-card {
      padding: 1rem;
      border: 1px solid var(--line);
      border-radius: 16px;
      background: #fff;
    }
    .summary-card h2, .summary-card h3 {
      margin: 0 0 0.45rem;
      font-size: 1rem;
    }
    .summary-card p, .summary-card ul {
      margin: 0;
      color: var(--muted);
    }
    .mono {
      font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
      font-size: 0.9rem;
    }
    @media (max-width: 720px) {
      main { width: min(100vw - 1rem, 1080px); padding-top: 1rem; }
      .panel, .hero { padding: 1rem; border-radius: 14px; }
      table, tbody, thead, tr, th, td { display: block; }
      thead { display: none; }
      tr {
        border-bottom: 1px solid var(--line);
        padding: 0.65rem 0;
      }
      td {
        border: 0;
        padding: 0.35rem 0;
      }
      td::before {
        content: attr(data-label);
        display: block;
        font-size: 0.8rem;
        color: var(--muted);
        text-transform: uppercase;
        letter-spacing: 0.04em;
      }
    }
  </style>
</head>
<body>
  <main>
{{end}}

{{define "page-end"}}
  </main>
</body>
</html>
{{end}}
`

const taskListTemplate = `
{{define "task-list"}}
{{template "page-start" .}}
  <section class="hero">
    <h1>Task Report</h1>
    <p>Read-only local reporting over the shared task database. Generated at {{.Generated}}.</p>
  </section>

  <section class="panel">
    <form action="/tasks" method="get">
      <div class="grid">
        <label>Search
          <input type="text" name="search" value="{{.Filters.Search}}" placeholder="title or description">
        </label>
        <label>Status
          <input type="text" name="status" value="{{.StatusText}}" placeholder="backlog, active">
        </label>
        <label>Tags
          <input type="text" name="tag" value="{{.TagText}}" placeholder="cli, docs">
        </label>
        <label>Domain
          <input type="text" name="domain" value="{{.Filters.Domain}}" placeholder="DOMAIN-1 or uuid">
        </label>
        <label>Project
          <input type="text" name="project" value="{{.Filters.Project}}" placeholder="PROJECT-1 or uuid">
        </label>
        <label>Milestone
          <input type="text" name="milestone" value="{{.Filters.Milestone}}" placeholder="MILE-1 or uuid">
        </label>
        <label>Assignee
          <input type="text" name="assignee" value="{{.Filters.Assignee}}" placeholder="actor handle or uuid">
        </label>
        <label>Due Before
          <input type="text" name="due-before" value="{{.Filters.DueBefore}}" placeholder="RFC3339 timestamp">
        </label>
        <label>Due After
          <input type="text" name="due-after" value="{{.Filters.DueAfter}}" placeholder="RFC3339 timestamp">
        </label>
      </div>
      <div class="actions">
        <button type="submit">Apply Filters</button>
        <a class="button secondary" href="/tasks">Clear</a>
        <a class="button secondary" href="/api/tasks?{{query .Filters}}">JSON Endpoint</a>
      </div>
    </form>
  </section>

  <section class="panel">
    <div class="actions">
      <span class="muted">{{len .Tasks}} task(s)</span>
    </div>
    <table>
      <thead>
        <tr>
          <th>Handle</th>
          <th>Title</th>
          <th>Status</th>
          <th>Tags</th>
          <th>Updated</th>
        </tr>
      </thead>
      <tbody>
      {{range .Tasks}}
        <tr>
          <td data-label="Handle"><a class="mono" href="/tasks/{{.Handle}}">{{.Handle}}</a></td>
          <td data-label="Title">{{.Title}}</td>
          <td data-label="Status">{{.Status}}</td>
          <td data-label="Tags">
            <div class="chips">
              {{range .Tags}}<span class="chip">{{.}}</span>{{end}}
            </div>
          </td>
          <td data-label="Updated" class="mono">{{.UpdatedAt}}</td>
        </tr>
      {{else}}
        <tr>
          <td colspan="5">No tasks matched the current filters.</td>
        </tr>
      {{end}}
      </tbody>
    </table>
  </section>
{{template "page-end" .}}
{{end}}
`

const taskDetailTemplate = `
{{define "task-detail"}}
{{template "page-start" .}}
  <section class="hero">
    <div class="actions">
      <a class="button secondary" href="{{.BackURL}}">Back to tasks</a>
      <span class="chip mono">{{.Task.Handle}}</span>
      <span class="chip">{{.Task.Status}}</span>
    </div>
    <h1>{{.Task.Title}}</h1>
    <p>{{.Task.Description}}</p>
  </section>

  <section class="summary-grid">
    <article class="summary-card">
      <h2>Task Metadata</h2>
      <p class="mono">UUID: {{.Task.UUID}}</p>
      <p class="mono">Created: {{.Task.CreatedAt}}</p>
      <p class="mono">Updated: {{.Task.UpdatedAt}}</p>
      {{if .Task.DueAt}}<p class="mono">Due: {{.Task.DueAt}}</p>{{end}}
      {{if .Task.ClosedAt}}<p class="mono">Closed: {{.Task.ClosedAt}}</p>{{end}}
    </article>

    <article class="summary-card">
      <h2>Classification</h2>
      {{if .Task.DomainID}}<p class="mono">Domain: {{.Task.DomainID}}</p>{{else}}<p>Unclassified domain.</p>{{end}}
      {{if .Task.ProjectID}}<p class="mono">Project: {{.Task.ProjectID}}</p>{{else}}<p>Unclassified project.</p>{{end}}
      {{if .Task.MilestoneHandle}}<p class="mono">Milestone: {{.Task.MilestoneHandle}}</p>{{else}}<p>Unassigned milestone.</p>{{end}}
      {{if .Task.AssigneeActorID}}<p class="mono">Assignee: {{.Task.AssigneeActorID}}</p>{{else}}<p>Unassigned.</p>{{end}}
    </article>

    <article class="summary-card">
      <h2>Tags</h2>
      {{if .Task.Tags}}
        <div class="chips">
          {{range .Task.Tags}}<span class="chip">{{.}}</span>{{end}}
        </div>
      {{else}}
        <p>No tags.</p>
      {{end}}
    </article>
  </section>

  <section class="panel">
    <h2>External Links</h2>
    {{if .Links}}
      <ul>
      {{range .Links}}
        <li><strong>{{.Type}}</strong>: <a href="{{.Target}}">{{.Target}}</a></li>
      {{end}}
      </ul>
    {{else}}
      <p class="muted">No external links.</p>
    {{end}}
  </section>

  <section class="panel">
    <h2>Relationships</h2>
    {{if .Relationships}}
      <ul>
      {{range .Relationships}}
        <li><span class="mono">{{.SourceTask}}</span> {{.Type}} <span class="mono">{{.TargetTask}}</span></li>
      {{end}}
      </ul>
    {{else}}
      <p class="muted">No relationships.</p>
    {{end}}
  </section>
{{template "page-end" .}}
{{end}}
`
