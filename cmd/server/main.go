package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/alvindcastro/groupscout/config"
	"github.com/alvindcastro/groupscout/internal/leadnotify"
	"github.com/alvindcastro/groupscout/internal/logger"
	"github.com/alvindcastro/groupscout/internal/ollama"
	"github.com/alvindcastro/groupscout/internal/storage"
)

var runOnce = flag.Bool("run-once", false, "run the full collect→enrich→notify pipeline once and exit")

func main() {
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	if flag.NArg() > 0 && flag.Arg(0) == "ollama" {
		handleOllamaCommands(cfg)
		return
	}

	if flag.NArg() > 0 && flag.Arg(0) == "audit" {
		handleAuditCommand(cfg)
		return
	}

	if flag.NArg() > 0 && flag.Arg(0) == "audit-retention" {
		handleAuditRetentionCommand(cfg)
		return
	}

	logger.Init(cfg.JSONLog, cfg.SentryDSN)
	l := logger.Log

	if cfg.SentryDSN != "" {
		defer sentry.Flush(2 * time.Second)
	}

	var ollamaClient ollama.LLMClient
	if cfg.OllamaEnabled {
		oc := &ollama.OllamaClient{
			Endpoint: cfg.OllamaEndpoint,
			Model:    cfg.OllamaModel,
			Timeout:  30 * time.Second,
		}
		ollamaClient = oc
		l.Info(fmt.Sprintf("ollama endpoint: %s", cfg.OllamaEndpoint))
		l.Info("ollama enabled", "endpoint", cfg.OllamaEndpoint, "model", cfg.OllamaModel)

		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := oc.HealthCheck(ctx); err != nil {
				l.Warn("ollama: unavailable — running in degraded mode", "endpoint", cfg.OllamaEndpoint, "error", err)
			} else {
				l.Info("ollama: ready", "endpoint", cfg.OllamaEndpoint)
			}
		}()
	} else {
		ollamaClient = &ollama.NoopClient{}
		l.Info("ollama disabled (using no-op client)")
	}

	db, err := storage.Open(cfg.DatabaseURL)
	if err != nil {
		l.Error("failed to open database", "url", cfg.DatabaseURL, "error", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := storage.Migrate(db, cfg.DatabaseURL); err != nil {
		l.Error("failed to migrate database", "error", err)
		os.Exit(1)
	}
	l.Info("database ready", "url", cfg.DatabaseURL)

	if *runOnce {
		if cfg.AIProvider == "claude" && cfg.ClaudeAPIKey == "" {
			l.Error("CLAUDE_API_KEY is not set but AI_PROVIDER is claude")
			os.Exit(1)
		}
		if cfg.AIProvider == "gemini" && cfg.GeminiAPIKey == "" {
			l.Error("GEMINI_API_KEY is not set but AI_PROVIDER is gemini")
			os.Exit(1)
		}
		if cfg.SlackWebhookURL == "" {
			l.Error("SLACK_WEBHOOK_URL is not set")
			os.Exit(1)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()

		if _, err := runPipeline(ctx, cfg, db, PipelineRunOptions{}); err != nil {
			l.Error("pipeline failed", "error", err)
			sentry.CaptureException(err)
			os.Exit(1)
		}
		return
	}

	startAuditRetentionWorker(cfg, db)

	if cfg.APIToken == "" {
		l.Warn("API_TOKEN not set; server will be insecure (all requests allowed)")
	}

	http.Handle("/metrics", promhttp.Handler())
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		status := map[string]string{
			"status":   "ok",
			"database": "ok",
			"ollama":   "unavailable",
		}

		if err := db.Ping(); err != nil {
			l.Error("health check failed: DB ping", "error", err)
			status["database"] = "error"
			status["status"] = "error"
		}

		if cfg.OllamaEnabled {
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
	})

	http.HandleFunc("/run", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if !authorized(r, cfg.APIToken) {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		l.Info("pipeline triggered via HTTP /run")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()

		type runRequest struct {
			BCBidRawInput    string `json:"bcbid_raw_input"`
			GuaranteeOneLead bool   `json:"guarantee_one_lead"`
			DeliveryMode     string `json:"delivery_mode"`
			CadenceKey       string `json:"cadence_key"`
			IdempotencyKey   string `json:"idempotency_key"`
			ScheduleKey      string `json:"schedule_key"`
		}
		var req runRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.BCBidRawInput != "" {
			ctx = context.WithValue(ctx, "bcbid_raw_input", req.BCBidRawInput)
		}
		if req.IdempotencyKey == "" {
			req.IdempotencyKey = r.Header.Get("Idempotency-Key")
		}
		if req.ScheduleKey == "" {
			req.ScheduleKey = req.CadenceKey
		}
		if req.IdempotencyKey == "" {
			req.IdempotencyKey = req.CadenceKey
		}

		deliveryMode := strings.ToLower(req.DeliveryMode)
		// A cadence_key or schedule_key implies guaranteed delivery — this is defensive against
		// n8n expression failures that silently drop guarantee_one_lead from the request body.
		hasScheduleKey := req.CadenceKey != "" || req.ScheduleKey != ""
		result, err := runPipeline(ctx, cfg, db, PipelineRunOptions{
			GuaranteeOneLead: req.GuaranteeOneLead || deliveryMode == "all_eligible" || deliveryMode == "one_lead" || deliveryMode == "exactly_one" || hasScheduleKey,
			IdempotencyKey:   req.IdempotencyKey,
			ScheduleKey:      req.ScheduleKey,
		})
		if err != nil {
			l.Error("pipeline failed", "error", err)
			sentry.CaptureException(err)
			http.Error(w, fmt.Sprintf("Pipeline failed: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(result)
	})

	http.HandleFunc("/digest", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if !authorized(r, cfg.APIToken) {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		l.Info("weekly digest triggered via HTTP /digest")
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		leadStore := storage.NewLeadStoreWithDSN(db, cfg.DatabaseURL)
		leads, err := leadStore.ListForDigest(ctx)
		if err != nil {
			l.Error("list for digest failed", "error", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if len(leads) == 0 {
			fmt.Fprintln(w, "No leads for digest")
			return
		}

		emailNotifier := leadnotify.NewEmailNotifier(cfg.ResendAPIKey, cfg.EmailFrom)
		toEmail := r.URL.Query().Get("to")
		if toEmail == "" {
			toEmail = "alvin@groupscout.ai"
		}

		if err := emailNotifier.SendWeeklyDigest(ctx, toEmail, leads); err != nil {
			l.Error("send email failed", "error", err)
			sentry.CaptureException(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		fmt.Fprintf(w, "Digest sent with %d leads to %s\n", len(leads), toEmail)
	})
	http.HandleFunc("/ingest", handleIngest(cfg, func() singleProjectProcessor {
		return buildEnricher(cfg, db, nil)
	}))
	http.HandleFunc("/n8n/webhook", handleN8NWebhook(
		cfg,
		storage.NewLeadStoreWithDSN(db, cfg.DatabaseURL),
		leadnotify.NewSlackNotifier(cfg.SlackWebhookURL, cfg.BaseURL),
	))

	http.HandleFunc("/leads/", func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) != 3 || parts[0] != "leads" || parts[2] != "raw" {
			http.NotFound(w, r)
			return
		}
		leadID := parts[1]

		ctx := r.Context()
		leadStore := storage.NewLeadStoreWithDSN(db, cfg.DatabaseURL)
		lead, err := leadStore.GetByID(ctx, leadID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if lead == nil {
			http.NotFound(w, r)
			return
		}

		if lead.RawInputID == "" {
			http.Error(w, "Lead has no raw input associated", http.StatusNotFound)
			return
		}

		rawInputID, err := uuid.Parse(lead.RawInputID)
		if err != nil {
			http.Error(w, "Invalid raw input ID", http.StatusInternalServerError)
			return
		}

		auditStore := storage.NewAuditStoreWithDSN(db, cfg.DatabaseURL)
		raw, err := auditStore.GetByID(ctx, rawInputID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if raw == nil {
			http.Error(w, "Raw input not found in audit store", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", raw.PayloadType)
		w.WriteHeader(http.StatusOK)
		w.Write(raw.Payload)
	})

	addr := ":" + strconv.Itoa(cfg.Port)
	l.Info("server listening", "addr", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		l.Error("server failed", "error", err)
		os.Exit(1)
	}
}
