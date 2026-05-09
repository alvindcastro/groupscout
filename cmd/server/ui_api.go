package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/alvindcastro/groupscout/internal/storage"
	"github.com/google/uuid"
)

type uiAPIConfig struct {
	DB             *sql.DB
	DSN            string
	APIToken       string
	AdminAuth      *adminAuthenticator
	PipelineRunner pipelineRunner
}

type pipelineRunRequest struct {
	Sources       []string `json:"sources"`
	BCBidRawInput string   `json:"bcbid_raw_input"`
	DryRun        bool     `json:"dry_run"`
}

type pipelineRunResult struct {
	Sources []string       `json:"sources"`
	Counts  map[string]int `json:"counts"`
	Errors  []string       `json:"errors"`
}

type pipelineRunner interface {
	Run(ctx context.Context, req pipelineRunRequest) (pipelineRunResult, error)
}

type noopPipelineRunner struct{}

func (noopPipelineRunner) Run(ctx context.Context, req pipelineRunRequest) (pipelineRunResult, error) {
	return pipelineRunResult{Sources: req.Sources, Counts: map[string]int{"new_leads": 0}, Errors: []string{}}, nil
}

func newUIAPIHandler(db *sql.DB, dsn, apiToken string) http.Handler {
	return newUIAPIHandlerWithDeps(uiAPIConfig{DB: db, DSN: dsn, APIToken: apiToken, PipelineRunner: noopPipelineRunner{}})
}

func newUIAPIHandlerWithDeps(cfg uiAPIConfig) http.Handler {
	if cfg.PipelineRunner == nil {
		cfg.PipelineRunner = noopPipelineRunner{}
	}
	mux := http.NewServeMux()
	if cfg.AdminAuth != nil {
		mux.HandleFunc("/api/auth/status", cfg.AdminAuth.handleStatus)
		mux.HandleFunc("/api/auth/login", cfg.AdminAuth.handleLogin)
		mux.HandleFunc("/api/auth/me", cfg.AdminAuth.handleMe)
	}
	protected := http.NewServeMux()
	protected.HandleFunc("/api/leads", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/leads" {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		handleUILeadList(w, r, storage.NewLeadStoreWithDSN(cfg.DB, cfg.DSN))
	})
	protected.HandleFunc("/api/leads/", func(w http.ResponseWriter, r *http.Request) {
		handleUILeadResource(w, r, cfg.DB, cfg.DSN, cfg.APIToken)
	})
	protected.HandleFunc("/api/pipeline/runs", func(w http.ResponseWriter, r *http.Request) {
		handleUIPipelineRuns(w, r, cfg)
	})
	protected.HandleFunc("/api/stats", func(w http.ResponseWriter, r *http.Request) {
		handleUIStats(w, r, cfg)
	})
	protected.HandleFunc("/api/system", func(w http.ResponseWriter, r *http.Request) {
		handleUISystem(w, r, cfg)
	})
	protected.HandleFunc("/api/alerts", func(w http.ResponseWriter, r *http.Request) {
		handleUIAlerts(w, r)
	})
	mux.Handle("/api/", cfg.AdminAuth.requireSession(protected))
	return mux
}

func handleUILeadList(w http.ResponseWriter, r *http.Request, leadStore storage.LeadStore) {
	query := r.URL.Query()
	filter := storage.LeadListFilter{
		Status: query.Get("status"),
		Source: query.Get("source"),
		Query:  query.Get("q"),
		Cursor: query.Get("cursor"),
	}
	if raw := query.Get("min_score"); raw != "" {
		minScore, err := strconv.Atoi(raw)
		if err != nil || minScore < 0 {
			writeJSONError(w, http.StatusBadRequest, "invalid min_score")
			return
		}
		filter.MinScore = minScore
	}
	if raw := query.Get("limit"); raw != "" {
		limit, err := strconv.Atoi(raw)
		if err != nil || limit < 1 || limit > 100 {
			writeJSONError(w, http.StatusBadRequest, "invalid limit")
			return
		}
		filter.Limit = limit
	}

	leads, next, err := leadStore.ListFiltered(r.Context(), filter)
	if err != nil {
		if strings.Contains(err.Error(), "invalid cursor") {
			writeJSONError(w, http.StatusBadRequest, "invalid cursor")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "list leads failed")
		return
	}

	items := make([]leadSummaryResponse, 0, len(leads))
	for _, lead := range leads {
		items = append(items, leadSummary(lead))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items":       items,
		"next_cursor": next,
		"filters": map[string]any{
			"status":    filter.Status,
			"source":    filter.Source,
			"min_score": filter.MinScore,
			"q":         filter.Query,
			"limit":     filter.Limit,
			"cursor":    filter.Cursor,
		},
	})
}

func handleUIPipelineRuns(w http.ResponseWriter, r *http.Request, cfg uiAPIConfig) {
	store := storage.NewPipelineRunStoreWithDSN(cfg.DB, cfg.DSN)
	switch r.Method {
	case http.MethodPost:
		var req pipelineRunRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err.Error() != "EOF" {
			writeJSONError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		if req.DryRun {
			writeJSONError(w, http.StatusBadRequest, "dry_run is not supported")
			return
		}
		run, err := store.Create(r.Context(), storage.PipelineRun{
			Sources: req.Sources,
			Request: map[string]any{
				"sources":         req.Sources,
				"bcbid_raw_input": req.BCBidRawInput != "",
				"dry_run":         req.DryRun,
			},
		})
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "create pipeline run failed")
			return
		}
		go func(runID string, request pipelineRunRequest) {
			result, err := cfg.PipelineRunner.Run(context.Background(), request)
			completion := storage.PipelineRunCompletion{Status: "succeeded", Counts: result.Counts, Errors: result.Errors}
			if err != nil {
				completion.Status = "failed"
				completion.Errors = append(completion.Errors, err.Error())
			}
			_ = store.Complete(context.Background(), runID, completion)
		}(run.ID, req)
		writeJSON(w, http.StatusAccepted, map[string]any{"run_id": run.ID, "status": run.Status, "started_at": run.StartedAt})
	case http.MethodGet:
		filter := storage.PipelineRunListFilter{Status: r.URL.Query().Get("status"), Cursor: r.URL.Query().Get("cursor")}
		if filter.Status != "" && !validPipelineStatus(filter.Status) {
			writeJSONError(w, http.StatusBadRequest, "invalid status")
			return
		}
		if rawLimit := r.URL.Query().Get("limit"); rawLimit != "" {
			limit, err := strconv.Atoi(rawLimit)
			if err != nil || limit < 1 || limit > 100 {
				writeJSONError(w, http.StatusBadRequest, "invalid limit")
				return
			}
			filter.Limit = limit
		}
		runs, next, err := store.List(r.Context(), filter)
		if err != nil {
			if strings.Contains(err.Error(), "invalid cursor") {
				writeJSONError(w, http.StatusBadRequest, "invalid cursor")
				return
			}
			writeJSONError(w, http.StatusInternalServerError, "list pipeline runs failed")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": runs, "next_cursor": next, "filters": filter})
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func validPipelineStatus(status string) bool {
	switch status {
	case "queued", "running", "succeeded", "failed":
		return true
	default:
		return false
	}
}

func handleUIStats(w http.ResponseWriter, r *http.Request, cfg uiAPIConfig) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	stats, err := storage.NewStatsStoreWithDSN(cfg.DB, cfg.DSN).Summary(r.Context(), storage.StatsFilter{Window: r.URL.Query().Get("window")})
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "load stats failed")
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

func handleUISystem(w http.ResponseWriter, r *http.Request, cfg uiAPIConfig) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	status := "healthy"
	database := map[string]string{"status": "ok"}
	code := http.StatusOK
	if err := cfg.DB.PingContext(r.Context()); err != nil {
		status = "degraded"
		database["status"] = "error"
		database["error"] = err.Error()
		code = http.StatusServiceUnavailable
	}
	latest, err := storage.NewPipelineRunStoreWithDSN(cfg.DB, cfg.DSN).Latest(r.Context())
	if err != nil {
		status = "degraded"
	}
	if latest != nil && latest.Status == "failed" && code == http.StatusOK {
		status = "degraded"
	}
	writeJSON(w, code, map[string]any{
		"status":            status,
		"database":          database,
		"ollama":            map[string]string{"status": "not_checked"},
		"metrics_available": true,
		"last_pipeline_run": latest,
		"note":              fmt.Sprintf("metrics_available is a server capability flag; browser UI does not parse /metrics"),
	})
}

func handleUIAlerts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	filter := alertListFilter{
		State:    r.URL.Query().Get("state"),
		Property: r.URL.Query().Get("property"),
		Cursor:   r.URL.Query().Get("cursor"),
	}
	if rawLimit := r.URL.Query().Get("limit"); rawLimit != "" {
		limit, err := strconv.Atoi(rawLimit)
		if err != nil || limit < 1 || limit > 100 {
			writeJSONError(w, http.StatusBadRequest, "invalid limit")
			return
		}
		filter.Limit = limit
	}

	alerts := []alertResponse{}
	writeJSON(w, http.StatusOK, map[string]any{
		"alerts":      alerts,
		"items":       alerts,
		"next_cursor": nil,
		"read_only":   true,
		"filters":     filter,
	})
}

func handleUILeadResource(w http.ResponseWriter, r *http.Request, db *sql.DB, dsn, apiToken string) {
	rest := strings.TrimPrefix(r.URL.Path, "/api/leads/")
	parts := strings.Split(strings.Trim(rest, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		http.NotFound(w, r)
		return
	}
	leadID := parts[0]
	leadStore := storage.NewLeadStoreWithDSN(db, dsn)
	auditStore := storage.NewAuditStoreWithDSN(db, dsn)
	outreachStore := storage.NewOutreachStoreWithDSN(db, dsn)

	if len(parts) == 1 {
		switch r.Method {
		case http.MethodGet:
			handleUILeadDetail(w, r, leadStore, auditStore, leadID)
		case http.MethodPatch:
			handleUILeadPatch(w, r, leadStore, leadID)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
		return
	}
	if len(parts) == 2 && parts[1] == "raw" {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		handleUILeadRaw(w, r, leadStore, auditStore, leadID, apiToken)
		return
	}
	if len(parts) == 2 && parts[1] == "outreach" {
		switch r.Method {
		case http.MethodGet:
			handleUIOutreachList(w, r, leadStore, outreachStore, leadID)
		case http.MethodPost:
			handleUIOutreachCreate(w, r, leadStore, outreachStore, leadID)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
		return
	}
	http.NotFound(w, r)
}

func handleUILeadDetail(w http.ResponseWriter, r *http.Request, leadStore storage.LeadStore, auditStore storage.AuditStore, leadID string) {
	lead, err := leadStore.GetByID(r.Context(), leadID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "get lead failed")
		return
	}
	if lead == nil {
		writeJSONError(w, http.StatusNotFound, "lead not found")
		return
	}
	audit, err := auditMetadata(r.Context(), *lead, auditStore, "/api/leads/"+lead.ID+"/raw")
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "get audit metadata failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"lead":             leadDetail(*lead),
		"audit":            audit,
		"outreach_summary": map[string]any{"count": 0, "latest_at": nil},
		"activity":         []any{},
	})
}

func handleUILeadPatch(w http.ResponseWriter, r *http.Request, leadStore storage.LeadStore, leadID string) {
	var raw map[string]json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	allowed := map[string]bool{"status": true, "notes": true, "action": true, "owner": true, "snoozed_until": true}
	for field := range raw {
		if !allowed[field] {
			writeJSONError(w, http.StatusBadRequest, "unsupported field: "+field)
			return
		}
	}

	if _, ok := raw["action"]; ok {
		action, err := decodeLeadAction(raw)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		result, err := leadStore.ApplyAction(r.Context(), leadID, action)
		if err != nil {
			if errors.Is(err, storage.ErrLeadNotFound) {
				writeJSONError(w, http.StatusNotFound, "lead not found")
				return
			}
			if errors.Is(err, storage.ErrInvalidLeadTransition) {
				writeJSONError(w, http.StatusConflict, err.Error())
				return
			}
			writeJSONError(w, http.StatusInternalServerError, "apply lead action failed")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"lead":           leadDetail(result.Lead),
			"changed_fields": result.ChangedFields,
			"updated_at":     result.UpdatedAt,
		})
		return
	}

	var patch storage.LeadPatch
	if v, ok := raw["status"]; ok {
		var status string
		if err := json.Unmarshal(v, &status); err != nil || strings.TrimSpace(status) == "" {
			writeJSONError(w, http.StatusBadRequest, "invalid status")
			return
		}
		patch.Status = &status
	}
	if v, ok := raw["notes"]; ok {
		var notes string
		if err := json.Unmarshal(v, &notes); err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid notes")
			return
		}
		patch.Notes = &notes
	}

	result, err := leadStore.UpdateOperatorFields(r.Context(), leadID, patch)
	if err != nil {
		if errors.Is(err, storage.ErrLeadNotFound) {
			writeJSONError(w, http.StatusNotFound, "lead not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "update lead failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"lead":           leadDetail(result.Lead),
		"changed_fields": result.ChangedFields,
		"updated_at":     result.UpdatedAt,
	})
}

func decodeLeadAction(raw map[string]json.RawMessage) (storage.LeadAction, error) {
	var action storage.LeadAction
	if v, ok := raw["action"]; ok {
		if err := json.Unmarshal(v, &action.Action); err != nil || strings.TrimSpace(action.Action) == "" {
			return action, errors.New("invalid action")
		}
	}
	if v, ok := raw["owner"]; ok {
		if err := json.Unmarshal(v, &action.Owner); err != nil {
			return action, errors.New("invalid owner")
		}
	}
	if v, ok := raw["notes"]; ok {
		var notes string
		if err := json.Unmarshal(v, &notes); err != nil {
			return action, errors.New("invalid notes")
		}
		action.Notes = &notes
	}
	if v, ok := raw["snoozed_until"]; ok {
		var text string
		if err := json.Unmarshal(v, &text); err != nil {
			return action, errors.New("invalid snoozed_until")
		}
		parsed, err := time.Parse(time.RFC3339, text)
		if err != nil {
			return action, errors.New("invalid snoozed_until")
		}
		action.SnoozedUntil = &parsed
	}
	return action, nil
}

func handleUIOutreachList(w http.ResponseWriter, r *http.Request, leadStore storage.LeadStore, outreachStore storage.OutreachStore, leadID string) {
	lead, err := leadStore.GetByID(r.Context(), leadID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "get lead failed")
		return
	}
	if lead == nil {
		writeJSONError(w, http.StatusNotFound, "lead not found")
		return
	}
	filter := storage.OutreachListFilter{Cursor: r.URL.Query().Get("cursor")}
	if rawLimit := r.URL.Query().Get("limit"); rawLimit != "" {
		limit, err := strconv.Atoi(rawLimit)
		if err != nil || limit < 1 || limit > 100 {
			writeJSONError(w, http.StatusBadRequest, "invalid limit")
			return
		}
		filter.Limit = limit
	}
	events, next, err := outreachStore.ListByLead(r.Context(), leadID, filter)
	if err != nil {
		if strings.Contains(err.Error(), "invalid cursor") {
			writeJSONError(w, http.StatusBadRequest, "invalid cursor")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "list outreach failed")
		return
	}
	items := make([]outreachEventResponse, 0, len(events))
	for _, event := range events {
		items = append(items, outreachEvent(event))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "next_cursor": next})
}

func handleUIOutreachCreate(w http.ResponseWriter, r *http.Request, leadStore storage.LeadStore, outreachStore storage.OutreachStore, leadID string) {
	lead, err := leadStore.GetByID(r.Context(), leadID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "get lead failed")
		return
	}
	if lead == nil {
		writeJSONError(w, http.StatusNotFound, "lead not found")
		return
	}
	var req struct {
		Contact string `json:"contact"`
		Channel string `json:"channel"`
		Notes   string `json:"notes"`
		Outcome string `json:"outcome"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	event, err := outreachStore.Insert(r.Context(), storage.OutreachEvent{
		LeadID:  leadID,
		Contact: req.Contact,
		Channel: req.Channel,
		Notes:   req.Notes,
		Outcome: req.Outcome,
	})
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"outreach": outreachEvent(*event),
		"lead":     leadDetail(*lead),
	})
}

func handleUILeadRaw(w http.ResponseWriter, r *http.Request, leadStore storage.LeadStore, auditStore storage.AuditStore, leadID, apiToken string) {
	if apiToken != "" && !requestHasAdminSession(r) {
		authHeader := r.Header.Get("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") || strings.TrimPrefix(authHeader, "Bearer ") != apiToken {
			writeJSONError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
	}
	raw, status, errMessage := leadRawInput(r.Context(), leadStore, auditStore, leadID)
	if errMessage != "" {
		writeJSONError(w, status, errMessage)
		return
	}
	contentType := raw.PayloadType
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(raw.Payload)
}

func leadRawInput(ctx context.Context, leadStore storage.LeadStore, auditStore storage.AuditStore, leadID string) (*storage.RawInput, int, string) {
	lead, err := leadStore.GetByID(ctx, leadID)
	if err != nil {
		return nil, http.StatusInternalServerError, "get lead failed"
	}
	if lead == nil {
		return nil, http.StatusNotFound, "lead not found"
	}
	if lead.RawInputID == "" {
		return nil, http.StatusNotFound, "lead has no raw input associated"
	}
	rawInputID, err := uuid.Parse(lead.RawInputID)
	if err != nil {
		return nil, http.StatusInternalServerError, "invalid raw input ID"
	}
	raw, err := auditStore.GetByID(ctx, rawInputID)
	if err != nil {
		return nil, http.StatusInternalServerError, "get raw input failed"
	}
	if raw == nil {
		return nil, http.StatusNotFound, "raw input not found"
	}
	return raw, 0, ""
}

type leadSummaryResponse struct {
	ID             string    `json:"id"`
	Title          string    `json:"title"`
	Source         string    `json:"source"`
	Location       string    `json:"location"`
	ProjectValue   int64     `json:"project_value"`
	PriorityScore  int       `json:"priority_score"`
	PriorityReason string    `json:"priority_reason"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	HasRaw         bool      `json:"has_raw"`
	AuditSourceURL string    `json:"audit_source_url,omitempty"`
}

type leadDetailResponse struct {
	ID                      string     `json:"id"`
	Source                  string     `json:"source"`
	Title                   string     `json:"title"`
	Location                string     `json:"location"`
	ProjectValue            int64      `json:"project_value"`
	GeneralContractor       string     `json:"general_contractor"`
	Applicant               string     `json:"applicant"`
	Contractor              string     `json:"contractor"`
	SourceURL               string     `json:"source_url"`
	ProjectType             string     `json:"project_type"`
	EstimatedCrewSize       int        `json:"estimated_crew_size"`
	EstimatedDurationMonths int        `json:"estimated_duration_months"`
	OutOfTownCrewLikely     bool       `json:"out_of_town_crew_likely"`
	PriorityScore           int        `json:"priority_score"`
	PriorityReason          string     `json:"priority_reason"`
	Rationale               string     `json:"rationale"`
	SuggestedOutreachTiming string     `json:"suggested_outreach_timing"`
	Notes                   string     `json:"notes"`
	Owner                   string     `json:"owner"`
	SnoozedUntil            *time.Time `json:"snoozed_until"`
	Flagged                 bool       `json:"flagged"`
	VerificationState       string     `json:"verification_state"`
	Status                  string     `json:"status"`
	CreatedAt               time.Time  `json:"created_at"`
	UpdatedAt               time.Time  `json:"updated_at"`
}

type outreachEventResponse struct {
	ID       string    `json:"id"`
	LeadID   string    `json:"lead_id"`
	Contact  string    `json:"contact"`
	Channel  string    `json:"channel"`
	Notes    string    `json:"notes"`
	Outcome  string    `json:"outcome"`
	LoggedAt time.Time `json:"logged_at"`
}

type alertListFilter struct {
	State    string `json:"state"`
	Property string `json:"property"`
	Limit    int    `json:"limit"`
	Cursor   string `json:"cursor"`
}

type alertResponse struct {
	ID            string               `json:"id"`
	Property      string               `json:"property"`
	SPS           int                  `json:"sps"`
	State         string               `json:"state"`
	Impact        string               `json:"impact"`
	UpdatedAt     time.Time            `json:"updated_at"`
	Evidence      []alertEvidence      `json:"evidence"`
	RoomInventory alertRoomInventory   `json:"room_inventory"`
	ActionHistory []alertActionHistory `json:"action_history"`
}

type alertEvidence struct {
	Type       string    `json:"type"`
	Label      string    `json:"label"`
	Value      string    `json:"value"`
	SourceURL  string    `json:"source_url"`
	ObservedAt time.Time `json:"observed_at"`
}

type alertRoomInventory struct {
	Total        int       `json:"total"`
	Unavailable  int       `json:"unavailable"`
	Available    int       `json:"available"`
	OutOfService int       `json:"out_of_service"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type alertActionHistory struct {
	Actor     string    `json:"actor"`
	Action    string    `json:"action"`
	Channel   string    `json:"channel"`
	Note      string    `json:"note"`
	CreatedAt time.Time `json:"created_at"`
}

type auditMetadataResponse struct {
	HasRaw        bool      `json:"has_raw"`
	RawLink       string    `json:"raw_link,omitempty"`
	PayloadType   string    `json:"payload_type,omitempty"`
	SourceURL     string    `json:"source_url,omitempty"`
	CollectorName string    `json:"collector_name,omitempty"`
	CollectedAt   time.Time `json:"collected_at,omitempty"`
}

func leadSummary(lead storage.Lead) leadSummaryResponse {
	return leadSummaryResponse{
		ID:             lead.ID,
		Title:          lead.Title,
		Source:         lead.Source,
		Location:       lead.Location,
		ProjectValue:   lead.ProjectValue,
		PriorityScore:  lead.PriorityScore,
		PriorityReason: lead.PriorityReason,
		Status:         lead.Status,
		CreatedAt:      lead.CreatedAt,
		UpdatedAt:      lead.UpdatedAt,
		HasRaw:         lead.RawInputID != "",
		AuditSourceURL: lead.SourceURL,
	}
}

func leadDetail(lead storage.Lead) leadDetailResponse {
	return leadDetailResponse{
		ID:                      lead.ID,
		Source:                  lead.Source,
		Title:                   lead.Title,
		Location:                lead.Location,
		ProjectValue:            lead.ProjectValue,
		GeneralContractor:       lead.GeneralContractor,
		Applicant:               lead.Applicant,
		Contractor:              lead.Contractor,
		SourceURL:               lead.SourceURL,
		ProjectType:             lead.ProjectType,
		EstimatedCrewSize:       lead.EstimatedCrewSize,
		EstimatedDurationMonths: lead.EstimatedDurationMonths,
		OutOfTownCrewLikely:     lead.OutOfTownCrewLikely,
		PriorityScore:           lead.PriorityScore,
		PriorityReason:          lead.PriorityReason,
		Rationale:               lead.Rationale,
		SuggestedOutreachTiming: lead.SuggestedOutreachTiming,
		Notes:                   lead.Notes,
		Owner:                   lead.Owner,
		SnoozedUntil:            lead.SnoozedUntil,
		Flagged:                 lead.Flagged,
		VerificationState:       lead.VerificationState,
		Status:                  lead.Status,
		CreatedAt:               lead.CreatedAt,
		UpdatedAt:               lead.UpdatedAt,
	}
}

func outreachEvent(event storage.OutreachEvent) outreachEventResponse {
	return outreachEventResponse{
		ID:       event.ID,
		LeadID:   event.LeadID,
		Contact:  event.Contact,
		Channel:  event.Channel,
		Notes:    event.Notes,
		Outcome:  event.Outcome,
		LoggedAt: event.LoggedAt,
	}
}

func auditMetadata(ctx context.Context, lead storage.Lead, auditStore storage.AuditStore, rawLink string) (auditMetadataResponse, error) {
	if lead.RawInputID == "" {
		return auditMetadataResponse{HasRaw: false}, nil
	}
	rawInputID, err := uuid.Parse(lead.RawInputID)
	if err != nil {
		return auditMetadataResponse{}, err
	}
	raw, err := auditStore.GetByID(ctx, rawInputID)
	if err != nil {
		return auditMetadataResponse{}, err
	}
	if raw == nil {
		return auditMetadataResponse{HasRaw: false}, nil
	}
	return auditMetadataResponse{
		HasRaw:        true,
		RawLink:       rawLink,
		PayloadType:   raw.PayloadType,
		SourceURL:     raw.SourceURL,
		CollectorName: raw.CollectorName,
		CollectedAt:   raw.CreatedAt,
	}, nil
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeJSONError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
