package evalops

import (
	"fmt"
	"strings"
	"testing"
)

func makeReport(passed, warned, critical int) EvalReport {
	var results []CaseResult
	for i := 0; i < passed; i++ {
		results = append(results, makePassedResult(fmt.Sprintf("pass-%03d", i+1), "lead"))
	}
	for i := 0; i < warned; i++ {
		results = append(results, makeWarnedResult(fmt.Sprintf("warn-%03d", i+1), "lead"))
	}
	for i := 0; i < critical; i++ {
		results = append(results, makeFailedResult(fmt.Sprintf("fail-%03d", i+1), "lead", "keep", "drop"))
	}
	return BuildReport("gate-test-run", results)
}

func TestRunGate_CriticalFailureExitsNonZero(t *testing.T) {
	report := makeReport(3, 0, 1) // 1 critical failure
	result := RunGate(report, GateConfig{})

	if result.Pass {
		t.Error("expected gate to block (Pass=false) for critical failure")
	}
	if result.CriticalCount != 1 {
		t.Errorf("expected 1 critical count, got %d", result.CriticalCount)
	}
	if !strings.Contains(result.Summary, "FAIL") {
		t.Errorf("expected summary to contain FAIL, got: %q", result.Summary)
	}
	if len(result.BlockingReasons) == 0 {
		t.Error("expected at least one blocking reason for critical failure")
	}
}

func TestRunGate_WarningOnlyPassesByDefault(t *testing.T) {
	report := makeReport(2, 2, 0) // warnings only
	result := RunGate(report, GateConfig{})

	if !result.Pass {
		t.Errorf("expected gate to pass for warning-only, got blocking reasons: %v", result.BlockingReasons)
	}
	if result.WarningCount != 2 {
		t.Errorf("expected 2 warnings, got %d", result.WarningCount)
	}
	if !strings.Contains(result.Summary, "PASS") {
		t.Errorf("expected summary to contain PASS, got: %q", result.Summary)
	}
}

func TestRunGate_WarningsAsErrorsBlocks(t *testing.T) {
	report := makeReport(2, 1, 0)
	cfg := GateConfig{WarningsAsErrors: true}
	result := RunGate(report, cfg)

	if result.Pass {
		t.Error("expected gate to block when warnings_as_errors=true")
	}
	if !strings.Contains(result.Summary, "FAIL") {
		t.Errorf("expected summary to contain FAIL, got: %q", result.Summary)
	}
}

func TestRunGate_MaxWarningsExceeded(t *testing.T) {
	report := makeReport(0, 5, 0) // 5 warnings
	cfg := GateConfig{MaxWarnings: 3}
	result := RunGate(report, cfg)

	if result.Pass {
		t.Error("expected gate to block when warning count exceeds max_warnings")
	}
	found := false
	for _, reason := range result.BlockingReasons {
		if strings.Contains(reason, "WARNING LIMIT") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected WARNING LIMIT blocking reason, got: %v", result.BlockingReasons)
	}
}

func TestRunGate_MaxWarningsNotExceeded(t *testing.T) {
	report := makeReport(0, 3, 0) // exactly at limit
	cfg := GateConfig{MaxWarnings: 3}
	result := RunGate(report, cfg)

	if !result.Pass {
		t.Errorf("expected gate to pass when warnings <= max_warnings, got: %v", result.BlockingReasons)
	}
}

func TestRunGate_AllPass(t *testing.T) {
	report := makeReport(5, 0, 0)
	result := RunGate(report, GateConfig{})

	if !result.Pass {
		t.Errorf("expected gate to pass for all-passing report, got: %v", result.BlockingReasons)
	}
	if result.CriticalCount != 0 {
		t.Errorf("expected 0 critical, got %d", result.CriticalCount)
	}
}

func TestRunGate_MultipleCriticalFailures(t *testing.T) {
	report := makeReport(1, 0, 3) // 3 critical failures
	result := RunGate(report, GateConfig{})

	if result.Pass {
		t.Error("expected gate to block for multiple critical failures")
	}
	if result.CriticalCount != 3 {
		t.Errorf("expected 3 critical count, got %d", result.CriticalCount)
	}
	if len(result.BlockingReasons) < 3 {
		t.Errorf("expected at least 3 blocking reasons for 3 critical failures, got %d", len(result.BlockingReasons))
	}
}

func TestRunGate_EmptyReport(t *testing.T) {
	report := BuildReport("empty", nil)
	result := RunGate(report, GateConfig{})

	if !result.Pass {
		t.Errorf("expected gate to pass for empty report, got: %v", result.BlockingReasons)
	}
}

func TestRunGate_SummaryContainsCounts(t *testing.T) {
	report := makeReport(3, 2, 1)
	result := RunGate(report, GateConfig{})

	if !strings.Contains(result.Summary, "1 critical") {
		t.Errorf("expected summary to mention critical count, got: %q", result.Summary)
	}
}

func TestRunGate_BlockingReasonsIncludeCaseID(t *testing.T) {
	report := makeReport(0, 0, 1)
	result := RunGate(report, GateConfig{})

	if len(result.BlockingReasons) == 0 {
		t.Fatal("expected at least one blocking reason")
	}
	// Should include the case ID in the blocking reason
	found := false
	for _, reason := range result.BlockingReasons {
		if strings.Contains(reason, "fail-001") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected blocking reason to contain case ID 'fail-001', got: %v", result.BlockingReasons)
	}
}
