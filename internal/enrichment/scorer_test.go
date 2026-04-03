package enrichment

import (
	"strings"
	"testing"

	"github.com/alvindcastro/blockscout/internal/collector"
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
