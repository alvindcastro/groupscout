package evalops

import (
	"errors"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type RuntimeMetrics struct {
	collectorFailures *prometheus.CounterVec
	enrichmentSkipped *prometheus.CounterVec
	llmErrors         *prometheus.CounterVec
	alertDecisions    *prometheus.CounterVec
	stageLatency      *prometheus.HistogramVec
	llmTokens         *prometheus.CounterVec
	llmCostCents      *prometheus.CounterVec
}

func NewRuntimeMetrics(registry *prometheus.Registry) (*RuntimeMetrics, error) {
	if registry == nil {
		registry = prometheus.NewRegistry()
	}
	metrics := &RuntimeMetrics{
		collectorFailures: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "groupscout_pipeline_collector_failures_total",
			Help: "Total collector failures observed by GroupScout quality telemetry.",
		}, []string{"collector", "reason"}),
		enrichmentSkipped: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "groupscout_pipeline_enrichment_skipped_total",
			Help: "Total enrichments skipped by source and reason.",
		}, []string{"source", "reason"}),
		llmErrors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "groupscout_pipeline_llm_errors_total",
			Help: "Total LLM/provider errors by provider and reason.",
		}, []string{"provider", "reason"}),
		alertDecisions: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "groupscout_pipeline_alert_decisions_total",
			Help: "Total alert decisions by airport, state, and priority.",
		}, []string{"airport", "state", "priority"}),
		stageLatency: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "groupscout_pipeline_stage_latency_seconds",
			Help:    "Runtime stage latency in seconds.",
			Buckets: []float64{0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30},
		}, []string{"stage", "operation"}),
		llmTokens: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "groupscout_pipeline_llm_tokens_total",
			Help: "Total estimated LLM tokens by provider, model, and token kind.",
		}, []string{"provider", "model", "kind"}),
		llmCostCents: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "groupscout_pipeline_llm_cost_cents_total",
			Help: "Total estimated LLM cost in cents by provider and model.",
		}, []string{"provider", "model"}),
	}
	var err error
	if metrics.collectorFailures, err = registerCounterVec(registry, metrics.collectorFailures); err != nil {
		return nil, err
	}
	if metrics.enrichmentSkipped, err = registerCounterVec(registry, metrics.enrichmentSkipped); err != nil {
		return nil, err
	}
	if metrics.llmErrors, err = registerCounterVec(registry, metrics.llmErrors); err != nil {
		return nil, err
	}
	if metrics.alertDecisions, err = registerCounterVec(registry, metrics.alertDecisions); err != nil {
		return nil, err
	}
	if metrics.stageLatency, err = registerHistogramVec(registry, metrics.stageLatency); err != nil {
		return nil, err
	}
	if metrics.llmTokens, err = registerCounterVec(registry, metrics.llmTokens); err != nil {
		return nil, err
	}
	if metrics.llmCostCents, err = registerCounterVec(registry, metrics.llmCostCents); err != nil {
		return nil, err
	}
	return metrics, nil
}

func registerCounterVec(registry *prometheus.Registry, collector *prometheus.CounterVec) (*prometheus.CounterVec, error) {
	if err := registry.Register(collector); err != nil {
		var alreadyRegistered prometheus.AlreadyRegisteredError
		if errors.As(err, &alreadyRegistered) {
			existing, ok := alreadyRegistered.ExistingCollector.(*prometheus.CounterVec)
			if !ok {
				return nil, err
			}
			return existing, nil
		}
		return nil, err
	}
	return collector, nil
}

func registerHistogramVec(registry *prometheus.Registry, collector *prometheus.HistogramVec) (*prometheus.HistogramVec, error) {
	if err := registry.Register(collector); err != nil {
		var alreadyRegistered prometheus.AlreadyRegisteredError
		if errors.As(err, &alreadyRegistered) {
			existing, ok := alreadyRegistered.ExistingCollector.(*prometheus.HistogramVec)
			if !ok {
				return nil, err
			}
			return existing, nil
		}
		return nil, err
	}
	return collector, nil
}

func (m *RuntimeMetrics) IncCollectorFailure(collector, reason string) {
	m.collectorFailures.WithLabelValues(collector, reason).Inc()
}

func (m *RuntimeMetrics) IncEnrichmentSkipped(source, reason string) {
	m.enrichmentSkipped.WithLabelValues(source, reason).Inc()
}

func (m *RuntimeMetrics) IncLLMError(provider, reason string) {
	m.llmErrors.WithLabelValues(provider, reason).Inc()
}

func (m *RuntimeMetrics) IncAlertDecision(airport, state string, priority bool) {
	m.alertDecisions.WithLabelValues(airport, state, boolLabel(priority)).Inc()
}

func (m *RuntimeMetrics) ObserveStageLatency(stage TraceStage, operation string, duration time.Duration) {
	m.stageLatency.WithLabelValues(string(stage), operation).Observe(duration.Seconds())
}

func (m *RuntimeMetrics) AddLLMTokens(provider, model, kind string, tokens float64) {
	if tokens <= 0 {
		return
	}
	m.llmTokens.WithLabelValues(provider, model, kind).Add(tokens)
}

func (m *RuntimeMetrics) AddLLMCostCents(provider, model string, cents float64) {
	if cents <= 0 {
		return
	}
	m.llmCostCents.WithLabelValues(provider, model).Add(cents)
}

func boolLabel(value bool) string {
	if value {
		return "true"
	}
	return "false"
}
