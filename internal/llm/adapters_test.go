package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// captured is one request a fake provider received.
type captured struct {
	path    string
	headers http.Header
	body    map[string]any
}

// fakeServer answers every request with status and body and records what it
// was sent.
func fakeServer(t *testing.T, status int, body string) (*httptest.Server, *[]captured) {
	t.Helper()
	var got []captured
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var m map[string]any
		_ = json.Unmarshal(raw, &m)
		got = append(got, captured{path: r.URL.Path, headers: r.Header.Clone(), body: m})
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)
	return srv, &got
}

var jsonRequest = Request{UseCase: "enrich", System: "be terse", User: "score this", MaxTokens: 700, JSON: true}

// ── Anthropic ─────────────────────────────────────────────────────────────────

func TestAnthropic_sendsMessagesRequest(t *testing.T) {
	srv, got := fakeServer(t, 200, `{"content":[{"type":"text","text":"{\"priority_score\":8}"}],"stop_reason":"end_turn"}`)
	c := NewAnthropic("sk-ant-test", "", srv.URL, 5*time.Second)

	resp, err := c.Complete(context.Background(), jsonRequest)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Text != `{"priority_score":8}` || resp.Provider != "claude" || resp.Model != DefaultAnthropicModel {
		t.Errorf("response = %+v", resp)
	}

	req := (*got)[0]
	if req.path != "/v1/messages" {
		t.Errorf("path = %s", req.path)
	}
	if req.headers.Get("x-api-key") != "sk-ant-test" || req.headers.Get("anthropic-version") != anthropicVersion {
		t.Errorf("headers = %v", req.headers)
	}
	if req.body["system"] != "be terse" || req.body["max_tokens"] != float64(700) || req.body["model"] != DefaultAnthropicModel {
		t.Errorf("body = %v", req.body)
	}
	msgs := req.body["messages"].([]any)
	if len(msgs) != 1 || msgs[0].(map[string]any)["content"] != "score this" {
		t.Errorf("messages = %v", msgs)
	}
}

func TestAnthropic_classifiesErrors(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
		want   Kind
	}{
		// The exact reply that stopped enrichment from 2026-10-07 to 10-16.
		{"credit balance", 400, `{"type":"error","error":{"type":"invalid_request_error","message":"Your credit balance is too low to access the Anthropic API. Please go to Plans & Billing to upgrade or purchase credits."}}`, KindAccount},
		{"bad key", 401, `{"type":"error","error":{"type":"authentication_error","message":"invalid x-api-key"}}`, KindAccount},
		{"unknown model", 404, `{"type":"error","error":{"type":"not_found_error","message":"model: claude-nope"}}`, KindAccount},
		{"rate limited", 429, `{"type":"error","error":{"type":"rate_limit_error","message":"slow down"}}`, KindTransient},
		{"overloaded", 529, `{"type":"error","error":{"type":"overloaded_error","message":"Overloaded"}}`, KindTransient},
		{"server error", 500, `oops`, KindTransient},
		{"bad request", 400, `{"type":"error","error":{"type":"invalid_request_error","message":"messages: field required"}}`, KindRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv, _ := fakeServer(t, tt.status, tt.body)
			_, err := NewAnthropic("k", "", srv.URL, 5*time.Second).Complete(context.Background(), jsonRequest)
			if KindOf(err) != tt.want {
				t.Fatalf("kind = %s, want %s (err %v)", KindOf(err), tt.want, err)
			}
			if e := err.(*Error); e.Status != tt.status || e.Provider != "claude" {
				t.Errorf("error = %+v", e)
			}
		})
	}
}

func TestAnthropic_emptyReplyIsInvalidOutput(t *testing.T) {
	srv, _ := fakeServer(t, 200, `{"content":[],"stop_reason":"max_tokens"}`)
	_, err := NewAnthropic("k", "", srv.URL, 5*time.Second).Complete(context.Background(), jsonRequest)
	if KindOf(err) != KindInvalidOutput || !strings.Contains(err.Error(), "token limit") {
		t.Fatalf("err = %v", err)
	}
}

func TestAnthropic_unreachableIsTransientAndHidesURL(t *testing.T) {
	srv, _ := fakeServer(t, 200, `{}`)
	url := srv.URL + "/?key=secret"
	srv.Close()
	_, err := NewAnthropic("k", "", url, 2*time.Second).Complete(context.Background(), jsonRequest)
	if KindOf(err) != KindTransient {
		t.Fatalf("kind = %s (err %v)", KindOf(err), err)
	}
	if strings.Contains(err.Error(), "secret") {
		t.Errorf("transport error leaked the URL: %v", err)
	}
}

// ── OpenAI format ─────────────────────────────────────────────────────────────

const openAIReply = `{"choices":[{"message":{"role":"assistant","content":"{\"priority_score\":6}"},"finish_reason":"stop"}]}`

func TestOpenAI_sendsChatCompletionRequest(t *testing.T) {
	srv, got := fakeServer(t, 200, openAIReply)
	c := NewOpenAI(OpenAIConfig{
		Provider: "openai", BaseURL: srv.URL + "/v1/", APIKey: "sk-test", Model: "gpt-test",
		TokenParam: TokenParamMaxCompletionTokens, ReasoningEffort: "low", JSONMode: true, Timeout: 5 * time.Second,
	})

	resp, err := c.Complete(context.Background(), jsonRequest)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Text != `{"priority_score":6}` || resp.Provider != "openai" || resp.Model != "gpt-test" {
		t.Errorf("response = %+v", resp)
	}

	req := (*got)[0]
	if req.path != "/v1/chat/completions" || req.headers.Get("Authorization") != "Bearer sk-test" {
		t.Errorf("path %s, auth %q", req.path, req.headers.Get("Authorization"))
	}
	if req.body["max_completion_tokens"] != float64(700) || req.body["max_tokens"] != nil {
		t.Errorf("token field wrong: %v", req.body)
	}
	if req.body["reasoning_effort"] != "low" {
		t.Errorf("reasoning_effort = %v", req.body["reasoning_effort"])
	}
	if rf, _ := req.body["response_format"].(map[string]any); rf["type"] != "json_object" {
		t.Errorf("response_format = %v", req.body["response_format"])
	}
	msgs := req.body["messages"].([]any)
	if len(msgs) != 2 || msgs[0].(map[string]any)["role"] != "system" || msgs[1].(map[string]any)["content"] != "score this" {
		t.Errorf("messages = %v", msgs)
	}
}

func TestOpenAI_optionalFieldsAreLeftOut(t *testing.T) {
	srv, got := fakeServer(t, 200, openAIReply)
	c := NewOpenAI(OpenAIConfig{Provider: "custom", BaseURL: srv.URL, Model: "local", TokenParam: TokenParamNone, Timeout: 5 * time.Second})

	if _, err := c.Complete(context.Background(), Request{User: "hi", MaxTokens: 100, JSON: true}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	req := (*got)[0]
	for _, field := range []string{"max_tokens", "max_completion_tokens", "reasoning_effort", "response_format"} {
		if _, ok := req.body[field]; ok {
			t.Errorf("%s should not be sent: %v", field, req.body)
		}
	}
	if req.headers.Get("Authorization") != "" {
		t.Error("no key configured, so no Authorization header")
	}
	if msgs := req.body["messages"].([]any); len(msgs) != 1 {
		t.Errorf("no system prompt, so one message: %v", msgs)
	}
}

func TestOpenAI_classifiesErrors(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
		want   Kind
	}{
		{"quota used up", 429, `{"error":{"message":"You exceeded your current quota, please check your plan and billing details.","type":"insufficient_quota","code":"insufficient_quota"}}`, KindAccount},
		{"rate limited", 429, `{"error":{"message":"Rate limit reached for gpt-4.1-mini","type":"requests","code":"rate_limit_exceeded"}}`, KindTransient},
		{"bad key", 401, `{"error":{"message":"Incorrect API key provided: sk-****abcd.","type":"invalid_request_error","code":"invalid_api_key"}}`, KindAccount},
		{"unknown model", 404, `{"error":{"message":"The model gpt-nope does not exist","type":"invalid_request_error","code":"model_not_found"}}`, KindAccount},
		{"bad parameter", 400, `{"error":{"message":"Unsupported parameter: 'max_tokens'","type":"invalid_request_error","code":"unsupported_parameter"}}`, KindRequest},
		{"gemini array error", 400, `[{"error":{"code":400,"message":"API key not valid. Please pass a valid API key.","status":"INVALID_ARGUMENT"}}]`, KindAccount},
		{"gemini overloaded", 503, `[{"error":{"code":503,"message":"The model is overloaded.","status":"UNAVAILABLE"}}]`, KindTransient},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv, _ := fakeServer(t, tt.status, tt.body)
			c := NewOpenAI(OpenAIConfig{Provider: "openai", BaseURL: srv.URL, Model: "m", Timeout: 5 * time.Second})
			_, err := c.Complete(context.Background(), jsonRequest)
			if KindOf(err) != tt.want {
				t.Fatalf("kind = %s, want %s (err %v)", KindOf(err), tt.want, err)
			}
			var e *Error = err.(*Error)
			if e.Message == "" || strings.HasPrefix(e.Message, "[") || strings.HasPrefix(e.Message, "{") {
				t.Errorf("message should be the provider's text, got %q", e.Message)
			}
		})
	}
}

func TestOpenAI_unusableReplies(t *testing.T) {
	for name, body := range map[string]string{
		"cut off":    `{"choices":[{"message":{"content":""},"finish_reason":"length"}]}`,
		"no choices": `{"choices":[]}`,
		"null text":  `{"choices":[{"message":{"content":null},"finish_reason":"stop"}]}`,
		"refusal":    `{"choices":[{"message":{"content":null,"refusal":"I can't help with that."},"finish_reason":"stop"}]}`,
		"not json":   `<html>proxy error</html>`,
	} {
		t.Run(name, func(t *testing.T) {
			srv, _ := fakeServer(t, 200, body)
			c := NewOpenAI(OpenAIConfig{Provider: "openai", BaseURL: srv.URL, Model: "m", Timeout: 5 * time.Second})
			_, err := c.Complete(context.Background(), jsonRequest)
			if KindOf(err) != KindInvalidOutput {
				t.Fatalf("kind = %s (err %v)", KindOf(err), err)
			}
		})
	}
}

func TestErrorMessagesAreBounded(t *testing.T) {
	long := strings.Repeat("x", 5000)
	e := classifyHTTP("openai", 500, []byte(long))
	if len(e.Message) > maxErrorMessage+len("…") {
		t.Errorf("message length %d", len(e.Message))
	}
}
