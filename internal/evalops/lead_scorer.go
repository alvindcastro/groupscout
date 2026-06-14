package evalops

import (
	"fmt"
	"strings"
)

// LeadDecision represents a keep/drop/needs_review decision for a lead case.
type LeadDecision string

const (
	LeadKeep        LeadDecision = "keep"
	LeadDrop        LeadDecision = "drop"
	LeadNeedsReview LeadDecision = "needs_review"
)

// ScoreResult holds the result of scoring a lead eval case.
type ScoreResult struct {
	CaseID   string
	Decision LeadDecision
	Score    float64
	Reasons  []string
	Findings []Finding
}

// Finding is a single quality check outcome.
type Finding struct {
	Severity string // "critical", "warning", "info"
	Code     string
	Message  string
}

// LeadScorerResult is the complete output from the lead relevance scorer.
type LeadScorerResult struct {
	CaseID            string
	Expected          LeadDecision
	Actual            LeadDecision
	ScoreBandOK       bool
	Score             float64
	Findings          []Finding
	IsCriticalFailure bool
	Reasons           []string
}

// ScoreLeadCase evaluates a lead Case against its expected decision and score band.
// It returns a LeadScorerResult with findings at the appropriate severity.
// No live LLMs or external APIs are called.
func ScoreLeadCase(c Case) LeadScorerResult {
	result := LeadScorerResult{
		CaseID:   c.ID,
		Expected: LeadDecision(c.Expected.Decision),
	}

	raw, err := ParseRawLead(c)
	if err != nil {
		result.Findings = append(result.Findings, Finding{
			Severity: "critical",
			Code:     "LEAD_PARSE_ERROR",
			Message:  fmt.Sprintf("cannot parse raw lead: %v", err),
		})
		result.IsCriticalFailure = true
		return result
	}

	score, reasons, decision := scoreLeadRaw(raw, c)
	result.Score = score
	result.Reasons = reasons
	result.Actual = decision

	// Check score band
	if score < c.Expected.ScoreBand.Min || score > c.Expected.ScoreBand.Max {
		severity := c.Expected.SeverityOnMismatch
		if severity == "" {
			severity = "warning"
		}
		result.Findings = append(result.Findings, Finding{
			Severity: severity,
			Code:     "LEAD_SCORE_BAND_MISS",
			Message:  fmt.Sprintf("score %.1f outside expected band [%.1f, %.1f]", score, c.Expected.ScoreBand.Min, c.Expected.ScoreBand.Max),
		})
		if severity == "critical" {
			result.IsCriticalFailure = true
		}
	}

	// Check decision match
	if result.Actual != result.Expected {
		severity := c.Expected.SeverityOnMismatch
		if severity == "" {
			severity = "warning"
		}
		result.Findings = append(result.Findings, Finding{
			Severity: severity,
			Code:     "LEAD_DECISION_MISMATCH",
			Message:  fmt.Sprintf("expected decision %q but got %q (score=%.1f)", result.Expected, result.Actual, score),
		})
		if severity == "critical" {
			result.IsCriticalFailure = true
		}
	}

	// Check for duplicate case: if metadata says duplicate_of, must drop
	if raw.Metadata != nil {
		if _, isDup := raw.Metadata["duplicate_of"]; isDup {
			if result.Actual != LeadDrop {
				result.Findings = append(result.Findings, Finding{
					Severity: "critical",
					Code:     "LEAD_DUPLICATE_NOT_DROPPED",
					Message:  fmt.Sprintf("case %q has duplicate_of but was not dropped (decision=%q)", c.ID, result.Actual),
				})
				result.IsCriticalFailure = true
			}
		}
	}

	// Check forbidden claims are absent from the raw text
	rawText := strings.ToLower(raw.Text + " " + raw.Title)
	for _, forbidden := range c.Expected.ForbiddenClaims {
		if strings.Contains(rawText, strings.ToLower(forbidden)) {
			result.Findings = append(result.Findings, Finding{
				Severity: "warning",
				Code:     "LEAD_FORBIDDEN_CLAIM_IN_SOURCE",
				Message:  fmt.Sprintf("forbidden claim %q found in raw source text", forbidden),
			})
		}
	}

	// Privacy: check that known redact_patterns (PII acknowledged by the case author)
	// are present in raw source text with a warning — these must be stripped before output.
	// forbidden_output_patterns are checked against enrichment/outreach output elsewhere,
	// not against raw source text (which is expected to contain the unredacted originals).
	for _, rp := range c.Expected.Privacy.RedactPatterns {
		if rp == "" {
			continue
		}
		if strings.Contains(rawText, strings.ToLower(rp)) {
			result.Findings = append(result.Findings, Finding{
				Severity: "warning",
				Code:     "LEAD_PII_IN_SOURCE_NEEDS_REDACTION",
				Message:  fmt.Sprintf("known PII pattern %q found in raw source — must be stripped before any output", rp),
			})
		}
	}

	result.ScoreBandOK = (score >= c.Expected.ScoreBand.Min && score <= c.Expected.ScoreBand.Max)
	return result
}

// scoreLeadRaw computes a priority score from the raw lead fixture data.
// This mirrors the pre-scoring logic in internal/enrichment/scorer.go but
// operates on the fixture shape without importing the enrichment package.
func scoreLeadRaw(raw *RawLead, c Case) (float64, []string, LeadDecision) {
	if raw == nil {
		return 0, nil, LeadDrop
	}

	// Duplicate check first
	if raw.Metadata != nil {
		if _, isDup := raw.Metadata["duplicate_of"]; isDup {
			return -10, []string{"duplicate detected by metadata"}, LeadDrop
		}
		if parseError, _ := raw.Metadata["parse_error"].(bool); parseError {
			return -5, []string{"parse error in source"}, LeadDrop
		}
	}

	var score float64
	var reasons []string

	text := strings.ToLower(raw.Title + " " + raw.Text + " " + raw.Location)

	// Low-value residential patterns force a drop regardless of other signals
	lowValuePatterns := []string{
		"residential", "interior renovation", "landscaping",
		"tenant improvement", "single family", "duplex",
		"townhouse", "condo", "signage", "swimming pool",
		"farmers market", "community", "local vendor",
	}
	for _, kw := range lowValuePatterns {
		if strings.Contains(text, kw) {
			score -= 5
			reasons = append(reasons, fmt.Sprintf("low-value signal: %s", kw))
		}
	}

	// Value-based scoring
	var valueCAD float64
	if raw.ValueCAD != nil {
		valueCAD = *raw.ValueCAD
	}
	if valueCAD >= 10_000_000 {
		score += 4
		reasons = append(reasons, "value >= $10M")
	} else if valueCAD >= 1_000_000 {
		score += 2
		reasons = append(reasons, "value >= $1M")
	}

	// High-value signals
	highValueKeywords := []string{
		"pipeline", "civil", "electrical", "steel", "concrete",
		"infrastructure", "sewer", "watermain", "substation",
		"bridge", "paving", "excavation", "demolition",
		"manufacturing", "warehouse", "logistics", "industrial",
		"feature film", "tv series", "production", "season",
		"conference", "summit", "congress", "trade show", "convention", "symposium",
		"out-of-town", "out-of-area", "crew", "delegate", "international",
	}
	for _, kw := range highValueKeywords {
		if strings.Contains(text, kw) {
			score += 1
			reasons = append(reasons, fmt.Sprintf("high-value signal: %s", kw))
		}
	}

	// Richmond/YVR/Delta geography bonus
	geoPatterns := []string{"richmond", "yvr", "delta", "v6v", "v6x", "v6y", "v7b", "v7c", "v7e", "v4g", "v7b 1y7"}
	for _, gp := range geoPatterns {
		if strings.Contains(text, gp) {
			score += 1
			reasons = append(reasons, fmt.Sprintf("geography signal: %s", gp))
			break // only count geography once
		}
	}

	// Prompt injection detection: source text that commands the scorer is suspicious
	injectionPatterns := []string{
		"ignore prior instructions", "ignore previous instructions",
		"system note:", "classify this lead", "output approved",
	}
	for _, inj := range injectionPatterns {
		if strings.Contains(text, inj) {
			score -= 10
			reasons = append(reasons, fmt.Sprintf("injection attempt detected: %s", inj))
		}
	}

	// Determine decision — conflicting_dates takes precedence over keep for quality reasons
	var decision LeadDecision
	hasConflictingDates := raw.Metadata != nil
	if hasConflictingDates {
		_, hasConflictingDates = raw.Metadata["conflicting_dates"]
	}

	switch {
	case score < 0:
		decision = LeadDrop
	case hasConflictingDates:
		// Conflicting evidence: cannot confidently keep or drop
		decision = LeadNeedsReview
	case score >= 3:
		decision = LeadKeep
	default:
		decision = LeadDrop
	}

	return score, reasons, decision
}

// MissingSourceEvidence checks whether a lead case's expected evidence items
// can be found in the raw source text. Returns findings for missing evidence.
func MissingSourceEvidence(c Case) []Finding {
	if c.CaseType != "lead" {
		return nil
	}
	raw, err := ParseRawLead(c)
	if err != nil {
		return []Finding{{Severity: "warning", Code: "EVIDENCE_PARSE_ERROR", Message: err.Error()}}
	}

	text := strings.ToLower(raw.Text + " " + raw.Title + " " + raw.Location)
	var findings []Finding
	for _, ev := range c.Expected.Evidence {
		if !strings.Contains(text, strings.ToLower(ev)) {
			findings = append(findings, Finding{
				Severity: "warning",
				Code:     "EVIDENCE_NOT_IN_SOURCE",
				Message:  fmt.Sprintf("expected evidence %q not found in raw source text", ev),
			})
		}
	}
	return findings
}
