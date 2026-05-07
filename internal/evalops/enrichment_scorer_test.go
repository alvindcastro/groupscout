package evalops

import "testing"

func TestScoreEnrichmentCompletenessPassesRequiredFields(t *testing.T) {
	c := leadCase("lead-enrichment-pass", "construction_permit", "richmond_permits", "Airport Logistics Centre Phase 2", "Declared value CAD 8400000. construction schedule 14 months.", 8_400_000, nil, "keep", ScoreBand{Min: 8, Max: 10})
	c.Expected.Enrichment = &EnrichmentExpectation{
		ProjectType:           "industrial construction",
		EstimatedRoomNights:   RangeExpectation{Min: 1200, Max: 4200, UnknownAllowed: false},
		ProjectDurationMonths: RangeExpectation{Min: 10, Max: 18, UnknownAllowed: false},
		LodgingNeed:           "high",
		Confidence:            "medium",
	}
	output := EnrichmentOutput{
		ProjectType:            "industrial construction",
		EstimatedRoomNights:    NumericOutput{Value: 1800},
		ProjectDurationMonths:  NumericOutput{Value: 14},
		LodgingNeed:            "high",
		Confidence:             "medium",
		Rationale:              "CAD 8400000 and 14 month schedule support a high lodging need.",
		SourceEvidence:         []string{"CAD 8400000", "14 months"},
		ReviewedForContradicts: true,
	}

	result := ScoreEnrichmentCompleteness(c, output)
	if result.Status != ResultPass {
		t.Fatalf("ScoreEnrichmentCompleteness() = %+v, want pass", result)
	}
}

func TestScoreEnrichmentCompletenessFailsMissingRequiredFields(t *testing.T) {
	c := leadCase("lead-enrichment-missing", "construction_permit", "richmond_permits", "Airport Logistics Centre Phase 2", "Declared value CAD 8400000. construction schedule 14 months.", 8_400_000, nil, "keep", ScoreBand{Min: 8, Max: 10})
	c.Expected.Enrichment = &EnrichmentExpectation{EstimatedRoomNights: RangeExpectation{Min: 1200, Max: 4200}}

	result := ScoreEnrichmentCompleteness(c, EnrichmentOutput{})
	if result.Status != ResultFail || result.Severity != SeverityCritical {
		t.Fatalf("ScoreEnrichmentCompleteness() = %+v, want critical missing-field failure", result)
	}
}

func TestScoreEnrichmentCompletenessFailsUnsupportedRoomNightCertainty(t *testing.T) {
	c := leadCase("lead-enrichment-unsupported", "construction_permit", "richmond_permits", "Hospitality Training Centre Addition", "Permit omits declared value. ten month phased construction schedule.", 0, map[string]any{"value_missing": true}, "needs_review", ScoreBand{Min: 5, Max: 8})
	c.Expected.Enrichment = &EnrichmentExpectation{
		EstimatedRoomNights:   RangeExpectation{Min: 0, Max: 1800, UnknownAllowed: true},
		ProjectDurationMonths: RangeExpectation{Min: 8, Max: 12, UnknownAllowed: false},
		LodgingNeed:           "possible",
		Confidence:            "low",
	}
	output := EnrichmentOutput{
		ProjectType:           "commercial addition",
		EstimatedRoomNights:   NumericOutput{Value: 4000},
		ProjectDurationMonths: NumericOutput{Value: 10},
		LodgingNeed:           "high",
		Confidence:            "high",
		Rationale:             "Large project will need rooms.",
		SourceEvidence:        []string{"ten month schedule"},
	}

	result := ScoreEnrichmentCompleteness(c, output)
	if result.Status != ResultFail || result.Severity != SeverityCritical {
		t.Fatalf("ScoreEnrichmentCompleteness() = %+v, want critical unsupported certainty", result)
	}
}

func TestScoreEnrichmentCompletenessRequiresExplicitUnknownLabels(t *testing.T) {
	c := leadCase("lead-enrichment-unknown", "construction_permit", "delta_permits", "Ladner Civic Works Yard Expansion", "contractor unreadable and address partly missing.", 3_200_000, nil, "needs_review", ScoreBand{Min: 4, Max: 7})
	c.Expected.Enrichment = &EnrichmentExpectation{
		EstimatedRoomNights:   RangeExpectation{Min: 200, Max: 1400, UnknownAllowed: true},
		ProjectDurationMonths: RangeExpectation{Min: 6, Max: 12, UnknownAllowed: true},
		LodgingNeed:           "unknown",
		Confidence:            "low",
	}
	output := EnrichmentOutput{
		ProjectType:           "municipal construction",
		EstimatedRoomNights:   NumericOutput{Unknown: true},
		ProjectDurationMonths: NumericOutput{Unknown: true},
		LodgingNeed:           "unknown",
		Confidence:            "medium",
		Rationale:             "The project is probably relevant.",
		SourceEvidence:        []string{"contractor unreadable"},
	}

	result := ScoreEnrichmentCompleteness(c, output)
	if result.Status != ResultFail || result.Severity != SeverityWarning {
		t.Fatalf("ScoreEnrichmentCompleteness() = %+v, want warning for unclear unknown handling", result)
	}
}

func TestScoreEnrichmentCompletenessFailsContradictoryEvidenceWithHighConfidence(t *testing.T) {
	c := leadCase("lead-enrichment-conflict", "procurement_bid", "bcbid", "Synthetic Marine Terminal Roadworks Award", "construction starts 2026-06-01 and runs 9 months. Attachment summary says work completed 2025-11-30.", 9_300_000, map[string]any{"conflict": "start date after completed date"}, "needs_review", ScoreBand{Min: 4, Max: 7})
	c.Expected.Enrichment = &EnrichmentExpectation{
		EstimatedRoomNights:   RangeExpectation{Min: 0, Max: 2400, UnknownAllowed: true},
		ProjectDurationMonths: RangeExpectation{Min: 0, Max: 9, UnknownAllowed: true},
		LodgingNeed:           "possible",
		Confidence:            "low",
	}
	output := EnrichmentOutput{
		ProjectType:           "roadworks infrastructure",
		EstimatedRoomNights:   NumericOutput{Value: 1200},
		ProjectDurationMonths: NumericOutput{Value: 9},
		LodgingNeed:           "high",
		Confidence:            "high",
		Rationale:             "Confirmed active construction window.",
		SourceEvidence:        []string{"starts 2026-06-01"},
	}

	result := ScoreEnrichmentCompleteness(c, output)
	if result.Status != ResultFail || result.Severity != SeverityCritical {
		t.Fatalf("ScoreEnrichmentCompleteness() = %+v, want critical contradictory evidence failure", result)
	}
}
