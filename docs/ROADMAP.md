# ROADMAP.md — groupscout

> **Master roadmap** consolidating `PHASES.md`, `AI.md`, `FUTURE_INTEGRATION.md`, `TECH_IDEAS.md`, and `PLANNING.md` into one strategic view.
>
> - `PHASES.md` remains the **atomic task tracker** (checkboxes + commit workflow).
> - This file is the **big-picture list** — what's done, what's next, what's future.
> - Source docs are preserved as-is; this is a read-only synthesis for navigation.

---

## Phase Summary

- [x] **Phase 1** — Foundation: DB boots, schema applied
- [x] **Phase 2** — Richmond → Claude → Slack (first full pipeline)
- [x] **Phase 3** — Dedup hardened, BC Bid/Delta added, n8n trigger
- [ ] **Phase 4** — Creative BC, VCC, Eventbrite, news, announcements, instant alert, email digest *(in progress)*
- [ ] **Phase 5** — Smart refresh: avoid redundant PDF fetches *(deferred)*
- [ ] **Phase 6** — Productionize: Docker, Postgres, VPS deploy
- [ ] **Phase 7** — User requests & API refinements *(in progress)*
- [ ] **Phase 8** — System reliability & observability *(in progress)*
- [ ] **Phase 9** — Architecture & scaling: concurrency, caching
- [ ] **Phase 10** — Ecosystem & UI: dashboard, CRM, extensions
- [ ] **Phase 11** — CRM direct integration: HubSpot, Salesforce
- [ ] **Phase 12** — Source expansion: Metro Vancouver municipalities
- [ ] **Phase 13** — Public tenders & utilities: BC Hydro, FortisBC
- [ ] **Phase 14** — Infrastructure & self-hosting: Docker ecosystem *(in progress)*
- [ ] **Phase 15** — AI/LLM enhancements: hybrid scoring, multi-agent, RAG
- [ ] **Phase 16** — Future integrations: cloud-native, event-driven, IaC

---

## Phase 4 — More Sources + Full Notifications 🔄

### Collectors
- [x] Google News RSS monitor (`internal/collector/news.go`)
- [x] Creative BC "In Production" scraper (`internal/collector/creativebc.go`)
- [x] Vancouver Convention Centre event scraper (`internal/collector/vcc.go`)
- [x] Eventbrite conference scraper (`internal/collector/eventbrite.go`)
- [x] Infrastructure announcements — BCIB, TransLink, YVR (`internal/collector/announcements.go`)

### Notifications
- [x] Instant alert for priority score ≥ 9 (`internal/enrichment/enricher.go`)
- [x] Weekly email digest via SendGrid (`internal/notify/email.go`)
- [x] Claude outreach draft generation (`internal/enrichment/claude.go`)
- [x] `/digest` HTTP endpoint for n8n triggers

---

## Phase 5 — Smart Refresh ⏸ (Deferred)

> Richmond publishes weekly; a weekly cron is sufficient for now.
> Add this when reducing unnecessary PDF downloads matters.

- [ ] `migrations/002_scrape_state.up.sql` — add `scrape_state` table
- [ ] `internal/storage/scrapestate.go` — `ScrapeStateStore` interface + SQLite impl
- [ ] `internal/collector/richmond.go` — hash reports page HTML before downloading PDFs
- [ ] `internal/collector/richmond.go` — diff current vs. previously seen PDF links; download new only
- [ ] Skip entire run if page hash unchanged

---

## Phase 6 — Productionize 📋

- [ ] Dockerfile + docker-compose for app + SQLite
- [ ] Postgres migration path (`DATABASE_URL=postgres://...`)
- [ ] `golang-migrate/migrate` wired to migration files
- [ ] VPS or Railway deployment (single container)
- [ ] Env var hardening + `.env.example` documentation
- [ ] Smoke test all collectors end-to-end on production

---

## Phase 7 — User Requests & API Refinements 🔄

- [x] Slack notifications show lead source
- [ ] `internal/storage/leads.go` — `List` with filtering (status, score) and pagination
- [ ] `cmd/server/main.go` — `GET /leads` endpoint (filter by status, source, min_score)
- [ ] `cmd/server/main.go` — `PATCH /leads/{id}` endpoint (update status, add notes)
- [ ] `cmd/server/main.go` — `GET /leads/{id}` endpoint (include outreach history)
- [ ] `cmd/server/main.go` — `POST /leads/{id}/outreach` endpoint (log outreach attempt)
- [ ] `internal/storage/outreach.go` — `OutreachLogStore` for tracking interactions

---

## Phase 8 — System Reliability & Observability 🔄

- [x] Structured logging via `log/slog`
- [x] Sentry integration — runtime errors + pipeline failures
- [x] `/health` endpoint — DB ping + critical env var check
- [x] Pipeline run metrics (counts logged)
- [x] Basic lead deduplication (hash-based)
- [x] `TESTING.md` created
- [ ] Distributed tracing via OpenTelemetry — visualize lead flow end-to-end
- [ ] Prometheus `/metrics` endpoint — counters for leads collected, enriched, notified
- [ ] Grafana Loki dashboard — structured log aggregation
- [ ] Advanced health check — last successful run time per collector

---

## Phase 9 — Architecture & Scaling 📋

- [ ] Collector registry — centralized registry to simplify adding/removing sources
- [ ] Parallel collector execution — goroutine worker pools (reduces total pipeline runtime)
- [ ] Dynamic run parameters — pass keyword/date overrides via `/run` endpoint
- [ ] Redis caching layer — cache Claude enrichment results + scraper states
- [ ] Incremental scraping — only process data newer than last successful run
- [ ] `GET /collectors` — list active collectors and last run status
- [ ] `POST /collectors/{name}/run` — manually trigger a specific collector
- [ ] `GET /stats` — aggregated metrics (total leads, enrichment rate, score distribution)

---

## Phase 10 — Ecosystem & UI 📋

- [ ] Admin dashboard — lightweight React/Vite frontend for lead visualization
- [ ] CRM sync — one-click push to HubSpot/Salesforce/Pipedrive via their APIs
- [ ] Multi-persona outreach drafting — A/B templates (e.g., "Technical Specialist" vs. "Sales Executive")
- [ ] Chrome Extension — manually clip a lead from any webpage into the pipeline
- [ ] Smart scheduling — scrapers run more frequently during high-activity periods
- [ ] Fully automated follow-up reminders based on lead status

---

## Phase 11 — CRM Integration 📋

- [ ] HubSpot sync — auto-push leads with score > 8 to HubSpot via API
- [ ] LinkedIn helper — API endpoint to generate pre-filled LinkedIn search URLs for GCs/applicants
- [ ] Salesforce connector — create/update contact + opportunity records
- [ ] "One-Click Outreach" — button in Slack notification → triggers email draft + CRM record creation

---

## Phase 12 — Source Expansion (Metro Vancouver) 📋

> Collector interface is additive — no core pipeline changes needed.

### Municipal Building Permits
- [ ] **Burnaby** — daily/weekly PDF reports
- [ ] **Vancouver** — Open Data Portal (CSV/API)
- [ ] **Coquitlam** — monthly PDF reports
- [ ] **Surrey** — monthly building permit summary PDFs
- [ ] **New Westminster** — weekly building permit reports
- [ ] **North Vancouver (DNV)** — monthly building permit reports

### Specialized Industry Sources
- [ ] **Journal of Commerce (Canada)** — major BC project starts + contract awards; priority on Richmond/Delta
- [ ] **Daily Hive Vancouver / Vancouver Sun** — "hotel pipeline", "office-to-hotel conversion" keywords
- [ ] **PNE / Playland** — large trade shows or seasonal events
- [ ] **Abbotsford Centre / Langley Events Centre** — regional events

### Future Lead Segments
- [ ] Sports teams — Rogers Arena schedule, BC Lions, Vancouver FC fixtures
- [ ] Film / TV crews — BC film permit registry
- [ ] Government contractors — DND, CBSA, Transport Canada near YVR
- [ ] Touring acts — venue booking announcements

---

## Phase 13 — Public Tenders & Utilities 📋

- [ ] BC Hydro — major utility infrastructure project announcements + tenders
- [ ] FortisBC — major utility projects (often 2-3 year crew deployments)

---

## Phase 14 — Infrastructure & Self-Hosting 🔄

- [x] Docker Compose — GroupScout + n8n + Redis
- [x] n8n workflow — trigger `/run` and `/digest` on schedule
- [x] `/n8n/webhook` endpoint — receive external leads from n8n
- [x] Prometheus + Grafana Loki — infrastructure monitoring + log aggregation
- [ ] Metabase or Grafana — connect to `groupscout.db` for lead analytics dashboard
- [ ] Meilisearch — fast lead search for Admin UI
- [ ] Gotify or Apprise — alternative/secondary notification channels
- [ ] Sentry self-hosted (Docker) — if data privacy becomes a concern

---

## Phase 15 — AI/LLM Enhancements 🔭

> All items from `AI.md`. Organized by effort and dependency order.

### Near-Term AI Upgrades

- [ ] **Hybrid pre-scorer** — Go rules first (free), then Claude yes/no for borderline 4–6 scores
  - Model: Haiku | Cost: ~10 tokens/call | Catches edge cases rules miss
- [ ] **GC contact enrichment** — for leads with score ≥ 8, Claude + web search tool finds office phone, PM name, LinkedIn page
  - Model: Sonnet + tools | Cost: ~$0.01–0.05/search | Only for high-priority leads
- [ ] **Cross-source deduplication** — Claude semantic check: "Is this the same project as any of these 5 recent leads?"
  - Handles cases where Richmond and Delta both permit the same large project
  - Upgrade path: embedding similarity when lead volume justifies it

### Medium-Term AI Upgrades

- [ ] **News article summarization** — Claude converts raw RSS snippets to leads
  - Flow: headline + 500 chars → yes/no project signal → extract structured fields
- [ ] **Announcement summarizer** — Claude reads BCIB/TransLink/YVR prose press releases
  - Flow: scrape → extract text → Claude 2-sentence summary + value estimate
- [ ] **Multimodal PDF parsing** — pass PDFs directly to Claude (vision) instead of `pdftotext`
  - Benefit: handles any PDF format, no custom parser per city
  - Defer until adding 5+ new cities (currently ~10–50x more expensive)
- [ ] **Lead history & timeline** — link related leads across sources (announcement → permit → award)
  - Tracks project evolution over time; AI or fuzzy match for entity resolution
- [ ] **Sentiment analysis on news** — NLP to flag delays/cancellations and adjust scoring
- [ ] **AI observability** — RAGAS or Vertex Eval to monitor hallucinations + track enrichment quality

### Longer-Term AI Upgrades

- [ ] **Conversational lead query CLI** — `groupscout ask "show all industrial leads last 30 days over $1M"`
  - Claude + function calling; translates NL to DB query + plain-English summary
  - Model: Haiku | Low stakes
- [ ] **Extended thinking for ambiguous scoring** — Sonnet with extended thinking for permits where score is 5–7 AND value > $2M
  - Thinking budget: 2000 tokens | Cost: ~$0.02/call | Only for genuinely unclear cases
- [ ] **Digest personalization** — Claude writes a narrative digest based on current leads + past outreach log
  - Input: leads + outreach history | Output: "3 leads this week. GC from Alberta → prioritize."
- [ ] **Multi-agent pipeline** — one agent per source (parallel), coordinator agent merges + deduplicates + ranks
  - Claude Agent SDK for orchestration | Justified when sources exceed 8–10

### Model Selection Reference

| Use case | Model | Approx. cost/call |
|---|---|---|
| Permit enrichment (bulk) | Haiku | ~$0.001 |
| Ambiguous scoring | Sonnet | ~$0.01 |
| Email drafting | Sonnet | ~$0.01 |
| Complex scoring (extended thinking) | Sonnet + thinking | ~$0.02 |
| Web search enrichment | Sonnet + tools | ~$0.01–0.05 |
| Conversational query | Haiku | ~$0.001 |
| Multimodal PDF parsing | Sonnet (vision) | ~$0.01–0.05 |

### Weekly Cost Estimate (20 permits/week, 5 pass filter)

| Integration | Cost/week |
|---|---|
| Current enrichment (Haiku × 5) | ~$0.005 |
| + Email drafts (Sonnet × 5) | ~$0.05 |
| + Web search for score ≥ 8 leads (×2) | ~$0.10 |
| + News summarization (×20 articles) | ~$0.02 |
| **Total** | **~$0.18/week → ~$9/year** |

---

## Phase 16 — Future Integrations & Cloud-Native 🔭

> Items from `FUTURE_INTEGRATION.md`. Long-horizon / architectural ambition.

### Agentic Engineering
- [ ] **Reasoning loops (ReAct / Plan-and-Solve)** — multi-step enrichment for complex project analysis
  - Target: `internal/enrichment` | Pilot: prototype with Claude Sonnet
- [ ] **RAG implementation** — vector DB (ChromaDB, Pinecone, or Vertex AI) for context-aware lead matching against historical successful leads
  - Enables: "This project is similar to the PCL contract we won in 2024"
- [ ] **Tool-calling agents** — agents that search LinkedIn, verify business registrations during enrichment
  - Target: `internal/enrichment`

### Data & AI Pipelines
- [ ] **Unstructured ingestion** — AI-driven parsing of complex PDF tender documents
  - Target: `internal/collector`
- [ ] **AI-ready SQL** — pre-aggregate and clean data optimized for LLM consumption
  - Target: `internal/storage`

### Integration & Cloud
- [ ] **AIaaS API layer** — expose Claude enrichment as a standalone inference service
  - Target: `config`, `infrastructure`
- [ ] **Event-driven architecture** — transition from cron to Pub/Sub / webhook-triggered model (e.g., Google Cloud Workflows)
  - Enables real-time lead processing instead of scheduled batches
- [ ] **CRM/ERP direct integration** — HubSpot, Salesforce, SAP via secure API orchestration
  - Agents create CRM records or trigger workflows automatically
- [ ] **Infrastructure as Code (IaC)** — Terraform templates for Google Cloud (Vertex AI, Cloud Run, Cloud SQL)
  - Target: `infrastructure/`

### Quality & Validation
- [ ] **Automated UAT** — validate business value of AI-generated endpoints and drafts
- [ ] **Technical validation** — stricter DoD for lead scoring; document speed vs. scalability trade-offs per model

---

*groupscout — group lodging demand intelligence*
*Sandman Hotel Vancouver Airport, Richmond BC*
*Consolidated roadmap — see `PHASES.md` for atomic task tracking*
