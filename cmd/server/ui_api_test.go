package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alvindcastro/groupscout/internal/storage"
)

type uiAPIFixture struct {
	db      *sql.DB
	dsn     string
	handler http.Handler
	lead    *storage.Lead
}

func newUIAPIFixture(t *testing.T, token string) uiAPIFixture {
	t.Helper()
	db, err := storage.Open(":memory:")
	if err != nil {
		t.Fatalf("Open SQLite: %v", err)
	}
	if err := storage.Migrate(db, ":memory:"); err != nil {
		t.Fatalf("Migrate SQLite: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	ctx := context.Background()
	auditStore := storage.NewAuditStoreWithDSN(db, ":memory:")
	rawID, err := auditStore.Store(ctx, storage.RawInput{
		Hash:          "ui-api-raw",
		PayloadType:   "application/json",
		Payload:       []byte(`{"secret_raw_body":true}`),
		SourceURL:     "https://example.test/source",
		CollectorName: "richmond_permits",
	})
	if err != nil {
		t.Fatalf("Store raw input: %v", err)
	}

	leadStore := storage.NewLeadStoreWithDSN(db, ":memory:")
	lead := &storage.Lead{
		RawInputID:        rawID.String(),
		Source:            "richmond_permits",
		Title:             "Airport hotel tower",
		Location:          "Richmond, BC",
		PriorityScore:     9,
		PriorityReason:    "Large hotel-adjacent construction near YVR",
		Rationale:         "Crew lodging likely.",
		Status:            "new",
		Notes:             "Review this week",
		ProjectValue:      12000000,
		SourceURL:         "https://example.test/permit",
		GeneralContractor: "PCL",
	}
	if err := leadStore.Insert(ctx, lead); err != nil {
		t.Fatalf("Insert lead: %v", err)
	}
	if err := leadStore.Insert(ctx, &storage.Lead{
		Source:        "eventbrite",
		Title:         "Downtown convention",
		Location:      "Vancouver, BC",
		PriorityScore: 4,
		Status:        "new",
	}); err != nil {
		t.Fatalf("Insert second lead: %v", err)
	}

	return uiAPIFixture{
		db:      db,
		dsn:     ":memory:",
		handler: newUIAPIHandler(db, ":memory:", token),
		lead:    lead,
	}
}

func TestUIAPIListLeadsFiltersAndSummarizes(t *testing.T) {
	fx := newUIAPIFixture(t, "")
	req := httptest.NewRequest(http.MethodGet, "/api/leads?status=new&source=richmond_permits&min_score=8&q=airport", nil)
	rec := httptest.NewRecorder()

	fx.handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Items) != 1 {
		t.Fatalf("len(items) = %d, want 1; body=%s", len(body.Items), rec.Body.String())
	}
	item := body.Items[0]
	if item["id"] != fx.lead.ID {
		t.Fatalf("id = %v, want %s", item["id"], fx.lead.ID)
	}
	if item["has_raw"] != true {
		t.Fatalf("has_raw = %v, want true", item["has_raw"])
	}
	if _, ok := item["raw_input_id"]; ok {
		t.Fatal("lead summaries must not expose raw_input_id")
	}
}

func TestUIAPIGetLeadDetailIncludesAuditMetadataWithoutRawPayload(t *testing.T) {
	fx := newUIAPIFixture(t, "")
	req := httptest.NewRequest(http.MethodGet, "/api/leads/"+fx.lead.ID, nil)
	rec := httptest.NewRecorder()

	fx.handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if strings.Contains(body, "secret_raw_body") || strings.Contains(body, "raw_input_id") {
		t.Fatalf("detail response exposed raw payload or raw_input_id: %s", body)
	}
	var decoded struct {
		Lead  map[string]any `json:"lead"`
		Audit map[string]any `json:"audit"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if decoded.Lead["id"] != fx.lead.ID {
		t.Fatalf("lead.id = %v, want %s", decoded.Lead["id"], fx.lead.ID)
	}
	if decoded.Audit["has_raw"] != true {
		t.Fatalf("audit.has_raw = %v, want true", decoded.Audit["has_raw"])
	}
	if decoded.Audit["raw_link"] != "/api/leads/"+fx.lead.ID+"/raw" {
		t.Fatalf("audit.raw_link = %v", decoded.Audit["raw_link"])
	}
}

func TestUIAPIPatchLeadAllowsStatusAndNotes(t *testing.T) {
	fx := newUIAPIFixture(t, "")
	req := httptest.NewRequest(http.MethodPatch, "/api/leads/"+fx.lead.ID, bytes.NewBufferString(`{"status":"contacted","notes":"Called GC"}`))
	rec := httptest.NewRecorder()

	fx.handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Lead          map[string]any `json:"lead"`
		ChangedFields []string       `json:"changed_fields"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Lead["status"] != "contacted" || body.Lead["notes"] != "Called GC" {
		t.Fatalf("lead not updated: %#v", body.Lead)
	}
	if len(body.ChangedFields) != 2 {
		t.Fatalf("changed_fields = %#v, want two fields", body.ChangedFields)
	}
}

func TestUIAPIPatchLeadRejectsUnsafeFields(t *testing.T) {
	fx := newUIAPIFixture(t, "")
	req := httptest.NewRequest(http.MethodPatch, "/api/leads/"+fx.lead.ID, bytes.NewBufferString(`{"title":"overwrite source data"}`))
	rec := httptest.NewRecorder()

	fx.handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
	got, err := storage.NewLeadStoreWithDSN(fx.db, fx.dsn).GetByID(context.Background(), fx.lead.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Title != fx.lead.Title {
		t.Fatalf("title = %q, want unchanged %q", got.Title, fx.lead.Title)
	}
}

func TestUIAPIRawLeadRequiresBearerToken(t *testing.T) {
	fx := newUIAPIFixture(t, "test-token")

	for _, tc := range []struct {
		name   string
		auth   string
		status int
	}{
		{name: "missing auth", status: http.StatusUnauthorized},
		{name: "wrong auth", auth: "Bearer wrong", status: http.StatusUnauthorized},
		{name: "correct auth", auth: "Bearer test-token", status: http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/leads/"+fx.lead.ID+"/raw", nil)
			if tc.auth != "" {
				req.Header.Set("Authorization", tc.auth)
			}
			rec := httptest.NewRecorder()

			fx.handler.ServeHTTP(rec, req)

			if rec.Code != tc.status {
				t.Fatalf("status = %d, want %d; body=%s", rec.Code, tc.status, rec.Body.String())
			}
			if tc.status == http.StatusOK && rec.Body.String() != `{"secret_raw_body":true}` {
				t.Fatalf("body = %q, want raw payload", rec.Body.String())
			}
		})
	}
}
