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
	Status            string   `json:"status"`
	Message           string   `json:"message,omitempty"`
	NewLeads          int      `json:"new_leads"`
	NotifiedLeads     int      `json:"notified_leads"`
	DeliveryStatus    string   `json:"delivery_status,omitempty"`
	DeliveredLeadID   string   `json:"delivered_lead_id,omitempty"`
	DeliveredLeadIDs  []string `json:"delivered_lead_ids,omitempty"`
	IdempotencyKey    string   `json:"idempotency_key,omitempty"`
	ScheduleKey       string   `json:"schedule_key,omitempty"`
	DeliveryDuplicate bool     `json:"delivery_duplicate,omitempty"`
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
			result.Message = cadenceMessage("locked", result.ScheduleKey, 0)
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
			result.NotifiedLeads = deliveredCount(existing.Result)
			result.DeliveredLeadID = existing.LeadID
			result.DeliveryStatus = "duplicate"
			result.DeliveryDuplicate = true
			result.Message = cadenceMessage("duplicate", result.ScheduleKey, result.NotifiedLeads)
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
		delivery, deliveredLeads, err := deliverGuaranteedLeads(ctx, leadStore, deliveryStore, leadnotify.NewSlackNotifier(cfg.SlackWebhookURL, cfg.BaseURL), leadnotify.NewEmailNotifier(cfg.ResendAPIKey, cfg.EmailFrom), cfg.LeadNotifyEmails, opts)
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
		result.DeliveredLeadIDs = leadIDs(deliveredLeads)
		if len(result.DeliveredLeadIDs) > 0 {
			result.DeliveredLeadID = result.DeliveredLeadIDs[0]
		} else {
			result.DeliveredLeadID = delivery.LeadID
		}
		if delivery.Status == "sent" {
			result.NotifiedLeads = len(deliveredLeads)
		}
		result.Message = cadenceMessage(delivery.Status, result.ScheduleKey, result.NotifiedLeads)
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

	// Email is a best-effort copy of the Slack digest; a failure here must not
	// fail the run or block marking leads notified, since Slack already delivered.
	emailNotifier := leadnotify.NewEmailNotifier(cfg.ResendAPIKey, cfg.EmailFrom)
	if err := emailNotifier.SendLeads(ctx, cfg.LeadNotifyEmails, leads); err != nil {
		l.Warn("failed to email leads", "error", err, "recipients", cfg.LeadNotifyEmails)
	} else {
		l.Info("sent leads via email", "count", len(leads), "recipients", cfg.LeadNotifyEmails)
	}

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

// cadenceMessage returns a human-readable, ready-to-post summary of a cadence
// run so the Slack/n8n step can post it verbatim instead of stitching together
// raw flags like run_ok/no_lead/status.
func cadenceMessage(status, scheduleKey string, count int) string {
	day := cadenceDayLabel(scheduleKey)
	switch status {
	case "sent":
		if count == 1 {
			return fmt.Sprintf("📬 GroupScout sent 1 new lead for %s.", day)
		}
		return fmt.Sprintf("📬 GroupScout sent %d new leads for %s.", count, day)
	case "duplicate":
		return fmt.Sprintf("✅ GroupScout already delivered today's leads for %s — nothing new to resend.", day)
	case "locked":
		return fmt.Sprintf("⏳ GroupScout cadence for %s is already running; skipped this duplicate trigger.", day)
	case "no_eligible_lead":
		return fmt.Sprintf("📭 No new leads for %s. GroupScout ran cleanly — every current lead has already been delivered, so there was nothing fresh to send.", day)
	default:
		return fmt.Sprintf("GroupScout cadence finished for %s (status: %s).", day, status)
	}
}

// cadenceDayLabel turns a schedule key like "lead-cadence:2026-06-25:thursday"
// into a friendly "Thursday, 2026-06-25". Falls back to the raw key.
func cadenceDayLabel(scheduleKey string) string {
	parts := strings.Split(scheduleKey, ":")
	if len(parts) >= 3 {
		date := parts[len(parts)-2]
		weekday := parts[len(parts)-1]
		if weekday != "" {
			weekday = strings.ToUpper(weekday[:1]) + weekday[1:]
			return fmt.Sprintf("%s, %s", weekday, date)
		}
		return date
	}
	if scheduleKey == "" {
		return "today"
	}
	return scheduleKey
}

const maxCadenceDeliveryLeads = 100

func deliverGuaranteedLeads(ctx context.Context, leadStore storage.LeadStore, deliveryStore storage.DeliveryStore, notifier leadnotify.Notifier, emailNotifier *leadnotify.EmailNotifier, recipients []string, opts PipelineRunOptions) (storage.LeadDelivery, []storage.Lead, error) {
	candidates, err := leadStore.ListDeliveryCandidates(ctx, maxCadenceDeliveryLeads)
	if err != nil {
		return storage.LeadDelivery{}, nil, fmt.Errorf("list delivery candidates: %w", err)
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
			return delivery, nil, fmt.Errorf("record no eligible lead: %w", err)
		}

		// Mirror the Slack cadence message to email so recipients stay in sync
		// even on days with no leads. Best-effort: a failure only warns.
		if emailNotifier != nil {
			msg := cadenceMessage(delivery.Status, opts.ScheduleKey, 0)
			if err := emailNotifier.SendNotice(ctx, recipients, "GroupScout: no new leads today", msg); err != nil {
				logger.Log.Warn("failed to email no-lead notice", "error", err, "recipients", recipients)
			} else {
				logger.Log.Info("emailed no-lead notice", "recipients", recipients)
			}
		}
		return delivery, nil, nil
	}

	if err := notifier.Send(ctx, candidates); err != nil {
		return storage.LeadDelivery{Status: "failed"}, nil, err
	}
	for _, lead := range candidates {
		if lead.Status != "new" {
			continue
		}
		if err := leadStore.UpdateStatus(ctx, lead.ID, "notified"); err != nil {
			return storage.LeadDelivery{LeadID: lead.ID, Status: "failed"}, candidates, fmt.Errorf("mark lead notified: %w", err)
		}
	}

	// Email is a best-effort copy of the delivered lead; Slack remains the
	// source of truth for delivery status, so an email failure only warns.
	if emailNotifier != nil {
		if err := emailNotifier.SendLeads(ctx, recipients, candidates); err != nil {
			logger.Log.Warn("failed to email guaranteed leads", "error", err, "count", len(candidates), "recipients", recipients)
		} else {
			logger.Log.Info("emailed guaranteed leads", "count", len(candidates), "recipients", recipients)
		}
	}

	sentAt := time.Now().UTC()
	for _, lead := range candidates {
		leadDelivery := storage.LeadDelivery{
			IdempotencyKey: opts.IdempotencyKey + ":" + lead.ID,
			ScheduleKey:    opts.ScheduleKey,
			LeadID:         lead.ID,
			Channel:        "slack",
			Status:         "sent",
			Result:         "sent lead to slack",
			SentAt:         sentAt,
		}
		if err := deliveryStore.UpsertResult(ctx, leadDelivery); err != nil {
			return leadDelivery, candidates, fmt.Errorf("record sent lead delivery: %w", err)
		}
	}

	delivery := storage.LeadDelivery{
		IdempotencyKey: opts.IdempotencyKey,
		ScheduleKey:    opts.ScheduleKey,
		Channel:        "slack",
		Status:         "sent",
		Result:         fmt.Sprintf("sent %d leads to slack", len(candidates)),
		SentAt:         sentAt,
	}
	if err := deliveryStore.UpsertResult(ctx, delivery); err != nil {
		return delivery, candidates, fmt.Errorf("record sent delivery: %w", err)
	}
	return delivery, candidates, nil
}

func leadIDs(leads []storage.Lead) []string {
	if len(leads) == 0 {
		return nil
	}
	ids := make([]string, 0, len(leads))
	for _, lead := range leads {
		ids = append(ids, lead.ID)
	}
	return ids
}

func deliveredCount(result string) int {
	var n int
	if _, err := fmt.Sscanf(result, "sent %d leads", &n); err == nil && n > 0 {
		return n
	}
	return 1
}
