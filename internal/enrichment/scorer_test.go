package enrichment

import (
	"strings"
	"testing"

	"github.com/alvindcastro/groupscout/internal/collector"
)

func TestScorer_Score(t *testing.T) {
	tests := []struct {
		name           string
		project        collector.RawProject
		wantMinScore   int
		wantMaxScore   int
		containsReason string
	}{
		{
			name: "Richmond/YVR location boost",
			project: collector.RawProject{
				Location: "Richmond, V6V 1A1",
				Value:    100_000,
			},
			wantMinScore:   2,
			containsReason: "Richmond/YVR location",
		},
		{
			name: "High value boost",
			project: collector.RawProject{
				Value: 12_000_000,
			},
			wantMinScore:   2,
			containsReason: "Value > $10M",
		},
		{
			name: "Keyword boost",
			project: collector.RawProject{
				Title: "Pipeline upgrade project",
			},
			wantMinScore:   1,
			containsReason: "Keyword: pipeline",
		},
		{
			name: "Deprioritize noise",
			project: collector.RawProject{
				Title: "Single family home renovation",
			},
			wantMaxScore:   -1,
			containsReason: "Deprioritized: single family",
		},
		{
			name: "Combined score",
			project: collector.RawProject{
				Title:    "Civil infrastructure upgrade",
				Location: "Richmond V7C 1X1",
				Value:    15_000_000,
			},
			// Location (+2) + Value (+2) + Keyword civil (+1) + Keyword infrastructure (+1) = 6
			wantMinScore: 6,
		},
	}

	scorer := NewScorer(1)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score, reason := scorer.Score(tt.project)
			if tt.wantMinScore != 0 && score < tt.wantMinScore {
				t.Errorf("Score() = %v, want at least %v", score, tt.wantMinScore)
			}
			if tt.wantMaxScore != 0 && score > tt.wantMaxScore {
				t.Errorf("Score() = %v, want at most %v", score, tt.wantMaxScore)
			}
			if tt.containsReason != "" && !strings.Contains(reason, tt.containsReason) {
				t.Errorf("Reason %q does not contain %q", reason, tt.containsReason)
			}
		})
	}
}

// TestScorer_RealItems uses titles the production pre-scorer skipped with score 0
// before it read descriptions, alongside junk and out-of-region items it must
// keep skipping.
func TestScorer_RealItems(t *testing.T) {
	tests := []struct {
		name    string
		project collector.RawProject
		enrich  bool
	}{
		{"Fraser River Tunnel news", collector.RawProject{Source: "google_news", Title: "$4.15B Fraser River Tunnel Project inches closer to fruition - constructconnect.com"}, true},
		{"Massey Tunnel news", collector.RawProject{Source: "google_news", Title: "$3.3B George Massey Tunnel Replacement Project Near Vancouver Moves Forward"}, true},
		{"YVR expansion news", collector.RawProject{Source: "google_news", Title: "Massive Vancouver airport expansion already in flight - constructconnect.com"}, true},
		{"Roberts Bank news", collector.RawProject{Source: "google_news", Title: "Consortium awarded landmass, wharf contract for Roberts Bank Terminal 2 - constructconnect.com"}, true},
		{"YVR Sea Island announcement", collector.RawProject{Source: "announcements", Title: "Sea Island Climate Resilience Project", Description: "Major project at YVR"}, true},
		{"YVR airfield announcement", collector.RawProject{Source: "announcements", Title: "2026 South Airfield Maintenance and Construction Projects"}, true},
		{"BCIB subway", collector.RawProject{Source: "announcements", Title: "Broadway Subway Project"}, true},
		{"VCC annual meeting", collector.RawProject{Source: "vcc_events", Title: "IEEE IAS Annual Meeting 2026", Description: "Category:  | Date: 4 Oct › 8 Oct"}, true},
		{"CreativeBC feature in description", collector.RawProject{Source: "creativebc", Title: "Salt And Bone", Location: "Burnaby, BC", Description: "Feature Film — Salt Bone Productions Inc | Schedule: 9/8/2026 - 10/30/2026"}, true},
		{"news value in description", collector.RawProject{Source: "google_news", Title: "Port Mann works begin", Description: `<a href="https://example.com">B.C. awards $54.5 million contract</a>`}, true},
		{"Richmond permit with city in location", collector.RawProject{Source: "richmond_permits", Title: "Office — 5900 No 3 Road", Location: "5900 No 3 Road, Richmond BC", Value: 600_000}, true},

		{"BCIB nav heading", collector.RawProject{Source: "announcements", Title: "More"}, false},
		{"BCIB card heading", collector.RawProject{Source: "announcements", Title: "Employee"}, false},
		{"networking event", collector.RawProject{Source: "eventbrite", Title: "Vancouver Healthcare Pharma Business Networking Event"}, false},
		{"Ontario GO station", collector.RawProject{Source: "google_news", Title: "Construction starts on Caledonia GO station - constructconnect.com"}, false},
		{"Edmonton LRT", collector.RawProject{Source: "google_news", Title: "Design-build contract awarded for Edmonton’s Capital Line South LRT expansion"}, false},
		{"Saskatchewan hospital", collector.RawProject{Source: "google_news", Title: "PCL Construction begins site preparation for $900M Sask. hospital tower"}, false},
		{"BC Bid consulting", collector.RawProject{Source: "bcbid", Title: "Islands Trust Accessibility Plan Development (Islands Trust)", Description: "Request for Proposals<br/>Location: Victoria"}, false},
	}

	scorer := NewScorer(1)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score, reason := scorer.Score(tt.project)
			if got := scorer.ShouldEnrich(score); got != tt.enrich {
				t.Errorf("ShouldEnrich = %v (score %d, reason %q), want %v", got, score, reason, tt.enrich)
			}
		})
	}
}

func TestLargestAmount(t *testing.T) {
	tests := []struct {
		text string
		want int64
	}{
		{"$4.15B Fraser River Tunnel", 4_150_000_000},
		{"$967.5M contract awarded", 967_500_000},
		{"new $700-million North Shore wastewater plant", 700_000_000},
		{"receives $54.5 million contract", 54_500_000},
		{"valued at $1,200,000.00", 1_200_000},
		{"$2M and $3.3B", 3_300_000_000},
		{"no amount here", 0},
	}
	for _, tt := range tests {
		if got := largestAmount(tt.text); got != tt.want {
			t.Errorf("largestAmount(%q) = %d, want %d", tt.text, got, tt.want)
		}
	}
}
