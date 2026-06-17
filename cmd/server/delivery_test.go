package main

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/alvindcastro/groupscout/internal/storage"
)

type captureNotifier struct {
	leads []storage.Lead
	err   error
}

func (n *captureNotifier) Send(ctx context.Context, leads []storage.Lead) error {
	n.leads = append(n.leads, leads...)
	return n.err
}

func TestNormalizeDeliveryOptions_DefaultsCadenceKey(t *testing.T) {
	now := time.Date(2026, 5, 27, 9, 0, 0, 0, time.UTC)

	got := normalizeDeliveryOptions(PipelineRunOptions{}, now)

	if got.ScheduleKey != "lead-cadence:2026-05-27:wednesday" {
		t.Fatalf("ScheduleKey = %q", got.ScheduleKey)
	}
	if got.IdempotencyKey != got.ScheduleKey {
		t.Fatalf("IdempotencyKey = %q, want schedule key", got.IdempotencyKey)
	}
}

func TestDeliverGuaranteedLead_SendsExactlyOneCandidate(t *testing.T) {
	db, dsn := storageTestDB(t)
	leadStore := storage.NewLeadStoreWithDSN(db, dsn)
	deliveryStore := storage.NewDeliveryStore(db, dsn)
	ctx := context.Background()

	first := &storage.Lead{Source: "test", Title: "Top lead", Status: "new", PriorityScore: 10}
	second := &storage.Lead{Source: "test", Title: "Second lead", Status: "new", PriorityScore: 9}
	for _, lead := range []*storage.Lead{first, second} {
		if err := leadStore.Insert(ctx, lead); err != nil {
			t.Fatalf("Insert %q: %v", lead.Title, err)
		}
	}

	notifier := &captureNotifier{}
	delivery, err := deliverGuaranteedLead(ctx, leadStore, deliveryStore, notifier, nil, nil, PipelineRunOptions{
		IdempotencyKey: "lead-cadence:2026-05-27:wednesday",
		ScheduleKey:    "lead-cadence:2026-05-27:wednesday",
	})
	if err != nil {
		t.Fatalf("deliverGuaranteedLead: %v", err)
	}

	if delivery.Status != "sent" {
		t.Fatalf("delivery.Status = %q, want sent", delivery.Status)
	}
	if delivery.LeadID != first.ID {
		t.Fatalf("delivery.LeadID = %q, want first lead", delivery.LeadID)
	}
	if len(notifier.leads) != 1 {
		t.Fatalf("sent %d leads, want 1", len(notifier.leads))
	}
	if notifier.leads[0].ID != first.ID {
		t.Fatalf("sent lead = %q, want first lead", notifier.leads[0].ID)
	}

	updated, err := leadStore.GetByID(ctx, first.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if updated.Status != "notified" {
		t.Fatalf("updated status = %q, want notified", updated.Status)
	}
}

func TestDeliverGuaranteedLead_RecordsNoEligibleLead(t *testing.T) {
	db, dsn := storageTestDB(t)
	leadStore := storage.NewLeadStoreWithDSN(db, dsn)
	deliveryStore := storage.NewDeliveryStore(db, dsn)
	ctx := context.Background()

	delivery, err := deliverGuaranteedLead(ctx, leadStore, deliveryStore, &captureNotifier{}, nil, nil, PipelineRunOptions{
		IdempotencyKey: "lead-cadence:2026-05-27:wednesday",
		ScheduleKey:    "lead-cadence:2026-05-27:wednesday",
	})
	if err != nil {
		t.Fatalf("deliverGuaranteedLead: %v", err)
	}
	if delivery.Status != "no_eligible_lead" {
		t.Fatalf("delivery.Status = %q, want no_eligible_lead", delivery.Status)
	}

	got, err := deliveryStore.GetByIdempotencyKey(ctx, "lead-cadence:2026-05-27:wednesday")
	if err != nil {
		t.Fatalf("GetByIdempotencyKey: %v", err)
	}
	if got == nil || got.Status != "no_eligible_lead" {
		t.Fatalf("stored delivery = %#v, want no_eligible_lead", got)
	}
}

func storageTestDB(t *testing.T) (*sql.DB, string) {
	t.Helper()
	db, err := storage.Open(":memory:")
	if err != nil {
		t.Fatalf("Open SQLite: %v", err)
	}
	if err := storage.Migrate(db, ":memory:"); err != nil {
		t.Fatalf("Migrate SQLite: %v", err)
	}
	t.Cleanup(func() {
		db.Close()
	})
	return db, ":memory:"
}
