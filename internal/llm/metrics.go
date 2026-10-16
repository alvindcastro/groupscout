package llm

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metric names are referenced by the Grafana alert rules in
// deploy/config/grafana/provisioning/alerting/llm-rules.yml.
var (
	requestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "groupscout_llm_requests_total",
		Help: "LLM calls by provider, model, use case and result (ok or an error kind).",
	}, []string{"provider", "model", "use_case", "result"})

	requestSeconds = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "groupscout_llm_request_duration_seconds",
		Help:    "LLM call latency by provider and use case.",
		Buckets: []float64{0.5, 1, 2, 5, 10, 20, 40, 80, 160},
	}, []string{"provider", "use_case"})

	providerFailuresTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "groupscout_llm_provider_failures_total",
		Help: "Requests a provider gave up on after its retries, so the chain moved to the next provider or failed.",
	}, []string{"provider", "use_case", "reason"})

	exhaustedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "groupscout_llm_exhausted_total",
		Help: "Requests that failed on every configured provider.",
	}, []string{"use_case"})

	providerAvailable = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "groupscout_llm_provider_available",
		Help: "0 once the chain parks a provider after an account error or repeated transient errors, 1 again after its next success.",
	}, []string{"provider"})
)
