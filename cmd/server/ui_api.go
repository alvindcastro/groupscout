package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/alvindcastro/groupscout/internal/storage"
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
		mux.HandleFunc("/api/auth/logout", cfg.AdminAuth.handleLogout)
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

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeJSONError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
