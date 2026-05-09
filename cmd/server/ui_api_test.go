package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

func TestUIAPIAdminLoginExchangesSetupTokenForSession(t *testing.T) {
	fx := newUIAPIFixture(t, "automation-token")
	auth, err := newAdminAuthenticator(adminAuthConfig{
		Enabled:    true,
		SetupToken: "setup-token",
		SessionTTL: time.Hour,
	})
	if err != nil {
		t.Fatalf("newAdminAuthenticator: %v", err)
	}
	fx.handler = newUIAPIHandlerWithDeps(uiAPIConfig{
		DB:        fx.db,
		DSN:       fx.dsn,
		APIToken:  "automation-token",
		AdminAuth: auth,
	})

	statusReq := httptest.NewRequest(http.MethodGet, "/api/auth/status", nil)
	statusRec := httptest.NewRecorder()
	fx.handler.ServeHTTP(statusRec, statusReq)
	if statusRec.Code != http.StatusOK {
		t.Fatalf("status endpoint = %d, want 200; body=%s", statusRec.Code, statusRec.Body.String())
	}
	if !strings.Contains(statusRec.Body.String(), `"auth_required":true`) {
		t.Fatalf("status response does not require auth: %s", statusRec.Body.String())
	}

	blockedReq := httptest.NewRequest(http.MethodGet, "/api/leads", nil)
	blockedRec := httptest.NewRecorder()
	fx.handler.ServeHTTP(blockedRec, blockedReq)
	if blockedRec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated leads status = %d, want 401", blockedRec.Code)
	}

	loginReq := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"token":"setup-token"}`))
	loginRec := httptest.NewRecorder()
	fx.handler.ServeHTTP(loginRec, loginReq)
	if loginRec.Code != http.StatusOK {
		t.Fatalf("login status = %d, want 200; body=%s", loginRec.Code, loginRec.Body.String())
	}
	cookies := loginRec.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != adminSessionCookieName || cookies[0].Value == "" || !cookies[0].HttpOnly {
		t.Fatalf("login cookie = %#v, want HttpOnly %s cookie", cookies, adminSessionCookieName)
	}

	authedReq := httptest.NewRequest(http.MethodGet, "/api/leads", nil)
	authedReq.AddCookie(cookies[0])
	authedRec := httptest.NewRecorder()
	fx.handler.ServeHTTP(authedRec, authedReq)
	if authedRec.Code != http.StatusOK {
		t.Fatalf("authenticated leads status = %d, want 200; body=%s", authedRec.Code, authedRec.Body.String())
	}

	rawReq := httptest.NewRequest(http.MethodGet, "/api/leads/"+fx.lead.ID+"/raw", nil)
	rawReq.AddCookie(cookies[0])
	rawRec := httptest.NewRecorder()
	fx.handler.ServeHTTP(rawRec, rawReq)
	if rawRec.Code != http.StatusOK {
		t.Fatalf("admin raw status = %d, want 200; body=%s", rawRec.Code, rawRec.Body.String())
	}

	var loginBody struct {
		SessionToken      string `json:"session_token"`
		SetupTokenRotated bool   `json:"setup_token_rotated"`
	}
	if err := json.Unmarshal(loginRec.Body.Bytes(), &loginBody); err != nil {
		t.Fatalf("decode login body: %v", err)
	}
	if loginBody.SessionToken == "" {
		t.Fatalf("session_token must be returned for non-browser clients")
	}
	if loginBody.SetupTokenRotated {
		t.Fatalf("env-backed setup token should not report rotation")
	}

	meReq := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	meReq.Header.Set("Authorization", "Bearer "+loginBody.SessionToken)
	meRec := httptest.NewRecorder()
	fx.handler.ServeHTTP(meRec, meReq)
	if meRec.Code != http.StatusOK {
		t.Fatalf("bearer session /me status = %d, want 200; body=%s", meRec.Code, meRec.Body.String())
	}
}

func TestUIAPIAdminLoginRejectsInvalidSetupToken(t *testing.T) {
	fx := newUIAPIFixture(t, "")
	auth, err := newAdminAuthenticator(adminAuthConfig{
		Enabled:    true,
		SetupToken: "setup-token",
		SessionTTL: time.Hour,
	})
	if err != nil {
		t.Fatalf("newAdminAuthenticator: %v", err)
	}
	fx.handler = newUIAPIHandlerWithDeps(uiAPIConfig{DB: fx.db, DSN: fx.dsn, AdminAuth: auth})

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"token":"wrong"}`))
	rec := httptest.NewRecorder()
	fx.handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("invalid login status = %d, want 401; body=%s", rec.Code, rec.Body.String())
	}
}

func TestUIAPIAdminLogoutRevokesSession(t *testing.T) {
	fx := newUIAPIFixture(t, "")
	auth, err := newAdminAuthenticator(adminAuthConfig{
		Enabled:    true,
		SetupToken: "setup-token",
		SessionTTL: time.Hour,
	})
	if err != nil {
		t.Fatalf("newAdminAuthenticator: %v", err)
	}
	fx.handler = newUIAPIHandlerWithDeps(uiAPIConfig{DB: fx.db, DSN: fx.dsn, AdminAuth: auth})

	loginReq := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"token":"setup-token"}`))
	loginRec := httptest.NewRecorder()
	fx.handler.ServeHTTP(loginRec, loginReq)
	if loginRec.Code != http.StatusOK {
		t.Fatalf("login status = %d, want 200; body=%s", loginRec.Code, loginRec.Body.String())
	}
	cookie := loginRec.Result().Cookies()[0]

	logoutReq := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	logoutReq.AddCookie(cookie)
	logoutRec := httptest.NewRecorder()
	fx.handler.ServeHTTP(logoutRec, logoutReq)
	if logoutRec.Code != http.StatusOK {
		t.Fatalf("logout status = %d, want 200; body=%s", logoutRec.Code, logoutRec.Body.String())
	}
	cleared := logoutRec.Result().Cookies()
	if len(cleared) != 1 || cleared[0].Name != adminSessionCookieName || cleared[0].MaxAge != -1 {
		t.Fatalf("logout cookie = %#v, want cleared %s cookie", cleared, adminSessionCookieName)
	}

	meReq := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	meReq.AddCookie(cookie)
	meRec := httptest.NewRecorder()
	fx.handler.ServeHTTP(meRec, meReq)
	if meRec.Code != http.StatusUnauthorized {
		t.Fatalf("revoked /me status = %d, want 401; body=%s", meRec.Code, meRec.Body.String())
	}
}

func TestUIAPIAdminSessionExpires(t *testing.T) {
	fx := newUIAPIFixture(t, "")
	base := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)
	auth, err := newAdminAuthenticator(adminAuthConfig{
		Enabled:    true,
		SetupToken: "setup-token",
		SessionTTL: time.Hour,
	})
	if err != nil {
		t.Fatalf("newAdminAuthenticator: %v", err)
	}
	auth.now = func() time.Time { return base }
	fx.handler = newUIAPIHandlerWithDeps(uiAPIConfig{DB: fx.db, DSN: fx.dsn, AdminAuth: auth})

	loginReq := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"token":"setup-token"}`))
	loginRec := httptest.NewRecorder()
	fx.handler.ServeHTTP(loginRec, loginReq)
	if loginRec.Code != http.StatusOK {
		t.Fatalf("login status = %d, want 200; body=%s", loginRec.Code, loginRec.Body.String())
	}
	cookie := loginRec.Result().Cookies()[0]
	auth.now = func() time.Time { return base.Add(2 * time.Hour) }

	meReq := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	meReq.AddCookie(cookie)
	meRec := httptest.NewRecorder()
	fx.handler.ServeHTTP(meRec, meReq)
	if meRec.Code != http.StatusUnauthorized {
		t.Fatalf("expired /me status = %d, want 401; body=%s", meRec.Code, meRec.Body.String())
	}
}

func TestUIAPIAdminLoginRotatesFileBackedSetupToken(t *testing.T) {
	fx := newUIAPIFixture(t, "")
	tokenPath := filepath.Join(t.TempDir(), "admin-setup-token")
	if err := os.WriteFile(tokenPath, []byte("setup-token\n"), 0o600); err != nil {
		t.Fatalf("write setup token: %v", err)
	}
	auth, err := newAdminAuthenticator(adminAuthConfig{
		Enabled:        true,
		SetupTokenFile: tokenPath,
		SessionTTL:     time.Hour,
	})
	if err != nil {
		t.Fatalf("newAdminAuthenticator: %v", err)
	}
	fx.handler = newUIAPIHandlerWithDeps(uiAPIConfig{DB: fx.db, DSN: fx.dsn, AdminAuth: auth})

	loginReq := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"token":"setup-token"}`))
	loginRec := httptest.NewRecorder()
	fx.handler.ServeHTTP(loginRec, loginReq)
	if loginRec.Code != http.StatusOK {
		t.Fatalf("login status = %d, want 200; body=%s", loginRec.Code, loginRec.Body.String())
	}
	var body struct {
		SetupTokenRotated bool `json:"setup_token_rotated"`
	}
	if err := json.Unmarshal(loginRec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode login body: %v", err)
	}
	if !body.SetupTokenRotated {
		t.Fatalf("setup_token_rotated = false, want true")
	}
	rotated, err := os.ReadFile(tokenPath)
	if err != nil {
		t.Fatalf("read rotated token: %v", err)
	}
	if strings.TrimSpace(string(rotated)) == "setup-token" {
		t.Fatalf("setup token was not rotated")
	}

	reuseReq := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"token":"setup-token"}`))
	reuseRec := httptest.NewRecorder()
	fx.handler.ServeHTTP(reuseRec, reuseReq)
	if reuseRec.Code != http.StatusUnauthorized {
		t.Fatalf("reused setup token status = %d, want 401; body=%s", reuseRec.Code, reuseRec.Body.String())
	}
}

func TestLegacyRawLeadHandlerRequiresAuth(t *testing.T) {
	fx := newUIAPIFixture(t, "automation-token")
	handler := legacyRawLeadHandler(fx.db, fx.dsn, "automation-token", nil)

	missingReq := httptest.NewRequest(http.MethodGet, "/leads/"+fx.lead.ID+"/raw", nil)
	missingRec := httptest.NewRecorder()
	handler.ServeHTTP(missingRec, missingReq)
	if missingRec.Code != http.StatusUnauthorized {
		t.Fatalf("missing auth status = %d, want 401; body=%s", missingRec.Code, missingRec.Body.String())
	}

	authedReq := httptest.NewRequest(http.MethodGet, "/leads/"+fx.lead.ID+"/raw", nil)
	authedReq.Header.Set("Authorization", "Bearer automation-token")
	authedRec := httptest.NewRecorder()
	handler.ServeHTTP(authedRec, authedReq)
	if authedRec.Code != http.StatusOK {
		t.Fatalf("bearer auth status = %d, want 200; body=%s", authedRec.Code, authedRec.Body.String())
	}
}
