package ollama

import (
	"context"
	"net/http"
)

// ModelfileManager handles pushing Modelfiles to the Ollama server.
type ModelfileManager struct {
	client *OllamaClient
}

// NewModelfileManager creates a new ModelfileManager.
func NewModelfileManager(client *OllamaClient) *ModelfileManager {
	return &ModelfileManager{client: client}
}

// Push sends a Modelfile to the Ollama /api/create endpoint.
func (m *ModelfileManager) Push(ctx context.Context, modelName, modelfileContent string) error {
	ctx = WithUseCase(ctx, "modelfile_push")
	reqBody := map[string]interface{}{
		"name":      modelName,
		"modelfile": modelfileContent,
		"stream":    false,
	}

	return m.client.doWithRetry(ctx, http.MethodPost, "/api/create", reqBody, nil)
}

// ListModels delegates to the underlying OllamaClient to list available models.
func (m *ModelfileManager) ListModels(ctx context.Context) ([]string, error) {
	ctx = WithUseCase(ctx, "list_models")
	return m.client.ListModels(ctx)
}
