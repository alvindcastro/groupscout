package evalops

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"html"
	"regexp"
	"sort"
	"strings"
)

type Report struct {
	Summary ReportSummary `json:"summary"`
	Results []Result      `json:"results"`
}

type ReportSummary struct {
	Total                   int `json:"total"`
	Passed                  int `json:"passed"`
	Warnings                int `json:"warnings"`
	CriticalFailures        int `json:"critical_failures"`
	ReleaseBlockingFailures int `json:"release_blocking_failures"`
}

func BuildReport(results []Result) Report {
	sorted := append([]Result(nil), results...)
	sort.SliceStable(sorted, func(i, j int) bool {
		left, right := sorted[i], sorted[j]
		if severityRank(left) != severityRank(right) {
			return severityRank(left) < severityRank(right)
		}
		if left.CaseID != right.CaseID {
			return left.CaseID < right.CaseID
		}
		return left.Scorer < right.Scorer
	})

	report := Report{Results: sorted}
	report.Summary.Total = len(sorted)
	for _, result := range sorted {
		switch {
		case result.Status == ResultPass:
			report.Summary.Passed++
		case result.Severity == SeverityCritical:
			report.Summary.CriticalFailures++
		case result.Severity == SeverityWarning:
			report.Summary.Warnings++
		}
		if result.Status == ResultFail && result.ReleaseBlocking {
			report.Summary.ReleaseBlockingFailures++
		}
	}
	return report
}

func JSONSummary(results []Result) ([]byte, error) {
	return json.MarshalIndent(BuildReport(redactResults(results)), "", "  ")
}

func MarshalReportJSON(report Report) ([]byte, error) {
	report.Results = redactResults(report.Results)
	return json.MarshalIndent(report, "", "  ")
}

func MarkdownSummary(results []Result) string {
	report := BuildReport(redactResults(results))
	var b strings.Builder
	fmt.Fprintf(&b, "# GroupScout EvalOps Report\n\n")
	fmt.Fprintf(&b, "- Total: %d\n", report.Summary.Total)
	fmt.Fprintf(&b, "- Passed: %d\n", report.Summary.Passed)
	fmt.Fprintf(&b, "- Warnings: %d\n", report.Summary.Warnings)
	fmt.Fprintf(&b, "- Critical failures: %d\n", report.Summary.CriticalFailures)
	fmt.Fprintf(&b, "- Release-blocking failures: %d\n\n", report.Summary.ReleaseBlockingFailures)
	if len(report.Results) == 0 {
		return b.String()
	}
	fmt.Fprintf(&b, "| Case | Scorer | Status | Severity | Blocking | Message |\n")
	fmt.Fprintf(&b, "|---|---|---|---|---:|---|\n")
	for _, result := range report.Results {
		fmt.Fprintf(&b, "| %s | %s | %s | %s | %t | %s |\n",
			escapeMarkdown(result.CaseID),
			escapeMarkdown(result.Scorer),
			result.Status,
			result.Severity,
			result.ReleaseBlocking,
			escapeMarkdown(result.Message),
		)
	}
	return b.String()
}

func JUnitXML(results []Result) ([]byte, error) {
	report := BuildReport(redactResults(results))
	suite := junitTestSuite{
		Name:     "groupscout-evalops",
		Tests:    report.Summary.Total,
		Failures: report.Summary.CriticalFailures + report.Summary.Warnings,
	}
	for _, result := range report.Results {
		testCase := junitTestCase{
			ClassName: result.Scorer,
			Name:      result.CaseID,
		}
		if result.Status == ResultFail {
			testCase.Failure = &junitFailure{
				Type:    string(result.Severity),
				Message: result.Message,
				Text:    result.Message,
			}
		}
		suite.Cases = append(suite.Cases, testCase)
	}
	var b bytes.Buffer
	b.WriteString(xml.Header)
	encoder := xml.NewEncoder(&b)
	encoder.Indent("", "  ")
	if err := encoder.Encode(suite); err != nil {
		return nil, err
	}
	b.WriteByte('\n')
	return b.Bytes(), nil
}

type junitTestSuite struct {
	XMLName  xml.Name        `xml:"testsuite"`
	Name     string          `xml:"name,attr"`
	Tests    int             `xml:"tests,attr"`
	Failures int             `xml:"failures,attr"`
	Cases    []junitTestCase `xml:"testcase"`
}

type junitTestCase struct {
	ClassName string        `xml:"classname,attr"`
	Name      string        `xml:"name,attr"`
	Failure   *junitFailure `xml:"failure,omitempty"`
}

type junitFailure struct {
	Type    string `xml:"type,attr"`
	Message string `xml:"message,attr"`
	Text    string `xml:",chardata"`
}

func severityRank(result Result) int {
	if result.Status == ResultPass {
		return 3
	}
	switch result.Severity {
	case SeverityCritical:
		return 0
	case SeverityWarning:
		return 1
	default:
		return 2
	}
}

func redactResults(results []Result) []Result {
	redacted := append([]Result(nil), results...)
	for i := range redacted {
		redacted[i].Message = redactSensitive(redacted[i].Message)
	}
	return redacted
}

func redactSensitive(text string) string {
	text = webhookPattern.ReplaceAllString(text, "[REDACTED_WEBHOOK]")
	text = secretPattern.ReplaceAllString(text, "[REDACTED_SECRET]")
	text = emailPattern.ReplaceAllString(text, "[REDACTED_EMAIL]")
	text = phonePattern.ReplaceAllString(text, "[REDACTED_PHONE]")
	text = rawPIIPattern.ReplaceAllString(text, "[REDACTED_PII]")
	text = regexpTokenValue.ReplaceAllString(text, "[REDACTED_TOKEN]")
	return text
}

var regexpTokenValue = regexp.MustCompile(`(?i)\bfixture-token-[a-z0-9\-]+\b`)
var rawPIIPattern = regexp.MustCompile(`(?i)\b(?:SIN|SSN)\s*\d{3}[-\s]?\d{3}[-\s]?\d{3}\b`)

func escapeMarkdown(text string) string {
	text = strings.ReplaceAll(text, "|", "\\|")
	return html.EscapeString(text)
}
