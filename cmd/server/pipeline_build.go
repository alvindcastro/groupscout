package main

import (
	"database/sql"
	"strings"
	"time"

	"github.com/alvindcastro/groupscout/config"
	"github.com/alvindcastro/groupscout/internal/collector"
	"github.com/alvindcastro/groupscout/internal/collector/events"
	"github.com/alvindcastro/groupscout/internal/collector/news"
	"github.com/alvindcastro/groupscout/internal/collector/permits"
	"github.com/alvindcastro/groupscout/internal/enrichment"
	"github.com/alvindcastro/groupscout/internal/logger"
	"github.com/alvindcastro/groupscout/internal/ollama"
	"github.com/alvindcastro/groupscout/internal/storage"
)

func buildCollectors(cfg *config.Config) []collector.Collector {
	l := logger.Log
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
	return collectors
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
