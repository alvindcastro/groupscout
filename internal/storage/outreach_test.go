package storage

import (
	"context"
	"testing"
	"time"
)

func TestOutreachStore_InsertAndListByLead(t *testing.T) {
	db, dsn := newTestSQLiteDB(t)
	leadStore := NewLeadStoreWithDSN(db, dsn)
	outreachStore := NewOutreachStoreWithDSN(db, dsn)
	ctx := context.Background()

	lead := &Lead{Source: "test", Title: "Outreach lead", Status: "new"}
	if err := leadStore.Insert(ctx, lead); err != nil {
		t.Fatalf("Insert lead: %v", err)
	}

	first, err := outreachStore.Insert(ctx, OutreachEvent{
		LeadID:  lead.ID,
		Contact: "gc@example.test",
		Channel: "email",
		Notes:   "Sent intro",
		Outcome: "sent",
	})
	if err != nil {
		t.Fatalf("Insert first outreach: %v", err)
	}
	time.Sleep(time.Millisecond)
	second, err := outreachStore.Insert(ctx, OutreachEvent{
		LeadID:  lead.ID,
		Contact: "gc@example.test",
		Channel: "phone",
		Notes:   "Left voicemail",
		Outcome: "left_voicemail",
	})
	if err != nil {
		t.Fatalf("Insert second outreach: %v", err)
	}

	events, next, err := outreachStore.ListByLead(ctx, lead.ID, OutreachListFilter{Limit: 10})
	if err != nil {
		t.Fatalf("ListByLead: %v", err)
	}
	if next != "" {
		t.Fatalf("next cursor = %q, want empty", next)
	}
	if len(events) != 2 {
		t.Fatalf("len(events) = %d, want 2", len(events))
	}
	if events[0].ID != second.ID || events[1].ID != first.ID {
		t.Fatalf("events not newest-first: got %q then %q", events[0].ID, events[1].ID)
	}
}

func TestLeadStore_ApplyActionTransitions(t *testing.T) {
	db, dsn := newTestSQLiteDB(t)
	store := NewLeadStoreWithDSN(db, dsn)
	ctx := context.Background()

	lead := &Lead{Source: "test", Title: "Action lead", Status: "new", VerificationState: "needs_review"}
	if err := store.Insert(ctx, lead); err != nil {
		t.Fatalf("Insert lead: %v", err)
	}

	claim, err := store.ApplyAction(ctx, lead.ID, LeadAction{
		Action: "claim",
		Owner:  "alex@example.test",
	})
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	if claim.Lead.Status != "claimed" || claim.Lead.Owner != "alex@example.test" {
		t.Fatalf("claim result = status %q owner %q", claim.Lead.Status, claim.Lead.Owner)
	}
	if claim.Lead.VerificationState != "needs_review" {
		t.Fatalf("verification_state changed to %q", claim.Lead.VerificationState)
	}

	contacted, err := store.ApplyAction(ctx, lead.ID, LeadAction{Action: "contacted"})
	if err != nil {
		t.Fatalf("contacted: %v", err)
	}
	if contacted.Lead.Status != "contacted" {
		t.Fatalf("status = %q, want contacted", contacted.Lead.Status)
	}

	won, err := store.ApplyAction(ctx, lead.ID, LeadAction{Action: "won"})
	if err != nil {
		t.Fatalf("won: %v", err)
	}
	if won.Lead.Status != "won" {
		t.Fatalf("status = %q, want won", won.Lead.Status)
	}

	_, err = store.ApplyAction(ctx, lead.ID, LeadAction{Action: "contacted"})
	if err == nil {
		t.Fatal("contacted after won should fail")
	}

	reopened, err := store.ApplyAction(ctx, lead.ID, LeadAction{Action: "reopen"})
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if reopened.Lead.Status != "new" {
		t.Fatalf("status = %q, want new", reopened.Lead.Status)
	}

	snoozeUntil := time.Now().UTC().Add(24 * time.Hour).Truncate(time.Second)
	snoozed, err := store.ApplyAction(ctx, lead.ID, LeadAction{Action: "snooze", SnoozedUntil: &snoozeUntil})
	if err != nil {
		t.Fatalf("snooze: %v", err)
	}
	if snoozed.Lead.Status != "snoozed" || snoozed.Lead.SnoozedUntil == nil {
		t.Fatalf("snooze result status=%q snoozed_until=%v", snoozed.Lead.Status, snoozed.Lead.SnoozedUntil)
	}

	flagged, err := store.ApplyAction(ctx, lead.ID, LeadAction{Action: "flag"})
	if err != nil {
		t.Fatalf("flag: %v", err)
	}
	if flagged.Lead.Status != "flagged" || !flagged.Lead.Flagged {
		t.Fatalf("flag result status=%q flagged=%v", flagged.Lead.Status, flagged.Lead.Flagged)
	}
}
