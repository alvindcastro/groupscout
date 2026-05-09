package evalops

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestReviewSampleWriterWritesRedactedJSONL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "samples.jsonl")
	writer := NewReviewSampleWriter(ReviewSampleWriterConfig{
		Enabled:       true,
		Path:          path,
		MaxFieldBytes: 128,
		Clock:         func() time.Time { return time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC) },
	})
	sample := ReviewSample{
		TraceID:      "trace-gq4-sample",
		CaseID:       "case-gq4-sample",
		Stage:        TraceStageEnrichment,
		Severity:     SeverityWarning,
		Reason:       "unsupported_room_nights",
		SourceSystem: "richmond_permits",
		SourceType:   "permit_pdf",
		Attributes: map[string]string{
			"source_excerpt": "safe permit excerpt with logistics centre and 14 month schedule",
		},
		Results: []Result{{
			CaseID:   "case-gq4-sample",
			Scorer:   "enrichment_completeness",
			Status:   ResultFail,
			Severity: SeverityWarning,
			Message:  "review fixture-token-do-not-use before promotion",
		}},
	}

	if err := writer.Write(sample); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	line := readSingleLine(t, path)
	if strings.Contains(line, "fixture-token-do-not-use") {
		t.Fatalf("sample leaked token: %s", line)
	}
	var got ReviewSample
	if err := json.Unmarshal([]byte(line), &got); err != nil {
		t.Fatalf("unmarshal sample: %v", err)
	}
	if got.TraceID != sample.TraceID || got.CaseID != sample.CaseID || !got.ReviewRequired {
		t.Fatalf("sample metadata = %+v, want trace/case/review_required preserved", got)
	}
	if got.CapturedAt.IsZero() {
		t.Fatalf("sample captured_at was not populated: %+v", got)
	}
}

func TestReviewSampleWriterAppendsMultipleSamples(t *testing.T) {
	path := filepath.Join(t.TempDir(), "samples.jsonl")
	writer := NewReviewSampleWriter(ReviewSampleWriterConfig{Enabled: true, Path: path})

	if err := writer.Write(ReviewSample{TraceID: "trace-one", CaseID: "case-one", Stage: TraceStageCollector, Reason: "collector_failed"}); err != nil {
		t.Fatalf("first Write() error = %v", err)
	}
	if err := writer.Write(ReviewSample{TraceID: "trace-two", CaseID: "case-two", Stage: TraceStageAlert, Reason: "false_priority"}); err != nil {
		t.Fatalf("second Write() error = %v", err)
	}

	lines := readLines(t, path)
	if len(lines) != 2 {
		t.Fatalf("line count = %d, want 2: %#v", len(lines), lines)
	}
	if !strings.Contains(lines[0], "trace-one") || !strings.Contains(lines[1], "trace-two") {
		t.Fatalf("samples were not appended in order: %#v", lines)
	}
}

func TestReviewSampleWriterRejectsUnredactedSensitiveFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "samples.jsonl")
	writer := NewReviewSampleWriter(ReviewSampleWriterConfig{Enabled: true, Path: path})

	err := writer.Write(ReviewSample{
		TraceID: "trace-unsafe",
		CaseID:  "case-unsafe",
		Stage:   TraceStageLLM,
		Attributes: map[string]string{
			"api_key": "API_KEY=sk-live-do-not-use",
		},
	})
	if !errors.Is(err, ErrUnsafeReviewSample) {
		t.Fatalf("Write() error = %v, want ErrUnsafeReviewSample", err)
	}
	if _, statErr := os.Stat(path); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("sample file stat error = %v, want not exists", statErr)
	}
}

func TestReviewSampleWriterLimitsOversizedSamples(t *testing.T) {
	path := filepath.Join(t.TempDir(), "samples.jsonl")
	writer := NewReviewSampleWriter(ReviewSampleWriterConfig{Enabled: true, Path: path, MaxFieldBytes: 80})
	hugeExcerpt := strings.Repeat("permit source text ", 80)

	if err := writer.Write(ReviewSample{
		TraceID: "trace-large",
		CaseID:  "case-large",
		Stage:   TraceStageCollector,
		Reason:  "collector_parse_warning",
		Attributes: map[string]string{
			"source_excerpt": hugeExcerpt,
		},
	}); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	line := readSingleLine(t, path)
	if strings.Contains(line, hugeExcerpt) {
		t.Fatalf("oversized sample was not limited: %s", line)
	}
	if !strings.Contains(line, "[TRUNCATED]") {
		t.Fatalf("oversized sample missing truncated marker: %s", line)
	}
}

func TestReviewSampleWriterCanBeDisabled(t *testing.T) {
	path := filepath.Join(t.TempDir(), "samples.jsonl")
	writer := NewReviewSampleWriter(ReviewSampleWriterConfig{Enabled: false, Path: path})
	if err := writer.Write(ReviewSample{TraceID: "trace-disabled", CaseID: "case-disabled", Stage: TraceStageCollector}); err != nil {
		t.Fatalf("disabled Write() error = %v", err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("disabled writer created file, stat error = %v", err)
	}
}

func readSingleLine(t *testing.T, path string) string {
	t.Helper()
	lines := readLines(t, path)
	if len(lines) != 1 {
		t.Fatalf("line count = %d, want 1: %#v", len(lines), lines)
	}
	return lines[0]
}

func readLines(t *testing.T, path string) []string {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer file.Close()
	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan %s: %v", path, err)
	}
	return lines
}
