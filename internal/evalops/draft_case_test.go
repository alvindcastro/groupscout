package evalops

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDraftCasesFromReviewSamplesRequiresTODOFieldsAndReviewRequired(t *testing.T) {
	samples := []ReviewSample{{
		TraceID:        "trace-gq5-001",
		CaseID:         "lead-malformed-review-004",
		Stage:          TraceStageEnrichment,
		Severity:       SeverityCritical,
		Reason:         "unsupported_room_night_estimate",
		SourceSystem:   "richmond_permits",
		SourceType:     "permit_pdf",
		CapturedAt:     time.Date(2026, 5, 7, 13, 30, 0, 0, time.UTC),
		ReviewRequired: true,
		Attributes: map[string]string{
			"case_type":      "lead",
			"category":       "construction_permit",
			"title":          "Airport Logistics Centre Phase 2",
			"source_excerpt": "Safe redacted permit excerpt with CAD 8400000 and 14 month schedule.",
			"issued_at":      "2026-04-20",
			"location":       "Richmond, BC",
			"value_cad":      "8400000",
		},
	}}

	drafts, err := DraftCasesFromReviewSamples(samples, DraftCaseOptions{})
	if err != nil {
		t.Fatalf("DraftCasesFromReviewSamples() error = %v", err)
	}
	if len(drafts) != 1 {
		t.Fatalf("draft count = %d, want 1", len(drafts))
	}

	got := drafts[0]
	if got.ID != "draft-lead-malformed-review-004" {
		t.Fatalf("ID = %q, want draft ID from source case", got.ID)
	}
	if got.TraceID != "trace-gq5-001" || !got.ReviewRequired {
		t.Fatalf("metadata = trace %q review_required %t, want trace preserved and review required", got.TraceID, got.ReviewRequired)
	}
	if got.Expected.ReleaseBlocking {
		t.Fatalf("draft case was marked release-blocking: %+v", got.Expected)
	}
	if got.Expected.Decision != TODOReviewDecision || got.Expected.EvalResult != TODOReviewEvalResult {
		t.Fatalf("expected TODO fields = %+v", got.Expected)
	}
	if len(got.Expected.Evidence) != 1 || got.Expected.Evidence[0].Claim != TODOReviewEvidenceClaim {
		t.Fatalf("expected evidence TODOs = %+v", got.Expected.Evidence)
	}
	if got.Raw.Text != samples[0].Attributes["source_excerpt"] {
		t.Fatalf("raw text = %q, want source excerpt", got.Raw.Text)
	}
	if got.Raw.ValueCAD == nil || *got.Raw.ValueCAD != 8400000 {
		t.Fatalf("value_cad = %v, want parsed 8400000", got.Raw.ValueCAD)
	}
	if got.Raw.Metadata["trace_id"] != "trace-gq5-001" {
		t.Fatalf("raw metadata trace_id = %#v, want trace-gq5-001", got.Raw.Metadata["trace_id"])
	}
	if !strings.Contains(got.Notes, "trace-gq5-001") || !strings.Contains(got.Notes, "unsupported_room_night_estimate") {
		t.Fatalf("notes = %q, want trace and reason", got.Notes)
	}
}

func TestDraftCasesFromReviewSamplesGeneratesUniqueIDsForDuplicates(t *testing.T) {
	samples := []ReviewSample{
		{
			TraceID:        "trace-dupe-a",
			CaseID:         "lead-existing-001",
			Stage:          TraceStageEnrichment,
			Reason:         "first_failure",
			SourceSystem:   "richmond_permits",
			SourceType:     "permit_pdf",
			ReviewRequired: true,
			Attributes: map[string]string{
				"case_type":      "lead",
				"source_excerpt": "safe first excerpt",
			},
		},
		{
			TraceID:        "trace-dupe-b",
			CaseID:         "lead-existing-001",
			Stage:          TraceStageEnrichment,
			Reason:         "second_failure",
			SourceSystem:   "richmond_permits",
			SourceType:     "permit_pdf",
			ReviewRequired: true,
			Attributes: map[string]string{
				"case_type":      "lead",
				"source_excerpt": "safe second excerpt",
			},
		},
	}

	drafts, err := DraftCasesFromReviewSamples(samples, DraftCaseOptions{
		ExistingCaseIDs: map[string]struct{}{
			"draft-lead-existing-001": {},
		},
	})
	if err != nil {
		t.Fatalf("DraftCasesFromReviewSamples() error = %v", err)
	}

	gotIDs := []string{drafts[0].ID, drafts[1].ID}
	wantIDs := []string{"draft-lead-existing-001-2", "draft-lead-existing-001-3"}
	for i := range wantIDs {
		if gotIDs[i] != wantIDs[i] {
			t.Fatalf("draft IDs = %#v, want %#v", gotIDs, wantIDs)
		}
	}
}

func TestDraftCasesFromReviewSamplesRejectsUnreviewedSamples(t *testing.T) {
	_, err := DraftCasesFromReviewSamples([]ReviewSample{{
		TraceID:        "trace-not-reviewed",
		Stage:          TraceStageCollector,
		Reason:         "collector_parse_warning",
		ReviewRequired: false,
		Attributes: map[string]string{
			"source_excerpt": "safe excerpt",
		},
	}}, DraftCaseOptions{})
	if !errors.Is(err, ErrDraftCaseReviewRequired) {
		t.Fatalf("DraftCasesFromReviewSamples() error = %v, want ErrDraftCaseReviewRequired", err)
	}
}

func TestWriteDraftCasesJSONLWritesHumanReviewJSONL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "draft_cases.jsonl")
	drafts, err := DraftCasesFromReviewSamples([]ReviewSample{{
		TraceID:        "trace-jsonl",
		Stage:          TraceStageAlert,
		Severity:       SeverityWarning,
		Reason:         "false_priority_alert",
		SourceSystem:   "yvr_disruption",
		SourceType:     "multi_signal_snapshot",
		ReviewRequired: true,
		Attributes: map[string]string{
			"case_type":      "alert",
			"category":       "airport_disruption",
			"airport_code":   "YVR",
			"observed_at":    "2026-05-07T12:45:00Z",
			"source_excerpt": "Safe redacted disruption snapshot.",
		},
	}}, DraftCaseOptions{})
	if err != nil {
		t.Fatalf("DraftCasesFromReviewSamples() error = %v", err)
	}

	if err := WriteDraftCasesJSONL(path, drafts); err != nil {
		t.Fatalf("WriteDraftCasesJSONL() error = %v", err)
	}

	line := readSingleLine(t, path)
	var got DraftCase
	if err := json.Unmarshal([]byte(line), &got); err != nil {
		t.Fatalf("unmarshal draft case: %v", err)
	}
	if got.CaseType != CaseTypeAlert || got.Raw.AirportCode != "YVR" {
		t.Fatalf("draft alert case = %+v, want alert metadata", got)
	}
	if !got.ReviewRequired || got.Expected.Decision != TODOReviewDecision {
		t.Fatalf("draft review gates = review_required %t expected %+v", got.ReviewRequired, got.Expected)
	}

	if _, err := LoadCases(path); err == nil || !strings.Contains(err.Error(), "unknown decision "+TODOReviewDecision) {
		t.Fatalf("LoadCases(%q) error = %v, want TODO decision rejection before golden promotion", path, err)
	}
}
