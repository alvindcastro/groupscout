package evalops

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestEvalTargetHandlesValidRequestWithTraceAndSafeShape(t *testing.T) {
	cases := []Case{testLeadCase("lead-target-001", "keep", SeverityCritical, true)}
	target := NewEvalTarget(cases, EvalTargetOptions{Timeout: time.Second})
	server := httptest.NewServer(target.Handler())
	defer server.Close()

	body := `{"case_id":"lead-target-001","trace_id":"trace-from-promptfoo"}`
	resp, err := http.Post(server.URL+"/eval/run", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST /eval/run error = %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var got EvalTargetResponse
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.TraceID != "trace-from-promptfoo" {
		t.Fatalf("trace_id = %q, want propagated trace", got.TraceID)
	}
	if got.Output == "" || len(got.Scores) == 0 || len(got.Sources) != 1 || len(got.Actions) == 0 {
		t.Fatalf("response missing output/scores/sources/actions: %+v", got)
	}
	encoded, _ := json.Marshal(got)
	for _, forbidden := range []string{"SECRET_TOKEN=", "alex.fixture@example.invalid", "604-555-0199", "raw commercial permit text"} {
		if bytes.Contains(encoded, []byte(forbidden)) {
			t.Fatalf("response leaked %q: %s", forbidden, encoded)
		}
	}
}

func TestEvalTargetRejectsInvalidJSON(t *testing.T) {
	target := NewEvalTarget([]Case{testLeadCase("lead-target-001", "keep", SeverityCritical, true)}, EvalTargetOptions{})
	req := httptest.NewRequest(http.MethodPost, "/eval/run", strings.NewReader(`{"case_id":`))
	rec := httptest.NewRecorder()

	target.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "invalid JSON") {
		t.Fatalf("body = %q, want invalid JSON error", rec.Body.String())
	}
}

func TestEvalTargetRejectsUnknownCaseID(t *testing.T) {
	target := NewEvalTarget([]Case{testLeadCase("lead-target-001", "keep", SeverityCritical, true)}, EvalTargetOptions{})
	req := httptest.NewRequest(http.MethodPost, "/eval/run", strings.NewReader(`{"case_id":"missing-case"}`))
	rec := httptest.NewRecorder()

	target.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "unknown case_id") {
		t.Fatalf("body = %q, want unknown case error", rec.Body.String())
	}
}

func TestEvalTargetTimesOutSlowExecutor(t *testing.T) {
	cases := []Case{testLeadCase("lead-target-001", "keep", SeverityCritical, true)}
	target := NewEvalTarget(cases, EvalTargetOptions{
		Timeout: 10 * time.Millisecond,
		Executor: blockingExecutor{
			done: make(chan struct{}),
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/eval/run", strings.NewReader(`{"case_id":"lead-target-001"}`))
	rec := httptest.NewRecorder()

	target.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusGatewayTimeout {
		t.Fatalf("status = %d, want 504", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "timed out") {
		t.Fatalf("body = %q, want timeout error", rec.Body.String())
	}
}

func TestEvalTargetReturnsScorerFailure(t *testing.T) {
	cases := []Case{testLeadCase("lead-target-001", "keep", SeverityCritical, true)}
	target := NewEvalTarget(cases, EvalTargetOptions{
		Timeout: time.Second,
		Scorer:  errorScorer{err: errors.New("fixture scorer failed")},
	})
	req := httptest.NewRequest(http.MethodPost, "/eval/run", strings.NewReader(`{"case_id":"lead-target-001"}`))
	rec := httptest.NewRecorder()

	target.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "scorer failed") {
		t.Fatalf("body = %q, want scorer error", rec.Body.String())
	}
}

type blockingExecutor struct {
	done chan struct{}
}

func (e blockingExecutor) Execute(ctx context.Context, req EvalTargetRequest, c Case) (TargetOutputs, error) {
	select {
	case <-ctx.Done():
		return TargetOutputs{}, ctx.Err()
	case <-e.done:
		return TargetOutputs{}, nil
	}
}

type errorScorer struct {
	err error
}

func (s errorScorer) Score(context.Context, Case, TargetOutputs) ([]Result, error) {
	return nil, s.err
}

func testLeadCase(id, decision string, severity Severity, releaseBlocking bool) Case {
	value := int64(8400000)
	return Case{
		ID:        id,
		CaseType:  CaseTypeLead,
		Category:  "construction_permit",
		RiskLevel: severity,
		Source: Source{
			System:      "richmond_permits",
			Type:        "permit_pdf",
			FixtureURL:  "https://fixtures.groupscout.local/richmond/" + id + ".pdf",
			CollectedAt: "2026-05-07T00:00:00Z",
		},
		Raw: RawPayload{
			Title:    "Airport Logistics Centre Phase 2",
			Text:     "Building permit summary: new logistics warehouse near airport. Declared value CAD 8400000. Estimated construction schedule 14 months. Site staging calls for structural steel crews.",
			Location: "Richmond, BC",
			ValueCAD: &value,
			Metadata: map[string]any{
				"duration_months": 14,
			},
		},
		Expected: ExpectedOutcome{
			EvalResult:         "pass",
			Decision:           decision,
			ReleaseBlocking:    releaseBlocking,
			SeverityOnMismatch: severity,
			ScoreBand: ScoreBand{
				Min: 8,
				Max: 10,
			},
			Enrichment: &EnrichmentExpectation{
				ProjectType: "industrial construction",
				EstimatedRoomNights: RangeExpectation{
					Min: 1200,
					Max: 4200,
				},
				ProjectDurationMonths: RangeExpectation{
					Min: 10,
					Max: 18,
				},
				LodgingNeed: "high",
				Confidence:  "medium",
			},
			Evidence: []EvidenceRequirement{
				{Claim: "large commercial construction", MustSupportWith: "CAD 8400000 warehouse and 14 month schedule"},
			},
			Privacy: PrivacyExpectation{},
		},
	}
}
