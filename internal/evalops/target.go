package evalops

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// EvalTargetRequest is the JSON body accepted by the local eval target HTTP server.
// Promptfoo sends this to http://localhost:18080/eval/run.
type EvalTargetRequest struct {
	CaseID   string         `json:"case_id"`
	CaseType string         `json:"case_type,omitempty"`
	Raw      map[string]any `json:"raw,omitempty"`
	TraceID  string         `json:"trace_id,omitempty"`
}

// EvalTargetResponse is the JSON response from the eval target.
// It follows the Promptfoo provider response shape.
type EvalTargetResponse struct {
	Output  string              `json:"output"`
	TraceID string              `json:"trace_id"`
	Scores  map[string]float64  `json:"scores"`
	Sources []string            `json:"sources"`
	Actions []string            `json:"actions,omitempty"`
}

// EvalTargetHandler handles HTTP requests to the local eval target endpoint.
// It runs the relevant scorers for the given case and returns a safe response.
type EvalTargetHandler struct {
	cases   []Case
	timeout time.Duration
}

// NewEvalTargetHandler creates a new handler loaded with the given cases.
func NewEvalTargetHandler(cases []Case, timeout time.Duration) *EvalTargetHandler {
	if timeout == 0 {
		timeout = 10 * time.Second
	}
	return &EvalTargetHandler{cases: cases, timeout: timeout}
}

// ServeHTTP handles POST /eval/run requests from Promptfoo.
func (h *EvalTargetHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !strings.HasSuffix(r.URL.Path, "/eval/run") {
		http.NotFound(w, r)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.timeout)
	defer cancel()

	var req EvalTargetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("invalid JSON request: %v", err))
		return
	}

	if req.CaseID == "" {
		writeJSONError(w, http.StatusBadRequest, "case_id is required")
		return
	}

	// Generate trace ID if not provided
	traceID := req.TraceID
	if traceID == "" {
		traceID = generateTraceID()
	}

	// Find the case
	c, found := h.findCase(req.CaseID)
	if !found {
		writeJSONError(w, http.StatusNotFound, fmt.Sprintf("case %q not found", req.CaseID))
		return
	}

	// Check context timeout
	select {
	case <-ctx.Done():
		writeJSONError(w, http.StatusGatewayTimeout, "eval timed out")
		return
	default:
	}

	// Run scorers
	resp := h.runScorers(c, traceID)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

func (h *EvalTargetHandler) findCase(id string) (Case, bool) {
	for _, c := range h.cases {
		if c.ID == id {
			return c, true
		}
	}
	return Case{}, false
}

func (h *EvalTargetHandler) runScorers(c Case, traceID string) EvalTargetResponse {
	scores := make(map[string]float64)
	var sources []string
	var outputParts []string
	var criticalFailures []string

	switch c.CaseType {
	case "lead":
		result := ScoreLeadCase(c)
		if result.IsCriticalFailure {
			scores["lead_quality"] = 0.0
			for _, f := range result.Findings {
				criticalFailures = append(criticalFailures, f.Code+": "+f.Message)
			}
		} else {
			scores["lead_quality"] = 1.0
		}
		scores["lead_score"] = result.Score
		outputParts = append(outputParts, fmt.Sprintf("decision=%s score=%.1f", result.Actual, result.Score))
		sources = append(sources, fmt.Sprintf("case:%s type:lead", c.ID))

	case "alert":
		result := ScoreAlertCase(c)
		if result.IsCriticalFailure {
			scores["alert_quality"] = 0.0
			for _, f := range result.Findings {
				criticalFailures = append(criticalFailures, f.Code+": "+f.Message)
			}
		} else {
			scores["alert_quality"] = 1.0
		}
		scores["sps_score"] = result.SPSScore
		outputParts = append(outputParts, fmt.Sprintf("decision=%s sps=%.2f stale=%v", result.Decision, result.SPSScore, result.IsStale))
		sources = append(sources, fmt.Sprintf("case:%s type:alert", c.ID))
	}

	if len(criticalFailures) > 0 {
		outputParts = append(outputParts, "CRITICAL: "+strings.Join(criticalFailures, "; "))
	}

	return EvalTargetResponse{
		Output:  strings.Join(outputParts, " | "),
		TraceID: traceID,
		Scores:  scores,
		Sources: sources,
	}
}

// generateTraceID returns a simple trace ID based on current time.
// In production, use a proper UUID generator.
func generateTraceID() string {
	return fmt.Sprintf("eval-%d", time.Now().UnixNano())
}

func writeJSONError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
