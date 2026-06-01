package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/alvindcastro/groupscout/config"
	"github.com/alvindcastro/groupscout/internal/collector"
	"github.com/alvindcastro/groupscout/internal/logger"
)

type singleProjectProcessor interface {
	EnrichOne(ctx context.Context, p collector.RawProject) (bool, error)
}

type ingestRequest struct {
	Source       string         `json:"source"`
	ExternalID   string         `json:"external_id"`
	Title        string         `json:"title"`
	Location     string         `json:"location"`
	Value        int64          `json:"value"`
	ProjectValue int64          `json:"project_value"`
	Description  string         `json:"description"`
	IssuedAt     time.Time      `json:"issued_at"`
	SourceURL    string         `json:"source_url"`
	RawData      string         `json:"raw_data"`
	RawType      string         `json:"raw_type"`
	Metadata     map[string]any `json:"metadata"`
}

func handleIngest(cfg *config.Config, newProcessor func() singleProjectProcessor) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !authorized(r, cfg.APIToken) {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		var req ingestRequest
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON body", http.StatusBadRequest)
			return
		}

		project, err := req.rawProject()
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		inserted, err := newProcessor().EnrichOne(r.Context(), project)
		if err != nil {
			logger.Log.Error("ingest enrichment failed", "error", err)
			http.Error(w, fmt.Sprintf("Failed to ingest project: %v", err), http.StatusInternalServerError)
			return
		}

		status := "created"
		code := http.StatusCreated
		if !inserted {
			status = "duplicate"
			code = http.StatusOK
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		json.NewEncoder(w).Encode(map[string]any{
			"status":      status,
			"inserted":    inserted,
			"source":      project.Source,
			"external_id": project.ExternalID,
		})
	}
}

func (r ingestRequest) rawProject() (collector.RawProject, error) {
	if strings.TrimSpace(r.Title) == "" && strings.TrimSpace(r.Description) == "" && strings.TrimSpace(r.RawData) == "" {
		return collector.RawProject{}, fmt.Errorf("title, description, or raw_data is required")
	}
	source := strings.TrimSpace(r.Source)
	if source == "" {
		source = "api"
	}
	value := r.Value
	if value == 0 {
		value = r.ProjectValue
	}
	raw := []byte(r.RawData)
	if len(raw) == 0 {
		raw = []byte(r.Description)
	}
	rawType := strings.TrimSpace(r.RawType)
	if rawType == "" {
		rawType = "text/plain"
	}
	return collector.RawProject{
		Source:      source,
		ExternalID:  strings.TrimSpace(r.ExternalID),
		Title:       strings.TrimSpace(r.Title),
		Location:    strings.TrimSpace(r.Location),
		Value:       value,
		Description: strings.TrimSpace(r.Description),
		IssuedAt:    r.IssuedAt,
		SourceURL:   strings.TrimSpace(r.SourceURL),
		RawData:     raw,
		RawType:     rawType,
		Metadata:    r.Metadata,
	}, nil
}
