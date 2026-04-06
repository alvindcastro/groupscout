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
- **Claude AI**: Uses Anthropic's Claude API to parse unstructured text, estimate project values, crew sizes, and duration.
- **Pre-Scorer**: Applies rule-based Go logic to filter out low-value projects (e.g., small renovations) before calling the AI API, significantly reducing costs.
- **Deduplication**: Ensures that only new, unique projects are sent for AI enrichment via SHA-256 hash checks.
- **Outreach Drafting**: Generates personalized cold email drafts for each lead using Claude.

#### 4. Notification & Observability Layer (`internal/notify`)
Dispatches alerts and monitors system health.
- **Slack**: Sends real-time alerts and lead digests to configured webhooks.
- **Email**: Sends weekly HTML summaries and outreach drafts via SendGrid.
- **Sentry**: Captures and reports runtime errors and pipeline failures.
- **Health Check**: Exposes a `/health` endpoint to verify database and API connectivity.
- **Monitoring**: Integrates with Prometheus and Grafana Loki (via Docker) for infrastructure-level observability.

---

### Data Flow

1.  **Trigger**: The pipeline is triggered via a scheduled cron job, an HTTP POST request to the `/run` endpoint, or an external push to `/n8n/webhook`.
2.  **Collection**: Registered collectors fetch data and return `RawProject` objects.
3.  **Deduplication**: The system checks the `raw_projects` table for existing hashes.
4.  **Enrichment**: New projects are sent to Claude AI for analysis and scoring.
5.  **Persistence**: The resulting `Lead` is saved to the database.
6.  **Notification**: If the lead's score exceeds the `PriorityAlertThreshold`, an alert is dispatched to Slack/Email.

---

### Technology Stack

-   **Language**: Go (Golang)
-   **Database**: PostgreSQL (with `pgvector`) and SQLite (local-first, easily portable). Includes a one-way migration script (`scripts/migrate_to_postgres/main.go`).
-   **AI**: Anthropic Claude API (3.5 Sonnet/Haiku)
-   **Integrations**: Slack Webhooks, SendGrid API, Sentry, **n8n**, Prometheus, Grafana Loki
-   **Configuration**: Environment variables (supporting `.env` files)
-   **Observability**: Structured JSON logging (slog), Sentry Error Tracking

---

### Directory Structure

-   `cmd/server/`: Main application entry point (HTTP server and CLI).
-   `config/`: Configuration loading and environment management.
-   `internal/collector/`: Source-specific data fetching logic.
-   `internal/enrichment/`: AI orchestration and scoring logic.
-   `internal/storage/`: Database schema, migrations, and CRUD operations.
-   `internal/notify/`: Alerting and communication logic.
-   `migrations/`: SQL migration files (if versioned migrations are used).
-   `scripts/`: Utility scripts for database maintenance.

---

### Running Modes

-   **Server Mode**: Runs as a long-lived process listening on a port (default: 8080), providing an API for external triggers (e.g., n8n, Zapier).
    -   See [swagger.yaml](./swagger.yaml) for API documentation.
-   **Run-Once Mode**: Executes the pipeline once and exits (`--run-once`). Ideal for simple cron jobs.
