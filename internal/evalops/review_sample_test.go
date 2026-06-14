package evalops

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func makeSample(traceID string, stage Stage, sourceSystem, failureReason, issueClass string) ReviewSample {
	return ReviewSample{
		TraceID:        traceID,
		Stage:          stage,
		SourceSystem:   sourceSystem,
		SourceType:     "permit_pdf",
		FailureReason:  failureReason,
		IssueClass:     issueClass,
		RawExcerpt:     "Industrial warehouse $15M steel concrete crew required Richmond BC",
		ReviewRequired: true,
		CapturedAt:     "2026-06-10T12:00:00Z",
		Metadata:       map[string]string{"priority_score": "7"},
	}
}

func TestReviewSampleWriter_WritesJSONL(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "samples.jsonl")
	w := NewReviewSampleWriter(path, true, 0)

	s1 := makeSample("trace-001", StageEnrichment, "richmond_permits", "enrichment gap: missing rationale", "enrichment_gap")
	s2 := makeSample("trace-002", StageCollector, "bcbid", "collector drift: feed shape changed", "collector_drift")

	if err := w.Write(s1); err != nil {
		t.Fatalf("write s1: %v", err)
	}
	if err := w.Write(s2); err != nil {
		t.Fatalf("write s2: %v", err)
	}

	samples, err := ReadSamples(path)
	if err != nil {
		t.Fatalf("read samples: %v", err)
	}
	if len(samples) != 2 {
		t.Errorf("expected 2 samples, got %d", len(samples))
	}
	if samples[0].TraceID != "trace-001" {
		t.Errorf("expected trace-001, got %q", samples[0].TraceID)
	}
}

func TestReviewSampleWriter_AppendsMultipleSamples(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "append_test.jsonl")
	w := NewReviewSampleWriter(path, true, 0)

	for i := 0; i < 5; i++ {
		traceID := fmt.Sprintf("trace-append-%c", 'a'+i)
		s := makeSample(
			traceID,
			StageEnrichment, "richmond_permits",
			"test failure", "enrichment_gap",
		)
		if err := w.Write(s); err != nil {
			t.Fatalf("write sample %d: %v", i, err)
		}
	}

	samples, err := ReadSamples(path)
	if err != nil {
		t.Fatalf("read samples: %v", err)
	}
	if len(samples) != 5 {
		t.Errorf("expected 5 samples, got %d", len(samples))
	}
}

func TestReviewSampleWriter_RejectsUnredactedEmail(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "reject_email.jsonl")
	w := NewReviewSampleWriter(path, true, 0)

	s := makeSample("trace-pii-001", StageEnrichment, "richmond_permits",
		"failure reason with john.doe@example.com personal contact", "privacy_leak")

	// The writer should redact the email before writing
	// and pass (since redaction is applied), OR reject if redaction wasn't applied.
	// Our implementation auto-redacts, so we expect success.
	if err := w.Write(s); err != nil {
		t.Fatalf("expected writer to auto-redact and succeed, got: %v", err)
	}

	// Verify the written sample does not contain the email
	samples, _ := ReadSamples(path)
	if len(samples) > 0 && strings.Contains(samples[0].FailureReason, "john.doe@example.com") {
		t.Error("expected email to be redacted in written sample")
	}
}

func TestReviewSampleWriter_RejectsUnredactedAPIKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "reject_key.jsonl")
	w := NewReviewSampleWriter(path, true, 0)

	s := makeSample("trace-key-001", StageLLM, "richmond_permits",
		"LLM call failed: api_key sk-ant-fake-key-12345678901234 rejected", "provider_regression")

	if err := w.Write(s); err != nil {
		t.Fatalf("expected auto-redact and succeed, got: %v", err)
	}

	samples, _ := ReadSamples(path)
	if len(samples) > 0 && strings.Contains(samples[0].FailureReason, "sk-ant-fake-key") {
		t.Error("expected API key to be redacted in written sample")
	}
}

func TestReviewSampleWriter_MissingTraceIDRejected(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "reject_no_trace.jsonl")
	w := NewReviewSampleWriter(path, true, 0)

	s := makeSample("", StageEnrichment, "richmond_permits", "some failure", "enrichment_gap")

	if err := w.Write(s); err == nil {
		t.Error("expected error for missing trace_id, got nil")
	}
}

func TestReviewSampleWriter_ReviewRequiredFalseRejected(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "reject_no_review.jsonl")
	w := NewReviewSampleWriter(path, true, 0)

	s := makeSample("trace-no-review-001", StageEnrichment, "richmond_permits", "some failure", "enrichment_gap")
	s.ReviewRequired = false

	if err := w.Write(s); err == nil {
		t.Error("expected error for review_required=false, got nil")
	}
}

func TestReviewSampleWriter_Disabled(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "disabled.jsonl")
	w := NewReviewSampleWriter(path, false, 0)

	s := makeSample("trace-disabled-001", StageEnrichment, "richmond_permits", "some failure", "enrichment_gap")
	if err := w.Write(s); err != nil {
		t.Fatalf("expected no error for disabled writer, got: %v", err)
	}

	// File should not exist since writer is disabled
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("expected file to not exist for disabled writer")
	}
}

func TestReviewSampleWriter_MaxSizeExceeded(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "maxsize.jsonl")

	// Write a large file first
	if err := os.WriteFile(path, make([]byte, 100), 0644); err != nil {
		t.Fatalf("pre-write: %v", err)
	}

	w := NewReviewSampleWriter(path, true, 50) // max 50 bytes, file is 100
	s := makeSample("trace-max-001", StageEnrichment, "richmond_permits", "test failure", "enrichment_gap")

	if err := w.Write(s); err == nil {
		t.Error("expected error for max size exceeded, got nil")
	}
}

func TestDraftCasesFromReviewSamples_RequiredTODOFields(t *testing.T) {
	samples := []ReviewSample{
		makeSample("trace-draft-001", StageEnrichment, "richmond_permits", "missing rationale", "enrichment_gap"),
	}

	drafts, err := DraftCasesFromReviewSamples(samples)
	if err != nil {
		t.Fatalf("generate drafts: %v", err)
	}
	if len(drafts) != 1 {
		t.Fatalf("expected 1 draft, got %d", len(drafts))
	}

	d := drafts[0]
	if !strings.HasPrefix(d.Expected.Decision, "TODO_REVIEW_") {
		t.Errorf("expected TODO_REVIEW_ prefix in decision, got %q", d.Expected.Decision)
	}
	if !strings.HasPrefix(d.Expected.EvalResult, "TODO_REVIEW_") {
		t.Errorf("expected TODO_REVIEW_ prefix in eval_result, got %q", d.Expected.EvalResult)
	}
	if !d.ReviewRequired {
		t.Error("expected review_required=true in draft")
	}
	if d.Expected.ReleaseBlocking {
		t.Error("expected release_blocking=false in draft")
	}
}

func TestDraftCasesFromReviewSamples_PreservesTraceID(t *testing.T) {
	samples := []ReviewSample{
		makeSample("trace-preserve-001", StageEnrichment, "richmond_permits", "failure", "enrichment_gap"),
	}

	drafts, err := DraftCasesFromReviewSamples(samples)
	if err != nil {
		t.Fatalf("generate drafts: %v", err)
	}

	if drafts[0].TraceID != "trace-preserve-001" {
		t.Errorf("expected trace_id preserved, got %q", drafts[0].TraceID)
	}
	// TraceID should also be in raw metadata
	if md, ok := drafts[0].Raw["metadata"].(map[string]any); ok {
		if tid, ok := md["trace_id"]; !ok || tid != "trace-preserve-001" {
			t.Errorf("expected trace_id in raw.metadata, got %v", md)
		}
	}
}

func TestDraftCasesFromReviewSamples_DuplicateHandling(t *testing.T) {
	samples := []ReviewSample{
		makeSample("trace-dup-001", StageEnrichment, "richmond_permits", "failure 1", "enrichment_gap"),
		makeSample("trace-dup-001", StageEnrichment, "richmond_permits", "failure 2", "enrichment_gap"),
	}

	drafts, err := DraftCasesFromReviewSamples(samples)
	if err != nil {
		t.Fatalf("generate drafts: %v", err)
	}
	if len(drafts) != 2 {
		t.Fatalf("expected 2 drafts, got %d", len(drafts))
	}

	// IDs must be different
	if drafts[0].ID == drafts[1].ID {
		t.Errorf("expected different IDs for duplicate trace, got both %q", drafts[0].ID)
	}
}

func TestDraftCasesFromReviewSamples_EmptySamples(t *testing.T) {
	drafts, err := DraftCasesFromReviewSamples(nil)
	if err != nil {
		t.Fatalf("expected no error for empty samples, got: %v", err)
	}
	if len(drafts) != 0 {
		t.Errorf("expected 0 drafts for empty samples, got %d", len(drafts))
	}
}

func TestDraftCasesFromReviewSamples_MissingTraceIDError(t *testing.T) {
	samples := []ReviewSample{
		makeSample("", StageEnrichment, "richmond_permits", "failure", "enrichment_gap"),
	}

	_, err := DraftCasesFromReviewSamples(samples)
	if err == nil {
		t.Error("expected error for sample with missing trace_id, got nil")
	}
}

func TestDraftCasesFromReviewSamples_AlertCaseType(t *testing.T) {
	samples := []ReviewSample{
		makeSample("trace-alert-draft-001", StageAlert, "yvr_disruption", "false priority alert", "alert_false_positive"),
	}

	drafts, err := DraftCasesFromReviewSamples(samples)
	if err != nil {
		t.Fatalf("generate drafts: %v", err)
	}
	if drafts[0].CaseType != "alert" {
		t.Errorf("expected case_type alert for alert stage, got %q", drafts[0].CaseType)
	}
}

func TestWriteDraftCasesJSONL_WritesAndReads(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "draft_cases.jsonl")

	drafts := []DraftCase{
		{
			ID:             "draft-test-001",
			CaseType:       "lead",
			Category:       "enrichment_gap",
			RiskLevel:      "warning",
			TraceID:        "trace-write-001",
			ReviewRequired: true,
			Source: CaseSource{
				System:      "richmond_permits",
				Type:        "permit_pdf",
				FixtureURL:  "https://fixtures.groupscout.local/draft/trace-write-001",
				CollectedAt: "2026-06-10T12:00:00Z",
			},
			Raw: map[string]any{"title": "TODO_REVIEW_title", "text": "excerpt"},
			Expected: DraftExpected{
				EvalResult:         "TODO_REVIEW_eval_result",
				Decision:           "TODO_REVIEW_decision",
				ReleaseBlocking:    false,
				SeverityOnMismatch: "warning",
			},
			Notes: "test draft",
		},
	}

	if err := WriteDraftCasesJSONL(path, drafts); err != nil {
		t.Fatalf("write drafts: %v", err)
	}

	// Read back and verify
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 1 {
		t.Errorf("expected 1 line in output, got %d", len(lines))
	}

	var decoded DraftCase
	if err := json.Unmarshal([]byte(lines[0]), &decoded); err != nil {
		t.Fatalf("decode draft: %v", err)
	}
	if decoded.ID != "draft-test-001" {
		t.Errorf("expected ID draft-test-001, got %q", decoded.ID)
	}
	if !decoded.ReviewRequired {
		t.Error("expected review_required=true in written draft")
	}
}

func TestDraftCasesFromReviewSamples_NeverAutoMarkReleaseBlocking(t *testing.T) {
	samples := []ReviewSample{
		makeSample("trace-noblock-001", StageEnrichment, "richmond_permits", "critical enrichment gap", "enrichment_gap"),
		makeSample("trace-noblock-002", StageAlert, "yvr_disruption", "false positive alert", "alert_false_positive"),
	}

	drafts, err := DraftCasesFromReviewSamples(samples)
	if err != nil {
		t.Fatalf("generate drafts: %v", err)
	}
	for _, d := range drafts {
		if d.Expected.ReleaseBlocking {
			t.Errorf("draft %q must not be release_blocking (human review required)", d.ID)
		}
	}
}
