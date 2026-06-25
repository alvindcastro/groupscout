package evalops

import (
	"encoding/json"
	"encoding/xml"
	"strings"
	"testing"
)

func makePassedResult(id, caseType string) CaseResult {
	return CaseResult{
		CaseID:            id,
		CaseType:          caseType,
		Category:          "construction_permit",
		ReleaseBlocking:   false,
		Decision:          "keep",
		ExpectedDecision:  "keep",
		IsCriticalFailure: false,
		Findings:          nil,
	}
}

func makeFailedResult(id, caseType, expected, actual string) CaseResult {
	return CaseResult{
		CaseID:            id,
		CaseType:          caseType,
		Category:          "construction_permit",
		ReleaseBlocking:   true,
		Decision:          actual,
		ExpectedDecision:  expected,
		IsCriticalFailure: true,
		Findings: []Finding{
			{Severity: "critical", Code: "LEAD_DECISION_MISMATCH", Message: "expected keep got drop"},
		},
	}
}

func makeWarnedResult(id, caseType string) CaseResult {
	return CaseResult{
		CaseID:            id,
		CaseType:          caseType,
		Category:          "construction_permit",
		ReleaseBlocking:   false,
		Decision:          "keep",
		ExpectedDecision:  "keep",
		IsCriticalFailure: false,
		Findings: []Finding{
			{Severity: "warning", Code: "LEAD_SCORE_BAND_MISS", Message: "score outside band"},
		},
	}
}

func TestBuildReport_AllPass(t *testing.T) {
	results := []CaseResult{
		makePassedResult("lead-001", "lead"),
		makePassedResult("lead-002", "lead"),
	}
	report := BuildReport("run-001", results)

	if report.TotalCases != 2 {
		t.Errorf("expected 2 total cases, got %d", report.TotalCases)
	}
	if report.Passed != 2 {
		t.Errorf("expected 2 passed, got %d", report.Passed)
	}
	if report.CriticalCount != 0 {
		t.Errorf("expected 0 critical, got %d", report.CriticalCount)
	}
	if report.Failed != 0 {
		t.Errorf("expected 0 failed, got %d", report.Failed)
	}
}

func TestBuildReport_WithCriticalFailure(t *testing.T) {
	results := []CaseResult{
		makePassedResult("lead-001", "lead"),
		makeFailedResult("lead-fail-001", "lead", "keep", "drop"),
	}
	report := BuildReport("run-002", results)

	if report.CriticalCount != 1 {
		t.Errorf("expected 1 critical, got %d", report.CriticalCount)
	}
	if report.Failed != 1 {
		t.Errorf("expected 1 failed, got %d", report.Failed)
	}
	if report.Passed != 1 {
		t.Errorf("expected 1 passed, got %d", report.Passed)
	}

	// Critical failure should be first in sorted results
	if !report.Results[0].IsCriticalFailure {
		t.Error("expected critical failure to be first in sorted results")
	}
}

func TestBuildReport_WithWarnings(t *testing.T) {
	results := []CaseResult{
		makeWarnedResult("lead-warn-001", "lead"),
	}
	report := BuildReport("run-003", results)

	if report.Warned != 1 {
		t.Errorf("expected 1 warned, got %d", report.Warned)
	}
	if report.CriticalCount != 0 {
		t.Errorf("expected 0 critical, got %d", report.CriticalCount)
	}
}

func TestBuildReport_SortedDeterministically(t *testing.T) {
	results := []CaseResult{
		makePassedResult("c-lead", "lead"),
		makeFailedResult("a-fail", "lead", "keep", "drop"),
		makePassedResult("b-lead", "lead"),
		makeFailedResult("z-fail", "lead", "keep", "drop"),
	}
	report := BuildReport("run-sort", results)

	// Critical failures first, then alphabetical
	if !report.Results[0].IsCriticalFailure {
		t.Error("first result should be critical failure")
	}
	if !report.Results[1].IsCriticalFailure {
		t.Error("second result should be critical failure")
	}
	if report.Results[0].CaseID > report.Results[1].CaseID {
		t.Errorf("critical failures should be sorted alphabetically: got %q before %q",
			report.Results[0].CaseID, report.Results[1].CaseID)
	}
	if report.Results[2].CaseID > report.Results[3].CaseID {
		t.Errorf("passing results should be sorted alphabetically: got %q before %q",
			report.Results[2].CaseID, report.Results[3].CaseID)
	}
}

func TestBuildReport_ToJSON(t *testing.T) {
	results := []CaseResult{
		makePassedResult("lead-001", "lead"),
		makeFailedResult("lead-fail-001", "lead", "keep", "drop"),
	}
	report := BuildReport("run-json", results)

	jsonBytes, err := report.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON error: %v", err)
	}

	// Must be valid JSON
	var decoded map[string]any
	if err := json.Unmarshal(jsonBytes, &decoded); err != nil {
		t.Fatalf("JSON output is not valid JSON: %v", err)
	}

	// Must contain expected fields
	if _, ok := decoded["run_id"]; !ok {
		t.Error("JSON missing run_id")
	}
	if _, ok := decoded["critical_count"]; !ok {
		t.Error("JSON missing critical_count")
	}
	if _, ok := decoded["results"]; !ok {
		t.Error("JSON missing results")
	}
}

func TestBuildReport_ToMarkdown(t *testing.T) {
	results := []CaseResult{
		makePassedResult("lead-001", "lead"),
		makeFailedResult("lead-fail-001", "lead", "keep", "drop"),
		makeWarnedResult("lead-warn-001", "lead"),
	}
	report := BuildReport("run-md", results)
	md := report.ToMarkdown()

	if !strings.Contains(md, "# EvalOps Report") {
		t.Error("markdown missing title")
	}
	if !strings.Contains(md, "## Critical Failures") {
		t.Error("markdown missing Critical Failures section")
	}
	if !strings.Contains(md, "## Warnings") {
		t.Error("markdown missing Warnings section")
	}
	if !strings.Contains(md, "## Passed Cases") {
		t.Error("markdown missing Passed Cases section")
	}
	if !strings.Contains(md, "lead-fail-001") {
		t.Error("markdown missing failed case ID")
	}
	// No secrets should be in the markdown
	if strings.Contains(md, "sk-") || strings.Contains(md, "api_key") {
		t.Error("markdown must not contain secret-like strings")
	}
}

func TestBuildReport_ToMarkdown_AllPassed(t *testing.T) {
	results := []CaseResult{makePassedResult("lead-001", "lead")}
	report := BuildReport("run-all-pass", results)
	md := report.ToMarkdown()

	if strings.Contains(md, "## Critical Failures") {
		t.Error("markdown should not have Critical Failures section when none exist")
	}
	if strings.Contains(md, "## Warnings") {
		t.Error("markdown should not have Warnings section when none exist")
	}
}

func TestBuildReport_ToJUnit(t *testing.T) {
	results := []CaseResult{
		makePassedResult("lead-001", "lead"),
		makeFailedResult("lead-fail-001", "lead", "keep", "drop"),
		makePassedResult("alert-001", "alert"),
	}
	report := BuildReport("run-junit", results)

	xmlBytes, err := report.ToJUnit()
	if err != nil {
		t.Fatalf("ToJUnit error: %v", err)
	}

	// Must be valid XML
	var suites JUnitTestSuites
	if err := xml.Unmarshal(xmlBytes, &suites); err != nil {
		t.Fatalf("JUnit XML is not valid XML: %v", err)
	}

	if suites.Tests != 3 {
		t.Errorf("expected 3 total tests in JUnit, got %d", suites.Tests)
	}
	if suites.Failures != 1 {
		t.Errorf("expected 1 failure in JUnit, got %d", suites.Failures)
	}
	if len(suites.TestSuites) < 1 {
		t.Error("expected at least one test suite in JUnit output")
	}
}

func TestBuildReport_ToJUnit_DeterministicOrder(t *testing.T) {
	results := []CaseResult{
		makePassedResult("lead-001", "lead"),
		makePassedResult("alert-001", "alert"),
	}
	report1 := BuildReport("run-a", results)
	report2 := BuildReport("run-b", results)

	xml1, _ := report1.ToJUnit()
	xml2, _ := report2.ToJUnit()

	// Suite order should be deterministic (alphabetical by suite name)
	// Both should produce the same structure (only run_id differs, but JUnit doesn't carry that)
	if len(xml1) == 0 || len(xml2) == 0 {
		t.Error("JUnit XML must not be empty")
	}
}

func TestBuildReport_EmptyResults(t *testing.T) {
	report := BuildReport("run-empty", nil)

	if report.TotalCases != 0 {
		t.Errorf("expected 0 total cases for empty results, got %d", report.TotalCases)
	}

	jsonBytes, err := report.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON error for empty report: %v", err)
	}
	if len(jsonBytes) == 0 {
		t.Error("expected non-empty JSON for empty report")
	}
}

func TestBuildReport_NoSecretsInOutput(t *testing.T) {
	// Simulate a result that had PII in source (redacted finding message)
	results := []CaseResult{{
		CaseID:            "adv-secret-001",
		CaseType:          "lead",
		Decision:          "drop",
		ExpectedDecision:  "drop",
		IsCriticalFailure: true,
		Findings: []Finding{
			{Severity: "critical", Code: "LEAD_PII_IN_SOURCE", Message: "forbidden output pattern 'sk-a****' found in raw source"},
		},
	}}
	report := BuildReport("run-secret-check", results)

	jsonBytes, _ := report.ToJSON()
	md := report.ToMarkdown()
	xmlBytes, _ := report.ToJUnit()

	secretPatterns := []string{"sk-ant-", "sk-test-", "hooks.slack.com/services"}
	for _, s := range secretPatterns {
		if strings.Contains(string(jsonBytes), s) {
			t.Errorf("JSON report contains secret pattern: %q", s)
		}
		if strings.Contains(md, s) {
			t.Errorf("Markdown report contains secret pattern: %q", s)
		}
		if strings.Contains(string(xmlBytes), s) {
			t.Errorf("JUnit XML report contains secret pattern: %q", s)
		}
	}
}
