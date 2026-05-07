package evalops

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

func TestRuntimeMetricsRecordPipelineQualitySignals(t *testing.T) {
	registry := prometheus.NewRegistry()
	metrics, err := NewRuntimeMetrics(registry)
	if err != nil {
		t.Fatalf("NewRuntimeMetrics() error = %v", err)
	}

	metrics.IncCollectorFailure("richmond_permits", "parse_error")
	metrics.IncEnrichmentSkipped("delta_permits", "duplicate_raw_hash")
	metrics.IncLLMError("ollama", "timeout")
	metrics.IncAlertDecision("YVR", "priority", true)
	metrics.ObserveStageLatency(TraceStageEnrichment, "score_lead", 250*time.Millisecond)
	metrics.AddLLMTokens("ollama", "llama3.1:8b", "completion", 42)
	metrics.AddLLMCostCents("ollama", "llama3.1:8b", 3.5)

	families, err := registry.Gather()
	if err != nil {
		t.Fatalf("Gather() error = %v", err)
	}

	assertCounterValue(t, families, "groupscout_pipeline_collector_failures_total", 1)
	assertCounterValue(t, families, "groupscout_pipeline_enrichment_skipped_total", 1)
	assertCounterValue(t, families, "groupscout_pipeline_llm_errors_total", 1)
	assertCounterValue(t, families, "groupscout_pipeline_alert_decisions_total", 1)
	assertCounterValue(t, families, "groupscout_pipeline_llm_tokens_total", 42)
	assertCounterValue(t, families, "groupscout_pipeline_llm_cost_cents_total", 3.5)
	assertHistogramCount(t, families, "groupscout_pipeline_stage_latency_seconds", 1)
}

func TestRuntimeMetricsCanRegisterTwiceWithoutPanicking(t *testing.T) {
	registry := prometheus.NewRegistry()
	first, err := NewRuntimeMetrics(registry)
	if err != nil {
		t.Fatalf("first NewRuntimeMetrics() error = %v", err)
	}
	second, err := NewRuntimeMetrics(registry)
	if err != nil {
		t.Fatalf("second NewRuntimeMetrics() error = %v", err)
	}

	first.IncCollectorFailure("richmond_permits", "parse_error")
	second.IncCollectorFailure("richmond_permits", "parse_error")

	families, err := registry.Gather()
	if err != nil {
		t.Fatalf("Gather() error = %v", err)
	}
	assertCounterValue(t, families, "groupscout_pipeline_collector_failures_total", 2)
}

func assertCounterValue(t *testing.T, families []*dto.MetricFamily, name string, want float64) {
	t.Helper()
	family := metricFamily(families, name)
	if family == nil {
		t.Fatalf("missing metric family %s", name)
	}
	var got float64
	for _, metric := range family.GetMetric() {
		if metric.Counter != nil {
			got += metric.Counter.GetValue()
		}
	}
	if got != want {
		t.Fatalf("%s = %v, want %v", name, got, want)
	}
}

func assertHistogramCount(t *testing.T, families []*dto.MetricFamily, name string, want uint64) {
	t.Helper()
	family := metricFamily(families, name)
	if family == nil {
		t.Fatalf("missing metric family %s", name)
	}
	var got uint64
	for _, metric := range family.GetMetric() {
		if metric.Histogram != nil {
			got += metric.Histogram.GetSampleCount()
		}
	}
	if got != want {
		t.Fatalf("%s sample count = %d, want %d", name, got, want)
	}
}

func metricFamily(families []*dto.MetricFamily, name string) *dto.MetricFamily {
	for _, family := range families {
		if family.GetName() == name {
			return family
		}
	}
	return nil
}
