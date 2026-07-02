//go:build integration

package storage

import (
	"context"
	"database/sql"
	"os"
	"testing"
)

// testAdvisoryLockKey serializes integration tests that share this database.
// `go test ./pkgA ./pkgB` builds and runs each package's test binary as a
// separate OS process, and those processes run concurrently against the one
// shared TEST_POSTGRES_URL database. Without serialization, one package's
// `DELETE FROM raw_inputs` races another package's lead insert and trips
// leads_raw_input_id_fkey (SQLSTATE 23503). A session-level Postgres advisory
// lock held on a pinned connection for the lifetime of each test makes the
// test bodies mutually exclusive across every package and process. The
// enrichment package uses the same key so the two suites serialize together.
const testAdvisoryLockKey = 0x67734954 // "gsIT"

func acquireTestLock(t *testing.T, db *sql.DB) *sql.Conn {
	t.Helper()
	ctx := context.Background()
	conn, err := db.Conn(ctx)
	if err != nil {
		t.Fatalf("advisory lock conn: %v", err)
	}
	if _, err := conn.ExecContext(ctx, "SELECT pg_advisory_lock($1)", testAdvisoryLockKey); err != nil {
		conn.Close()
		t.Fatalf("pg_advisory_lock: %v", err)
	}
	return conn
}

func releaseTestLock(conn *sql.Conn) {
	if conn == nil {
		return
	}
	conn.ExecContext(context.Background(), "SELECT pg_advisory_unlock($1)", testAdvisoryLockKey)
	conn.Close()
}

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

	lockConn := acquireTestLock(t, db)
	clean := func() {
		db.Exec("DELETE FROM delivery_locks")
		db.Exec("DELETE FROM lead_deliveries")
		db.Exec("DELETE FROM leads")
		db.Exec("DELETE FROM raw_projects")
		db.Exec("DELETE FROM raw_inputs")
	}
	clean()
	t.Cleanup(func() {
		clean()
		releaseTestLock(lockConn)
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

func TestLeadStore_GetByID(t *testing.T) {
	db, dsn := newTestDB(t)
	store := NewLeadStoreWithDSN(db, dsn)
	ctx := context.Background()

	lead := &Lead{Source: "test", Title: "GetByID test", Status: "new"}
	if err := store.Insert(ctx, lead); err != nil {
		t.Fatalf("Insert: %v", err)
	}

	got, err := store.GetByID(ctx, lead.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got == nil {
		t.Fatal("expected lead, got nil")
	}
	if got.Title != "GetByID test" {
		t.Errorf("Title = %q, want %q", got.Title, "GetByID test")
	}

	// Test non-existent
	got, err = store.GetByID(ctx, NewUUID())
	if err != nil {
		t.Fatalf("GetByID (non-existent): %v", err)
	}
	if got != nil {
		t.Fatal("expected nil for non-existent lead")
	}
}
