# GroupScout Backend

GroupScout backend is the Go service that collects public demand signals, enriches leads, stores audit data, and exposes the automation/API surface used by n8n, Slack, alertd, and the operator UI.

Long-lived documentation is centralized in the coordination docs repository. This source repo intentionally keeps only this README as Markdown so implementation work stays close to code while planning and runbooks stay in one place.

## Where To Find The Docs

- Coordination repo: `/mnt/c/Users/alvin/groupscout-site`
- Backend docs home: `/mnt/c/Users/alvin/groupscout-site/backend/README.md`
- Developer workflow: `/mnt/c/Users/alvin/groupscout-site/backend/docs/DEVELOPER.md`
- Setup guide: `/mnt/c/Users/alvin/groupscout-site/backend/docs/guides/SETUP.md`
- Testing guide: `/mnt/c/Users/alvin/groupscout-site/backend/docs/guides/TESTING.md`
- Verification guide: `/mnt/c/Users/alvin/groupscout-site/backend/docs/guides/VERIFICATION.md`
- Docker guide: `/mnt/c/Users/alvin/groupscout-site/backend/docs/guides/DOCKER.md`
- Architecture: `/mnt/c/Users/alvin/groupscout-site/backend/docs/ARCHITECTURE.md`
- API/config reference: `/mnt/c/Users/alvin/groupscout-site/backend/docs/API_CONFIG.md`
- Roadmap and active work: `/mnt/c/Users/alvin/groupscout-site/backend/docs/planning/ROADMAP.md`
- Not-done and upgrade snapshot: `/mnt/c/Users/alvin/groupscout-site/backend/docs/planning/NOT_DONE_AND_UPGRADES.md`

Start in `/mnt/c/Users/alvin/groupscout-site` for planning, Beads issues, and cross-repo coordination. Make backend code changes here only after the owning task is clear.

## What Runs Here

- Main API and pipeline server: `cmd/server/main.go`
- Alert daemon: `cmd/alertd/main.go`
- Collector, enrichment, storage, notification, and HTTP support packages under `internal/`
- Docker runtime for app, Postgres/pgvector, n8n, alerting, and observability through `docker-compose.yml`
- Environment template in `.env.example`

## Quick Start

Prerequisites:

- Go toolchain matching `go.mod`
- Docker with the Compose v2 plugin for the full stack
- A local `.env` copied from `.env.example`
- Optional Ollama models for local LLM paths

```sh
cp .env.example .env
make test
make run
```

The server listens on `PORT` from `.env` or `8080` by default. Set `API_TOKEN` for bearer-token protection before exposing the service beyond local development.

Run the full collect-enrich-notify pipeline once:

```sh
make run-once
```

Start the Docker stack:

```sh
make docker-up
make docker-logs
```

Stop the Docker stack:

```sh
make docker-down
```

## Common Developer Commands

```sh
make help          # list supported targets
make test          # go test -v ./...
make fmt           # go fmt ./...
make vet           # go vet ./...
make build         # build server and alertd into build/
make doctor        # run environment health checks
make docker-up     # start the Compose stack
make docker-down   # stop the Compose stack
make docker-logs   # follow Compose logs
make ollama-pull   # pull local LLM models
make ollama-push   # push local persona Modelfiles
make run-alertd    # run the alertd service locally
make start-fresh   # clear data, start Docker services, run one pipeline pass
```

Useful direct commands:

```sh
go run cmd/server/main.go
go run cmd/server/main.go --run-once
go run cmd/server/main.go audit-retention purge --days 30
go run cmd/server/main.go ollama push-models
go run cmd/server/main.go ollama list-models
go test ./cmd/server ./internal/storage ./internal/enrichment
```

Useful diagnostics and tools:

```sh
go run ./cmd/tools/smoketest
go run ./cmd/tools/smoketest -rawpdf
go run ./cmd/tools/smoketest -testslack
go run ./cmd/tools/test_n8n
go run ./cmd/tools/test_sentry
go run ./cmd/tools/migrate_db --sqlite groupscout.db --postgres "postgres://..." --dry-run
```

## Runtime Surface

Primary local ports and entrypoints:

- `cmd/server/main.go` serves the main API on `PORT`, default `8080`.
- `cmd/alertd/main.go` serves alertd on `ALERTD_PORT`, default `8081`.
- Docker Compose starts app, Postgres/pgvector, n8n, alerting, and observability services.

Common HTTP routes:

- `GET /health` for service/database/LLM health
- `POST /run` for collect-enrich-notify pipeline execution
- `POST /digest` for digest delivery
- `POST /ingest` and `POST /n8n/webhook` for event-driven external leads
- `GET /leads/{id}/raw` for server/admin raw audit access
- `/metrics` when metrics runtime support is enabled

API examples and collections, when present, live under `api/bruno/`. Long-form route contracts live in the centralized API docs.

## Storage And Migrations

`DATABASE_URL` selects Postgres or local SQLite fallback. App startup applies the required migrations automatically for normal development runs. Migration files and storage code remain in this source repo; migration planning and operational runbooks live in the centralized backend docs.

## Environment Notes

Important variables are documented in `.env.example`. The most commonly needed local values are:

- `DATABASE_URL` for Postgres or local SQLite fallback
- `API_TOKEN` for `/run`, `/digest`, `/ingest`, and related protected endpoints
- `SLACK_WEBHOOK_URL` for lead delivery
- `CLAUDE_API_KEY`, `OLLAMA_*`, or future provider settings for enrichment
- collector toggles such as `CREATIVEBC_ENABLED`, `VCC_ENABLED`, `BCBID_ENABLED`, `NEWS_ENABLED`, and `EVENTBRITE_ENABLED`
- `AUDIT_RETENTION_*` for raw audit cleanup behavior

Keep secrets in local environment files or secret stores. Do not commit real tokens, webhook URLs, provider keys, or database credentials.

## Documentation Policy

Keep this repository focused on backend source, tests, migrations, scripts, Docker/runtime files, and this README. Add or update long-lived Markdown in `/mnt/c/Users/alvin/groupscout-site/backend/` unless the file is generated or must live beside runtime output.
