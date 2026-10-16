package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/alvindcastro/groupscout/internal/llm"
	"github.com/alvindcastro/groupscout/internal/ollama"
)

// version is the build revision, stamped at build time with
// -ldflags "-X main.version=<git sha>". /health reports it so a running
// container can be traced back to the commit it was built from.
var version = "dev"

type pinger interface {
	Ping() error
}

// aiStatusReporter is the part of *llm.Chain /health reads.
type aiStatusReporter interface {
	Status() []llm.ProviderStatus
}

// healthHandler reports database, Ollama and AI provider status. Only a
// database failure makes the service unhealthy: a restart cannot fix an
// empty AI account, so the AI chain is reported but never fails the check.
func healthHandler(db pinger, ollamaEnabled bool, ollamaClient ollama.LLMClient, ai aiStatusReporter, l *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status := map[string]string{
			"status":   "ok",
			"database": "ok",
			"ollama":   "unavailable",
			"version":  version,
		}

		if err := db.Ping(); err != nil {
			l.Error("health check failed: DB ping", "error", err)
			status["database"] = "error"
			status["status"] = "error"
		}

		if ai != nil {
			status["ai"], status["ai_providers"] = aiStatus(ai.Status())
		}

		if ollamaEnabled {
			ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
			defer cancel()
			if err := ollamaClient.HealthCheck(ctx); err != nil {
				l.Warn("health check: ollama degraded", "error", err)
				status["ollama"] = "degraded"
			} else {
				status["ollama"] = "ok"
			}
		}

		w.Header().Set("Content-Type", "application/json")
		if status["status"] == "error" {
			w.WriteHeader(http.StatusServiceUnavailable)
		} else {
			w.WriteHeader(http.StatusOK)
		}
		json.NewEncoder(w).Encode(status)
	}
}
