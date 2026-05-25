package auditretention

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"
)

type fakeStore struct {
	cutoff  time.Time
	deleted int64
	err     error
	calls   int
}

func (s *fakeStore) PurgeOlderThan(_ context.Context, olderThan time.Time) (int64, error) {
	s.calls++
	s.cutoff = olderThan
	return s.deleted, s.err
}

func TestWorkerPurgeOnceUsesRetentionCutoff(t *testing.T) {
	now := time.Date(2026, 5, 25, 12, 30, 0, 0, time.UTC)
	store := &fakeStore{deleted: 3}
	worker, err := New(store, Policy{
		RetentionDays: 30,
		Interval:      time.Hour,
		Now:           func() time.Time { return now },
	}, slog.Default())
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	result, err := worker.PurgeOnce(context.Background())
	if err != nil {
		t.Fatalf("PurgeOnce: %v", err)
	}

	wantCutoff := now.AddDate(0, 0, -30)
	if !store.cutoff.Equal(wantCutoff) {
		t.Fatalf("cutoff = %s, want %s", store.cutoff, wantCutoff)
	}
	if result.Deleted != 3 {
		t.Fatalf("Deleted = %d, want 3", result.Deleted)
	}
	if !result.Cutoff.Equal(wantCutoff) {
		t.Fatalf("result cutoff = %s, want %s", result.Cutoff, wantCutoff)
	}
}

func TestWorkerPurgeOnceReturnsStoreErrorWithResult(t *testing.T) {
	now := time.Date(2026, 5, 25, 12, 30, 0, 0, time.UTC)
	store := &fakeStore{err: errors.New("database unavailable")}
	worker, err := New(store, Policy{
		RetentionDays: 7,
		Interval:      time.Hour,
		Now:           func() time.Time { return now },
	}, slog.Default())
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	result, err := worker.PurgeOnce(context.Background())
	if err == nil {
		t.Fatal("PurgeOnce error = nil, want error")
	}

	wantCutoff := now.AddDate(0, 0, -7)
	if !result.Cutoff.Equal(wantCutoff) {
		t.Fatalf("result cutoff = %s, want %s", result.Cutoff, wantCutoff)
	}
}

func TestNewValidatesPolicy(t *testing.T) {
	store := &fakeStore{}
	tests := []struct {
		name   string
		store  Store
		policy Policy
	}{
		{name: "missing store", store: nil, policy: Policy{RetentionDays: 1, Interval: time.Hour}},
		{name: "missing days", store: store, policy: Policy{Interval: time.Hour}},
		{name: "missing interval", store: store, policy: Policy{RetentionDays: 1}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := New(tt.store, tt.policy, slog.Default()); err == nil {
				t.Fatal("New error = nil, want error")
			}
		})
	}
}
