# PROMPTS_PHASE16.md — LLM Provider Abstraction

> Copy-paste prompts for each part of Phase 16.
> Parts must be done in order: A → B → C → D → E.
>
> **Goal:** All LLM calls go through a single `LLMClient` interface. Provider is selected by `LLM_PROVIDER` env var. No pipeline code changes when switching providers.
>
> **Full design + provider comparison:** `docs/AI_DATA_STRATEGY.md`
>
> **Test conventions:**
> - Standard `testing` package only (no testify)
> - Table-driven tests using `[]struct{ name string; ... }` slices
> - `t.Errorf()` for assertions
> - Same package as production code
> - Run unit tests: `go test ./...`

---

## Current state (before Phase 16)

- `internal/enrichment/claude.go` — `ClaudeEnricher` struct, implements `EnricherAI`
- `internal/enrichment/gemini.go` — `GeminiEnricher` struct, implements `EnricherAI`
- Both duplicate the same 6 `*Prompt()` functions by importing from `claude.go`
- `EnricherAI` interface in `enricher.go`: `Enrich(ctx, RawProject) (*EnrichedLead, error)` + `DraftOutreach(ctx, Lead) (string, error)`
- `main.go` selects provider inline: `if cfg.AIProvider == "gemini" { ai = NewGeminiEnricher(...) } else { ai = NewClaudeEnricher(...) }`
- Config field: `AIProvider string` from `AI_PROVIDER` env var

---

## Part A — Interface Extraction (internal refactor, no behavior change)

**Files to create:** `internal/enrichment/llm.go`, `internal/enrichment/llm_factory.go`
**Files to edit:** `internal/enrichment/claude.go`, `internal/enrichment/gemini.go`, `internal/enrichment/enricher.go`, `config/config.go`, `cmd/server/main.go`

```
Context:
- claude.go has ClaudeEnricher implementing EnricherAI (Enrich + DraftOutreach)
- gemini.go has GeminiEnricher implementing EnricherAI
- Both make HTTP calls to their respective APIs using the shared *Prompt() functions in claude.go
- enricher.go has: type EnricherAI interface { Enrich(...) DraftOutreach(...) }
- main.go: if cfg.AIProvider == "gemini" { ai = NewGeminiEnricher(...) } else { ai = NewClaudeEnricher(...) }

Task A1 — create internal/enrichment/llm.go:
  Define the LLMClient interface:
    type LLMClient interface {
        Complete(ctx context.Context, req CompletionRequest) (string, error)
    }
  Define CompletionRequest struct:
    type CompletionRequest struct {
        System    string
        User      string
        MaxTokens int
    }

Task A2 — refactor claude.go:
  - Rename ClaudeEnricher → ClaudeClient
  - Remove Enrich() and DraftOutreach() methods
  - Add Complete(ctx, CompletionRequest) (string, error) — sends the Anthropic Messages API
    request using req.System and req.User; returns the raw text string
  - Keep all *Prompt() functions and systemPrompt const (they are used by the enricher)
  - Keep extractText(), stripMarkdown() helpers
  - NewClaudeClient(apiKey string) *ClaudeClient

Task A3 — refactor gemini.go:
  - Rename GeminiEnricher → GeminiClient
  - Remove Enrich() and DraftOutreach() methods
  - Add Complete(ctx, CompletionRequest) (string, error) — sends Gemini API request
    using req.System + "\n\n" + req.User as the combined text; returns raw text
  - Keep extractGeminiText() helper
  - NewGeminiClient(apiKey string) *GeminiClient

Task A4 — update enricher.go:
  - Replace EnricherAI interface and the `ai EnricherAI` field with `llm LLMClient`
  - Move Enrich logic into processProject():
      userContent := selectPrompt(p)  // same switch as before
      req := CompletionRequest{System: systemPrompt, User: userContent, MaxTokens: 512}
      text, err := e.llm.Complete(ctx, req)
      // parse JSON into EnrichedLead (same as before)
  - Move DraftOutreach logic into a method on Enricher:
      func (e *Enricher) DraftOutreach(ctx context.Context, l storage.Lead) (string, error)
      Uses e.llm.Complete() with the outreach system prompt and lead details
  - Update NewEnricher() signature: replace ai EnricherAI with llm LLMClient

Task A5 — create internal/enrichment/llm_factory.go:
  func NewLLMClient(provider, claudeKey, geminiKey string) (LLMClient, error)
  - "claude" (default) → ClaudeClient
  - "gemini" → GeminiClient
  - unknown provider → return error

Task A6 — update config/config.go:
  - Keep existing AIProvider field for now (it maps to LLM_PROVIDER)
  - Add LLMProvider string from getEnv("LLM_PROVIDER", cfg.AIProvider)
    so both LLM_PROVIDER and AI_PROVIDER work during transition
  - Note: LLMAPIKey, LLMModel, LLMBaseURL will be added in Part B

Task A7 — update cmd/server/main.go:
  - Replace inline provider switch with: ai, err := enrichment.NewLLMClient(cfg.LLMProvider, cfg.ClaudeAPIKey, cfg.GeminiAPIKey)
  - Pass ai (LLMClient) to NewEnricher()

Verify:
  go test ./...                          # all existing tests pass
  go build ./...                         # clean compile
  docker compose up -d --build           # pipeline output unchanged
```

---

## Part B — OpenAI-Compatible Client

**File to create:** `internal/enrichment/openai_compat.go`
**Files to edit:** `internal/enrichment/llm_factory.go`, `config/config.go`, `.env.example`

```
Context:
- OpenAI, Groq, and Mistral all use POST /v1/chat/completions with identical request/response format
- Request body: { "model": "...", "max_tokens": N, "messages": [{"role": "system", "content": "..."}, {"role": "user", "content": "..."}] }
- Response: parse choices[0].message.content
- Auth: Authorization: Bearer {API_KEY} header

Task B1 — create internal/enrichment/openai_compat.go:
  type OpenAICompatibleClient struct {
      baseURL string
      apiKey  string
      model   string
      client  *http.Client
  }
  func NewOpenAICompatibleClient(baseURL, apiKey, model string) *OpenAICompatibleClient
  func (c *OpenAICompatibleClient) Complete(ctx context.Context, req CompletionRequest) (string, error)
    - POST to baseURL + "/v1/chat/completions"
    - Build messages array from req.System (role: "system") and req.User (role: "user")
    - Parse response: choices[0].message.content
    - Return the content string

Task B2 — update llm_factory.go:
  Add these providers to NewLLMClient():
  - "openai"  → OpenAICompatibleClient, baseURL "https://api.openai.com"
  - "groq"    → OpenAICompatibleClient, baseURL "https://api.groq.com/openai"
  - "mistral" → OpenAICompatibleClient, baseURL "https://api.mistral.ai"
  Accept apiKey and model from config (add LLMAPIKey, LLMModel params)

Task B3 — update config/config.go:
  Add fields:
    LLMAPIKey  string  // from LLM_API_KEY env var
    LLMModel   string  // from LLM_MODEL env var (no default — providers have different model names)
    LLMBaseURL string  // from LLM_BASE_URL env var (optional override)
  Provider-specific defaults for LLMModel in the factory, not in config

Task B4 — update .env.example:
  Add section:
    # LLM Provider (default: claude)
    # LLM_PROVIDER=claude         # claude | gemini | openai | groq | mistral | azure | ollama
    # LLM_MODEL=claude-haiku-4-5-20251001
    # LLM_API_KEY=                # used for non-Claude providers

Verify:
  Set LLM_PROVIDER=openai LLM_MODEL=gpt-4o-mini LLM_API_KEY=sk-... in .env
  docker compose up -d --build
  curl -X POST http://localhost:8080/run -H "Authorization: Bearer YOUR_TOKEN"
  Check logs: enrichment succeeds, JSON parsed correctly
```

---

## Part C — Azure OpenAI

**File to edit:** `internal/enrichment/openai_compat.go`, `internal/enrichment/llm_factory.go`, `config/config.go`, `.env.example`

```
Context:
- Azure OpenAI uses the same request/response format as OpenAI
- URL format: https://{resource}.openai.azure.com/openai/deployments/{deployment}/chat/completions?api-version={version}
- Auth header is "api-key: {key}" NOT "Authorization: Bearer {key}"

Task C1 — update openai_compat.go:
  Add an azureMode bool field to OpenAICompatibleClient.
  When azureMode is true:
    - Use "api-key" header instead of "Authorization: Bearer"
    - Do not prepend "/v1/chat/completions" — use baseURL as-is (caller provides full Azure URL)

Task C2 — update llm_factory.go:
  Add "azure" provider:
    - Build URL: fmt.Sprintf("https://%s.openai.azure.com/openai/deployments/%s/chat/completions?api-version=%s",
        cfg.AzureResourceName, cfg.AzureDeploymentName, cfg.AzureAPIVersion)
    - Use NewOpenAICompatibleClient with azureMode=true

Task C3 — update config/config.go:
  Add fields:
    AzureResourceName   string  // from AZURE_RESOURCE_NAME
    AzureDeploymentName string  // from AZURE_DEPLOYMENT_NAME
    AzureAPIVersion     string  // from getEnv("AZURE_API_VERSION", "2024-02-01")

Task C4 — update .env.example:
  # Azure OpenAI (only needed if LLM_PROVIDER=azure)
  # AZURE_RESOURCE_NAME=my-resource
  # AZURE_DEPLOYMENT_NAME=gpt-4o-mini
  # AZURE_API_VERSION=2024-02-01
  # LLM_API_KEY=your-azure-api-key
```

---

## Part D — Ollama (local / Docker)

**Files to edit:** `internal/enrichment/llm_factory.go`, `docker-compose.yml`, `.env.example`

```
Context:
- Ollama exposes an OpenAI-compatible API at http://ollama:11434 (in Docker) or http://localhost:11434 (local)
- No API key required
- Models must be pulled before use: docker exec groupscout-ollama-1 ollama pull llama3.2

Task D1 — update llm_factory.go:
  Add "ollama" provider:
    - baseURL: getEnv("LLM_BASE_URL", "http://ollama:11434")
    - model: cfg.LLMModel (no default — user must set LLM_MODEL=llama3.2 or similar)
    - apiKey: "" (empty — Ollama needs no auth)
    - In OpenAICompatibleClient.Complete(): skip Authorization header when apiKey is empty

Task D2 — update docker-compose.yml:
  Add ollama service (disabled by default — only used when LLM_PROVIDER=ollama):
    ollama:
      image: ollama/ollama
      volumes:
        - ollama_data:/root/.ollama
      profiles: ["ollama"]   # opt-in: docker compose --profile ollama up

  Add ollama_data to the volumes section.

  Add comment above the service explaining how to pull a model after first start:
    # docker exec groupscout-ollama-1 ollama pull llama3.2

Task D3 — update .env.example:
  # Ollama (only needed if LLM_PROVIDER=ollama)
  # LLM_PROVIDER=ollama
  # LLM_MODEL=llama3.2
  # LLM_BASE_URL=http://ollama:11434   # default when using Docker Compose

Verify:
  docker compose --profile ollama up -d --build
  docker exec groupscout-ollama-1 ollama pull llama3.2
  Set LLM_PROVIDER=ollama LLM_MODEL=llama3.2 in .env
  curl -X POST http://localhost:8080/run -H "Authorization: Bearer YOUR_TOKEN"
  Confirm: no external API calls, enrichment succeeds (quality will be lower than Claude)
```

---

## Part E — Fallback & Resilience

**Files to edit:** `internal/enrichment/llm.go`, `internal/enrichment/llm_factory.go`, `config/config.go`

```
Context:
- If the primary LLM provider fails (API down, rate limit, bad key), the pipeline currently
  logs the error and skips the lead. With a fallback, it retries with a secondary provider.
- Sentry is already wired in enricher.go for error capture.

Task E1 — update llm.go:
  Add FallbackClient struct:
    type FallbackClient struct {
        primary   LLMClient
        secondary LLMClient
    }
    func (f *FallbackClient) Complete(ctx context.Context, req CompletionRequest) (string, error)
      - Try primary.Complete()
      - If error: log the error (use logger.Log), capture to Sentry, try secondary.Complete()
      - If secondary also fails: return the secondary error

Task E2 — update config/config.go:
  Add fields:
    LLMFallbackProvider string  // from LLM_FALLBACK_PROVIDER
    LLMFallbackModel    string  // from LLM_FALLBACK_MODEL
    LLMFallbackAPIKey   string  // from LLM_FALLBACK_API_KEY
    LLMFallbackBaseURL  string  // from LLM_FALLBACK_BASE_URL (for azure/ollama fallback)

Task E3 — update llm_factory.go:
  After building the primary LLMClient, check if LLMFallbackProvider is set.
  If yes: build a secondary LLMClient using the same NewLLMClient() logic with fallback params,
  then wrap both in FallbackClient.
  If no: return primary directly.

Task E4 — update .env.example:
  # Fallback LLM provider (optional — activates if primary fails)
  # LLM_FALLBACK_PROVIDER=openai
  # LLM_FALLBACK_MODEL=gpt-4o-mini
  # LLM_FALLBACK_API_KEY=sk-...

Verify:
  Set LLM_API_KEY to an invalid key, LLM_FALLBACK_PROVIDER=openai with a valid key.
  docker compose up -d --build
  curl -X POST http://localhost:8080/run -H "Authorization: Bearer YOUR_TOKEN"
  Check logs: primary failure logged, fallback activates, pipeline completes.
  Check Sentry: primary error captured.
```

---

---

## Part F — Gemini Factory Integration

**Files to edit:** `internal/enrichment/llm_factory.go`, `internal/enrichment/gemini.go` (if not already implementing `LLMClient`), `config/config.go`, `.env.example`

> `internal/enrichment/gemini.go` was added externally. This part wires it into the Phase 16 factory so `LLM_PROVIDER=gemini` works without any inline switch in `main.go`.

```
Context:
- After Part A, gemini.go has GeminiClient implementing LLMClient (Complete method)
- llm_factory.go currently has: "claude" and possibly "gemini" already
- Verify gemini.go's Complete() sends the correct Gemini API format:
    POST https://generativelanguage.googleapis.com/v1beta/models/{model}:generateContent?key={apiKey}
    Body: { "contents": [{ "parts": [{ "text": system + "\n\n" + user }] }] }
    Response: candidates[0].content.parts[0].text

Task F1 — verify gemini.go implements LLMClient:
  - Check that GeminiClient has Complete(ctx, CompletionRequest) (string, error)
  - If it still has the old Enrich/DraftOutreach signature, refactor per Part A Task A3
  - extractGeminiText() helper should parse candidates[0].content.parts[0].text

Task F2 — update llm_factory.go:
  Ensure NewLLMClient() handles "gemini":
    case "gemini":
        return NewGeminiClient(cfg.GeminiAPIKey), nil  // or cfg.LLMAPIKey if unified
  Check: if "gemini" is already in the factory from Part A, this task is just verification

Task F3 — update config/config.go:
  - Ensure GEMINI_API_KEY env var is loaded (may already exist from before Phase 16)
  - Add note: when LLM_PROVIDER=gemini, the LLM_API_KEY takes precedence over GEMINI_API_KEY
    so both env var names work during transition

Task F4 — update .env.example:
  Add Gemini section:
    # Google Gemini
    # LLM_PROVIDER=gemini
    # LLM_MODEL=gemini-1.5-flash
    # LLM_API_KEY=AIza...          # or keep GEMINI_API_KEY for backward compat

Task F-T (write first) — internal/enrichment/gemini_test.go:
  If the file doesn't exist, write:
  - TestGeminiClient_Complete_ParsesResponse: httptest.NewServer returning fixture Gemini JSON;
    assert correct text extracted from candidates[0].content.parts[0].text
  - TestGeminiClient_Complete_NonOKError: server returns 400; assert error propagated
  - Run: all tests fail before any refactor; commit; then implement

Verify:
  Set LLM_PROVIDER=gemini LLM_MODEL=gemini-1.5-flash LLM_API_KEY=AIza... in .env
  docker compose up -d --build
  curl -X POST http://localhost:8080/run -H "Authorization: Bearer YOUR_TOKEN"
  Check logs: Gemini API called, enrichment JSON returned correctly.
  go test ./internal/enrichment/... — all tests green.
```

---

## Reference files

| File | Role |
|---|---|
| `internal/enrichment/claude.go` | ClaudeEnricher → ClaudeClient; all *Prompt() functions live here |
| `internal/enrichment/gemini.go` | GeminiEnricher → GeminiClient (added externally, wired in Part F) |
| `internal/enrichment/enricher.go` | EnricherAI interface → llm LLMClient field |
| `config/config.go` | AIProvider (existing) + new LLM* fields |
| `cmd/server/main.go` | Provider selection: inline switch → llm_factory |
| `docs/AI_DATA_STRATEGY.md` | Provider comparison table + full design rationale |
