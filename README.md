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
*   **Airport Disruption Alert System (`alertd`):** A separate real-time binary that monitors YVR flight disruptions, weather alerts (ECCC), and NOTAMs (NavCanada) to compute a **Stranded Passenger Score (SPS)** and alert hotel teams via Slack before passengers arrive. (Phase 17)
*   **Real-time Notifications:** Delivers formatted Block Kit messages directly to **Slack**.
*   **Weekly Digest:** Sends a formatted HTML email digest of the week's best leads via **SendGrid**.
*   **Secure API Trigger:** Can be integrated with automation tools like **n8n** via a protected HTTP endpoint.

### 🛠 Tech Stack

*   **Go (Golang):** Core application logic and concurrent scrapers.
*   **Database:** Dual-driver support for **PostgreSQL** (via `pgx/v5`) and **SQLite** (local persistent storage). Includes a migration script for one-way transfers from SQLite to Postgres.
*   **Vector Search:** Native **pgvector** support in Postgres and a Go-native cosine similarity fallback for SQLite.
*   **Sentry:** Production-grade error monitoring and real-time alerting.
*   **pdftotext:** Used for high-accuracy PDF parsing (via Poppler or Git for Windows).
*   **Ollama:** Local LLM runtime for privacy-preserving lead extraction and scoring rationale.
*   **Anthropic Claude API:** Advanced project analysis and room night estimation.
*   **SendGrid:** Delivery of weekly HTML email digests.
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
SENDGRID_API_KEY=your_sendgrid_api_key_here
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

#### 3. Docker Mode
Run the entire stack (app, database, monitoring) using Docker Compose:
```bash
docker compose up -d
```

#### 2. CLI Mode (Run Once)
Executes the full pipeline once and exits immediately.
```bash
go run cmd/server/main.go --run-once
```

### 📄 Documentation
 
*   [DEVELOPER.md](./DEVELOPER.md) - Developer's guide for running and testing the system.
*   [DOCKER.md](./docs/guides/DOCKER.md) - Running and troubleshooting Docker.
*   [ROADMAP.md](./docs/planning/ROADMAP.md) - Project roadmap and development progress.
*   [ARCHITECTURE.md](./docs/ARCHITECTURE.md) - System design and data flow.
*   [SETUP.md](./docs/guides/SETUP.md) - Installation and configuration guide.
*   [OLLAMA_INTEGRATION.md](./docs/planning/OLLAMA_INTEGRATION.md) - Local LLM integration plan and phases.
*   [OLLAMA_SETUP.md](./docs/guides/OLLAMA_SETUP.md) - Docker and native setup guide for Ollama.
*   [groupscout-build-log.md](./docs/prompts/groupscout-build-log.md) - Developer's narrative and blog-style build notes.
