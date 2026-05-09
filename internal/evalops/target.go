package evalops

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

type EvalTargetRequest struct {
	CaseID  string         `json:"case_id"`
	TraceID string         `json:"trace_id,omitempty"`
	Vars    map[string]any `json:"vars,omitempty"`
	Prompt  string         `json:"prompt,omitempty"`
}

type EvalTargetResponse struct {
	Output   string         `json:"output"`
	TraceID  string         `json:"trace_id"`
	Scores   []Result       `json:"scores"`
	Sources  []TargetSource `json:"sources"`
	Actions  []TargetAction `json:"actions"`
	Metadata TargetMetadata `json:"metadata"`
}

type TargetSource struct {
	System     string `json:"system"`
	Type       string `json:"type"`
	FixtureURL string `json:"fixture_url,omitempty"`
}

type TargetAction struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
}

type TargetMetadata struct {
	CaseID   string   `json:"case_id"`
	CaseType CaseType `json:"case_type"`
	Category string   `json:"category,omitempty"`
}

type TargetOutputs struct {
	Enrichment EnrichmentOutput `json:"enrichment,omitempty"`
	Outreach   OutreachDraft    `json:"outreach,omitempty"`
	SlackText  string           `json:"slack_text,omitempty"`
}

type TargetExecutor interface {
	Execute(context.Context, EvalTargetRequest, Case) (TargetOutputs, error)
}

type TargetScorer interface {
	Score(context.Context, Case, TargetOutputs) ([]Result, error)
}

type EvalTargetOptions struct {
	Timeout  time.Duration
	Executor TargetExecutor
	Scorer   TargetScorer
}

type EvalTarget struct {
	cases    map[string]Case
	timeout  time.Duration
	executor TargetExecutor
	scorer   TargetScorer
}

func NewEvalTarget(cases []Case, options EvalTargetOptions) *EvalTarget {
	index := make(map[string]Case, len(cases))
	for _, c := range cases {
		index[c.ID] = c
	}
	timeout := options.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	executor := options.Executor
	if executor == nil {
		executor = deterministicExecutor{}
	}
	scorer := options.Scorer
	if scorer == nil {
		scorer = deterministicTargetScorer{}
	}
	return &EvalTarget{
		cases:    index,
		timeout:  timeout,
		executor: executor,
		scorer:   scorer,
	}
}

func (t *EvalTarget) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
	})
	mux.HandleFunc("/eval/run", t.handleRun)
	return mux
}

func (t *EvalTarget) handleRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeTargetError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	defer r.Body.Close()

	var req EvalTargetRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeTargetError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	req.CaseID = strings.TrimSpace(req.CaseID)
	if req.CaseID == "" && req.Vars != nil {
		req.CaseID = stringVar(req.Vars, "case_id")
	}
	if req.TraceID == "" && req.Vars != nil {
		req.TraceID = stringVar(req.Vars, "trace_id")
	}
	if req.CaseID == "" {
		writeTargetError(w, http.StatusBadRequest, "missing case_id")
		return
	}
	if req.TraceID == "" {
		req.TraceID = uuid.NewString()
	}
	c, ok := t.cases[req.CaseID]
	if !ok {
		writeTargetError(w, http.StatusNotFound, "unknown case_id "+req.CaseID)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), t.timeout)
	defer cancel()

	outputs, err := t.executor.Execute(ctx, req, c)
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		writeTargetError(w, http.StatusGatewayTimeout, "eval target timed out")
		return
	}
	if err != nil {
		writeTargetError(w, http.StatusInternalServerError, "executor failed: "+err.Error())
		return
	}
	results, err := t.scorer.Score(ctx, c, outputs)
	if err != nil {
		writeTargetError(w, http.StatusInternalServerError, "scorer failed: "+err.Error())
		return
	}

	response := EvalTargetResponse{
		Output:  summarizeTargetOutput(c, results),
		TraceID: req.TraceID,
		Scores:  redactResults(results),
		Sources: []TargetSource{{
			System:     c.Source.System,
			Type:       c.Source.Type,
			FixtureURL: c.Source.FixtureURL,
		}},
		Actions: targetActions(c, results),
		Metadata: TargetMetadata{
			CaseID:   c.ID,
			CaseType: c.CaseType,
			Category: c.Category,
		},
	}
	writeTargetJSON(w, http.StatusOK, response)
}

type deterministicExecutor struct{}

func (deterministicExecutor) Execute(ctx context.Context, _ EvalTargetRequest, c Case) (TargetOutputs, error) {
	select {
	case <-ctx.Done():
		return TargetOutputs{}, ctx.Err()
	default:
	}

	outputs := TargetOutputs{}
	if c.Expected.Enrichment != nil {
		outputs.Enrichment = expectedEnrichmentOutput(c)
	}
	switch c.CaseType {
	case CaseTypeLead:
		if c.Expected.Decision != "drop" {
			outputs.Outreach = safeOutreachDraft(c)
		}
	case CaseTypeAlert:
		outputs.SlackText = safeSlackText(c)
	}
	return outputs, nil
}

type deterministicTargetScorer struct{}

func (deterministicTargetScorer) Score(ctx context.Context, c Case, outputs TargetOutputs) ([]Result, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	switch c.CaseType {
	case CaseTypeLead:
		leadScorer := NewLeadRelevanceScorer()
		results := []Result{leadScorer.Score(c)}
		if c.Expected.Enrichment != nil {
			results = append(results, ScoreEnrichmentCompleteness(c, outputs.Enrichment))
		}
		results = append(results, ScoreOutreachSafety(c, outputs.Outreach))
		return results, nil
	case CaseTypeAlert:
		return []Result{ScoreAlertThreshold(c, outputs.SlackText)}, nil
	default:
		return nil, fmt.Errorf("unsupported case_type %s", c.CaseType)
	}
}

func expectedEnrichmentOutput(c Case) EnrichmentOutput {
	expected := c.Expected.Enrichment
	if expected == nil {
		return EnrichmentOutput{}
	}
	roomNights := midpointNumeric(expected.EstimatedRoomNights)
	duration := midpointNumeric(expected.ProjectDurationMonths)
	return EnrichmentOutput{
		ProjectType:           expected.ProjectType,
		EstimatedRoomNights:   roomNights,
		ProjectDurationMonths: duration,
		LodgingNeed:           expected.LodgingNeed,
		Confidence:            expected.Confidence,
		Rationale:             safeEvidenceSummary(c),
		SourceEvidence:        []string{safeEvidenceSummary(c)},
	}
}

func midpointNumeric(expected RangeExpectation) NumericOutput {
	if expected.UnknownAllowed && expected.Min == 0 && expected.Max == 0 {
		return NumericOutput{
			Unknown:      true,
			UnknownLabel: "unknown with low confidence from fixture expectation",
		}
	}
	value := expected.Min + ((expected.Max - expected.Min) / 2)
	if value == 0 && expected.Max > 0 {
		value = expected.Max
	}
	return NumericOutput{Value: value}
}

func safeOutreachDraft(c Case) OutreachDraft {
	token := firstSafeSourceToken(c)
	if token == "" {
		token = strings.ReplaceAll(c.Category, "_", " ")
	}
	return OutreachDraft{
		Status: "draft_for_review",
		Body:   "Draft for review: source-backed context references " + token + " from the public fixture without private contact details.",
	}
}

func safeSlackText(c Case) string {
	signals := c.Raw.Signals
	parts := []string{fmt.Sprintf("%d cancellations", signals.CancelledCount)}
	if signals.WeatherAlert != nil {
		parts = append(parts, strings.ReplaceAll(signals.WeatherAlert.Type, "_", " "))
	}
	if signals.Notams != nil && len(*signals.Notams) > 0 {
		parts = append(parts, "NOTAM capacity signal")
	}
	return strings.Join(parts, "; ")
}

func safeEvidenceSummary(c Case) string {
	if len(c.Expected.Evidence) > 0 {
		return redactSensitive(c.Expected.Evidence[0].MustSupportWith)
	}
	if c.Raw.Title != "" {
		return redactSensitive(c.Raw.Title)
	}
	return "fixture evidence unavailable"
}

func firstSafeSourceToken(c Case) string {
	for _, token := range sourceEvidenceTokens(c) {
		if firstPrivacyLeak(c.Expected.Privacy, token) == "" && !strings.Contains(strings.ToLower(token), "token") {
			return token
		}
	}
	return ""
}

func summarizeTargetOutput(c Case, results []Result) string {
	report := BuildReport(results)
	status := "pass"
	if report.Summary.CriticalFailures > 0 || report.Summary.ReleaseBlockingFailures > 0 {
		status = "fail"
	} else if report.Summary.Warnings > 0 {
		status = "warning"
	}
	return fmt.Sprintf("case %s %s: %d checks, %d critical, %d warnings", c.ID, status, report.Summary.Total, report.Summary.CriticalFailures, report.Summary.Warnings)
}

func targetActions(c Case, results []Result) []TargetAction {
	actions := []TargetAction{{
		Name:   "score_case",
		Status: "completed",
		Reason: "deterministic eval scorer",
	}}
	if c.CaseType == CaseTypeLead && c.Expected.Decision != "drop" {
		actions = append(actions, TargetAction{
			Name:   "outreach_review",
			Status: "draft_for_review",
			Reason: "human review required before any outreach",
		})
	}
	report := BuildReport(results)
	if report.Summary.ReleaseBlockingFailures > 0 {
		actions = append(actions, TargetAction{
			Name:   "release_gate",
			Status: "blocked",
			Reason: "release-blocking eval failure",
		})
	}
	return actions
}

func stringVar(vars map[string]any, key string) string {
	value, ok := vars[key]
	if !ok || value == nil {
		return ""
	}
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}

func writeTargetError(w http.ResponseWriter, status int, message string) {
	writeTargetJSON(w, status, map[string]string{"error": redactSensitive(message)})
}

func writeTargetJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
