package evalops

import (
	"strings"
	"testing"
)

func TestBuildReportSortsAndSummarizesResults(t *testing.T) {
	results := []Result{
		{CaseID: "case-c", Scorer: "lead", Status: ResultPass, Severity: SeverityInfo},
		{CaseID: "case-b", Scorer: "outreach", Status: ResultFail, Severity: SeverityWarning, Message: "warning"},
		{CaseID: "case-a", Scorer: "alert", Status: ResultFail, Severity: SeverityCritical, ReleaseBlocking: true, Message: "critical"},
	}

	report := BuildReport(results)
	if report.Summary.Total != 3 || report.Summary.Passed != 1 || report.Summary.Warnings != 1 || report.Summary.CriticalFailures != 1 || report.Summary.ReleaseBlockingFailures != 1 {
		t.Fatalf("summary = %+v, want total/pass/warning/critical/blocking counts", report.Summary)
	}
	gotOrder := []string{report.Results[0].CaseID, report.Results[1].CaseID, report.Results[2].CaseID}
	wantOrder := []string{"case-a", "case-b", "case-c"}
	for i := range wantOrder {
		if gotOrder[i] != wantOrder[i] {
			t.Fatalf("result order = %v, want %v", gotOrder, wantOrder)
		}
	}
}

func TestJSONSummaryIsDeterministic(t *testing.T) {
	results := []Result{
		{CaseID: "case-b", Scorer: "lead", Status: ResultPass, Severity: SeverityInfo, Message: "ok"},
		{CaseID: "case-a", Scorer: "outreach", Status: ResultFail, Severity: SeverityCritical, ReleaseBlocking: true, Message: "unsafe"},
	}

	first, err := JSONSummary(results)
	if err != nil {
		t.Fatalf("JSONSummary() error = %v", err)
	}
	second, err := JSONSummary(results)
	if err != nil {
		t.Fatalf("JSONSummary() second error = %v", err)
	}
	if string(first) != string(second) {
		t.Fatalf("JSONSummary() unstable:\n%s\n---\n%s", first, second)
	}
	want := `"critical_failures": 1`
	if !strings.Contains(string(first), want) {
		t.Fatalf("JSONSummary() = %s, missing %s", first, want)
	}
}

func TestMarkdownAndJUnitReportsRedactSecretsAndIncludeFailures(t *testing.T) {
	results := []Result{
		{
			CaseID:          "adv-secret-like-token-005",
			Scorer:          "outreach_safety",
			Status:          ResultFail,
			Severity:        SeverityCritical,
			ReleaseBlocking: true,
			Message:         "leaked SECRET_TOKEN=fixture-token-do-not-use and alex.fixture@example.invalid and 604-555-0199",
		},
	}

	markdown := MarkdownSummary(results)
	junit, err := JUnitXML(results)
	if err != nil {
		t.Fatalf("JUnitXML() error = %v", err)
	}
	combined := markdown + string(junit)
	for _, leaked := range []string{"SECRET_TOKEN=fixture-token-do-not-use", "alex.fixture@example.invalid", "604-555-0199"} {
		if strings.Contains(combined, leaked) {
			t.Fatalf("reports leaked %q:\n%s", leaked, combined)
		}
	}
	for _, want := range []string{"adv-secret-like-token-005", "outreach_safety", "critical"} {
		if !strings.Contains(strings.ToLower(combined), strings.ToLower(want)) {
			t.Fatalf("reports missing %q:\n%s", want, combined)
		}
	}
}
