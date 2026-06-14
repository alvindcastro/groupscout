package evalops

import (
	"strings"
	"testing"
)

func TestRedactSensitive_APIKey(t *testing.T) {
	input := "using key sk-ant-api03-fake-key-value-here-xxxxxyz for enrichment"
	result := RedactSensitive(input)

	if strings.Contains(result, "sk-ant-api03") {
		t.Errorf("expected API key to be redacted, got: %q", result)
	}
	if !strings.Contains(result, "[REDACTED]") {
		t.Errorf("expected [REDACTED] marker, got: %q", result)
	}
}

func TestRedactSensitive_SlackWebhook(t *testing.T) {
	input := "sending to https://hooks.slack.com/services/T00000000/B00000000/XXXXXXXX"
	result := RedactSensitive(input)

	if strings.Contains(result, "hooks.slack.com") {
		t.Errorf("expected Slack webhook to be redacted, got: %q", result)
	}
	if !strings.Contains(result, "[REDACTED]") {
		t.Errorf("expected [REDACTED] marker, got: %q", result)
	}
}

func TestRedactSensitive_Email(t *testing.T) {
	input := "contact: jane.smith@example.com for details"
	result := RedactSensitive(input)

	if strings.Contains(result, "jane.smith@example.com") {
		t.Errorf("expected email to be redacted, got: %q", result)
	}
	if !strings.Contains(result, "[REDACTED]") {
		t.Errorf("expected [REDACTED] marker, got: %q", result)
	}
}

func TestRedactSensitive_PhoneNumber(t *testing.T) {
	input := "call us at 604-555-0192 anytime"
	result := RedactSensitive(input)

	if strings.Contains(result, "604-555-0192") {
		t.Errorf("expected phone number to be redacted, got: %q", result)
	}
	if !strings.Contains(result, "[REDACTED]") {
		t.Errorf("expected [REDACTED] marker, got: %q", result)
	}
}

func TestRedactSensitive_BearerToken(t *testing.T) {
	input := "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.fakepayload"
	result := RedactSensitive(input)

	if strings.Contains(result, "eyJhbGciO") {
		t.Errorf("expected bearer token to be redacted, got: %q", result)
	}
}

func TestRedactSensitive_NoSensitiveData(t *testing.T) {
	input := "Construction project permit value $15M civil engineering crew required"
	result := RedactSensitive(input)

	if result != input {
		t.Errorf("expected no change for non-sensitive input, got: %q (original: %q)", result, input)
	}
}

func TestRedactSensitive_MultiplePatternsInOneString(t *testing.T) {
	input := "key=sk-fake12345678901234 email=user@example.com phone=604-555-1234"
	result := RedactSensitive(input)

	if strings.Contains(result, "sk-fake") {
		t.Error("expected API key to be redacted")
	}
	if strings.Contains(result, "@example.com") {
		t.Error("expected email to be redacted")
	}
	if strings.Contains(result, "604-555") {
		t.Error("expected phone to be redacted")
	}
}

func TestTruncateRawText_LongText(t *testing.T) {
	long := strings.Repeat("Construction project permit value $15M. ", 50) // > 500 chars
	result := TruncateRawText(long)

	if len(result) > rawTextMaxLen+len("...[truncated]") {
		t.Errorf("expected truncated result <= %d chars, got %d", rawTextMaxLen+14, len(result))
	}
	if !strings.Contains(result, "...[truncated]") {
		t.Error("expected truncated marker in long text")
	}
}

func TestTruncateRawText_ShortText(t *testing.T) {
	short := "Short project text."
	result := TruncateRawText(short)

	if result != short {
		t.Errorf("expected no change for short text, got: %q", result)
	}
}

func TestTruncateRawText_RedactsSecrets(t *testing.T) {
	text := "Project info. ANTHROPIC_API_KEY=sk-ant-fake123456789012345. Value $5M."
	result := TruncateRawText(text)

	if strings.Contains(result, "sk-ant-fake") {
		t.Error("expected API key to be redacted in truncated text")
	}
}

func TestNewCollectorEvent_SafeAttributes(t *testing.T) {
	attrs := map[string]string{
		"source":    "richmond_permits",
		"api_key":   "sk-fake-key-should-be-redacted",
		"project":   "Industrial Warehouse",
	}
	event := NewCollectorEvent("trace-001", "src-001", "permit_pdf", "collected", 150, attrs, "")

	if err := IsSafeForExport(event); err != nil {
		t.Errorf("expected safe event after redaction, got error: %v", err)
	}
	if event.Attributes["api_key"] == "sk-fake-key-should-be-redacted" {
		t.Error("expected api_key attribute to be redacted")
	}
}

func TestNewEnrichmentEvent_ErrorMessageRedacted(t *testing.T) {
	errMsg := "enrichment failed: api key sk-ant-api03-fake-key-12345678901234 rejected by anthropic"
	event := NewEnrichmentEvent("trace-002", "src-002", "permit_pdf", "enrichment_failed", 500, nil, errMsg)

	if strings.Contains(event.Error, "sk-ant-api03") {
		t.Errorf("expected API key in error to be redacted, got: %q", event.Error)
	}
	if event.Error == "" {
		t.Error("expected non-empty error field")
	}
}

func TestNewLLMEvent_BasicFields(t *testing.T) {
	event := NewLLMEvent("trace-003", "src-003", "json", "llm_call", 2500, nil, "")

	if event.Stage != StageLLM {
		t.Errorf("expected stage LLM, got %q", event.Stage)
	}
	if event.TraceID != "trace-003" {
		t.Errorf("expected trace_id 'trace-003', got %q", event.TraceID)
	}
	if event.EmittedAt == "" {
		t.Error("expected non-empty emitted_at")
	}
	if event.DurationMS != 2500 {
		t.Errorf("expected duration_ms 2500, got %d", event.DurationMS)
	}
}

func TestNewNotificationEvent_Stage(t *testing.T) {
	event := NewNotificationEvent("trace-004", "src-004", "slack", "notify_sent", 80, nil, "")
	if event.Stage != StageNotification {
		t.Errorf("expected stage Notification, got %q", event.Stage)
	}
}

func TestNewAlertEvent_Stage(t *testing.T) {
	event := NewAlertEvent("trace-005", "src-005", "multi_signal", "alert_fired", 100, nil, "")
	if event.Stage != StageAlert {
		t.Errorf("expected stage Alert, got %q", event.Stage)
	}
}

func TestIsSafeForExport_UnsafeAttributeDetected(t *testing.T) {
	event := TraceEvent{
		TraceID:   "trace-bad",
		Stage:     StageCollector,
		EventType: "test",
		EmittedAt: "2026-06-01T00:00:00Z",
		Attributes: map[string]string{
			"key": "sk-ant-fake-key-value-here-1234567",
		},
	}

	// Manually set without redaction to simulate unsafe event
	err := IsSafeForExport(event)
	if err == nil {
		t.Error("expected error for unsafe attribute containing API key")
	}
}

func TestIsSafeForExport_SafeEvent(t *testing.T) {
	event := TraceEvent{
		TraceID:   "trace-safe",
		Stage:     StageEnrichment,
		SourceID:  "richmond-permit-001",
		EventType: "enrichment_complete",
		EmittedAt: "2026-06-01T00:00:00Z",
		Attributes: map[string]string{
			"priority_score": "7",
			"source_type":    "permit_pdf",
		},
	}

	if err := IsSafeForExport(event); err != nil {
		t.Errorf("expected safe event to pass export check, got: %v", err)
	}
}

func TestSafeRawExcerpt_TruncatesAndRedacts(t *testing.T) {
	raw := strings.Repeat("Construction project data. ", 30) + "secret key: sk-ant-fake123456789012345"
	result := SafeRawExcerpt(raw)

	if strings.Contains(result, "sk-ant-fake") {
		t.Error("expected secret key to be redacted in safe excerpt")
	}
	if len(result) > rawTextMaxLen+len("...[truncated]") {
		t.Errorf("expected excerpt to be truncated, got length %d", len(result))
	}
}

func TestSafeRawExcerpt_ShortSafeText(t *testing.T) {
	raw := "Short safe construction text."
	result := SafeRawExcerpt(raw)
	if result != raw {
		t.Errorf("expected unchanged short safe text, got: %q", result)
	}
}
