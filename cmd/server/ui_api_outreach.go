package main

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/alvindcastro/groupscout/internal/storage"
)

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

type outreachEventResponse struct {
	ID       string    `json:"id"`
	LeadID   string    `json:"lead_id"`
	Contact  string    `json:"contact"`
	Channel  string    `json:"channel"`
	Notes    string    `json:"notes"`
	Outcome  string    `json:"outcome"`
	LoggedAt time.Time `json:"logged_at"`
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
