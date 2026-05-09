# Backend And Frontend Docker E2E

Use this runbook when you want the current GroupScout backend and the separate UI Docker image running together on one Docker network.

This is a smoke path, not a full product UI acceptance test. The backend is live, the UI production container serves static assets and forwards `/api/*`, and the backend implements the core UI smoke routes used here.

## Repos

| Repo | Path |
|---|---|
| Backend | `/mnt/c/Users/alvin/GolandProjects/groupscout` |
| UI | `/mnt/c/Users/alvin/WebstormProjects/groupscout-ui` |

## What Runs

| Runtime | Port | Purpose |
|---|---:|---|
| Backend `groupscout` service | `8080` | Live Go API container from backend Compose. |
| UI D3 product dev server | `3001` by default | Proves the UI service can build, join the backend Compose network, serve product assets, and expose `/healthz`. |
| UI D4 production container | `3002` in this runbook | Serves `web/dist`, exposes `/healthz`, and proxies same-origin `/api/*` to `http://groupscout:8080`. |
| Grafana | `3000` | Backend observability stack, so the UI examples avoid host port `3000`. |

The D4 production UI runtime is not wired into Compose yet. Until that exists, start the backend with Compose and attach the UI production container manually to `groupscout_groupscout_net`.

## Prerequisites

- Docker Desktop or Docker Engine with Compose v2.
- Backend `.env` in `/mnt/c/Users/alvin/GolandProjects/groupscout`.
- UI repo present at `/mnt/c/Users/alvin/WebstormProjects/groupscout-ui`.
- Time for first backend boot: the current `groupscout` service depends on `ollama` and `ollama-init`, so first startup may pull models.

## Start Backend Plus D3 UI Product Server

Run from the UI repo so `compose.dev.yml` is local, but pin the Compose project name to `groupscout` so the Docker network name is stable:

```sh
cd /mnt/c/Users/alvin/WebstormProjects/groupscout-ui

docker compose -p groupscout \
  -f /mnt/c/Users/alvin/GolandProjects/groupscout/docker-compose.yml \
  -f compose.dev.yml \
  up -d --build groupscout groupscout-ui
```

Smoke the backend and the D3 UI product server:

```sh
curl -i http://localhost:8080/health
curl -i http://localhost:${GROUPSCOUT_UI_HOST_PORT:-3001}/healthz
```

Expected:

- Backend `/health` returns `200`.
- UI D3 `/healthz` returns `200`.

Use the D4 production container on port `3002` for same-origin production proxy smoke checks.

## Start The D4 Production UI Container

Still from the UI repo:

```sh
docker build --target production -t groupscout-ui-production .

docker run --rm -d \
  --name groupscout-ui-production-smoke \
  --network groupscout_groupscout_net \
  -p 3002:3000 \
  -e UI_API_PROXY_TARGET=http://groupscout:8080 \
  groupscout-ui-production
```

Smoke the frontend origin:

```sh
curl -i http://localhost:3002/healthz
curl -i http://localhost:3002/
curl -i http://localhost:3002/assets/app.js
curl -i http://localhost:3002/api/system
curl -i http://localhost:3002/api/alerts?limit=1
```

Expected:

- `/healthz`, `/`, and `/assets/app.js` should return `200` from the UI container.
- `/api/system`, `/api/leads?limit=1`, and `/api/alerts?limit=1` should return `200` through the same-origin UI proxy.
- A `502` from `/api/*` means the UI container could not reach the backend target.

## Cleanup

```sh
docker stop groupscout-ui-production-smoke

docker compose -p groupscout \
  -f /mnt/c/Users/alvin/GolandProjects/groupscout/docker-compose.yml \
  -f compose.dev.yml \
  down
```

Use `--volumes` only when you intentionally want to delete Postgres, n8n, Grafana, and Ollama volumes.

## Important Boundaries

- Do not run `/mnt/c/Users/alvin/WebstormProjects/groupscout-ui/compose.dev.yml` by itself. It depends on backend service `groupscout` and network `groupscout_net`.
- Do not pass the backend `.env` into UI containers.
- Do not expose `API_TOKEN`, provider keys, Slack tokens, email-provider keys, database URLs, Ollama endpoints, or `UI_SESSION_SECRET` to browser-visible UI config or static assets.
- Browser-facing UI code should use relative `/api/*`; only the server-side D4 container should know `http://groupscout:8080`.

