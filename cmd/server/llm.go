package main

import (
	"strings"
	"sync"
	"time"

	"github.com/alvindcastro/groupscout/config"
	"github.com/alvindcastro/groupscout/internal/llm"
	"github.com/alvindcastro/groupscout/internal/logger"
)

var (
	aiChainOnce sync.Once
	aiChain     *llm.Chain
)

// sharedAIChain returns the process-wide AI provider chain. It is shared by
// every run and /ingest call so a provider parked after an account error
// stays parked across them, and /health can report it.
func sharedAIChain(cfg *config.Config) *llm.Chain {
	aiChainOnce.Do(func() {
		clients, skipped := llm.BuildClients(llmSettings(cfg))
		for _, reason := range skipped {
			logger.Log.Warn("AI provider left out of the chain", "reason", reason)
		}
		aiChain = llm.NewChain(clients...)
		aiChain.Logger = logger.Log
		if len(clients) == 0 {
			logger.Log.Error("no AI provider is configured; enrichment will fail",
				"providers", strings.Join(cfg.LLMProviders, ","))
			return
		}
		var order []string
		for _, c := range clients {
			order = append(order, c.Provider()+"/"+c.Model())
		}
		logger.Log.Info("AI provider chain", "order", strings.Join(order, " -> "))
	})
	return aiChain
}

func llmSettings(cfg *config.Config) llm.Settings {
	return llm.Settings{
		Order:                 cfg.LLMProviders,
		AnthropicAPIKey:       cfg.ClaudeAPIKey,
		AnthropicModel:        cfg.ClaudeModel,
		OpenAIAPIKey:          cfg.OpenAIAPIKey,
		OpenAIModel:           cfg.OpenAIModel,
		OpenAIBaseURL:         cfg.OpenAIBaseURL,
		OpenAIReasoningEffort: cfg.OpenAIReasoningEffort,
		GeminiAPIKey:          cfg.GeminiAPIKey,
		GeminiModel:           cfg.GeminiModel,
		OllamaEndpoint:        cfg.OllamaEndpoint,
		OllamaModel:           cfg.OllamaModel,
		CustomBaseURL:         cfg.CustomLLMBaseURL,
		CustomAPIKey:          cfg.CustomLLMAPIKey,
		CustomModel:           cfg.CustomLLMModel,
		CustomJSONMode:        cfg.CustomLLMJSONMode,
		Timeout:               time.Duration(cfg.LLMTimeoutS) * time.Second,
		LocalTimeout:          time.Duration(cfg.LLMLocalTimeoutS) * time.Second,
	}
}

// aiStatus summarises the chain for /health: "ok" when every provider is in
// use, "degraded" when some are parked, "down" when all are, and
// "unconfigured" when there are none.
func aiStatus(providers []llm.ProviderStatus) (summary, detail string) {
	if len(providers) == 0 {
		return "unconfigured", ""
	}
	parts := make([]string, len(providers))
	available := 0
	for i, p := range providers {
		state := "ok"
		if p.Available {
			available++
		} else {
			state = "parked"
		}
		parts[i] = p.Provider + ":" + state
	}
	switch available {
	case len(providers):
		summary = "ok"
	case 0:
		summary = "down"
	default:
		summary = "degraded"
	}
	return summary, strings.Join(parts, ",")
}
