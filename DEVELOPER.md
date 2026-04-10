# Developer Guide — GroupScout

This guide provides technical details for developers working on the `groupscout` project, including the main lead generation server and the `alertd` airport disruption monitor.

## 🏗 Project Architecture

GroupScout consists of two primary Go binaries:
1.  **Lead Generation Server (`cmd/server`)**: Scrapes building permits, film productions, and events to generate hotel group sales leads.
2.  **Airport Disruption Alert System (`alertd` / `cmd/alertd`)**: Monitors YVR flight disruptions, ECCC weather alerts, and NavCanada NOTAMs to alert hotel teams via Slack.

## 🚀 Getting Started

### Prerequisites
- **Go 1.26+**
- **pdftotext** (included with Git for Windows, or install via Poppler/XPDF on Linux/macOS)
- **Docker & Docker Compose** (optional, for Postgres/n8n/monitoring)

### Environment Variables
Create a `.env` file in the project root. Essential variables:
```env
CLAUDE_API_KEY=sk-ant-...        # Anthropic API key for AI enrichment
SLACK_BOT_TOKEN=xoxb-...          # Slack Bot Token (required for alertd)
SLACK_WEBHOOK_URL=https://...     # Slack Incoming Webhook (for lead digests)
DATABASE_URL=groupscout.db        # SQLite or Postgres connection string
ALERTD_PORT=8081                  # Port for alertd slash commands (default: 8081)
```

## 🏃 Running the Binaries

### 1. Lead Generation Server
**Server Mode (API triggered):**
```bash
go run cmd/server/main.go
```
**CLI Mode (One-time run):**
```bash
go run cmd/server/main.go --run-once
```

### 2. Alertd (Disruption Monitor)
**Run Alertd:**
```bash
go run cmd/alertd/main.go
```
Alertd requires a configuration file at `config/airports.yaml` to define hotels and their monitored airports.

## 🛠 New Features & How to Run

### `/inventory` Slack Slash Command (alertd)
`alertd` now includes an HTTP server that listens for Slack slash commands to update real-time room availability in alerts.

#### Local Development & Testing
To test Slack slash commands locally without a public URL:
1.  **Expose your local port** (e.g., using `ngrok`):
    ```bash
    ngrok http 8081
    ```
2.  **Configure Slack App**: Set the Request URL for the `/inventory` command to `https://<your-ngrok-url>/slack/inventory`.
3.  **Simulate with curl**:
    ```bash
    curl -X POST -d "command=/inventory&text=34" http://localhost:8081/slack/inventory
    ```

#### Verification
When `/inventory 34` is called, the current `alertd` instance updates its in-memory inventory. Subsequent Slack alerts will display "34 rooms available" instead of the "room count not set" fallback.

## 🧪 Testing

Run all tests:
```bash
go test ./...
```

Run specific tests for `alertd`:
```bash
go test ./cmd/alertd/... ./internal/alert/...
```

## 📂 Project Structure
- `api/`: OpenAPI / Swagger specifications.
- `cmd/`: Entry points for `server`, `alertd`, and dev tools.
- `config/`: Centralized environment and YAML configuration.
- `docs/`: In-depth guides and planning documents.
- `internal/`: Core business logic (scrapers, scoring, state machines, storage).
- `migrations/`: SQL migration files (for Postgres/SQLite).

## 📄 Related Documentation
- [README.md](./README.md) - Project overview and user setup.
- [DOCKER.md](./docs/guides/DOCKER.md) - Running and troubleshooting Docker.
- [SETUP.md](./docs/guides/SETUP.md) - Detailed environment and dependency setup.
- [ALERTD_SETUP.md](./docs/guides/ALERTD_SETUP.md) - Specific configuration for the alert system.
- [PHASES.md](./docs/planning/PHASES.md) - Build tracker and phase history.
