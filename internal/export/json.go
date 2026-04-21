package export

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/H4ZM47/grind/internal/app"
)

type jsonDocument struct {
	Version       int                      `json:"version"`
	Tasks         []app.TaskRecord         `json:"tasks"`
	Domains       []app.DomainRecord       `json:"domains"`
	Projects      []app.ProjectRecord      `json:"projects"`
	Milestones    []app.MilestoneRecord    `json:"milestones"`
	Actors        []app.ActorRecord        `json:"actors"`
	Links         []app.LinkRecord         `json:"links"`
	Relationships []app.RelationshipRecord `json:"relationships"`
}

// EncodeJSON serializes a Bundle as a deterministic JSON document.
//
// Entity collections that are empty in the Bundle become empty arrays rather
// than null. Output is indented with two spaces and ends with a single
// trailing newline so diffs are clean.
func EncodeJSON(bundle Bundle) ([]byte, error) {
	doc := jsonDocument{
		Version:       DocumentVersion,
		Tasks:         normalizeTasks(bundle.Tasks),
		Domains:       normalizeDomains(bundle.Domains),
		Projects:      normalizeProjects(bundle.Projects),
		Milestones:    normalizeMilestones(bundle.Milestones),
		Actors:        normalizeSlice(bundle.Actors),
		Links:         normalizeLinks(bundle.Links),
		Relationships: normalizeSlice(bundle.Relationships),
	}

	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(doc); err != nil {
		return nil, fmt.Errorf("encode export document: %w", err)
	}
	return buf.Bytes(), nil
}

func normalizeSlice[T any](in []T) []T {
	if in == nil {
		return []T{}
	}
	return in
}

func normalizeTasks(tasks []app.TaskRecord) []app.TaskRecord {
	out := make([]app.TaskRecord, len(tasks))
	for i, t := range tasks {
		if t.Tags == nil {
			t.Tags = []string{}
		}
		out[i] = t
	}
	return out
}

func normalizeDomains(domains []app.DomainRecord) []app.DomainRecord {
	out := make([]app.DomainRecord, len(domains))
	for i, d := range domains {
		if d.Tags == nil {
			d.Tags = []string{}
		}
		out[i] = d
	}
	return out
}

func normalizeProjects(projects []app.ProjectRecord) []app.ProjectRecord {
	out := make([]app.ProjectRecord, len(projects))
	for i, p := range projects {
		if p.Tags == nil {
			p.Tags = []string{}
		}
		out[i] = p
	}
	return out
}

func normalizeMilestones(milestones []app.MilestoneRecord) []app.MilestoneRecord {
	if milestones == nil {
		return []app.MilestoneRecord{}
	}
	out := make([]app.MilestoneRecord, len(milestones))
	copy(out, milestones)
	return out
}

func normalizeLinks(links []app.LinkRecord) []app.LinkRecord {
	out := make([]app.LinkRecord, len(links))
	for i, l := range links {
		if l.Metadata == nil {
			l.Metadata = map[string]string{}
		}
		out[i] = l
	}
	return out
}
