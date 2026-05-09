package evalops

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadCasesLoadsOneValidJSONLFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cases.jsonl")
	writeEvalFixture(t, path, `{"id":"lead-valid-001","case_type":"lead","category":"construction_permit","risk_level":"critical","source":{"system":"richmond_permits","type":"permit_pdf","fixture_url":"https://fixtures.groupscout.local/richmond/lead-valid-001.pdf","collected_at":"2026-05-07T00:00:00Z"},"raw":{"title":"Airport Logistics Centre Phase 2","text":"Declared value CAD 8400000. Estimated construction schedule 14 months.","issued_at":"2026-04-20","location":"Richmond, BC","value_cad":8400000,"metadata":{"project_subtype":"industrial_warehouse"}},"expected":{"eval_result":"pass","decision":"keep","release_blocking":true,"severity_on_mismatch":"critical","score_band":{"min":8,"max":10},"enrichment":{"project_type":"industrial construction","estimated_room_nights":{"min":1200,"max":4200,"unknown_allowed":false},"project_duration_months":{"min":10,"max":18,"unknown_allowed":false},"lodging_need":"high","confidence":"medium"},"alert":null,"evidence":[{"claim":"large commercial construction","must_support_with":"CAD 8400000"}],"forbidden_claims":[],"privacy":{"must_redact":[],"forbidden_patterns":[]},"critical_failure_if":["drops the lead"]},"notes":"valid"}`)

	cases, err := LoadCases(path)
	if err != nil {
		t.Fatalf("LoadCases() error = %v", err)
	}
	if len(cases) != 1 {
		t.Fatalf("LoadCases() returned %d cases, want 1", len(cases))
	}
	got := cases[0]
	if got.ID != "lead-valid-001" || got.Source.System != "richmond_permits" {
		t.Fatalf("loaded wrong case: %+v", got)
	}
	if got.Raw.ValueCAD == nil || *got.Raw.ValueCAD != 8400000 {
		t.Fatalf("Raw.ValueCAD = %v, want 8400000", got.Raw.ValueCAD)
	}
}

func TestLoadCasesAggregatesValidationErrorsWithLineNumbers(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.jsonl")
	writeEvalFixture(t, path, strings.Join([]string{
		`{"id":`,
		`{"case_type":"lead","category":"construction_permit","risk_level":"critical","source":{"system":"richmond_permits","type":"permit_pdf","fixture_url":"https://fixtures.groupscout.local/missing-id.pdf","collected_at":"2026-05-07T00:00:00Z"},"raw":{"title":"x","text":"x","issued_at":"2026-04-20","location":"Richmond, BC","value_cad":1,"metadata":{}},"expected":{"eval_result":"pass","decision":"keep","release_blocking":true,"severity_on_mismatch":"critical","score_band":{"min":1,"max":2},"enrichment":null,"alert":null,"evidence":[],"forbidden_claims":[],"privacy":{"must_redact":[],"forbidden_patterns":[]},"critical_failure_if":[]}}`,
		`{"id":"bad-risk","case_type":"lead","category":"construction_permit","risk_level":"urgent","source":{"system":"richmond_permits","type":"permit_pdf","fixture_url":"https://fixtures.groupscout.local/bad-risk.pdf","collected_at":"2026-05-07T00:00:00Z"},"raw":{"title":"x","text":"x","issued_at":"2026-04-20","location":"Richmond, BC","value_cad":1,"metadata":{}},"expected":{"eval_result":"pass","decision":"keep","release_blocking":true,"severity_on_mismatch":"critical","score_band":{"min":1,"max":2},"enrichment":null,"alert":null,"evidence":[],"forbidden_claims":[],"privacy":{"must_redact":[],"forbidden_patterns":[]},"critical_failure_if":[]}}`,
		`{"id":"bad-source-type","case_type":"lead","category":"construction_permit","risk_level":"critical","source":{"system":"richmond_permits","type":"spreadsheet","fixture_url":"https://fixtures.groupscout.local/bad-source-type.xlsx","collected_at":"2026-05-07T00:00:00Z"},"raw":{"title":"x","text":"x","issued_at":"2026-04-20","location":"Richmond, BC","value_cad":1,"metadata":{}},"expected":{"eval_result":"pass","decision":"keep","release_blocking":true,"severity_on_mismatch":"critical","score_band":{"min":1,"max":2},"enrichment":null,"alert":null,"evidence":[],"forbidden_claims":[],"privacy":{"must_redact":[],"forbidden_patterns":[]},"critical_failure_if":[]}}`,
		`{"id":"missing-expected","case_type":"lead","category":"construction_permit","risk_level":"critical","source":{"system":"richmond_permits","type":"permit_pdf","fixture_url":"https://fixtures.groupscout.local/missing-expected.pdf","collected_at":"2026-05-07T00:00:00Z"},"raw":{"title":"x","text":"x","issued_at":"2026-04-20","location":"Richmond, BC","value_cad":1,"metadata":{}}}`,
	}, "\n"))

	_, err := LoadCases(path)
	if err == nil {
		t.Fatal("LoadCases() error = nil, want validation errors")
	}
	var validationErrors ValidationErrors
	if !errors.As(err, &validationErrors) {
		t.Fatalf("LoadCases() error type = %T, want ValidationErrors", err)
	}
	if len(validationErrors) < 5 {
		t.Fatalf("got %d validation errors, want at least 5: %v", len(validationErrors), validationErrors)
	}
	errText := err.Error()
	for _, want := range []string{"line 1", "missing id", "unsupported risk_level", "unknown source type", "missing expected"} {
		if !strings.Contains(errText, want) {
			t.Fatalf("validation error %q missing %q", errText, want)
		}
	}
}

func TestLoadCasesRejectsDuplicateIDsAcrossFiles(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "a.jsonl")
	second := filepath.Join(dir, "b.jsonl")
	caseJSON := `{"id":"dupe-001","case_type":"lead","category":"construction_permit","risk_level":"critical","source":{"system":"richmond_permits","type":"permit_pdf","fixture_url":"https://fixtures.groupscout.local/dupe-001.pdf","collected_at":"2026-05-07T00:00:00Z"},"raw":{"title":"x","text":"x","issued_at":"2026-04-20","location":"Richmond, BC","value_cad":1,"metadata":{}},"expected":{"eval_result":"pass","decision":"keep","release_blocking":true,"severity_on_mismatch":"critical","score_band":{"min":1,"max":2},"enrichment":null,"alert":null,"evidence":[],"forbidden_claims":[],"privacy":{"must_redact":[],"forbidden_patterns":[]},"critical_failure_if":[]}}`
	writeEvalFixture(t, first, caseJSON)
	writeEvalFixture(t, second, caseJSON)

	_, err := LoadCases(first, second)
	if err == nil {
		t.Fatal("LoadCases() error = nil, want duplicate ID error")
	}
	if !strings.Contains(err.Error(), "duplicate id dupe-001") {
		t.Fatalf("LoadCases() error = %q, want duplicate id", err)
	}
}

func writeEvalFixture(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content+"\n"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
}
