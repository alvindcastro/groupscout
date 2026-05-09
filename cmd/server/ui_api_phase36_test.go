package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alvindcastro/groupscout/internal/storage"
)

func TestUIAPIOutreachListAndCreate(t *testing.T) {
	fx := newUIAPIFixture(t, "")

	postReq := httptest.NewRequest(http.MethodPost, "/api/leads/"+fx.lead.ID+"/outreach", bytes.NewBufferString(`{
		"contact": "gc@example.test",
		"channel": "email",
		"notes": "Sent intro",
		"outcome": "sent"
	}`))
	postRec := httptest.NewRecorder()
	fx.handler.ServeHTTP(postRec, postReq)
	if postRec.Code != http.StatusCreated {
		t.Fatalf("POST status = %d, want 201; body=%s", postRec.Code, postRec.Body.String())
	}
	var postBody struct {
		Outreach map[string]any `json:"outreach"`
		Lead     map[string]any `json:"lead"`
	}
	if err := json.Unmarshal(postRec.Body.Bytes(), &postBody); err != nil {
		t.Fatalf("decode POST response: %v", err)
	}
	if postBody.Outreach["lead_id"] != fx.lead.ID {
		t.Fatalf("outreach.lead_id = %v, want %s", postBody.Outreach["lead_id"], fx.lead.ID)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/leads/"+fx.lead.ID+"/outreach", nil)
	getRec := httptest.NewRecorder()
	fx.handler.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("GET status = %d, want 200; body=%s", getRec.Code, getRec.Body.String())
	}
	var getBody struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(getRec.Body.Bytes(), &getBody); err != nil {
		t.Fatalf("decode GET response: %v", err)
	}
	if len(getBody.Items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(getBody.Items))
	}
	if getBody.Items[0]["channel"] != "email" {
		t.Fatalf("channel = %v, want email", getBody.Items[0]["channel"])
	}
}

func TestUIAPIOutreachRejectsMissingLead(t *testing.T) {
	fx := newUIAPIFixture(t, "")
	req := httptest.NewRequest(http.MethodPost, "/api/leads/missing/outreach", bytes.NewBufferString(`{"contact":"gc@example.test","channel":"email"}`))
	rec := httptest.NewRecorder()

	fx.handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body=%s", rec.Code, rec.Body.String())
	}
}

func TestUIAPILeadActions(t *testing.T) {
	fx := newUIAPIFixture(t, "")
	snoozedUntil := time.Now().UTC().Add(24 * time.Hour).Format(time.RFC3339)

	for _, tc := range []struct {
		name       string
		body       string
		wantStatus string
	}{
		{name: "claim", body: `{"action":"claim","owner":"alex@example.test"}`, wantStatus: "claimed"},
		{name: "contacted", body: `{"action":"contacted"}`, wantStatus: "contacted"},
		{name: "no response", body: `{"action":"no-response"}`, wantStatus: "no_response"},
		{name: "reopen", body: `{"action":"reopen"}`, wantStatus: "new"},
		{name: "snooze", body: `{"action":"snooze","snoozed_until":"` + snoozedUntil + `"}`, wantStatus: "snoozed"},
		{name: "flag", body: `{"action":"flag"}`, wantStatus: "flagged"},
		{name: "dismiss", body: `{"action":"dismiss"}`, wantStatus: "dismissed"},
		{name: "reopen again", body: `{"action":"reopen"}`, wantStatus: "new"},
		{name: "lost", body: `{"action":"lost"}`, wantStatus: "lost"},
		{name: "reopen lost", body: `{"action":"reopen"}`, wantStatus: "new"},
		{name: "won", body: `{"action":"won"}`, wantStatus: "won"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPatch, "/api/leads/"+fx.lead.ID, bytes.NewBufferString(tc.body))
			rec := httptest.NewRecorder()
			fx.handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
			}
			var body struct {
				Lead map[string]any `json:"lead"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if body.Lead["status"] != tc.wantStatus {
				t.Fatalf("lead.status = %v, want %s", body.Lead["status"], tc.wantStatus)
			}
		})
	}

	got, err := storage.NewLeadStoreWithDSN(fx.db, fx.dsn).GetByID(context.Background(), fx.lead.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.VerificationState != "unverified" {
		t.Fatalf("verification_state = %q, want unchanged unverified", got.VerificationState)
	}
}

func TestUIAPILeadActionRejectsInvalidTransition(t *testing.T) {
	fx := newUIAPIFixture(t, "")
	req := httptest.NewRequest(http.MethodPatch, "/api/leads/"+fx.lead.ID, bytes.NewBufferString(`{"action":"won"}`))
	rec := httptest.NewRecorder()
	fx.handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("setup action failed: status=%d body=%s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPatch, "/api/leads/"+fx.lead.ID, bytes.NewBufferString(`{"action":"contacted"}`))
	rec = httptest.NewRecorder()
	fx.handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409; body=%s", rec.Code, rec.Body.String())
	}
}
