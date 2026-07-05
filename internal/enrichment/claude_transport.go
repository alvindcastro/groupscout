package enrichment

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/alvindcastro/groupscout/internal/collector"
	"github.com/alvindcastro/groupscout/internal/storage"
)

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
