package enrichment

import (
	"context"
	"fmt"
	"strings"

	"github.com/alvindcastro/groupscout/internal/ollama"
	"github.com/alvindcastro/groupscout/internal/storage"
)

// OllamaScorer handles generating rationale for lead scores.
type OllamaScorer struct {
	client ollama.LLMClient
}

// NewOllamaScorer returns a new OllamaScorer.
func NewOllamaScorer(client ollama.LLMClient) *OllamaScorer {
	return &OllamaScorer{client: client}
}

// Rationale generates a 2-3 sentence explanation for why a lead has its priority score.
func (s *OllamaScorer) Rationale(ctx context.Context, lead storage.Lead) (string, error) {
	ctx = ollama.WithUseCase(ctx, "scoring")
	systemPrompt := `
You are a senior hotel sales analyst at a Vancouver-area full-service hotel.
You receive structured data about a potential group lodging lead.
Your job is to write 2–3 concise sentences explaining why this lead is strong
(or not), what type of group it represents, and what action the sales rep should take.

Be direct. Use dollar figures and room night estimates where you can infer them.
Do not use bullet points. Write in plain prose.
Keep your response under 250 characters.
`

	summary := fmt.Sprintf(
		"Project: %s. Type: %s. Location: %s. Crew Size: %d. Duration Months: %d. Priority Score: %d. Value: %d.",
		lead.Title, lead.ProjectType, lead.Location, lead.EstimatedCrewSize, lead.EstimatedDurationMonths, lead.PriorityScore, lead.ProjectValue,
	)

	resp, err := s.client.ChatComplete(ctx, systemPrompt, summary)
	if err != nil {
		// Fallback: empty rationale
		return "", nil
	}

	rationale := strings.TrimSpace(resp)

	// Truncate to 280 characters at the last complete word
	if len(rationale) > 280 {
		rationale = rationale[:281] // include one extra to see if it's a word boundary
		lastSpace := strings.LastIndex(rationale, " ")
		if lastSpace != -1 {
			rationale = rationale[:lastSpace]
		} else {
			rationale = rationale[:280]
		}
	}

	return strings.TrimSpace(rationale), nil
}
