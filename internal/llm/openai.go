package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Output-limit request fields. OpenAI's own API wants max_completion_tokens
// (its reasoning models reject max_tokens); most compatible servers only know
// max_tokens.
const (
	TokenParamMaxCompletionTokens = "max_completion_tokens"
	TokenParamMaxTokens           = "max_tokens"
	TokenParamNone                = ""
)

// OpenAIConfig describes one OpenAI-format endpoint.
type OpenAIConfig struct {
	// Provider is the name used in logs, metrics and the leads table, such
	// as "openai", "gemini", "ollama" or "custom".
	Provider string
	// BaseURL is the API root that /chat/completions is appended to, for
	// example https://api.openai.com/v1.
	BaseURL string
	// APIKey is sent as a bearer token. Local servers may not need one.
	APIKey string
	Model  string
	// TokenParam names the output-limit field; TokenParamNone sends none.
	TokenParam string
	// ReasoningEffort is sent as reasoning_effort when set, for reasoning
	// models that accept it.
	ReasoningEffort string
	// JSONMode sends response_format {"type":"json_object"} when a request
	// asks for JSON.
	JSONMode bool
	Timeout  time.Duration
}

// OpenAI calls any server that speaks OpenAI's Chat Completions API.
type OpenAI struct {
	cfg  OpenAIConfig
	http *http.Client
}

// NewOpenAI returns a client for one OpenAI-format endpoint.
func NewOpenAI(cfg OpenAIConfig) *OpenAI {
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")
	return &OpenAI{cfg: cfg, http: &http.Client{Timeout: cfg.Timeout}}
}

func (o *OpenAI) Provider() string { return o.cfg.Provider }
func (o *OpenAI) Model() string    { return o.cfg.Model }

// Complete sends one chat completion request.
func (o *OpenAI) Complete(ctx context.Context, req Request) (Response, error) {
	var messages []map[string]string
	if req.System != "" {
		messages = append(messages, map[string]string{"role": "system", "content": req.System})
	}
	messages = append(messages, map[string]string{"role": "user", "content": req.User})

	payload := map[string]any{
		"model":    o.cfg.Model,
		"messages": messages,
		"stream":   false,
	}
	if o.cfg.TokenParam != TokenParamNone && req.MaxTokens > 0 {
		payload[o.cfg.TokenParam] = req.MaxTokens
	}
	if o.cfg.ReasoningEffort != "" {
		payload["reasoning_effort"] = o.cfg.ReasoningEffort
	}
	if req.JSON && o.cfg.JSONMode {
		payload["response_format"] = map[string]string{"type": "json_object"}
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return Response{}, fmt.Errorf("%s: marshal request: %w", o.cfg.Provider, err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, o.cfg.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return Response{}, fmt.Errorf("%s: build request: %w", o.cfg.Provider, err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if o.cfg.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+o.cfg.APIKey)
	}

	raw, err := send(ctx, o.http, httpReq, o.cfg.Provider)
	if err != nil {
		return Response{}, err
	}

	var resp struct {
		Choices []struct {
			Message struct {
				Content *string `json:"content"`
				Refusal string  `json:"refusal"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return Response{}, &Error{Provider: o.cfg.Provider, Kind: KindInvalidOutput, Message: "response is not JSON: " + truncate(string(raw))}
	}
	if len(resp.Choices) == 0 {
		return Response{}, emptyReply(o.cfg.Provider, false)
	}
	choice := resp.Choices[0]
	if choice.Message.Refusal != "" {
		return Response{}, &Error{Provider: o.cfg.Provider, Kind: KindInvalidOutput, Message: "model refused: " + truncate(choice.Message.Refusal)}
	}
	if choice.Message.Content == nil || strings.TrimSpace(*choice.Message.Content) == "" {
		return Response{}, emptyReply(o.cfg.Provider, choice.FinishReason == "length")
	}
	return Response{Text: *choice.Message.Content, Provider: o.cfg.Provider, Model: o.cfg.Model}, nil
}
