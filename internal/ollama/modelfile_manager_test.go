package ollama

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestModelfileManager_Push(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("expected POST, got %s", r.Method)
			}
			if r.URL.Path != "/api/create" {
				t.Errorf("expected /api/create, got %s", r.URL.Path)
			}

			var body struct {
				Name      string `json:"name"`
				Modelfile string `json:"modelfile"`
				Stream    bool   `json:"stream"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}

			if body.Name != "test-model" {
				t.Errorf("expected test-model, got %s", body.Name)
			}
			if body.Modelfile != "FROM mistral" {
				t.Errorf("expected FROM mistral, got %s", body.Modelfile)
			}
			if body.Stream != false {
				t.Error("expected stream false")
			}

			w.WriteHeader(http.StatusOK)
		}))
		defer ts.Close()

		client := &OllamaClient{
			Endpoint: ts.URL,
			Timeout:  5 * time.Second,
		}
		manager := &ModelfileManager{client: client}

		err := manager.Push(context.Background(), "test-model", "FROM mistral")
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("failure", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer ts.Close()

		client := &OllamaClient{
			Endpoint: ts.URL,
			Timeout:  5 * time.Second,
		}
		manager := &ModelfileManager{client: client}

		err := manager.Push(context.Background(), "test-model", "FROM mistral")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func TestModelfileManager_ListModels(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resp := map[string]interface{}{
				"models": []map[string]string{
					{"name": "model1"},
					{"name": "model2"},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
		}))
		defer ts.Close()

		client := &OllamaClient{
			Endpoint: ts.URL,
			Timeout:  5 * time.Second,
		}
		manager := &ModelfileManager{client: client}

		models, err := manager.ListModels(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if len(models) != 2 {
			t.Errorf("expected 2 models, got %d", len(models))
		}
		if models[0] != "model1" || models[1] != "model2" {
			t.Errorf("unexpected models: %v", models)
		}
	})

	t.Run("empty", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resp := map[string]interface{}{
				"models": []map[string]string{},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
		}))
		defer ts.Close()

		client := &OllamaClient{
			Endpoint: ts.URL,
			Timeout:  5 * time.Second,
		}
		manager := &ModelfileManager{client: client}

		models, err := manager.ListModels(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if len(models) != 0 {
			t.Errorf("expected 0 models, got %d", len(models))
		}
	})
}
