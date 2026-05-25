package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/alvindcastro/groupscout/config"
	"github.com/alvindcastro/groupscout/internal/auditretention"
	"github.com/alvindcastro/groupscout/internal/collector"
	"github.com/alvindcastro/groupscout/internal/collector/events"
	"github.com/alvindcastro/groupscout/internal/collector/news"
	"github.com/alvindcastro/groupscout/internal/collector/permits"
	"github.com/alvindcastro/groupscout/internal/enrichment"
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

	// Ollama CLI subcommands
	if flag.NArg() > 0 && flag.Arg(0) == "ollama" {
		handleOllamaCommands(cfg)
		return
	}

	// Audit CLI subcommand
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

	// Ollama initialization
	var ollamaClient ollama.LLMClient
	if cfg.OllamaEnabled {
		oc := &ollama.OllamaClient{
			Endpoint: cfg.OllamaEndpoint,
			Model:    cfg.OllamaModel,
			Timeout:  30 * time.Second, // default timeout
		}
		ollamaClient = oc
		l.Info(fmt.Sprintf("ollama endpoint: %s", cfg.OllamaEndpoint))
		l.Info("ollama enabled", "endpoint", cfg.OllamaEndpoint, "model", cfg.OllamaModel)

		// Health check (non-blocking)
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
	_ = ollamaClient // will be used in subsequent phases

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

	// Server mode
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

		// Auth check
		if cfg.APIToken != "" {
			authHeader := r.Header.Get("Authorization")
			if !strings.HasPrefix(authHeader, "Bearer ") || strings.TrimPrefix(authHeader, "Bearer ") != cfg.APIToken {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
		}

		l.Info("pipeline triggered via HTTP /run")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()

		// Extract BC Bid raw input if provided in JSON body
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
		result, err := runPipeline(ctx, cfg, db, PipelineRunOptions{
			GuaranteeOneLead: req.GuaranteeOneLead || deliveryMode == "one_lead" || deliveryMode == "exactly_one",
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

		// Auth check
		if cfg.APIToken != "" {
			authHeader := r.Header.Get("Authorization")
			if !strings.HasPrefix(authHeader, "Bearer ") || strings.TrimPrefix(authHeader, "Bearer ") != cfg.APIToken {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
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

		emailNotifier := leadnotify.NewEmailNotifier(cfg.ResendAPIKey)
		toEmail := r.URL.Query().Get("to")
		if toEmail == "" {
			toEmail = "alvin@groupscout.ai" // default
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
		// Manual routing for /leads/{id}/raw
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

func handleN8NWebhook(cfg *config.Config, leadStore storage.LeadStore, notifier leadnotify.Notifier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if !authorized(r, cfg.APIToken) {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		var l storage.Lead
		if err := json.NewDecoder(r.Body).Decode(&l); err != nil {
			logger.Log.Error("failed to decode n8n lead", "error", err)
			http.Error(w, "Invalid JSON body", http.StatusBadRequest)
			return
		}

		normalizeWebhookLead(&l)

		if err := leadStore.Insert(r.Context(), &l); err != nil {
			logger.Log.Error("failed to insert n8n lead", "error", err)
			http.Error(w, fmt.Sprintf("Failed to store lead: %v", err), http.StatusInternalServerError)
			return
		}

		logger.Log.Info("lead received from n8n", "source", l.Source, "title", l.Title)

		if err := notifier.Send(r.Context(), []storage.Lead{l}); err != nil {
			logger.Log.Warn("failed to notify Slack for n8n lead", "error", err)
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"status": "success", "id": l.ID})
	}
}

func normalizeWebhookLead(l *storage.Lead) {
	if l.Source == "" {
		l.Source = "n8n"
	}
	if l.ID == "" {
		l.ID = storage.NewUUID()
	}
	l.PriorityScore = normalizeExternalPriorityScore(l.PriorityScore)
}

func normalizeExternalPriorityScore(score int) int {
	switch {
	case score < 0:
		return 0
	case score <= 10:
		return score
	case score <= 100:
		return int(math.Round(float64(score) / 10))
	default:
		return 10
	}
}

type PipelineRunOptions struct {
	GuaranteeOneLead bool
	IdempotencyKey   string
	ScheduleKey      string
}

type PipelineRunResult struct {
	Status            string `json:"status"`
	NewLeads          int    `json:"new_leads"`
	NotifiedLeads     int    `json:"notified_leads"`
	DeliveryStatus    string `json:"delivery_status,omitempty"`
	DeliveredLeadID   string `json:"delivered_lead_id,omitempty"`
	IdempotencyKey    string `json:"idempotency_key,omitempty"`
	ScheduleKey       string `json:"schedule_key,omitempty"`
	DeliveryDuplicate bool   `json:"delivery_duplicate,omitempty"`
}

type singleProjectProcessor interface {
	EnrichOne(ctx context.Context, p collector.RawProject) (bool, error)
}

type ingestRequest struct {
	Source       string         `json:"source"`
	ExternalID   string         `json:"external_id"`
	Title        string         `json:"title"`
	Location     string         `json:"location"`
	Value        int64          `json:"value"`
	ProjectValue int64          `json:"project_value"`
	Description  string         `json:"description"`
	IssuedAt     time.Time      `json:"issued_at"`
	SourceURL    string         `json:"source_url"`
	RawData      string         `json:"raw_data"`
	RawType      string         `json:"raw_type"`
	Metadata     map[string]any `json:"metadata"`
}

func handleIngest(cfg *config.Config, newProcessor func() singleProjectProcessor) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !authorized(r, cfg.APIToken) {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		var req ingestRequest
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON body", http.StatusBadRequest)
			return
		}

		project, err := req.rawProject()
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		inserted, err := newProcessor().EnrichOne(r.Context(), project)
		if err != nil {
			logger.Log.Error("ingest enrichment failed", "error", err)
			http.Error(w, fmt.Sprintf("Failed to ingest project: %v", err), http.StatusInternalServerError)
			return
		}

		status := "created"
		code := http.StatusCreated
		if !inserted {
			status = "duplicate"
			code = http.StatusOK
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		json.NewEncoder(w).Encode(map[string]any{
			"status":      status,
			"inserted":    inserted,
			"source":      project.Source,
			"external_id": project.ExternalID,
		})
	}
}

func (r ingestRequest) rawProject() (collector.RawProject, error) {
	if strings.TrimSpace(r.Title) == "" && strings.TrimSpace(r.Description) == "" && strings.TrimSpace(r.RawData) == "" {
		return collector.RawProject{}, fmt.Errorf("title, description, or raw_data is required")
	}
	source := strings.TrimSpace(r.Source)
	if source == "" {
		source = "api"
	}
	value := r.Value
	if value == 0 {
		value = r.ProjectValue
	}
	raw := []byte(r.RawData)
	if len(raw) == 0 {
		raw = []byte(r.Description)
	}
	rawType := strings.TrimSpace(r.RawType)
	if rawType == "" {
		rawType = "text/plain"
	}
	return collector.RawProject{
		Source:      source,
		ExternalID:  strings.TrimSpace(r.ExternalID),
		Title:       strings.TrimSpace(r.Title),
		Location:    strings.TrimSpace(r.Location),
		Value:       value,
		Description: strings.TrimSpace(r.Description),
		IssuedAt:    r.IssuedAt,
		SourceURL:   strings.TrimSpace(r.SourceURL),
		RawData:     raw,
		RawType:     rawType,
		Metadata:    r.Metadata,
	}, nil
}

func authorized(r *http.Request, token string) bool {
	if token == "" {
		return true
	}
	authHeader := r.Header.Get("Authorization")
	return strings.HasPrefix(authHeader, "Bearer ") && strings.TrimPrefix(authHeader, "Bearer ") == token
}

func runPipeline(ctx context.Context, cfg *config.Config, db *sql.DB, opts PipelineRunOptions) (PipelineRunResult, error) {
	l := logger.Log
	leadStore := storage.NewLeadStoreWithDSN(db, cfg.DatabaseURL)
	deliveryStore := storage.NewDeliveryStore(db, cfg.DatabaseURL)

	result := PipelineRunResult{Status: "success"}
	if opts.GuaranteeOneLead {
		opts = normalizeDeliveryOptions(opts, time.Now())
		result.IdempotencyKey = opts.IdempotencyKey
		result.ScheduleKey = opts.ScheduleKey

		owner := storage.NewUUID()
		locked, err := deliveryStore.TryAcquireRunLock(ctx, "lead-delivery", owner, 15*time.Minute)
		if err != nil {
			return result, fmt.Errorf("acquire delivery lock: %w", err)
		}
		if !locked {
			result.Status = "locked"
			result.DeliveryStatus = "locked"
			return result, nil
		}
		defer func() {
			if err := deliveryStore.ReleaseRunLock(context.Background(), "lead-delivery", owner); err != nil {
				logger.Log.Warn("failed to release delivery lock", "error", err)
			}
		}()

		existing, err := deliveryStore.GetByIdempotencyKey(ctx, opts.IdempotencyKey)
		if err != nil {
			return result, fmt.Errorf("load delivery by idempotency key: %w", err)
		}
		if existing != nil && existing.Status == "sent" {
			result.NotifiedLeads = 1
			result.DeliveredLeadID = existing.LeadID
			result.DeliveryStatus = "duplicate"
			result.DeliveryDuplicate = true
			return result, nil
		}
	}

	rc := permits.NewRichmondCollector()
	rc.MinValue = cfg.MinPermitValueCAD
	rc.Verbose = true
	collectors := []collector.Collector{rc}

	if cfg.DeltaPermitsURL != "" {
		dc := permits.NewDeltaCollector(cfg.DeltaPermitsURL)
		dc.MinValue = cfg.MinPermitValueCAD
		dc.Verbose = true
		collectors = append(collectors, dc)
	}

	if cfg.CreativeBCEnabled {
		cbc := events.NewCreativeBCCollector(cfg.CreativeBCURL)
		cbc.Verbose = true
		collectors = append(collectors, cbc)
	}

	if cfg.VCCEnabled {
		vc := events.NewVCCCollector(cfg.VCCURL)
		vc.Verbose = true
		l.Info("VCC collector enabled", "url", cfg.VCCURL)
		collectors = append(collectors, vc)
	} else {
		l.Info("VCC collector disabled")
	}
	if cfg.BCBidEnabled {
		bc := news.NewBCBidCollector(strings.Split(cfg.BCBidRSSURL, ","))
		bc.Verbose = true
		collectors = append(collectors, bc)
	}

	l.Debug("config news", "enabled", cfg.NewsEnabled, "url", cfg.NewsRSSURL)

	if cfg.NewsEnabled {
		nc := news.NewNewsCollector(cfg.NewsRSSURL)
		nc.Verbose = true
		collectors = append(collectors, nc)
	}

	if cfg.AnnouncementsEnabled {
		ac := news.NewAnnouncementsCollector()
		ac.Verbose = true
		collectors = append(collectors, ac)
	}

	if cfg.EventbriteEnabled {
		ec := events.NewEventbriteCollector(cfg.EventbriteURL)
		ec.Verbose = true
		collectors = append(collectors, ec)
	}

	var names []string
	for _, c := range collectors {
		names = append(names, c.Name())
	}
	l.Info("active collectors", "count", len(names), "names", names)

	e := buildEnricher(cfg, db, collectors)
	e.Verbose = true

	l.Info("running pipeline...")
	n, err := e.Run(ctx)
	if err != nil {
		return result, fmt.Errorf("enricher run: %w", err)
	}
	l.Info("enrichment complete", "new_leads", n)
	result.NewLeads = n

	if opts.GuaranteeOneLead {
		delivery, err := deliverGuaranteedLead(ctx, leadStore, deliveryStore, leadnotify.NewSlackNotifier(cfg.SlackWebhookURL, cfg.BaseURL), opts)
		if err != nil {
			_ = deliveryStore.UpsertResult(ctx, storage.LeadDelivery{
				IdempotencyKey: opts.IdempotencyKey,
				ScheduleKey:    opts.ScheduleKey,
				Channel:        "slack",
				Status:         "failed",
				Result:         err.Error(),
			})
			return result, fmt.Errorf("guaranteed lead delivery: %w", err)
		}
		result.DeliveryStatus = delivery.Status
		result.DeliveredLeadID = delivery.LeadID
		if delivery.Status == "sent" {
			result.NotifiedLeads = 1
		}
		return result, nil
	}

	leads, err := leadStore.ListNew(ctx)
	if err != nil {
		return result, fmt.Errorf("list leads: %w", err)
	}

	if len(leads) == 0 {
		l.Info("no new leads to notify")
		return result, nil
	}

	notifier := leadnotify.NewSlackNotifier(cfg.SlackWebhookURL, cfg.BaseURL)
	if err := notifier.Send(ctx, leads); err != nil {
		return result, fmt.Errorf("slack notify: %w", err)
	}
	l.Info("sent leads to Slack", "count", len(leads))
	result.NotifiedLeads = len(leads)

	// Mark leads as notified so they don't re-appear in the next run's digest.
	for _, l := range leads {
		if err := leadStore.UpdateStatus(ctx, l.ID, "notified"); err != nil {
			logger.Log.Warn("failed to update status for lead", "id", l.ID, "error", err)
		}
	}
	return result, nil
}

func buildEnricher(cfg *config.Config, db *sql.DB, collectors []collector.Collector) *enrichment.Enricher {
	rawStore := storage.NewRawProjectStoreWithDSN(db, cfg.DatabaseURL)
	auditStore := storage.NewAuditStoreWithDSN(db, cfg.DatabaseURL)
	leadStore := storage.NewLeadStoreWithDSN(db, cfg.DatabaseURL)

	var ai enrichment.EnricherAI
	if cfg.AIProvider == "gemini" {
		ai = enrichment.NewGeminiEnricher(cfg.GeminiAPIKey)
		logger.Log.Info("using Gemini for enrichment")
	} else {
		ai = enrichment.NewClaudeEnricher(cfg.ClaudeAPIKey)
		logger.Log.Info("using Claude for enrichment")
	}

	scorer := enrichment.NewScorer(cfg.EnrichmentThreshold)

	var ollamaExtractor *enrichment.Extractor
	var ollamaScorer *enrichment.OllamaScorer
	if cfg.OllamaEnabled {
		oc := &ollama.OllamaClient{
			Endpoint: cfg.OllamaEndpoint,
			Model:    cfg.OllamaModel,
			Timeout:  time.Duration(cfg.OllamaExtractTimeoutS) * time.Second,
		}
		ollamaExtractor = enrichment.NewExtractor(oc)

		sc := &ollama.OllamaClient{
			Endpoint: cfg.OllamaEndpoint,
			Model:    cfg.OllamaModel,
			Timeout:  time.Duration(cfg.OllamaScoreTimeoutS) * time.Second,
		}
		ollamaScorer = enrichment.NewOllamaScorer(sc)
	}

	return enrichment.NewEnricher(collectors, rawStore, auditStore, leadStore, ai, scorer, cfg.PriorityAlertThreshold, ollamaExtractor, ollamaScorer, cfg.OllamaExtractionEnabled, cfg.OllamaScoringEnabled)
}

func normalizeDeliveryOptions(opts PipelineRunOptions, now time.Time) PipelineRunOptions {
	if opts.ScheduleKey == "" {
		opts.ScheduleKey = cadenceKey(now)
	}
	if opts.IdempotencyKey == "" {
		opts.IdempotencyKey = opts.ScheduleKey
	}
	return opts
}

func cadenceKey(now time.Time) string {
	loc, err := time.LoadLocation("America/Vancouver")
	if err == nil {
		now = now.In(loc)
	}
	weekday := strings.ToLower(now.Weekday().String())
	return fmt.Sprintf("lead-cadence:%s:%s", now.Format("2006-01-02"), weekday)
}

func deliverGuaranteedLead(ctx context.Context, leadStore storage.LeadStore, deliveryStore storage.DeliveryStore, notifier leadnotify.Notifier, opts PipelineRunOptions) (storage.LeadDelivery, error) {
	candidates, err := leadStore.ListDeliveryCandidates(ctx, 1)
	if err != nil {
		return storage.LeadDelivery{}, fmt.Errorf("list delivery candidates: %w", err)
	}
	if len(candidates) == 0 {
		delivery := storage.LeadDelivery{
			IdempotencyKey: opts.IdempotencyKey,
			ScheduleKey:    opts.ScheduleKey,
			Channel:        "slack",
			Status:         "no_eligible_lead",
			Result:         "no eligible new or backlog lead available",
		}
		if err := deliveryStore.UpsertResult(ctx, delivery); err != nil {
			return delivery, fmt.Errorf("record no eligible lead: %w", err)
		}
		return delivery, nil
	}

	lead := candidates[0]
	if err := notifier.Send(ctx, []storage.Lead{lead}); err != nil {
		return storage.LeadDelivery{LeadID: lead.ID, Status: "failed"}, err
	}
	if lead.Status == "new" {
		if err := leadStore.UpdateStatus(ctx, lead.ID, "notified"); err != nil {
			return storage.LeadDelivery{LeadID: lead.ID, Status: "failed"}, fmt.Errorf("mark lead notified: %w", err)
		}
	}

	delivery := storage.LeadDelivery{
		IdempotencyKey: opts.IdempotencyKey,
		ScheduleKey:    opts.ScheduleKey,
		LeadID:         lead.ID,
		Channel:        "slack",
		Status:         "sent",
		Result:         "sent one lead to slack",
		SentAt:         time.Now().UTC(),
	}
	if err := deliveryStore.UpsertResult(ctx, delivery); err != nil {
		return delivery, fmt.Errorf("record sent delivery: %w", err)
	}
	return delivery, nil
}

func startAuditRetentionWorker(cfg *config.Config, db *sql.DB) {
	if !cfg.AuditRetentionEnabled {
		return
	}

	worker, err := auditretention.New(
		storage.NewAuditStoreWithDSN(db, cfg.DatabaseURL),
		auditretention.Policy{
			RetentionDays: cfg.AuditRetentionDays,
			Interval:      time.Duration(cfg.AuditRetentionIntervalH) * time.Hour,
			RunOnStart:    cfg.AuditRetentionRunOnStart,
		},
		logger.Log,
	)
	if err != nil {
		logger.Log.Error("invalid audit retention configuration", "error", err)
		os.Exit(1)
	}

	go worker.Run(context.Background())
	logger.Log.Info(
		"audit retention worker started",
		"retention_days", cfg.AuditRetentionDays,
		"interval_hours", cfg.AuditRetentionIntervalH,
		"run_on_start", cfg.AuditRetentionRunOnStart,
	)
}

func handleOllamaCommands(cfg *config.Config) {
	if flag.NArg() < 2 {
		fmt.Println("Usage: ollama [push-models | list-models]")
		os.Exit(1)
	}

	oc := &ollama.OllamaClient{
		Endpoint: cfg.OllamaEndpoint,
		Model:    cfg.OllamaModel,
		Timeout:  30 * time.Second,
	}
	manager := ollama.NewModelfileManager(oc)
	ctx := context.Background()

	switch flag.Arg(1) {
	case "push-models":
		pushModels(ctx, manager)
	case "list-models":
		listModels(ctx, manager)
	default:
		fmt.Printf("Unknown ollama subcommand: %s\n", flag.Arg(1))
		os.Exit(1)
	}
	os.Exit(0)
}

func pushModels(ctx context.Context, manager *ollama.ModelfileManager) {
	files, err := os.ReadDir("internal/ollama/modelfile")
	if err != nil {
		log.Fatalf("failed to read modelfiles: %v", err)
	}

	for _, f := range files {
		if !strings.HasSuffix(f.Name(), ".modelfile") {
			continue
		}

		path := filepath.Join("internal/ollama/modelfile", f.Name())
		content, err := os.ReadFile(path)
		if err != nil {
			log.Printf("failed to read %s: %v", path, err)
			continue
		}

		// "permit_extractor.modelfile" → "groupscout-permit-extractor"
		stem := strings.TrimSuffix(f.Name(), ".modelfile")
		modelName := "groupscout-" + strings.ReplaceAll(stem, "_", "-")

		fmt.Printf("Pushing %s as %s... ", f.Name(), modelName)
		if err := manager.Push(ctx, modelName, string(content)); err != nil {
			fmt.Printf("FAILED: %v\n", err)
		} else {
			fmt.Println("OK")
		}
	}
}

func listModels(ctx context.Context, manager *ollama.ModelfileManager) {
	models, err := manager.ListModels(ctx)
	if err != nil {
		log.Fatalf("failed to list models: %v", err)
	}

	fmt.Println("Loaded Ollama models:")
	for _, m := range models {
		fmt.Printf("- %s\n", m)
	}
}

func handleAuditCommand(cfg *config.Config) {
	auditCmd := flag.NewFlagSet("audit", flag.ExitOnError)
	savePath := auditCmd.String("save", "", "save payload to a file")
	showMeta := auditCmd.Bool("meta", false, "show metadata only")
	auditCmd.Parse(flag.Args()[1:])

	if auditCmd.NArg() < 1 {
		fmt.Println("Usage: groupscout audit <lead_id> [--save <path>] [--meta]")
		os.Exit(1)
	}
	leadID := auditCmd.Arg(0)

	db, err := storage.Open(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	leadStore := storage.NewLeadStoreWithDSN(db, cfg.DatabaseURL)
	lead, err := leadStore.GetByID(ctx, leadID)
	if err != nil {
		log.Fatalf("get lead: %v", err)
	}
	if lead == nil {
		log.Fatalf("lead %s not found", leadID)
	}

	if lead.RawInputID == "" {
		log.Fatalf("lead %s has no raw input associated", leadID)
	}

	rawInputID, err := uuid.Parse(lead.RawInputID)
	if err != nil {
		log.Fatalf("invalid raw input ID: %v", err)
	}

	auditStore := storage.NewAuditStoreWithDSN(db, cfg.DatabaseURL)
	raw, err := auditStore.GetByID(ctx, rawInputID)
	if err != nil {
		log.Fatalf("get audit record: %v", err)
	}
	if raw == nil {
		log.Fatalf("raw input %s not found", lead.RawInputID)
	}

	if *showMeta {
		fmt.Printf("Lead:         %s\n", lead.Title)
		fmt.Printf("Audit ID:     %s\n", raw.ID)
		fmt.Printf("Source URL:   %s\n", raw.SourceURL)
		fmt.Printf("Collector:    %s\n", raw.CollectorName)
		fmt.Printf("Payload Type: %s\n", raw.PayloadType)
		fmt.Printf("Fetched At:   %s\n", raw.CreatedAt.Format(time.RFC3339))
		fmt.Printf("Hash:         %s\n", raw.Hash)
		return
	}

	if *savePath != "" {
		if err := os.WriteFile(*savePath, raw.Payload, 0644); err != nil {
			log.Fatalf("save file: %v", err)
		}
		fmt.Printf("Saved payload to %s\n", *savePath)
	} else {
		os.Stdout.Write(raw.Payload)
		fmt.Println()
	}
}

func handleAuditRetentionCommand(cfg *config.Config) {
	retentionCmd := flag.NewFlagSet("audit-retention", flag.ExitOnError)
	days := retentionCmd.Int("days", cfg.AuditRetentionDays, "delete unreferenced raw inputs older than this many days")

	args := flag.Args()[1:]
	if len(args) < 1 || args[0] != "purge" {
		fmt.Println("Usage: groupscout audit-retention purge [--days N]")
		os.Exit(1)
	}
	retentionCmd.Parse(args[1:])

	db, err := storage.Open(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer db.Close()

	if err := storage.Migrate(db, cfg.DatabaseURL); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	worker, err := auditretention.New(
		storage.NewAuditStoreWithDSN(db, cfg.DatabaseURL),
		auditretention.Policy{
			RetentionDays: *days,
			Interval:      24 * time.Hour,
		},
		logger.Log,
	)
	if err != nil {
		log.Fatalf("audit retention config: %v", err)
	}

	result, err := worker.PurgeOnce(context.Background())
	if err != nil {
		log.Fatalf("audit retention purge: %v", err)
	}

	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		log.Fatalf("encode result: %v", err)
	}
}
