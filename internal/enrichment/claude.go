package enrichment

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/alvindcastro/blockscout/internal/collector"
)

const (
	claudeAPIURL     = "https://api.anthropic.com/v1/messages"
	claudeAPIVersion = "2023-06-01"
	defaultModel     = "claude-haiku-4-5-20251001"
)

// EnrichedLead is the structured output from the Claude enrichment step.
// Field names match the JSON keys Claude is prompted to return.
type EnrichedLead struct {
	GeneralContractor       string `json:"general_contractor"`
	ProjectType             string `json:"project_type"`
	EstimatedCrewSize       int    `json:"estimated_crew_size"`
	EstimatedDurationMonths int    `json:"estimated_duration_months"`
	OutOfTownCrewLikely     bool   `json:"out_of_town_crew_likely"`
	PriorityScore           int    `json:"priority_score"`
	PriorityReason          string `json:"priority_reason"`
	SuggestedOutreachTiming string `json:"suggested_outreach_timing"`
	Notes                   string `json:"notes"`
}

// ClaudeEnricher calls the Claude Messages API to enrich a RawProject into an EnrichedLead.
type ClaudeEnricher struct {
	APIKey string
	Model  string // defaults to claude-haiku-4-5-20251001; swap to claude-sonnet-4-6 if quality needs improvement
	client *http.Client
}

// NewClaudeEnricher returns a ClaudeEnricher using Haiku by default.
func NewClaudeEnricher(apiKey string) *ClaudeEnricher {
	return &ClaudeEnricher{
		APIKey: apiKey,
		Model:  defaultModel,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

// Enrich sends a RawProject to the Claude API and returns the parsed EnrichedLead.
func (c *ClaudeEnricher) Enrich(ctx context.Context, p collector.RawProject) (*EnrichedLead, error) {
	body, err := json.Marshal(c.buildRequest(p))
	if err != nil {
		return nil, fmt.Errorf("enrichment: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, claudeAPIURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("enrichment: build request: %w", err)
	}
	req.Header.Set("x-api-key", c.APIKey)
	req.Header.Set("anthropic-version", claudeAPIVersion)
	req.Header.Set("content-type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("enrichment: api call: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("enrichment: read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("enrichment: claude returned HTTP %d: %s", resp.StatusCode, raw)
	}

	text, err := extractText(raw)
	if err != nil {
		return nil, err
	}

	var lead EnrichedLead
	if err := json.Unmarshal([]byte(stripMarkdown(text)), &lead); err != nil {
		return nil, fmt.Errorf("enrichment: parse claude json: %w\nraw response: %s", err, text)
	}

	return &lead, nil
}

// DraftOutreach generates a cold outreach email for a lead using Claude.
func (c *ClaudeEnricher) DraftOutreach(ctx context.Context, l storage.Lead) (string, error) {
	prompt := fmt.Sprintf(`Draft a short, professional cold outreach email from the Sandman Hotel Vancouver Airport sales team to the following lead.
The goal is to offer room blocks and professional rates for their upcoming project/event.

Lead Details:
Title: %s
Location: %s
General Contractor/Organizer: %s
Project Type: %s
Priority Reason: %s
Notes: %s

Guidelines:
- Keep it under 150 words.
- Focus on proximity to YVR and Richmond-based projects.
- Mention that we specialize in construction crew and event speaker lodging.
- Professional but approachable tone.`,
		l.Title, l.Location, l.GeneralContractor, l.ProjectType, l.PriorityReason, l.Notes)

	reqBody := map[string]any{
		"model":      c.Model,
		"max_tokens": 512,
		"system":     "You are a senior hotel sales manager. Draft professional outreach emails.",
		"messages": []map[string]any{
			{"role": "user", "content": prompt},
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, claudeAPIURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("x-api-key", c.APIKey)
	req.Header.Set("anthropic-version", claudeAPIVersion)
	req.Header.Set("content-type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("claude error %d: %s", resp.StatusCode, raw)
	}

	return extractText(raw)
}

// buildRequest assembles the Messages API payload.
// For Creative BC productions the user turn is source-specific; all other sources use permitPrompt.
func (c *ClaudeEnricher) buildRequest(p collector.RawProject) map[string]any {
	userContent := permitPrompt(p)
	switch p.Source {
	case "creativebc":
		userContent = creativeBCPrompt(p)
	case "vcc_events":
		userContent = vccEventsPrompt(p)
	case "google_news":
		userContent = newsPrompt(p)
	case "announcements":
		userContent = announcementsPrompt(p)
	case "eventbrite":
		userContent = eventbritePrompt(p)
	}
	return map[string]any{
		"model":      c.Model,
		"max_tokens": 512,
		"system":     systemPrompt,
		"messages": []map[string]any{
			{"role": "user", "content": userContent},
		},
	}
}

// vccEventsPrompt builds the user turn for Vancouver Convention Centre events.
// It instructs Claude to estimate attendee count and out-of-town ratio.
// Score boosters per PHASES.md: 3+ day event → +1; 500+ expected attendees → +1;
// engineering/medical/mining/government → +1; event within 6 weeks → +1.
func vccEventsPrompt(p collector.RawProject) string {
	return fmt.Sprintf(`Evaluate this event from the Vancouver Convention Centre calendar.
The Sandman Hotel Vancouver Airport (Richmond, BC) wants to reach the event organizers
to offer room blocks and professional rates for out-of-town attendees and speakers.

Return a JSON object with exactly these fields:
{
  "general_contractor": "name of the organizing association or company, or \"unknown\"",
  "project_type": "one of: conference, congress, summit, symposium, trade_show, forum, unknown",
  "estimated_crew_size": <integer — estimated total attendees; 0 if unknown>,
  "estimated_duration_months": <integer — duration of event in days; 0 if unknown>,
  "out_of_town_crew_likely": <true if the event industry is professional/medical/scientific; false for local consumer shows>,
  "priority_score": <integer 1–10; start at 4, then: +1 if 3+ day event, +1 if 500+ attendees, +1 if industry is medical/engineering/mining/tech/government, +1 if event is within 6 weeks of today>,
  "priority_reason": "one sentence explaining the score focusing on hotel night potential",
  "suggested_outreach_timing": "reach out to the organizing association's event manager; professional events book 6–12 months in advance, but last-minute speaker blocks are possible",
  "notes": "note the specific industry and any likely out-of-town attendee count"
}

Event data:
Title:       %s
Description: %s
URL:         %s`,
		p.Title,
		p.Description,
		p.SourceURL,
	)
}

// extractText pulls the assistant's text block from a Claude API response.
func extractText(raw []byte) (string, error) {
	var resp struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return "", fmt.Errorf("enrichment: parse api response: %w", err)
	}
	if resp.Error != nil {
		return "", fmt.Errorf("enrichment: claude error: %s", resp.Error.Message)
	}
	for _, block := range resp.Content {
		if block.Type == "text" {
			return block.Text, nil
		}
	}
	return "", fmt.Errorf("enrichment: no text block in claude response")
}

// stripMarkdown removes ```json ... ``` fences if Claude includes them despite instructions.
func stripMarkdown(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		if lines := strings.SplitN(s, "\n", 2); len(lines) > 1 {
			s = lines[1]
		}
		if idx := strings.LastIndex(s, "```"); idx != -1 {
			s = s[:idx]
		}
	}
	return strings.TrimSpace(s)
}

const systemPrompt = `You are a lead analyst for the Sandman Hotel Vancouver Airport in Richmond, BC (near YVR).
You evaluate building permit records to identify projects that will generate demand for construction crew lodging.

Key factors to weigh:
- Project value and type: new builds and major industrial/commercial projects bring large out-of-town crews
- Location: projects within 30 km of Richmond BC are the target area
- Contractor identity: larger or out-of-province GCs typically bring travelling crews
- Duration: longer projects mean extended-stay demand (room blocks, direct billing, weekly rates)

Respond with ONLY a valid JSON object. No markdown, no explanation, no code fences.`

// creativeBCPrompt builds the user turn for Creative BC film/TV productions.
// It instructs Claude to estimate crew size and out-of-town ratio rather than permit fields.
// Score boosters per PHASES.md: US studio → +2; "Richmond" or "Surrey" location reference → +1.
func creativeBCPrompt(p collector.RawProject) string {
	schedule, _ := p.RawData["schedule"].(string)
	address, _ := p.RawData["address"].(string)
	manager, _ := p.RawData["manager"].(string)
	return fmt.Sprintf(`Evaluate this film or TV production from the Creative BC in-production list.
The Sandman Hotel Vancouver Airport (Richmond, BC) wants to reach the production's travel coordinator
to offer room blocks and extended-stay rates for out-of-town cast and crew.

Return a JSON object with exactly these fields:
{
  "general_contractor": "name of the local production company, or \"unknown\"",
  "project_type": "one of: feature_film, tv_series, unknown",
  "estimated_crew_size": <integer — Feature Film: 150–400, TV Series per block: 80–200; 0 if unknown>,
  "estimated_duration_months": <integer — derive from the schedule dates if provided; Feature Film ~2–4 months, TV Series ~6–9 months; 0 if unknown>,
  "out_of_town_crew_likely": <true if the production company or studio is US/international; false for local BC indie>,
  "priority_score": <integer 1–10; start at 5, then: +2 if US/international studio, +1 if production address is in Richmond or Surrey (near YVR), +1 if schedule start date is within 4 weeks of today>,
  "priority_reason": "one sentence explaining the score",
  "suggested_outreach_timing": "reach out now — contact the Production Manager directly if listed; productions are actively crewing when they appear on this list",
  "notes": "include production manager name and email if present; note address city for proximity to Sandman YVR"
}

Production data:
Title:            %s
Details:          %s
Schedule:         %s
Production Address: %s
Production Manager: %s`,
		p.Title,
		p.Description,
		schedule,
		address,
		manager,
	)
}

// newsPrompt builds the user turn for Google News signals.
func newsPrompt(p collector.RawProject) string {
	return fmt.Sprintf(`Evaluate this news article for infrastructure or construction signals.
The Sandman Hotel Vancouver Airport (Richmond, BC) wants to identify projects that will bring large out-of-town crews.

Return a JSON object with exactly these fields:
{
  "general_contractor": "name of the company or developer mentioned, or \"unknown\"",
  "project_type": "one of: civil, commercial, industrial, residential, unknown",
  "estimated_crew_size": <integer — estimate based on project scale; 0 if unknown>,
  "estimated_duration_months": <integer — estimate based on project type; 0 if unknown>,
  "out_of_town_crew_likely": <true if it's a major infrastructure/industrial project; false for local retail or small housing>,
  "priority_score": <integer 1–10; start at 5, then: +1 if Richmond/YVR location, +1 if large value mentioned, +1 if civil/industrial type>,
  "priority_reason": "one sentence explaining the score",
  "suggested_outreach_timing": "reach out to the mentioned company's business development or logistics manager",
  "notes": "note any specific contractors or developers mentioned in the snippet"
}

Article data:
Title:       %s
Description: %s
URL:         %s`,
		p.Title,
		p.Description,
		p.SourceURL,
	)
}

// announcementsPrompt builds the user turn for major project announcements.
func announcementsPrompt(p collector.RawProject) string {
	return fmt.Sprintf(`Evaluate this infrastructure project announcement.
The Sandman Hotel Vancouver Airport (Richmond, BC) wants to identify projects that will bring large out-of-town crews.

Return a JSON object with exactly these fields:
{
  "general_contractor": "name of the primary contractor or 'unknown'",
  "project_type": "one of: civil, commercial, industrial, residential, unknown",
  "estimated_crew_size": <integer — estimate based on project scale; 0 if unknown>,
  "estimated_duration_months": <integer — estimate based on project type; 0 if unknown>,
  "out_of_town_crew_likely": <true if it's a major infrastructure/industrial project; false for local retail or small housing>,
  "priority_score": <integer 1–10; start at 5, then: +1 if Richmond/YVR location, +1 if major infrastructure (bridge/tunnel/airport), +1 if multi-year duration>,
  "priority_reason": "one sentence explaining the score",
  "suggested_outreach_timing": "reach out to the project's procurement or logistics manager",
  "notes": "note any specific location or scope details"
}

Announcement data:
Title:       %s
Description: %s
URL:         %s`,
		p.Title,
		p.Description,
		p.SourceURL,
	)
}

// eventbritePrompt builds the user turn for Eventbrite professional events.
func eventbritePrompt(p collector.RawProject) string {
	return fmt.Sprintf(`Evaluate this event from Eventbrite.
The Sandman Hotel Vancouver Airport (Richmond, BC) wants to offer room blocks for out-of-town attendees.

Return a JSON object with exactly these fields:
{
  "general_contractor": "name of the organizer, or \"unknown\"",
  "project_type": "one of: conference, summit, trade_show, professional_event, unknown",
  "estimated_crew_size": <integer — estimated attendees; 0 if unknown>,
  "estimated_duration_months": <integer — duration in days; 0 if unknown>,
  "out_of_town_crew_likely": <true if it's a professional/industry event; false for local workshops>,
  "priority_score": <integer 1–10; start at 4, then: +1 if 2+ days, +1 if industry is medical/tech/mining/gov, +1 if 200+ attendees>,
  "priority_reason": "one sentence explaining the score",
  "suggested_outreach_timing": "reach out to the organizer via Eventbrite or LinkedIn",
  "notes": "note the organizer name and industry"
}

Event data:
Title:       %s
Description: %s
URL:         %s`,
		p.Title,
		p.Description,
		p.SourceURL,
	)
}

// permitPrompt formats a RawProject as the user turn sent to Claude.
func permitPrompt(p collector.RawProject) string {
	return fmt.Sprintf(`Evaluate this building permit and return a JSON object with exactly these fields:
{
  "general_contractor": "company name, or \"unknown\"",
  "project_type": "one of: civil, commercial, industrial, utility, residential, unknown",
  "estimated_crew_size": <integer, 0 if unknown>,
  "estimated_duration_months": <integer, 0 if unknown>,
  "out_of_town_crew_likely": <true or false>,
  "priority_score": <integer 1-10, 10 = highest priority for hotel sales outreach>,
  "priority_reason": "one sentence explaining the score",
  "suggested_outreach_timing": "when and how the hotel should reach out",
  "notes": "any other details useful to the hotel sales team"
}

Permit data:
Source:   %s
Title:    %s
Location: %s
Value:    $%d CAD
Details:  %s
Issued:   %s`,
		p.Source, p.Title, p.Location, p.Value, p.Description, p.IssuedAt.Format("2006-01-02"),
	)
}
