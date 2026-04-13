# Developer Guide — GroupScout

This guide provides technical details for developers working on the `groupscout` project, including the main lead generation server and the `alertd` airport disruption monitor.

## 🛠 Project Management (Makefile)

The `Makefile` is the central hub for common development tasks. Run `make help` to see all available commands.

### Common Tasks:
- `make build`: Build `server` and `alertd` binaries.
- `make test`: Run all Go tests.
- `make run`: Run the lead generation server.
- `make docker-up`: Start the full Docker stack.
- `make clean`: Remove build artifacts.

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

### 3. Ollama Modelfile Management
GroupScout uses local Ollama models for enrichment and alert copy. Modelfiles define the personas and instructions for these models.

**Push Modelfiles:**
To update or create the personas on your local Ollama server without restarting the main server:
```bash
go run cmd/server/main.go ollama push-models
```
This reads all `.modelfile` files from `internal/ollama/modelfile/` and pushes them to the Ollama endpoint.

**List Models:**
To see which models are currently loaded in your Ollama instance:
```bash
go run cmd/server/main.go ollama list-models
```

**Workflow for Updating Persona Prompts:**
1. Edit the relevant `.modelfile` in `internal/ollama/modelfile/`.
2. Run `go run cmd/server/main.go ollama push-models`.
3. The new persona is immediately available for the next LLM call.

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

We use a combination of unit tests, integration tests, and manual verification scripts.

### 1. Automated Tests
```bash
make test
```
See [docs/guides/TESTING.md](./docs/guides/TESTING.md) for details on running specific tests and Postgres integration.

### 2. Ollama & LLM Testing
A dedicated script verifies Ollama connectivity and model availability:
```bash
./scripts/test-ollama.sh
```

### 3. API Testing
Manual API testing can be done via `curl` or the **Bruno** collection in `api/bruno`.
See [docs/API_TESTING.md](./docs/API_TESTING.md) for examples.

## 📂 Project Structure
- `api/`: OpenAPI / Swagger specifications.
- `cmd/`: Entry points for `server`, `alertd`, and dev tools.
- `config/`: Centralized environment and YAML configuration.
- `.junie/agents/`: Definitions for specialized Junie subagents.
- `docs/`: In-depth guides and planning documents.
- `internal/`: Core business logic (scrapers, scoring, state machines, storage).
- `migrations/`: SQL migration files (for Postgres/SQLite).

## 🤖 Junie Subagents

This project uses specialized Junie subagents to streamline development in different domains. These agents are defined in `.junie/agents/` and can be used to handle specific tasks with tailored instructions.

See [docs/guides/SUBAGENTS.md](./docs/guides/SUBAGENTS.md) for a list of available agents and how to use them.

## 📄 Related Documentation
- [README.md](./README.md) - Project overview and user setup.
- [ARCHITECTURE.md](./docs/ARCHITECTURE.md) - System design and data flow.
- [CHANGELOG.md](./docs/CHANGELOG.md) - Plain-English record of all changes.
- [DOCKER.md](./docs/guides/DOCKER.md) - Running and troubleshooting Docker.
- [SUBAGENTS.md](./docs/guides/SUBAGENTS.md) - How to use specialized Junie agents.
- [TROUBLESHOOTING.md](./docs/guides/TROUBLESHOOTING.md) - Pipeline and missing lead troubleshooting.
- [SETUP.md](./docs/guides/SETUP.md) - Detailed environment and dependency setup.
- [TESTING.md](./docs/guides/TESTING.md) - Comprehensive testing guide.
- [N8N_GUIDE.md](./docs/guides/N8N_GUIDE.md) - Workflow automation and scheduling.
- [HOME_DEPLOY.md](./docs/guides/HOME_DEPLOY.md) - Self-hosting and deployment guide.
- [API_CONFIG.md](./docs/API_CONFIG.md) - Detailed API and endpoint configuration.
- [API_TESTING.md](./docs/API_TESTING.md) - Guide on how to test the APIs.
- [ALERTD_SETUP.md](./docs/guides/ALERTD_SETUP.md) - Specific configuration for the alert system.
- [OLLAMA_INTEGRATION.md](./docs/planning/OLLAMA_INTEGRATION.md) - Local LLM integration plan and phases.
- [OLLAMA_SETUP.md](./docs/guides/OLLAMA_SETUP.md) - Docker and native setup guide for Ollama.
- [PHASES.md](./docs/planning/PHASES.md) - Build tracker and phase history.
- [ROADMAP.md](./docs/planning/ROADMAP.md) - Long-term project roadmap.
- [AUDIT_TRAIL.md](./docs/planning/AUDIT_TRAIL.md) - Input storage and verification plan.
- [PROMPTS.md](./docs/planning/PROMPTS.md) - Prompt library and Strict TDD guide.
