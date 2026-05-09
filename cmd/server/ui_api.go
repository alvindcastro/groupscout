package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/alvindcastro/groupscout/internal/storage"
	"github.com/google/uuid"
)

func newUIAPIHandler(db *sql.DB, dsn, apiToken string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/leads", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/leads" {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		handleUILeadList(w, r, storage.NewLeadStoreWithDSN(db, dsn))
	})
	mux.HandleFunc("/api/leads/", func(w http.ResponseWriter, r *http.Request) {
		handleUILeadResource(w, r, db, dsn, apiToken)
	})
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

	allowed := map[string]bool{"status": true, "notes": true}
	for field := range raw {
		if !allowed[field] {
			writeJSONError(w, http.StatusBadRequest, "unsupported field: "+field)
			return
		}
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

func handleUILeadRaw(w http.ResponseWriter, r *http.Request, leadStore storage.LeadStore, auditStore storage.AuditStore, leadID, apiToken string) {
	if apiToken != "" {
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
	ID                      string    `json:"id"`
	Source                  string    `json:"source"`
	Title                   string    `json:"title"`
	Location                string    `json:"location"`
	ProjectValue            int64     `json:"project_value"`
	GeneralContractor       string    `json:"general_contractor"`
	Applicant               string    `json:"applicant"`
	Contractor              string    `json:"contractor"`
	SourceURL               string    `json:"source_url"`
	ProjectType             string    `json:"project_type"`
	EstimatedCrewSize       int       `json:"estimated_crew_size"`
	EstimatedDurationMonths int       `json:"estimated_duration_months"`
	OutOfTownCrewLikely     bool      `json:"out_of_town_crew_likely"`
	PriorityScore           int       `json:"priority_score"`
	PriorityReason          string    `json:"priority_reason"`
	Rationale               string    `json:"rationale"`
	SuggestedOutreachTiming string    `json:"suggested_outreach_timing"`
	Notes                   string    `json:"notes"`
	Status                  string    `json:"status"`
	CreatedAt               time.Time `json:"created_at"`
	UpdatedAt               time.Time `json:"updated_at"`
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
		Status:                  lead.Status,
		CreatedAt:               lead.CreatedAt,
		UpdatedAt:               lead.UpdatedAt,
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
