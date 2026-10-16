package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	// DefaultAnthropicModel is Claude's cheap structured-output model.
	DefaultAnthropicModel = "claude-haiku-4-5-20251001"

	anthropicBaseURL = "https://api.anthropic.com"
	anthropicVersion = "2023-06-01"
)

// Anthropic calls Claude through the Messages API.
type Anthropic struct {
	apiKey  string
	model   string
	baseURL string
	http    *http.Client
}

// NewAnthropic returns a Claude client. An empty model uses
// DefaultAnthropicModel; an empty baseURL uses the public API.
func NewAnthropic(apiKey, model, baseURL string, timeout time.Duration) *Anthropic {
	if model == "" {
		model = DefaultAnthropicModel
	}
	if baseURL == "" {
		baseURL = anthropicBaseURL
	}
	return &Anthropic{
		apiKey:  apiKey,
		model:   model,
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    &http.Client{Timeout: timeout},
	}
}

func (a *Anthropic) Provider() string { return "claude" }
func (a *Anthropic) Model() string    { return a.model }

// Complete sends one Messages API request. Claude has no JSON mode switch on
// this API version, so Request.JSON relies on the prompt.
func (a *Anthropic) Complete(ctx context.Context, req Request) (Response, error) {
	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 1024
	}
	payload := map[string]any{
		"model":      a.model,
		"max_tokens": maxTokens,
		"messages":   []map[string]string{{"role": "user", "content": req.User}},
	}
	if req.System != "" {
		payload["system"] = req.System
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return Response{}, fmt.Errorf("claude: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, a.baseURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return Response{}, fmt.Errorf("claude: build request: %w", err)
	}
	httpReq.Header.Set("x-api-key", a.apiKey)
	httpReq.Header.Set("anthropic-version", anthropicVersion)
	httpReq.Header.Set("content-type", "application/json")

	raw, err := send(ctx, a.http, httpReq, a.Provider())
	if err != nil {
		return Response{}, err
	}

	var resp struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		StopReason string `json:"stop_reason"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return Response{}, &Error{Provider: a.Provider(), Kind: KindInvalidOutput, Message: "response is not JSON: " + truncate(string(raw))}
	}
	for _, block := range resp.Content {
		if block.Type == "text" && strings.TrimSpace(block.Text) != "" {
			return Response{Text: block.Text, Provider: a.Provider(), Model: a.model}, nil
		}
	}
	return Response{}, emptyReply(a.Provider(), resp.StopReason == "max_tokens")
}

// send performs the request and returns the 2xx body. Transport failures are
// transient; a cancelled or expired caller context is returned as is so the
// chain stops instead of failing over.
func send(ctx context.Context, client *http.Client, req *http.Request, provider string) ([]byte, error) {
	resp, err := client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, &Error{Provider: provider, Kind: KindTransient, Message: transportMessage(err)}
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, &Error{Provider: provider, Kind: KindTransient, Status: resp.StatusCode, Message: "read response: " + err.Error()}
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, classifyHTTP(provider, resp.StatusCode, raw)
	}
	return raw, nil
}

// transportMessage drops the request URL that net/http puts in its errors, so
// a credential in a URL can never reach the logs.
func transportMessage(err error) string {
	msg := err.Error()
	if i := strings.Index(msg, "\": "); i >= 0 && strings.Contains(msg[:i], "://") {
		msg = msg[i+3:]
	}
	return truncate(msg)
}

func emptyReply(provider string, cutOff bool) *Error {
	if cutOff {
		return &Error{Provider: provider, Kind: KindInvalidOutput, Message: "reply was cut off at the token limit before any text"}
	}
	return &Error{Provider: provider, Kind: KindInvalidOutput, Message: "reply had no text"}
}
