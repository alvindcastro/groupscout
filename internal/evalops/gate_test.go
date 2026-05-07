package evalops

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGateFailsCriticalReleaseBlockingReport(t *testing.T) {
	reportPath := writeReportFile(t, Report{
		Summary: ReportSummary{Total: 1, CriticalFailures: 1, ReleaseBlockingFailures: 1},
		Results: []Result{{
			CaseID:          "adv-secret-like-token-005",
			Scorer:          "outreach_safety",
			Status:          ResultFail,
			Severity:        SeverityCritical,
			ReleaseBlocking: true,
			Message:         "unsafe output",
		}},
	})
	thresholdPath := writeThresholdFile(t, "max_critical_failures: 0\nmax_release_blocking_failures: 0\nwarnings_as_errors: false\n")

	result, err := RunGate(context.Background(), GateOptions{ReportPath: reportPath, ThresholdPath: thresholdPath})
	if err != nil {
		t.Fatalf("RunGate() error = %v", err)
	}
	if result.Pass || result.ExitCode != 1 {
		t.Fatalf("gate result = %+v, want failing exit 1", result)
	}
	if !strings.Contains(result.Summary, "blocked") || !strings.Contains(result.Summary, "critical=1") {
		t.Fatalf("summary = %q, want blocked critical summary", result.Summary)
	}
}

func TestGateAllowsWarningOnlyByDefault(t *testing.T) {
	reportPath := writeReportFile(t, Report{
		Summary: ReportSummary{Total: 1, Warnings: 1},
		Results: []Result{{
			CaseID:   "lead-warning",
			Scorer:   "lead_relevance",
			Status:   ResultFail,
			Severity: SeverityWarning,
			Message:  "review needed",
		}},
	})
	thresholdPath := writeThresholdFile(t, "max_critical_failures: 0\nmax_release_blocking_failures: 0\nwarnings_as_errors: false\n")

	result, err := RunGate(context.Background(), GateOptions{ReportPath: reportPath, ThresholdPath: thresholdPath})
	if err != nil {
		t.Fatalf("RunGate() error = %v", err)
	}
	if !result.Pass || result.ExitCode != 0 {
		t.Fatalf("gate result = %+v, want pass", result)
	}
}

func TestGateCanTreatWarningsAsErrors(t *testing.T) {
	reportPath := writeReportFile(t, Report{
		Summary: ReportSummary{Total: 1, Warnings: 1},
		Results: []Result{{
			CaseID:   "lead-warning",
			Scorer:   "lead_relevance",
			Status:   ResultFail,
			Severity: SeverityWarning,
			Message:  "review needed",
		}},
	})
	thresholdPath := writeThresholdFile(t, "max_critical_failures: 0\nmax_release_blocking_failures: 0\nwarnings_as_errors: true\n")

	result, err := RunGate(context.Background(), GateOptions{ReportPath: reportPath, ThresholdPath: thresholdPath})
	if err != nil {
		t.Fatalf("RunGate() error = %v", err)
	}
	if result.Pass || result.ExitCode != 1 {
		t.Fatalf("gate result = %+v, want warning-as-error failure", result)
	}
}

func TestGateReportsMissingFilesAndMalformedThresholds(t *testing.T) {
	reportPath := writeReportFile(t, Report{Summary: ReportSummary{Total: 0}})
	missingPath := filepath.Join(t.TempDir(), "missing.yaml")
	if _, err := RunGate(context.Background(), GateOptions{ReportPath: reportPath, ThresholdPath: missingPath}); err == nil {
		t.Fatal("RunGate() missing threshold error = nil")
	}

	badThresholdPath := writeThresholdFile(t, "max_critical_failures: nope\n")
	if _, err := RunGate(context.Background(), GateOptions{ReportPath: reportPath, ThresholdPath: badThresholdPath}); err == nil {
		t.Fatal("RunGate() malformed threshold error = nil")
	}
}

func writeReportFile(t *testing.T, report Report) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "report.json")
	data, err := MarshalReportJSON(report)
	if err != nil {
		t.Fatalf("MarshalReportJSON() error = %v", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write report: %v", err)
	}
	return path
}

func writeThresholdFile(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "thresholds.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write thresholds: %v", err)
	}
	return path
}
