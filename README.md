# groupscout

`groupscout` is a lead generation and market intelligence platform for hotel sales teams. It monitors public data sources (permits, film productions, conferences, and procurement bids) to identify high-value group lodging opportunities before they reach traditional market reports.

### 🚀 Core Features

*   **Multi-Source Scrapers:**
    *   **Richmond & Delta Building Permits:** Weekly PDF scraping for large-scale construction and industrial projects.
    *   **Creative BC "In Production" List:** Monitors film and TV productions currently filming or in pre-production.
    *   **Vancouver Convention Centre (VCC):** Scrapes the event calendar for professional conferences and trade shows.
    *   **CivicInfo BC (BC Bid):** Automated RSS monitoring for construction-related government contract awards.
    *   **Infrastructure Announcements:** Monitors major project updates from BCIB, TransLink, and YVR Newsroom.
    *   **Professional Events:** Scrapes Eventbrite for conferences and industry summits in Vancouver.
*   **Intelligent Pre-Scoring:** A rules-based Go engine filters out low-value leads (residential renovations, small repairs) to save on API costs.
*   **AI Enrichment:** High-potential leads are enriched via the **Anthropic Claude API** to estimate room night potential, project duration, and lodging requirements.
*   **Automated Outreach:** Generates personalized cold email drafts for each lead using AI.
*   **Airport Disruption Alert System (`alertd`):** A real-time binary that monitors YVR flight disruptions, weather alerts (ECCC), and NOTAMs (NavCanada) to compute a **Stranded Passenger Score (SPS)** and alert hotel teams via Slack.
*   **Real-time Notifications:** Delivers formatted Block Kit messages directly to **Slack**.
*   **Weekly Digest:** Sends a formatted HTML email digest of the week's best leads via **Resend**.
*   **Secure API Trigger:** Can be integrated with automation tools like **n8n** via a protected HTTP endpoint.

### 🛠 Tech Stack

*   **Go (Golang):** Core application logic and concurrent scrapers.
*   **Database:** Dual-driver support for **PostgreSQL** (via `pgx/v5`) and **SQLite** (local persistent storage). Includes a migration script for one-way transfers from SQLite to Postgres.
*   **Vector Search:** Native **pgvector** support in Postgres and a Go-native cosine similarity fallback for SQLite.
*   **Sentry:** Production-grade error monitoring and real-time alerting.
*   **pdftotext:** Used for high-accuracy PDF parsing (via Poppler or Git for Windows).
*   **Ollama:** Local LLM runtime for privacy-preserving lead extraction and scoring rationale.
*   **Anthropic Claude API:** Advanced project analysis and room night estimation.
*   **Resend:** Delivery of weekly HTML email digests.
*   **Slack Webhooks:** Real-time delivery of prioritized leads.

### 🏗 Setup & Installation

1.  **Install Prerequisites:**
    *   [Go 1.26+](https://go.dev/dl/)
    *   [Docker & Docker Compose](https://docs.docker.com/get-docker/) (Optional, for simplified deployment)
    *   [pdftotext](https://www.xpdfreader.com/pdftotext-man.html) (Included with Git for Windows at `C:\Program Files\Git\mingw64\bin\pdftotext.exe`)

2.  **Clone the Repository:**
    ```bash
    git clone https://github.com/alvindcastro/groupscout.git
    cd groupscout
    ```

3.  **Configure Environment Variables:**
    Create a `.env` file in the root directory. You **must** define an `API_TOKEN` (a secret string of your choice) to secure the API.
    *   **To generate a secure token**: Run `go run -e "import 'crypto/rand'; import 'encoding/hex'; func main() { b := make([]byte, 32); rand.Read(b); println(hex.EncodeToString(b)) }"` or `openssl rand -hex 32`.
    *   Set it in `.env`: `API_TOKEN=your_generated_token_here`.

4.  **Install Dependencies:**
    ```bash
    go mod download
    ```

### 🐳 Docker Deployment (Recommended)

GroupScout includes a `docker-compose.yml` that starts the app along with **n8n** (automation), **Prometheus/Grafana** (monitoring), and **Loki** (logging).

```bash
# Define your keys in .env first, then:
docker compose up -d
```

*   **GroupScout API**: `http://localhost:8080`
*   **n8n Dashboard**: `http://localhost:5678`
*   **Grafana Dashboard**: `http://localhost:3000`

---

### 📋 Sample `.env` File Content

```env
# --- REQUIRED ---
CLAUDE_API_KEY=your_anthropic_api_key_here
SLACK_WEBHOOK_URL=https://hooks.slack.com/services/YOUR/WEBHOOK/URL
RESEND_API_KEY=re_your_resend_key_here
API_TOKEN=a_secure_random_string_for_n8n_authentication

# --- OBSERVABILITY ---
SENTRY_DSN=https://your_sentry_dsn_here
JSON_LOG=true

# --- APP SETTINGS ---
PORT=8080
DATABASE_URL=groupscout.db
# For Postgres:
# DATABASE_URL=postgres://groupscout:groupscout@localhost:5432/groupscout
ENRICHMENT_ENABLED=true
ENRICHMENT_THRESHOLD=1
PRIORITY_ALERT_THRESHOLD=9
MIN_PERMIT_VALUE_CAD=500000
PII_STRIP=true             # Redact PII from raw audit trail

# --- COLLECTOR TOGGLES ---
VCC_ENABLED=true
BCBID_ENABLED=true
CREATIVEBC_ENABLED=true
NEWS_ENABLED=true
ANNOUNCEMENTS_ENABLED=true
EVENTBRITE_ENABLED=true

# --- SOURCE URLS (Optional Overrides) ---
RICHMOND_PERMITS_URL=https://www.richmond.ca/shared/assets/Building_Permit_Reports_Current_Year57037.pdf
DELTA_PERMITS_URL=https://www.delta.ca/sites/default/files/2024-03/Building%20Permit%20Report%20-%20Current.pdf
VCC_URL=https://www.vancouverconventioncentre.com/events
BCBID_RSS_URL=https://www.civicinfo.bc.ca/rss/bids-bt.php?id=14,https://www.civicinfo.bc.ca/rss/bids-bt.php?id=53
EVENTBRITE_URL=https://www.eventbrite.ca/d/canada--vancouver/professional-services--events/

# --- OLLAMA ---
OLLAMA_ENABLED=true
OLLAMA_ENDPOINT=http://localhost:11434
OLLAMA_MODEL=mistral
OLLAMA_EXTRACTION_ENABLED=true
OLLAMA_SCORING_ENABLED=true
OLLAMA_ALERT_COPY_ENABLED=true
```

### 🏃 How to Run

The application operates in two modes:

#### 1. Server Mode (Default)
Runs a persistent HTTP server that listens for remote triggers (ideal for n8n/cron automation).

**Endpoints:**
- `GET /health`: Health check.
- `POST /run`: Trigger the full collect→enrich→notify pipeline.
- `POST /digest?to=email@example.com`: Send a weekly summary digest.
- `POST /n8n/webhook`: Receive a lead manually from external automation.

See [swagger.yaml](./api/swagger.yaml) for the full OpenAPI specification.

```bash
go run cmd/server/main.go
```
*   **Trigger via API:** Send a `POST` request to `http://localhost:8080/run` with `Authorization: Bearer YOUR_API_TOKEN`.

#### 2. CLI Mode (Run Once)
Executes the full pipeline once and exits immediately.
```bash
go run cmd/server/main.go --run-once
```

#### 3. Docker Mode
Run the entire stack (app, database, monitoring) using Docker Compose:
```bash
docker compose up -d
```

#### 4. AI Quality EvalOps
Run the deterministic GroupScout quality evals and release gate without live provider keys:
```bash
make eval-quality
make eval-gate
```

To drive the same cases through Promptfoo, run the local Go target first:
```bash
make eval-target
promptfoo eval -c evals/promptfoo/groupscout.yaml
```

GQ5 review samples can be converted into review-only draft cases with the evalops draft-case helper; generated drafts keep `TODO_REVIEW_*` expected fields and must be reviewed before they move into `data/evals/groupscout/`.

### 📄 Documentation

*   [DEVELOPER.md](./docs/DEVELOPER.md) - Developer's guide for running and testing the system.
*   [PHASES.md](./docs/planning/PHASES.md) - Living task tracker and build progress.
*   [CHANGELOG.md](./docs/CHANGELOG.md) - Plain-English record of all changes.
*   [ARCHITECTURE.md](./docs/ARCHITECTURE.md) - System design and data flow.
*   [SETUP.md](./docs/guides/SETUP.md) - Installation and configuration guide.
*   [DOCKER.md](./docs/guides/DOCKER.md) - Running and troubleshooting Docker.
*   [TESTING.md](./docs/guides/TESTING.md) - Comprehensive testing guide (unit, integration, Ollama).
*   [VERIFICATION.md](./docs/guides/VERIFICATION.md) - Runtime and EvalOps verification checklist.
*   [N8N_GUIDE.md](./docs/guides/N8N_GUIDE.md) - Workflow automation and scheduling.
*   [HOME_DEPLOY.md](./docs/guides/HOME_DEPLOY.md) - Self-hosting and deployment guide.
*   [OLLAMA_SETUP.md](./docs/guides/OLLAMA_SETUP.md) - Local LLM setup guide.
*   [ALERTD_SETUP.md](./docs/guides/ALERTD_SETUP.md) - Airport disruption system configuration.
*   [TROUBLESHOOTING.md](./docs/guides/TROUBLESHOOTING.md) - Pipeline and missing lead troubleshooting.
*   [API_CONFIG.md](./docs/API_CONFIG.md) - Endpoint reference and configuration.
*   [TESTING.md](./docs/guides/TESTING.md#8-api-testing-details) - How to test the APIs with curl, Bruno, or Swagger.
*   [ROADMAP.md](./docs/planning/ROADMAP.md) - Long-term project roadmap.
*   [UI_STRATEGY.md](./docs/planning/ui/UI_STRATEGY.md) - Operator UI strategy, information architecture, and UI API plan.
*   [UI_DESIGN_SYSTEM.md](./docs/planning/ui/UI_DESIGN_SYSTEM.md) - Operator UI visual rules and design-system adaptation.
*   [UI_PHASE31_DESIGN_SYSTEM_CONTRACT.md](./docs/planning/ui/UI_PHASE31_DESIGN_SYSTEM_CONTRACT.md) - Phase 31 token, primitive, component-test, accessibility, secret-safety, and no-hero contract.
*   [UI_PHASE32_APP_SHELL_ROUTING_CONTRACT.md](./docs/planning/ui/UI_PHASE32_APP_SHELL_ROUTING_CONTRACT.md) - Phase 32 operator shell, routes, responsive behavior, and build/component-test contract.
*   [UI_PHASE33_MOCKED_LEAD_INBOX_CONTRACT.md](./docs/planning/ui/UI_PHASE33_MOCKED_LEAD_INBOX_CONTRACT.md) - Phase 33 mocked lead inbox fixture, table, interaction, accessibility, and API-boundary contract.
*   [UI_PHASE34_LEAD_DETAIL_EVIDENCE_CONTRACT.md](./docs/planning/ui/UI_PHASE34_LEAD_DETAIL_EVIDENCE_CONTRACT.md) - Phase 34 lead detail, evidence review, raw audit safety, outreach/activity, status action, and correction-control contract.
*   [UI_PHASE35_API_CONTRACT.md](./docs/planning/ui/UI_PHASE35_API_CONTRACT.md) - Phase 35 implemented `/api/leads` list, detail, patch, raw evidence, and schema-gap contract.
*   [UI_PHASE35_FRONTEND_TYPES.md](./docs/planning/ui/UI_PHASE35_FRONTEND_TYPES.md) - Temporary frontend type contract for the Phase 35 UI API responses.
*   [UI_PHASE36_OUTREACH_STATE_CONTRACT.md](./docs/planning/ui/UI_PHASE36_OUTREACH_STATE_CONTRACT.md) - Phase 36 outreach logging and lead state action contract.
*   [UI_PHASE37_PIPELINE_STATS_SYSTEM_CONTRACT.md](./docs/planning/ui/UI_PHASE37_PIPELINE_STATS_SYSTEM_CONTRACT.md) - Phase 37 pipeline run, stats, and system API contract.
*   [UI_PHASE38_DOCKER_SMOKE_CONTRACT.md](./docs/planning/ui/UI_PHASE38_DOCKER_SMOKE_CONTRACT.md) - Phase 38 backend plus external UI Docker smoke contract.
*   [UI_TDD_PHASE_PLAN.md](./docs/planning/ui/UI_TDD_PHASE_PLAN.md) - Strict TDD phase checklist for operator UI and `/api/*` work.
*   [PROMPTS_PHASE31_UI.md](./docs/prompts/PROMPTS_PHASE31_UI.md) - Copy-paste prompts for future UI and UI API TDD implementation tasks.
*   [PROMPTS_PHASE32_UI.md](./docs/prompts/PROMPTS_PHASE32_UI.md) - Copy-paste prompts for Phase 32 route, app shell, responsive layout, build, and component-test implementation.
*   [PROMPTS_PHASE33_UI.md](./docs/prompts/PROMPTS_PHASE33_UI.md) - Copy-paste prompts for Phase 33 mocked lead inbox implementation.
*   [PROMPTS_PHASE34_UI.md](./docs/prompts/PROMPTS_PHASE34_UI.md) - Copy-paste prompts for Phase 34 lead detail and evidence review implementation.
*   [PROMPTS_PHASE35_UI.md](./docs/prompts/PROMPTS_PHASE35_UI.md) - Copy-paste prompts for Phase 35 UI API contract refinement.
*   [BACKEND_FOR_UI_TESTING.md](./docs/planning/ui/BACKEND_FOR_UI_TESTING.md) - Backend startup paths and API status for UI testing.
*   [AI_QUALITY_EVALOPS.md](./docs/planning/AI_QUALITY_EVALOPS.md) - EvalOps plan for AI enrichment, scoring, outreach, and alert quality.
*   [groupscout-case-schema.md](./docs/evals/groupscout-case-schema.md) - GQ1 schema for reusable JSONL eval fixtures in `data/evals/groupscout/`.
*   [TDD_AI_QUALITY.md](./docs/guides/TDD_AI_QUALITY.md) - Strict TDD policy for AI quality work.
*   [PROMPTS_AI_QUALITY.md](./docs/prompts/PROMPTS_AI_QUALITY.md) - Task prompts for AI quality implementation phases.
*   [evals/promptfoo/groupscout.yaml](./evals/promptfoo/groupscout.yaml) - GQ3 Promptfoo config for the local Go eval target.
