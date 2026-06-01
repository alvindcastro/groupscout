package main

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"

	"github.com/alvindcastro/groupscout/config"
	"github.com/alvindcastro/groupscout/internal/leadnotify"
	"github.com/alvindcastro/groupscout/internal/logger"
	"github.com/alvindcastro/groupscout/internal/storage"
)

func handleN8NWebhook(cfg *config.Config, leadStore storage.LeadStore, notifier leadnotify.Notifier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if !authorized(r, cfg.APIToken) {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		var l storage.Lead
		if err := json.NewDecoder(r.Body).Decode(&l); err != nil {
			logger.Log.Error("failed to decode n8n lead", "error", err)
			http.Error(w, "Invalid JSON body", http.StatusBadRequest)
			return
		}

		normalizeWebhookLead(&l)

		if err := leadStore.Insert(r.Context(), &l); err != nil {
			logger.Log.Error("failed to insert n8n lead", "error", err)
			http.Error(w, fmt.Sprintf("Failed to store lead: %v", err), http.StatusInternalServerError)
			return
		}

		logger.Log.Info("lead received from n8n", "source", l.Source, "title", l.Title)

		if err := notifier.Send(r.Context(), []storage.Lead{l}); err != nil {
			logger.Log.Warn("failed to notify Slack for n8n lead", "error", err)
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"status": "success", "id": l.ID})
	}
}

func normalizeWebhookLead(l *storage.Lead) {
	if l.Source == "" {
		l.Source = "n8n"
	}
	if l.ID == "" {
		l.ID = storage.NewUUID()
	}
	l.PriorityScore = normalizeExternalPriorityScore(l.PriorityScore)
}

func normalizeExternalPriorityScore(score int) int {
	switch {
	case score < 0:
		return 0
	case score <= 10:
		return score
	case score <= 100:
		return int(math.Round(float64(score) / 10))
	default:
		return 10
	}
}
