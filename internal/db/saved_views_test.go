package db

import (
	"context"
	"errors"
	"testing"
)

func TestCreateSavedViewPersistsNameAndFilters(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)

	view, err := CreateSavedView(context.Background(), db, SavedViewInput{
		Name:        "Active Work",
		FiltersJSON: `{"statuses":["active"]}`,
	})
	if err != nil {
		t.Fatalf("CreateSavedView() error = %v", err)
	}

	if view.Name != "Active Work" {
		t.Fatalf("view.Name = %q, want %q", view.Name, "Active Work")
	}
	if view.EntityType != "task" {
		t.Fatalf("view.EntityType = %q, want %q", view.EntityType, "task")
	}
	if view.FiltersJSON != `{"statuses":["active"]}` {
		t.Fatalf("view.FiltersJSON = %q", view.FiltersJSON)
	}
	if view.UUID == "" {
		t.Fatal("view.UUID is empty")
	}
}

func TestCreateSavedViewRejectsDuplicateName(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)
	if _, err := CreateSavedView(context.Background(), db, SavedViewInput{Name: "Dup"}); err != nil {
		t.Fatalf("CreateSavedView() error = %v", err)
	}

	_, err := CreateSavedView(context.Background(), db, SavedViewInput{Name: "Dup"})
	if !errors.Is(err, ErrSavedViewNameTaken) {
		t.Fatalf("CreateSavedView() err = %v, want ErrSavedViewNameTaken", err)
	}
}

func TestCreateSavedViewRejectsInvalidJSON(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)
	_, err := CreateSavedView(context.Background(), db, SavedViewInput{
		Name:        "Bad",
		FiltersJSON: `{not json`,
	})
	if err == nil {
		t.Fatal("CreateSavedView() expected error for invalid JSON")
	}
}

func TestListSavedViewsOrdersByName(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)
	for _, name := range []string{"Zeta", "Alpha", "Mu"} {
		if _, err := CreateSavedView(context.Background(), db, SavedViewInput{Name: name}); err != nil {
			t.Fatalf("CreateSavedView(%q) error = %v", name, err)
		}
	}

	views, err := ListSavedViews(context.Background(), db)
	if err != nil {
		t.Fatalf("ListSavedViews() error = %v", err)
	}

	gotNames := []string{views[0].Name, views[1].Name, views[2].Name}
	wantNames := []string{"Alpha", "Mu", "Zeta"}
	for i := range wantNames {
		if gotNames[i] != wantNames[i] {
			t.Fatalf("views[%d].Name = %q, want %q", i, gotNames[i], wantNames[i])
		}
	}
}

func TestUpdateSavedViewReplacesFiltersAndName(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)
	if _, err := CreateSavedView(context.Background(), db, SavedViewInput{
		Name:        "Original",
		FiltersJSON: `{}`,
	}); err != nil {
		t.Fatalf("CreateSavedView() error = %v", err)
	}

	view, err := UpdateSavedView(context.Background(), db, "Original", SavedViewInput{
		Name:        "Renamed",
		FiltersJSON: `{"search":"cli"}`,
	})
	if err != nil {
		t.Fatalf("UpdateSavedView() error = %v", err)
	}

	if view.Name != "Renamed" {
		t.Fatalf("view.Name = %q, want %q", view.Name, "Renamed")
	}
	if view.FiltersJSON != `{"search":"cli"}` {
		t.Fatalf("view.FiltersJSON = %q", view.FiltersJSON)
	}

	if _, err := FindSavedView(context.Background(), db, "Original"); !errors.Is(err, ErrSavedViewNotFound) {
		t.Fatalf("FindSavedView(Original) err = %v, want ErrSavedViewNotFound", err)
	}
}

func TestUpdateSavedViewRenameOnlyPreservesExistingFilters(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)
	if _, err := CreateSavedView(context.Background(), db, SavedViewInput{
		Name:        "Original",
		FiltersJSON: `{"statuses":["backlog"]}`,
	}); err != nil {
		t.Fatalf("CreateSavedView() error = %v", err)
	}

	view, err := UpdateSavedView(context.Background(), db, "Original", SavedViewInput{
		Name:        "Renamed",
		FiltersJSON: `{"statuses":["backlog"]}`,
	})
	if err != nil {
		t.Fatalf("UpdateSavedView() error = %v", err)
	}

	if got, want := view.Name, "Renamed"; got != want {
		t.Fatalf("view.Name = %q, want %q", got, want)
	}
	if got, want := view.FiltersJSON, `{"statuses":["backlog"]}`; got != want {
		t.Fatalf("view.FiltersJSON = %q, want %q", got, want)
	}
}

func TestUpdateSavedViewMissingReturnsNotFound(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)
	_, err := UpdateSavedView(context.Background(), db, "Missing", SavedViewInput{Name: "Missing"})
	if !errors.Is(err, ErrSavedViewNotFound) {
		t.Fatalf("UpdateSavedView() err = %v, want ErrSavedViewNotFound", err)
	}
}

func TestDeleteSavedViewRemovesRow(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)
	if _, err := CreateSavedView(context.Background(), db, SavedViewInput{Name: "Temp"}); err != nil {
		t.Fatalf("CreateSavedView() error = %v", err)
	}

	if err := DeleteSavedView(context.Background(), db, "Temp"); err != nil {
		t.Fatalf("DeleteSavedView() error = %v", err)
	}

	if _, err := FindSavedView(context.Background(), db, "Temp"); !errors.Is(err, ErrSavedViewNotFound) {
		t.Fatalf("FindSavedView() err = %v, want ErrSavedViewNotFound", err)
	}
}

func TestDeleteSavedViewMissingReturnsNotFound(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)
	err := DeleteSavedView(context.Background(), db, "Nope")
	if !errors.Is(err, ErrSavedViewNotFound) {
		t.Fatalf("DeleteSavedView() err = %v, want ErrSavedViewNotFound", err)
	}
}
