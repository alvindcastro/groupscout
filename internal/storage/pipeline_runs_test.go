package storage

import (
	"context"
	"testing"
	"time"
)

func TestPipelineRunStore_CreateCompleteAndList(t *testing.T) {
	db, dsn := newTestSQLiteDB(t)
	store := NewPipelineRunStoreWithDSN(db, dsn)
	ctx := context.Background()

	first, err := store.Create(ctx, PipelineRun{Sources: []string{"richmond_permits"}, Request: map[string]any{"dry_run": false}})
	if err != nil {
		t.Fatalf("Create first: %v", err)
	}
	time.Sleep(time.Millisecond)
	second, err := store.Create(ctx, PipelineRun{Sources: []string{"eventbrite"}})
	if err != nil {
		t.Fatalf("Create second: %v", err)
	}
	if err := store.Complete(ctx, first.ID, PipelineRunCompletion{
		Status: "succeeded",
		Counts: map[string]int{"new_leads": 2},
		Errors: []string{},
	}); err != nil {
		t.Fatalf("Complete first: %v", err)
	}
	if err := store.Complete(ctx, second.ID, PipelineRunCompletion{
		Status: "failed",
		Counts: map[string]int{"new_leads": 0},
		Errors: []string{"collector failed"},
	}); err != nil {
		t.Fatalf("Complete second: %v", err)
	}

	failed, next, err := store.List(ctx, PipelineRunListFilter{Status: "failed", Limit: 10})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if next != "" {
		t.Fatalf("next cursor = %q, want empty", next)
	}
	if len(failed) != 1 || failed[0].ID != second.ID {
		t.Fatalf("failed runs = %#v, want second run", failed)
	}
	if failed[0].Errors[0] != "collector failed" {
		t.Fatalf("errors = %#v", failed[0].Errors)
	}
}

func TestStatsStore_SummarizesSupportedLeadFields(t *testing.T) {
	db, dsn := newTestSQLiteDB(t)
	leadStore := NewLeadStoreWithDSN(db, dsn)
	outreachStore := NewOutreachStoreWithDSN(db, dsn)
	statsStore := NewStatsStoreWithDSN(db, dsn)
	ctx := context.Background()

	leadA := &Lead{Source: "richmond_permits", Title: "A", Status: "new", PriorityScore: 9, Owner: "alex@example.test"}
	leadB := &Lead{Source: "eventbrite", Title: "B", Status: "won", PriorityScore: 4, Owner: ""}
	if err := leadStore.Insert(ctx, leadA); err != nil {
		t.Fatalf("Insert leadA: %v", err)
	}
	if err := leadStore.Insert(ctx, leadB); err != nil {
		t.Fatalf("Insert leadB: %v", err)
	}
	if _, err := outreachStore.Insert(ctx, OutreachEvent{LeadID: leadB.ID, Channel: "email", Outcome: "won"}); err != nil {
		t.Fatalf("Insert outreach: %v", err)
	}

	got, err := statsStore.Summary(ctx, StatsFilter{Window: "30d"})
	if err != nil {
		t.Fatalf("Summary: %v", err)
	}
	if got.ByStatus["new"] != 1 || got.ByStatus["won"] != 1 {
		t.Fatalf("ByStatus = %#v", got.ByStatus)
	}
	if got.BySource["richmond_permits"] != 1 || got.BySource["eventbrite"] != 1 {
		t.Fatalf("BySource = %#v", got.BySource)
	}
	if got.ScoreBands["high"] != 1 || got.ScoreBands["medium"] != 1 {
		t.Fatalf("ScoreBands = %#v", got.ScoreBands)
	}
	if got.ByOwner["alex@example.test"] != 1 || got.ByOwner["unassigned"] != 1 {
		t.Fatalf("ByOwner = %#v", got.ByOwner)
	}
	if got.ByOutcome["won"] != 1 {
		t.Fatalf("ByOutcome = %#v", got.ByOutcome)
	}
	if len(got.ByWeek) == 0 {
		t.Fatal("ByWeek should include current week")
	}
}
