package evalops

import (
	"fmt"
	"strings"
)

// GateConfig holds thresholds for the release gate.
type GateConfig struct {
	// WarningsAsErrors, when true, causes any warning-level finding to block release.
	WarningsAsErrors bool `yaml:"warnings_as_errors" json:"warnings_as_errors"`
	// MaxWarnings is the maximum number of warnings allowed before blocking.
	// 0 means no limit (only critical failures block by default).
	MaxWarnings int `yaml:"max_warnings" json:"max_warnings"`
}

// GateResult is the outcome of the release gate evaluation.
type GateResult struct {
	Pass            bool
	CriticalCount   int
	WarningCount    int
	BlockingReasons []string
	Summary         string
}

// RunGate evaluates an EvalReport against the gate config and returns
// a GateResult. Critical failures always block. Warnings block only
// when WarningsAsErrors is true or MaxWarnings is exceeded.
func RunGate(report EvalReport, cfg GateConfig) GateResult {
	result := GateResult{}

	for _, r := range report.Results {
		if r.IsCriticalFailure {
			result.CriticalCount++
			for _, f := range r.Findings {
				if f.Severity == "critical" {
					result.BlockingReasons = append(result.BlockingReasons,
						fmt.Sprintf("[CRITICAL] %s — %s: %s", r.CaseID, f.Code, f.Message))
				}
			}
		}
		for _, f := range r.Findings {
			if f.Severity == "warning" {
				result.WarningCount++
				if cfg.WarningsAsErrors {
					result.BlockingReasons = append(result.BlockingReasons,
						fmt.Sprintf("[WARNING-AS-ERROR] %s — %s: %s", r.CaseID, f.Code, f.Message))
				}
			}
		}
	}

	// Check MaxWarnings
	if cfg.MaxWarnings > 0 && result.WarningCount > cfg.MaxWarnings {
		result.BlockingReasons = append(result.BlockingReasons,
			fmt.Sprintf("[WARNING LIMIT] %d warnings exceed max_warnings=%d", result.WarningCount, cfg.MaxWarnings))
	}

	result.Pass = len(result.BlockingReasons) == 0

	if result.Pass {
		result.Summary = fmt.Sprintf("PASS — %d cases evaluated, %d warnings, 0 critical failures",
			report.TotalCases, result.WarningCount)
	} else {
		result.Summary = fmt.Sprintf("FAIL — %d critical failure(s), %d warning(s): %s",
			result.CriticalCount, result.WarningCount,
			strings.Join(result.BlockingReasons, "; "))
	}

	return result
}
