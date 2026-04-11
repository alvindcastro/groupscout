# GroupScout — Ollama Integration Plan

> Local LLM enrichment for lead extraction, scoring rationale, and disruption alert copy.  
> All processing stays on-server. No lead data leaves the host.  
> Repository: github.com/alvindcastro/groupscout

---

## Table of Contents

1. [Why Ollama](#1-why-ollama)
2. [Architecture & Package Layout](#2-architecture--package-layout)
3. [Phase 1 — Core Ollama Client](#phase-1--core-ollama-client)
4. [Phase 2 — NLP Signal Extraction](#phase-2--nlp-signal-extraction)
5. [Phase 3 — Lead Scoring Rationale](#phase-3--lead-scoring-rationale)
6. [Phase 4 — Disruption Alert Copy](#phase-4--disruption-alert-copy)
7. [Phase 5 — Modelfile Management & Hot-Swap](#phase-5--modelfile-management--hot-swap)
8. [Phase 6 — Observability & Fallback](#phase-6--observability--fallback)
9. [Model Reference](#model-reference)
10. [Hardware Requirements](#hardware-requirements)
11. [Environment Variables](#environment-variables)

---

## 1. Why Ollama

GroupScout already calls the Anthropic Claude API for lead enrichment. Ollama adds a **local, zero-cost, privacy-preserving LLM layer** that handles high-volume, lower-stakes NLP tasks — permit text parsing, score rationale generation, disruption copy — so Claude API calls are reserved for final, high-confidence enrichment.

**Key properties:**

- OpenAI-compatible `/v1/chat/completions` endpoint — minimal code delta from existing HTTP client patterns
- Quantized models run on a standard 16 GB VPS (Mistral 7B, Llama 3.1 8B)
- No external API calls — critical for PIPEDA compliance and enterprise hotel groups
- Modelfiles allow persona-stamping per use case without prompt repetition

**Three integration points, in priority order:**

| # | Use Case | Model | Replaces / Augments |
|---|---|---|---|
| 1 | NLP signal extraction | Mistral 7B | Regex + Claude API preprocessing |
| 2 | Lead scoring rationale | Mistral 7B | Raw numeric score in Slack |
| 3 | Disruption alert copy | Llama 3.1 8B | Hardcoded alert text |

---

## 2. Architecture & Package Layout

```
internal/
  ollama/
    client.go          — HTTP client, request/response types, retry logic
    client_test.go     — unit tests (mock HTTP server)
    extractor.go       — NLP signal extraction against raw permit/news text
    extractor_test.go
    scorer.go          — lead scoring rationale generation
    scorer_test.go
    alertcopy.go       — disruption alert copy generation
    alertcopy_test.go
    modelfile/
      permit_extractor.modelfile
      lead_scorer.modelfile
      disruption_alert.modelfile
    testdata/
      permit_raw.txt   — fixture: sample permit filing text
      news_raw.txt     — fixture: sample press release
      disruption.json  — fixture: sample SPS event payload
```

All new code lives under `internal/ollama`. Each `.go` file has a corresponding `_test.go` written **before** the implementation (TDD, red-green-refactor).

---

## Phase 1 — Core Ollama Client

**Goal:** A thin, tested HTTP client that speaks Ollama's `/v1/chat/completions` and `/api/generate` endpoints. All subsequent phases depend on this.

### Tasks

- [x] Write `internal/ollama/client_test.go` — mock HTTP server, assert request shape and response parsing
- [x] Implement `internal/ollama/client.go` — `OllamaClient` struct with `Generate()` and `ChatComplete()` methods
- [x] Add `OLLAMA_ENDPOINT` and `OLLAMA_MODEL` to `.env` and `config/config.go`
- [x] Add `OLLAMA_ENABLED` toggle — if false, client returns empty string without error
- [x] Wire health check: `GET /api/tags` on startup; log warning if Ollama unreachable but do not block boot
- [x] Add to `docker-compose.yml` — `ollama/ollama` container image, GPU passthrough commented out

---

### Detailed Prompt: Phase 1 Implementation

```
You are an experienced Go engineer working on GroupScout, a hotel group demand intelligence tool.
The project is at github.com/alvindcastro/groupscout.
Language: Go 1.22+. Style: standard library preferred, minimal dependencies.
All code must be written test-first (TDD). Write the _test.go file first, then the implementation.

Task: Implement internal/ollama/client.go and internal/ollama/client_test.go.

Requirements:
1. OllamaClient struct with fields:
     Endpoint string  // e.g. "http://localhost:11434"
     Model    string  // e.g. "mistral"
     Timeout  time.Duration
2. Method: Generate(ctx context.Context, prompt string) (string, error)
   - POST to {Endpoint}/api/generate
   - Body: {"model": Model, "prompt": prompt, "stream": false}
   - Parse response field "response" (string)
3. Method: ChatComplete(ctx context.Context, system, user string) (string, error)
   - POST to {Endpoint}/v1/chat/completions
   - Body: OpenAI-compatible messages array with system + user roles
   - Parse choices[0].message.content
4. Method: HealthCheck(ctx context.Context) error
   - GET {Endpoint}/api/tags
   - Return nil if 200, error otherwise
5. OllamaClient must implement an interface LLMClient so it can be mocked in tests.
6. If OLLAMA_ENABLED env var is false, return a NoopClient that satisfies the interface with no-op methods.
7. Tests must use httptest.NewServer to mock the Ollama HTTP server. No real network calls in tests.
8. Use context for all HTTP calls. Respect cancellation.
9. Retry: one retry with 2s backoff on 5xx or connection refused. Do not retry 4xx.

Do not use langchaingo or any third-party Ollama SDK. Implement the HTTP client directly.
```

---

## Phase 2 — NLP Signal Extraction

**Goal:** Replace (or augment) regex-based field extraction in the existing scraper pipeline. Feed raw permit/news text to a local Mistral model; receive structured JSON back.

### Tasks

- [x] Write `internal/ollama/extractor_test.go` — table-driven tests with `testdata/permit_raw.txt` and `testdata/news_raw.txt` fixtures; assert returned `LeadSignal` fields
- [x] Implement `internal/ollama/extractor.go` — `Extractor` struct wrapping `LLMClient`, `Extract(ctx, rawText string) (*LeadSignal, error)` method
- [x] Define `LeadSignal` struct (or reuse existing model if one exists) with fields: `OrgName`, `Location`, `StartDate`, `DurationDays`, `CrewSize`, `ProjectType`, `Confidence float64`
- [x] Write `modelfile/permit_extractor.modelfile` — persona + JSON output instruction
- [x] Add JSON parse guard: strip markdown fences if model wraps output in ```json ... ``` blocks
- [x] Add extraction metrics: log field hit rate (how many of 6 fields were populated) per run
- [x] Wire `Extractor` into `internal/enrichment/` pipeline — call after raw text fetch, before rules-based pre-scorer
- [x] Add `OLLAMA_EXTRACTION_ENABLED` toggle so existing regex path is preserved as fallback

---

### Detailed Prompt: Modelfile — Permit Extractor

Save as `internal/ollama/modelfile/permit_extractor.modelfile`.

```
FROM mistral

SYSTEM """
You are a data extraction assistant for a hotel group sales intelligence system.
You receive raw text from building permit filings, government contract awards,
film production permits, and press releases — all from the Vancouver, BC metro area.

Your job is to extract structured information and return ONLY valid JSON.
Do not include any explanation, preamble, or markdown formatting.
If a field cannot be determined from the text, set it to null.

Output this exact JSON schema:
{
  "org_name": "string or null",
  "location": "string or null — street address or neighbourhood",
  "start_date": "YYYY-MM-DD or null",
  "duration_days": integer or null,
  "crew_size": integer or null,
  "project_type": "one of: construction | film_production | government_contract | conference | sports | other",
  "confidence": float between 0 and 1 — your confidence in the overall extraction
}
"""
```

---

### Detailed Prompt: Phase 2 Implementation

```
You are an experienced Go engineer working on GroupScout (github.com/alvindcastro/groupscout).
TDD: write extractor_test.go before extractor.go.

Task: Implement internal/ollama/extractor.go and internal/ollama/extractor_test.go.

LeadSignal struct (define in this package, or import from internal/storage if it exists):
  OrgName     string
  Location    string
  StartDate   time.Time  (zero value if null)
  DurationDays int
  CrewSize    int
  ProjectType string
  Confidence  float64

Extractor struct:
  client LLMClient

Method: Extract(ctx context.Context, rawText string) (*LeadSignal, error)
  1. Build a prompt from rawText using the permit_extractor persona (system) + rawText (user).
  2. Call client.ChatComplete(ctx, systemPrompt, rawText).
  3. Strip ```json ... ``` fences if present.
  4. json.Unmarshal into a raw map first, then map fields to LeadSignal.
  5. If JSON parse fails, return nil + a wrapped error. Do not panic.
  6. Log field hit rate: count of non-zero fields / 6 (as info log).

Tests:
  - Load testdata/permit_raw.txt and testdata/news_raw.txt as fixtures.
  - Mock LLMClient to return canned JSON responses.
  - Assert OrgName, ProjectType, Confidence are populated correctly.
  - Test JSON fence stripping: mock returns ```json {...} ``` — assert clean parse.
  - Test graceful handling when mock returns malformed JSON — assert error returned, no panic.
  - All tests use table-driven style.

Do not make real HTTP calls in tests.
```

---

## Phase 3 — Lead Scoring Rationale

**Goal:** Each Slack lead card currently shows a numeric priority score. Replace or augment with a 2–3 sentence plain-English rationale explaining *why* the lead scores high — making it actionable without the rep needing to decode the number.

### Tasks

- [ ] Write `internal/ollama/scorer_test.go` — mock LLMClient, assert rationale is non-empty string, test empty lead gracefully
- [ ] Implement `internal/ollama/scorer.go` — `Scorer` struct, `Rationale(ctx, lead Lead) (string, error)` method
- [ ] Write `modelfile/lead_scorer.modelfile` — hotel sales analyst persona
- [ ] Inject `Scorer` into existing Slack notification builder — append rationale to Block Kit message
- [ ] Cap rationale at 280 characters before sending to Slack (prevents overflow)
- [ ] Add `OLLAMA_SCORING_ENABLED` toggle
- [ ] Add `ollama_rationale_latency_ms` to Prometheus metrics if metrics are wired

---

### Detailed Prompt: Modelfile — Lead Scorer

Save as `internal/ollama/modelfile/lead_scorer.modelfile`.

```
FROM mistral

SYSTEM """
You are a senior hotel sales analyst at a Vancouver-area full-service hotel.
You receive structured data about a potential group lodging lead.
Your job is to write 2–3 concise sentences explaining why this lead is strong
(or not), what type of group it represents, and what action the sales rep should take.

Be direct. Use dollar figures and room night estimates where you can infer them.
Do not use bullet points. Write in plain prose.
Keep your response under 250 characters.
"""
```

---

### Detailed Prompt: Phase 3 Implementation

```
You are an experienced Go engineer working on GroupScout (github.com/alvindcastro/groupscout).
TDD: write scorer_test.go before scorer.go.

Task: Implement internal/ollama/scorer.go and internal/ollama/scorer_test.go.

Scorer struct:
  client LLMClient

The Lead type should match what already exists in internal/models (or internal/leads).
If it does not exist, define it inline:
  type Lead struct {
    OrgName     string
    ProjectType string
    Location    string
    DurationDays int
    CrewSize    int
    PriorityScore int
    EstRoomNights int
  }

Method: Rationale(ctx context.Context, lead Lead) (string, error)
  1. Serialize lead to a short summary string (not JSON — readable prose summary for the model).
     Example: "Construction project by ABC Contracting in Richmond, BC. 90 crew, 45 days, priority 8."
  2. Call client.ChatComplete(ctx, systemPrompt, summary).
  3. Trim whitespace. Truncate to 280 chars if longer (truncate at last complete word before 280).
  4. Return rationale string.

Tests:
  - Happy path: mock returns a rationale, assert non-empty and ≤ 280 chars.
  - Truncation: mock returns 400-char string, assert output ≤ 280 chars and ends at a word boundary.
  - Empty lead: all zero values — assert method returns without panicking.
  - Context cancellation: mock hangs, cancel context, assert error is context.Canceled.
```

---

## Phase 4 — Disruption Alert Copy

**Goal:** The `alertd` binary currently uses hardcoded text for Slack alert messages. Replace the "Suggested Actions" block with dynamically generated copy from a local Llama 3.1 8B model, tuned by storm type, time of day, and SPS score.

### Tasks

- [ ] Write `internal/ollama/alertcopy_test.go` — fixture `testdata/disruption.json` with sample SPS event; assert non-empty copy, test each alert state
- [ ] Implement `internal/ollama/alertcopy.go` — `AlertCopyGenerator` struct, `Generate(ctx, event DisruptionEvent) (string, error)` method
- [ ] Write `modelfile/disruption_alert.modelfile` — hotel ops persona
- [ ] Map `DisruptionEvent` fields into a structured prompt (storm type, SPS score, time of day, estimated pax count, duration)
- [ ] Integrate into `cmd/alertd` Slack message builder — replace hardcoded "Suggested actions" section
- [ ] Add `OLLAMA_ALERT_COPY_ENABLED` toggle — fallback to hardcoded template if disabled or Ollama unreachable
- [ ] Prefer `llama3.1:8b` for this use case; fall back to `mistral` if not available

---

### Detailed Prompt: Modelfile — Disruption Alert

Save as `internal/ollama/modelfile/disruption_alert.modelfile`.

```
FROM llama3.1:8b

SYSTEM """
You are a hotel operations assistant at a Vancouver-area airport hotel (near YVR).
You receive real-time flight disruption data and write 2–3 actionable bullet points
for the front desk manager. 

Rules:
- Be specific: name the cause (e.g., atmospheric river, fog), estimated passenger count,
  and earliest recovery time.
- Include a dollar or revenue angle: e.g., "pull back OTA inventory", "activate distressed rate".
- Adjust tone by time of day: assertive urgency after 9pm, calm preparation before 6pm.
- Never use generic filler phrases like "stay informed" or "monitor the situation".
- Output plain text only. Three bullet points maximum. No markdown headers.
"""
```

---

### Detailed Prompt: Phase 4 Implementation

```
You are an experienced Go engineer working on GroupScout (github.com/alvindcastro/groupscout).
TDD: write alertcopy_test.go before alertcopy.go.

Task: Implement internal/ollama/alertcopy.go and internal/ollama/alertcopy_test.go.

DisruptionEvent struct (should match what alertd uses — import from internal/aviation or define here):
  Cause           string   // e.g. "atmospheric_river", "fog", "snow"
  SPS             float64
  AlertState      string   // "watch" | "soft_alert" | "hard_alert" | "resolve"
  FlightsCancelled int
  FlightsScheduled int
  EstStranded     int
  StartedAt       time.Time
  RecoveryETA     time.Time // zero if unknown

AlertCopyGenerator struct:
  client LLMClient

Method: Generate(ctx context.Context, event DisruptionEvent) (string, error)
  1. Build a user prompt from event fields:
     - Duration so far (time.Since(StartedAt), rounded to nearest 15 min)
     - Cancellation percentage (FlightsCancelled/FlightsScheduled * 100)
     - Time of day classification: "morning" / "afternoon" / "evening" / "overnight"
     - RecoveryETA hours from now (or "unknown")
  2. Call client.ChatComplete(ctx, systemPrompt, userPrompt).
  3. Return raw copy string (do not truncate — Slack block can handle up to 3000 chars).

Tests (table-driven, testdata/disruption.json as fixture):
  - Atmospheric river hard alert — assert output non-empty, contains at least one bullet.
  - Fog event — assert output non-empty.
  - Resolve state — assert output mentions "clear" or "resolved" (case-insensitive).
  - Context cancellation mid-call — assert error is context.Canceled.
  - Ollama unavailable (mock returns error) — assert error propagated, no panic.
  - Verify OLLAMA_ALERT_COPY_ENABLED=false returns fallback hardcoded string (no LLM call made).
```

---

## Phase 5 — Modelfile Management & Hot-Swap

**Goal:** Modelfiles should be pushable to the running Ollama server without restarting GroupScout. Useful for iterating on persona prompts in production.

### Tasks

- [ ] Write `internal/ollama/modelfile_manager_test.go` — test `Push()` method with mock HTTP server
- [ ] Implement `internal/ollama/modelfile_manager.go` — `ModelfileManager` struct, `Push(ctx, name, modelfileContent string) error` method using Ollama `/api/create` endpoint
- [ ] Add CLI subcommand: `go run cmd/server/main.go ollama push-models` — reads all `.modelfile` files from `internal/ollama/modelfile/` and pushes to local Ollama
- [ ] Add `ollama list-models` subcommand — calls `/api/tags`, pretty-prints loaded models
- [ ] Document in `DEVELOPER.md`: workflow for updating a persona prompt without downtime

---

### Detailed Prompt: Phase 5 Implementation

```
You are an experienced Go engineer working on GroupScout (github.com/alvindcastro/groupscout).
TDD: write modelfile_manager_test.go before modelfile_manager.go.

Task: Implement internal/ollama/modelfile_manager.go and its test.

ModelfileManager struct:
  client *OllamaClient  // use concrete type here, not interface

Method: Push(ctx context.Context, modelName, modelfileContent string) error
  - POST to {Endpoint}/api/create
  - Body: {"name": modelName, "modelfile": modelfileContent, "stream": false}
  - Return nil on 200, wrapped error on failure.

Method: ListModels(ctx context.Context) ([]string, error)
  - GET {Endpoint}/api/tags
  - Parse response: {"models": [{"name": "mistral:latest"}, ...]}
  - Return slice of model name strings.

CLI integration:
  - Add subcommand "ollama" with sub-subcommands "push-models" and "list-models" to cmd/server/main.go.
  - "push-models": walk internal/ollama/modelfile/*.modelfile, call Push() for each.
    - Derive model name from filename stem: "permit_extractor.modelfile" → "groupscout-permit-extractor".
  - "list-models": call ListModels(), print each name to stdout.

Tests:
  - Push happy path: mock returns 200, assert request body JSON is correct.
  - Push failure: mock returns 500, assert error returned.
  - ListModels happy path: mock returns JSON with 2 models, assert slice has 2 items.
  - ListModels empty: mock returns {"models": []}, assert empty slice, no error.
```

---

## Phase 6 — Observability & Fallback

**Goal:** Ollama calls must be observable (latency, error rate, model used) and must degrade gracefully if the local model is slow or unavailable.

### Tasks

- [ ] Add structured log fields to every Ollama call: `model`, `use_case`, `latency_ms`, `tokens_approx`, `error`
- [ ] Add Prometheus counters/histograms: `ollama_calls_total{use_case, status}`, `ollama_latency_ms{use_case}`
- [ ] Implement timeout per use case (configurable via env):
  - `OLLAMA_EXTRACT_TIMEOUT_S` — default 30s
  - `OLLAMA_SCORE_TIMEOUT_S` — default 20s
  - `OLLAMA_ALERT_COPY_TIMEOUT_S` — default 15s
- [ ] Implement fallback chain per use case:
  - Signal extraction: fallback to existing regex extractor
  - Lead scoring: fallback to empty rationale (score number only in Slack)
  - Alert copy: fallback to hardcoded template
- [ ] Write unit tests for fallback logic: mock `LLMClient.ChatComplete()` to return `context.DeadlineExceeded`; assert fallback value returned, no error propagated to caller
- [ ] Add `/health` endpoint field: `"ollama": "ok" | "degraded" | "unavailable"`

---

## Model Reference

| Model | Pull command | RAM needed | Best for |
|---|---|---|---|
| Mistral 7B | `ollama pull mistral` | 8 GB | Signal extraction, scoring rationale |
| Llama 3.1 8B | `ollama pull llama3.1:8b` | 10 GB | Alert copy (richer prose) |
| Phi-3 Mini | `ollama pull phi3:mini` | 4 GB | Fast extraction on constrained servers |
| Gemma 2 2B | `ollama pull gemma2:2b` | 3 GB | Fallback, very low memory |

**Recommended minimum for production:** Mistral 7B. Llama 3.1 8B if RAM allows.

---

## Hardware Requirements

| RAM | Usable configuration |
|---|---|
| 8 GB | Phi-3 Mini only (extraction + scoring, no alert copy) |
| 16 GB | Mistral 7B — all three use cases |
| 32 GB+ | Mistral 7B + Llama 3.1 8B concurrently |

A standard Hetzner CX31 (8 GB, ~€9/mo) runs Phi-3 Mini.  
A Hetzner CX41 (16 GB, ~€17/mo) runs Mistral 7B for all use cases comfortably.

---

## Environment Variables

Add these to `.env` alongside existing variables:

```env
# --- OLLAMA ---
OLLAMA_ENABLED=true
OLLAMA_ENDPOINT=http://localhost:11434
OLLAMA_MODEL=mistral

# --- FEATURE TOGGLES ---
OLLAMA_EXTRACTION_ENABLED=true
OLLAMA_SCORING_ENABLED=true
OLLAMA_ALERT_COPY_ENABLED=true

# --- TIMEOUTS (seconds) ---
OLLAMA_EXTRACT_TIMEOUT_S=30
OLLAMA_SCORE_TIMEOUT_S=20
OLLAMA_ALERT_COPY_TIMEOUT_S=15
```

---

## TDD Principles for This Integration

All phases follow strict red-green-refactor:

1. **Write the `_test.go` file first** — define the interface and method signatures in the test before any implementation file exists.
2. **Mock the HTTP layer** — use `httptest.NewServer` to intercept Ollama calls. No real network in unit tests.
3. **Fixture-driven** — raw text inputs live in `internal/ollama/testdata/`. Never inline large strings in test files.
4. **Table-driven tests** — every method gets a `[]struct{ name, input, want, wantErr }` table.
5. **Test the fallback** — every Ollama call path must have a test that simulates timeout or error and asserts the correct fallback behaviour.
6. **No mocks framework** — implement `LLMClient` as a minimal interface; write stub implementations inline in test files. No `gomock` or `testify/mock` required.

---

*Last updated: April 2026*  
*Repository: github.com/alvindcastro/groupscout*
