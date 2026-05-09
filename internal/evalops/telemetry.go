package evalops

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

type TraceStage string

const (
	TraceStageCollector    TraceStage = "collector"
	TraceStageEnrichment   TraceStage = "enrichment"
	TraceStageLLM          TraceStage = "llm"
	TraceStageNotification TraceStage = "notification"
	TraceStageAlert        TraceStage = "alert"
)

type TraceEvent struct {
	Stage      TraceStage        `json:"stage"`
	TraceID    string            `json:"trace_id"`
	CaseID     string            `json:"case_id,omitempty"`
	Operation  string            `json:"operation"`
	Status     string            `json:"status"`
	ObservedAt time.Time         `json:"observed_at,omitempty"`
	Attributes map[string]string `json:"attributes,omitempty"`
}

func NewTraceEvent(stage TraceStage, traceID, caseID, operation, status string, attributes map[string]any) TraceEvent {
	event := TraceEvent{
		Stage:      stage,
		TraceID:    strings.TrimSpace(traceID),
		CaseID:     strings.TrimSpace(caseID),
		Operation:  strings.TrimSpace(operation),
		Status:     strings.TrimSpace(status),
		ObservedAt: time.Now().UTC(),
		Attributes: make(map[string]string, len(attributes)),
	}
	for key, value := range attributes {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		event.Attributes[key] = fmt.Sprint(value)
	}
	return event
}

func (e TraceEvent) Safe() TraceEvent {
	safe := e
	safe.Attributes = sanitizeStringMap(e.Attributes, 512)
	return safe
}

func (e TraceEvent) TelemetryAttributes() map[string]string {
	safe := e.Safe()
	attrs := map[string]string{
		"groupscout.trace_id":  safe.TraceID,
		"groupscout.case_id":   safe.CaseID,
		"groupscout.stage":     string(safe.Stage),
		"groupscout.operation": safe.Operation,
		"groupscout.status":    safe.Status,
	}
	keys := make([]string, 0, len(safe.Attributes))
	for key := range safe.Attributes {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		attrs[key] = safe.Attributes[key]
	}
	return attrs
}

func sanitizeStringMap(input map[string]string, maxBytes int) map[string]string {
	if len(input) == 0 {
		return nil
	}
	output := make(map[string]string, len(input))
	for key, value := range input {
		output[key] = sanitizeAttributeValue(key, value, maxBytes)
	}
	return output
}

func sanitizeAttributeValue(key, value string, maxBytes int) string {
	if sensitiveAttributeKey(key) {
		return "[REDACTED_SECRET]"
	}
	value = redactSensitive(value)
	if maxBytes > 0 && len(value) > maxBytes {
		value = value[:maxBytes] + " [TRUNCATED]"
	}
	return value
}

func sensitiveAttributeKey(key string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(key, "-", "_"))
	for _, marker := range []string{"api_key", "secret", "token", "webhook", "password", "authorization", "raw_pii"} {
		if strings.Contains(normalized, marker) {
			return true
		}
	}
	return false
}
