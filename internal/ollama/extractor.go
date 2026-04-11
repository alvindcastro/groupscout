package ollama

import (
	"context"
	"encoding/json"
	"log"
	"strings"
	"time"
)

// LeadSignal represents structured data extracted from raw text.
type LeadSignal struct {
	OrgName      string    `json:"org_name"`
	Location     string    `json:"location"`
	StartDate    time.Time `json:"start_date"`
	DurationDays int       `json:"duration_days"`
	CrewSize     int       `json:"crew_size"`
	ProjectType  string    `json:"project_type"`
	Confidence   float64   `json:"confidence"`
}

// Extractor uses an LLMClient to extract signals from raw text.
type Extractor struct {
	client LLMClient
}

// NewExtractor returns a new Extractor.
func NewExtractor(client LLMClient) *Extractor {
	return &Extractor{client: client}
}

// Extract takes raw text and returns an extracted LeadSignal.
func (e *Extractor) Extract(ctx context.Context, rawText string) (*LeadSignal, error) {
	ctx = WithUseCase(ctx, "extraction")
	systemPrompt := `
You are a data extraction assistant for a hotel group sales intelligence system.
You receive raw text from building permit filings, government contract awards,
film production permits, and press releases — all from the Vancouver, BC metro area.

Your job is to extract structured information and return ONLY valid JSON.
Do not include any explanation, preamble, or markdown formatting.
If a field cannot be determined from the text, set it to null.

Output this exact JSON schema:
{
  "org_name": "string or null",
  "location": "string or null — street address or neighbourhood",
  "start_date": "YYYY-MM-DD or null",
  "duration_days": integer or null,
  "crew_size": integer or null,
  "project_type": "one of: construction | film_production | government_contract | conference | sports | other",
  "confidence": float between 0 and 1 — your confidence in the overall extraction
}
`

	resp, err := e.client.ChatComplete(ctx, systemPrompt, rawText)
	if err != nil {
		// Fallback: log error and return nil signal so pipeline can continue
		return nil, nil
	}

	// Strip markdown fences
	cleanJSON := stripJSONFences(resp)

	// Unmarshal into a temporary map to handle nulls and types gracefully
	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(cleanJSON), &raw); err != nil {
		// Fallback: malformed JSON, return nil signal
		return nil, nil
	}

	signal := &LeadSignal{}
	if v, ok := raw["org_name"].(string); ok {
		signal.OrgName = v
	}
	if v, ok := raw["location"].(string); ok {
		signal.Location = v
	}
	if v, ok := raw["start_date"].(string); ok && v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			signal.StartDate = t
		}
	}
	if v, ok := raw["duration_days"].(float64); ok {
		signal.DurationDays = int(v)
	}
	if v, ok := raw["crew_size"].(float64); ok {
		signal.CrewSize = int(v)
	}
	if v, ok := raw["project_type"].(string); ok {
		signal.ProjectType = v
	}
	if v, ok := raw["confidence"].(float64); ok {
		signal.Confidence = v
	}

	// Log field hit rate
	hitCount := 0
	fields := []interface{}{signal.OrgName, signal.Location, signal.StartDate, signal.DurationDays, signal.CrewSize, signal.ProjectType}
	for _, f := range fields {
		switch v := f.(type) {
		case string:
			if v != "" {
				hitCount++
			}
		case int:
			if v != 0 {
				hitCount++
			}
		case time.Time:
			if !v.IsZero() {
				hitCount++
			}
		}
	}
	log.Printf("Ollama extraction hit rate: %d/6", hitCount)

	return signal, nil
}

func stripJSONFences(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```json") {
		s = strings.TrimPrefix(s, "```json")
		s = strings.TrimSuffix(s, "```")
	} else if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```")
		s = strings.TrimSuffix(s, "```")
	}
	return strings.TrimSpace(s)
}
