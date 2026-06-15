package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alvindcastro/groupscout/config"
	"github.com/alvindcastro/groupscout/internal/storage"
)

type fakeWebhookLeadStore struct {
	inserted *storage.Lead
	err      error
}

func (s *fakeWebhookLeadStore) Insert(ctx context.Context, lead *storage.Lead) error {
	if s.err != nil {
		return s.err
	}
	copy := *lead
	s.inserted = &copy
	return nil
}

func (s *fakeWebhookLeadStore) ExistsBySourceTitle(ctx context.Context, source, title string) (bool, error) {
	return false, nil
}

func (s *fakeWebhookLeadStore) ListNew(ctx context.Context) ([]storage.Lead, error) {
	return nil, nil
}

func (s *fakeWebhookLeadStore) ListDeliveryCandidates(ctx context.Context, limit int) ([]storage.Lead, error) {
	return nil, nil
}

func (s *fakeWebhookLeadStore) UpdateStatus(ctx context.Context, id, status string) error {
	return nil
}

func (s *fakeWebhookLeadStore) ListForDigest(ctx context.Context) ([]storage.Lead, error) {
	return nil, nil
}

func (s *fakeWebhookLeadStore) GetByID(ctx context.Context, id string) (*storage.Lead, error) {
	return nil, nil
}

type fakeWebhookNotifier struct {
	sent []storage.Lead
	err  error
}

func (n *fakeWebhookNotifier) Send(ctx context.Context, leads []storage.Lead) error {
	if n.err != nil {
		return n.err
	}
	n.sent = append([]storage.Lead(nil), leads...)
	return nil
}

func TestHandleN8NWebhook_NormalizesPercentPriorityScoreBeforeStoreAndNotify(t *testing.T) {
	store := &fakeWebhookLeadStore{}
	notifier := &fakeWebhookNotifier{}
	handler := handleN8NWebhook(&config.Config{APIToken: "secret"}, store, notifier)

	req := httptest.NewRequest(http.MethodPost, "/n8n/webhook", strings.NewReader(`{"Title":"External lead","PriorityScore":98}`))
	req.Header.Set("Authorization", "Bearer secret")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusCreated, rr.Body.String())
	}
	if store.inserted == nil {
		t.Fatal("lead was not inserted")
	}
	if store.inserted.PriorityScore != 10 {
		t.Fatalf("inserted PriorityScore = %d, want 10", store.inserted.PriorityScore)
	}
	if len(notifier.sent) != 1 {
		t.Fatalf("sent leads = %d, want 1", len(notifier.sent))
	}
	if notifier.sent[0].PriorityScore != 10 {
		t.Fatalf("notified PriorityScore = %d, want 10", notifier.sent[0].PriorityScore)
	}
}

func TestHandleN8NWebhook_RequiresBearerToken(t *testing.T) {
	store := &fakeWebhookLeadStore{}
	handler := handleN8NWebhook(&config.Config{APIToken: "secret"}, store, &fakeWebhookNotifier{})

	req := httptest.NewRequest(http.MethodPost, "/n8n/webhook", strings.NewReader(`{"Title":"External lead","PriorityScore":9}`))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusUnauthorized)
	}
	if store.inserted != nil {
		t.Fatal("lead should not be inserted without auth")
	}
}

func TestHandleN8NWebhook_StoreErrorStopsNotification(t *testing.T) {
	notifier := &fakeWebhookNotifier{}
	handler := handleN8NWebhook(&config.Config{}, &fakeWebhookLeadStore{err: errors.New("store failed")}, notifier)

	req := httptest.NewRequest(http.MethodPost, "/n8n/webhook", strings.NewReader(`{"Title":"External lead","PriorityScore":9}`))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
	}
	if len(notifier.sent) != 0 {
		t.Fatalf("sent leads = %d, want 0", len(notifier.sent))
	}
}

func TestNormalizeExternalPriorityScore(t *testing.T) {
	tests := []struct {
		name  string
		score int
		want  int
	}{
		{name: "negative becomes zero", score: -5, want: 0},
		{name: "zero stays zero", score: 0, want: 0},
		{name: "in range stays unchanged", score: 7, want: 7},
		{name: "ten stays ten", score: 10, want: 10},
		{name: "legacy fifty percent maps to five", score: 50, want: 5},
		{name: "legacy ninety percent maps to nine", score: 90, want: 9},
		{name: "legacy percentage rounds to ten", score: 98, want: 10},
		{name: "oversized score clamps to ten", score: 101, want: 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeExternalPriorityScore(tt.score); got != tt.want {
				t.Fatalf("normalizeExternalPriorityScore(%d) = %d, want %d", tt.score, got, tt.want)
			}
		})
	}
}

func TestNormalizeWebhookLead_DefaultsSourceAndID(t *testing.T) {
	lead := storage.Lead{PriorityScore: 5}

	normalizeWebhookLead(&lead)

	if lead.Source != "n8n" {
		t.Fatalf("Source = %q, want n8n", lead.Source)
	}
	if lead.ID == "" {
		t.Fatal("ID should be generated")
	}
	if lead.PriorityScore != 5 {
		t.Fatalf("PriorityScore = %d, want 5", lead.PriorityScore)
	}
}
