package evalops

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"sort"
	"strings"
	"time"
)

// EvalReport is the full output from a single eval run.
type EvalReport struct {
	RunID         string       `json:"run_id"`
	GeneratedAt   string       `json:"generated_at"`
	TotalCases    int          `json:"total_cases"`
	Passed        int          `json:"passed"`
	Warned        int          `json:"warned"`
	Failed        int          `json:"failed"`
	CriticalCount int          `json:"critical_count"`
	Results       []CaseResult `json:"results"`
}

// CaseResult holds all findings for a single evaluated case.
type CaseResult struct {
	CaseID            string    `json:"case_id"`
	CaseType          string    `json:"case_type"`
	Category          string    `json:"category"`
	ReleaseBlocking   bool      `json:"release_blocking"`
	Decision          string    `json:"decision"`
	ExpectedDecision  string    `json:"expected_decision"`
	IsCriticalFailure bool      `json:"is_critical_failure"`
	Score             float64   `json:"score,omitempty"`
	Findings          []Finding `json:"findings"`
}

// BuildReport assembles an EvalReport from a set of case results.
// Results are sorted deterministically: critical failures first, then by case ID.
func BuildReport(runID string, results []CaseResult) EvalReport {
	// Sort: critical first, then alphabetically by case ID
	sorted := make([]CaseResult, len(results))
	copy(sorted, results)
	sort.SliceStable(sorted, func(i, j int) bool {
		ci, cj := sorted[i], sorted[j]
		if ci.IsCriticalFailure != cj.IsCriticalFailure {
			return ci.IsCriticalFailure
		}
		return ci.CaseID < cj.CaseID
	})

	report := EvalReport{
		RunID:       runID,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		TotalCases:  len(results),
		Results:     sorted,
	}

	for _, r := range results {
		if r.IsCriticalFailure {
			report.Failed++
			report.CriticalCount++
		} else if hasWarnings(r.Findings) {
			report.Warned++
		} else {
			report.Passed++
		}
	}

	return report
}

func hasWarnings(findings []Finding) bool {
	for _, f := range findings {
		if f.Severity == "warning" || f.Severity == "critical" {
			return true
		}
	}
	return false
}

// ToJSON returns a deterministic JSON encoding of the report.
func (r EvalReport) ToJSON() ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}

// ToMarkdown returns a human-readable Markdown summary of the report.
func (r EvalReport) ToMarkdown() string {
	var b strings.Builder

	b.WriteString("# EvalOps Report\n\n")
	b.WriteString(fmt.Sprintf("**Run ID:** `%s`  \n", r.RunID))
	b.WriteString(fmt.Sprintf("**Generated:** %s  \n\n", r.GeneratedAt))

	b.WriteString("## Summary\n\n")
	b.WriteString(fmt.Sprintf("| Metric | Value |\n|---|---|\n"))
	b.WriteString(fmt.Sprintf("| Total Cases | %d |\n", r.TotalCases))
	b.WriteString(fmt.Sprintf("| Passed | %d |\n", r.Passed))
	b.WriteString(fmt.Sprintf("| Warnings | %d |\n", r.Warned))
	b.WriteString(fmt.Sprintf("| Critical Failures | %d |\n", r.CriticalCount))
	b.WriteString("\n")

	if r.CriticalCount > 0 {
		b.WriteString("## Critical Failures\n\n")
		for _, res := range r.Results {
			if !res.IsCriticalFailure {
				continue
			}
			b.WriteString(fmt.Sprintf("### %s\n\n", res.CaseID))
			b.WriteString(fmt.Sprintf("- **Type:** %s\n", res.CaseType))
			b.WriteString(fmt.Sprintf("- **Expected:** `%s` | **Actual:** `%s`\n", res.ExpectedDecision, res.Decision))
			if len(res.Findings) > 0 {
				b.WriteString("- **Findings:**\n")
				for _, f := range res.Findings {
					b.WriteString(fmt.Sprintf("  - `[%s]` %s: %s\n", f.Severity, f.Code, f.Message))
				}
			}
			b.WriteString("\n")
		}
	}

	if r.Warned > 0 {
		b.WriteString("## Warnings\n\n")
		for _, res := range r.Results {
			if res.IsCriticalFailure || !hasWarnings(res.Findings) {
				continue
			}
			b.WriteString(fmt.Sprintf("### %s\n\n", res.CaseID))
			for _, f := range res.Findings {
				if f.Severity == "warning" {
					b.WriteString(fmt.Sprintf("- `[%s]` %s: %s\n", f.Severity, f.Code, f.Message))
				}
			}
			b.WriteString("\n")
		}
	}

	if r.Passed > 0 {
		b.WriteString("## Passed Cases\n\n")
		for _, res := range r.Results {
			if res.IsCriticalFailure || hasWarnings(res.Findings) {
				continue
			}
			b.WriteString(fmt.Sprintf("- `%s` (%s) — decision `%s` as expected\n", res.CaseID, res.CaseType, res.Decision))
		}
		b.WriteString("\n")
	}

	return b.String()
}

// JUnitTestSuites is the root XML element for JUnit XML output.
type JUnitTestSuites struct {
	XMLName    xml.Name         `xml:"testsuites"`
	Name       string           `xml:"name,attr"`
	Tests      int              `xml:"tests,attr"`
	Failures   int              `xml:"failures,attr"`
	TestSuites []JUnitTestSuite `xml:"testsuite"`
}

// JUnitTestSuite represents a single test suite (one per case type).
type JUnitTestSuite struct {
	XMLName   xml.Name        `xml:"testsuite"`
	Name      string          `xml:"name,attr"`
	Tests     int             `xml:"tests,attr"`
	Failures  int             `xml:"failures,attr"`
	TestCases []JUnitTestCase `xml:"testcase"`
}

// JUnitTestCase represents a single test case result.
type JUnitTestCase struct {
	XMLName   xml.Name      `xml:"testcase"`
	Classname string        `xml:"classname,attr"`
	Name      string        `xml:"name,attr"`
	Failure   *JUnitFailure `xml:"failure,omitempty"`
}

// JUnitFailure holds the failure message for a failed test case.
type JUnitFailure struct {
	Message string `xml:"message,attr"`
	Text    string `xml:",chardata"`
}

// ToJUnit returns a JUnit XML encoding of the report for CI upload.
func (r EvalReport) ToJUnit() ([]byte, error) {
	suiteMap := make(map[string]*JUnitTestSuite)

	for _, res := range r.Results {
		suiteName := "evalops." + res.CaseType
		suite, ok := suiteMap[suiteName]
		if !ok {
			suite = &JUnitTestSuite{Name: suiteName}
			suiteMap[suiteName] = suite
		}

		tc := JUnitTestCase{
			Classname: suiteName,
			Name:      res.CaseID,
		}
		if res.IsCriticalFailure {
			var msgs []string
			for _, f := range res.Findings {
				msgs = append(msgs, fmt.Sprintf("[%s] %s: %s", f.Severity, f.Code, f.Message))
			}
			tc.Failure = &JUnitFailure{
				Message: fmt.Sprintf("critical failure: expected=%s actual=%s", res.ExpectedDecision, res.Decision),
				Text:    strings.Join(msgs, "\n"),
			}
			suite.Failures++
		}
		suite.Tests++
		suite.TestCases = append(suite.TestCases, tc)
	}

	// Collect and sort suites for deterministic output
	var suiteNames []string
	for name := range suiteMap {
		suiteNames = append(suiteNames, name)
	}
	sort.Strings(suiteNames)

	suites := JUnitTestSuites{
		Name:     "groupscout-evalops",
		Tests:    r.TotalCases,
		Failures: r.CriticalCount,
	}
	for _, name := range suiteNames {
		suites.TestSuites = append(suites.TestSuites, *suiteMap[name])
	}

	return xml.MarshalIndent(suites, "", "  ")
}
