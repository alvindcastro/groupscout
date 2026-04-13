### Architecture Overview

GroupScout is a lead generation and automation platform designed to identify, enrich, and notify users about high-value business opportunities (leads) from various public and private data sources. The system follows a modular, pipeline-based architecture implemented in Go, often deployed alongside **n8n** for extended automation.

---

### System Components

The application is structured into four primary layers, plus an optional external automation layer:

#### 0. External Automation (Optional — n8n)
n8n can be used to:
- **Trigger**: Call the `/run` or `/digest` endpoints on a custom schedule.
- **Active Collect**: Scrape complex or authenticated sites and push the resulting JSON to the `/n8n/webhook` endpoint.
- **Workflow**: Route leads to other CRM tools (HubSpot, Salesforce) or specialized notification channels.

#### 1. Collector Layer (`internal/collector`)
Responsible for gathering raw data from external sources. Each data source implements the `Collector` interface:
- **BC Bid**: Scrapes government tender and bid opportunities.
- **Permit Portals**: Monitors municipal permit data (e.g., Richmond, Delta).
- **Eventbrite**: Tracks industry-relevant events.
- **News/RSS**: Monitors news feeds for construction and infrastructure keywords.
- **Standardized Output**: Every collector produces `RawProject` structs, normalizing diverse data into a common format before storage.

#### 2. Storage Layer (`internal/storage`)
Handles data persistence and deduplication with dual-driver support (**PostgreSQL** and **SQLite**).
- **Driver Selection**: Automatically switches between `pgx` (Postgres) and `sqlite` based on the `DATABASE_URL` prefix.
- **Migrations**: Uses `golang-migrate` for versioned Postgres schema updates, while maintaining an idempotent inline schema for SQLite.
- **SQL Rebinding**: Dynamically converts standard `?` placeholders to driver-specific formats (e.g., `$1`, `$2` for Postgres).
- **`raw_projects`**: Stores the original payload from collectors to ensure data lineage and prevent re-processing of the same items (via SHA-256 hashing).
- **`leads`**: Stores enriched business opportunities with scoring, contact details, and status tracking.
- **`outreach_log`**: Records interactions with leads (emails, calls, LinkedIn) for CRM-like functionality.
- **`lead_embeddings`**: Stores vector embeddings of leads for similarity-based retrieval (RAG). Uses **pgvector** on Postgres and a custom Go-native implementation for SQLite.

#### 3. Enrichment & Scoping Layer (`internal/enrichment`)
Responsible for identifying high-value leads and preparing them for outreach.
- **LLM Providers**: `ClaudeClient` (Anthropic Messages API) and `GeminiClient` (Google Gemini API) both implement the `LLMClient` interface. Provider selected via `LLM_PROVIDER` env var. Phase 16 adds OpenAI-compatible client (OpenAI, Groq, Mistral, Azure, Ollama) and a `FallbackClient`.
- **Pre-Scorer**: Applies rule-based Go logic to filter out low-value projects (e.g., small renovations) before calling the AI API, significantly reducing costs. `BudgetTier()` helper classifies project spend (Phase 18).
- **Contact Enrichment**: `hunter.go` looks up decision-maker contacts via Hunter.io (Phase 18). `repeat.go` flags organizations with prior winning outreach history (Phase 22).
- **Deduplication**: Ensures that only new, unique projects are sent for AI enrichment via SHA-256 hash checks.
- **Outreach Drafting**: Generates personalized cold email drafts for each lead using the active LLM provider.

#### 4. Notification & Observability Layer (`internal/leadnotify`)
Dispatches alerts and monitors system health.
- **Slack**: Sends real-time alerts and lead digests to configured webhooks. Phase 19 adds interactive action buttons (Claim/Dismiss/Snooze). Phase 20 appends analytics summary to the digest. Multi-property support (Phase 21) routes messages to property-specific webhooks.
- **Email**: Sends weekly HTML summaries and outreach drafts via Resend.
- **Sentry**: Captures and reports runtime errors and pipeline failures.
- **Health Check**: Exposes a `/health` endpoint to verify database and API connectivity.
- **Monitoring**: Integrates with Prometheus and Grafana Loki (via Docker) for infrastructure-level observability.

#### 5. Disruption Alert System (`cmd/alertd/` — Phase 17)
A **separate long-running binary** — distinct from the lead pipeline, different cadence and failure modes.
- **Weather**: `internal/weather/eccc.go` polls `api.weather.gc.ca` (no key required) for Metro Vancouver + Fraser Valley alert zones; classifies type (atmospheric river, fog, snow) and severity.
- **Aviation**: `internal/aviation/yvr.go` scrapes live YVR cancellation rate; `navcanada.go` parses NOTAMs for ground stop data (leads cancellations by 1–2 hrs).
- **Scoring**: `internal/aviation/scorer.go` computes the Stranded Passenger Score (SPS) using `(cancelled/scheduled) × avg_seats × connecting_pax_ratio × time_of_day_multiplier × duration_score`. Vancouver-tuned thresholds: fog → weight duration; snow → pre-alert before cancellations.
- **Alert lifecycle**: `internal/alert/lifecycle.go` state machine: Watch → Alert → Update → Resolve. Updates the *same* Slack message via `chat.update` — no channel spam. No hard alert fires until the event has been active for ≥ 30 minutes. Channel routing: HardAlert → #ops-urgent, SoftAlert/Watch → #ops-monitoring.
- **Hotel config**: `config/airports.go` (or shared `config/properties.yaml` in Phase 21): per-property airport mappings, SPS thresholds, distressed rate, airline ops contacts.

---

### Data Flow

GroupScout operates two distinct data lifecycles: the **Leads Pipeline** (collect-enrich-notify) and the **Disruption Alert System** (long-running poller).

For a detailed end-to-end breakdown of these processes, including architecture diagrams, see:
👉 **[Detailed Data Flow Documentation](./DATA_FLOW.md)**

---

### Technology Stack

-   **Language**: Go (Golang)
-   **Database**: PostgreSQL (with `pgvector`) and SQLite (local-first, easily portable). Includes a one-way migration script (`cmd/tools/migrate_db/main.go`).
-   **AI / LLM**: Claude (Anthropic Messages API) + Gemini (Google) — both implemented. Phase 16 adds OpenAI-compatible client (OpenAI, Groq, Mistral, Azure, Ollama) and `FallbackClient`.
-   **Integrations**: Slack Webhooks + Interactive Actions, Resend API, Hunter.io (Phase 18), Sentry, **n8n**, Prometheus, Grafana Loki
-   **Configuration**: Environment variables (supporting `.env` files); `config/properties.yaml` for multi-property (Phase 21)
-   **Observability**: Structured JSON logging (slog), Sentry Error Tracking

---

### Directory Structure

-   `cmd/server/`: Main lead pipeline entry point (HTTP server + `--run-once` mode).
-   `cmd/alertd/`: Airport disruption alert daemon — separate binary (Phase 17).
-   `cmd/tools/smoketest/`: Dev tool for live collector runs and Slack testing (moved from `cmd/smoketest`).
-   `api/`: OpenAPI specification (`swagger.yaml`).
-   `internal/collector/`: Source-specific data fetching grouped into `permits`, `events`, and `news` sub-packages.
-   `docs/`: Structured project documentation (`guides`, `planning`, `prompts`).
-   `config/`: Configuration loading, environment management.
-   `internal/leadnotify/`: Slack (lead alerts + interactive actions + digest), email.
-   `internal/weather/`: ECCC weather alert poller and classifier (Phase 17).
-   `internal/aviation/`: YVR flight scraper, NavCanada NOTAM parser, SPS scorer (Phase 17).
-   `internal/alert/`: Disruption alert lifecycle state machine + Slack updater (Phase 17).
-   `migrations/`: SQL migration files.
-   `scripts/`: Shell utility scripts (e.g., volume backups, environment health checks).
-   `cmd/tools/`: Internal maintenance tools (e.g., database migration, clearing, checking).

---

### Running Modes

-   **Server Mode**: Runs as a long-lived process listening on a port (default: 8080), providing an API for external triggers (e.g., n8n, Zapier).
    -   See [swagger.yaml](./api/swagger.yaml) for API documentation.
-   **Run-Once Mode**: Executes the pipeline once and exits (`--run-once`). Ideal for simple cron jobs.
