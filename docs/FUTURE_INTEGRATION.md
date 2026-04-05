### Future Integration Roadmap

This document maps future integration ideas to the existing [ARCHITECTURE.md](./ARCHITECTURE.md) to define a development roadmap for GroupScout.

> **Deep exploration of AI-Ready SQL + RAG:** see [AI_DATA_STRATEGY.md](./AI_DATA_STRATEGY.md).
> Short answer: they work together and do NOT require replacing SQLite.

---

### 1. Agentic Engineering & GenAI Workflows
*Target: `internal/enrichment`*

- [ ] **Reasoning Loops** — Evolve from simple single-call Claude API enrichment to multi-step reasoning (ReAct/Plan-and-Solve) for complex project analysis.
- [ ] **RAG Implementation** — Retrieve top-k similar past leads before each Claude call; inject as prompt context for better GC inference, crew sizing, and cross-source dedup. See [AI_DATA_STRATEGY.md](./AI_DATA_STRATEGY.md).
  - [ ] `internal/enrichment/embeddings.go` — `Embedder` interface + `VoyageEmbedder` (HTTP, no SDK, free tier)
  - [ ] `internal/storage/embeddings.go` — `EmbeddingStore` + Go cosine similarity (no CGO needed)
  - [ ] `migrations/003_ai_context.up.sql` — `lead_embeddings` table
  - [ ] `internal/enrichment/enricher.go` — save embedding after enrichment; retrieve top-k before Claude call
  - [ ] `internal/enrichment/claude.go` — update `permitPrompt()` to accept similar leads as context
  - [ ] Phase 6 upgrade path: swap to pgvector when migrating to Postgres (repository pattern isolates the change)
- [ ] **Tool-Calling** — Enable agents to use tools (e.g., searching LinkedIn, verifying business registrations) during the enrichment phase.

### 2. Data Foundation & AI Pipelines
*Target: `internal/collector`, `internal/storage`*

- [ ] **Unstructured Ingestion** — Expand collectors to handle PDF tender documents and complex logs, using AI-driven parsing to extract structured `RawProject` data.
- [ ] **AI-Ready SQL** — Denormalized `v_lead_context` view that joins leads + raw_projects into a pre-built LLM context string; replaces hand-crafted prompt strings in `claude.go`. See [AI_DATA_STRATEGY.md](./AI_DATA_STRATEGY.md).
  - [ ] `migrations/003_ai_context.up.sql` — `v_lead_context` view (works on SQLite today)
  - [ ] `internal/storage/leads.go` — `GetContext(ctx, id) string` method
  - [ ] `internal/enrichment/claude.go` — refactor all `*Prompt()` functions to use `GetContext()` instead of hand-building strings
- [ ] **AI Observability** — Integrate frameworks like RAGAS or Vertex Eval to monitor for hallucinations and track the quality of AI-generated outreach drafts.

### 3. Integration & Cloud-Native Development
*Target: `config`, `infrastructure`, `internal/enrichment`*

- [ ] **LLM Provider Abstraction (no lock-in)** — Replace hardcoded Claude calls with a `LLMClient` interface; config-driven provider selection. See [AI_DATA_STRATEGY.md](./AI_DATA_STRATEGY.md).
  - [ ] `internal/enrichment/llm.go` — `LLMClient` interface + `CompletionRequest` struct
  - [ ] `internal/enrichment/claude.go` — refactor existing code to implement `LLMClient`
  - [ ] `internal/enrichment/openai_compat.go` — `OpenAICompatibleClient` covering OpenAI, Azure OpenAI, Groq, Mistral, Ollama (same code, different base URL)
  - [ ] `internal/enrichment/llm_factory.go` — factory: `LLM_PROVIDER` env var selects the impl
  - [ ] `config/config.go` — `LLMProvider`, `LLMModel`, `LLMBaseURL`, `AzureAPIVersion`
  - Providers covered: Anthropic, OpenAI, Azure OpenAI, Groq, Mistral, Ollama (Docker)
- [ ] **AIaaS API Layer** — Build out the existing Go REST API into a robust Inference Layer, exposing AI enrichment capabilities as a standalone service.
- [ ] **Infrastructure as Code (IaC)** — Provide Terraform templates to deploy the entire GroupScout stack on Google Cloud (Vertex AI, Cloud Run, Cloud SQL).
- [ ] **Event-Driven Architecture** — Transition from scheduled cron jobs to a Pub/Sub or Webhook-triggered model (e.g., using Google Cloud Workflows) for real-time lead processing.
- [ ] **CRM/ERP Integration** — Connect `internal/notify` to common business tools (HubSpot, Salesforce, SAP) via secure API orchestration to allow agents to directly create records or trigger actions.

### 4. Agile Execution & Quality
*Target: `docs/TESTING.md`, `internal/notify`*

- [ ] **Automated UAT** — Develop automated User Acceptance Testing (UAT) suites to validate the business value of AI-generated endpoints and drafts.
- [ ] **Technical Validation** — Implement stricter validation rules (DoD) for lead scoring, ensuring technical trade-offs (speed vs. scalability) are documented for each enrichment model.

---

### Summary of Next Steps

- [ ] **Pilot Agentic Reasoning** — Prototype a `ReAct` loop in `internal/enrichment` using Claude Sonnet.
- [ ] **Vector Search POC** — Set up a local vector store for RAG-based context enrichment.
- [ ] **Terraform Scaffolding** — Create basic IaC configurations for cloud deployment.
