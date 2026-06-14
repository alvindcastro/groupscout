package evalops

import (
	"github.com/prometheus/client_golang/prometheus"
)

// EvalMetrics holds Prometheus metric instruments for the EvalOps quality layer.
// Construct with NewEvalMetrics and register with a dedicated (non-default) registry
// in tests to avoid duplicate registration panics.
type EvalMetrics struct {
	CollectorFailures   prometheus.Counter
	EnrichmentSkipped   prometheus.Counter
	LLMErrors           prometheus.Counter
	AlertDecisions      *prometheus.CounterVec
	EnrichmentLatencyMS prometheus.Histogram
	LLMLatencyMS        prometheus.Histogram
	TokensUsed          prometheus.Counter
	CostEstimateUSD     prometheus.Counter
}

// NewEvalMetrics creates a new EvalMetrics and registers all instruments with reg.
// Pass a fresh prometheus.NewRegistry() in tests to avoid panics.
func NewEvalMetrics(reg prometheus.Registerer) *EvalMetrics {
	m := &EvalMetrics{
		CollectorFailures: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "evalops_collector_failures_total",
			Help: "Total number of collector stage failures.",
		}),
		EnrichmentSkipped: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "evalops_enrichment_skipped_total",
			Help: "Total number of leads skipped before enrichment (below threshold or duplicate).",
		}),
		LLMErrors: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "evalops_llm_errors_total",
			Help: "Total number of LLM call errors during enrichment.",
		}),
		AlertDecisions: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "evalops_alert_decisions_total",
			Help: "Total number of alert decisions by state.",
		}, []string{"state"}),
		EnrichmentLatencyMS: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "evalops_enrichment_latency_ms",
			Help:    "Latency in milliseconds for enrichment pipeline stages.",
			Buckets: []float64{50, 100, 250, 500, 1000, 2500, 5000, 10000},
		}),
		LLMLatencyMS: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "evalops_llm_latency_ms",
			Help:    "Latency in milliseconds for LLM API calls.",
			Buckets: []float64{100, 250, 500, 1000, 2500, 5000, 15000, 30000},
		}),
		TokensUsed: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "evalops_tokens_used_total",
			Help: "Total number of LLM tokens consumed (estimated).",
		}),
		CostEstimateUSD: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "evalops_cost_estimate_usd_total",
			Help: "Estimated total cost in USD for LLM API calls.",
		}),
	}

	reg.MustRegister(
		m.CollectorFailures,
		m.EnrichmentSkipped,
		m.LLMErrors,
		m.AlertDecisions,
		m.EnrichmentLatencyMS,
		m.LLMLatencyMS,
		m.TokensUsed,
		m.CostEstimateUSD,
	)

	return m
}

// RecordCollectorFailure increments the collector failure counter.
func (m *EvalMetrics) RecordCollectorFailure() {
	m.CollectorFailures.Inc()
}

// RecordEnrichmentSkipped increments the enrichment-skipped counter.
func (m *EvalMetrics) RecordEnrichmentSkipped() {
	m.EnrichmentSkipped.Inc()
}

// RecordLLMError increments the LLM error counter.
func (m *EvalMetrics) RecordLLMError() {
	m.LLMErrors.Inc()
}

// RecordAlertDecision increments the alert decision counter for the given state.
func (m *EvalMetrics) RecordAlertDecision(state string) {
	m.AlertDecisions.WithLabelValues(state).Inc()
}

// ObserveEnrichmentLatency records an enrichment latency observation in ms.
func (m *EvalMetrics) ObserveEnrichmentLatency(ms float64) {
	m.EnrichmentLatencyMS.Observe(ms)
}

// ObserveLLMLatency records an LLM call latency observation in ms.
func (m *EvalMetrics) ObserveLLMLatency(ms float64) {
	m.LLMLatencyMS.Observe(ms)
}

// RecordTokensUsed adds to the total token count.
func (m *EvalMetrics) RecordTokensUsed(n float64) {
	m.TokensUsed.Add(n)
}

// RecordCostEstimate adds to the total cost estimate in USD.
func (m *EvalMetrics) RecordCostEstimate(usd float64) {
	m.CostEstimateUSD.Add(usd)
}
