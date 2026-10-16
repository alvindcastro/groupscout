package enrichment

import (
	"fmt"
	"html"
	"regexp"
	"strconv"
	"strings"

	"github.com/alvindcastro/groupscout/internal/collector"
)

var (
	// Richmond/YVR and the places whose crews lodge there: Richmond postal codes
	// (V6V, V6X, V6Y, V7A, V7B, V7C, V7E), Sea Island/YVR, and the Richmond-Delta
	// river crossings and port lands across the Fraser.
	richmondPC = regexp.MustCompile(`(?i)\b(V[67][VBCEYX]|Richmond|YVR|Sea Island|Vancouver (International )?Airport|Steveston|Massey|Fraser River Tunnel|Roberts Bank|Tsawwassen|Ladner|Delta,? BC)\b`)

	// inRegionRe marks text as being about British Columbia.
	inRegionRe = regexp.MustCompile(`(?i)\b(B\.C|BC|British Columbia|Vancouver|Richmond|Burnaby|Surrey|Delta|Lower Mainland)\b`)

	// outOfRegionRe marks text as being about somewhere a Richmond hotel cannot serve.
	// Only applied when inRegionRe does not match, so national round-ups that also
	// mention B.C. projects still reach enrichment.
	outOfRegionRe = regexp.MustCompile(`(?i)\b(Ontario|Ont|Alberta|Alta|Quebec|Saskatchewan|Sask|Manitoba|Nova Scotia|New Brunswick|Newfoundland|Toronto|Ottawa|Edmonton|Calgary|Montreal|Winnipeg|Regina|Saskatoon|Halifax|Metrolinx|GO (Rail|station|Transit|corridor|extension))\b`)

	// Keywords that indicate high-value crew lodging potential (+1 each).
	// Matched on word boundaries with an optional plural, so "port" does not
	// match "report" but "productions" still matches "production".
	highValueKeywords = []string{
		"pipeline", "civil", "electrical", "steel", "concrete",
		"infrastructure", "sewer", "watermain", "substation",
		"bridge", "paving", "excavation", "demolition",
		"manufacturing", "warehouse", "logistics", "industrial",
		"construction", "contract awarded", "tunnel", "highway", "interchange", "overpass",
		"airport", "airfield", "runway", "terminal", "port", "wharf",
		"transit", "skytrain", "subway", "railway", "lrt",
		"hospital", "wastewater", "treatment plant", "reservoir", "data centre", "data center",
		"expansion", "replacement", "redevelopment",
		"feature film", "tv series", "tv movie", "tv pilot", "production", "season",
		"conference", "summit", "congress", "trade show", "convention", "symposium",
		"expo", "annual meeting",
	}
	highValueKeywordRes = compileKeywords(highValueKeywords)

	// Keywords that deprioritize (-5 to effectively skip). Matched against title
	// and location only: descriptions often mention residential or condo work in
	// passing without the project itself being one.
	lowValueKeywords = []string{
		"interior renovation", "residential", "landscaping",
		"tenant improvement", "single family", "duplex",
		"townhouse", "condo", "signage", "swimming pool",
	}

	// moneyRe matches dollar amounts written in prose: "$967.5M", "$4.15B",
	// "$700-million", "$54.5 million", "$1,200,000".
	moneyRe = regexp.MustCompile(`(?i)\$\s?(\d{1,3}(?:,\d{3})+|\d+(?:\.\d+)?)(?:\s?-?\s?(billion|bn|b|million|mil|m)\b)?`)

	htmlTagRe = regexp.MustCompile(`<[^>]*>`)
)

func compileKeywords(keywords []string) []*regexp.Regexp {
	res := make([]*regexp.Regexp, len(keywords))
	for i, kw := range keywords {
		res[i] = regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(kw) + `(s|es)?\b`)
	}
	return res
}

// Scorer implements rule-based pre-scoring of raw projects.
type Scorer struct {
	Threshold int
}

// NewScorer returns a Scorer with the given threshold.
func NewScorer(threshold int) *Scorer {
	return &Scorer{Threshold: threshold}
}

// Score calculates a priority score for a RawProject.
// Title, location and description are all scored: news, event and tender
// collectors carry most of their signal in the description, and Location and
// Value are often empty for them.
func (s *Scorer) Score(p collector.RawProject) (int, string) {
	var score int
	var reasons []string

	text := p.Title + " " + p.Location + " " + plainText(p.Description)

	// 1. Geography: Richmond/YVR (+2)
	if richmondPC.MatchString(text) {
		score += 2
		reasons = append(reasons, "Richmond/YVR location")
	} else if outOfRegionRe.MatchString(text) && !inRegionRe.MatchString(text) {
		score -= 5
		reasons = append(reasons, "Deprioritized: outside B.C.")
	}

	// 2. Scale: Project value > $10M (+2). Collectors without a structured
	// value (news, announcements) often state it in the text instead.
	value := p.Value
	valueSource := ""
	if value == 0 {
		value = largestAmount(p.Title + " " + plainText(p.Description))
		valueSource = " (from text)"
	}
	if value >= 10_000_000 {
		score += 2
		reasons = append(reasons, "Value > $10M"+valueSource)
	} else if value >= 1_000_000 {
		score += 1
		reasons = append(reasons, "Value > $1M"+valueSource)
	}

	// 3. Keywords: High-value crew signals (+1 each)
	for i, re := range highValueKeywordRes {
		if re.MatchString(text) {
			score += 1
			reasons = append(reasons, fmt.Sprintf("Keyword: %s", highValueKeywords[i]))
		}
	}

	// 4. Keywords: Low-value/noise signals (-5)
	titleText := strings.ToLower(p.Title + " " + p.Location)
	for _, kw := range lowValueKeywords {
		if strings.Contains(titleText, kw) {
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

// plainText strips HTML tags and entities from RSS and scraped descriptions.
func plainText(s string) string {
	return html.UnescapeString(htmlTagRe.ReplaceAllString(s, " "))
}

// largestAmount returns the largest dollar amount stated in text, in dollars.
func largestAmount(text string) int64 {
	var largest float64
	for _, m := range moneyRe.FindAllStringSubmatch(text, -1) {
		n, err := strconv.ParseFloat(strings.ReplaceAll(m[1], ",", ""), 64)
		if err != nil {
			continue
		}
		switch strings.ToLower(m[2]) {
		case "billion", "bn", "b":
			n *= 1_000_000_000
		case "million", "mil", "m":
			n *= 1_000_000
		}
		if n > largest {
			largest = n
		}
	}
	return int64(largest)
}
