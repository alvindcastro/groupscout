# Backend For UI Testing

Use this runbook when a frontend developer needs a GroupScout backend available while building or testing the operator UI.

The current backend is ready for health checks, pipeline triggers, metrics, and raw audit lookup. The UI-specific `/api/*` lead list/detail endpoints are still planned, so a UI scaffold may need mocked lead data until those endpoints are implemented.

## Quick Start: Local Backend

This is the fastest path for UI work because it avoids the full Docker stack and does not pull Ollama models.

```bash
DATABASE_URL=groupscout-ui-dev.db \
API_TOKEN=dev-token \
OLLAMA_ENABLED=false \
ENRICHMENT_ENABLED=false \
go run cmd/server/main.go
```

The server listens on `http://localhost:8080` unless `PORT` is set.

Verify it:

```bash
curl -i http://localhost:8080/health
```

Expected result is `200 OK` with JSON similar to:

```json
{"database":"ok","ollama":"unavailable","status":"ok"}
```

## Postgres-Backed Local Backend

Use this when the UI needs behavior closer to production persistence, migrations, or pgvector.

```bash
docker compose up -d postgres
docker compose ps
```

Then start the Go server against the local Postgres port:

```bash
DATABASE_URL='postgres://groupscout:groupscout@localhost:5432/groupscout?sslmode=disable' \
API_TOKEN=dev-token \
OLLAMA_ENABLED=false \
ENRICHMENT_ENABLED=false \
go run cmd/server/main.go
```

Verify:

```bash
curl -i http://localhost:8080/health
```

## Full Docker Stack

Use this only when UI testing also needs the containerized app, n8n, Grafana, Prometheus, Loki, or the Ollama-backed enrichment path.

```bash
docker compose up -d groupscout
docker compose ps
docker compose logs -f groupscout
```

Service URLs:

| Service | URL |
|---|---|
| GroupScout API | `http://localhost:8080` |
| n8n | `http://localhost:5678` |
| Grafana | `http://localhost:3000` |
| Prometheus | `http://localhost:9090` |

The `groupscout` compose service depends on Postgres, Ollama, and `ollama-init`. First boot can take several minutes because `ollama-init` pulls models.

## Backend Plus UI Docker Smoke

For a single backend-plus-frontend Docker smoke path, see [BACKEND_FRONTEND_DOCKER_E2E.md](./BACKEND_FRONTEND_DOCKER_E2E.md).

Short version from the UI repo:

```bash
cd /mnt/c/Users/alvin/WebstormProjects/groupscout-ui

docker compose -p groupscout \
  -f /mnt/c/Users/alvin/GolandProjects/groupscout/docker-compose.yml \
  -f compose.dev.yml \
  up -d --build groupscout groupscout-ui

curl -i http://localhost:8080/health
curl -i http://localhost:${GROUPSCOUT_UI_HOST_PORT:-3001}/healthz
```

The `groupscout-ui` Compose service is the D3 product dev server. It proves the UI container can build, join the backend network, serve product assets, and expose `/healthz`. Use the D4 production container for the same-origin production proxy smoke.

For the current same-origin UI runtime, build and run the D4 production image on the backend Compose network:

```bash
docker build --target production -t groupscout-ui-production .

docker run --rm -d \
  --name groupscout-ui-production-smoke \
  --network groupscout_groupscout_net \
  -p 3002:3000 \
  -e UI_API_PROXY_TARGET=http://groupscout:8080 \
  groupscout-ui-production

curl -i http://localhost:3002/healthz
curl -i http://localhost:3002/
curl -i http://localhost:3002/assets/app.js
curl -i http://localhost:3002/api/system
curl -i http://localhost:3002/api/alerts?limit=1

docker stop groupscout-ui-production-smoke
```

`GET /healthz`, `GET /`, `GET /assets/app.js`, `GET /api/system`, and `GET /api/alerts?limit=1` should return `200` through the UI container. A `502` means the UI container could not reach `UI_API_PROXY_TARGET`.

## API Calls Useful During UI Work

Health check:

```bash
curl -i http://localhost:8080/health
```

Pipeline trigger:

```bash
curl -i -X POST http://localhost:8080/run \
  -H "Authorization: Bearer dev-token" \
  -H "Content-Type: application/json" \
  -d '{"bcbid_raw_input": ""}'
```

Metrics:

```bash
curl -i http://localhost:8080/metrics
```

Raw audit payload for a known lead:

```bash
curl -i http://localhost:8080/leads/LEAD_ID/raw
```

## Frontend Wiring Notes

Do not put `API_TOKEN` in browser JavaScript. It is an automation token for server-to-server calls such as n8n. A browser UI should use a same-origin session boundary, a local dev proxy, or mocked API data until the UI auth layer exists.

If the UI dev server runs on a different port, prefer a dev-server proxy that forwards API calls to `http://localhost:8080`. The Go server does not currently implement a browser CORS policy for the future UI.

Example Vite-style proxy target:

```text
/health -> http://localhost:8080/health
/metrics -> http://localhost:8080/metrics
/api/* -> http://localhost:8080/api/*
```

The `/api/*` routes above are the live UI contract. They stay separate from the automation endpoints so browser response shapes and session boundaries can evolve without exposing `API_TOKEN`.

## Current UI Contract Status

Implemented today:

| Endpoint | UI testing use |
|---|---|
| `GET /health` | Backend reachability and database readiness. |
| `GET /metrics` | Observability smoke check; not intended as direct UI data. |
| `POST /run` | Manual pipeline trigger behind Bearer auth. |
| `POST /digest` | Manual digest trigger behind Bearer auth. |
| `POST /n8n/webhook` | External lead ingestion behind Bearer auth. |
| `GET /api/leads` | Lead inbox with filters and pagination. |
| `GET /api/leads/{id}` | Lead detail. |
| `PATCH /api/leads/{id}` | Status, owner, notes, snooze, and safe corrections. |
| `GET /api/leads/{id}/raw` | Authenticated UI alias for raw audit evidence. |
| `GET/POST /api/leads/{id}/outreach` | Outreach activity history and logging. |
| `GET/POST /api/pipeline/runs` | Run history and async pipeline controls. |
| `GET /api/stats` | UI summaries by status, source, score band, owner, and week. |
| `GET /api/system` | UI-friendly system health summary. |
| `GET /api/alerts` | Read-only alert-console compatibility endpoint. |

## Troubleshooting

| Symptom | Check |
|---|---|
| `connection refused` on `localhost:8080` | Confirm the Go process is still running or `docker compose ps` shows `groupscout` up. |
| `401 Unauthorized` on `POST /run` | Match the Bearer value to `API_TOKEN`; for this runbook use `Bearer dev-token`. |
| `/health` returns database error | Check `DATABASE_URL`; for local SQLite use a writable `.db` path, for Postgres verify `docker compose ps postgres`. |
| Docker startup is slow | `ollama-init` is likely pulling models. Use the local backend path for UI-only work. |
| Browser requests fail from a UI dev server | Add a dev proxy or same-origin wrapper; do not rely on CORS being enabled in the Go server. |
| D4 `/api/system` or `/api/alerts` returns `404` | Check that the backend image includes the current UI API routes and was rebuilt before starting the smoke. |
