package main

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/alvindcastro/groupscout/config"
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

	collectors := buildCollectors(cfg)
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

	for _, l := range leads {
		if err := leadStore.UpdateStatus(ctx, l.ID, "notified"); err != nil {
			logger.Log.Warn("failed to update status for lead", "id", l.ID, "error", err)
		}
	}
	return result, nil
}

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
