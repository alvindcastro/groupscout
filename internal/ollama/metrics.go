package ollama

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	CallsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "ollama_calls_total",
		Help: "Total number of Ollama API calls.",
	}, []string{"use_case", "status"})

	LatencyMS = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "ollama_latency_ms",
		Help:    "Latency of Ollama API calls in milliseconds.",
		Buckets: []float64{100, 500, 1000, 2500, 5000, 10000, 20000, 30000},
	}, []string{"use_case"})
)
