package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/alvindcastro/blockscout/config"
	"github.com/alvindcastro/blockscout/internal/collector"
	"github.com/alvindcastro/blockscout/internal/enrichment"
	"github.com/alvindcastro/blockscout/internal/notify"
	"github.com/alvindcastro/blockscout/internal/storage"
)

var runOnce = flag.Bool("run-once", false, "run the full collect→enrich→notify pipeline once and exit")

func main() {
	flag.Parse()
	log.SetFlags(0)
	log.SetPrefix("[blockscout] ")

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	db, err := storage.Open(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer db.Close()

	if err := storage.Migrate(db); err != nil {
		log.Fatalf("migrate: %v", err)
	}
	log.Printf("DB ready at %s", cfg.DatabaseURL)

	if *runOnce {
		if cfg.ClaudeAPIKey == "" {
			log.Fatal("CLAUDE_API_KEY is not set")
		}
		if cfg.SlackWebhookURL == "" {
			log.Fatal("SLACK_WEBHOOK_URL is not set")
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()

		if err := runPipeline(ctx, cfg, db); err != nil {
			log.Fatalf("pipeline: %v", err)
		}
		return
	}

	// Server mode
	if cfg.APIToken == "" {
		log.Println("warn: API_TOKEN not set; server will be insecure (all requests allowed)")
	}

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

		log.Println("pipeline triggered via HTTP /run")
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
			log.Printf("error: pipeline: %v", err)
			http.Error(w, fmt.Sprintf("Pipeline failed: %v", err), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "Pipeline completed successfully")
	})

	addr := ":" + strconv.Itoa(cfg.Port)
	log.Printf("server listening on %s", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("server: %v", err)
	}
}

func runPipeline(ctx context.Context, cfg *config.Config, db *sql.DB) error {
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
		log.Printf("VCC collector enabled: %s", cfg.VCCURL)
		collectors = append(collectors, vc)
	} else {
		log.Println("VCC collector disabled")
	}
	if cfg.BCBidEnabled {
		bc := collector.NewBCBidCollector(strings.Split(cfg.BCBidRSSURL, ","))
		bc.Verbose = true
		collectors = append(collectors, bc)
	}

	e := enrichment.NewEnricher(collectors, rawStore, leadStore, claude, scorer)
	e.Verbose = true

	log.Println("running pipeline...")
	n, err := e.Run(ctx)
	if err != nil {
		return fmt.Errorf("enricher run: %w", err)
	}
	log.Printf("enrichment complete: %d new leads", n)

	leads, err := leadStore.ListNew(ctx)
	if err != nil {
		return fmt.Errorf("list leads: %w", err)
	}

	if len(leads) == 0 {
		log.Println("no new leads to notify")
		return nil
	}

	notifier := notify.NewSlackNotifier(cfg.SlackWebhookURL)
	if err := notifier.Send(ctx, leads); err != nil {
		return fmt.Errorf("slack notify: %w", err)
	}
	log.Printf("sent %d leads to Slack", len(leads))

	// Mark leads as notified so they don't re-appear in the next run's digest.
	for _, l := range leads {
		if err := leadStore.UpdateStatus(ctx, l.ID, "notified"); err != nil {
			log.Printf("warn: update status for lead %s: %v", l.ID, err)
		}
	}
	return nil
}
