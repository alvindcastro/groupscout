# Troubleshooting — Missing Leads & Pipeline Gaps

This guide helps you diagnose why the GroupScout pipeline may return "0 new leads" or fewer leads than expected.

## 🔍 Core Pipeline Flow

GroupScout filters data at three distinct stages. Understanding these is key to troubleshooting:

1.  **Collector-level Filter**: Each source (Richmond, Delta, BCBid, etc.) has its own relevance rules. For example, Richmond building permits are filtered by a **minimum value** (default $500,000) and **commercial sub-types** (e.g., "hotel", "warehouse", "apartment"). Anything else is skipped before it even reaches the main pipeline.
2.  **Database Deduplication**: Before processing a lead, the system checks the `raw_projects` table for a matching SHA256 hash (based on title, location, and date). If it's already in the database, it's skipped.
3.  **Pre-scoring (Enrichment Threshold)**: The Go pre-scoring engine calculates a score based on location (Richmond/YVR gets +2) and keywords. If the score is below the `ENRICHMENT_THRESHOLD` (default: 1), it is marked as `skipped` and **will not** be enriched by AI or sent to Slack.

---

## 🛠 Troubleshooting Steps

### 1. Check the Logs
If you run `go run cmd/server/main.go --run-once`, look for these log markers:

- `parsed records from PDF count=102`: How many raw entries were found before filtering.
- `filtering complete source=richmond passed=5 skipped_low_value=90 skipped_residential=7`: Detailed breakdown of why permits were excluded.
- `skipping duplicate`: The permit was already processed in a previous run.
- `skipping enrichment: low score score=0`: The project was recorded but wasn't interesting enough to justify AI costs.

### 2. Inspect the Database
Use `psql` (Postgres) or `sqlite3` (SQLite) to see what's happening under the hood.

#### Postgres (Recommended)
```bash
# Total count of collected projects vs. those that were enriched
docker exec groupscout_postgres psql -U groupscout -d groupscout -c "SELECT status, count(*) FROM leads GROUP BY status;"

# Look at the most recent "skipped" leads and why they were skipped
docker exec groupscout_postgres psql -U groupscout -d groupscout -c "SELECT title, priority_score, priority_reason FROM leads WHERE status = 'skipped' ORDER BY created_at DESC LIMIT 10;"
```

#### SQLite
```bash
# Total count of collected projects vs. those that were enriched
sqlite3 groupscout.db "SELECT status, count(*) FROM leads GROUP BY status;"
```

### 3. Adjust Thresholds
If the filters are too strict, you can relax them in your `.env` file or environment variables:

- `MIN_PERMIT_VALUE_CAD`: Lower this (e.g., to `100000`) to include smaller construction projects.
- `ENRICHMENT_THRESHOLD`: Set this to `0` to enrich **every** project that passes the collector filter, regardless of keywords or location.
- `PRIORITY_ALERT_THRESHOLD`: Set this to a lower value (e.g., `5`) if you want more "instant alerts" in Slack for moderate-priority leads.

### 4. Verify External Tools
- **Richmond/Delta Permits**: These require `pdftotext` (from `poppler-utils`) to be installed on your system or inside your Docker container. If it's missing, these collectors will log an error and return 0 leads.
- **n8n / API Triggers**: If you are triggering the pipeline via `/run`, ensure your `API_TOKEN` is correct.

---

## 🖥 Backend Plus UI Docker Troubleshooting

Use [Backend And Frontend Docker E2E](../planning/ui/BACKEND_FRONTEND_DOCKER_E2E.md) for the full current smoke runbook.

### `compose.dev.yml` Fails By Itself

The UI repo's `compose.dev.yml` is an override for the backend Compose file. It expects backend service `groupscout` and network `groupscout_net`.

Run it merged with the backend file:

```bash
cd /mnt/c/Users/alvin/WebstormProjects/groupscout-ui

docker compose -p groupscout \
  -f /mnt/c/Users/alvin/GolandProjects/groupscout/docker-compose.yml \
  -f compose.dev.yml \
  config --quiet
```

### `localhost:3001` Does Not Serve The UI

That is expected. Port `3001` is the UI D3 health harness and only serves `/healthz`.

Use the D4 production UI container on port `3002` for static assets and `/api/*` proxy smoke checks.

### Port `3000` Is Busy

The backend Compose stack publishes Grafana on host port `3000`. Use `3001` for the D3 health harness and `3002` for the D4 production UI smoke container.

### D4 Proxy Returns `502`

The UI production container cannot reach the backend target.

Check:

- The container was started with `--network groupscout_groupscout_net`.
- `UI_API_PROXY_TARGET=http://groupscout:8080`.
- Backend service `groupscout` is running: `docker compose -p groupscout -f /mnt/c/Users/alvin/GolandProjects/groupscout/docker-compose.yml ps groupscout`.

### D4 Proxy Returns `404`

The proxy reached the backend, but the route does not exist. This is currently expected for UI-modeled routes such as `/api/system` and `/api/leads` until backend `/api/*` endpoints are implemented.

### UI Container Has Backend Secrets

Do not pass backend `.env` files into UI containers. The browser-visible UI must not receive `API_TOKEN`, provider keys, Slack tokens, email-provider keys, database URLs, Ollama endpoints, or `UI_SESSION_SECRET`.

---

## ❓ FAQ

### Why is there only 1 lead?
This usually happens if:
- You've already run the pipeline today, and all other permits are now **duplicates**.
- The latest weekly report only has 1 permit that meets both the **commercial sub-type** AND the **$500k+ value** criteria.
- Most other permits have a **score of 0** (meaning they aren't in Richmond/YVR and didn't contain any high-value keywords like "pipeline", "film", or "concrete").

### How do I re-process old leads?
If you want to force the pipeline to re-process everything (e.g., after changing the scoring logic), you must clear the database:
```bash
# This stops containers, removes volumes, and deletes local SQLite db
make clear
```
*(Or manually delete rows from `raw_projects` and `leads` tables).*
