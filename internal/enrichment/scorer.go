package enrichment

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/alvindcastro/blockscout/internal/collector"
)

var (
	// Richmond/YVR postal code pattern (e.g. V6V, V6X, V6Y, V7B, V7C, V7E)
	richmondPC = regexp.MustCompile(`(?i)\b(V[67][VBCEYX]|Richmond|YVR)\b`)

	// Keywords that indicate high-value crew lodging potential (+1 each)
	highValueKeywords = []string{
		"pipeline", "civil", "electrical", "steel", "concrete",
		"infrastructure", "sewer", "watermain", "substation",
		"bridge", "paving", "excavation", "demolition",
		"manufacturing", "warehouse", "logistics", "industrial",
		"feature film", "tv series", "production", "season",
		"conference", "summit", "congress", "trade show", "convention", "symposium",
	}

	// Keywords that deprioritize (-5 to effectively skip)
	lowValueKeywords = []string{
		"interior renovation", "residential", "landscaping",
		"tenant improvement", "single family", "duplex",
		"townhouse", "condo", "signage", "swimming pool",
	}
)

// Scorer implements rule-based pre-scoring of raw projects.
type Scorer struct {
	Threshold int
}

// NewScorer returns a Scorer with the given threshold.
func NewScorer(threshold int) *Scorer {
	return &Scorer{Threshold: threshold}
}

// Score calculates a priority score for a RawProject.
func (s *Scorer) Score(p collector.RawProject) (int, string) {
	var score int
	var reasons []string

	// 1. Geography: Richmond/YVR postal codes (+2)
	if richmondPC.MatchString(p.Location) || richmondPC.MatchString(p.Title) {
		score += 2
		reasons = append(reasons, "Richmond/YVR location")
	}

	// 2. Scale: Project value > $10M (+2)
	if p.Value >= 10_000_000 {
		score += 2
		reasons = append(reasons, "Value > $10M")
	} else if p.Value >= 1_000_000 {
		score += 1
		reasons = append(reasons, "Value > $1M")
	}

	// 3. Keywords: High-value crew signals (+1 each)
	text := strings.ToLower(p.Title + " " + p.Location)
	for _, kw := range highValueKeywords {
		if strings.Contains(text, kw) {
			score += 1
			reasons = append(reasons, fmt.Sprintf("Keyword: %s", kw))
		}
	}

	// 4. Keywords: Low-value/noise signals (-5)
	for _, kw := range lowValueKeywords {
		if strings.Contains(text, kw) {
			score -= 5
			reasons = append(reasons, fmt.Sprintf("Deprioritized: %s", kw))
		}
	}

	reason := strings.Join(reasons, ", ")
	return score, reason
}

// ShouldEnrich returns true if the project's score meets the threshold.
func (s *Scorer) ShouldEnrich(score int) bool {
	return score >= s.Threshold
}
