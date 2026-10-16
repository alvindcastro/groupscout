package llm

import (
	"fmt"
	"strings"
	"time"
)

// Provider names accepted in LLM_PROVIDERS.
const (
	ProviderClaude = "claude"
	ProviderOpenAI = "openai"
	ProviderGemini = "gemini"
	ProviderOllama = "ollama"
	// ProviderCustom is any other OpenAI-format endpoint: LM Studio,
	// llama.cpp, vLLM, LiteLLM, OpenRouter, Groq, Azure OpenAI and so on.
	ProviderCustom = "custom"
)

// Defaults for providers whose model is not configured. Every one can be
// overridden with its *_MODEL variable.
const (
	DefaultOpenAIModel = "gpt-4.1-mini"
	DefaultGeminiModel = "gemini-2.5-flash"

	openAIBaseURL = "https://api.openai.com/v1"
	geminiBaseURL = "https://generativelanguage.googleapis.com/v1beta/openai"
)

// Settings holds what every provider needs. BuildClients only builds the
// providers named in Order.
type Settings struct {
	Order []string

	AnthropicAPIKey string
	AnthropicModel  string

	OpenAIAPIKey          string
	OpenAIModel           string
	OpenAIBaseURL         string
	OpenAIReasoningEffort string

	GeminiAPIKey string
	GeminiModel  string

	OllamaEndpoint string
	OllamaModel    string

	CustomBaseURL  string
	CustomAPIKey   string
	CustomModel    string
	CustomJSONMode bool

	// Timeout bounds one call to a cloud provider; LocalTimeout one call to
	// Ollama or a custom endpoint, which may run on a CPU.
	Timeout      time.Duration
	LocalTimeout time.Duration
}

// BuildClients returns clients for s.Order, in order. Unknown names,
// repeats, and providers missing a key or endpoint are left out, and each is
// explained in skipped.
func BuildClients(s Settings) (clients []Client, skipped []string) {
	seen := make(map[string]bool)
	for _, name := range s.Order {
		name = canonicalProvider(name)
		if name == "" {
			continue
		}
		if seen[name] {
			skipped = append(skipped, name+": listed more than once")
			continue
		}
		seen[name] = true
		c, err := buildClient(name, s)
		if err != nil {
			skipped = append(skipped, err.Error())
			continue
		}
		clients = append(clients, c)
	}
	return clients, skipped
}

func canonicalProvider(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "anthropic" {
		return ProviderClaude
	}
	return name
}

func buildClient(name string, s Settings) (Client, error) {
	switch name {
	case ProviderClaude:
		if s.AnthropicAPIKey == "" {
			return nil, fmt.Errorf("%s: CLAUDE_API_KEY is not set", name)
		}
		return NewAnthropic(s.AnthropicAPIKey, s.AnthropicModel, "", s.Timeout), nil
	case ProviderOpenAI:
		if s.OpenAIAPIKey == "" {
			return nil, fmt.Errorf("%s: OPENAI_API_KEY is not set", name)
		}
		return NewOpenAI(OpenAIConfig{
			Provider:        name,
			BaseURL:         orDefault(s.OpenAIBaseURL, openAIBaseURL),
			APIKey:          s.OpenAIAPIKey,
			Model:           orDefault(s.OpenAIModel, DefaultOpenAIModel),
			TokenParam:      TokenParamMaxCompletionTokens,
			ReasoningEffort: s.OpenAIReasoningEffort,
			JSONMode:        true,
			Timeout:         s.Timeout,
		}), nil
	case ProviderGemini:
		if s.GeminiAPIKey == "" {
			return nil, fmt.Errorf("%s: GEMINI_API_KEY is not set", name)
		}
		// No output limit: Gemini's thinking models count thinking tokens
		// against it, and the prompt already bounds the reply.
		return NewOpenAI(OpenAIConfig{
			Provider:   name,
			BaseURL:    geminiBaseURL,
			APIKey:     s.GeminiAPIKey,
			Model:      orDefault(s.GeminiModel, DefaultGeminiModel),
			TokenParam: TokenParamNone,
			JSONMode:   true,
			Timeout:    s.Timeout,
		}), nil
	case ProviderOllama:
		if s.OllamaEndpoint == "" || s.OllamaModel == "" {
			return nil, fmt.Errorf("%s: OLLAMA_ENDPOINT and OLLAMA_MODEL must be set", name)
		}
		return NewOpenAI(OpenAIConfig{
			Provider:   name,
			BaseURL:    strings.TrimRight(s.OllamaEndpoint, "/") + "/v1",
			Model:      s.OllamaModel,
			TokenParam: TokenParamMaxTokens,
			JSONMode:   true,
			Timeout:    s.LocalTimeout,
		}), nil
	case ProviderCustom:
		if s.CustomBaseURL == "" || s.CustomModel == "" {
			return nil, fmt.Errorf("%s: CUSTOM_LLM_BASE_URL and CUSTOM_LLM_MODEL must be set", name)
		}
		return NewOpenAI(OpenAIConfig{
			Provider:   name,
			BaseURL:    s.CustomBaseURL,
			APIKey:     s.CustomAPIKey,
			Model:      s.CustomModel,
			TokenParam: TokenParamMaxTokens,
			JSONMode:   s.CustomJSONMode,
			Timeout:    s.LocalTimeout,
		}), nil
	}
	return nil, fmt.Errorf("%s: unknown provider (use claude, openai, gemini, ollama or custom)", name)
}

func orDefault(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}
