package enrichment

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/alvindcastro/groupscout/internal/collector"
	"github.com/alvindcastro/groupscout/internal/storage"
)

const (
	geminiAPIURL = "https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s"
	geminiModel  = "gemini-2.0-flash"
)

// GeminiEnricher calls the Google Gemini API to enrich a RawProject into an EnrichedLead.
type GeminiEnricher struct {
	APIKey string
	Model  string
	client *http.Client
}

// NewGeminiEnricher returns a GeminiEnricher using gemini-2.0-flash by default.
func NewGeminiEnricher(apiKey string) *GeminiEnricher {
	return &GeminiEnricher{
		APIKey: apiKey,
		Model:  geminiModel,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

// Enrich sends a RawProject to the Gemini API and returns the parsed EnrichedLead.
func (g *GeminiEnricher) Enrich(ctx context.Context, p collector.RawProject) (*EnrichedLead, error) {
	url := fmt.Sprintf(geminiAPIURL, g.Model, g.APIKey)

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

	reqBody := map[string]any{
		"contents": []map[string]any{
			{
				"parts": []map[string]any{
					{"text": systemPrompt + "\n\n" + userContent},
				},
			},
		},
		"generationConfig": map[string]any{
			"responseMimeType": "application/json",
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("gemini enrichment: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("gemini enrichment: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := g.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gemini enrichment: api call: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("gemini enrichment: read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gemini enrichment: gemini returned HTTP %d: %s", resp.StatusCode, raw)
	}

	text, err := extractGeminiText(raw)
	if err != nil {
		return nil, err
	}

	var lead EnrichedLead
	if err := json.Unmarshal([]byte(stripMarkdown(text)), &lead); err != nil {
		return nil, fmt.Errorf("gemini enrichment: parse json: %w\nraw response: %s", err, text)
	}

	return &lead, nil
}

// DraftOutreach generates a cold outreach email for a lead using Gemini.
func (g *GeminiEnricher) DraftOutreach(ctx context.Context, l storage.Lead) (string, error) {
	url := fmt.Sprintf(geminiAPIURL, g.Model, g.APIKey)

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
		"contents": []map[string]any{
			{
				"parts": []map[string]any{
					{"text": "You are a senior hotel sales manager. Draft professional outreach emails.\n\n" + prompt},
				},
			},
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := g.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("gemini error %d: %s", resp.StatusCode, raw)
	}

	return extractGeminiText(raw)
}

// extractGeminiText pulls the assistant's text block from a Gemini API response.
func extractGeminiText(raw []byte) (string, error) {
	var resp struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return "", fmt.Errorf("gemini enrichment: parse api response: %w", err)
	}
	if resp.Error != nil {
		return "", fmt.Errorf("gemini enrichment: gemini error: %s", resp.Error.Message)
	}
	if len(resp.Candidates) > 0 && len(resp.Candidates[0].Content.Parts) > 0 {
		return resp.Candidates[0].Content.Parts[0].Text, nil
	}
	return "", fmt.Errorf("gemini enrichment: no text block in gemini response")
}
