package evalops

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func makeTargetHandler(cases []Case) *EvalTargetHandler {
	return NewEvalTargetHandler(cases, 5*time.Second)
}

func postToTarget(t *testing.T, handler http.Handler, body any) *httptest.ResponseRecorder {
	t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request body: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/eval/run", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr
}

func TestEvalTargetHandler_ValidLeadRequest(t *testing.T) {
	cases := []Case{makeLeadCase(
		"target-lead-001", "keep", "critical",
		floatPtr(20_000_000),
		"Industrial warehouse steel concrete civil infrastructure Richmond BC crew required 18 months",
		"Richmond BC V4G 1J5",
		nil, 4, 20,
	)}
	handler := makeTargetHandler(cases)

	rr := postToTarget(t, handler, EvalTargetRequest{
		CaseID:  "target-lead-001",
		TraceID: "test-trace-001",
	})

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp EvalTargetResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.TraceID != "test-trace-001" {
		t.Errorf("expected trace_id 'test-trace-001', got %q", resp.TraceID)
	}
	if resp.Output == "" {
		t.Error("expected non-empty output")
	}
	if _, ok := resp.Scores["lead_quality"]; !ok {
		t.Error("expected lead_quality score in response")
	}
	if _, ok := resp.Scores["lead_score"]; !ok {
		t.Error("expected lead_score in response")
	}
	if len(resp.Sources) == 0 {
		t.Error("expected at least one source in response")
	}
}

func TestEvalTargetHandler_InvalidJSON(t *testing.T) {
	handler := makeTargetHandler(nil)

	req := httptest.NewRequest(http.MethodPost, "/eval/run", strings.NewReader(`{invalid json`))
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid JSON, got %d", rr.Code)
	}
}

func TestEvalTargetHandler_UnknownCaseID(t *testing.T) {
	handler := makeTargetHandler(nil)

	rr := postToTarget(t, handler, EvalTargetRequest{CaseID: "nonexistent-001"})

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404 for unknown case ID, got %d", rr.Code)
	}
}

func TestEvalTargetHandler_MissingCaseID(t *testing.T) {
	handler := makeTargetHandler(nil)

	rr := postToTarget(t, handler, EvalTargetRequest{CaseID: ""})

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing case_id, got %d", rr.Code)
	}
}

func TestEvalTargetHandler_MethodNotAllowed(t *testing.T) {
	handler := makeTargetHandler(nil)

	req := httptest.NewRequest(http.MethodGet, "/eval/run", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405 for GET, got %d", rr.Code)
	}
}

func TestEvalTargetHandler_WrongPath(t *testing.T) {
	handler := makeTargetHandler(nil)

	req := httptest.NewRequest(http.MethodPost, "/health", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404 for wrong path, got %d", rr.Code)
	}
}

func TestEvalTargetHandler_TraceIDPropagated(t *testing.T) {
	cases := []Case{makeLeadCase(
		"target-trace-001", "keep", "warning",
		floatPtr(5_000_000),
		"Commercial warehouse civil Richmond BC",
		"Richmond BC",
		nil, 0, 10,
	)}
	handler := makeTargetHandler(cases)

	rr := postToTarget(t, handler, EvalTargetRequest{
		CaseID:  "target-trace-001",
		TraceID: "my-custom-trace-abc-123",
	})

	var resp EvalTargetResponse
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp.TraceID != "my-custom-trace-abc-123" {
		t.Errorf("expected trace_id 'my-custom-trace-abc-123', got %q", resp.TraceID)
	}
}

func TestEvalTargetHandler_TraceIDGeneratedIfMissing(t *testing.T) {
	cases := []Case{makeLeadCase(
		"target-notrace-001", "keep", "warning",
		floatPtr(5_000_000),
		"Commercial warehouse civil Richmond BC",
		"Richmond BC",
		nil, 0, 10,
	)}
	handler := makeTargetHandler(cases)

	rr := postToTarget(t, handler, EvalTargetRequest{CaseID: "target-notrace-001"})

	var resp EvalTargetResponse
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp.TraceID == "" {
		t.Error("expected trace_id to be generated when not provided")
	}
	if !strings.HasPrefix(resp.TraceID, "eval-") {
		t.Errorf("expected generated trace_id to start with 'eval-', got %q", resp.TraceID)
	}
}

func TestEvalTargetHandler_ScorerFailureInResponse(t *testing.T) {
	// Create a case where the lead will fail (drop expected but score should keep)
	cases := []Case{makeLeadCase(
		"target-fail-001", "drop", "critical",
		floatPtr(25_000_000),
		"Industrial construction warehouse steel civil infrastructure out-of-town crew Richmond BC",
		"Richmond BC V4G 1J5",
		nil, 0, 2, // score will be >> 2 so band miss
	)}
	handler := makeTargetHandler(cases)

	rr := postToTarget(t, handler, EvalTargetRequest{CaseID: "target-fail-001"})

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 even for failing cases, got %d", rr.Code)
	}

	var resp EvalTargetResponse
	json.NewDecoder(rr.Body).Decode(&resp)

	// Lead quality score should be 0 for a critical failure
	if score, ok := resp.Scores["lead_quality"]; ok {
		if score == 1.0 {
			t.Errorf("expected lead_quality < 1.0 for a failing case, got %.1f", score)
		}
	}
}

func TestEvalTargetHandler_AlertCase(t *testing.T) {
	signals := AlertSignals{
		CancelledCount:     38,
		TotalFlights:       140,
		CancellationRate:   0.271,
		AvgSeatsPerFlight:  160,
		ConnectingPaxRatio: 0.58,
		HourOfDay:          19,
		MinutesActive:      75,
		WeatherAlert:       &WeatherSignal{Type: "AtmosphericRiver", Severity: "HIGH"},
		NotamActive:        true,
		SingleRunwayOps:    false,
		FeedAgeSeconds:     120,
	}
	c := makeAlertCase("target-alert-001", "hard_alert", "critical", signals, 60, 400)
	handler := makeTargetHandler([]Case{c})

	rr := postToTarget(t, handler, EvalTargetRequest{
		CaseID:  "target-alert-001",
		TraceID: "trace-alert-001",
	})

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp EvalTargetResponse
	json.NewDecoder(rr.Body).Decode(&resp)

	if _, ok := resp.Scores["alert_quality"]; !ok {
		t.Error("expected alert_quality score in response")
	}
	if _, ok := resp.Scores["sps_score"]; !ok {
		t.Error("expected sps_score in response")
	}
}

func TestEvalTargetHandler_ResponseShapeNoSecrets(t *testing.T) {
	cases := []Case{makeLeadCase(
		"target-secrets-001", "keep", "warning",
		floatPtr(5_000_000),
		"Commercial construction crew required Richmond BC",
		"Richmond BC",
		nil, 0, 10,
	)}
	handler := makeTargetHandler(cases)

	rr := postToTarget(t, handler, EvalTargetRequest{CaseID: "target-secrets-001"})
	responseBody := rr.Body.String()

	secretPatterns := []string{"sk-ant-", "sk-test-", "ANTHROPIC_API_KEY", "hooks.slack.com/services"}
	for _, s := range secretPatterns {
		if strings.Contains(responseBody, s) {
			t.Errorf("response contains secret pattern %q", s)
		}
	}
}

func TestEvalTargetHandler_TimeoutContextCancelled(t *testing.T) {
	// Very short timeout — should handle gracefully
	handler := NewEvalTargetHandler(nil, 1*time.Nanosecond)

	rr := postToTarget(t, handler, EvalTargetRequest{CaseID: "any"})

	// With an extremely short timeout the context is likely already cancelled
	// but we should still get a valid response (timeout or not found)
	if rr.Code != http.StatusGatewayTimeout && rr.Code != http.StatusNotFound {
		t.Errorf("expected 504 (timeout) or 404 (case not found), got %d", rr.Code)
	}
}
