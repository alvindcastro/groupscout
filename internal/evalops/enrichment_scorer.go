package evalops

import (
	"fmt"
	"strings"
)

type NumericOutput struct {
	Value        int
	Unknown      bool
	UnknownLabel string
}

type EnrichmentOutput struct {
	ProjectType            string
	EstimatedRoomNights    NumericOutput
	ProjectDurationMonths  NumericOutput
	LodgingNeed            string
	Confidence             string
	Rationale              string
	SourceEvidence         []string
	ReviewedForContradicts bool
}

func ScoreEnrichmentCompleteness(c Case, output EnrichmentOutput) Result {
	const scorer = "enrichment_completeness"
	expected := c.Expected.Enrichment
	if expected == nil {
		return passResult(c, scorer, "", 0, "no enrichment expected")
	}

	missing := missingEnrichmentFields(output)
	if len(missing) > 0 {
		return failResult(c, scorer, SeverityCritical, c.Expected.ReleaseBlocking, "", 0, "missing required enrichment fields: "+strings.Join(missing, ", "))
	}
	if hasContradictoryEvidence(c) && strings.EqualFold(output.Confidence, "high") {
		return failResult(c, scorer, SeverityCritical, c.Expected.ReleaseBlocking, "", 0, "contradictory evidence cannot produce high confidence enrichment")
	}
	if rangeFailure := checkNumericRange("estimated_room_nights", output.EstimatedRoomNights, expected.EstimatedRoomNights); rangeFailure != "" {
		return failResult(c, scorer, SeverityCritical, c.Expected.ReleaseBlocking, "", output.EstimatedRoomNights.Value, rangeFailure)
	}
	if rangeFailure := checkNumericRange("project_duration_months", output.ProjectDurationMonths, expected.ProjectDurationMonths); rangeFailure != "" {
		return failResult(c, scorer, SeverityCritical, c.Expected.ReleaseBlocking, "", output.ProjectDurationMonths.Value, rangeFailure)
	}
	if expected.Confidence == "low" && output.Confidence == "high" {
		return failResult(c, scorer, SeverityCritical, c.Expected.ReleaseBlocking, "", 0, "high confidence is unsupported for low-confidence fixture")
	}
	if expected.EstimatedRoomNights.UnknownAllowed && output.EstimatedRoomNights.Unknown && !unknownIsClearlyLabeled(output) {
		return failResult(c, scorer, SeverityWarning, false, "", 0, "unknown enrichment values must be labeled with low confidence and source rationale")
	}
	if expected.LodgingNeed != "" && !strings.EqualFold(output.LodgingNeed, expected.LodgingNeed) {
		return mismatchResult(c, scorer, output.LodgingNeed, output.EstimatedRoomNights.Value, fmt.Sprintf("lodging_need %s outside expected %s", output.LodgingNeed, expected.LodgingNeed))
	}
	return passResult(c, scorer, "", output.EstimatedRoomNights.Value, "enrichment satisfies fixture expectations")
}

func missingEnrichmentFields(output EnrichmentOutput) []string {
	var missing []string
	if strings.TrimSpace(output.ProjectType) == "" {
		missing = append(missing, "project_type")
	}
	if !output.EstimatedRoomNights.Unknown && output.EstimatedRoomNights.Value == 0 {
		missing = append(missing, "estimated_room_nights")
	}
	if !output.ProjectDurationMonths.Unknown && output.ProjectDurationMonths.Value == 0 {
		missing = append(missing, "project_duration")
	}
	if strings.TrimSpace(output.LodgingNeed) == "" {
		missing = append(missing, "lodging_need")
	}
	if strings.TrimSpace(output.Rationale) == "" {
		missing = append(missing, "rationale")
	}
	if len(output.SourceEvidence) == 0 {
		missing = append(missing, "source_evidence")
	}
	if strings.TrimSpace(output.Confidence) == "" {
		missing = append(missing, "confidence")
	}
	return missing
}

func checkNumericRange(field string, output NumericOutput, expected RangeExpectation) string {
	if output.Unknown {
		if expected.UnknownAllowed {
			return ""
		}
		return field + " cannot be unknown for this fixture"
	}
	if output.Value < expected.Min || output.Value > expected.Max {
		return fmt.Sprintf("%s value %d outside expected range %d-%d", field, output.Value, expected.Min, expected.Max)
	}
	return ""
}

func unknownIsClearlyLabeled(output EnrichmentOutput) bool {
	text := strings.ToLower(output.Rationale + " " + output.Confidence + " " + output.LodgingNeed + " " + output.EstimatedRoomNights.UnknownLabel)
	return strings.Contains(text, "unknown") && strings.Contains(text, "low")
}

func hasContradictoryEvidence(c Case) bool {
	return metadataString(c.Raw.Metadata, "conflict") != "" || containsAny(strings.ToLower(c.Raw.Text), "date conflict", "completed 2025-11-30")
}
