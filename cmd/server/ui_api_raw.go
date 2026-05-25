package main

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/alvindcastro/groupscout/internal/storage"
	"github.com/google/uuid"
)

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

type auditMetadataResponse struct {
	HasRaw        bool      `json:"has_raw"`
	RawLink       string    `json:"raw_link,omitempty"`
	PayloadType   string    `json:"payload_type,omitempty"`
	SourceURL     string    `json:"source_url,omitempty"`
	CollectorName string    `json:"collector_name,omitempty"`
	CollectedAt   time.Time `json:"collected_at,omitempty"`
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
