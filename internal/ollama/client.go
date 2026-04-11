package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/alvindcastro/groupscout/internal/logger"
)

type useCaseKey struct{}

// WithUseCase adds a use case name to the context for metrics and logging.
func WithUseCase(ctx context.Context, useCase string) context.Context {
	return context.WithValue(ctx, useCaseKey{}, useCase)
}

func getUseCase(ctx context.Context) string {
	if v, ok := ctx.Value(useCaseKey{}).(string); ok {
		return v
	}
	return "unknown"
}

// LLMClient defines the interface for local LLM interactions.
type LLMClient interface {
	Generate(ctx context.Context, prompt string) (string, error)
	ChatComplete(ctx context.Context, system, user string) (string, error)
	HealthCheck(ctx context.Context) error
	ListModels(ctx context.Context) ([]string, error)
}

// OllamaClient implements the LLMClient interface for the Ollama HTTP API.
type OllamaClient struct {
	Endpoint string
	Model    string
	Timeout  time.Duration
}

type GenerateResponse struct {
	Response        string `json:"response"`
	PromptEvalCount int    `json:"prompt_eval_count"`
	EvalCount       int    `json:"eval_count"`
}

type ChatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// Generate sends a simple prompt to the Ollama /api/generate endpoint.
func (c *OllamaClient) Generate(ctx context.Context, prompt string) (string, error) {
	reqBody := map[string]interface{}{
		"model":  c.Model,
		"prompt": prompt,
		"stream": false,
	}

	var respBody GenerateResponse

	err := c.doWithRetry(ctx, http.MethodPost, "/api/generate", reqBody, &respBody)
	if err != nil {
		return "", err
	}

	return respBody.Response, nil
}

// ChatComplete sends a system and user prompt to the Ollama /v1/chat/completions endpoint.
func (c *OllamaClient) ChatComplete(ctx context.Context, system, user string) (string, error) {
	reqBody := map[string]interface{}{
		"model": c.Model,
		"messages": []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": user},
		},
		"stream": false,
	}

	var respBody ChatResponse

	err := c.doWithRetry(ctx, http.MethodPost, "/v1/chat/completions", reqBody, &respBody)
	if err != nil {
		return "", err
	}

	if len(respBody.Choices) == 0 {
		return "", fmt.Errorf("ollama: no chat choices returned")
	}

	return respBody.Choices[0].Message.Content, nil
}

// HealthCheck verifies that the Ollama server is reachable and responding.
func (c *OllamaClient) HealthCheck(ctx context.Context) error {
	return c.doWithRetry(ctx, http.MethodGet, "/api/tags", nil, nil)
}

// ListModels returns a list of model names currently available in Ollama.
func (c *OllamaClient) ListModels(ctx context.Context) ([]string, error) {
	var respBody struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}

	err := c.doWithRetry(ctx, http.MethodGet, "/api/tags", nil, &respBody)
	if err != nil {
		return nil, err
	}

	names := make([]string, len(respBody.Models))
	for i, m := range respBody.Models {
		names[i] = m.Name
	}
	return names, nil
}

func (c *OllamaClient) doWithRetry(ctx context.Context, method, path string, body interface{}, out interface{}) error {
	useCase := getUseCase(ctx)
	start := time.Now()

	var bodyBytes []byte
	var err error
	if body != nil {
		bodyBytes, err = json.Marshal(body)
		if err != nil {
			return fmt.Errorf("ollama marshal: %w", err)
		}
	}

	attempt := 0
	for {
		attempt++
		err = c.doOnce(ctx, method, path, bodyBytes, out)
		if err == nil {
			latency := time.Since(start)
			recordMetrics(useCase, "success", latency)
			logCall(useCase, c.Model, latency, out, nil)
			return nil
		}

		// Retry only on 5xx or connection errors, once, with 2s backoff.
		if attempt < 2 && isRetryable(err) {
			select {
			case <-ctx.Done():
				recordMetrics(useCase, "error", time.Since(start))
				logCall(useCase, c.Model, time.Since(start), nil, ctx.Err())
				return ctx.Err()
			case <-time.After(2 * time.Second):
				continue
			}
		}

		recordMetrics(useCase, "error", time.Since(start))
		logCall(useCase, c.Model, time.Since(start), nil, err)
		return err
	}
}

func recordMetrics(useCase, status string, latency time.Duration) {
	CallsTotal.WithLabelValues(useCase, status).Inc()
	LatencyMS.WithLabelValues(useCase).Observe(float64(latency.Milliseconds()))
}

func logCall(useCase, model string, latency time.Duration, out interface{}, err error) {
	tokens := 0
	if out != nil {
		switch r := out.(type) {
		case *GenerateResponse:
			tokens = r.PromptEvalCount + r.EvalCount
		case *ChatResponse:
			tokens = r.Usage.TotalTokens
		}
	}

	args := []any{
		"use_case", useCase,
		"model", model,
		"latency_ms", latency.Milliseconds(),
	}
	if tokens > 0 {
		args = append(args, "tokens_approx", tokens)
	}
	if err != nil {
		args = append(args, "error", err.Error())
		logger.Log.Error("ollama call failed", args...)
	} else {
		logger.Log.Info("ollama call successful", args...)
	}
}

func (c *OllamaClient) doOnce(ctx context.Context, method, path string, body []byte, out interface{}) error {
	url := c.Endpoint + path
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("ollama request: %w", err)
	}

	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/json")
	}

	client := &http.Client{
		Timeout: c.Timeout,
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("ollama do: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return &OllamaError{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("ollama error: %s", resp.Status),
		}
	}

	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return fmt.Errorf("ollama decode: %w", err)
		}
	}

	return nil
}

type OllamaError struct {
	StatusCode int
	Message    string
}

func (e *OllamaError) Error() string {
	return e.Message
}

func isRetryable(err error) bool {
	if err == nil {
		return false
	}
	if oerr, ok := err.(*OllamaError); ok {
		return oerr.StatusCode >= 500
	}
	// Connection errors are also retryable
	return true
}

// NoopClient is a no-op implementation of LLMClient used when Ollama is disabled.
type NoopClient struct{}

func (c *NoopClient) Generate(ctx context.Context, prompt string) (string, error) {
	return "", nil
}

func (c *NoopClient) ChatComplete(ctx context.Context, system, user string) (string, error) {
	return "", nil
}

func (c *NoopClient) HealthCheck(ctx context.Context) error {
	return nil
}

func (c *NoopClient) ListModels(ctx context.Context) ([]string, error) {
	return nil, nil
}
