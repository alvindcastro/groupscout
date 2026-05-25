//go:build integration

package storage

import (
	"context"
	"testing"
	"time"
)

func TestDeliveryStore_PostgresIdempotencyAndCandidateFallback(t *testing.T) {
	db, dsn := newTestDB(t)
	leadStore := NewLeadStoreWithDSN(db, dsn)
	deliveryStore := NewDeliveryStore(db, dsn)
	ctx := context.Background()

	sent := &Lead{Source: "delivery-integration", Title: "Already delivered", Status: "new", PriorityScore: 10}
	fresh := &Lead{Source: "delivery-integration", Title: "Fresh candidate", Status: "new", PriorityScore: 8}
	backlog := &Lead{Source: "delivery-integration", Title: "Backlog candidate", Status: "notified", PriorityScore: 9}
	for _, lead := range []*Lead{sent, fresh, backlog} {
		if err := leadStore.Insert(ctx, lead); err != nil {
			t.Fatalf("Insert %q: %v", lead.Title, err)
		}
	}

	key := "lead-cadence:2026-05-27:wednesday:" + NewUUID()
	if err := deliveryStore.UpsertResult(ctx, LeadDelivery{
		IdempotencyKey: key,
		ScheduleKey:    key,
		LeadID:         sent.ID,
		Status:         "sent",
		Result:         "sent one lead",
	}); err != nil {
		t.Fatalf("UpsertResult sent: %v", err)
	}
	if err := deliveryStore.UpsertResult(ctx, LeadDelivery{
		IdempotencyKey: key,
		ScheduleKey:    key,
		LeadID:         fresh.ID,
		Status:         "sent",
		Result:         "must not overwrite sent lead",
	}); err != nil {
		t.Fatalf("UpsertResult duplicate sent: %v", err)
	}

	got, err := deliveryStore.GetByIdempotencyKey(ctx, key)
	if err != nil {
		t.Fatalf("GetByIdempotencyKey: %v", err)
	}
	if got == nil || got.LeadID != sent.ID {
		t.Fatalf("delivery LeadID = %#v, want %s", got, sent.ID)
	}

	candidates, err := leadStore.ListDeliveryCandidates(ctx, 2)
	if err != nil {
		t.Fatalf("ListDeliveryCandidates: %v", err)
	}
	if len(candidates) != 2 {
		t.Fatalf("len(candidates) = %d, want 2", len(candidates))
	}
	if candidates[0].ID != fresh.ID {
		t.Fatalf("first candidate = %q, want fresh", candidates[0].Title)
	}
	if candidates[1].ID != backlog.ID {
		t.Fatalf("second candidate = %q, want backlog", candidates[1].Title)
	}
}

func TestDeliveryStore_PostgresRunLock(t *testing.T) {
	db, dsn := newTestDB(t)
	store := NewDeliveryStore(db, dsn)
	ctx := context.Background()

	locked, err := store.TryAcquireRunLock(ctx, "lead-delivery-integration", "owner-1", time.Minute)
	if err != nil {
		t.Fatalf("TryAcquireRunLock first: %v", err)
	}
	if !locked {
		t.Fatal("first lock attempt should acquire")
	}

	locked, err = store.TryAcquireRunLock(ctx, "lead-delivery-integration", "owner-2", time.Minute)
	if err != nil {
		t.Fatalf("TryAcquireRunLock second: %v", err)
	}
	if locked {
		t.Fatal("second lock attempt should not acquire before release")
	}
}
