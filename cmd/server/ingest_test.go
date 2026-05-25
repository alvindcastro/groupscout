package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alvindcastro/groupscout/config"
	"github.com/alvindcastro/groupscout/internal/collector"
)

type fakeIngestProcessor struct {
	project  collector.RawProject
	called   bool
	inserted bool
	err      error
}

func (p *fakeIngestProcessor) EnrichOne(ctx context.Context, project collector.RawProject) (bool, error) {
	p.called = true
	p.project = project
	return p.inserted, p.err
}

func TestHandleIngest_RequiresBearerToken(t *testing.T) {
	processor := &fakeIngestProcessor{inserted: true}
	handler := handleIngest(&config.Config{APIToken: "secret"}, func() singleProjectProcessor {
		return processor
	})

	req := httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(`{"title":"Richmond warehouse"}`))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusUnauthorized)
	}
	if processor.called {
		t.Fatal("processor should not be called without auth")
	}
}

func TestHandleIngest_EnrichesSingleRawProject(t *testing.T) {
	processor := &fakeIngestProcessor{inserted: true}
	handler := handleIngest(&config.Config{APIToken: "secret"}, func() singleProjectProcessor {
		return processor
	})

	body := `{
		"source":"n8n",
		"external_id":"evt-123",
		"title":"Richmond warehouse infrastructure",
		"location":"Richmond BC",
		"project_value":12000000,
		"description":"Civil warehouse expansion",
		"source_url":"https://example.com/project",
		"raw_data":"raw event payload",
		"raw_type":"application/json",
		"metadata":{"applicant":"Example Applicant"}
	}`
	req := httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer secret")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusCreated, rr.Body.String())
	}
	if !processor.called {
		t.Fatal("processor was not called")
	}
	if processor.project.Source != "n8n" {
		t.Fatalf("Source = %q, want n8n", processor.project.Source)
	}
	if processor.project.ExternalID != "evt-123" {
		t.Fatalf("ExternalID = %q, want evt-123", processor.project.ExternalID)
	}
	if processor.project.Value != 12000000 {
		t.Fatalf("Value = %d, want 12000000", processor.project.Value)
	}
	if string(processor.project.RawData) != "raw event payload" {
		t.Fatalf("RawData = %q", string(processor.project.RawData))
	}
	if processor.project.RawType != "application/json" {
		t.Fatalf("RawType = %q, want application/json", processor.project.RawType)
	}
	if processor.project.Metadata["applicant"] != "Example Applicant" {
		t.Fatalf("metadata applicant = %#v", processor.project.Metadata["applicant"])
	}
}

func TestHandleIngest_DuplicateReturnsOK(t *testing.T) {
	processor := &fakeIngestProcessor{inserted: false}
	handler := handleIngest(&config.Config{}, func() singleProjectProcessor {
		return processor
	})

	req := httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(`{"title":"Richmond warehouse"}`))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	if !strings.Contains(rr.Body.String(), `"status":"duplicate"`) {
		t.Fatalf("body = %s, want duplicate status", rr.Body.String())
	}
}

func TestHandleIngest_RejectsEmptyProject(t *testing.T) {
	handler := handleIngest(&config.Config{}, func() singleProjectProcessor {
		return &fakeIngestProcessor{inserted: true}
	})

	req := httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(`{"source":"n8n"}`))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestHandleIngest_ProcessorError(t *testing.T) {
	handler := handleIngest(&config.Config{}, func() singleProjectProcessor {
		return &fakeIngestProcessor{err: errors.New("store failed")}
	})

	req := httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(`{"title":"Richmond warehouse"}`))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
	}
}
