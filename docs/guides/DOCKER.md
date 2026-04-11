# Docker Guide — GroupScout

This guide provides technical details for running and troubleshooting GroupScout using Docker and Docker Compose.

## 🚀 Running with Docker Compose

GroupScout includes a `docker-compose.yml` that starts the full stack, including the lead generation server, disruption monitor, database, and monitoring tools.

### 1. Prerequisites
- **Docker Desktop** (for Windows/macOS) or **Docker Engine + Docker Compose** (for Linux).
- **WSL2** (strongly recommended for Windows users).

### 2. Environment Setup
Ensure you have a `.env` file in the project root with the minimum required variables:
```env
CLAUDE_API_KEY=sk-ant-...
SLACK_WEBHOOK_URL=https://hooks.slack.com/services/XXX/YYY/ZZZ
SLACK_BOT_TOKEN=xoxb-...
API_TOKEN=your_secure_token_here
DATABASE_URL=postgres://groupscout:groupscout@postgres:5432/groupscout
```

### 3. Start the Stack
```bash
docker compose up -d
```
This starts all services in the background.

### 4. Service Overview

| Service | Port | Description |
|---|---|---|
| `app` | 8080 | Main lead generation server (cmd/server) |
| `alertd` | 8081 | Airport disruption monitor (cmd/alertd) |
| `postgres` | 5432 | Database with `pgvector` extension |
| `n8n` | 5678 | Workflow automation / scheduler |
| `prometheus`| 9090 | Metrics collection |
| `grafana` | 3000 | Dashboards and log viewer |
| `loki` | 3100 | Log aggregation |
| `promtail`| 9080 | Log scraper (internal) |

## 🛠 Common Commands

### Check Container Status
```bash
docker compose ps
```

### View Logs
```bash
# Follow app logs
docker compose logs -f app

# Follow alertd logs
docker compose logs -f alertd

# View last 50 lines for all services
docker compose logs --tail=50
```

### Rebuild and Restart
If you change the Go code, you must rebuild the containers:
```bash
docker compose up -d --build
```

### Teardown
```bash
# Stop and remove containers
docker compose down

# Stop and remove containers, volumes, and images
docker compose down --rmi all --volumes
```

## 🧠 Ollama Model Management

Models are stored in a persistent volume (`groupscout_ollama_data`).

### Update a Model
To pull the latest version of a model (e.g., `mistral`):
```bash
docker exec groupscout_ollama ollama pull mistral
docker restart groupscout_app
```

### Changing the Default Model
Edit the `OLLAMA_MODEL` variable in your `.env` file and restart the stack:
```env
OLLAMA_MODEL=phi3:mini
```

## 💾 Backups

It is recommended to back up your database and Ollama models periodically.

### Automated Backup Script
Run the provided script to back up all named volumes:
```bash
./scripts/backup-volumes.sh
```
This creates compressed tarballs in the `./backups` directory.

### Manual Database Dump (Postgres)
```bash
docker exec -t groupscout_postgres pg_dumpall -c -U groupscout > dump.sql
```

## 📊 Monitoring & Logging

GroupScout includes a full observability stack.

### Logging (Loki + Promtail)
Logs from all containers are automatically scraped by `promtail` and sent to `loki`.
- **View logs in Grafana**: Open `http://localhost:3000`, go to **Explore**, and select the **Loki** data source.
- **Filter by container**: Use `{container="groupscout_app"}` or `{container="groupscout_ollama"}`.

### Metrics (Prometheus + Grafana)
Metrics are collected by `prometheus` and can be visualized in `grafana`.
- **Prometheus UI**: `http://localhost:9090`
- **Grafana UI**: `http://localhost:3000` (default login: `admin/admin`)

#### Custom Ollama Panel
To monitor Ollama resource usage, add a panel with these queries:
- **CPU Usage**: `rate(container_cpu_usage_seconds_total{container="groupscout_ollama"}[5m])`
- **Memory Usage**: `container_memory_usage_bytes{container="groupscout_ollama"}`

## 🔍 Troubleshooting

### 1. `pdftotext not found` (Alpine Container)
GroupScout requires the `pdftotext` utility for scraping certain PDF sources. The Alpine-based Dockerfile now includes `poppler-utils` in the final stage to provide this functionality.
- **Symptom:** `richmond_permits` and `delta_permits` collectors failing.
- **Fix:** Ensure you are using the latest `Dockerfile` and have rebuilt your container with `docker compose up -d --build`.

### 2. Go Version Mismatch
- **Symptom:** `go mod download` fails during `docker build`.
- **Fix:** Ensure the `FROM golang:X.XX-alpine` version in `Dockerfile` matches the `go 1.XX` version in `go.mod`.

### 3. Container Communication
- **Symptom:** n8n cannot connect to the `app` server.
- **Fix:** When n8n is running inside Docker, use `http://app:8080` instead of `localhost:8080`. Containers on the same Docker network use their service names as hostnames.

### 4. WSL2 Specifics (Windows)
- **Symptom:** `docker-compose: command not found`.
- **Fix:** Modern Docker uses `docker compose` (v2 plugin). Ensure **Docker Desktop → Settings → Resources → WSL Integration** is enabled for your specific Linux distro.

### 5. Permission Denied on Docker Socket (Linux)
- **Symptom:** `docker compose` requires `sudo`.
- **Fix:** Add your user to the `docker` group:
  ```bash
  sudo usermod -aG docker $USER
  newgrp docker
  ```

### 6. Database Connection Issues
- **Symptom:** `app` container fails to start because it can't reach Postgres.
- **Fix:** The `app` service is configured to depend on `postgres` being healthy. Check `docker compose logs postgres` to ensure the database initialized correctly.
