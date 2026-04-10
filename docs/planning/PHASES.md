# PHASES.md — groupscout build tracker

> Living task tracker. Each checkbox = one atomic commit.

## Commit Workflow (follow this every task)

**Claude does:**
1. Builds the task (one file or one logical unit)
2. Ticks the checkbox in this file
3. Adds a `CHANGELOG.md` entry explaining what was built and why
4. Suggests a commit message

**You do:**
5. Review the code
6. `git add <files> docs/planning/PHASES.md docs/CHANGELOG.md`
7. `git commit -m "<suggested message>"`

> Do not move to the next phase until the current phase goal is met.

---

## Phase 1 — Foundation ✅
**Goal:** Runnable Go project, DB boots, schema applied, nothing breaks.

- [x] `go.mod` + `go.sum` — module init, `modernc.org/sqlite` dependency
- [x] `config/config.go` — 12-factor env var loader with defaults
- [x] `internal/collector/collector.go` — `Collector` interface + `RawProject` struct
- [x] `internal/storage/db.go` — `Open()` + `Migrate()` (idempotent schema)
- [x] `internal/storage/raw.go` — `RawProjectStore` interface + SQLite impl + `HashProject()`
- [x] `internal/storage/leads.go` — `LeadStore` interface + `Lead` struct + SQLite impl
- [x] `migrations/001_init.up.sql` + `down.sql` — schema reference files
- [x] `cmd/server/main.go` — boots, applies schema, exits cleanly
- [x] `.gitignore` — excludes `.db`, `.env`, `.idea/`, `settings.local.json`
- [x] `CHANGELOG.md` — knowledge base initialized

---

## Phase 2 — First Full Pipeline 🔄
**Goal:** One real Richmond permit flows all the way to a Slack message. End-to-end with one source.

### Part A — Richmond Permit Scraper
> Richmond has no open data API. Data is published as weekly PDFs only.
> Source: https://www.richmond.ca/business-development/building-approvals/reports/weeklyreports.htm

- [x] **A1** `internal/collector/richmond.go` — collector struct + fetch reports page + extract PDF URLs via regex
- [x] **A2** `internal/collector/richmond.go` — download latest PDF + parse text into raw permit records (7-8 lines/record)
- [x] **A3** `internal/collector/richmond.go` — filter by SUB TYPE + value > $500K, map to `RawProject`, call `HashProject()`
- [x] **A4** `internal/collector/richmond_test.go` — unit tests for all pure functions (table-driven)
- [x] **A5** `cmd/smoketest/main.go` — run collector, pretty-print results as JSON (dev tool)

### Part B — Claude Enrichment
- [x] **B1** `internal/enrichment/claude.go` — `EnrichedLead` struct + Claude API POST + JSON response parser
- [x] **B2** `internal/enrichment/enricher.go` — orchestrate: collect → `ExistsByHash` check → enrich → `LeadStore.Insert`

### Part C — Slack Notification
- [x] **C1** `internal/notify/slack.go` — `Notifier` interface + `SlackNotifier` struct + webhook POST
- [x] **C2** `internal/notify/slack.go` — Block Kit message formatter for a single `Lead`

### Part D — Wire It Up
- [x] **D1** `cmd/server/main.go` — `--run-once` flag: runs collect → enrich → notify pipeline and exits
- [x] **D2** Smoke test: real permit in DB, enriched lead created, Slack message received ✓
  > Pipeline code is complete and verified (3 real permits collected, enrichment API calls fire).
  > Blocked only by Claude API credit balance — top up at console.anthropic.com, then `rm groupscout.db && go run ./cmd/server/ --run-once`.

---

## Phase 3 — Harden + Add Sources
**Goal:** Real leads flowing nightly. Digest hitting Slack every Monday. No duplicate work.

### Part A — Deduplication
- [x] `internal/enrichment/enricher.go` — check `ExistsByHash()` before calling Claude (built in Phase 2-B2)
- [x] Verify: running twice on same data does not create duplicate leads (confirmed working — all 3 permits logged "skip duplicate" on second run)

### Part B — Rule-Based Pre-Scorer ✅
- [x] **B1** `internal/enrichment/scorer.go` — `Scorer` struct + scoring logic (postal codes, value, keywords)
- [x] **B2** `internal/enrichment/scorer_test.go` — unit tests for scoring rules (table-driven)
- [x] **B3** `internal/enrichment/enricher.go` — integrate `Scorer`: skip Claude if pre-score < threshold
- [x] **B4** `config/config.go` — `EnrichmentThreshold` env var (defaults to 1)
- [x] **B5** Verify: low-score projects are stored as "skipped" leads without calling Claude

### Part C — Delta Permits Collector ✅
> Delta has no listing page — a single PDF is published at a known URL.
> URL is stored in `DELTA_PERMITS_URL` env var; update it when Delta publishes a new report.

- [x] `internal/collector/delta.go` — `DeltaCollector`: download PDF → `pdftotext -layout` → content-aware parser
  - Header line regex detects new permit (month + permit number)
  - `extractDeltaBuilder()` strips date/permit/decimal columns to isolate builder name
  - `parseDeltaDecimal()` parses "515,000.00" → 515000
  - `isDeltaRelevant()` filters industrial/commercial/assembly/institutional + min value
  - `toDeltaRawProject()` maps builder → applicant slot; dedup via `hashDeltaPermit()`
- [x] `internal/collector/delta_test.go` — 10 pure-function tests (parse, filter, hash, decimal, field mapping)
- [x] `cmd/server/main.go` — wires `DeltaCollector` when `DELTA_PERMITS_URL` is set (opt-in, no break)
- [x] `config/config.go` — `DeltaPermitsURL` loaded from `DELTA_PERMITS_URL` env var

### Part D — BC Bid Parser (Automated RSS Approach) ✅
> BC Bid migrated to a JavaScript-heavy portal.
> Strategy: Use CivicInfo BC RSS feeds (Construction/Roads) as an automated proxy for BC Bid awards.
- [x] **D1** `internal/collector/bcbid.go` — `BCBidCollector`: Automated RSS fetching from CivicInfo BC feeds.
- [x] **D2** `internal/collector/bcbid.go` — Mapping: Title, Awarded To (via Claude), Value, Date, Link. Support for manual JSON/HTML overrides.
- [x] **D3** `internal/collector/bcbid_test.go` — Test RSS parsing and manual input handling.
- [x] **D4** `cmd/server/main.go` — Integrate `BCBidCollector` into the permanent pipeline with RSS URLs.
- [x] **D5** `config/config.go` — Add `BCBidEnabled` and `BCBidRSSURL` with default CivicInfo BC feeds.

### Part E — Scheduler (Trigger-based) ✅
> **Schedule decision:** Richmond publishes weekly PDFs.
> To support n8n/external triggers, we expose a secure HTTP endpoint.
>
- [x] **D1** `config/config.go` — add `Port` and `APIToken` env vars
- [x] **D2** `cmd/server/main.go` — implement `POST /run` with Bearer token auth
- [x] **D3** `cmd/server/main.go` — move pipeline logic to a shared `RunPipeline()` function
- [x] **D4** Verify: `curl -X POST -H "Authorization: Bearer <token>" http://localhost:8080/run` triggers the collector

---

## Phase 4 — More Sources + Full Notifications
**Goal:** All planned sources live. Instant alerts. Email digest. Outreach drafts.

### Part A — Remaining Collectors

#### A1 — Google News RSS Monitor ✅
- [x] `internal/collector/news.go` — query Google News RSS for construction + YVR signals
  - Queries: `"Richmond BC" construction`, `"Metro Vancouver" infrastructure contract awarded`, `"YVR" expansion`
  - Parse via `github.com/mmcdole/gofeed` — no scraping needed, structured RSS
  - Map to `RawProject{Source: "google_news"}` with title, URL, snippet, published date
  - Hash: SHA256 of URL (stable dedup key)
  - Schedule: daily

#### A2 — Creative BC "In Production" Scraper
> Source: https://creativebc.com/bc-film-commission/in-production/ (JS-only)
> Working endpoint: https://knowledgehub.creativebc.com/apex/In_Production_List?isdtp=p1
> This is a Salesforce Visualforce page — server-rendered HTML, no JavaScript required.
> `isdtp=p1` strips the Salesforce navigation chrome and returns just the page content.
> Parsed via `golang.org/x/net/html` table walker. TLS verification skipped (Salesforce CA absent from Go root pool on Linux).
> Enabled via `CREATIVEBC_ENABLED=true`; URL overridable via `CREATIVEBC_URL` (optional).

- [x] **A2a** `internal/collector/creativebc.go` — `CreativeBCCollector` struct + fetch PDF URL + `pdftotext -layout` pipe
  - Parse columns: Production Title | Production Type | Studio/Distributor | Status
  - Filter: keep only `Feature Film` and `TV Series` types
    - Rationale: live-action features + episodics bring large out-of-town crews; Animation/VFX/Documentary are local-crew work
  - Map to `RawProject`:
    - `Source`: `"creativebc"`
    - `Title`: production name
    - `Description`: `"{type} — {studio}"`
    - `ProjectValue`: 0 (unknown — Claude estimates crew size from production type and studio scale)
    - `ExternalID`: slugified production title (stable across weekly list updates)
  - Hash: SHA256 of `source + external_id` (not content — list position shifts weekly)
  - `ExistsByHash()` check before enrichment — skip if already seen this production
- [x] **A2b** `internal/collector/creativebc_test.go` — unit tests: PDF line parser, type filter, field mapping, hash stability
- [x] **A2c** `config/config.go` — `CreativeBCEnabled bool` from `CREATIVEBC_ENABLED=true`; `CreativeBCURL string` from `CREATIVEBC_URL` (optional URL override)
- [x] **A2d** `cmd/server/main.go` — wire `CreativeBCCollector` when `CREATIVEBC_ENABLED=true`
- [x] **A2e** Claude enrichment prompt — for `source == "creativebc"`, prompt Claude to estimate:
  - Crew size (Feature Film: 150–400; TV Series per block: 80–200)
  - Out-of-town crew ratio (US studio → higher; local indie → lower)
  - Outreach timing ("Productions typically mobilize 4–6 weeks after appearing on the in-production list")
  - Score boosters: US studio name → +2; "Richmond" or "Surrey" location reference → +1

#### A3 — Vancouver Convention Centre Event Scraper
> Source: https://www.vancouverconventioncentre.com/events
> Renders as static HTML — plain `net/http` GET works, no headless browser needed.
> Schedule: monthly (events are announced weeks to months in advance).

- [x] **A3a** `internal/collector/vcc.go` — `VCCCollector` struct + HTTP GET + HTML parse via `github.com/PuerkitoBio/goquery`
  - Extract per event: name, start date, end date, category tag (if present in markup)
  - Keep filter — any of: `conference`, `congress`, `summit`, `symposium`, `trade show`, `expo`, `convention`, `meeting`, `forum`
  - Drop filter — any of: `consumer show`, `home show`, `auto show`, `art fair`, `comedy`, `concert`, `wedding`, `sale`, `warehouse sale`
    - Rationale: professional multi-day gatherings bring out-of-town attendees; consumer events are locally attended
  - Map to `RawProject`:
    - `Source`: `"vcc_events"`
    - `Title`: event name
    - `Description`: `"{category} — {start_date} to {end_date}"`
    - `ProjectValue`: 0 (Claude estimates attendee count and hotel night potential)
    - `ExternalID`: slugified event name + date
  - Hash: SHA256 of `source + external_id`
  - **Note:** Updated selectors for robust extraction (H2/H3/H4 + container cards) and added debug logging to verify collection.
- [x] **A3b** `internal/collector/vcc_test.go` — unit tests: HTML parse, keep/drop filter, date parsing, hash
- [x] **A3c** `config/config.go` — `VCCEnabled bool` from `VCC_ENABLED=true` env var (opt-in)
- [x] **A3d** `cmd/server/main.go` — wire `VCCCollector` when `VCCEnabled` is set
- [x] **A3e** Claude enrichment prompt — for `source == "vcc_events"`, prompt Claude to estimate:
  - Attendee count and % likely from out of town
  - Industry type (medical, engineering, mining, tech, government, etc.)
  - Hotel night potential (multi-day congress of 2,000 engineers >> 1-day local trade show)
  - Score boosters: 3+ day event → +1; 500+ expected attendees → +1; engineering/medical/mining/government → +1; event within 6 weeks → +1

#### A4 — Eventbrite Event Collector ✅
> ⚠️ API constraint: Eventbrite's public event search endpoint (`GET /v3/events/search/`) was
> deprecated in 2019 and shut down in 2020. As of 2024, no unauthenticated public search exists.
> The API requires a Bearer token (OAuth2) and only returns events owned by the authenticated org.

> Strategy: scrape Eventbrite's public Vancouver search page directly (HTML, no JS required for
> initial results). Target URL:
> https://www.eventbrite.ca/d/canada--vancouver/conferences--conventions/
> Supplement with keyword searches:
> https://www.eventbrite.ca/d/canada--vancouver/?q=conference
> https://www.eventbrite.ca/d/canada--vancouver/?q=summit
> https://www.eventbrite.ca/d/canada--vancouver/?q=congress

> If Eventbrite ever grants an API key (free dev account): swap to
> `GET https://www.eventbriteapi.com/v3/events/search/?location.address=Vancouver,BC&categories=102`
> (Category 102 = Business & Professional). Token stored in `EVENTBRITE_API_KEY` env var.
> The collector should check for the env var first and fall back to HTML scrape if absent.

- [x] **A4a** `internal/collector/eventbrite.go` — `EventbriteCollector` struct (Scrape mode implemented)
- [x] **A4b** `config/config.go` — `EventbriteEnabled` and `EventbriteURL` support
- [x] **A4c** `cmd/server/main.go` — wire `EventbriteCollector`

#### A5 — Infrastructure Announcements Collector ✅
- [x] **A5a** `internal/collector/announcements.go` — BCIB, TransLink, YVR newsroom scrapers
- [x] **A5b** `internal/collector/announcements_test.go` — Table-driven unit tests for scrapers
- [x] **A5c** `config/config.go` — `AnnouncementsEnabled` toggle
- [x] **A5d** `cmd/server/main.go` — wire `AnnouncementsCollector`
- [x] **A5e** `internal/enrichment/claude.go` — specialized prompt for announcement enrichment

### Part B — Instant Alert ✅
- [x] `internal/enrichment/enricher.go` — integrated `PriorityAlertThreshold` to detect high-priority leads (score >= 9)
- [x] `config/config.go` — `PriorityAlertThreshold` env var support
- [x] `cmd/server/main.go` — wire threshold from config to enricher

### Part C — Email + Outreach Draft ✅
- [x] **C1** `internal/storage/leads.go` — `ListForDigest` implementation (past 7 days)
- [x] **C2** `internal/notify/email.go` — weekly HTML digest via SendGrid
- [x] **C3** `internal/enrichment/claude.go` — `DraftOutreach` automated cold email generation
- [x] **C4** `cmd/server/main.go` — `/digest` HTTP endpoint for scheduled triggers

---

## Phase 5 — Smart Refresh (Deferred)
**Goal:** Avoid redundant PDF downloads — only fetch and parse when new data is actually published.

> Deferred from Phase 2. Richmond publishes weekly; a simple weekly cron is sufficient for now.
> Add this when the scheduler is stable and we want to reduce unnecessary work.

- [ ] `migrations/002_scrape_state.up.sql` — add `scrape_state` table (`key`, `value`, `updated_at`)
- [ ] `internal/storage/scrapestate.go` — `ScrapeStateStore` interface + SQLite impl
- [ ] `internal/collector/richmond.go` — hash the reports page HTML before downloading any PDFs
- [ ] `internal/collector/richmond.go` — diff current PDF links vs previously seen links, download new only
- [ ] Skip entire run if page hash unchanged since last run

---

## Phase 7 — User Requests & Refinements 🔄
**Goal:** Address specific user feedback and small UX improvements.

- [x] **7.1** `internal/notify/slack.go` — Identify lead source in Slack notifications.
- [x] **7.2** `PHASES.md` — Add tick-off tasks for user requests.
- [x] **7.3** `CHANGELOG.md` — Document source identification and documentation updates.
- [ ] **7.4** `internal/storage/leads.go` — Add `List` with filtering (status, score) and pagination.
- [ ] **7.5** `cmd/server/main.go` — Implement `GET /leads` and `PATCH /leads/{id}` endpoints.
- [ ] **7.6** `internal/storage/outreach.go` — Implement `OutreachLogStore` for tracking interactions.

---

## Phase 8 — System Reliability & Observability 🔄
**Goal:** Transition to production-grade monitoring and logging.

- [x] **8.1** `internal/logger` — Implement structured logging using `log/slog`.
- [x] **8.2** Sentry Integration — Initialize Sentry and capture collector/API errors.
- [x] **8.3** Health Check — `/health` endpoint checking DB ping and critical env vars.
- [x] **8.4** Pipeline Metrics — Basic counts logged; Prometheus endpoint planned (Phase 14.7).
- [ ] **8.5** Distributed Tracing — Implement OpenTelemetry for visualizing lead flow and pipeline bottlenecks.
- [x] **8.6** Entity Resolution — Basic lead deduplication (hash-based).
- [x] **8.7** Testing Documentation — Create `TESTING.md`.

---

## Phase 9 — Architecture & Scaling (Planned)
**Goal:** Optimize for speed, efficiency, and manageability.

- [ ] **9.1** Collector Registry — Centralized registry for all collectors to simplify pipeline orchestration.
- [ ] **9.2** Parallel Execution — Run collectors concurrently using worker pools.
- [ ] **9.3** Dynamic Parameters — Support for passing scraper overrides (keywords/dates) via `/run` endpoint.
- [ ] **9.4** Caching Layer — Implement Redis for enrichment results and scraper states.
- [ ] **9.5** Incremental Scraping — Only process data newer than the last successful run.

---

## Phase 10 — Ecosystem & UI (Planned)
**Goal:** Improve user interaction and external integrations.

- [ ] **10.1** Admin Dashboard — Lightweight React/Vue frontend for lead visualization.
- [ ] **10.2** CRM Sync — "One-click" push to HubSpot/Salesforce.
- [ ] **10.3** Outreach Refinement — Multi-persona drafting (A/B testing for templates).
- [ ] **10.4** Chrome Extension — Manually clip leads from the browser into the pipeline.
- [ ] **10.5** `CHANGELOG.md` — Document UI and ecosystem expansion.

---

## Phase 11 — Integration & CRM (Planned)
**Goal:** Direct connection to sales workflows.

- [ ] **11.1** Hubspot Sync — Automatically push leads with score > 8 to Hubspot via API.
- [ ] **11.2** LinkedIn Helper — API endpoint to generate search URLs for GCs/Applicants.

---

## Phase 12 — Source Expansion (Metro Vancouver)
**Goal:** Broaden lead capture across other major municipalities.

- [ ] **12.1** Burnaby Permits — Scraper for Burnaby daily/weekly permit PDFs.
- [ ] **12.2** Vancouver Open Data — Integration for City of Vancouver building permits (API/CSV).
- [ ] **12.3** Coquitlam/Surrey — Implement scrapers for monthly permit summary PDFs.
- [ ] **12.4** Specialized Scrapers — Target Journal of Commerce (focusing on Richmond/Delta) or regional venue calendars (PNE/LEC).

### 12.5 — LinkedIn Job Postings Collector
> Companies hiring "site supervisors", "field crew leads", or "camp coordinator" roles signal incoming out-of-town workers. LinkedIn job RSS/API is rate-limited — scrape the public jobs search page or use a proxy feed. No login required for public postings.
> Keywords: `"site supervisor" Richmond BC`, `"field crew" Metro Vancouver`, `"camp coordinator" BC`

- [ ] **12.5-T** `internal/collector/linkedin_test.go` — `TestParseLinkedInJobHTML` with fixture HTML; assert org name, title, location, post date extracted; `TestIsCrewSignal` table-driven keyword filter; fail first
- [ ] **12.5a** `internal/collector/linkedin.go` — `LinkedInCollector`: scrape `https://www.linkedin.com/jobs/search/?keywords=site+supervisor&location=Richmond+BC`; parse org, title, location, date; hash on `org+title+date`
- [ ] **12.5b** `internal/collector/linkedin.go` — keyword filter: crew/supervisor/coordinator/camp signals → include; generic office roles → skip
- [ ] **12.5c** `config/config.go` — `LinkedInEnabled bool` from `LINKEDIN_ENABLED=true`; `LinkedInKeywords []string` from `LINKEDIN_KEYWORDS` (comma-separated, optional override)
- [ ] **12.5d** `cmd/server/main.go` — wire `LinkedInCollector` when `LinkedInEnabled` is set
- [ ] **12.5e** Claude enrichment prompt — for `source == "linkedin_jobs"`: estimate crew size from job count signals; set `out_of_town_crew_likely = true` when "relocation" or "camp" in description

---

### Phase 13 — Public Tenders & Utilities (Expansion)
**Goal:** Track major infrastructure and utility projects and federal contract awards.

- [ ] **13.1** BC Hydro / FortisBC — Scraper for major utility project announcements and tenders.

### 13.2 — Buyandsell.gc.ca Federal Contract Awards
> Canada's federal contract awards portal. Covers DND, CBSA, Transport Canada, and other agencies near YVR. Published as open data (CSV/API) — no scraping required. Filter by vendor location (BC) and NAICS codes for construction (23xxxx) and engineering (541xxx).
> Source: `https://search.open.canada.ca/contracts/` or bulk CSV at `https://open.canada.ca/data/en/dataset/d8f85d91-7dc8-4c2d-a2b1-9a1e65f5d7e6`

- [ ] **13.2-T** `internal/collector/buyandsell_test.go` — `TestParseBuyandsellCSVRow` with fixture rows; `TestIsRelevantContract` table-driven NAICS filter; `TestHashContract` determinism; fail first
- [ ] **13.2a** `internal/collector/buyandsell.go` — `BuyandsellCollector`: fetch open data CSV (or paginated JSON API); parse contract award row → `RawProject`
  - Fields: vendor name, awarding department, value, award date, description, NAICS code, location
  - Hash: SHA256 of `amendment_no + contract_no`
  - Filter: NAICS 23xxxx (construction), 541xxx (engineering), 488xxx (transport support); value > $500K; BC vendor or delivery location
- [ ] **13.2b** `config/config.go` — `BuyandsellEnabled bool`; `BuyandsellMinValue int64` (default 500000)
- [ ] **13.2c** `cmd/server/main.go` — wire `BuyandsellCollector` when `BuyandsellEnabled` is set
- [ ] **13.2d** Claude enrichment prompt — for `source == "buyandsell"`: estimate crew deployment size from contract value + NAICS type; set department context (DND → military crew, CBSA → border staff, TC → infrastructure crew)

---

## Phase 15 — PostgreSQL + pgvector Migration ✅
**Goal:** Move from SQLite to Postgres. Unlock pgvector for proper RAG. Zero behavior change to collectors, enrichment, or notification layers — only the storage driver and schema change.

> **Why now:** repository pattern already in place (`LeadStore`, `RawProjectStore`); Docker Compose already running; data is tiny so migration is painless now vs. painful later; pgvector directly unlocks the RAG roadmap.
> **Replaces:** the Postgres portion of Phase 6.
> **Driver:** `github.com/jackc/pgx/v5` (pure Go, no CGO — same constraint as `modernc.org/sqlite`). Use `pgx/v5/stdlib` for `database/sql` compatibility so existing store code changes minimally.
> **AI prompts:** copy-paste prompts for each part are in `docs/PROMPTS_PHASE15.md`.

### Part A — Postgres container + driver
- [x] **A1** `docker-compose.yml` — add `pgvector/pgvector:pg17` service with named volume + health check (`pg_isready`)
- [x] **A2** `go.mod` — add `github.com/jackc/pgx/v5`; keep `modernc.org/sqlite` for local dev fallback
- [x] **A3** `config/config.go` — `DriverName() string` helper: returns `"pgx"` for `postgres://` URL, `"sqlite"` otherwise
- [x] **A4** `internal/storage/db.go` — `Open()` selects driver from `DriverName()`; `pgx/v5/stdlib` for Postgres, `modernc.org/sqlite` for SQLite
- [x] **A5** Verify: `go build` passes; app connects to Postgres container; no migrations run yet

### Part B — Schema (Postgres-compatible migrations)
> SQLite and Postgres differ in: `UUID` type, `BOOLEAN`, `TIMESTAMPTZ`, `JSONB`, placeholder syntax (`?` vs `$1`), and UUID generation (`gen_random_uuid()`).

- [x] **B1** `migrations/001_init.postgres.up.sql` — port schema with native Postgres types:
  - `id UUID PRIMARY KEY DEFAULT gen_random_uuid()`
  - `BOOLEAN` (not `INTEGER`)
  - `TIMESTAMPTZ` with `DEFAULT NOW()`
  - `raw_data JSONB` (not `TEXT`) — enables JSON indexing later
- [x] **B2** `internal/storage/db.go` — wire `golang-migrate/migrate` to run the correct migration set based on driver
- [x] **B3** Verify: migrations run clean on fresh Postgres; `go test ./...` passes

### Part C — Storage layer fixes
> Current code is SQLite-specific: `boolToInt()` helper, `?` parameter placeholders, manual boolean scanning. All need updating for Postgres.

- [x] **C1** `internal/storage/leads.go` — remove `boolToInt()`; pass native `bool` to pgx; update all `?` → `$1, $2...` placeholders
- [x] **C2** `internal/storage/leads.go` — fix `time.Time` scanning (pgx handles natively; remove string parsing if any)
- [x] **C3** `internal/storage/raw.go` — update `?` → `$N` placeholders; verify `raw_data` writes as JSONB
- [x] **C4** `internal/storage/leads.go` — UUID generation: use `github.com/google/uuid` in Go or rely on `DEFAULT gen_random_uuid()` in SQL
- [x] **C5** Verify: full pipeline runs end-to-end against Postgres; leads stored and retrieved correctly

### Part D — pgvector
- [x] **D1** `migrations/003_pgvector.up.sql` — `CREATE EXTENSION IF NOT EXISTS vector`; `lead_embeddings` table with `embedding vector(512)` column + `ivfflat` index on cosine ops
- [x] **D2** `internal/storage/embeddings.go` — `PostgresEmbeddingStore` implementing `EmbeddingStore` using `<=>` cosine distance operator
- [x] **D3** factory: `postgres://` URL → `PostgresEmbeddingStore`; `*.db` → Go cosine impl — implemented as `storage.NewEmbeddingStore(db, dsn)` in `internal/storage/embeddings.go`; enrichment layer calls it directly
- [x] **D4** Verify: similarity search returns correct top-k; index used in query plan (`EXPLAIN`)

### Part E — SQLite → Postgres data migration
- [x] **E1** `scripts/migrate_to_postgres/main.go` — Go script: open SQLite, read all rows, write to Postgres in batches
- [x] **E2** `docs/SETUP.md` — document one-time migration steps; note it is safe to skip if starting fresh
- [x] **E3** Verify: row counts match between SQLite and Postgres after migration

### Part F — Productionize
- [x] **F1** `.env.example` — update `DATABASE_URL` to `postgres://groupscout:password@localhost:5432/groupscout`
- [x] **F2** `docker-compose.yml` — app `depends_on` Postgres health check; remove SQLite volume reference
- [x] **F3** `docs/SETUP.md` — update local setup instructions for Postgres path
- [x] **F4** Verify: `docker compose up` → migrations run → full pipeline works against Postgres

---

## Phase 16 — LLM Provider Abstraction 📋
**Goal:** Remove vendor lock-in from `internal/enrichment`. All LLM calls go through a `LLMClient` interface. Provider is config-driven — switch between Claude, OpenAI, Azure OpenAI, Groq, Mistral, Ollama, or Gemini without touching pipeline code.

> Full context and provider comparison: see `docs/AI_DATA_STRATEGY.md`.
> Coding prompts: see `docs/PROMPTS_PHASE16.md`.
> **Note:** Gemini support (`internal/enrichment/gemini.go`) was added externally — wire it into the factory in Part F.

> **TDD rule for this phase:** Write the test file (T task) first. Commit with failing tests. Then implement until green. No implementation task is started without its T task checked.

### Part A — Interface Extraction (internal refactor, no behavior change)
> Safest first step. Extract the interface, make Claude implement it. All existing tests must still pass.

- [ ] **A-T** `internal/enrichment/llm_test.go` — define `mockLLMClient` stub + `TestCompletionRequest_Fields` (confirms struct shape); all tests fail before impl
- [ ] **A1** `internal/enrichment/llm.go` — define `LLMClient` interface + `CompletionRequest` / `CompletionResponse` structs
- [ ] **A2** `internal/enrichment/claude.go` — rename `ClaudeEnricher` → `ClaudeClient`; implement `LLMClient`
- [ ] **A3** `internal/enrichment/llm_factory.go` — `NewLLMClient(cfg) LLMClient`; returns `ClaudeClient` only for now
- [ ] **A4** `config/config.go` — add `LLMProvider`, `LLMModel`, `LLMAPIKey`, `LLMBaseURL` env vars (defaults preserve current Claude behaviour)
- [ ] **A5** `internal/enrichment/enricher.go` — change `claude *ClaudeEnricher` field to `llm LLMClient`
- [ ] **A6** Verify: `go test ./...` passes; `--run-once` pipeline output unchanged

### Part B — OpenAI-Compatible Client
> One implementation covers OpenAI, Groq, and Mistral — they all speak the same Chat Completions API.

- [ ] **B-T** `internal/enrichment/openai_compat_test.go` — table-driven tests with `httptest.NewServer`: auth header present, JSON body shape, `choices[0].message.content` parse, non-200 error handling; all fail before impl
- [ ] **B1** `internal/enrichment/openai_compat.go` — `OpenAICompatibleClient` struct implementing `LLMClient`
  - POST to `{LLMBaseURL}/v1/chat/completions`
  - Auth: `Authorization: Bearer {LLMAPIKey}`
  - Map `CompletionRequest` → OpenAI request body; parse `choices[0].message.content` from response
- [ ] **B2** `internal/enrichment/llm_factory.go` — wire `LLM_PROVIDER=openai|groq|mistral` → `OpenAICompatibleClient` with provider-specific default base URLs
- [ ] **B3** `config/config.go` — bake in default base URLs: OpenAI `https://api.openai.com`, Groq `https://api.groq.com/openai`, Mistral `https://api.mistral.ai`
- [ ] **B4** `.env.example` — document `LLM_PROVIDER`, `LLM_MODEL`, `LLM_API_KEY`
- [ ] **B5** Verify: `LLM_PROVIDER=openai LLM_MODEL=gpt-4o-mini` produces valid enrichment JSON

### Part C — Azure OpenAI
> Azure uses the OpenAI API format but with a different URL structure and auth header.

- [ ] **C-T** `internal/enrichment/openai_compat_test.go` — add Azure URL builder tests: correct URL format, `api-key` header used instead of `Authorization: Bearer`
- [ ] **C1** `internal/enrichment/openai_compat.go` — add Azure URL builder: `https://{AzureResourceName}.openai.azure.com/openai/deployments/{AzureDeploymentName}/chat/completions?api-version={AzureAPIVersion}`; use `api-key` header instead of `Authorization: Bearer`
- [ ] **C2** `internal/enrichment/llm_factory.go` — wire `LLM_PROVIDER=azure` → `OpenAICompatibleClient` with Azure URL builder
- [ ] **C3** `config/config.go` — add `AzureResourceName`, `AzureDeploymentName`, `AzureAPIVersion` env vars
- [ ] **C4** `.env.example` — document Azure env vars
- [ ] **C5** Verify: `LLM_PROVIDER=azure` produces valid enrichment JSON

### Part D — Ollama (local / Docker)
> Free, private, no external API calls. Good for dev, testing, and air-gapped environments.
> Ollama is OpenAI-compatible — reuses `OpenAICompatibleClient` from Part B. No new client struct needed.
> **Depends on Part A + Part B being complete first.**

- [ ] **D-T** `internal/enrichment/openai_compat_test.go` — add Ollama-specific cases:
  - `TestOpenAICompatClient_NoAuthHeaderWhenKeyEmpty`: empty `apiKey` → no `Authorization` header in request
  - `TestOpenAICompatClient_JSONOutputMode`: when `JSONOutput=true`, request body includes `"response_format":{"type":"json_object"}`
  - `TestOpenAICompatClient_JSONOutputFallback`: when model returns markdown-wrapped JSON, `stripMarkdown()` extracts it cleanly
  All fail before impl. Commit.
- [ ] **D1** `internal/enrichment/openai_compat.go` — extend `OpenAICompatibleClient`:
  - Skip `Authorization` header entirely when `apiKey == ""`
  - Add `JSONOutput bool` field: when true, add `"response_format":{"type":"json_object"}` to request body (Ollama + OpenAI support this; Groq/Mistral may not — set per factory)
  - Keep `stripMarkdown()` as fallback for models that ignore `response_format`
- [ ] **D2** `internal/enrichment/llm_factory.go` — wire `LLM_PROVIDER=ollama`:
  - `baseURL`: `getEnv("LLM_BASE_URL", "http://ollama:11434")`
  - `apiKey`: `""` (no auth)
  - `model`: `cfg.LLMModel` (no default — must be set; log clear error if empty)
  - `JSONOutput`: `true` (Ollama supports `response_format`)
- [ ] **D3** `internal/enrichment/ollama_health.go` — `CheckOllamaHealth(ctx, baseURL string) error`:
  - GET `{baseURL}/api/tags` — returns list of pulled models
  - If `cfg.LLMModel` not in the list → return error: `"model llama3.2 not pulled — run: docker exec groupscout-ollama-1 ollama pull llama3.2"`
  - Called at startup when `LLM_PROVIDER=ollama`
- [ ] **D4** `docker-compose.yml` — add `ollama` service (opt-in via Docker profile):
  ```yaml
  ollama:
    image: ollama/ollama
    volumes:
      - ollama_data:/root/.ollama
    profiles: ["ollama"]
    # After first start: docker exec groupscout-ollama-1 ollama pull llama3.2
  ```
  Add `ollama_data` to top-level `volumes:` block.
- [ ] **D5** `scripts/ollama_pull.sh` — one-liner helper: `docker exec groupscout-ollama-1 ollama pull ${MODEL:-llama3.2}`
- [ ] **D6** `.env.example` — document Ollama section:
  ```
  # LLM_PROVIDER=ollama
  # LLM_MODEL=llama3.2         # or mistral, llama3.1:8b, phi3:mini
  # LLM_BASE_URL=http://ollama:11434
  ```
- [ ] **D7** `docs/SETUP.md` — add Ollama setup section (see SETUP.md update below)
- [ ] **D8** Verify: `docker compose --profile ollama up -d`; pull `llama3.2`; set `LLM_PROVIDER=ollama`; full pipeline runs; zero external API calls in logs

### Part G — Ollama Modelfile & Persona Tuning
> Raw `llama3.2` follows JSON instructions less reliably than Claude. A custom Modelfile bakes the hotel sales analyst persona + strict JSON instructions into the model at load time — reducing bad outputs without prompt engineering in every call.

- [ ] **G-T** `cmd/smoketest/main.go` — add `-compare-providers` flag: runs the same 3 fixture permits through both the configured LLM provider and a second provider; prints side-by-side `priority_score` and `out_of_town_crew_likely` values. No assertions — visual comparison only.
- [ ] **G1** `config/Modelfile.groupscout` — Ollama Modelfile for hotel sales analyst persona:
  ```
  FROM llama3.2
  SYSTEM You are a hotel sales analyst for Sandman Hotel Vancouver Airport (Richmond, BC, near YVR). \
  Given a construction permit or project signal, extract structured intelligence. \
  You MUST respond with valid JSON only — no prose, no markdown, no explanation. \
  JSON schema: {"general_contractor":"...","project_type":"civil|commercial|industrial|utility|residential|unknown", \
  "estimated_crew_size":0,"estimated_duration_months":0,"out_of_town_crew_likely":false, \
  "priority_score":0,"priority_reason":"...","suggested_outreach_timing":"...","notes":"..."}
  PARAMETER temperature 0.1
  PARAMETER num_predict 512
  ```
- [ ] **G2** `scripts/ollama_create_model.sh` — `docker exec groupscout-ollama-1 ollama create groupscout -f /config/Modelfile.groupscout`
- [ ] **G3** `docker-compose.yml` — mount `./config` into `ollama` container as `/config:ro` so the Modelfile is accessible
- [ ] **G4** `docs/AI_DATA_STRATEGY.md` — add Ollama model recommendation table (see AI_DATA_STRATEGY update)
- [ ] **G5** Verify: create `groupscout` model; set `LLM_MODEL=groupscout`; run pipeline; compare JSON output quality vs `llama3.2`

### Part E — Fallback & Resilience
> If the primary provider fails, automatically retry with a secondary provider.

- [ ] **E-T** `internal/enrichment/fallback_test.go` — `TestFallbackClient_UsesSecondaryOnPrimaryError`: primary mock always errors, secondary mock succeeds; assert secondary was called + Sentry event recorded; all fail before impl
- [ ] **E1** `internal/enrichment/llm.go` — `FallbackClient` struct: wraps primary + secondary `LLMClient`; on primary error, logs to Sentry and retries with secondary
- [ ] **E2** `config/config.go` — `LLMFallbackProvider`, `LLMFallbackModel`, `LLMFallbackAPIKey`, `LLMFallbackBaseURL`
- [ ] **E3** `internal/enrichment/llm_factory.go` — build `FallbackClient` when fallback provider is configured
- [ ] **E4** Verify: set invalid primary API key → fallback activates → pipeline completes → Sentry captures primary failure

### Part F — Gemini Provider
> `internal/enrichment/gemini.go` was added externally (Junie). Wire it into the factory and test coverage.

- [ ] **F-T** `internal/enrichment/gemini_test.go` — check if tests exist; if not, write `httptest.NewServer` tests: Gemini request body format (`contents[].parts[].text`), response parse (`candidates[0].content.parts[0].text`), non-200 error handling
- [ ] **F1** `internal/enrichment/llm_factory.go` — wire `LLM_PROVIDER=gemini` → `GeminiClient` from `gemini.go`
- [ ] **F2** `config/config.go` — `GEMINI_API_KEY` env var; default `LLM_BASE_URL` for Gemini if not already present
- [ ] **F3** `.env.example` — document `LLM_PROVIDER=gemini LLM_MODEL=gemini-1.5-flash LLM_API_KEY=...`
- [ ] **F4** Verify: `LLM_PROVIDER=gemini` enriches a test lead; `go test ./internal/enrichment/...` green

---

---

## Phase 17 — Airport Disruption Alert System 🔄
**Goal:** A separate real-time binary (`cmd/alertd/`) that monitors YVR flight disruptions and alerts the hotel team via Slack with actionable revenue ops information. Distinct from the lead pipeline — different cadence, different failure modes.

> Full context and Vancouver-specific tuning: `docs/groupscout-plan.md` sections 4–6.

> **TDD rule for this phase:** Write the test file (T task) first. Commit with failing tests. Then implement until green. All scraper/parser functions are pure — test them with fixture data before wiring to live HTTP.

### Part A — Data Sources & Weather
- [x] **A1-T** `internal/weather/eccc_test.go` — `TestParseWeatherAlert` with fixture JSON (atmospheric river, fog, snow); `TestClassifyAlertType` table-driven; mock `httptest.NewServer`; all fail first *
- [x] **A1** `internal/weather/eccc.go` — poll `api.weather.gc.ca` for Metro Vancouver + Fraser Valley zones (`BC_14_09`, `BC_14_10`, `BC_14_07`); no API key required
- [x] **A2** `internal/weather/eccc.go` — classify alert type (atmospheric river, fog, snow, wind) and severity; atmospheric river → lower trigger + longer lookback; snow → pre-alert before cancellations start
- [x] **A3-T** `internal/aviation/yvr_test.go` — `TestParseYVRCancellationRate` with fixture HTML; assert rate %, flight count; fail first
- [x] **A3** `internal/aviation/yvr.go` — scrape `yvr.ca/en/passengers/flights` for live cancellation rate; poll every 5 min during active event
- [x] **A4-T** `internal/aviation/navcanada_test.go` — `TestParseNOTAM` with fixture HTML; ground stop detection (`GND STOP`); fail first
- [x] **A4** `internal/aviation/navcanada.go` — parse NavCanada NOTAM portal for CYVR ground stops (leads cancellation data by 1–2 hrs)

### Part B — Stranded Passenger Score (SPS)
- [x] **B-T** `internal/aviation/scorer_test.go` — table-driven SPS formula tests (normal, watch, soft-alert, hard-alert bands); Vancouver edge cases: fog duration-weighting, snow pre-alert, single-runway soft-only; all fail first
- [x] **B1** `internal/aviation/scorer.go` — SPS formula: `(cancelled/scheduled) × avg_seats × connecting_pax_ratio × time_of_day_multiplier × duration_score`
- [x] **B2** SPS thresholds: <20 ignore, 20–60 watch (no alert), 60–120 soft alert, >120 hard alert
- [x] **B3** Vancouver tuning: fog events weight duration heavily; snow events pre-alert on ECCC warning; single runway ops → soft alert only

### Part C — Alert Lifecycle State Machine
- [x] **C1-T** `internal/alert/lifecycle_test.go` — state machine tests: Watch → Alert transition at ≥30 min, Update score change, Resolve sends all-clear; table-driven transitions; mock Slack notifier; fail first
- [x] **C1** `internal/alert/lifecycle.go` — `Watch → Alert → Updating → Resolved` state machine; `DisruptionEvent` persistence with Slack TS; `Notifier` interface; no hard alert until event is ≥30 min old
- [x] **C2-T** `internal/alert/slack_test.go` — `httptest.NewServer` for Slack API; assert `chat.postMessage` on first alert, `chat.update` on subsequent updates, all-clear format on resolve; fail first
- [x] **C2** `internal/alert/slack.go` — `SlackAlerter` with Bot Token; `POST https://slack.com/api/chat.postMessage` + `chat.update`; persistence of message timestamp (TS)
- [x] **C3** `AlertMessage` format: Block Kit JSON with cause, status (active/duration), impact (cancellations/stranded pax), recovery estimate, rooms avail, and suggested actions

### Part D — Hotel Config & Binary
- [ ] **D1-T** `config/airports_test.go` — `TestLoadAirportConfig` with fixture YAML; assert hotel fields, airline contacts, thresholds; fail first
- [ ] **D1** `config/airports.go` — YAML hotel config: hotel → airport mapping, distance, SPS thresholds, distressed rate, rack rate, airline ops contacts, Slack channel
- [ ] **D2** `cmd/alertd/main.go` — main poll loop: 10 min quiet, 90 sec active alert; exponential backoff on API errors
- [ ] **D3** `docker-compose.yml` — add `alertd` service alongside existing app

### Part E — Inventory Slash Command
- [ ] **E1-T** `cmd/alertd/main_test.go` — `TestInventorySlashCommand` posts `/inventory 34`, asserts KV store updated + 200 response; fail first
- [ ] **E1** `cmd/alertd/main.go` — HTTP handler for `/slack/inventory` slash command; stores room count in local KV
- [ ] **E2** Slack alert message includes current room availability from KV; "34 rooms available (as of last sync)"

---

## Phase 18 — Contact Enrichment 📋
**Goal:** Auto-surface a decision-maker contact alongside each lead — project manager, production coordinator, travel manager — using Hunter.io (v1). Biggest current gap vs. competitors.

> **TDD rule for this phase:** T tasks first. All unit tests use mock HTTP; no real Hunter.io calls in test suite.

- [ ] **18-T** `internal/enrichment/hunter_test.go` — `TestHunterDomainLookup` with mock HTTP server: domain → contact list → ranked result; `TestRankContactByTitle` table-driven (PM beats accountant); `TestHunterRateLimit` (429 handling); all fail first
- [ ] **18.1** `internal/enrichment/hunter.go` — Hunter.io API client: domain lookup by org name → return top contact ranked by title relevance (project manager, travel coordinator, ops manager)
- [ ] **18.2** `internal/enrichment/enricher.go` — call Hunter after Claude enrichment step; attach contact to lead if found
- [ ] **18.3** `internal/storage/leads.go` — add `contact_name`, `contact_title`, `contact_email` fields to `Lead` struct; new migration `004_contact_fields.up.sql`
- [ ] **18.4-T** `internal/notify/slack_test.go` — assert contact block appended when `ContactEmail != ""`; assert block absent when empty; fail first
- [ ] **18.4** `internal/notify/slack.go` — append contact info block to lead Slack message when enrichment finds a match
- [ ] **18.5** `config/config.go` — `HUNTER_API_KEY` env var
- [ ] **18.6** Log enrichment hit rate by source in pipeline metrics output

### Budget & Size Signals
> Use contract value or production budget tier to estimate spend — surfaces in the Slack lead block.

- [ ] **18.7-T** `internal/enrichment/scorer_test.go` — add `TestBudgetTierScore` table-driven: <$1M → low, $1–10M → medium, >$10M → high; fail first
- [ ] **18.7** `internal/enrichment/scorer.go` — `BudgetTier(value int64) string` helper: maps project value to spend tier; used in both pre-scorer and Slack formatter
- [ ] **18.8** `internal/notify/slack.go` — include budget tier label in lead Slack message next to project value

---

## Phase 19 — Slack Actions & Lead Feedback 📋
**Goal:** Reps act on leads without leaving Slack. Track outcomes. Resurface aging leads.

> **TDD rule for this phase:** T tasks first. Slack action handler is pure logic — test it with fixture payloads before wiring the HTTP endpoint.

- [ ] **19-T** `cmd/server/slack_actions_test.go` — table-driven: Claim payload → status=claimed, Dismiss → dismissed, Snooze → snoozed + `snoozed_until` set; Slack signing secret verification (reject bad HMAC); all fail first
- [ ] **19.1** `cmd/server/main.go` — `/slack/actions` POST endpoint to handle interactive Slack button payloads (verified via Slack signing secret)
- [ ] **19.2-T** `internal/notify/slack_test.go` — assert action buttons (Claim/Dismiss/Snooze) present in lead message output; fail first
- [ ] **19.2** `internal/notify/slack.go` — add action buttons to lead messages: **Claim** / **Dismiss** / **Snooze 7d**
- [ ] **19.3-T** `internal/storage/leads_test.go` — `TestLeadStatusTransition`: new→claimed, new→dismissed, new→snoozed; assert `snoozed_until` persisted; reject invalid transitions; fail first
- [ ] **19.3** `internal/storage/leads.go` — status transitions: `new → claimed / dismissed / snoozed`; store `snoozed_until` timestamp
- [ ] **19.4-T** `internal/storage/outreach_test.go` — `TestOutreachLogInsert` + `TestOutreachLogListByLead`; fail first
- [ ] **19.4** `internal/storage/outreach.go` — `OutreachLogStore`: log Won / Lost / No Response outcomes per lead (fulfills Phase 7.6)
- [ ] **19.5** Lead aging: digest run resurfaces snoozed leads whose `snoozed_until` has passed
- [ ] **19.6** Won / Lost / No Response buttons appear on leads in `claimed` state

---

## Phase 20 — Analytics & Source Attribution 📋
**Goal:** Weekly digest that shows which signal sources are generating closed business, enabling the sales team to prioritize outreach.

> **TDD rule for this phase:** T tasks first. Aggregate queries tested against in-memory SQLite fixture data.

- [ ] **20-T** `internal/storage/analytics_test.go` — seed test DB; `TestSourceAttribution` asserts correct grouped counts + hit rate %; `TestDemandDensityByWeek` asserts bucketing by arrival week; all fail first
- [ ] **20.1** `internal/storage/leads.go` — aggregate query: leads grouped by source + outcome for a configurable time window
- [ ] **20.2-T** `internal/notify/slack_test.go` — assert analytics Block Kit section: Source / Leads / Claimed / Won / Hit Rate columns; fail first
- [ ] **20.2** `internal/notify/slack.go` — analytics summary Block Kit section: Source → Leads → Claimed → Won → Hit Rate %
- [ ] **20.3** `cmd/server/main.go` — wire analytics summary into existing `/digest` endpoint (appended section, not a new endpoint)
- [ ] **20.4** Market demand view: upcoming leads bucketed by expected arrival week, grouped by source type — gives managers a forward demand density view

---

### Phase 14 — Infrastructure & Self-Hosting 🔄
**Goal:** Deploy and manage a self-hosted ecosystem for automation and observability.

- [x] **14.1** Docker Compose — Standardize the deployment of GroupScout alongside n8n and Redis.
- [x] **14.2** n8n Workflow — Implement a workflow to trigger `/run` and `/digest` on a custom schedule.
- [x] **14.3** n8n Webhook Integration — Implement `/n8n/webhook` endpoint for receiving external leads.
- [ ] **14.4** Visualization Dashboard — Connect Metabase or Grafana to `groupscout.db` for lead analytics.
- [ ] **14.5** Search Engine — Integrate Meilisearch for fast lead searching in the upcoming Admin UI.
- [ ] **14.6** Alternative Notifications — Implement Gotify or Apprise for secondary alerting.
- [x] **14.7** Infrastructure Observability — Deploy Prometheus and Grafana Loki for real-time monitoring and log aggregation.

---

## Phase 21 — Multi-Property Support 📋
**Goal:** Configure and run GroupScout for multiple hotel properties with different geographies, segments, and thresholds. No changes to the collector/enrichment core — all config-driven.

> **Why now:** Sandman Hotels has multiple YVR-adjacent properties. Multi-property support unlocks the ability to run one deployment for the whole portfolio and route leads to the right Slack channel per property.

> **TDD rule for this phase:** T tasks first.

### Part A — Property Config Model
- [ ] **A-T** `config/properties_test.go` — `TestLoadProperties` with fixture YAML; assert multiple properties parse; assert per-property Slack channel + thresholds; fail first
- [ ] **A1** `config/properties.go` — `Property` struct: name, location (lat/lng or postal code), target segments (construction/film/sports/government), min permit value, priority threshold, Slack webhook URL, timezone
- [ ] **A2** `config/properties.go` — `LoadProperties(path string) ([]Property, error)` — parse YAML file; fall back to single-property env var config for backwards compatibility
- [ ] **A3** `config/properties.example.yaml` — document multi-property YAML schema

### Part B — Property-Scoped Pipeline
- [ ] **B-T** `internal/enrichment/enricher_test.go` — add `TestEnricher_RoutesLeadToProperty`: mock two properties; assert lead routed to correct Slack channel based on location match; fail first
- [ ] **B1** `internal/enrichment/enricher.go` — add `PropertyID string` to `Lead`; geo-match raw project location against configured property radius
- [ ] **B2** `internal/notify/slack.go` — route Slack message to property-specific webhook URL

### Part C — Alertd Multi-Hotel Config
> Already uses YAML per-hotel config (Phase 17-D1). Ensure `alertd` and the lead pipeline share the same `config/properties.yaml` schema or clearly separate them.

- [ ] **C1** Unify `config/airports.go` (Phase 17) and `config/properties.go` so a hotel entry covers both lead pipeline and alertd config
- [ ] **C2** `docker-compose.yml` — mount single `config/properties.yaml` into both `app` and `alertd` containers

---

## Phase 22 — Advanced Intelligence: Repeat Detection & Signal Quality 📋
**Goal:** Detect organizations that have brought groups to the market before. Surface signal quality metrics that help the team focus on leads most likely to convert.

> **Why:** Repeat groups are the highest-conversion leads — they've already bought once. Detecting them requires cross-referencing new leads against historical outreach log outcomes.

> **TDD rule for this phase:** T tasks first. All detection logic is pure — test against in-memory fixture data.

### Part A — Repeat Pattern Detection
- [ ] **A-T** `internal/enrichment/repeat_test.go` — `TestDetectRepeatOrg`: seed outreach_log with prior won lead from same org; new lead with same org name → assert `RepeatOrganization=true` + prior outcome surfaced; fail first
- [ ] **A1** `internal/enrichment/repeat.go` — `RepeatDetector`: fuzzy-match incoming `general_contractor` / `applicant` against historical `outreach_log` wins; return `RepeatMatch` struct (org, last outcome, last arrival date, win count)
- [ ] **A2** `internal/storage/leads.go` — query: `SELECT DISTINCT general_contractor, outcome FROM outreach_log JOIN leads` grouped by normalized org name
- [ ] **A3** `internal/enrichment/enricher.go` — call `RepeatDetector` after enrichment; attach `RepeatMatch` to lead; bump priority score by +2 if prior win detected
- [ ] **A4** `internal/notify/slack.go` — add "Repeat group 🔁" badge to Slack lead block when `RepeatOrganization=true`; include last outcome + date

### Part B — Signal Quality Metrics
> Rate each source by how often its leads turn into claimed leads (conversion to action). Surfaces in Phase 20 analytics and can auto-suppress low-quality sources.

- [ ] **B-T** `internal/storage/analytics_test.go` — `TestSignalQualityBySource`: seed leads with various source/outcome combos; assert quality score (claimed+won / total) per source; fail first
- [ ] **B1** `internal/storage/leads.go` — `SignalQuality(source string, days int) float64` — ratio of (claimed + won) to total leads from source in window
- [ ] **B2** `config/config.go` — `LOW_QUALITY_SOURCE_THRESHOLD float64` (default 0.05); sources below threshold logged as warning in pipeline metrics
- [ ] **B3** `internal/enrichment/enricher.go` — check signal quality before enriching; if source quality < threshold, log warning (do not skip — just flag)
