package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/alvindcastro/groupscout/internal/storage"
)

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
