# GroupScout Backend

GroupScout is the backend lead intelligence engine for hotel sales teams. It collects public market signals, turns them into normalized lead records, enriches promising opportunities with AI, and exposes API surfaces used by automation, Slack/email notifications, and the operator UI.

Long-form project documentation lives in the coordination repo:

- Coordination repo: `/mnt/c/Users/alvin/groupscout-site`
- Backend docs: `/mnt/c/Users/alvin/groupscout-site/backend`
- Frontend docs: `/mnt/c/Users/alvin/groupscout-site/frontend`
- Frontend app repo: `/mnt/c/Users/alvin/WebstormProjects/groupscout-ui`

Keep this repository focused on source, tests, runtime configuration, and this README. Add durable planning docs, phase notes, prompt packs, and architecture writeups to `groupscout-site`.

## What This Service Does

The backend runs two related products:

- Lead pipeline: collects permits, events, bids, news, and infrastructure signals; deduplicates raw projects; pre-scores them; enriches high-value leads; stores evidence; and notifies operators.
- Airport disruption alerting: runs `alertd`, a separate long-running service that monitors weather and aviation signals and computes stranded-passenger risk.

The main operational boundary is HTTP:

- `GET /health` checks service health.
- `POST /run` triggers the collect -> enrich -> notify pipeline.
- `POST /digest` sends a summary digest.
- `POST /n8n/webhook` receives externally collected leads.
- `/api/*` endpoints support the same-origin operator UI.

## Repository Map

- `cmd/server`: main API server and run-once entry point.
- `cmd/alertd`: airport disruption alert daemon.
- `cmd/evalquality`, `cmd/evalgate`, `cmd/evaltarget`: AI quality and release-gate tools.
- `internal/collector`: source-specific collectors for permits, bids, events, and news.
- `internal/storage`: persistence, migrations, audit records, lead state, stats, and pipeline run storage.
- `internal/enrichment`: scoring, extraction, LLM clients, and outreach draft generation.
- `internal/evalops`: deterministic evaluation cases, scoring, reports, and gates.
- `internal/leadnotify`: Slack and email delivery.
- `internal/aviation`, `internal/weather`, `internal/alert`: disruption signal collection, scoring, and alert lifecycle.
- `config`: runtime and airport/property configuration.
- `migrations`: PostgreSQL schema migrations.
- `api`: OpenAPI and Bruno API collections.
- `scripts`: local health, smoke, and utility scripts.

## Prerequisites

- Go 1.26 or newer.
- Docker and Docker Compose for the full local stack.
- `pdftotext` for high-quality PDF parsing.
- Optional local Ollama runtime or provider API keys for enrichment.

The app can run with PostgreSQL in Docker or with a local SQLite database for lightweight development.

## Configuration

Start from `.env.example` and create a local `.env`.

Required for protected API operations:

```sh
API_TOKEN=replace-with-a-secret-token
```

Common optional settings:

```sh
DATABASE_URL=groupscout.db
PORT=8080
ENRICHMENT_ENABLED=true
OLLAMA_ENABLED=true
OLLAMA_ENDPOINT=http://localhost:11434
OLLAMA_MODEL=mistral
SLACK_WEBHOOK_URL=
RESEND_API_KEY=
CLAUDE_API_KEY=
```

Use a Postgres URL for the Docker or production database:

```sh
DATABASE_URL=postgres://groupscout:groupscout@localhost:5432/groupscout
```

Never commit real `.env` files or provider credentials.

## Local Development

Install dependencies:

```sh
go mod download
```

Run the API server:

```sh
make run
```

Run one pipeline pass and exit:

```sh
make run-once
```

Run the alert daemon:

```sh
make run-alertd
```

Build binaries:

```sh
make build
```

## Docker Stack

The Compose stack starts the backend, Postgres with pgvector, n8n, Grafana, Prometheus, Loki, Promtail, and Ollama.

```sh
docker compose up -d
```

Useful local URLs:

- Backend API: `http://localhost:8080`
- n8n: `http://localhost:5678`
- Grafana: `http://localhost:3000`
- Prometheus: `http://localhost:9090`

Shut the stack down:

```sh
docker compose down
```

## Testing And Quality Gates

Run the full Go suite:

```sh
make test
```

Format and vet:

```sh
make fmt
make vet
```

Run deterministic AI quality checks:

```sh
make eval-quality
make eval-gate
```

Run environment diagnostics:

```sh
make doctor
```

## Working With The UI

The UI lives in `/mnt/c/Users/alvin/WebstormProjects/groupscout-ui`. It expects browser traffic to stay on same-origin `/api/*` routes and must not expose backend automation tokens or provider credentials.

For backend plus UI smoke testing, start the backend Compose stack here and use the UI repo's `compose.dev.yml` overlay from the frontend repository.

## Documentation Policy

This repo should contain only source-adjacent files needed to build, test, and run the backend. Keep durable documentation in `/mnt/c/Users/alvin/groupscout-site/backend`, including architecture, data flow, setup guides, phase plans, prompt packs, and troubleshooting notes.

If a source change needs documentation, update this README only for developer-critical runbook information. Put deeper explanations and planning material in the coordination repo.
