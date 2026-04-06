package main

import (
	"context"
	"database/sql"
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

	"github.com/alvindcastro/groupscout/config"
	"github.com/alvindcastro/groupscout/internal/collector"
	"github.com/alvindcastro/groupscout/internal/enrichment"
	"github.com/alvindcastro/groupscout/internal/logger"
	"github.com/alvindcastro/groupscout/internal/notify"
	"github.com/alvindcastro/groupscout/internal/storage"
)

var runOnce = flag.Bool("run-once", false, "run the full collect→enrich→notify pipeline once and exit")

func main() {
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	logger.Init(cfg.JSONLog, cfg.SentryDSN)
	l := logger.Log

	if cfg.SentryDSN != "" {
		defer sentry.Flush(2 * time.Second)
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
		if cfg.ClaudeAPIKey == "" {
			l.Error("CLAUDE_API_KEY is not set")
			os.Exit(1)
		}
		if cfg.SlackWebhookURL == "" {
			l.Error("SLACK_WEBHOOK_URL is not set")
			os.Exit(1)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()

		if err := runPipeline(ctx, cfg, db); err != nil {
			l.Error("pipeline failed", "error", err)
			sentry.CaptureException(err)
			os.Exit(1)
		}
		return
	}

	// Server mode
	if cfg.APIToken == "" {
		l.Warn("API_TOKEN not set; server will be insecure (all requests allowed)")
	}

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		if err := db.Ping(); err != nil {
			l.Error("health check failed: DB ping", "error", err)
			http.Error(w, "Service Unavailable: Database Ping Failed", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "OK")
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
			BCBidRawInput string `json:"bcbid_raw_input"`
		}
		var req runRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.BCBidRawInput != "" {
			ctx = context.WithValue(ctx, "bcbid_raw_input", req.BCBidRawInput)
		}

		if err := runPipeline(ctx, cfg, db); err != nil {
			l.Error("pipeline failed", "error", err)
			sentry.CaptureException(err)
			http.Error(w, fmt.Sprintf("Pipeline failed: %v", err), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "Pipeline completed successfully")
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

		leadStore := storage.NewLeadStore(db)
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

		emailNotifier := notify.NewEmailNotifier(cfg.SendGridAPIKey)
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
	http.HandleFunc("/n8n/webhook", func(w http.ResponseWriter, r *http.Request) {
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

		var l storage.Lead
		if err := json.NewDecoder(r.Body).Decode(&l); err != nil {
			logger.Log.Error("failed to decode n8n lead", "error", err)
			http.Error(w, "Invalid JSON body", http.StatusBadRequest)
			return
		}

		if l.Source == "" {
			l.Source = "n8n"
		}
		if l.ID == "" {
			l.ID = storage.NewUUID()
		}

		leadStore := storage.NewLeadStore(db)
		if err := leadStore.Insert(context.Background(), &l); err != nil {
			logger.Log.Error("failed to insert n8n lead", "error", err)
			http.Error(w, fmt.Sprintf("Failed to store lead: %v", err), http.StatusInternalServerError)
			return
		}

		logger.Log.Info("lead received from n8n", "source", l.Source, "title", l.Title)

		// Optionally notify Slack immediately
		notifier := notify.NewSlackNotifier(cfg.SlackWebhookURL)
		if err := notifier.Send(context.Background(), []storage.Lead{l}); err != nil {
			logger.Log.Warn("failed to notify Slack for n8n lead", "error", err)
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"status": "success", "id": l.ID})
	})

	addr := ":" + strconv.Itoa(cfg.Port)
	l.Info("server listening", "addr", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		l.Error("server failed", "error", err)
		os.Exit(1)
	}
}

func runPipeline(ctx context.Context, cfg *config.Config, db *sql.DB) error {
	l := logger.Log
	rawStore := storage.NewRawProjectStore(db)
	leadStore := storage.NewLeadStore(db)

	claude := enrichment.NewClaudeEnricher(cfg.ClaudeAPIKey)
	scorer := enrichment.NewScorer(cfg.EnrichmentThreshold)
	rc := collector.NewRichmondCollector()
	rc.MinValue = cfg.MinPermitValueCAD
	rc.Verbose = true
	collectors := []collector.Collector{rc}

	if cfg.DeltaPermitsURL != "" {
		dc := collector.NewDeltaCollector(cfg.DeltaPermitsURL)
		dc.MinValue = cfg.MinPermitValueCAD
		dc.Verbose = true
		collectors = append(collectors, dc)
	}

	if cfg.CreativeBCEnabled {
		cbc := collector.NewCreativeBCCollector(cfg.CreativeBCURL)
		cbc.Verbose = true
		collectors = append(collectors, cbc)
	}

	if cfg.VCCEnabled {
		vc := collector.NewVCCCollector(cfg.VCCURL)
		vc.Verbose = true
		l.Info("VCC collector enabled", "url", cfg.VCCURL)
		collectors = append(collectors, vc)
	} else {
		l.Info("VCC collector disabled")
	}
	if cfg.BCBidEnabled {
		bc := collector.NewBCBidCollector(strings.Split(cfg.BCBidRSSURL, ","))
		bc.Verbose = true
		collectors = append(collectors, bc)
	}

	l.Debug("config news", "enabled", cfg.NewsEnabled, "url", cfg.NewsRSSURL)

	if cfg.NewsEnabled {
		nc := collector.NewNewsCollector(cfg.NewsRSSURL)
		nc.Verbose = true
		collectors = append(collectors, nc)
	}

	if cfg.AnnouncementsEnabled {
		ac := collector.NewAnnouncementsCollector()
		ac.Verbose = true
		collectors = append(collectors, ac)
	}

	if cfg.EventbriteEnabled {
		ec := collector.NewEventbriteCollector(cfg.EventbriteURL)
		ec.Verbose = true
		collectors = append(collectors, ec)
	}

	var names []string
	for _, c := range collectors {
		names = append(names, c.Name())
	}
	l.Info("active collectors", "count", len(names), "names", names)

	e := enrichment.NewEnricher(collectors, rawStore, leadStore, claude, scorer, cfg.PriorityAlertThreshold)
	e.Verbose = true

	l.Info("running pipeline...")
	n, err := e.Run(ctx)
	if err != nil {
		return fmt.Errorf("enricher run: %w", err)
	}
	l.Info("enrichment complete", "new_leads", n)

	leads, err := leadStore.ListNew(ctx)
	if err != nil {
		return fmt.Errorf("list leads: %w", err)
	}

	if len(leads) == 0 {
		l.Info("no new leads to notify")
		return nil
	}

	notifier := notify.NewSlackNotifier(cfg.SlackWebhookURL)
	if err := notifier.Send(ctx, leads); err != nil {
		return fmt.Errorf("slack notify: %w", err)
	}
	l.Info("sent leads to Slack", "count", len(leads))

	// Mark leads as notified so they don't re-appear in the next run's digest.
	for _, l := range leads {
		if err := leadStore.UpdateStatus(ctx, l.ID, "notified"); err != nil {
			logger.Log.Warn("failed to update status for lead", "id", l.ID, "error", err)
		}
	}
	return nil
}
