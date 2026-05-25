package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alvindcastro/groupscout/internal/storage"
	"github.com/google/uuid"
)

type stubAuditStore struct {
	raw       *storage.RawInput
	err       error
	requested uuid.UUID
}

func (s *stubAuditStore) Store(context.Context, storage.RawInput) (uuid.UUID, error) {
	return uuid.Nil, errors.New("not implemented")
}

func (s *stubAuditStore) GetByID(_ context.Context, id uuid.UUID) (*storage.RawInput, error) {
	s.requested = id
	return s.raw, s.err
}

func (s *stubAuditStore) GetByHash(context.Context, string) (*storage.RawInput, error) {
	return nil, errors.New("not implemented")
}

func (s *stubAuditStore) ExistsByHash(context.Context, string) (bool, error) {
	return false, errors.New("not implemented")
}

func (s *stubAuditStore) PurgeOlderThan(context.Context, time.Time) (int64, error) {
	return 0, errors.New("not implemented")
}

func TestDecodeLeadActionMapsOperatorFields(t *testing.T) {
	snoozedUntil := "2026-05-25T17:30:00Z"
	raw := map[string]json.RawMessage{
		"action":        json.RawMessage(`"snooze"`),
		"owner":         json.RawMessage(`"alvin"`),
		"notes":         json.RawMessage(`"Wait for permit update"`),
		"snoozed_until": json.RawMessage(`"` + snoozedUntil + `"`),
	}

	action, err := decodeLeadAction(raw)
	if err != nil {
		t.Fatalf("decodeLeadAction returned error: %v", err)
	}
	if action.Action != "snooze" || action.Owner != "alvin" {
		t.Fatalf("decoded action = %#v", action)
	}
	if action.Notes == nil || *action.Notes != "Wait for permit update" {
		t.Fatalf("notes = %#v, want mapped notes", action.Notes)
	}
	if action.SnoozedUntil == nil || action.SnoozedUntil.Format(time.RFC3339) != snoozedUntil {
		t.Fatalf("snoozed_until = %#v, want %s", action.SnoozedUntil, snoozedUntil)
	}
}

func TestDecodeLeadActionRejectsInvalidRequiredFields(t *testing.T) {
	for _, tc := range []struct {
		name string
		raw  map[string]json.RawMessage
		want string
	}{
		{name: "blank action", raw: map[string]json.RawMessage{"action": json.RawMessage(`"   "`)}, want: "invalid action"},
		{name: "bad owner", raw: map[string]json.RawMessage{"action": json.RawMessage(`"claim"`), "owner": json.RawMessage(`42`)}, want: "invalid owner"},
		{name: "bad snooze time", raw: map[string]json.RawMessage{"action": json.RawMessage(`"snooze"`), "snoozed_until": json.RawMessage(`"tomorrow"`)}, want: "invalid snoozed_until"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := decodeLeadAction(tc.raw)
			if err == nil || err.Error() != tc.want {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestLeadResponseMappersHideRawInputIDAndPreserveOperatorFields(t *testing.T) {
	snoozedUntil := time.Date(2026, 5, 25, 17, 30, 0, 0, time.UTC)
	lead := storage.Lead{
		ID:                      "lead_123",
		RawInputID:              uuid.NewString(),
		Source:                  "richmond_permits",
		Title:                   "Airport hotel tower",
		Location:                "Richmond, BC",
		ProjectValue:            12000000,
		GeneralContractor:       "PCL",
		Applicant:               "Airport Hotels Ltd.",
		Contractor:              "PCL",
		SourceURL:               "https://example.test/permit",
		ProjectType:             "hotel",
		EstimatedCrewSize:       24,
		EstimatedDurationMonths: 8,
		OutOfTownCrewLikely:     true,
		PriorityScore:           91,
		PriorityReason:          "Large crew signal",
		Rationale:               "Likely room-night demand.",
		SuggestedOutreachTiming: "This week",
		Notes:                   "Call GC",
		Owner:                   "alvin",
		SnoozedUntil:            &snoozedUntil,
		Flagged:                 true,
		VerificationState:       "needs_review",
		Status:                  "snoozed",
		CreatedAt:               snoozedUntil.Add(-time.Hour),
		UpdatedAt:               snoozedUntil,
	}

	summary := leadSummary(lead)
	if summary.ID != lead.ID || !summary.HasRaw || summary.AuditSourceURL != lead.SourceURL {
		t.Fatalf("summary = %#v", summary)
	}
	detail := leadDetail(lead)
	if detail.ID != lead.ID || detail.Owner != "alvin" || detail.SnoozedUntil != &snoozedUntil || !detail.Flagged {
		t.Fatalf("detail = %#v", detail)
	}
	for name, value := range map[string]any{"summary": summary, "detail": detail} {
		encoded, err := json.Marshal(value)
		if err != nil {
			t.Fatalf("marshal %s: %v", name, err)
		}
		if strings.Contains(string(encoded), "raw_input_id") {
			t.Fatalf("%s response exposed raw_input_id: %s", name, encoded)
		}
	}
}

func TestAuditMetadataHandlesMissingInvalidAndStoredRawInputs(t *testing.T) {
	ctx := context.Background()
	missing, err := auditMetadata(ctx, storage.Lead{}, &stubAuditStore{}, "/raw")
	if err != nil {
		t.Fatalf("missing raw metadata error = %v", err)
	}
	if missing.HasRaw {
		t.Fatalf("missing raw metadata = %#v, want has_raw false", missing)
	}

	if _, err := auditMetadata(ctx, storage.Lead{RawInputID: "not-a-uuid"}, &stubAuditStore{}, "/raw"); err == nil {
		t.Fatalf("invalid raw input ID returned nil error")
	}

	rawID := uuid.New()
	collectedAt := time.Date(2026, 5, 25, 18, 0, 0, 0, time.UTC)
	store := &stubAuditStore{raw: &storage.RawInput{
		ID:            rawID,
		PayloadType:   "application/json",
		SourceURL:     "https://example.test/source",
		CollectorName: "richmond_permits",
		CreatedAt:     collectedAt,
	}}
	metadata, err := auditMetadata(ctx, storage.Lead{RawInputID: rawID.String()}, store, "/api/leads/lead_123/raw")
	if err != nil {
		t.Fatalf("stored raw metadata error = %v", err)
	}
	if store.requested != rawID {
		t.Fatalf("requested raw ID = %s, want %s", store.requested, rawID)
	}
	if !metadata.HasRaw || metadata.RawLink != "/api/leads/lead_123/raw" || metadata.PayloadType != "application/json" || metadata.CollectedAt != collectedAt {
		t.Fatalf("metadata = %#v", metadata)
	}

	store.raw = nil
	metadata, err = auditMetadata(ctx, storage.Lead{RawInputID: rawID.String()}, store, "/raw")
	if err != nil {
		t.Fatalf("nil raw metadata error = %v", err)
	}
	if metadata.HasRaw {
		t.Fatalf("nil raw metadata = %#v, want has_raw false", metadata)
	}
}

func TestUIAPILeadResourceRoutingAndListValidation(t *testing.T) {
	fx := newUIAPIFixture(t, "")

	for _, tc := range []struct {
		name   string
		method string
		path   string
		status int
	}{
		{name: "lead list rejects post", method: http.MethodPost, path: "/api/leads", status: http.StatusMethodNotAllowed},
		{name: "empty lead resource", method: http.MethodGet, path: "/api/leads/", status: http.StatusNotFound},
		{name: "raw rejects post", method: http.MethodPost, path: "/api/leads/" + fx.lead.ID + "/raw", status: http.StatusMethodNotAllowed},
		{name: "outreach rejects put", method: http.MethodPut, path: "/api/leads/" + fx.lead.ID + "/outreach", status: http.StatusMethodNotAllowed},
		{name: "unknown subresource", method: http.MethodGet, path: "/api/leads/" + fx.lead.ID + "/unknown", status: http.StatusNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			rec := httptest.NewRecorder()

			fx.handler.ServeHTTP(rec, req)

			if rec.Code != tc.status {
				t.Fatalf("status = %d, want %d; body=%s", rec.Code, tc.status, rec.Body.String())
			}
		})
	}

	for _, path := range []string{
		"/api/leads?min_score=-1",
		"/api/leads?min_score=bad",
		"/api/leads?limit=0",
		"/api/leads?limit=101",
		"/api/leads?limit=bad",
		"/api/leads?cursor=bad",
	} {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			rec := httptest.NewRecorder()

			fx.handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
			}
		})
	}

	req := httptest.NewRequest(http.MethodGet, "/api/leads?limit=1", nil)
	rec := httptest.NewRecorder()
	fx.handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Items      []map[string]any `json:"items"`
		NextCursor string           `json:"next_cursor"`
		Filters    struct {
			Limit float64 `json:"limit"`
		} `json:"filters"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Items) != 1 || body.NextCursor != "1" || body.Filters.Limit != 1 {
		t.Fatalf("limited response = %#v", body)
	}
}
