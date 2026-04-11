package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// LLMClient defines the interface for local LLM interactions.
type LLMClient interface {
	Generate(ctx context.Context, prompt string) (string, error)
	ChatComplete(ctx context.Context, system, user string) (string, error)
	HealthCheck(ctx context.Context) error
}

// OllamaClient implements the LLMClient interface for the Ollama HTTP API.
type OllamaClient struct {
	Endpoint string
	Model    string
	Timeout  time.Duration
}

// Generate sends a simple prompt to the Ollama /api/generate endpoint.
func (c *OllamaClient) Generate(ctx context.Context, prompt string) (string, error) {
	reqBody := map[string]interface{}{
		"model":  c.Model,
		"prompt": prompt,
		"stream": false,
	}

	var respBody struct {
		Response string `json:"response"`
	}

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

	var respBody struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

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

func (c *OllamaClient) doWithRetry(ctx context.Context, method, path string, body interface{}, out interface{}) error {
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
			return nil
		}

		// Retry only on 5xx or connection errors, once, with 2s backoff.
		// For simplicity, we'll check if it's a retryable error.
		if attempt < 2 && isRetryable(err) {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(2 * time.Second):
				continue
			}
		}

		return err
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
