package ollama

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestOllamaClient_Generate(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("expected POST, got %s", r.Method)
			}
			if r.URL.Path != "/api/generate" {
				t.Errorf("expected /api/generate, got %s", r.URL.Path)
			}

			var body struct {
				Model  string `json:"model"`
				Prompt string `json:"prompt"`
				Stream bool   `json:"stream"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}

			if body.Model != "mistral" {
				t.Errorf("expected mistral, got %s", body.Model)
			}
			if body.Prompt != "hello" {
				t.Errorf("expected hello, got %s", body.Prompt)
			}
			if body.Stream != false {
				t.Error("expected stream false")
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"response": "world"})
		}))
		defer ts.Close()

		client := &OllamaClient{
			Endpoint: ts.URL,
			Model:    "mistral",
			Timeout:  5 * time.Second,
		}

		resp, err := client.Generate(context.Background(), "hello")
		if err != nil {
			t.Fatal(err)
		}
		if resp != "world" {
			t.Errorf("expected world, got %s", resp)
		}
	})

	t.Run("retry on 500", func(t *testing.T) {
		attempts := 0
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			attempts++
			if attempts == 1 {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"response": "recovered"})
		}))
		defer ts.Close()

		client := &OllamaClient{
			Endpoint: ts.URL,
			Model:    "mistral",
			Timeout:  5 * time.Second,
		}

		// Use a short retry wait for tests if we can, but the prompt says 2s backoff.
		// For the sake of this test, we'll just check it retries.
		resp, err := client.Generate(context.Background(), "hello")
		if err != nil {
			t.Fatal(err)
		}
		if attempts != 2 {
			t.Errorf("expected 2 attempts, got %d", attempts)
		}
		if resp != "recovered" {
			t.Errorf("expected recovered, got %s", resp)
		}
	})

	t.Run("context cancellation", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(100 * time.Millisecond)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"response": "too late"})
		}))
		defer ts.Close()

		client := &OllamaClient{
			Endpoint: ts.URL,
			Model:    "mistral",
			Timeout:  5 * time.Second,
		}

		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		_, err := client.Generate(ctx, "hello")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "context deadline exceeded") {
			t.Errorf("expected deadline exceeded, got %v", err)
		}
	})
}

func TestOllamaClient_ChatComplete(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("expected /v1/chat/completions, got %s", r.URL.Path)
		}

		var body struct {
			Model    string `json:"model"`
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}

		if len(body.Messages) != 2 {
			t.Errorf("expected 2 messages, got %d", len(body.Messages))
		}
		if body.Messages[0].Role != "system" || body.Messages[0].Content != "sys" {
			t.Error("bad system message")
		}
		if body.Messages[1].Role != "user" || body.Messages[1].Content != "usr" {
			t.Error("bad user message")
		}

		resp := map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]string{
						"content": "chat response",
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client := &OllamaClient{
		Endpoint: ts.URL,
		Model:    "mistral",
		Timeout:  5 * time.Second,
	}

	resp, err := client.ChatComplete(context.Background(), "sys", "usr")
	if err != nil {
		t.Fatal(err)
	}
	if resp != "chat response" {
		t.Errorf("expected 'chat response', got %s", resp)
	}
}

func TestOllamaClient_HealthCheck(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/api/tags" {
				t.Errorf("expected /api/tags, got %s", r.URL.Path)
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer ts.Close()

		client := &OllamaClient{Endpoint: ts.URL}
		if err := client.HealthCheck(context.Background()); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("fail", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer ts.Close()

		client := &OllamaClient{Endpoint: ts.URL}
		if err := client.HealthCheck(context.Background()); err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("degraded_503", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer ts.Close()

		client := &OllamaClient{Endpoint: ts.URL, Timeout: 1 * time.Second}
		err := client.HealthCheck(context.Background())
		if err == nil {
			t.Fatal("expected error for 503, got nil")
		}

		oerr, ok := err.(*OllamaError)
		if !ok {
			t.Fatalf("expected OllamaError, got %T", err)
		}
		if oerr.StatusCode != 503 {
			t.Errorf("expected status code 503, got %d", oerr.StatusCode)
		}
	})
}

func TestNoopClient(t *testing.T) {
	client := &NoopClient{}

	resp, err := client.Generate(context.Background(), "hello")
	if err != nil || resp != "" {
		t.Errorf("expected (empty, nil), got (%s, %v)", resp, err)
	}

	resp, err = client.ChatComplete(context.Background(), "sys", "usr")
	if err != nil || resp != "" {
		t.Errorf("expected (empty, nil), got (%s, %v)", resp, err)
	}

	if err := client.HealthCheck(context.Background()); err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}
