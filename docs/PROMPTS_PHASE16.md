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

**Files to create:** `internal/enrichment/ollama_health.go`, `scripts/ollama_pull.sh`
**Files to edit:** `internal/enrichment/openai_compat.go`, `internal/enrichment/llm_factory.go`, `docker-compose.yml`, `.env.example`

> **Depends on Parts A + B.** Ollama reuses `OpenAICompatibleClient` — no new client struct.
> **Read this section fully before starting** — there are 3 Ollama-specific details that differ from OpenAI:
> 1. No API key → skip `Authorization` header entirely
> 2. Use `response_format: json_object` instead of prompt-only JSON instruction
> 3. Low-temp tuned Modelfile prevents hallucinated JSON keys

```
Context:
- Ollama exposes OpenAI-compatible API at http://ollama:11434 (Docker) or http://localhost:11434 (local)
- No API key required — skip the Authorization header entirely
- Models must be pulled before use: docker exec groupscout-ollama-1 ollama pull llama3.2
- Ollama supports response_format: {"type": "json_object"} with llama3.2, mistral, llama3.1 models
- Without response_format, local models often wrap JSON in ```json ... ``` — stripMarkdown() handles this fallback

--- JSON OUTPUT APPROACH ---

Claude: prompt-only (system prompt instructs JSON output)
Gemini: responseMimeType: "application/json" in generationConfig
OpenAI / Groq / Mistral / Ollama: response_format: {"type": "json_object"} in request body

Add JSONOutput bool field to OpenAICompatibleClient.
When JSONOutput=true, add to request body: "response_format": {"type": "json_object"}
Factory sets JSONOutput=true for: openai, ollama, groq (check support first — groq may not support it)
Factory sets JSONOutput=false for: mistral (falls back to stripMarkdown)

--- TASK D-T: WRITE TESTS FIRST ---

Add to internal/enrichment/openai_compat_test.go:

TestOpenAICompatClient_NoAuthHeaderWhenKeyEmpty:
  client with apiKey="" makes a request
  httptest.NewServer captures the request
  Assert: no "Authorization" header in request
  Assert: request succeeds normally

TestOpenAICompatClient_JSONOutputMode:
  client with JSONOutput=true makes a request
  httptest.NewServer captures the request body
  Parse body as JSON
  Assert: body["response_format"]["type"] == "json_object"

TestOpenAICompatClient_JSONOutputFalseOmitsField:
  client with JSONOutput=false makes a request
  Assert: body has no "response_format" key

TestOpenAICompatClient_StripMarkdownFallback:
  server returns: {"choices":[{"message":{"content":"```json\n{\"priority_score\":8}\n```"}}]}
  Assert: Complete() returns {"priority_score":8} (markdown stripped)

All 4 tests fail before impl. Commit.

--- TASK D1: UPDATE openai_compat.go ---

Add JSONOutput bool field to OpenAICompatibleClient struct:
  type OpenAICompatibleClient struct {
      baseURL    string
      apiKey     string
      model      string
      JSONOutput bool    // adds response_format: json_object to request
      client     *http.Client
  }

Update Complete():
  1. Build messages array from req.System + req.User (same as before)
  2. Build request body map:
       body := map[string]any{
           "model":      c.model,
           "max_tokens": req.MaxTokens,
           "messages":   messages,
       }
       if c.JSONOutput {
           body["response_format"] = map[string]any{"type": "json_object"}
       }
  3. Set Authorization header only when apiKey != "":
       if c.apiKey != "" {
           req.Header.Set("Authorization", "Bearer " + c.apiKey)
       }
  4. Apply stripMarkdown() to the response text before returning
     (belt-and-suspenders: some models ignore response_format)

--- TASK D2: UPDATE llm_factory.go ---

Add "ollama" case to NewLLMClient():
  case "ollama":
      baseURL := getEnv("LLM_BASE_URL", "http://ollama:11434")
      if cfg.LLMModel == "" {
          return nil, fmt.Errorf("LLM_MODEL must be set when LLM_PROVIDER=ollama (e.g. llama3.2)")
      }
      return &OpenAICompatibleClient{
          baseURL:    baseURL,
          apiKey:     "",       // no auth for Ollama
          model:      cfg.LLMModel,
          JSONOutput: true,
      }, nil

--- TASK D3: CREATE internal/enrichment/ollama_health.go ---

func CheckOllamaHealth(ctx context.Context, baseURL, model string) error:
  GET {baseURL}/api/tags
  Parse response: {"models": [{"name": "llama3.2:latest"}, ...]}
  Check if any model name has model as a prefix (e.g. "llama3.2" matches "llama3.2:latest")
  If not found:
    return fmt.Errorf("model %q not pulled in Ollama — run: docker exec groupscout-ollama-1 ollama pull %s", model, model)
  Return nil if found or if request fails with connection refused (Ollama not running — caller decides)

Write test: TestCheckOllamaHealth_ModelFound, TestCheckOllamaHealth_ModelMissing (httptest)

--- TASK D4: UPDATE docker-compose.yml ---

Add after the postgres service:

  ollama:
    image: ollama/ollama
    volumes:
      - ollama_data:/root/.ollama
      - ./config:/config:ro   # for Modelfile access (Phase 16-G)
    profiles: ["ollama"]
    # First-time setup: docker exec groupscout-ollama-1 ollama pull llama3.2
    # Custom persona:   docker exec groupscout-ollama-1 ollama create groupscout -f /config/Modelfile.groupscout

Add to volumes section:
  ollama_data:

--- TASK D5: CREATE scripts/ollama_pull.sh ---

  #!/bin/bash
  MODEL=${1:-llama3.2}
  echo "Pulling $MODEL into Ollama container..."
  docker exec groupscout-ollama-1 ollama pull "$MODEL"
  echo "Done. Set LLM_MODEL=$MODEL in .env"

chmod +x scripts/ollama_pull.sh

--- TASK D6: UPDATE .env.example ---

Add section:
  # --- Ollama (local LLM — no API cost, no data leaves server) ---
  # Requires: docker compose --profile ollama up -d
  # Then pull a model: ./scripts/ollama_pull.sh llama3.2
  # LLM_PROVIDER=ollama
  # LLM_MODEL=llama3.2           # recommended: llama3.2 (fast) or mistral (quality)
  # LLM_BASE_URL=http://ollama:11434

--- TASK D7: UPDATE docs/SETUP.md ---

See "Ollama Setup" section added separately.

--- TASK D8: VERIFY ---

  docker compose --profile ollama up -d --build
  ./scripts/ollama_pull.sh llama3.2
  # In .env: LLM_PROVIDER=ollama LLM_MODEL=llama3.2
  curl -X POST http://localhost:8080/run -H "Authorization: Bearer YOUR_TOKEN"
  # Check logs: "LLM provider: ollama" and NO calls to api.anthropic.com or generativelanguage.googleapis.com
  # Check leads table: new leads created with valid JSON enrichment
  go test ./internal/enrichment/... -v
```

---

## Part G — Ollama Modelfile & Persona Tuning

**Files to create:** `config/Modelfile.groupscout`, `scripts/ollama_create_model.sh`
**Files to edit:** `cmd/smoketest/main.go`, `docker-compose.yml`

```
Context:
- Raw llama3.2 follows JSON schema ~80% of the time — occasional extra fields, wrong types, or markdown wrapping
- A Modelfile bakes the system prompt + JSON constraint + temperature into the model at creation time
- The custom model is named "groupscout" — set LLM_MODEL=groupscout in .env
- temperature 0.1 makes output highly deterministic — critical for structured extraction

--- TASK G1: CREATE config/Modelfile.groupscout ---

FROM llama3.2

SYSTEM You are a hotel sales analyst for Sandman Hotel Vancouver Airport in Richmond, BC (near YVR). \
You analyze construction permits, project awards, and event signals to identify groups that need hotel room blocks. \
You MUST respond with ONLY valid JSON — no prose, no markdown fences, no explanation text. \
If a field is unknown, use null for strings or 0 for numbers. \
\
Required JSON schema (all fields required): \
{"general_contractor":"string","project_type":"civil|commercial|industrial|utility|residential|unknown", \
"estimated_crew_size":0,"estimated_duration_months":0,"out_of_town_crew_likely":false, \
"priority_score":0,"priority_reason":"string","suggested_outreach_timing":"string","notes":"string"}

PARAMETER temperature 0.1
PARAMETER num_predict 512
PARAMETER stop "```"

--- TASK G2: CREATE scripts/ollama_create_model.sh ---

  #!/bin/bash
  echo "Creating groupscout model from Modelfile..."
  docker exec groupscout-ollama-1 ollama create groupscout -f /config/Modelfile.groupscout
  echo "Done. Set LLM_MODEL=groupscout in .env"

chmod +x scripts/ollama_create_model.sh

--- TASK G-T: ADD -compare-providers TO cmd/smoketest/main.go ---

Add flag: -compare-providers
When set:
  - Load 3 hardcoded fixture RawProject structs (a Richmond permit, a VCC event, a news item)
  - Run enrichment with the PRIMARY provider (cfg.LLMProvider)
  - Run enrichment with a SECONDARY provider (from COMPARE_LLM_PROVIDER env var)
  - Print side-by-side table:
      Source     | Claude score | Ollama score | Claude crew | Ollama crew | Match?
      richmond   |     8        |     7        |    120      |    100      | close
  - No assertions — visual comparison for the developer

Write test: TestCompareFlagExists (just confirms the flag is registered — compile test)

--- RECOMMENDED MODEL TABLE ---

| Model         | Size  | Best for                          | JSON reliability |
|---------------|-------|-----------------------------------|-----------------|
| llama3.2      | 2GB   | Fast permit extraction, dev/test  | ~85% with response_format |
| groupscout    | 2GB   | llama3.2 + custom persona         | ~95% — recommended default |
| mistral       | 4GB   | Higher quality scoring rationale  | ~90% with response_format |
| llama3.1:8b   | 5GB   | Best outreach email drafts        | ~88% |
| phi3:mini     | 2GB   | Pre-scoring only (cheap + fast)   | ~75% — use only for scorer |

See docs/AI_DATA_STRATEGY.md for the full comparison table.

--- TASK G4: UPDATE docs/AI_DATA_STRATEGY.md ---

See AI_DATA_STRATEGY update below.

--- VERIFY ---

  ./scripts/ollama_pull.sh llama3.2
  ./scripts/ollama_create_model.sh
  # In .env: LLM_MODEL=groupscout
  go run ./cmd/smoketest/ -compare-providers
  # Review table: groupscout scores should be close to Claude baseline
```
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
