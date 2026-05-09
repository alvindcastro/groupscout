package storage

import (
	"context"
	"testing"
)

func TestLeadStore_ListFiltered(t *testing.T) {
	db, dsn := newTestSQLiteDB(t)
	store := NewLeadStoreWithDSN(db, dsn)
	ctx := context.Background()

	leads := []*Lead{
		{
			Source:        "richmond_permits",
			Title:         "Airport hotel tower",
			Location:      "Richmond, BC",
			PriorityScore: 9,
			Status:        "new",
		},
		{
			Source:        "eventbrite",
			Title:         "Downtown convention",
			Location:      "Vancouver, BC",
			PriorityScore: 7,
			Status:        "new",
		},
		{
			Source:        "richmond_permits",
			Title:         "Small retail renovation",
			Location:      "Richmond, BC",
			PriorityScore: 3,
			Status:        "contacted",
		},
	}
	for _, lead := range leads {
		if err := store.Insert(ctx, lead); err != nil {
			t.Fatalf("Insert %q: %v", lead.Title, err)
		}
	}

	got, next, err := store.ListFiltered(ctx, LeadListFilter{
		Status:   "new",
		Source:   "richmond_permits",
		MinScore: 8,
		Query:    "airport",
		Limit:    25,
	})
	if err != nil {
		t.Fatalf("ListFiltered: %v", err)
	}
	if next != "" {
		t.Fatalf("next cursor = %q, want empty for single result", next)
	}
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1: %#v", len(got), got)
	}
	if got[0].ID != leads[0].ID {
		t.Fatalf("got lead ID = %q, want %q", got[0].ID, leads[0].ID)
	}
}

func TestLeadStore_ListFilteredLimitReturnsNextCursor(t *testing.T) {
	db, dsn := newTestSQLiteDB(t)
	store := NewLeadStoreWithDSN(db, dsn)
	ctx := context.Background()

	for _, title := range []string{"A", "B", "C"} {
		if err := store.Insert(ctx, &Lead{Source: "test", Title: title, PriorityScore: 5, Status: "new"}); err != nil {
			t.Fatalf("Insert %q: %v", title, err)
		}
	}

	page, next, err := store.ListFiltered(ctx, LeadListFilter{Limit: 2})
	if err != nil {
		t.Fatalf("ListFiltered first page: %v", err)
	}
	if len(page) != 2 {
		t.Fatalf("len(first page) = %d, want 2", len(page))
	}
	if next == "" {
		t.Fatal("next cursor should be set when more leads exist")
	}

	second, next, err := store.ListFiltered(ctx, LeadListFilter{Limit: 2, Cursor: next})
	if err != nil {
		t.Fatalf("ListFiltered second page: %v", err)
	}
	if len(second) != 1 {
		t.Fatalf("len(second page) = %d, want 1", len(second))
	}
	if next != "" {
		t.Fatalf("second page next cursor = %q, want empty", next)
	}
}
