//go:build integration

package storage

import (
	"context"
	"database/sql"
	"os"
	"testing"
)

func newTestDB(t *testing.T) (*sql.DB, string) {
	t.Helper()
	dsn := os.Getenv("TEST_POSTGRES_URL")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_URL not set")
	}
	db, err := Open(dsn)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := Migrate(db, dsn); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	t.Cleanup(func() {
		db.Exec("DELETE FROM leads")
		db.Exec("DELETE FROM raw_projects")
		db.Exec("DELETE FROM raw_inputs")
		db.Close()
	})
	return db, dsn
}

func TestLeadStore_Insert_and_ListNew(t *testing.T) {
	db, dsn := newTestDB(t)
	store := NewLeadStoreWithDSN(db, dsn)
	ctx := context.Background()

	lead := &Lead{
		Source:                  "richmond_permits",
		Title:                   "Test Warehouse — 1234 No. 3 Road",
		Location:                "Richmond, BC",
		ProjectValue:            5_000_000,
		GeneralContractor:       "PCL Construction",
		ProjectType:             "industrial",
		EstimatedCrewSize:       80,
		EstimatedDurationMonths: 6,
		OutOfTownCrewLikely:     true,
		PriorityScore:           9,
		PriorityReason:          "Large industrial near YVR",
		Status:                  "new",
	}

	if err := store.Insert(ctx, lead); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	if lead.ID == "" {
		t.Error("ID should be populated after Insert")
	}

	leads, err := store.ListNew(ctx)
	if err != nil {
		t.Fatalf("ListNew: %v", err)
	}
	if len(leads) == 0 {
		t.Fatal("expected at least one lead")
	}

	got := leads[0]
	if got.OutOfTownCrewLikely != true {
		t.Errorf("OutOfTownCrewLikely = %v, want true (bool round-trip failed)", got.OutOfTownCrewLikely)
	}
	if got.ProjectValue != 5_000_000 {
		t.Errorf("ProjectValue = %d, want 5000000", got.ProjectValue)
	}
	if got.PriorityScore != 9 {
		t.Errorf("PriorityScore = %d, want 9", got.PriorityScore)
	}
}

func TestLeadStore_WithRawInputID(t *testing.T) {
	db, dsn := newTestDB(t)
	store := NewLeadStoreWithDSN(db, dsn)
	auditStore := NewAuditStoreWithDSN(db, dsn)
	ctx := context.Background()

	rawID, err := auditStore.Store(ctx, RawInput{
		Hash:    "test-hash-2",
		Payload: []byte("test payload"),
	})
	if err != nil {
		t.Fatalf("Store raw input: %v", err)
	}

	lead := &Lead{
		Source:     "test",
		Title:      "Lead with Audit",
		RawInputID: rawID.String(),
		Status:     "new",
	}

	if err := store.Insert(ctx, lead); err != nil {
		t.Fatalf("Insert: %v", err)
	}

	leads, err := store.ListNew(ctx)
	if err != nil {
		t.Fatalf("ListNew: %v", err)
	}

	var got *Lead
	for _, l := range leads {
		if l.ID == lead.ID {
			got = &l
			break
		}
	}
	if got == nil {
		t.Fatal("lead not found")
	}
	if got.RawInputID != rawID.String() {
		t.Errorf("RawInputID = %q, want %q", got.RawInputID, rawID.String())
	}
}

func TestLeadStore_bool_false_roundtrip(t *testing.T) {
	db, dsn := newTestDB(t)
	store := NewLeadStoreWithDSN(db, dsn)
	ctx := context.Background()

	lead := &Lead{
		Source:              "test",
		Title:               "Local renovation",
		OutOfTownCrewLikely: false,
		Status:              "new",
	}
	store.Insert(ctx, lead)

	leads, _ := store.ListNew(ctx)
	for _, l := range leads {
		if l.Title == "Local renovation" && l.OutOfTownCrewLikely != false {
			t.Errorf("OutOfTownCrewLikely = %v, want false", l.OutOfTownCrewLikely)
		}
	}
}

func TestLeadStore_UpdateStatus(t *testing.T) {
	db, dsn := newTestDB(t)
	store := NewLeadStoreWithDSN(db, dsn)
	ctx := context.Background()

	lead := &Lead{Source: "test", Title: "Status test", Status: "new"}
	store.Insert(ctx, lead)

	if err := store.UpdateStatus(ctx, lead.ID, "contacted"); err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}

	leads, _ := store.ListNew(ctx)
	for _, l := range leads {
		if l.ID == lead.ID {
			t.Errorf("lead %s should not be 'new' anymore", lead.ID)
		}
	}
}
