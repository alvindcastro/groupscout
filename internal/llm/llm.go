// Package llm is how GroupScout calls a language model without depending on
// one provider. Two wire formats cover every supported provider: Anthropic's
// Messages API (Claude) and OpenAI's Chat Completions API (OpenAI, Gemini's
// OpenAI-compatible endpoint, Ollama, LM Studio, llama.cpp, vLLM, LiteLLM and
// other gateways). Chain tries an ordered list of providers and fails over
// between them.
package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// Request is one provider-neutral completion request.
type Request struct {
	// UseCase labels metrics and logs, for example "enrich" or "outreach".
	UseCase   string
	System    string
	User      string
	MaxTokens int
	// JSON asks for a single JSON object where the provider has a JSON output
	// mode. The prompt must still ask for JSON: not every provider has one.
	JSON bool
	// Validate, when set, checks the reply before Chain accepts it. A non-nil
	// error counts as invalid output: Chain retries the provider once, then
	// moves on to the next one.
	Validate func(text string) error
}

// Response is a completion and the provider and model that produced it.
type Response struct {
	Text     string
	Provider string
	Model    string
}

// Client is one provider and model.
type Client interface {
	Provider() string
	Model() string
	Complete(ctx context.Context, req Request) (Response, error)
}

// Kind says how a failed call should be handled.
type Kind string

const (
	// KindAccount means the provider account cannot serve any request: no
	// credit, quota used up, a bad key or an unknown model. Retrying is
	// pointless until someone fixes the account, so Chain stops using the
	// provider for a while.
	KindAccount Kind = "account_error"
	// KindTransient covers rate limits, overload, server errors, timeouts and
	// network failures. Chain retries with backoff.
	KindTransient Kind = "transient_error"
	// KindRequest means the provider rejected this particular request.
	KindRequest Kind = "request_error"
	// KindInvalidOutput means the reply was empty, cut off, or failed
	// Request.Validate.
	KindInvalidOutput Kind = "invalid_output"
)

// Error is a failed call to one provider.
type Error struct {
	Provider string
	Kind     Kind
	Status   int // HTTP status, 0 when there was no response
	Message  string
}

func (e *Error) Error() string {
	if e.Status > 0 {
		return fmt.Sprintf("%s: %s (HTTP %d): %s", e.Provider, e.Kind, e.Status, e.Message)
	}
	return fmt.Sprintf("%s: %s: %s", e.Provider, e.Kind, e.Message)
}

// KindOf returns the Kind of err. Errors that are not *Error are treated as
// transient.
func KindOf(err error) Kind {
	var e *Error
	if errors.As(err, &e) {
		return e.Kind
	}
	return KindTransient
}

// maxErrorMessage bounds provider error text kept in errors and logs.
const maxErrorMessage = 300

// classifyHTTP turns a non-2xx provider response into an *Error. It reads the
// error body shapes Anthropic, OpenAI and Gemini's compatibility endpoint use.
func classifyHTTP(provider string, status int, body []byte) *Error {
	errType, code, message := parseErrorBody(body)
	lower := strings.ToLower(errType + " " + code + " " + message)

	kind := KindRequest
	switch {
	// Anthropic reports an empty prepaid balance as a 400
	// invalid_request_error, so the message is the only reliable signal.
	case strings.Contains(lower, "credit balance"),
		strings.Contains(lower, "billing"),
		strings.Contains(lower, "insufficient_quota"),
		strings.Contains(lower, "insufficient balance"),
		// Gemini answers a bad key with 400 INVALID_ARGUMENT.
		strings.Contains(lower, "api key not valid"),
		strings.Contains(lower, "api_key_invalid"):
		kind = KindAccount
	case status == 401, status == 402, status == 403, status == 404:
		kind = KindAccount
	case status == 408, status == 409, status == 429, status >= 500:
		kind = KindTransient
	}
	return &Error{Provider: provider, Kind: kind, Status: status, Message: message}
}

// parseErrorBody extracts the error type, code and message from a provider
// error body, falling back to the raw text.
func parseErrorBody(body []byte) (errType, code, message string) {
	type apiError struct {
		Error struct {
			Type    string `json:"type"`
			Code    any    `json:"code"`
			Status  string `json:"status"`
			Message string `json:"message"`
		} `json:"error"`
	}
	var one apiError
	if err := json.Unmarshal(body, &one); err != nil || one.Error.Message == "" {
		// Gemini's compatibility endpoint can wrap the error in an array.
		var many []apiError
		if err := json.Unmarshal(body, &many); err == nil && len(many) > 0 {
			one = many[0]
		}
	}
	e := one.Error
	if e.Message == "" {
		return "", "", truncate(strings.TrimSpace(string(body)))
	}
	if e.Code != nil {
		code = fmt.Sprint(e.Code)
	}
	errType = e.Type
	if errType == "" {
		errType = e.Status
	}
	return errType, code, truncate(e.Message)
}

func truncate(s string) string {
	if len(s) <= maxErrorMessage {
		return s
	}
	return s[:maxErrorMessage] + "…"
}
