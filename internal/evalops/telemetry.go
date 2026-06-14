package evalops

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// Stage represents which pipeline stage emitted a trace event.
type Stage string

const (
	StageCollector   Stage = "collector"
	StageEnrichment  Stage = "enrichment"
	StageLLM         Stage = "llm"
	StageNotification Stage = "notification"
	StageAlert       Stage = "alert"
)

// TraceEvent is a safe, redacted trace event for logs, Loki, Sentry breadcrumbs,
// and OpenTelemetry attributes. No secrets, PII, webhook URLs, or oversized
// raw source text are included.
type TraceEvent struct {
	TraceID    string            `json:"trace_id"`
	Stage      Stage             `json:"stage"`
	SourceID   string            `json:"source_id"`
	SourceType string            `json:"source_type,omitempty"`
	EventType  string            `json:"event_type"`
	DurationMS int64             `json:"duration_ms,omitempty"`
	Attributes map[string]string `json:"attributes,omitempty"`
	Error      string            `json:"error,omitempty"`
	EmittedAt  string            `json:"emitted_at"`
}

// rawTextMaxLen is the maximum number of bytes of raw source text preserved in a trace event.
const rawTextMaxLen = 500

// redactPatterns matches common secret, webhook, email, and phone patterns.
var redactPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(sk-[a-zA-Z0-9\-_]{16,})`),                              // API keys
	regexp.MustCompile(`(?i)(https?://hooks\.slack\.com/[^\s"']+)`),                  // Slack webhooks
	regexp.MustCompile(`(?i)(https?://[^\s"']*webhook[^\s"']*)`),                     // Generic webhooks
	regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`),          // Email
	regexp.MustCompile(`\b\d{3}[-.\s]\d{3}[-.\s]\d{4}\b`),                            // Phone (xxx-xxx-xxxx)
	regexp.MustCompile(`(?i)(bearer\s+[a-zA-Z0-9\-._~+/]+=*)`),                       // Bearer tokens
	regexp.MustCompile(`(?i)(api[_-]?key\s*[:=]\s*["']?[a-zA-Z0-9\-_]{16,}["']?)`),  // API key assignments
}

// RedactSensitive replaces any sensitive patterns in s with [REDACTED].
func RedactSensitive(s string) string {
	for _, pat := range redactPatterns {
		s = pat.ReplaceAllString(s, "[REDACTED]")
	}
	return s
}

// TruncateRawText truncates raw source text to rawTextMaxLen bytes and redacts
// any sensitive patterns found within it.
func TruncateRawText(raw string) string {
	if len(raw) > rawTextMaxLen {
		raw = raw[:rawTextMaxLen] + "...[truncated]"
	}
	return RedactSensitive(raw)
}

// NewCollectorEvent creates a safe trace event for a collector stage span.
func NewCollectorEvent(traceID, sourceID, sourceType, eventType string, durationMS int64, attrs map[string]string, errMsg string) TraceEvent {
	return newEvent(traceID, StageCollector, sourceID, sourceType, eventType, durationMS, attrs, errMsg)
}

// NewEnrichmentEvent creates a safe trace event for an enrichment stage span.
func NewEnrichmentEvent(traceID, sourceID, sourceType, eventType string, durationMS int64, attrs map[string]string, errMsg string) TraceEvent {
	return newEvent(traceID, StageEnrichment, sourceID, sourceType, eventType, durationMS, attrs, errMsg)
}

// NewLLMEvent creates a safe trace event for an LLM call span.
func NewLLMEvent(traceID, sourceID, sourceType, eventType string, durationMS int64, attrs map[string]string, errMsg string) TraceEvent {
	return newEvent(traceID, StageLLM, sourceID, sourceType, eventType, durationMS, attrs, errMsg)
}

// NewNotificationEvent creates a safe trace event for a notification stage span.
func NewNotificationEvent(traceID, sourceID, sourceType, eventType string, durationMS int64, attrs map[string]string, errMsg string) TraceEvent {
	return newEvent(traceID, StageNotification, sourceID, sourceType, eventType, durationMS, attrs, errMsg)
}

// NewAlertEvent creates a safe trace event for an alert stage span.
func NewAlertEvent(traceID, sourceID, sourceType, eventType string, durationMS int64, attrs map[string]string, errMsg string) TraceEvent {
	return newEvent(traceID, StageAlert, sourceID, sourceType, eventType, durationMS, attrs, errMsg)
}

func newEvent(traceID string, stage Stage, sourceID, sourceType, eventType string, durationMS int64, attrs map[string]string, errMsg string) TraceEvent {
	safeAttrs := make(map[string]string, len(attrs))
	for k, v := range attrs {
		safeAttrs[RedactSensitive(k)] = RedactSensitive(v)
	}

	e := TraceEvent{
		TraceID:    traceID,
		Stage:      stage,
		SourceID:   sourceID,
		SourceType: sourceType,
		EventType:  eventType,
		DurationMS: durationMS,
		Attributes: safeAttrs,
		EmittedAt:  time.Now().UTC().Format(time.RFC3339),
	}
	if errMsg != "" {
		e.Error = RedactSensitive(errMsg)
	}
	return e
}

// IsSafeForExport checks whether a TraceEvent is safe to export to logs,
// Loki, Sentry, or OpenTelemetry. Returns an error describing the issue
// if any sensitive data is detected.
func IsSafeForExport(e TraceEvent) error {
	fields := []string{
		e.TraceID, e.SourceID, e.SourceType, e.EventType, e.Error,
	}
	for _, v := range e.Attributes {
		fields = append(fields, v)
	}

	for _, f := range fields {
		for _, pat := range redactPatterns {
			if pat.MatchString(f) {
				return fmt.Errorf("trace event contains unredacted sensitive data matching pattern %q in value %q",
					pat.String(), truncateForError(f))
			}
		}
	}
	return nil
}

func truncateForError(s string) string {
	if len(s) > 40 {
		return s[:40] + "..."
	}
	return s
}

// SafeRawExcerpt returns a redacted, truncated excerpt of raw source text
// suitable for including in trace events or review samples.
func SafeRawExcerpt(rawText string) string {
	trimmed := strings.TrimSpace(rawText)
	return TruncateRawText(trimmed)
}
