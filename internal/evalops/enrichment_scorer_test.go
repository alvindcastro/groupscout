package evalops

import (
	"testing"
)

func makeEnrichmentCase(id string, enrichment *ExpectedEnrichment) Case {
	return Case{
		ID:       id,
		CaseType: "lead",
		Expected: CaseExpected{
			Decision:   "keep",
			Enrichment: enrichment,
		},
	}
}

func defaultExpectedEnrichment() *ExpectedEnrichment {
	return &ExpectedEnrichment{
		ProjectType:           "industrial_construction",
		EstimatedRoomNights:   RoomNightRange{Min: 50, Max: 1000, UnknownAllowed: true},
		ProjectDurationMonths: DurationRange{Min: 12, Max: 30, UnknownAllowed: false},
		LodgingNeed:           "high",
		Confidence:            "medium",
	}
}

func defaultEnrichedLead() EnrichedLeadInput {
	return EnrichedLeadInput{
		ProjectType:           "industrial_construction",
		EstimatedRoomNights:   200,
		RoomNightsUnknown:     false,
		ProjectDurationMonths: 18,
		DurationUnknown:       false,
		LodgingNeed:           "high",
		Rationale:             "Out-of-town crews expected for 18 months based on project scope.",
		SourceEvidence:        "Permit text states 24 months, crew signals present.",
		Confidence:            "medium",
	}
}

func TestScoreEnrichment_AllRequiredFieldsPresent(t *testing.T) {
	c := makeEnrichmentCase("enrich-ok-001", defaultExpectedEnrichment())
	enriched := defaultEnrichedLead()

	result := ScoreEnrichment(c, enriched)

	if result.IsCriticalFailure {
		t.Errorf("expected no critical failure for complete enrichment, findings: %+v", result.Findings)
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected no findings for valid enrichment, got: %+v", result.Findings)
	}
}

func TestScoreEnrichment_MissingProjectType(t *testing.T) {
	c := makeEnrichmentCase("enrich-missing-type", defaultExpectedEnrichment())
	enriched := defaultEnrichedLead()
	enriched.ProjectType = ""

	result := ScoreEnrichment(c, enriched)

	if !result.IsCriticalFailure {
		t.Error("expected critical failure for missing project_type")
	}
	if !hasFindingCode(result.Findings, "ENRICH_MISSING_PROJECT_TYPE") {
		t.Errorf("expected ENRICH_MISSING_PROJECT_TYPE finding, got: %+v", result.Findings)
	}
}

func TestScoreEnrichment_MissingLodgingNeed(t *testing.T) {
	c := makeEnrichmentCase("enrich-missing-lodging", defaultExpectedEnrichment())
	enriched := defaultEnrichedLead()
	enriched.LodgingNeed = ""

	result := ScoreEnrichment(c, enriched)

	if !result.IsCriticalFailure {
		t.Error("expected critical failure for missing lodging_need")
	}
	if !hasFindingCode(result.Findings, "ENRICH_MISSING_LODGING_NEED") {
		t.Errorf("expected ENRICH_MISSING_LODGING_NEED finding, got: %+v", result.Findings)
	}
}

func TestScoreEnrichment_MissingRationale(t *testing.T) {
	c := makeEnrichmentCase("enrich-missing-rationale", defaultExpectedEnrichment())
	enriched := defaultEnrichedLead()
	enriched.Rationale = ""

	result := ScoreEnrichment(c, enriched)

	if !result.IsCriticalFailure {
		t.Error("expected critical failure for missing rationale")
	}
	if !hasFindingCode(result.Findings, "ENRICH_MISSING_RATIONALE") {
		t.Errorf("expected ENRICH_MISSING_RATIONALE finding, got: %+v", result.Findings)
	}
}

func TestScoreEnrichment_MissingSourceEvidence(t *testing.T) {
	c := makeEnrichmentCase("enrich-missing-evidence", defaultExpectedEnrichment())
	enriched := defaultEnrichedLead()
	enriched.SourceEvidence = ""

	result := ScoreEnrichment(c, enriched)

	if !result.IsCriticalFailure {
		t.Error("expected critical failure for missing source_evidence")
	}
	if !hasFindingCode(result.Findings, "ENRICH_MISSING_SOURCE_EVIDENCE") {
		t.Errorf("expected ENRICH_MISSING_SOURCE_EVIDENCE finding, got: %+v", result.Findings)
	}
}

func TestScoreEnrichment_MissingConfidence(t *testing.T) {
	c := makeEnrichmentCase("enrich-missing-confidence", defaultExpectedEnrichment())
	enriched := defaultEnrichedLead()
	enriched.Confidence = ""

	result := ScoreEnrichment(c, enriched)

	if !result.IsCriticalFailure {
		t.Error("expected critical failure for missing confidence")
	}
	if !hasFindingCode(result.Findings, "ENRICH_MISSING_CONFIDENCE") {
		t.Errorf("expected ENRICH_MISSING_CONFIDENCE finding, got: %+v", result.Findings)
	}
}

func TestScoreEnrichment_UnsupportedHighConfidence_Critical(t *testing.T) {
	// Case where unknown_allowed=true means evidence is likely insufficient
	exp := defaultExpectedEnrichment()
	exp.EstimatedRoomNights.UnknownAllowed = true
	c := makeEnrichmentCase("enrich-high-conf", exp)

	enriched := defaultEnrichedLead()
	enriched.Confidence = "high" // claiming high confidence when evidence is insufficient

	result := ScoreEnrichment(c, enriched)

	if !result.IsCriticalFailure {
		t.Error("expected critical failure for confident unsupported room-night estimate")
	}
	if !hasFindingCode(result.Findings, "ENRICH_UNSUPPORTED_HIGH_CONFIDENCE") {
		t.Errorf("expected ENRICH_UNSUPPORTED_HIGH_CONFIDENCE finding, got: %+v", result.Findings)
	}
}

func TestScoreEnrichment_UnknownRoomNightsAllowed(t *testing.T) {
	exp := defaultExpectedEnrichment()
	exp.EstimatedRoomNights.UnknownAllowed = true
	c := makeEnrichmentCase("enrich-unknown-rn", exp)

	enriched := defaultEnrichedLead()
	enriched.RoomNightsUnknown = true
	enriched.EstimatedRoomNights = 0
	enriched.Confidence = "low" // not high, so no unsupported claim

	result := ScoreEnrichment(c, enriched)

	if result.IsCriticalFailure {
		t.Errorf("expected no critical failure when unknown_allowed=true and room_nights_unknown=true, findings: %+v", result.Findings)
	}
}

func TestScoreEnrichment_UnknownRoomNightsNotAllowed(t *testing.T) {
	exp := defaultExpectedEnrichment()
	exp.EstimatedRoomNights.UnknownAllowed = false
	c := makeEnrichmentCase("enrich-unknown-not-allowed", exp)

	enriched := defaultEnrichedLead()
	enriched.RoomNightsUnknown = true

	result := ScoreEnrichment(c, enriched)

	if !result.IsCriticalFailure {
		t.Error("expected critical failure when unknown_allowed=false but room_nights_unknown=true")
	}
	if !hasFindingCode(result.Findings, "ENRICH_ROOM_NIGHTS_UNKNOWN_NOT_ALLOWED") {
		t.Errorf("expected ENRICH_ROOM_NIGHTS_UNKNOWN_NOT_ALLOWED finding, got: %+v", result.Findings)
	}
}

func TestScoreEnrichment_DurationUnknownNotAllowed(t *testing.T) {
	exp := defaultExpectedEnrichment()
	exp.ProjectDurationMonths.UnknownAllowed = false
	c := makeEnrichmentCase("enrich-dur-unknown", exp)

	enriched := defaultEnrichedLead()
	enriched.DurationUnknown = true

	result := ScoreEnrichment(c, enriched)

	if !result.IsCriticalFailure {
		t.Error("expected critical failure when duration unknown not allowed")
	}
	if !hasFindingCode(result.Findings, "ENRICH_DURATION_UNKNOWN_NOT_ALLOWED") {
		t.Errorf("expected ENRICH_DURATION_UNKNOWN_NOT_ALLOWED finding, got: %+v", result.Findings)
	}
}

func TestScoreEnrichment_RoomNightsOutOfRange(t *testing.T) {
	exp := defaultExpectedEnrichment()
	exp.EstimatedRoomNights = RoomNightRange{Min: 100, Max: 500, UnknownAllowed: false}
	c := makeEnrichmentCase("enrich-rn-range", exp)

	enriched := defaultEnrichedLead()
	enriched.EstimatedRoomNights = 2000 // way above max
	enriched.Confidence = "low"

	result := ScoreEnrichment(c, enriched)

	if !hasFindingCode(result.Findings, "ENRICH_ROOM_NIGHTS_OUT_OF_RANGE") {
		t.Errorf("expected ENRICH_ROOM_NIGHTS_OUT_OF_RANGE finding, got: %+v", result.Findings)
	}
}

func TestScoreEnrichment_ContradictoryEvidence(t *testing.T) {
	c := makeEnrichmentCase("enrich-contradiction", defaultExpectedEnrichment())

	enriched := defaultEnrichedLead()
	enriched.Rationale = "Confirmed 200 room nights based on crew signals."
	enriched.SourceEvidence = "Evidence is unknown due to limited data."
	enriched.Confidence = "high"

	result := ScoreEnrichment(c, enriched)

	if !hasFindingCode(result.Findings, "ENRICH_CONTRADICTORY_EVIDENCE") {
		t.Errorf("expected ENRICH_CONTRADICTORY_EVIDENCE finding for contradictory rationale/evidence, got: %+v", result.Findings)
	}
}

func TestScoreEnrichment_AlertCaseSkipped(t *testing.T) {
	c := Case{
		ID:       "alert-skip-001",
		CaseType: "alert",
		Expected: CaseExpected{
			Decision:   "hard_alert",
			Enrichment: nil,
		},
	}

	result := ScoreEnrichment(c, EnrichedLeadInput{})

	if result.IsCriticalFailure {
		t.Errorf("expected no critical failure for alert case (no enrichment expected), findings: %+v", result.Findings)
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected no findings for alert case, got: %+v", result.Findings)
	}
}

func TestScoreEnrichment_LodgingNeedMismatch(t *testing.T) {
	exp := defaultExpectedEnrichment()
	exp.LodgingNeed = "high"
	c := makeEnrichmentCase("enrich-lodging-mismatch", exp)

	enriched := defaultEnrichedLead()
	enriched.LodgingNeed = "low" // mismatch with expected high

	result := ScoreEnrichment(c, enriched)

	if !hasFindingCode(result.Findings, "ENRICH_LODGING_NEED_MISMATCH") {
		t.Errorf("expected ENRICH_LODGING_NEED_MISMATCH finding, got: %+v", result.Findings)
	}
}

func TestScoreEnrichment_InvalidConfidenceValue(t *testing.T) {
	c := makeEnrichmentCase("enrich-invalid-conf", defaultExpectedEnrichment())
	enriched := defaultEnrichedLead()
	enriched.Confidence = "very_high" // not a recognized value

	result := ScoreEnrichment(c, enriched)

	if !hasFindingCode(result.Findings, "ENRICH_INVALID_CONFIDENCE") {
		t.Errorf("expected ENRICH_INVALID_CONFIDENCE finding for unrecognized confidence value, got: %+v", result.Findings)
	}
}

func hasFindingCode(findings []Finding, code string) bool {
	for _, f := range findings {
		if f.Code == code {
			return true
		}
	}
	return false
}
