package evalops

import (
	"fmt"
	"strings"
)

// EnrichedLeadInput holds the AI enrichment output to be scored.
// Fields mirror the GQ0 quality dimensions for enrichment completeness.
type EnrichedLeadInput struct {
	ProjectType           string  `json:"project_type"`
	EstimatedRoomNights   float64 `json:"estimated_room_nights"`
	RoomNightsUnknown     bool    `json:"room_nights_unknown"`
	ProjectDurationMonths float64 `json:"project_duration_months"`
	DurationUnknown       bool    `json:"duration_unknown"`
	LodgingNeed           string  `json:"lodging_need"`
	Rationale             string  `json:"rationale"`
	SourceEvidence        string  `json:"source_evidence"`
	Confidence            string  `json:"confidence"`
}

// EnrichmentScorerResult holds the outcome of enrichment completeness scoring.
type EnrichmentScorerResult struct {
	CaseID            string
	Findings          []Finding
	IsCriticalFailure bool
}

// validConfidenceLevels are the accepted confidence field values.
var validConfidenceLevels = map[string]bool{
	"high":    true,
	"medium":  true,
	"low":     true,
	"unknown": true,
}

// validLodgingNeeds are the accepted lodging_need field values.
var validLodgingNeeds = map[string]bool{
	"high":    true,
	"medium":  true,
	"low":     true,
	"unknown": true,
	"none":    true,
}

// ScoreEnrichment checks an AI enrichment output against the case's expected
// enrichment fields. Returns findings for any completeness or critical failures.
// Returns an empty result if the case has no expected enrichment (alert-only cases).
func ScoreEnrichment(c Case, enriched EnrichedLeadInput) EnrichmentScorerResult {
	result := EnrichmentScorerResult{CaseID: c.ID}

	if c.Expected.Enrichment == nil {
		// Alert-only or drop case — no enrichment expected
		return result
	}

	exp := c.Expected.Enrichment

	// Required field: project_type
	if enriched.ProjectType == "" {
		result.Findings = append(result.Findings, Finding{
			Severity: "critical",
			Code:     "ENRICH_MISSING_PROJECT_TYPE",
			Message:  "required field project_type is empty",
		})
		result.IsCriticalFailure = true
	}

	// Required field: lodging_need
	if enriched.LodgingNeed == "" {
		result.Findings = append(result.Findings, Finding{
			Severity: "critical",
			Code:     "ENRICH_MISSING_LODGING_NEED",
			Message:  "required field lodging_need is empty",
		})
		result.IsCriticalFailure = true
	} else if !validLodgingNeeds[strings.ToLower(enriched.LodgingNeed)] {
		result.Findings = append(result.Findings, Finding{
			Severity: "warning",
			Code:     "ENRICH_INVALID_LODGING_NEED",
			Message:  fmt.Sprintf("lodging_need %q is not a recognized value", enriched.LodgingNeed),
		})
	}

	// Required field: rationale
	if strings.TrimSpace(enriched.Rationale) == "" {
		result.Findings = append(result.Findings, Finding{
			Severity: "critical",
			Code:     "ENRICH_MISSING_RATIONALE",
			Message:  "required field rationale is empty",
		})
		result.IsCriticalFailure = true
	}

	// Required field: source_evidence
	if strings.TrimSpace(enriched.SourceEvidence) == "" {
		result.Findings = append(result.Findings, Finding{
			Severity: "critical",
			Code:     "ENRICH_MISSING_SOURCE_EVIDENCE",
			Message:  "required field source_evidence is empty",
		})
		result.IsCriticalFailure = true
	}

	// Required field: confidence
	if enriched.Confidence == "" {
		result.Findings = append(result.Findings, Finding{
			Severity: "critical",
			Code:     "ENRICH_MISSING_CONFIDENCE",
			Message:  "required field confidence is empty",
		})
		result.IsCriticalFailure = true
	} else if !validConfidenceLevels[strings.ToLower(enriched.Confidence)] {
		result.Findings = append(result.Findings, Finding{
			Severity: "warning",
			Code:     "ENRICH_INVALID_CONFIDENCE",
			Message:  fmt.Sprintf("confidence %q is not a recognized value", enriched.Confidence),
		})
	}

	// Check room nights: if unknown is not allowed, must have a value
	rnRange := exp.EstimatedRoomNights
	if enriched.RoomNightsUnknown {
		if !rnRange.UnknownAllowed {
			result.Findings = append(result.Findings, Finding{
				Severity: "critical",
				Code:     "ENRICH_ROOM_NIGHTS_UNKNOWN_NOT_ALLOWED",
				Message:  "room_nights_unknown=true but case requires a room night estimate",
			})
			result.IsCriticalFailure = true
		}
	} else {
		// Value provided — check range
		if enriched.EstimatedRoomNights < rnRange.Min || enriched.EstimatedRoomNights > rnRange.Max {
			result.Findings = append(result.Findings, Finding{
				Severity: "warning",
				Code:     "ENRICH_ROOM_NIGHTS_OUT_OF_RANGE",
				Message: fmt.Sprintf("estimated_room_nights %.0f outside expected range [%.0f, %.0f]",
					enriched.EstimatedRoomNights, rnRange.Min, rnRange.Max),
			})
		}

		// Critical: confident unsupported claim — high confidence with a very precise round number
		// and the case expects unknown_allowed=true (meaning evidence is likely insufficient)
		if strings.ToLower(enriched.Confidence) == "high" && rnRange.UnknownAllowed {
			result.Findings = append(result.Findings, Finding{
				Severity: "critical",
				Code:     "ENRICH_UNSUPPORTED_HIGH_CONFIDENCE",
				Message: fmt.Sprintf("confidence=high for room nights %.0f when evidence is likely insufficient (unknown_allowed=true in case)",
					enriched.EstimatedRoomNights),
			})
			result.IsCriticalFailure = true
		}
	}

	// Check project duration: if unknown is not allowed, must have a value
	durRange := exp.ProjectDurationMonths
	if enriched.DurationUnknown {
		if !durRange.UnknownAllowed {
			result.Findings = append(result.Findings, Finding{
				Severity: "critical",
				Code:     "ENRICH_DURATION_UNKNOWN_NOT_ALLOWED",
				Message:  "duration_unknown=true but case requires a duration estimate",
			})
			result.IsCriticalFailure = true
		}
	} else {
		// Duration provided — check range
		if enriched.ProjectDurationMonths < durRange.Min || enriched.ProjectDurationMonths > durRange.Max {
			result.Findings = append(result.Findings, Finding{
				Severity: "warning",
				Code:     "ENRICH_DURATION_OUT_OF_RANGE",
				Message: fmt.Sprintf("project_duration_months %.0f outside expected range [%.0f, %.0f]",
					enriched.ProjectDurationMonths, durRange.Min, durRange.Max),
			})
		}
	}

	// Contradictory evidence: check if rationale mentions confidence signals
	// that conflict with "unknown" evidence markers.
	if strings.Contains(strings.ToLower(enriched.Rationale), "confirmed") &&
		strings.ToLower(enriched.Confidence) == "high" &&
		strings.Contains(strings.ToLower(enriched.SourceEvidence), "unknown") {
		result.Findings = append(result.Findings, Finding{
			Severity: "warning",
			Code:     "ENRICH_CONTRADICTORY_EVIDENCE",
			Message:  "rationale claims 'confirmed' but source_evidence contains 'unknown'",
		})
	}

	// Lodging need match check
	if exp.LodgingNeed != "" && strings.ToLower(enriched.LodgingNeed) != strings.ToLower(exp.LodgingNeed) {
		result.Findings = append(result.Findings, Finding{
			Severity: "warning",
			Code:     "ENRICH_LODGING_NEED_MISMATCH",
			Message: fmt.Sprintf("expected lodging_need %q but got %q",
				exp.LodgingNeed, enriched.LodgingNeed),
		})
	}

	return result
}
