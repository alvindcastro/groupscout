package main

import (
	"context"
	"flag"
	"log"
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

	if !*runOnce {
		log.Println("nothing to do — pass --run-once to run the pipeline")
		return
	}

	if cfg.ClaudeAPIKey == "" {
		log.Fatal("CLAUDE_API_KEY is not set")
	}
	if cfg.SlackWebhookURL == "" {
		log.Fatal("SLACK_WEBHOOK_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	rawStore := storage.NewRawProjectStore(db)
	leadStore := storage.NewLeadStore(db)

	claude := enrichment.NewClaudeEnricher(cfg.ClaudeAPIKey)
	collectors := []collector.Collector{
		collector.NewRichmondCollector(),
	}

	e := enrichment.NewEnricher(collectors, rawStore, leadStore, claude)
	e.Verbose = true

	log.Println("running pipeline...")
	n, err := e.Run(ctx)
	if err != nil {
		log.Fatalf("pipeline: %v", err)
	}
	log.Printf("enrichment complete: %d new leads", n)

	leads, err := leadStore.ListNew(ctx)
	if err != nil {
		log.Fatalf("list leads: %v", err)
	}

	if len(leads) == 0 {
		log.Println("no new leads to notify")
		return
	}

	notifier := notify.NewSlackNotifier(cfg.SlackWebhookURL)
	if err := notifier.Send(ctx, leads); err != nil {
		log.Fatalf("slack: %v", err)
	}
	log.Printf("sent %d leads to Slack", len(leads))
}
