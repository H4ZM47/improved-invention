package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	taskdb "github.com/H4ZM47/improved-invention/internal/db"
)

// ViewManager provides service-layer saved-view workflows.
type ViewManager struct {
	DB *sql.DB
}

// Create persists a new saved view.
func (m ViewManager) Create(ctx context.Context, req CreateViewRequest) (ViewRecord, error) {
	filtersJSON, err := marshalSavedViewFilters(req.Filters)
	if err != nil {
		return ViewRecord{}, err
	}

	view, err := taskdb.CreateSavedView(ctx, m.DB, taskdb.SavedViewInput{
		Name:        req.Name,
		EntityType:  "task",
		FiltersJSON: filtersJSON,
	})
	if err != nil {
		return ViewRecord{}, err
	}
	return toViewRecord(view)
}

// List returns all saved views in deterministic order.
func (m ViewManager) List(ctx context.Context, _ ListViewsRequest) ([]ViewRecord, error) {
	views, err := taskdb.ListSavedViews(ctx, m.DB)
	if err != nil {
		return nil, err
	}

	records := make([]ViewRecord, 0, len(views))
	for _, view := range views {
		record, err := toViewRecord(view)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, nil
}

// Show resolves a single saved view by name.
func (m ViewManager) Show(ctx context.Context, req ShowViewRequest) (ViewRecord, error) {
	view, err := taskdb.FindSavedView(ctx, m.DB, req.Name)
	if err != nil {
		return ViewRecord{}, err
	}
	return toViewRecord(view)
}

// Update replaces the filters and optionally the name of a saved view.
func (m ViewManager) Update(ctx context.Context, req UpdateViewRequest) (ViewRecord, error) {
	filtersJSON, err := marshalSavedViewFilters(req.Filters)
	if err != nil {
		return ViewRecord{}, err
	}

	targetName := req.NewName
	if targetName == "" {
		targetName = req.Name
	}

	view, err := taskdb.UpdateSavedView(ctx, m.DB, req.Name, taskdb.SavedViewInput{
		Name:        targetName,
		EntityType:  "task",
		FiltersJSON: filtersJSON,
	})
	if err != nil {
		return ViewRecord{}, err
	}
	return toViewRecord(view)
}

// Delete removes a saved view by name.
func (m ViewManager) Delete(ctx context.Context, req DeleteViewRequest) error {
	return taskdb.DeleteSavedView(ctx, m.DB, req.Name)
}

func toViewRecord(view taskdb.SavedView) (ViewRecord, error) {
	filters, err := unmarshalSavedViewFilters(view.FiltersJSON)
	if err != nil {
		return ViewRecord{}, fmt.Errorf("decode saved view %q filters: %w", view.Name, err)
	}
	return ViewRecord{
		Name:       view.Name,
		UUID:       view.UUID,
		EntityType: view.EntityType,
		Filters:    filters,
		CreatedAt:  view.CreatedAt,
		UpdatedAt:  view.UpdatedAt,
	}, nil
}

func marshalSavedViewFilters(filters SavedViewFilters) (string, error) {
	payload, err := json.Marshal(filters)
	if err != nil {
		return "", fmt.Errorf("encode saved view filters: %w", err)
	}
	return string(payload), nil
}

func unmarshalSavedViewFilters(payload string) (SavedViewFilters, error) {
	var filters SavedViewFilters
	if payload == "" {
		return filters, nil
	}
	if err := json.Unmarshal([]byte(payload), &filters); err != nil {
		return SavedViewFilters{}, err
	}
	return filters, nil
}

// ToListTasksRequest converts saved view filters into a ListTasksRequest.
//
// Empty-string scalar fields are treated as "no filter" and left nil; non-empty
// values become pointers for the request.
func (f SavedViewFilters) ToListTasksRequest() ListTasksRequest {
	req := ListTasksRequest{
		Statuses: f.Statuses,
		Tags:     f.Tags,
		Search:   f.Search,
	}
	req.DomainRef = stringPtrIfSet(f.DomainRef)
	req.ProjectRef = stringPtrIfSet(f.ProjectRef)
	req.AssigneeRef = stringPtrIfSet(f.AssigneeRef)
	req.DueBefore = stringPtrIfSet(f.DueBefore)
	req.DueAfter = stringPtrIfSet(f.DueAfter)
	return req
}

func stringPtrIfSet(value string) *string {
	if value == "" {
		return nil
	}
	v := value
	return &v
}
