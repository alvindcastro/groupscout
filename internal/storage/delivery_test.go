package storage

import (
	"context"
	"testing"
	"time"
)

func TestDeliveryStore_UpsertResult_IsIdempotentAfterSent(t *testing.T) {
	db, dsn := newTestSQLiteDB(t)
	store := NewDeliveryStore(db, dsn)
	ctx := context.Background()

	first := LeadDelivery{
		IdempotencyKey: "lead-cadence:2026-05-27:wednesday",
		ScheduleKey:    "lead-cadence:2026-05-27:wednesday",
		LeadID:         "lead-1",
		Status:         "sent",
		Result:         "sent one lead",
	}
	if err := store.UpsertResult(ctx, first); err != nil {
		t.Fatalf("UpsertResult first: %v", err)
	}

	second := first
	second.LeadID = "lead-2"
	second.Status = "sent"
	second.Result = "duplicate send should not overwrite"
	if err := store.UpsertResult(ctx, second); err != nil {
		t.Fatalf("UpsertResult second: %v", err)
	}

	got, err := store.GetByIdempotencyKey(ctx, first.IdempotencyKey)
	if err != nil {
		t.Fatalf("GetByIdempotencyKey: %v", err)
	}
	if got == nil {
		t.Fatal("expected delivery record")
	}
	if got.LeadID != "lead-1" {
		t.Fatalf("LeadID = %q, want lead-1", got.LeadID)
	}
	if got.Status != "sent" {
		t.Fatalf("Status = %q, want sent", got.Status)
	}
}

func TestDeliveryStore_TryAcquireRunLock(t *testing.T) {
	db, dsn := newTestSQLiteDB(t)
	store := NewDeliveryStore(db, dsn)
	ctx := context.Background()

	locked, err := store.TryAcquireRunLock(ctx, "lead-delivery", "owner-1", time.Minute)
	if err != nil {
		t.Fatalf("TryAcquireRunLock first: %v", err)
	}
	if !locked {
		t.Fatal("first lock attempt should acquire")
	}

	locked, err = store.TryAcquireRunLock(ctx, "lead-delivery", "owner-2", time.Minute)
	if err != nil {
		t.Fatalf("TryAcquireRunLock second: %v", err)
	}
	if locked {
		t.Fatal("second lock attempt should not acquire before expiry")
	}

	if err := store.ReleaseRunLock(ctx, "lead-delivery", "owner-1"); err != nil {
		t.Fatalf("ReleaseRunLock: %v", err)
	}
	locked, err = store.TryAcquireRunLock(ctx, "lead-delivery", "owner-2", time.Minute)
	if err != nil {
		t.Fatalf("TryAcquireRunLock after release: %v", err)
	}
	if !locked {
		t.Fatal("lock should acquire after release")
	}
}

func TestLeadStore_ListDeliveryCandidates_PrefersNewAndExcludesSent(t *testing.T) {
	db, dsn := newTestSQLiteDB(t)
	leadStore := NewLeadStoreWithDSN(db, dsn)
	deliveryStore := NewDeliveryStore(db, dsn)
	ctx := context.Background()

	sent := &Lead{Source: "test", Title: "Already sent", Status: "new", PriorityScore: 10}
	fresh := &Lead{Source: "test", Title: "Fresh lead", Status: "new", PriorityScore: 8}
	backlog := &Lead{Source: "test", Title: "Backlog lead", Status: "notified", PriorityScore: 9}
	for _, lead := range []*Lead{sent, fresh, backlog} {
		if err := leadStore.Insert(ctx, lead); err != nil {
			t.Fatalf("Insert %q: %v", lead.Title, err)
		}
	}
	if err := deliveryStore.UpsertResult(ctx, LeadDelivery{
		IdempotencyKey: "older-cadence",
		ScheduleKey:    "older-cadence",
		LeadID:         sent.ID,
		Status:         "sent",
	}); err != nil {
		t.Fatalf("UpsertResult: %v", err)
	}

	candidates, err := leadStore.ListDeliveryCandidates(ctx, 2)
	if err != nil {
		t.Fatalf("ListDeliveryCandidates: %v", err)
	}
	if len(candidates) != 2 {
		t.Fatalf("len(candidates) = %d, want 2", len(candidates))
	}
	if candidates[0].ID != fresh.ID {
		t.Fatalf("first candidate = %q, want fresh new lead", candidates[0].Title)
	}
	if candidates[1].ID != backlog.ID {
		t.Fatalf("second candidate = %q, want backlog lead", candidates[1].Title)
	}
}
