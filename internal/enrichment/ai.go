package enrichment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/alvindcastro/groupscout/internal/collector"
	"github.com/alvindcastro/groupscout/internal/llm"
	"github.com/alvindcastro/groupscout/internal/storage"
)

// EnrichedLead is the structured output of the AI enrichment step. Field
// names match the JSON keys the prompts ask for.
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

	// Provider and Model name what produced the enrichment. They are not
	// part of the model's reply.
	Provider string `json:"-"`
	Model    string `json:"-"`
}

// Completer sends one request through the provider chain (*llm.Chain).
type Completer interface {
	Complete(ctx context.Context, req llm.Request) (llm.Response, error)
}

const (
	// The enrichment reply is about 300 tokens. The limit leaves room for
	// reasoning models, whose thinking counts against it.
	enrichMaxTokens   = 2048
	outreachMaxTokens = 1024
)

// LLMEnricher enriches projects and drafts outreach through a provider
// chain, so any configured provider can serve the request.
type LLMEnricher struct {
	llm Completer
}

// NewLLMEnricher returns an EnricherAI backed by c.
func NewLLMEnricher(c Completer) *LLMEnricher {
	return &LLMEnricher{llm: c}
}

// Enrich asks the chain to score a project. A reply that is not a usable
// lead counts as invalid output, so the chain retries or moves on.
func (e *LLMEnricher) Enrich(ctx context.Context, p collector.RawProject) (*EnrichedLead, error) {
	resp, err := e.llm.Complete(ctx, llm.Request{
		UseCase:   "enrich",
		System:    systemPrompt,
		User:      enrichmentPrompt(p),
		MaxTokens: enrichMaxTokens,
		JSON:      true,
		Validate: func(text string) error {
			_, err := parseEnrichedLead(text)
			return err
		},
	})
	if err != nil {
		return nil, fmt.Errorf("enrichment: %w", err)
	}
	lead, err := parseEnrichedLead(resp.Text)
	if err != nil {
		return nil, fmt.Errorf("enrichment: %s: %w", resp.Provider, err)
	}
	lead.Provider = resp.Provider
	lead.Model = resp.Model
	return lead, nil
}

// DraftOutreach asks the chain for a cold outreach email about a lead.
func (e *LLMEnricher) DraftOutreach(ctx context.Context, l storage.Lead) (string, error) {
	resp, err := e.llm.Complete(ctx, llm.Request{
		UseCase:   "outreach",
		System:    outreachSystemPrompt,
		User:      outreachPrompt(l),
		MaxTokens: outreachMaxTokens,
	})
	if err != nil {
		return "", fmt.Errorf("draft outreach: %w", err)
	}
	return strings.TrimSpace(resp.Text), nil
}

// parseEnrichedLead reads a model reply into an EnrichedLead. It tolerates
// what different models do with the same prompt (code fences, text around
// the object, numbers as strings) and rejects a reply without a 1-10
// priority_score, the one field delivery depends on.
func parseEnrichedLead(text string) (*EnrichedLead, error) {
	var raw map[string]any
	if err := json.Unmarshal([]byte(jsonObject(text)), &raw); err != nil {
		return nil, fmt.Errorf("reply is not a JSON object: %w", err)
	}
	score, ok := asInt(raw["priority_score"])
	if !ok {
		return nil, errors.New("reply has no numeric priority_score")
	}
	if score < 1 || score > 10 {
		return nil, fmt.Errorf("priority_score %d is outside 1-10", score)
	}
	crew, _ := asInt(raw["estimated_crew_size"])
	duration, _ := asInt(raw["estimated_duration_months"])
	return &EnrichedLead{
		GeneralContractor:       asString(raw["general_contractor"]),
		ProjectType:             normalizeProjectType(asString(raw["project_type"])),
		EstimatedCrewSize:       max(crew, 0),
		EstimatedDurationMonths: max(duration, 0),
		OutOfTownCrewLikely:     asBool(raw["out_of_town_crew_likely"]),
		PriorityScore:           score,
		PriorityReason:          asString(raw["priority_reason"]),
		SuggestedOutreachTiming: asString(raw["suggested_outreach_timing"]),
		Notes:                   asString(raw["notes"]),
	}, nil
}

// jsonObject returns the outermost {...} in a reply, after removing code
// fences, or the trimmed reply when it has no braces.
func jsonObject(text string) string {
	s := stripMarkdown(text)
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start < 0 || end < start {
		return s
	}
	return s[start : end+1]
}

func asString(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(t)
	case float64, bool:
		return fmt.Sprint(t)
	default:
		b, err := json.Marshal(t)
		if err != nil {
			return ""
		}
		return string(b)
	}
}

// asInt reads a JSON number, or the first integer in a string such as "150",
// "1,200" or "80-200".
func asInt(v any) (int, bool) {
	switch t := v.(type) {
	case float64:
		return int(math.Round(t)), true
	case string:
		s := strings.ReplaceAll(strings.TrimSpace(t), ",", "")
		end := 0
		for end < len(s) && (s[end] >= '0' && s[end] <= '9' || (end == 0 && s[end] == '-')) {
			end++
		}
		n, err := strconv.Atoi(s[:end])
		return n, err == nil
	}
	return 0, false
}

func asBool(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		switch strings.ToLower(strings.TrimSpace(t)) {
		case "true", "yes", "likely":
			return true
		}
	}
	return false
}

func normalizeProjectType(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.Join(strings.Fields(s), "_")
	if s == "" {
		return "unknown"
	}
	return s
}
