# AI_DATA_STRATEGY.md — AI-Ready SQL + RAG for groupscout

> Brainstorm and implementation plan for combining AI-Ready SQL and RAG.
> Can they work together? Can they replace SQLite? Short answers: **yes** and **no** — details below.

---

## What We're Exploring

Two ideas from `FUTURE_INTEGRATION.md`:

1. **AI-Ready SQL** — structure the database so data is pre-formatted for LLM consumption
2. **RAG (Retrieval-Augmented Generation)** — store vector embeddings of past leads and retrieve similar ones to improve Claude's enrichment accuracy

The question: do they complement each other, and does adopting them require replacing SQLite?

---

## Key Constraint: No CGO

The project uses `modernc.org/sqlite` — pure Go, no CGO. This means:

- `sqlite-vec` (the most popular SQLite vector extension) **cannot be loaded** — it's a C shared library
- Switching to `mattn/go-sqlite3` would add CGO and complicate cross-compilation + Docker builds
- **Conclusion:** vector similarity search must happen in Go application code, not in SQL

This is fine at current scale. See options below.

---

## AI-Ready SQL — What It Means Here

"AI-Ready SQL" is not a specific technology — it's an approach: structure your database so that building LLM prompts is a query, not hand-crafted string logic.

### Current state (problem)

In `internal/enrichment/claude.go`, each prompt is hand-assembled:

```go
// today: manually build a string from raw fields
content := fmt.Sprintf("Title: %s\nLocation: %s\nValue: %d\n...", p.Title, p.Location, p.ProjectValue)
```

This means:
- Prompt logic is scattered across 6+ `*Prompt()` functions
- No historical context — Claude only sees the current permit
- Duplicate logic between enrichment, digest, and outreach draft prompts

### What AI-Ready SQL adds

A denormalized **context view** that joins `raw_projects + leads + outreach_log` into a single prompt-ready string per lead:

```sql
-- migrations/003_ai_context.up.sql
CREATE VIEW IF NOT EXISTS v_lead_context AS
SELECT
    l.id,
    l.source,
    l.title,
    l.location,
    l.project_value,
    l.general_contractor,
    l.project_type,
    l.estimated_crew_size,
    l.estimated_duration_months,
    l.out_of_town_crew_likely,
    l.priority_score,
    l.priority_reason,
    l.notes,
    l.status,
    l.created_at,
    rp.raw_data,
    -- pre-built context string ready for LLM injection
    'Source: ' || l.source || '. Title: ' || l.title ||
    '. Location: ' || COALESCE(l.location, 'unknown') ||
    '. Value: $' || COALESCE(CAST(l.project_value AS TEXT), 'unknown') ||
    '. GC: ' || COALESCE(l.general_contractor, 'unknown') ||
    '. Type: ' || COALESCE(l.project_type, 'unknown') ||
    '. Crew: ~' || COALESCE(CAST(l.estimated_crew_size AS TEXT), '?') ||
    '. Score: ' || CAST(l.priority_score AS TEXT) || '/10' ||
    '. Notes: ' || COALESCE(l.notes, 'none') AS context_text
FROM leads l
LEFT JOIN raw_projects rp ON l.raw_project_id = rp.id;
```

Now a helper in `internal/storage/leads.go`:

```go
// GetContext returns a pre-formatted LLM context string for a lead
func (s *SQLiteLeadStore) GetContext(ctx context.Context, id string) (string, error) {
    row := s.db.QueryRowContext(ctx,
        `SELECT context_text FROM v_lead_context WHERE id = ?`, id)
    var text string
    return text, row.Scan(&text)
}
```

And in `claude.go`, every `*Prompt()` function calls `GetContext()` instead of building strings manually — one source of truth.

---

## RAG Implementation — What It Means Here

RAG adds a retrieval step before every Claude call:

> "Before enriching this new permit, find the 3 most similar past leads and give them to Claude as context."

Result: Claude can say *"This looks like the PCL project in Burnaby last year — that one had 120 crew members, mostly from out of province"* instead of guessing from scratch every time.

### What gets embedded

The `context_text` from `v_lead_context` is the ideal embedding input — it's already a dense summary of everything known about a lead. This is where AI-Ready SQL and RAG directly connect.

### Embedding model options

Anthropic does not offer a public embeddings API. Options ranked by fit:

| Option | Model | Cost | Go integration | Notes |
|---|---|---|---|---|
| **OpenAI** | `text-embedding-3-small` | ~$0.00002/lead | HTTP POST | Cheapest, easiest |
| **Jina AI** | `jina-embeddings-v3` | Free tier | HTTP POST | 1M tokens/month free |
| **Voyage AI** | `voyage-3-lite` | Free tier | HTTP POST | Made by ex-Anthropic; best Claude pairing |
| **Google Vertex** | `text-embedding-004` | ~$0.00003/lead | HTTP POST | GCP dependency |
| **Local (Ollama)** | `nomic-embed-text` | Free | HTTP POST to localhost | Requires running Ollama sidecar |

**Recommendation:** Voyage AI `voyage-3-lite` — free tier, optimized for Claude pairing, simple HTTP API, no SDK needed.

> Full tool comparison including Docker and cloud options: see **Tool Selection** section below.

### Storage schema

```sql
-- migrations/003_ai_context.up.sql (continued)
CREATE TABLE IF NOT EXISTS lead_embeddings (
    lead_id     TEXT PRIMARY KEY REFERENCES leads(id),
    model       TEXT NOT NULL,           -- e.g., "voyage-3-lite"
    embedding   TEXT NOT NULL,           -- JSON array of float32
    created_at  TIMESTAMPTZ NOT NULL
);
```

### Similarity search in Go (SQLite) or pgvector (Postgres)

The system now supports dual-driver vector storage via the `EmbeddingStore` interface:

```go
// internal/storage/embeddings.go

type EmbeddingStore interface {
    Save(ctx context.Context, leadID string, model string, vec []float32) error
    Similar(ctx context.Context, vec []float32, k int) ([]string, error)
}
```

- **Postgres Implementation**: Uses the `pgvector` extension and the `<=>` (cosine distance) operator for high-performance similarity search.
- **SQLite Implementation**: Uses a dedicated table and Go-native `cosineSimilarity` calculation for local-first portability.

At 100 leads with 512-dim embeddings, a full scan in Go takes < 1ms. For larger datasets, Postgres with `pgvector` provides indexing support (IVFFlat/HNSW).

### New interface in `internal/enrichment/embeddings.go`

```go
type Embedder interface {
    Embed(ctx context.Context, text string) ([]float32, error)
}

type VoyageEmbedder struct {
    apiKey string
    model  string // "voyage-3-lite"
    client *http.Client
}
```

---

## How They Work Together

The combined flow for each new permit:

```
New RawProject arrives
        │
        ▼
AI-Ready SQL builds context_text for this project
        │
        ▼
Embedder generates vector for context_text
        │
        ▼
EmbeddingStore.Similar() → top-3 past lead IDs
        │
        ▼
Fetch context_text for each similar lead (v_lead_context)
        │
        ▼
Inject into Claude prompt:
  "Here are 3 similar past projects for reference:
   1. [context of lead A]
   2. [context of lead B]
   3. [context of lead C]"
        │
        ▼
Claude enriches with historical context → better scores
        │
        ▼
Save new lead + new embedding
```

This also enables:

- **Cross-source dedup** — if similarity > 0.95, likely same project; ask Claude to confirm
- **Conversational query** — `groupscout ask "..."` → embed the question → retrieve relevant leads → Claude answers
- **Digest narrative** — "This week's top lead is similar to the PCL Burnaby job that converted to a booking"

---

## Does This Replace SQLite?

**Yes — Postgres is now implemented (Phase 15).** SQLite remains available for local dev (detected from `DATABASE_URL` prefix). What changes at each phase:

| Layer | SQLite (local dev) | Postgres (production) |
|---|---|---|
| Relational data | SQLite | Postgres (`pgvector/pgvector:pg17`) |
| Prompt building | `v_lead_context` view | Same view, Postgres syntax |
| Vector storage | `lead_embeddings_sqlite` table | `lead_embeddings` table (`vector(512)`) |
| Similarity search | Go in-memory cosine | `<=>` pgvector operator + ivfflat index |
| Boolean columns | `INTEGER` | Native `BOOLEAN` |
| JSON columns | `TEXT` | `JSONB` (indexable) |

**Postgres pgvector query:**
```sql
-- similarity search
SELECT lead_id FROM lead_embeddings ORDER BY embedding <=> $1 LIMIT $2;
```

The `EmbeddingStore` interface means only the implementation swaps — `enricher.go`, `claude.go`, and the pipeline are untouched.

---

## Implementation Status

### Phase A — AI-Ready SQL only (no embeddings yet)
> Coding prompts: [PROMPTS_AI_READY_SQL.md](./PROMPTS_AI_READY_SQL.md)
- [ ] `migrations/004_lead_context_view.up.sql` — `v_lead_context` view
- [ ] `GetContext(ctx, id string) (string, error)` on `LeadStore`
- [ ] Refactor `DraftOutreach()` to accept and use `contextText string`
- [ ] Wire `GetContext()` into the outreach draft endpoint

### Phase B — Embeddings + storage
- [x] `lead_embeddings` table (Postgres/SQLite)
- [ ] `Embedder` interface + `VoyageEmbedder` impl
- [x] `EmbeddingStore` interface + Postgres (pgvector) + SQLite (in-memory cosine) impls
- [ ] `internal/enrichment/enricher.go` — generate + save embedding

### Phase 16 — LLM Provider Abstraction
> Full atomic task breakdown with TDD: see `PHASES.md` Phase 16.
> Coding prompts: [PROMPTS_PHASE16.md](./PROMPTS_PHASE16.md)

**Current state:** `ClaudeClient` + `GeminiClient` both exist and implement `EnricherAI`. Phase 16 extracts a lower-level `LLMClient` interface so the enricher is fully provider-agnostic.

- [ ] `LLMClient` interface + `CompletionRequest` struct (`internal/enrichment/llm.go`)
- [ ] `ClaudeClient` refactored to implement `LLMClient` (remove `Enrich`/`DraftOutreach`)
- [ ] `GeminiClient` refactored to implement `LLMClient` (Phase 16-F: wire into factory)
- [ ] `OpenAICompatibleClient` covers OpenAI, Groq, Mistral, Azure, Ollama
- [ ] `FallbackClient` wraps primary + secondary with Sentry logging
- [ ] `LLM_PROVIDER` env var selects the impl via factory

---

## Cost Estimate

| Item | Volume | Cost |
|---|---|---|
| Voyage AI embeddings (free tier) | Up to 1M tokens/month | $0 |
| Additional enrichment context tokens (Claude) | ~200 tokens × 5 leads/week | ~$0.001/week |
| **Total added cost** | | **~$0** |

---

## LLM Provider Abstraction

> Current: raw HTTP calls hardcoded to Anthropic's API in `internal/enrichment/claude.go`.
> Risk: any API change, pricing spike, or outage has no fallback.

### Why an abstraction makes sense

OpenAI's Chat Completions format (`POST /v1/chat/completions`) has become the de facto standard. The following providers all speak it natively:

- OpenAI (GPT-4o, GPT-4o-mini)
- Azure OpenAI (same models, different base URL + auth header)
- Groq (Llama, Mixtral — ultra-fast inference)
- Mistral AI (Mistral 7B, Large)
- Ollama (any local model — Llama 3.2, Phi-3, etc.)
- LM Studio (local)
- Together AI, Perplexity, Anyscale

Claude is the outlier — it uses Anthropic's Messages API format. So **two concrete implementations** cover everything:

```
LLMClient interface
├── ClaudeClient          — Anthropic Messages API (current)
└── OpenAICompatibleClient — covers OpenAI, Azure, Groq, Mistral, Ollama, ...
                             (configurable base URL + auth header)
```

### The interface

```go
// internal/enrichment/llm.go
type LLMClient interface {
    Complete(ctx context.Context, req CompletionRequest) (string, error)
}

type CompletionRequest struct {
    System    string
    User      string
    MaxTokens int
}
```

`claude.go` becomes `ClaudeClient`. A new `openai_compat.go` handles everything else. `enricher.go` and all prompt functions are untouched — they call `LLMClient.Complete()`, not a provider-specific method.

### Config

```env
LLM_PROVIDER=claude         # claude | openai | azure | groq | mistral | ollama
LLM_MODEL=claude-haiku-4-5  # model name within the provider
LLM_API_KEY=sk-ant-...
LLM_BASE_URL=               # required for azure, ollama, groq; empty = provider default
AZURE_API_VERSION=2024-02-01
```

### Provider Comparison

| Provider | Type | Best models for groupscout | API format | Input cost/1M tokens |
|---|---|---|---|---|
| **Anthropic Claude** | Cloud | Haiku 4.5 (bulk), Sonnet 4.6 (quality) | Anthropic Messages | $0.80 / $3.00 |
| **Google Gemini** ✅ implemented | Cloud | Gemini 1.5 Flash (bulk), 1.5 Pro (quality) | Gemini API (`internal/enrichment/gemini.go`) | $0.075 / $1.25 |
| **OpenAI** | Cloud | GPT-4o-mini (bulk), GPT-4o (quality) | OpenAI Chat Completions | $0.15 / $2.50 |
| **Azure OpenAI** | Cloud | Same GPT models | OpenAI-compatible | Same + Azure pricing |
| **Groq** | Cloud | Llama 3.1 8B (bulk), 70B (quality) | OpenAI-compatible | ~$0.05 / $0.59 |
| **Mistral AI** | Cloud | Mistral Nemo (bulk), Large (quality) | OpenAI-compatible | $0.10 / $1.00 |
| **Ollama** | Docker | Llama 3.2 3B, Phi-3, Mistral 7B | OpenAI-compatible | Free |

**Notes:**
- Azure OpenAI: same quality as OpenAI, but with enterprise compliance, Canadian data residency options, and no rate-limit surprises — good fit if Sandman/Douglas College has an Azure agreement
- Groq: fastest inference by far (~10x); good for high-volume batch enrichment runs
- Ollama: free and private; with a tuned Modelfile (`config/Modelfile.groupscout`) reaches ~95% JSON reliability — viable for production
- Gemini: already implemented in `internal/enrichment/gemini.go`; cheapest cloud option at $0.075/1M tokens

### JSON Output Reliability by Provider

Each provider uses a different mechanism to enforce JSON-only output:

| Provider | JSON enforcement | Reliability |
|---|---|---|
| **Claude** | System prompt instruction only | ~98% — very instruction-following |
| **Gemini** | `responseMimeType: "application/json"` in request | ~97% |
| **OpenAI** | `response_format: {"type":"json_object"}` | ~97% |
| **Groq** | `response_format` (supported on llama/mixtral) | ~94% |
| **Mistral** | Prompt-only — `response_format` not supported | ~88% — use `stripMarkdown()` fallback |
| **Ollama (raw)** | `response_format` (supported on llama3.2, mistral) | ~85% |
| **Ollama (groupscout Modelfile)** | Baked system prompt + `PARAMETER stop "` ``` `"` | ~95% — recommended |

`stripMarkdown()` in `openai_compat.go` handles the fallback case for all providers.

### Recommended strategy

- **Default:** Claude Haiku (current) — best quality-to-cost for permit enrichment
- **Fallback:** OpenAI GPT-4o-mini — same interface, swap via env var if Anthropic has issues
- **Local/free:** Ollama with `groupscout` Modelfile — full pipeline at zero API cost; ~95% JSON reliability
- **Enterprise path:** Azure OpenAI — if hotel or college requires data residency or enterprise SLAs

### Ollama Model Recommendations

| Model | Size | Use case | JSON reliability |
|---|---|---|---|
| `groupscout` (custom) | ~2GB | **Recommended default** — llama3.2 + hotel sales persona | ~95% |
| `llama3.2` | ~2GB | Fast permit extraction, dev/testing | ~85% |
| `mistral` | ~4GB | Higher quality scoring rationale | ~90% |
| `llama3.1:8b` | ~5GB | Best outreach email drafts | ~88% |
| `phi3:mini` | ~2GB | Pre-scoring only (cheapest, fastest) | ~75% |

Create the custom model: `./scripts/ollama_create_model.sh`
Pull a base model: `./scripts/ollama_pull.sh llama3.2`

### Implementation tasks

> Full atomic task breakdown: see `PHASES.md` Phase 15. Summary here for reference.

**Part A — Interface extraction:**
- [ ] `internal/enrichment/llm.go` — `LLMClient` interface + `CompletionRequest` struct
- [ ] `internal/enrichment/claude.go` — `ClaudeClient` implementing `LLMClient`
- [ ] `internal/enrichment/llm_factory.go` — factory returning `ClaudeClient` only initially
- [ ] `config/config.go` — `LLMProvider`, `LLMModel`, `LLMAPIKey`, `LLMBaseURL` env vars
- [ ] `internal/enrichment/enricher.go` — swap `*ClaudeEnricher` for `LLMClient`

**Part B — OpenAI-compatible client (OpenAI, Groq, Mistral):**
- [ ] `internal/enrichment/openai_compat.go` — `OpenAICompatibleClient`
- [ ] `internal/enrichment/llm_factory.go` — wire `openai|groq|mistral` providers

**Part C — Azure OpenAI:**
- [ ] `internal/enrichment/openai_compat.go` — Azure URL builder + `api-key` header
- [ ] `config/config.go` — `AzureResourceName`, `AzureDeploymentName`, `AzureAPIVersion`

**Part D — Ollama (Docker):**
- [ ] `internal/enrichment/llm_factory.go` — wire `ollama`; omit auth header if no key
- [ ] `docker-compose.yml` — `ollama` service + model volume

**Part E — Fallback & resilience:**
- [ ] `internal/enrichment/llm.go` — `FallbackClient` wrapping primary + secondary
- [ ] `config/config.go` — `LLMFallbackProvider`, `LLMFallbackModel`, `LLMFallbackAPIKey`

---

## Tool Selection

> All options evaluated for fit with: pure Go, existing Docker Compose stack, minimal ops overhead, current scale (~100 leads).

---

### 1. Embedding Providers

#### Cloud (no Docker needed)

| Tool | Model | Free Tier | Dimensions | Best For |
|---|---|---|---|---|
| **Voyage AI** ⭐ | `voyage-3-lite` | 200M tokens/month | 512 | Best Claude pairing; ex-Anthropic team |
| **Jina AI** | `jina-embeddings-v3` | 1M tokens/month | 1024 | Good general purpose; multilingual |
| **OpenAI** | `text-embedding-3-small` | None (~$0.02/1M tokens) | 1536 | Widest ecosystem support |
| **Cohere** | `embed-english-light-v3.0` | 1000 calls/month | 384 | Smallest vectors; lowest storage cost |
| **Mistral** | `mistral-embed` | None | 1024 | If already using Mistral for LLM |

#### Self-Hosted / Docker

| Tool | Docker Image | Dimensions | Notes |
|---|---|---|---|
| **Ollama** ⭐ | `ollama/ollama` | 768 (`nomic-embed-text`) | Already useful for local LLM dev; one container runs both embedding + chat models |
| **Infinity** | `michaelf34/infinity` | any HuggingFace model | Lightweight HTTP embedding server; drop-in for any sentence-transformer model |
| **Text Embeddings Inference (TEI)** | `ghcr.io/huggingface/text-embeddings-inference` | any HuggingFace model | HuggingFace's official server; GPU-optional; production-grade |

**Pick one:**
- [ ] **Cloud (recommended now):** Voyage AI `voyage-3-lite` — free, no Docker, HTTP POST, ideal Claude pairing
- [ ] **Docker (recommended later):** Ollama with `nomic-embed-text` — free forever, runs alongside groupscout in Docker Compose, no external API dependency

---

### 2. Vector Stores

#### Cloud

| Tool | Free Tier | Go client | Notes |
|---|---|---|---|
| **Pinecone** | 1 index free | HTTP REST | Most popular; serverless tier; no Docker option |
| **Qdrant Cloud** | 1 cluster free (1GB) | `go-qdrant` SDK or REST | Same API as self-hosted; easy to migrate |
| **Weaviate Cloud** | 14-day trial | `weaviate-go-client` | GraphQL-based; more complex |
| **Zilliz (Milvus Cloud)** | Free tier | `milvus-sdk-go` | Enterprise Milvus; overkill for this scale |

#### Self-Hosted / Docker

| Tool | Docker Image | Go client | Notes |
|---|---|---|---|
| **Qdrant** ⭐ | `qdrant/qdrant` | `go-qdrant` or HTTP REST | Best fit: lightweight, pure Rust, simple REST API, persistent storage, fast |
| **Chroma** | `chromadb/chroma` | HTTP REST | Python-native but has HTTP API; simpler ops than Weaviate |
| **Weaviate** | `semitechnologies/weaviate` | `weaviate-go-client` | Full-featured but heavier; GraphQL adds complexity |
| **Milvus** | `milvusdb/milvus` | `milvus-sdk-go` | Enterprise-grade; requires etcd + MinIO sidecars; overkill here |
| **pgvector (Postgres)** ⭐ | `pgvector/pgvector` | standard `pgx` | Best if already migrating to Postgres (Phase 6); zero extra infra |

**Pick one:**
- [ ] **Phase B (now):** Go in-memory cosine similarity + SQLite `lead_embeddings` table — zero infra, works for <1000 leads
- [ ] **Phase C (Docker upgrade):** Qdrant Docker — `docker compose` addition, REST API, Go-friendly, scales to millions
- [ ] **Phase C (Postgres path):** pgvector — add `vector` extension during Phase 15 Postgres migration; no new container needed

---

### 3. AI Observability (LLM Monitoring)

Track Claude API calls, token costs, response quality, and hallucinations over time.

#### Cloud

| Tool | Free Tier | Notes |
|---|---|---|
| **Langfuse Cloud** | Hobby plan free | Open-source core; tracks prompts, responses, costs, latency |
| **Helicone** | Free up to 10k req/month | Proxy-based; drop-in for any OpenAI-compatible API |
| **LangSmith** | 10k traces/month free | LangChain's observability platform; works without LangChain |
| **Braintrust** | Free tier | Evals + tracing; good for comparing prompt versions |

#### Self-Hosted / Docker

| Tool | Docker Image | Notes |
|---|---|---|
| **Langfuse** ⭐ | `langfuse/langfuse` | Open-source; self-host the full platform; Postgres backend; Go SDK or HTTP API |
| **Phoenix (Arize)** | `arizephoenix/phoenix` | Local-first tracing; no API key needed; good for dev |
| **OpenLIT** | `openlit/openlit` | OpenTelemetry-native; integrates with existing Grafana/Prometheus stack |

**Pick one:**
- [ ] **Cloud (start here):** Langfuse Cloud hobby plan — free, no Docker, tracks all Claude calls with a simple HTTP POST per call
- [ ] **Docker (when self-hosting matters):** Langfuse self-hosted — same product, add to Docker Compose alongside n8n; Postgres backend fits Phase 6

**Integration in Go** (no SDK required):
```go
// After each Claude call, POST to Langfuse /api/public/ingestion
// Payload: { "traces": [{ "name": "enrich", "input": prompt, "output": response, "usage": { "total_tokens": n } }] }
```

---

### 4. Full Docker Compose Picture

What the `docker-compose.yml` looks like when all tools are added:

```yaml
services:
  groupscout:       # Go app (existing)
  n8n:              # workflow automation (existing)
  prometheus:       # metrics (existing)
  grafana:          # dashboards (existing)

  # Phase B additions
  ollama:           # embedding model server (nomic-embed-text)
  qdrant:           # vector store (replaces in-memory cosine at scale)

  # Phase 6 additions
  postgres:         # replaces SQLite; enables pgvector
  langfuse:         # LLM observability
  langfuse-worker:  # Langfuse background processor
```

---

### 5. Recommended Stack (Phased)

| Phase | Embedding | Vector Store | Observability |
|---|---|---|---|
| **Now (Phase B)** | Voyage AI cloud (free) | SQLite + Go cosine | Langfuse cloud (free) |
| **Docker upgrade** | Ollama (`nomic-embed-text`) | Qdrant Docker | Langfuse Docker |
| **Phase 6 (Postgres)** | Ollama or Voyage AI | pgvector | Langfuse Docker |

All three tiers use the same Go interfaces (`Embedder`, `EmbeddingStore`) — swap the implementation, not the pipeline.

---

## Open Questions

- [ ] Should embeddings be generated for ALL leads or only those with score ≥ 5?
  - Recommendation: all leads — cost is negligible, and low-score leads still provide useful "this is NOT what we want" contrast context
- [ ] Should the `Embedder` be pluggable at runtime (env var selects provider)?
  - Yes — `EMBEDDING_PROVIDER=voyage|openai|jina|local` in config
- [ ] Should similar leads be shown in the Slack digest?
  - Yes — "Similar past lead: [title] (converted to booking)" adds credibility to the priority score

---

*groupscout AI data strategy — local planning doc*
*See `FUTURE_INTEGRATION.md` for high-level roadmap items*
*See `AI.md` for the full LLM integration backlog*
