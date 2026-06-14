package evalops

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeJSONLFile(t *testing.T, dir, name string, lines []string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	content := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write fixture %s: %v", path, err)
	}
	return path
}

func validCaseLine(id, caseType, decision string) string {
	c := map[string]any{
		"id":         id,
		"case_type":  caseType,
		"category":   "construction_permit",
		"risk_level": "warning",
		"source": map[string]any{
			"system":       "richmond_permits",
			"type":         "permit_pdf",
			"fixture_url":  "https://fixtures.groupscout.local/test.pdf",
			"collected_at": "2026-06-01T08:00:00Z",
		},
		"raw": map[string]any{
			"title":     "Test Project",
			"text":      "Test raw text for scoring.",
			"issued_at": "2026-06-01",
			"location":  "Richmond BC",
			"value_cad": 5000000,
			"metadata":  map[string]any{},
		},
		"expected": map[string]any{
			"eval_result":          "pass",
			"decision":             decision,
			"release_blocking":     false,
			"severity_on_mismatch": "warning",
			"score_band":           map[string]any{"min": 0, "max": 10},
			"enrichment":           nil,
			"alert":                nil,
			"evidence":             []string{"test evidence"},
			"forbidden_claims":     []string{},
			"privacy":              map[string]any{"redact_patterns": []string{}, "forbidden_output_patterns": []string{}},
			"critical_failure_if":  []string{},
		},
	}
	b, _ := json.Marshal(c)
	return string(b)
}

func TestLoadCases_ValidSingleFile(t *testing.T) {
	dir := t.TempDir()
	path := writeJSONLFile(t, dir, "test.jsonl", []string{
		validCaseLine("lead-001", "lead", "keep"),
		validCaseLine("lead-002", "lead", "drop"),
	})

	cases, err := LoadCases([]string{path})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cases) != 2 {
		t.Errorf("expected 2 cases, got %d", len(cases))
	}
	if cases[0].ID != "lead-001" {
		t.Errorf("expected first case ID lead-001, got %q", cases[0].ID)
	}
}

func TestLoadCases_MalformedJSON(t *testing.T) {
	dir := t.TempDir()
	path := writeJSONLFile(t, dir, "bad.jsonl", []string{
		`{"id":"ok","case_type":"lead"` + validCaseLine("ok2", "lead", "keep"),
		`{not valid json`,
	})

	_, err := LoadCases([]string{path})
	if err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
	if !strings.Contains(err.Error(), "malformed JSON") {
		t.Errorf("expected 'malformed JSON' in error, got: %v", err)
	}
}

func TestLoadCases_MissingID(t *testing.T) {
	dir := t.TempDir()
	line := validCaseLine("", "lead", "keep")
	path := writeJSONLFile(t, dir, "missing_id.jsonl", []string{line})

	_, err := LoadCases([]string{path})
	if err == nil {
		t.Fatal("expected error for missing id, got nil")
	}
	if !strings.Contains(err.Error(), "missing required field: id") {
		t.Errorf("expected 'missing required field: id' in error, got: %v", err)
	}
}

func TestLoadCases_DuplicateIDs(t *testing.T) {
	dir := t.TempDir()
	path := writeJSONLFile(t, dir, "dup.jsonl", []string{
		validCaseLine("lead-dup", "lead", "keep"),
		validCaseLine("lead-dup", "lead", "drop"),
	})

	_, err := LoadCases([]string{path})
	if err == nil {
		t.Fatal("expected error for duplicate ID, got nil")
	}
	if !strings.Contains(err.Error(), "duplicate id") {
		t.Errorf("expected 'duplicate id' in error, got: %v", err)
	}
}

func TestLoadCases_DuplicateIDsAcrossFiles(t *testing.T) {
	dir := t.TempDir()
	p1 := writeJSONLFile(t, dir, "a.jsonl", []string{validCaseLine("shared-001", "lead", "keep")})
	p2 := writeJSONLFile(t, dir, "b.jsonl", []string{validCaseLine("shared-001", "lead", "drop")})

	_, err := LoadCases([]string{p1, p2})
	if err == nil {
		t.Fatal("expected error for duplicate ID across files, got nil")
	}
	if !strings.Contains(err.Error(), "duplicate id") {
		t.Errorf("expected 'duplicate id' in error, got: %v", err)
	}
}

func TestLoadCases_UnknownSourceType(t *testing.T) {
	dir := t.TempDir()
	c := map[string]any{
		"id":         "bad-src",
		"case_type":  "lead",
		"category":   "construction_permit",
		"risk_level": "warning",
		"source": map[string]any{
			"system":       "unknown_source_xyz",
			"type":         "permit_pdf",
			"fixture_url":  "https://fixtures.groupscout.local/test.pdf",
			"collected_at": "2026-06-01T08:00:00Z",
		},
		"raw": map[string]any{"title": "t", "text": "t", "issued_at": "2026-06-01", "location": "l", "value_cad": nil, "metadata": map[string]any{}},
		"expected": map[string]any{
			"eval_result": "pass", "decision": "keep", "release_blocking": false,
			"severity_on_mismatch": "warning", "score_band": map[string]any{"min": 0, "max": 10},
			"enrichment": nil, "alert": nil, "evidence": []string{}, "forbidden_claims": []string{},
			"privacy": map[string]any{"redact_patterns": []string{}, "forbidden_output_patterns": []string{}},
			"critical_failure_if": []string{},
		},
	}
	b, _ := json.Marshal(c)
	path := writeJSONLFile(t, dir, "bad_src.jsonl", []string{string(b)})

	_, err := LoadCases([]string{path})
	if err == nil {
		t.Fatal("expected error for unknown source system, got nil")
	}
	if !strings.Contains(err.Error(), "unknown source system") {
		t.Errorf("expected 'unknown source system' in error, got: %v", err)
	}
}

func TestLoadCases_UnsupportedRiskLevel(t *testing.T) {
	dir := t.TempDir()
	c := map[string]any{
		"id":         "bad-risk",
		"case_type":  "lead",
		"category":   "construction_permit",
		"risk_level": "extreme",
		"source": map[string]any{
			"system":       "richmond_permits",
			"type":         "permit_pdf",
			"fixture_url":  "https://fixtures.groupscout.local/test.pdf",
			"collected_at": "2026-06-01T08:00:00Z",
		},
		"raw": map[string]any{"title": "t", "text": "t", "issued_at": "2026-06-01", "location": "l", "value_cad": nil, "metadata": map[string]any{}},
		"expected": map[string]any{
			"eval_result": "pass", "decision": "keep", "release_blocking": false,
			"severity_on_mismatch": "warning", "score_band": map[string]any{"min": 0, "max": 10},
			"enrichment": nil, "alert": nil, "evidence": []string{}, "forbidden_claims": []string{},
			"privacy": map[string]any{"redact_patterns": []string{}, "forbidden_output_patterns": []string{}},
			"critical_failure_if": []string{},
		},
	}
	b, _ := json.Marshal(c)
	path := writeJSONLFile(t, dir, "bad_risk.jsonl", []string{string(b)})

	_, err := LoadCases([]string{path})
	if err == nil {
		t.Fatal("expected error for unsupported risk level, got nil")
	}
	if !strings.Contains(err.Error(), "unknown risk_level") {
		t.Errorf("expected 'unknown risk_level' in error, got: %v", err)
	}
}

func TestLoadCases_TODODecisionRejected(t *testing.T) {
	dir := t.TempDir()
	path := writeJSONLFile(t, dir, "draft.jsonl", []string{
		validCaseLine("draft-001", "lead", "TODO_REVIEW_decision"),
	})

	_, err := LoadCases([]string{path})
	if err == nil {
		t.Fatal("expected error for TODO decision, got nil")
	}
	if !strings.Contains(err.Error(), "review required") {
		t.Errorf("expected 'review required' in error, got: %v", err)
	}
}

func TestLoadCases_EmptyLinesSkipped(t *testing.T) {
	dir := t.TempDir()
	path := writeJSONLFile(t, dir, "sparse.jsonl", []string{
		"",
		validCaseLine("lead-sparse-1", "lead", "keep"),
		"",
		validCaseLine("lead-sparse-2", "alert", "ignore"),
		"",
	})

	cases, err := LoadCases([]string{path})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cases) != 2 {
		t.Errorf("expected 2 cases, got %d", len(cases))
	}
}

func TestLoadCasesFromDir_MultipleFiles(t *testing.T) {
	dir := t.TempDir()
	writeJSONLFile(t, dir, "a_leads.jsonl", []string{validCaseLine("a-lead-001", "lead", "keep")})
	writeJSONLFile(t, dir, "b_alerts.jsonl", []string{validCaseLine("b-alert-001", "alert", "ignore")})

	cases, err := LoadCasesFromDir(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cases) != 2 {
		t.Errorf("expected 2 cases from dir, got %d", len(cases))
	}
}

func TestLoadCases_FixtureDir(t *testing.T) {
	// Integration-style test: load real fixture files if they exist.
	fixtureDir := "../../data/evals/groupscout"
	if _, err := os.Stat(fixtureDir); os.IsNotExist(err) {
		t.Skip("fixture dir not found, skipping real fixture load test")
	}

	cases, err := LoadCasesFromDir(fixtureDir)
	if err != nil {
		t.Fatalf("fixture dir load error: %v", err)
	}
	if len(cases) < 20 {
		t.Errorf("expected at least 20 golden cases, got %d", len(cases))
	}

	ids := make(map[string]bool)
	for _, c := range cases {
		if ids[c.ID] {
			t.Errorf("duplicate case ID in fixture dir: %q", c.ID)
		}
		ids[c.ID] = true
	}
}

func TestLoadCases_AggregatesMultipleErrors(t *testing.T) {
	dir := t.TempDir()
	path := writeJSONLFile(t, dir, "multi_err.jsonl", []string{
		`{not valid`,
		validCaseLine("", "lead", "keep"),
		validCaseLine("ok-001", "lead", "keep"),
	})

	_, err := LoadCases([]string{path})
	if err == nil {
		t.Fatal("expected aggregate errors, got nil")
	}
	errStr := err.Error()
	if !strings.Contains(errStr, "2 error") {
		t.Errorf("expected '2 error' in aggregate error, got: %v", errStr)
	}
}

func TestLoadCases_LineNumbersInErrors(t *testing.T) {
	dir := t.TempDir()
	path := writeJSONLFile(t, dir, "lined.jsonl", []string{
		validCaseLine("ok-001", "lead", "keep"),
		`{broken json`,
	})

	_, err := LoadCases([]string{path})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	// The error should reference line 2
	if !strings.Contains(err.Error(), ":2:") {
		t.Errorf("expected line number ':2:' in error, got: %v", err)
	}
}

func TestParseRawLead(t *testing.T) {
	dir := t.TempDir()
	path := writeJSONLFile(t, dir, "parse.jsonl", []string{validCaseLine("parse-001", "lead", "keep")})
	cases, err := LoadCases([]string{path})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	raw, err := ParseRawLead(cases[0])
	if err != nil {
		t.Fatalf("ParseRawLead: %v", err)
	}
	if raw.Title != "Test Project" {
		t.Errorf("expected title 'Test Project', got %q", raw.Title)
	}
}

func TestParseRawLead_WrongType(t *testing.T) {
	dir := t.TempDir()
	path := writeJSONLFile(t, dir, "parse_alert.jsonl", []string{validCaseLine("alert-001", "alert", "ignore")})
	cases, err := LoadCases([]string{path})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, err = ParseRawLead(cases[0])
	if err == nil {
		t.Fatal("expected error when parsing alert case as lead, got nil")
	}
}

func TestLoadCases_NoFiles(t *testing.T) {
	cases, err := LoadCases([]string{})
	if err != nil {
		t.Fatalf("expected no error for empty file list, got: %v", err)
	}
	if len(cases) != 0 {
		t.Errorf("expected 0 cases for empty file list, got %d", len(cases))
	}
}

func TestLoadCases_MissingFile(t *testing.T) {
	_, err := LoadCases([]string{"/nonexistent/path/file.jsonl"})
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
	if !strings.Contains(err.Error(), "cannot open file") {
		t.Errorf("expected 'cannot open file' in error, got: %v", err)
	}
}

func TestLoadCases_AlertCaseType(t *testing.T) {
	dir := t.TempDir()
	path := writeJSONLFile(t, dir, "alert.jsonl", []string{
		validCaseLine("alert-test-001", "alert", "hard_alert"),
	})

	cases, err := LoadCases([]string{path})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cases[0].CaseType != "alert" {
		t.Errorf("expected case_type alert, got %q", cases[0].CaseType)
	}
}

func TestValidCaseLine_DeterministicOrder(t *testing.T) {
	dir := t.TempDir()
	var lines []string
	for i := 1; i <= 5; i++ {
		lines = append(lines, validCaseLine(fmt.Sprintf("order-%03d", i), "lead", "keep"))
	}
	path := writeJSONLFile(t, dir, "order.jsonl", lines)

	cases, err := LoadCases([]string{path})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for i, c := range cases {
		expected := fmt.Sprintf("order-%03d", i+1)
		if c.ID != expected {
			t.Errorf("position %d: expected ID %q, got %q", i, expected, c.ID)
		}
	}
}
