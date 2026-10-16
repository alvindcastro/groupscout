package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alvindcastro/groupscout/internal/ollama"
)

type fakePinger struct{ err error }

func (f fakePinger) Ping() error { return f.err }

type failingOllama struct{ ollama.NoopClient }

func (failingOllama) HealthCheck(ctx context.Context) error { return errors.New("connection refused") }

func serveHealth(t *testing.T, h http.HandlerFunc) (int, map[string]string) {
	t.Helper()
	rr := httptest.NewRecorder()
	h(rr, httptest.NewRequest(http.MethodGet, "/health", nil))

	var body map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("health body is not JSON: %v (%q)", err, rr.Body.String())
	}
	return rr.Code, body
}

func TestHealthHandler_ReportsBuildVersion(t *testing.T) {
	old := version
	version = "abc1234"
	t.Cleanup(func() { version = old })

	l := slog.New(slog.NewTextHandler(io.Discard, nil))
	code, body := serveHealth(t, healthHandler(fakePinger{}, false, &ollama.NoopClient{}, nil, l))

	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
	want := map[string]string{"status": "ok", "database": "ok", "ollama": "unavailable", "version": "abc1234"}
	for k, v := range want {
		if body[k] != v {
			t.Errorf("%s: expected %q, got %q", k, v, body[k])
		}
	}
}

func TestHealthHandler_DatabaseDownReturns503(t *testing.T) {
	l := slog.New(slog.NewTextHandler(io.Discard, nil))
	code, body := serveHealth(t, healthHandler(fakePinger{err: errors.New("down")}, false, &ollama.NoopClient{}, nil, l))

	if code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", code)
	}
	if body["status"] != "error" || body["database"] != "error" {
		t.Errorf("expected status/database error, got %v", body)
	}
}

func TestHealthHandler_OllamaDegradedStaysHealthy(t *testing.T) {
	l := slog.New(slog.NewTextHandler(io.Discard, nil))
	code, body := serveHealth(t, healthHandler(fakePinger{}, true, &failingOllama{}, nil, l))

	if code != http.StatusOK {
		t.Fatalf("expected 200 when only Ollama is degraded, got %d", code)
	}
	if body["ollama"] != "degraded" {
		t.Errorf("expected ollama degraded, got %q", body["ollama"])
	}
}

func TestHealthHandler_OllamaOK(t *testing.T) {
	l := slog.New(slog.NewTextHandler(io.Discard, nil))
	_, body := serveHealth(t, healthHandler(fakePinger{}, true, &ollama.NoopClient{}, nil, l))

	if body["ollama"] != "ok" {
		t.Errorf("expected ollama ok, got %q", body["ollama"])
	}
}
