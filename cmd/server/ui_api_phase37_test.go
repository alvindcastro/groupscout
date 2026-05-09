package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alvindcastro/groupscout/internal/storage"
)

type blockingRunner struct {
	started chan struct{}
	release chan struct{}
}

func (r *blockingRunner) Run(ctx context.Context, req pipelineRunRequest) (pipelineRunResult, error) {
	close(r.started)
	<-r.release
	return pipelineRunResult{
		Sources: []string{"test"},
		Counts:  map[string]int{"new_leads": 1},
	}, nil
}

func TestUIAPIPostPipelineRunDoesNotBlock(t *testing.T) {
	fx := newUIAPIFixture(t, "")
	runner := &blockingRunner{started: make(chan struct{}), release: make(chan struct{})}
	fx.handler = newUIAPIHandlerWithDeps(uiAPIConfig{
		DB:             fx.db,
		DSN:            fx.dsn,
		PipelineRunner: runner,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/pipeline/runs", strings.NewReader(`{"sources":["test"]}`))
	rec := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		fx.handler.ServeHTTP(rec, req)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("POST /api/pipeline/runs blocked on runner")
	}
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202; body=%s", rec.Code, rec.Body.String())
	}

	close(runner.release)
	select {
	case <-runner.started:
	case <-time.After(time.Second):
		t.Fatal("runner did not start")
	}
}

func TestUIAPIListPipelineRunsAndStatsAndSystem(t *testing.T) {
	fx := newUIAPIFixture(t, "")
	store := storage.NewPipelineRunStoreWithDSN(fx.db, fx.dsn)
	run, err := store.Create(context.Background(), storage.PipelineRun{Sources: []string{"test"}})
	if err != nil {
		t.Fatalf("Create run: %v", err)
	}
	if err := store.Complete(context.Background(), run.ID, storage.PipelineRunCompletion{
		Status: "failed",
		Counts: map[string]int{"new_leads": 0},
		Errors: []string{"boom"},
	}); err != nil {
		t.Fatalf("Complete run: %v", err)
	}

	for _, tc := range []struct {
		path string
		want int
	}{
		{path: "/api/pipeline/runs?status=failed", want: http.StatusOK},
		{path: "/api/stats", want: http.StatusOK},
		{path: "/api/system", want: http.StatusOK},
	} {
		t.Run(tc.path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			rec := httptest.NewRecorder()
			fx.handler.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d; body=%s", rec.Code, tc.want, rec.Body.String())
			}
			if !json.Valid(rec.Body.Bytes()) {
				t.Fatalf("response is not JSON: %s", rec.Body.String())
			}
		})
	}
}
