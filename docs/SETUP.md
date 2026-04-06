# GroupScout — Setup & Run Guide

End-to-end walkthrough: from prerequisites to n8n scheduling the pipeline automatically.

---

## Prerequisites

| Tool | Required for | Install |
|---|---|---|
| Go 1.26+ | Local server mode | https://go.dev/dl/ |
| `pdftotext` | PDF permit scraping | Bundled with Git for Windows at `C:\Program Files\Git\mingw64\bin\pdftotext.exe` — no extra install needed |
| Docker + Docker Compose | Docker mode | https://docs.docker.com/get-docker/ |

---

## Step 1 — Create your `.env` file

Copy the example and fill in your keys:

```bash
cp .env.example .env
```

Minimum required values:

```env
CLAUDE_API_KEY=sk-ant-...
SLACK_WEBHOOK_URL=https://hooks.slack.com/services/XXX/YYY/ZZZ
API_TOKEN=your_secure_token_here
DATABASE_URL=groupscout.db # Or postgres://user:pass@localhost:5432/db
```

Generate a secure `API_TOKEN`:
```bash
openssl rand -hex 32
```

> Everything else has sensible defaults. See `.env.example` for the full list.

---

## Step 2 — Choose a run mode

### Option A — Local Go server

```bash
# Install dependencies
go mod download

# Start the HTTP server (stays running, listens on :8080)
go run cmd/server/main.go
```

Expected output:
```
INFO  GroupScout server started  addr=:8080
INFO  Database ready
```

**Or run the pipeline once and exit (no server):**
```bash
go run cmd/server/main.go --run-once
```

---

### Option B — Docker Compose (recommended)

Starts GroupScout + **Postgres (with pgvector)** + n8n + Prometheus + Grafana + Loki in one command.

```bash
docker-compose up -d
```

> **WSL2 users:** Use `docker-compose` (with hyphen, v1). The `docker compose` (v2 plugin) requires Docker Desktop WSL integration to be enabled for your distro — go to **Docker Desktop → Settings → Resources → WSL Integration** and toggle your distro on.

> **Permission denied on Docker socket?** Run `sudo usermod -aG docker $USER && newgrp docker` then retry.

Services that come up:

| Service | URL | Purpose |
|---|---|---|
| GroupScout API | http://localhost:8080 | Pipeline trigger + health check |
| Postgres | localhost:5432 | Primary database (when configured) |
| n8n | http://localhost:5678 | Workflow scheduler |
| Grafana | http://localhost:3000 | Dashboards + log viewer |
| Prometheus | http://localhost:9090 | Metrics |
| Loki | http://localhost:3100 | Log aggregation |

---

#### Docker — Log Commands

Check container status:
```bash
docker-compose ps
```

Follow GroupScout logs in real time:
```bash
docker-compose logs -f app
```

View recent logs (last 50 lines):
```bash
docker-compose logs app --tail=50
```

Follow logs for all services:
```bash
docker-compose logs -f
```

View logs for a specific service:
```bash
docker-compose logs n8n --tail=30
docker-compose logs grafana --tail=30
```

---

#### Docker — Reading a Pipeline Run

After triggering `/run`, check `docker-compose logs app --tail=50`. A healthy run looks like:

```
INFO  pipeline triggered via HTTP /run
INFO  active collectors  count=8  names="[richmond_permits delta_permits ...]"
INFO  starting collection...  collector=richmond_permits
INFO  processing latest report  source=richmond  count=64
INFO  starting collection...  collector=creativebc
INFO  collection complete  collector=creativebc  count=5
...
INFO  new lead inserted  title="..."  score=9
INFO  enrichment complete  new_leads=3
INFO  sent leads to Slack  count=1
```

**Known warnings (safe to ignore):**
- `"SENDGRID_API_KEY" variable is not set` — expected if email digest isn't configured
- `"RICHMOND_PERMITS_URL" variable is not set` — uses the hardcoded default URL

**Known errors (not yet fixed):**
- `pdftotext not found` on richmond_permits and delta_permits — Poppler isn't installed in the Alpine container yet (Phase 6). These collectors will fail silently; all other collectors still run.

---

#### Docker — Rebuild After Code Changes

```bash
docker-compose up -d --build
```

> If `go mod download` fails during build, check that the Go version in `Dockerfile` matches `go.mod`. `go.mod` currently declares `go 1.26` — the Dockerfile must use `golang:1.26-alpine` or higher.

---

#### Docker — Teardown

Stop containers (keep volumes/images):
```bash
docker-compose down
```

Stop and remove everything including volumes:
```bash
docker-compose down --rmi all --volumes
```

---

## Step 3 — Verify the server is up

```bash
curl http://localhost:8080/health
```

Expected response:
```json
{"status": "ok"}
```

---

## Step 4 — Test the pipeline manually

Trigger a full collect → enrich → Slack run:

```bash
curl -X POST http://localhost:8080/run \
  -H "Authorization: Bearer YOUR_API_TOKEN"
```

You should see new leads posted to Slack within ~30 seconds.

> **Tip:** Lower `MIN_PERMIT_VALUE_CAD=100000` in `.env` during testing to get more results through the filter.

---

## Step 5 — Set up n8n

### 5a — Open n8n

- **Docker mode**: http://localhost:5678
- **Local Go + separate n8n**: install n8n via `npm install -g n8n` and run `n8n start`

Complete the initial account setup on first launch.

---

### 5b — Create a credential

n8n needs your `API_TOKEN` to call GroupScout.

1. Go to **Credentials** → **Add Credential**
2. Search for **Header Auth**
3. Fill in:
   - **Name**: `GroupScout API`
   - **Header Name**: `Authorization`
   - **Value**: `Bearer YOUR_API_TOKEN`
4. Click **Save**

---

### 5c — Create the workflow

1. Go to **Workflows** → **New Workflow**
2. Add a **Schedule** trigger node:
   - **Trigger Interval**: `Weeks`
   - **Days of the Week**: `Monday`, `Wednesday`
   - **Time**: `09:00`
3. Add an **HTTP Request** node and connect it to the Schedule node:
   - **Method**: `POST`
   - **URL**:
     - Docker mode: `http://groupscout:8080/run`
     - Local Go mode: `http://host.docker.internal:8080/run`
   - **Authentication**: `Predefined Credential Type` → `Header Auth` → select `GroupScout API`
4. **Save** the workflow
5. Toggle **Active** (top-right switch) to enable it

---

### 5d — Test the workflow manually

Click **Test Workflow** (or the play button on the Schedule node) to fire it immediately without waiting for Monday.

Check:
- The HTTP Request node shows a `200 OK` response
- A new Slack message appears with leads

---

## Endpoints Reference

| Endpoint | Method | Description |
|---|---|---|
| `/health` | GET | Health check — no auth required |
| `/run` | POST | Run full pipeline (collect → enrich → notify) |
| `/digest` | POST | Send weekly email digest (`?to=email@example.com`) |
| `/n8n/webhook` | POST | Push a single lead from an external n8n workflow |

All endpoints except `/health` require `Authorization: Bearer YOUR_API_TOKEN`.

---

## Troubleshooting

| Problem | Fix |
|---|---|
| `401 Unauthorized` | Check `API_TOKEN` in `.env` matches the Bearer value in n8n |
| `Connection refused` (Docker n8n → app) | Use `http://groupscout:8080` not `localhost` — they share a Docker network |
| `Connection refused` (local n8n → local Go) | Use `http://host.docker.internal:8080` on Mac/Windows |
| No leads generated | Lower `MIN_PERMIT_VALUE_CAD` to `100000` for testing |
| PDF parse errors | Confirm `pdftotext` is on your PATH: `pdftotext -v` |
| `pdftotext not found` (local) | Add `C:\Program Files\Git\mingw64\bin` to your system PATH |
| `pdftotext not found` (Docker) | Poppler isn't installed in the Alpine image yet — Phase 6 fix. Other collectors still run. |
| `go mod download` fails during Docker build | Go version mismatch — `Dockerfile` must use `golang:1.26-alpine` to match `go.mod` |
| `Failed to initialize: protocol not available` | Docker Desktop WSL integration not enabled for your distro — see Docker Desktop → Settings → Resources → WSL Integration |
| `permission denied` on Docker socket | Run `sudo usermod -aG docker $USER && newgrp docker` |
| Docker Desktop UI stuck on stale compose entry | Run `docker-compose down` from the project directory, then restart Docker Desktop |
