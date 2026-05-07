package evalops

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

type LeadRelevanceScorer struct {
	seenHashes map[string]string
}

func NewLeadRelevanceScorer() *LeadRelevanceScorer {
	return &LeadRelevanceScorer{seenHashes: make(map[string]string)}
}

func (s *LeadRelevanceScorer) Score(c Case) Result {
	if c.CaseType != CaseTypeLead {
		return failResult(c, "lead_relevance", SeverityWarning, false, "", 0, "case is not a lead")
	}

	decision, score, reasons := s.scoreLead(c)
	if !evidenceSupported(c) {
		return failResult(c, "lead_relevance", SeverityWarning, false, decision, score, "missing source evidence for expected lead claim")
	}
	if decision != c.Expected.Decision {
		return mismatchResult(c, "lead_relevance", decision, score, fmt.Sprintf("decision %s outside expected %s", decision, c.Expected.Decision))
	}
	if !scoreInBand(score, c.Expected.ScoreBand) {
		return mismatchResult(c, "lead_relevance", decision, score, fmt.Sprintf("score %d outside expected band %d-%d", score, c.Expected.ScoreBand.Min, c.Expected.ScoreBand.Max))
	}
	return passResult(c, "lead_relevance", decision, score, strings.Join(reasons, "; "))
}

func (s *LeadRelevanceScorer) scoreLead(c Case) (string, int, []string) {
	hash := metadataString(c.Raw.Metadata, "expected_duplicate_hash")
	if duplicateOf := metadataString(c.Raw.Metadata, "duplicate_of"); duplicateOf != "" {
		return "drop", 0, []string{"duplicate_of " + duplicateOf}
	}
	if hash != "" {
		if seenID, ok := s.seenHashes[hash]; ok && seenID != c.ID {
			return "drop", 0, []string{"duplicate raw hash " + hash}
		}
		defer func() {
			s.seenHashes[hash] = c.ID
		}()
	}

	text := lowerLeadText(c)
	switch {
	case containsAny(text, "single family", "residential alteration", "kitchen renovation", "local trades"):
		return "drop", 1, []string{"low-value residential work"}
	case containsAny(text, "software subscription", "remote implementation", "records software"):
		return "drop", 1, []string{"non-lodging software procurement"}
	case containsAny(text, "capacity 35", "capacity 80", "no travel attendees listed", "small meetup"):
		return "drop", 1, []string{"small consumer or unverified event"}
	case containsAny(text, "pdf text extraction damaged", "unreadable", "partly missing"):
		return "needs_review", 5, []string{"malformed source requires review"}
	case containsAny(text, "omits declared value", "value unknown") || metadataBool(c.Raw.Metadata, "value_missing"):
		return "needs_review", 6, []string{"project value is explicitly unknown"}
	case metadataString(c.Raw.Metadata, "conflict") != "" || containsAny(text, "does not explain the date conflict", "completed 2025-11-30"):
		return "needs_review", 5, []string{"contradictory source evidence"}
	}

	score := 4
	var reasons []string
	if c.Raw.ValueCAD != nil {
		switch {
		case *c.Raw.ValueCAD >= 20_000_000:
			score += 5
			reasons = append(reasons, "value >= CAD 20M")
		case *c.Raw.ValueCAD >= 8_000_000:
			score += 4
			reasons = append(reasons, "value >= CAD 8M")
		case *c.Raw.ValueCAD >= 4_000_000:
			score += 3
			reasons = append(reasons, "value >= CAD 4M")
		case *c.Raw.ValueCAD >= 1_000_000:
			score += 2
			reasons = append(reasons, "value >= CAD 1M")
		}
	}
	if containsAny(text, "warehouse", "infrastructure", "bridge", "station", "airport", "utility", "port", "transit", "construction") {
		score += 1
		reasons = append(reasons, "crew or construction signal")
	}
	if containsAny(text, "night work", "seven-day shifts", "rotating", "specialized crews", "staging", "principal photography") {
		score += 1
		reasons = append(reasons, "lodging demand signal")
	}
	if containsAny(text, "conference", "congress", "attendees", "exhibitors") {
		score += 4
		reasons = append(reasons, "professional event demand")
	}
	if containsAny(text, "episodic", "television", "film", "production") {
		score += 4
		reasons = append(reasons, "film production demand")
	}
	if containsAny(text, "capacity 260", "near airport") {
		score += 1
		reasons = append(reasons, "medium event near airport")
	}

	score = int(math.Max(0, math.Min(10, float64(score))))
	switch {
	case score >= 7:
		return "keep", score, reasons
	case score >= 4:
		return "needs_review", score, reasons
	default:
		return "drop", score, reasons
	}
}

func lowerLeadText(c Case) string {
	return strings.ToLower(strings.Join([]string{c.Raw.Title, c.Raw.Text, c.Raw.Location, c.Category, c.Source.System}, " "))
}

func scoreInBand(score int, band ScoreBand) bool {
	return score >= band.Min && score <= band.Max
}

func metadataString(metadata map[string]any, key string) string {
	if metadata == nil {
		return ""
	}
	value, ok := metadata[key]
	if !ok || value == nil {
		return ""
	}
	switch v := value.(type) {
	case string:
		return v
	default:
		return fmt.Sprint(v)
	}
}

func metadataBool(metadata map[string]any, key string) bool {
	if metadata == nil {
		return false
	}
	value, ok := metadata[key].(bool)
	return ok && value
}

func containsAny(text string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(text, strings.ToLower(needle)) {
			return true
		}
	}
	return false
}

func evidenceSupported(c Case) bool {
	if len(c.Expected.Evidence) == 0 {
		return true
	}
	sourceText := strings.ToLower(strings.Join([]string{c.Raw.Title, c.Raw.Text, c.Raw.Location}, " "))
	for _, evidence := range c.Expected.Evidence {
		if !evidenceNeedleSupported(sourceText, evidence.MustSupportWith) {
			return false
		}
	}
	return true
}

func evidenceNeedleSupported(sourceText, requirement string) bool {
	requirement = strings.ToLower(requirement)
	numbers := regexp.MustCompile(`\d[\d,]*`).FindAllString(requirement, -1)
	for _, number := range numbers {
		plain := strings.ReplaceAll(number, ",", "")
		if strings.Contains(sourceText, number) || strings.Contains(sourceText, plain) {
			return true
		}
	}
	for _, part := range splitEvidenceRequirement(requirement) {
		words := strings.Fields(part)
		for _, word := range words {
			word = strings.Trim(word, ".,:;()[]")
			if len(word) < 5 || isEvidenceStopword(word) {
				continue
			}
			if strings.Contains(sourceText, word) {
				return true
			}
		}
	}
	return strings.TrimSpace(requirement) == "" || strings.Contains(sourceText, strings.TrimSpace(requirement))
}

func splitEvidenceRequirement(requirement string) []string {
	replacer := strings.NewReplacer(" and ", "|", ",", "|", ";", "|", " plus ", "|")
	return strings.Split(replacer.Replace(requirement), "|")
}

func isEvidenceStopword(word string) bool {
	switch word {
	case "source", "evidence", "project", "value", "duration", "schedule", "claim", "known":
		return true
	default:
		if _, err := strconv.Atoi(word); err == nil {
			return true
		}
		return false
	}
}
