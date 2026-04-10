### GroupScout Technical Nice-to-Haves & Roadmap

This document outlines potential technical enhancements and a roadmap for the future development of GroupScout, moving beyond the core Phase 6 productionization goals.

---

### Categorized Brainstorming

#### 1. System Reliability & Observability
- [x] **Structured Logging** — Standardize all logs into JSON format using `slog` for log aggregators like ELK or Grafana Loki.
- [x] **Sentry Integration** — Automated error reporting to catch and alert on scraper failures or API timeouts in real-time.
- [x] **Advanced Health Checks** — `/health` endpoint checking DB connectivity and last successful run times per collector.
- [ ] **Distributed Tracing (OpenTelemetry)** — Implement tracing to visualize lead flow from collection to notification, making it easier to identify bottlenecks or failures in the enrichment pipeline.

#### 2. Intelligence & Enrichment
- [x] **Entity Resolution (Deduplication)** — Basic hash-based dedup. Upgrade: AI or fuzzy matching to identify when multiple sources report on the same project.
- [ ] **Lead History & Tracking** — Track how a lead evolves over time (e.g., from an initial announcement to a building permit) to provide a "timeline" view of project development.
- [ ] **Custom AI Personas** — Allow users to define different "outreach personas" (e.g., "Technical Specialist" vs. "Sales Executive") for the Claude-generated email drafts.
- [ ] **Sentiment Analysis on News** — Use NLP to determine if industry news is positive (growth/expansion) or negative (delays/cancellations) to adjust lead scoring.
- [ ] **Outreach Logging** — Log interaction history (LinkedIn, Email, Phone) per lead to track conversion and avoid over-contacting.

#### 3. API & Integration

**Lead Management API (REST):**
- [ ] `GET /leads` — List leads with filtering (status, source, min_score) and pagination.
- [ ] `GET /leads/{id}` — Get detailed lead info including outreach history.
- [ ] `PATCH /leads/{id}` — Update lead status (e.g., 'contacted', 'booked') or add notes.
- [ ] `POST /leads/{id}/outreach` — Log a new outreach attempt.

**Collector Management & Monitoring:**
- [ ] `GET /collectors` — List active collectors and their last run status.
- [ ] `POST /collectors/{name}/run` — Manually trigger a specific collector.
- [ ] `GET /stats` — Aggregated metrics (total leads, enrichment rate, score distribution).

**System & Integration (existing endpoints):**
- [x] `POST /digest` — Trigger manual digest delivery.
- [x] `POST /run` — Trigger full pipeline.
- [x] `POST /n8n/webhook` — Direct lead ingestion for external automation (n8n/Make).
- [x] `GET /health` — System health check (DB connectivity, API status).

#### 4. Technology Selection: REST (JSON)

> **Decision: REST over gRPC/GraphQL.**
> - Primary clients are n8n, Slack, and simple webhooks — REST is their native language.
> - Easier to debug via `curl` or Bruno during this early phase.
> - gRPC adds Protobuf/HTTP2 overhead not yet justified; GraphQL is overkill for a Slack-first tool.
> - Stack: `net/http` or lightweight router like `chi`. Revisit GraphQL if a complex dashboard is built in Phase 10.

#### 5. Performance & Scaling
- [ ] **Parallel Scraper Execution** — Transition from sequential collector runs to a concurrent model using goroutine worker pools to significantly reduce total pipeline runtime.
- [ ] **Collector Registry & Unified Configuration** — Centralized registry for all collectors to simplify adding/removing sources and standardizing retry logic, timeouts, and rate limiting.
- [ ] **Generic Scraper Interface Improvements** — Enhance the `Collector` interface to support dynamic parameters (date ranges, keyword overrides) passed from the `/run` endpoint.
- [ ] **Caching Layer (Redis)** — Implement caching for frequent API calls (e.g., Claude enrichment for similar projects) or external scraper targets to reduce latency and costs.
- [ ] **Incremental Scraping** — Optimize scrapers to only fetch and process data newer than the last successful run, rather than re-scanning entire pages or feeds.

#### 6. Integration & UI
- [ ] **Interactive Dashboard** — A lightweight React or Vue frontend to visualize leads, manually trigger runs, and edit AI-generated outreach drafts before sending.
- [ ] **CRM Direct Sync** — Add "one-click" buttons to push high-value leads directly into HubSpot, Salesforce, or Pipedrive via their APIs.
- [ ] **Chrome Extension** — A browser tool to manually "clip" a lead from a website and send it directly into the GroupScout enrichment pipeline.

#### 7. Potential New Lead Sources

**Municipal Building Permits (Expansion):**
- [ ] **Burnaby** — Daily/Weekly PDF reports
- [ ] **Coquitlam** — Monthly PDF reports
- [ ] **Surrey** — Monthly Building Permit Summary PDFs
- [ ] **Vancouver** — Open Data Portal (CSV/API) for issued building permits
- [ ] **New Westminster** — Weekly building permit reports
- [ ] **North Vancouver** — Monthly building permit reports

**Specialized Industry News:**
- [ ] **Daily Hive Vancouver / Vancouver Sun** — Scrape for "hotel pipeline", "new hotel approval", or "office-to-hotel conversion" keywords.
- [ ] **Journal of Commerce (Canada)** — Track major project starts and contract awards in BC. Priority: Richmond and Delta.

**Major Event Venues:**
- [ ] **PNE / Playland (Vancouver)** — Large trade shows or seasonal events that bring contractors/vendors.
- [ ] **Abbotsford Centre / Langley Events Centre** — Regional events that might spill over to Richmond for specific groups.

**Public Tenders & Utilities:**
- [ ] **BC Hydro / FortisBC** — Major utility infrastructure projects; often involve 2–3 year crew deployments.

#### 8. Docker Ecosystem (Self-hosted Infrastructure)

> Full tool comparison (embedding providers, vector stores, observability) with cloud alternatives: see [AI_DATA_STRATEGY.md](./AI_DATA_STRATEGY.md).

**Automation & Orchestration:**
- [x] **n8n** — Workflow automation to trigger `/run` and `/digest`. Also used for "Active Collection" (n8n scrapes → pushes to `/n8n/webhook`).

**Monitoring & Observability:**
- [x] **Prometheus** — Infrastructure metrics.
- [x] **Grafana Loki** — Log aggregation.
- [ ] **Metabase or Grafana** — Connect to `groupscout.db` (via SQLite connector) to visualize lead trends, source performance, and enrichment accuracy.
- [ ] **Langfuse** — LLM observability: track Claude API calls, token costs, prompt versions, response quality. Self-hosted or cloud (free hobby tier).

**AI / Embeddings:**
- [ ] **Ollama** (`ollama/ollama`) — Local embedding + LLM server. Run `nomic-embed-text` for embeddings; no external API key needed. Add to Docker Compose alongside groupscout.
- [ ] **Infinity** (`michaelf34/infinity`) — Lightweight HTTP embedding server for any HuggingFace sentence-transformer model. Alternative to Ollama if only embeddings are needed.

**Vector Store:**
- [ ] **Qdrant** (`qdrant/qdrant`) — Lightweight vector store with REST API and Go client. Replaces in-memory cosine similarity at scale. Easiest self-hosted option.
- [ ] **pgvector** (`pgvector/pgvector`) — Vector extension for Postgres. Best choice if using Postgres in Phase 6; no extra container needed.

**Search:**
- [ ] **Meilisearch** — Fast full-text lead search for the Admin UI.

**Caching:**
- [ ] **Redis** — Cache enrichment results and scraper states to reduce API costs and latency.

**Notifications (alternative channels):**
- [ ] **Gotify or Apprise** — Alternative notification services if Slack is not the primary communication hub.

**Error Tracking:**
- [ ] **Sentry (Self-hosted)** — Full Sentry stack in Docker if data privacy is a concern.

---

### Phased Roadmap

#### Phase 7: Observability & Refinement (Next Up)
- [x] Implement structured logging (`slog`).
- [x] Add Sentry for error tracking.
- [x] Create `TESTING.md` and define testing strategy.
- [x] Implement basic lead deduplication (exact match on URL/Title).
- [x] Add `/health` and `/metrics` (Prometheus) endpoints.
- [x] Testing Documentation — Create `TESTING.md`.
- [x] Integrated n8n for orchestration.

#### Phase 8: Scaling & Intelligence (In Progress)
- [ ] Transition to concurrent collector execution (worker pools). (Phase 9.2)
- [ ] Implement Redis for caching enrichment results and scraper states. (Phase 9.4)
- [ ] Enhance AI Scorer with sentiment analysis.
- [ ] Add project "timelines" by linking related leads.

#### Phase 9: External Ecosystem
- [ ] Develop a minimal internal Admin UI (React/Vite).
- [ ] Build a HubSpot/Salesforce integration module.
- [ ] Implement "One-Click Outreach" directly from the Slack notification.

#### Phase 10: Advanced Automation
- [ ] Multi-persona outreach drafting (A/B testing for drafts).
- [ ] "Smart Scheduling" — Scrapers run more frequently during high-activity periods.
- [ ] Fully automated "Follow-up" reminders based on lead status.
