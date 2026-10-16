package llm

import (
	"strings"
	"testing"
	"time"
)

func TestBuildClients_orderKeysAndSkips(t *testing.T) {
	clients, skipped := BuildClients(Settings{
		Order:           []string{"Anthropic", " openai ", "gemini", "claude", "ollama", "custom", "mystery"},
		AnthropicAPIKey: "sk-ant",
		OpenAIAPIKey:    "sk-openai",
		OllamaEndpoint:  "http://ollama:11434/",
		OllamaModel:     "qwen2.5:7b",
		Timeout:         time.Minute,
		LocalTimeout:    3 * time.Minute,
	})

	var got []string
	for _, c := range clients {
		got = append(got, c.Provider()+"/"+c.Model())
	}
	want := []string{"claude/" + DefaultAnthropicModel, "openai/" + DefaultOpenAIModel, "ollama/qwen2.5:7b"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("clients = %v, want %v", got, want)
	}

	joined := strings.Join(skipped, "\n")
	for _, reason := range []string{"gemini: GEMINI_API_KEY is not set", "claude: listed more than once", "custom: CUSTOM_LLM_BASE_URL", "mystery: unknown provider"} {
		if !strings.Contains(joined, reason) {
			t.Errorf("skipped lacks %q:\n%s", reason, joined)
		}
	}

	ollama := clients[2].(*OpenAI)
	if ollama.cfg.BaseURL != "http://ollama:11434/v1" || ollama.cfg.Timeout != 3*time.Minute || ollama.cfg.TokenParam != TokenParamMaxTokens {
		t.Errorf("ollama config = %+v", ollama.cfg)
	}
	openai := clients[1].(*OpenAI)
	if openai.cfg.BaseURL != openAIBaseURL || openai.cfg.TokenParam != TokenParamMaxCompletionTokens || !openai.cfg.JSONMode {
		t.Errorf("openai config = %+v", openai.cfg)
	}
}

func TestBuildClients_geminiAndCustom(t *testing.T) {
	clients, skipped := BuildClients(Settings{
		Order:          []string{"gemini", "custom"},
		GeminiAPIKey:   "g-key",
		GeminiModel:    "gemini-test",
		CustomBaseURL:  "http://lmstudio:1234/v1",
		CustomModel:    "local-model",
		CustomJSONMode: false,
	})
	if len(skipped) != 0 || len(clients) != 2 {
		t.Fatalf("clients %d, skipped %v", len(clients), skipped)
	}
	gemini := clients[0].(*OpenAI).cfg
	if gemini.BaseURL != geminiBaseURL || gemini.Model != "gemini-test" || gemini.TokenParam != TokenParamNone || gemini.APIKey != "g-key" {
		t.Errorf("gemini config = %+v", gemini)
	}
	custom := clients[1].(*OpenAI).cfg
	if custom.Provider != "custom" || custom.APIKey != "" || custom.JSONMode {
		t.Errorf("custom config = %+v", custom)
	}
}
