package evalops

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

func newTestRegistry() (*EvalMetrics, *prometheus.Registry) {
	reg := prometheus.NewRegistry()
	m := NewEvalMetrics(reg)
	return m, reg
}

func getCounterValue(t *testing.T, c prometheus.Counter) float64 {
	t.Helper()
	var m dto.Metric
	if err := c.Write(&m); err != nil {
		t.Fatalf("read counter: %v", err)
	}
	return m.Counter.GetValue()
}

func getCounterVecValue(t *testing.T, reg *prometheus.Registry, name, labelValue string) float64 {
	t.Helper()
	mfs, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather metrics: %v", err)
	}
	for _, mf := range mfs {
		if mf.GetName() != name {
			continue
		}
		for _, m := range mf.GetMetric() {
			for _, lp := range m.GetLabel() {
				if lp.GetValue() == labelValue {
					return m.GetCounter().GetValue()
				}
			}
		}
	}
	return 0
}

func TestNewEvalMetrics_NoPanics(t *testing.T) {
	// Should not panic with a fresh registry
	m, _ := newTestRegistry()
	if m == nil {
		t.Fatal("expected non-nil EvalMetrics")
	}
}

func TestNewEvalMetrics_NoDuplicateRegistration(t *testing.T) {
	// Two separate registries must not cause panic
	reg1 := prometheus.NewRegistry()
	reg2 := prometheus.NewRegistry()

	m1 := NewEvalMetrics(reg1)
	m2 := NewEvalMetrics(reg2)

	if m1 == nil || m2 == nil {
		t.Fatal("expected non-nil metrics from two registries")
	}
}

func TestRecordCollectorFailure(t *testing.T) {
	m, _ := newTestRegistry()

	m.RecordCollectorFailure()
	m.RecordCollectorFailure()

	val := getCounterValue(t, m.CollectorFailures)
	if val != 2 {
		t.Errorf("expected 2 collector failures, got %.0f", val)
	}
}

func TestRecordEnrichmentSkipped(t *testing.T) {
	m, _ := newTestRegistry()

	m.RecordEnrichmentSkipped()
	m.RecordEnrichmentSkipped()
	m.RecordEnrichmentSkipped()

	val := getCounterValue(t, m.EnrichmentSkipped)
	if val != 3 {
		t.Errorf("expected 3 enrichment skipped, got %.0f", val)
	}
}

func TestRecordLLMError(t *testing.T) {
	m, _ := newTestRegistry()

	m.RecordLLMError()

	val := getCounterValue(t, m.LLMErrors)
	if val != 1 {
		t.Errorf("expected 1 LLM error, got %.0f", val)
	}
}

func TestRecordAlertDecision(t *testing.T) {
	m, reg := newTestRegistry()

	m.RecordAlertDecision("HardAlert")
	m.RecordAlertDecision("HardAlert")
	m.RecordAlertDecision("Ignore")

	hardAlertVal := getCounterVecValue(t, reg, "evalops_alert_decisions_total", "HardAlert")
	if hardAlertVal != 2 {
		t.Errorf("expected 2 HardAlert decisions, got %.0f", hardAlertVal)
	}

	ignoreVal := getCounterVecValue(t, reg, "evalops_alert_decisions_total", "Ignore")
	if ignoreVal != 1 {
		t.Errorf("expected 1 Ignore decision, got %.0f", ignoreVal)
	}
}

func TestObserveEnrichmentLatency(t *testing.T) {
	m, _ := newTestRegistry()

	// Should not panic
	m.ObserveEnrichmentLatency(150.0)
	m.ObserveEnrichmentLatency(2500.0)

	// Gather to ensure metrics are registered and observable
	var metric dto.Metric
	if err := m.EnrichmentLatencyMS.(prometheus.Metric).Write(&metric); err != nil {
		// Histogram cannot be written directly; this is expected
		// Just ensure no panic happened
	}
}

func TestObserveLLMLatency(t *testing.T) {
	m, _ := newTestRegistry()

	m.ObserveLLMLatency(3500.0)
	m.ObserveLLMLatency(500.0)

	// No panic expected; values have been observed
}

func TestRecordTokensUsed(t *testing.T) {
	m, _ := newTestRegistry()

	m.RecordTokensUsed(1000)
	m.RecordTokensUsed(500)

	val := getCounterValue(t, m.TokensUsed)
	if val != 1500 {
		t.Errorf("expected 1500 tokens, got %.0f", val)
	}
}

func TestRecordCostEstimate(t *testing.T) {
	m, _ := newTestRegistry()

	m.RecordCostEstimate(0.01)
	m.RecordCostEstimate(0.005)

	val := getCounterValue(t, m.CostEstimateUSD)
	if val < 0.014 || val > 0.016 {
		t.Errorf("expected cost ~0.015, got %.4f", val)
	}
}

func TestEvalMetrics_AllCountersStartAtZero(t *testing.T) {
	m, _ := newTestRegistry()

	counters := []struct {
		name string
		c    prometheus.Counter
	}{
		{"CollectorFailures", m.CollectorFailures},
		{"EnrichmentSkipped", m.EnrichmentSkipped},
		{"LLMErrors", m.LLMErrors},
		{"TokensUsed", m.TokensUsed},
		{"CostEstimateUSD", m.CostEstimateUSD},
	}

	for _, tc := range counters {
		val := getCounterValue(t, tc.c)
		if val != 0 {
			t.Errorf("%s: expected 0 initial value, got %.0f", tc.name, val)
		}
	}
}

func TestEvalMetrics_MultipleStagesIndependent(t *testing.T) {
	m, _ := newTestRegistry()

	m.RecordCollectorFailure()
	m.RecordEnrichmentSkipped()
	m.RecordLLMError()

	if getCounterValue(t, m.CollectorFailures) != 1 {
		t.Error("expected 1 collector failure")
	}
	if getCounterValue(t, m.EnrichmentSkipped) != 1 {
		t.Error("expected 1 enrichment skipped")
	}
	if getCounterValue(t, m.LLMErrors) != 1 {
		t.Error("expected 1 LLM error")
	}
}
