package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alvindcastro/groupscout/config"
	"github.com/alvindcastro/groupscout/internal/llm"
	"github.com/alvindcastro/groupscout/internal/ollama"
)

type fakeAIStatus []llm.ProviderStatus

func (f fakeAIStatus) Status() []llm.ProviderStatus { return f }

func TestAIStatus(t *testing.T) {
	parked := llm.ProviderStatus{Provider: "claude", Available: false, ParkedUntil: time.Now().Add(time.Hour)}
	up := llm.ProviderStatus{Provider: "openai", Available: true}
	tests := []struct {
		providers   []llm.ProviderStatus
		summary     string
		description string
	}{
		{nil, "unconfigured", ""},
		{[]llm.ProviderStatus{up}, "ok", "openai:ok"},
		{[]llm.ProviderStatus{parked, up}, "degraded", "claude:parked,openai:ok"},
		{[]llm.ProviderStatus{parked}, "down", "claude:parked"},
	}
	for _, tt := range tests {
		summary, detail := aiStatus(tt.providers)
		if summary != tt.summary || detail != tt.description {
			t.Errorf("aiStatus(%v) = %q, %q; want %q, %q", tt.providers, summary, detail, tt.summary, tt.description)
		}
	}
}

// An empty AI account must show in /health without failing the check: a
// container restart cannot fix it.
func TestHealthHandler_ReportsAIProvidersButStaysHealthy(t *testing.T) {
	l := slog.New(slog.NewTextHandler(io.Discard, nil))
	ai := fakeAIStatus{{Provider: "claude", Available: false}, {Provider: "openai", Available: false}}
	code, body := serveHealth(t, healthHandler(fakePinger{}, false, &ollama.NoopClient{}, ai, l))

	if code != http.StatusOK || body["status"] != "ok" {
		t.Fatalf("expected healthy 200, got %d %v", code, body)
	}
	if body["ai"] != "down" || body["ai_providers"] != "claude:parked,openai:parked" {
		t.Errorf("ai = %q, ai_providers = %q", body["ai"], body["ai_providers"])
	}
}

// llm-check end to end: config -> provider factory -> enricher -> an
// OpenAI-format server, with one provider skipped for a missing key.
func TestRunLLMCheck(t *testing.T) {
	var auth string
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		auth = r.Header.Get("Authorization")
		reply := `{"priority_score":8,"project_type":"industrial","estimated_crew_size":90,"out_of_town_crew_likely":true}`
		json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": map[string]any{"content": reply}, "finish_reason": "stop"}},
		})
	}))
	defer srv.Close()

	cfg := &config.Config{
		LLMProviders:  []string{"claude", "openai"},
		OpenAIAPIKey:  "sk-test",
		OpenAIBaseURL: srv.URL,
		LLMTimeoutS:   5,
	}
	var out bytes.Buffer
	if code := runLLMCheck(cfg, nil, &out); code != 0 {
		t.Fatalf("exit code %d:\n%s", code, out.String())
	}
	got := out.String()
	if !strings.Contains(got, "SKIP  claude: CLAUDE_API_KEY is not set") {
		t.Errorf("missing skip line:\n%s", got)
	}
	if strings.Count(got, "ok    openai") != len(llmCheckSamples()) || calls != len(llmCheckSamples()) {
		t.Errorf("want one ok line and call per sample (calls %d):\n%s", calls, got)
	}
	if auth != "Bearer sk-test" {
		t.Errorf("Authorization = %q", auth)
	}
}

func TestRunLLMCheck_accountErrorStopsProvider(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusUnauthorized)
		io.WriteString(w, `{"error":{"message":"Incorrect API key provided","type":"invalid_request_error","code":"invalid_api_key"}}`)
	}))
	defer srv.Close()

	cfg := &config.Config{OpenAIAPIKey: "bad", OpenAIBaseURL: srv.URL, LLMTimeoutS: 5}
	var out bytes.Buffer
	if code := runLLMCheck(cfg, []string{"openai"}, &out); code != 1 {
		t.Fatalf("exit code %d, want 1:\n%s", code, out.String())
	}
	if calls != 1 || !strings.Contains(out.String(), "account_error (HTTP 401)") || !strings.Contains(out.String(), "skipping its remaining samples") {
		t.Errorf("calls %d:\n%s", calls, out.String())
	}
}
