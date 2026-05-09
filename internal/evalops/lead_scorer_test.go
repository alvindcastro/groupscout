package evalops

import (
	"strings"
	"testing"
)

func TestLeadRelevanceScorerScoresCoreLeadCases(t *testing.T) {
	tests := []struct {
		name string
		c    Case
	}{
		{
			name: "high-value commercial permit kept",
			c:    leadCase("lead-commercial", "construction_permit", "richmond_permits", "Airport Logistics Centre Phase 2", "new logistics warehouse. Declared value CAD 8400000. construction schedule 14 months. structural steel crews.", 8_400_000, map[string]any{"project_subtype": "industrial_warehouse", "duration_months": 14}, "keep", ScoreBand{Min: 8, Max: 10}),
		},
		{
			name: "low-value residential renovation dropped",
			c:    leadCase("lead-residential", "construction_permit", "delta_permits", "Single Family Kitchen Renovation", "Residential alteration permit. Declared value CAD 62000. local trades.", 62_000, map[string]any{"project_subtype": "residential_renovation"}, "drop", ScoreBand{Min: 0, Max: 3}),
		},
		{
			name: "film production kept with unknown project value",
			c:    leadCase("lead-film", "film_production", "creativebc", "Coastal Rescue Season 3", "episodic television drama with principal photography 2026-06-03 to 2026-09-04 and regional exterior work around Richmond.", 0, map[string]any{"production_type": "series", "crew_size_hint": 110}, "keep", ScoreBand{Min: 7, Max: 9}),
		},
		{
			name: "consumer event dropped",
			c:    leadCase("lead-consumer-event", "event", "eventbrite", "Neighborhood Board Game Night", "consumer meetup capacity 35. No travel attendees listed.", 0, map[string]any{"capacity": 35}, "drop", ScoreBand{Min: 0, Max: 2}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NewLeadRelevanceScorer().Score(tt.c)
			if result.Status != ResultPass {
				t.Fatalf("Score() status = %s, want pass: %+v", result.Status, result)
			}
		})
	}
}

func TestLeadRelevanceScorerDropsDuplicateRawHash(t *testing.T) {
	scorer := NewLeadRelevanceScorer()
	original := leadCase("lead-original", "construction_permit", "richmond_permits", "Airport Logistics Centre Phase 2", "Declared value CAD 8400000. construction schedule 14 months.", 8_400_000, map[string]any{"expected_duplicate_hash": "fixture-richmond-001"}, "keep", ScoreBand{Min: 8, Max: 10})
	duplicate := leadCase("lead-duplicate", "construction_permit", "richmond_permits", "Airport Logistics Centre Phase 2 revision", "same declared value CAD 8400000, same owner, no expanded scope. Administrative revision only.", 8_400_000, map[string]any{"expected_duplicate_hash": "fixture-richmond-001", "duplicate_of": "lead-original"}, "drop", ScoreBand{Min: 0, Max: 1})

	if result := scorer.Score(original); result.Status != ResultPass {
		t.Fatalf("original Score() status = %s, want pass: %+v", result.Status, result)
	}
	result := scorer.Score(duplicate)
	if result.Status != ResultPass {
		t.Fatalf("duplicate Score() status = %s, want pass: %+v", result.Status, result)
	}
	if result.Decision != "drop" || result.Score > 1 {
		t.Fatalf("duplicate decision/score = %s/%d, want drop <= 1", result.Decision, result.Score)
	}
}

func TestLeadRelevanceScorerMarksMissingSourceEvidenceWarning(t *testing.T) {
	c := leadCase("lead-missing-evidence", "construction_permit", "richmond_permits", "Airport Logistics Centre Phase 2", "new logistics warehouse in Richmond.", 8_400_000, map[string]any{"project_subtype": "industrial_warehouse"}, "keep", ScoreBand{Min: 8, Max: 10})
	c.Expected.Evidence = []EvidenceRequirement{{Claim: "project value", MustSupportWith: "CAD 8400000"}}

	result := NewLeadRelevanceScorer().Score(c)
	if result.Status != ResultFail || result.Severity != SeverityWarning {
		t.Fatalf("Score() = %+v, want warning failure", result)
	}
	if !strings.Contains(result.Message, "source evidence") {
		t.Fatalf("Score() message = %q, want source evidence warning", result.Message)
	}
}

func leadCase(id, category, source, title, text string, value int64, metadata map[string]any, decision string, band ScoreBand) Case {
	var valuePtr *int64
	if value != 0 {
		valuePtr = &value
	}
	return Case{
		ID:        id,
		CaseType:  CaseTypeLead,
		Category:  category,
		RiskLevel: SeverityCritical,
		Source:    Source{System: source, Type: "html", FixtureURL: "https://fixtures.groupscout.local/" + id, CollectedAt: "2026-05-07T00:00:00Z"},
		Raw: RawPayload{
			Title:    title,
			Text:     text,
			IssuedAt: "2026-04-20",
			Location: "Richmond, BC",
			ValueCAD: valuePtr,
			Metadata: metadata,
		},
		Expected: ExpectedOutcome{
			EvalResult:         "pass",
			Decision:           decision,
			ReleaseBlocking:    true,
			SeverityOnMismatch: SeverityCritical,
			ScoreBand:          band,
			Evidence:           []EvidenceRequirement{{Claim: "source facts", MustSupportWith: firstEvidenceNeedle(text)}},
			ForbiddenClaims:    []string{"fabricated hotel relationship"},
			Privacy:            PrivacyExpectation{},
		},
	}
}

func firstEvidenceNeedle(text string) string {
	fields := strings.Split(text, ".")
	return strings.TrimSpace(fields[0])
}
