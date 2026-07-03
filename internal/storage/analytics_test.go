package storage

import (
	"context"
	"database/sql"
	"math"
	"testing"
	"time"
)

func TestLeadStore_SourceAttribution(t *testing.T) {
	db, dsn := newTestSQLiteDB(t)
	store := NewLeadStoreWithDSN(db, dsn)
	ctx := context.Background()

	now := time.Date(2026, 7, 3, 12, 0, 0, 0, time.UTC)
	seedAnalyticsLead(t, db, dsn, store, "announcements", "BCIB bridge", "new", now.Add(-24*time.Hour), 25)
	seedAnalyticsLead(t, db, dsn, store, "announcements", "TransLink roads", "claimed", now.Add(-48*time.Hour), 10)
	seedAnalyticsLead(t, db, dsn, store, "announcements", "YVR project", "won", now.Add(-72*time.Hour), 15)
	seedAnalyticsLead(t, db, dsn, store, "delta_permits", "Tilbury industrial", "lost", now.Add(-24*time.Hour), 8)
	seedAnalyticsLead(t, db, dsn, store, "bcbid", "Old bid", "won", now.Add(-45*24*time.Hour), 40)

	got, err := store.SourceAttribution(ctx, now.Add(-30*24*time.Hour))
	if err != nil {
		t.Fatalf("SourceAttribution: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2: %+v", len(got), got)
	}
	assertSourceAttribution(t, got[0], SourceAttribution{
		Source:  "announcements",
		Leads:   3,
		Claimed: 1,
		Won:     1,
		HitRate: 66.7,
	})
	assertSourceAttribution(t, got[1], SourceAttribution{
		Source:  "delta_permits",
		Leads:   1,
		Claimed: 0,
		Won:     0,
		HitRate: 0,
	})
}

func TestLeadStore_DemandDensityByWeek(t *testing.T) {
	db, dsn := newTestSQLiteDB(t)
	store := NewLeadStoreWithDSN(db, dsn)
	ctx := context.Background()

	weekOne := time.Date(2026, 7, 6, 9, 0, 0, 0, time.UTC)  // Monday
	weekTwo := time.Date(2026, 7, 13, 9, 0, 0, 0, time.UTC) // Monday
	seedAnalyticsLead(t, db, dsn, store, "announcements", "Bridge A", "new", weekOne, 20)
	seedAnalyticsLead(t, db, dsn, store, "announcements", "Bridge B", "claimed", weekOne.Add(24*time.Hour), 30)
	seedAnalyticsLead(t, db, dsn, store, "delta_permits", "Permit A", "new", weekTwo, 12)
	seedAnalyticsLead(t, db, dsn, store, "eventbrite", "Old event", "new", weekOne.Add(-14*24*time.Hour), 50)

	got, err := store.DemandDensityByWeek(ctx, weekOne.Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("DemandDensityByWeek: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2: %+v", len(got), got)
	}
	assertDemandBucket(t, got[0], DemandBucket{
		WeekStart:         time.Date(2026, 7, 6, 0, 0, 0, 0, time.UTC),
		Source:            "announcements",
		Leads:             2,
		EstimatedCrewSize: 50,
	})
	assertDemandBucket(t, got[1], DemandBucket{
		WeekStart:         time.Date(2026, 7, 13, 0, 0, 0, 0, time.UTC),
		Source:            "delta_permits",
		Leads:             1,
		EstimatedCrewSize: 12,
	})
}

func seedAnalyticsLead(t *testing.T, db *sql.DB, dsn string, store LeadStore, source, title, status string, createdAt time.Time, crew int) {
	t.Helper()
	lead := &Lead{
		Source:            source,
		Title:             title,
		Status:            status,
		PriorityScore:     7,
		EstimatedCrewSize: crew,
	}
	if err := store.Insert(context.Background(), lead); err != nil {
		t.Fatalf("Insert %q: %v", title, err)
	}
	query := Rebind(dsn, `UPDATE leads SET status = ?, created_at = ?, updated_at = ? WHERE id = ?`)
	if _, err := db.Exec(query, status, createdAt, createdAt, lead.ID); err != nil {
		t.Fatalf("update seeded lead %q: %v", title, err)
	}
}

func assertSourceAttribution(t *testing.T, got, want SourceAttribution) {
	t.Helper()
	if got.Source != want.Source || got.Leads != want.Leads || got.Claimed != want.Claimed || got.Won != want.Won {
		t.Fatalf("SourceAttribution = %+v, want %+v", got, want)
	}
	if math.Abs(got.HitRate-want.HitRate) > 0.05 {
		t.Fatalf("HitRate = %.2f, want %.2f", got.HitRate, want.HitRate)
	}
}

func assertDemandBucket(t *testing.T, got, want DemandBucket) {
	t.Helper()
	if !got.WeekStart.Equal(want.WeekStart) || got.Source != want.Source || got.Leads != want.Leads || got.EstimatedCrewSize != want.EstimatedCrewSize {
		t.Fatalf("DemandBucket = %+v, want %+v", got, want)
	}
}
