//go:build integration

package storage

import (
	"context"
	"testing"
	"time"
)

func TestLoadActivity_Postgres(t *testing.T) {
	db, dsn := newTestDB(t)
	ctx := context.Background()

	empty, err := LoadActivity(ctx, db, dsn)
	if err != nil {
		t.Fatalf("LoadActivity on empty tables: %v", err)
	}
	if !empty.CadenceRunAt.IsZero() || !empty.LeadCreatedAt.IsZero() {
		t.Fatalf("empty tables: got %+v, want zero times", empty)
	}

	lead := &Lead{Source: "activity-integration", Title: "Newest lead", Status: "new", PriorityScore: 7}
	if err := NewLeadStoreWithDSN(db, dsn).Insert(ctx, lead); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	deliveries := NewDeliveryStore(db, dsn)
	for _, d := range []LeadDelivery{
		{IdempotencyKey: "lead-cadence:2026-10-13:tuesday", ScheduleKey: "lead-cadence:2026-10-13:tuesday", Status: "no_eligible_lead"},
		{IdempotencyKey: "lead-cadence:2026-10-15:thursday", ScheduleKey: "lead-cadence:2026-10-15:thursday", Status: "failed", Result: "boom"},
	} {
		if err := deliveries.UpsertResult(ctx, d); err != nil {
			t.Fatalf("UpsertResult %s: %v", d.Status, err)
		}
	}
	// The failed run is the newest row, but it must not count as a completed run.
	if _, err := db.ExecContext(ctx, `UPDATE lead_deliveries SET updated_at = NOW() - INTERVAL '2 days' WHERE status = 'no_eligible_lead'`); err != nil {
		t.Fatal(err)
	}

	got, err := LoadActivity(ctx, db, dsn)
	if err != nil {
		t.Fatalf("LoadActivity: %v", err)
	}
	if age := time.Since(got.CadenceRunAt); age < 47*time.Hour || age > 49*time.Hour {
		t.Errorf("CadenceRunAt = %s (age %s), want the no_eligible_lead run two days ago", got.CadenceRunAt, age)
	}
	if age := time.Since(got.LeadCreatedAt); age < 0 || age > time.Minute {
		t.Errorf("LeadCreatedAt = %s (age %s), want just now", got.LeadCreatedAt, age)
	}
}
