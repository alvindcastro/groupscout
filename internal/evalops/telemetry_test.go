package evalops

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestTraceEventRedactsSensitivePayloadsForRuntimeTelemetry(t *testing.T) {
	rawSource := strings.Repeat("raw commercial permit text ", 80)
	for _, stage := range []TraceStage{
		TraceStageCollector,
		TraceStageEnrichment,
		TraceStageLLM,
		TraceStageNotification,
		TraceStageAlert,
	} {
		t.Run(string(stage), func(t *testing.T) {
			event := NewTraceEvent(stage, "trace-gq4-001", "case-gq4-001", "runtime_check", "failed", map[string]any{
				"api_key":         "API_KEY=sk-live-do-not-use",
				"slack_webhook":   "https://hooks.slack.com/services/T000/B000/secret",
				"contact_email":   "alex.fixture@example.invalid",
				"contact_phone":   "604-555-0199",
				"raw_pii":         "SIN 123-456-789",
				"raw_source_text": rawSource,
				"source_system":   "richmond_permits",
			})

			safe := event.Safe()
			encoded, err := json.Marshal(safe)
			if err != nil {
				t.Fatalf("marshal safe event: %v", err)
			}
			payload := string(encoded)
			for _, leaked := range []string{
				"sk-live-do-not-use",
				"hooks.slack.com/services",
				"alex.fixture@example.invalid",
				"604-555-0199",
				"SIN 123-456-789",
				rawSource,
			} {
				if strings.Contains(payload, leaked) {
					t.Fatalf("safe trace payload leaked %q: %s", leaked, payload)
				}
			}
			if safe.Stage != stage || safe.TraceID != "trace-gq4-001" || safe.CaseID != "case-gq4-001" {
				t.Fatalf("safe event lost identifiers: %+v", safe)
			}
			if got := safe.Attributes["source_system"]; got != "richmond_permits" {
				t.Fatalf("source_system attribute = %q, want preserved safe value", got)
			}
			if got := safe.Attributes["raw_source_text"]; !strings.Contains(got, "[TRUNCATED]") {
				t.Fatalf("raw_source_text = %q, want truncated marker", got)
			}
		})
	}
}

func TestTraceEventMapsSafeTelemetryAttributes(t *testing.T) {
	event := NewTraceEvent(TraceStageLLM, "trace-gq4-attrs", "case-gq4-attrs", "complete", "error", map[string]any{
		"provider": "ollama",
		"model":    "llama3.1:8b",
		"token":    "TOKEN=fixture-token-do-not-use",
	})

	attrs := event.TelemetryAttributes()
	for _, key := range []string{
		"groupscout.trace_id",
		"groupscout.case_id",
		"groupscout.stage",
		"groupscout.operation",
		"groupscout.status",
		"provider",
		"model",
	} {
		if attrs[key] == "" {
			t.Fatalf("TelemetryAttributes() missing %s in %#v", key, attrs)
		}
	}
	for _, leaked := range []string{"TOKEN=fixture-token-do-not-use", "fixture-token-do-not-use"} {
		if strings.Contains(strings.Join(mapValues(attrs), " "), leaked) {
			t.Fatalf("TelemetryAttributes() leaked %q: %#v", leaked, attrs)
		}
	}
}

func mapValues(values map[string]string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, value)
	}
	return out
}
