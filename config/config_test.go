package config

import (
	"slices"
	"testing"
)

func TestLLMProviders(t *testing.T) {
	tests := []struct {
		name       string
		providers  string
		aiProvider string
		want       []string
	}{
		{"default chain", "", "", []string{"claude", "openai", "gemini"}},
		{"explicit chain", "openai, ollama", "", []string{"openai", "ollama"}},
		{"legacy AI_PROVIDER alone", "", "gemini", []string{"gemini"}},
		{"LLM_PROVIDERS wins over AI_PROVIDER", "openai,claude", "gemini", []string{"openai", "claude"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("LLM_PROVIDERS", tt.providers)
			t.Setenv("AI_PROVIDER", tt.aiProvider)
			if got := llmProviders(); !slices.Equal(got, tt.want) {
				t.Errorf("llmProviders() = %v, want %v", got, tt.want)
			}
		})
	}
}
