package main

import (
	"log"

	"github.com/alvindcastro/blockscout/config"
	"github.com/alvindcastro/blockscout/internal/storage"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	db, err := storage.Open(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("db open: %v", err)
	}
	defer db.Close()

	if err := storage.Migrate(db); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	log.Printf("blockscout: DB ready at %s", cfg.DatabaseURL)
	log.Println("blockscout: Phase 2 — collectors not yet wired")

	// Phase 2 wiring goes here:
	//   rawStore  := storage.NewRawProjectStore(db)
	//   leadStore := storage.NewLeadStore(db)
	//   collectors := []collector.Collector{ richmond.New(cfg) }
	//   enricher  := enrichment.New(cfg, rawStore, leadStore, collectors)
	//   enricher.RunOnce(context.Background())
}
