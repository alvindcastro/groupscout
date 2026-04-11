# GroupScout — Ollama Docker Setup & Infrastructure

> How Ollama runs, where it lives, how it talks to GroupScout, and how to keep it working.  
> Covers: Docker Compose wiring, model persistence, networking, dev vs. prod, health checks, and first-boot model pull.

> **Your machine:** Windows 11 + WSL2 · Intel i5-13400F · **32 GB RAM** · **NVIDIA RTX 4060 Ti (16 GB VRAM)**  
> Mistral 7B (~4 GB) and Llama 3.1 8B (~5 GB) both fit in VRAM simultaneously with headroom to spare. Inference is ~1–2s per call with GPU, vs 8–15s CPU-only. **GPU passthrough is strongly recommended on this machine.**

---

## Table of Contents

1. [Overview — How Ollama Runs](#1-overview--how-ollama-runs)
2. [Phase 1 — Docker Compose Service](#phase-1--docker-compose-service)
3. [Phase 2 — Model Persistence & First-Boot Pull](#phase-2--model-persistence--first-boot-pull)
4. [Phase 3 — Networking (Ollama ↔ GroupScout)](#phase-3--networking-ollama--groupscout)
5. [Phase 4 — Dev Environment (Local, No Docker)](#phase-4--dev-environment-local-no-docker)
6. [Phase 5 — GPU Passthrough (Optional)](#phase-5--gpu-passthrough-optional)
7. [Phase 6 — Health Checks & Startup Ordering](#phase-6--health-checks--startup-ordering)
8. [Phase 7 — Prod Hardening](#phase-7--prod-hardening)
9. [Reference: Full docker-compose.yml Snippet](#reference-full-docker-composeyml-snippet)
10. [Reference: Environment Variables](#reference-environment-variables)
11. [Troubleshooting](#troubleshooting)

---

## 1. Overview — How Ollama Runs

Ollama runs as a **sidecar container** in the same Docker Compose stack as GroupScout. It exposes an HTTP API on port `11434`. GroupScout connects to it over the internal Docker network — no ports need to be published to the host in production.

```
┌─────────────────────────────────────────────┐
│  Docker Compose stack                       │
│                                             │
│  groupscout  ──HTTP──▶  ollama:11434        │
│  (Go binary)            (LLM runtime)       │
│                              │              │
│                         named volume        │
│                         ollama_data/        │
│                         (models on disk)    │
└─────────────────────────────────────────────┘
```

**Key design decisions:**

- Models are stored in a **named Docker volume** (`ollama_data`). They survive container restarts and rebuilds without re-downloading (each model is 2–5 GB).
- GroupScout reaches Ollama via `http://ollama:11434` inside Docker. Locally (without Docker), it reaches `http://localhost:11434`.
- `OLLAMA_ENDPOINT` in `.env` toggles between the two — no code change needed.
- A `model-puller` init container runs `ollama pull` on first boot so the main app never starts against an empty model store.

---

## Phase 1 — Docker Compose Service

**Goal:** Add Ollama as a named service in `docker-compose.yml` with a persistent volume, resource limits, and no published port (internal only).

### Tasks

- [x] Add `ollama` service to `docker-compose.yml`
- [x] Add `ollama_data` named volume to the `volumes:` section
- [x] Mount volume at `/root/.ollama` inside the container (Ollama's default model cache path)
- [x] Set `restart: unless-stopped`
- [x] Do **not** publish port `11434` to host in production — internal network only
- [x] Add `mem_limit: 12g` (adjust to your VPS RAM; Mistral 7B needs ~8 GB loaded)
- [x] Add `cpus: "2.0"` to prevent Ollama starving GroupScout during inference
- [x] Add Ollama to the existing internal Docker network (`groupscout_net` or whatever is defined)
- [x] Confirm `groupscout` service has `depends_on: ollama` with condition `service_healthy`

### docker-compose.yml snippet — Ollama service

```yaml
services:

  ollama:
    image: ollama/ollama:latest
    container_name: groupscout_ollama
    restart: unless-stopped
    volumes:
      - ollama_data:/root/.ollama
    networks:
      - groupscout_net
    # Do NOT publish 11434 to host in prod — internal only
    # ports:
    #   - "11434:11434"   # uncomment for local debugging only
    mem_limit: 12g
    cpus: "2.0"
    healthcheck:
      test: ["CMD", "ollama", "list"]
      interval: 30s
      timeout: 10s
      retries: 5
      start_period: 60s   # models may be loading on first start

volumes:
  ollama_data:
    name: groupscout_ollama_data

networks:
  groupscout_net:
    driver: bridge
```

---

## Phase 2 — Model Persistence & First-Boot Pull

**Goal:** Models must be pulled on first boot and never re-downloaded on restart. A dedicated init service handles the pull so GroupScout never starts against an empty model store.

### Tasks

- [x] Add `ollama-init` service to `docker-compose.yml` — runs once, pulls required models, then exits
- [x] Use `ollama/ollama:latest` image for init service (same image, different entrypoint)
- [x] Pull models: `mistral` (required), `llama3.1:8b` (optional, alert copy), `phi3:mini` (fallback)
- [x] Use `depends_on` in `groupscout` service: wait for both `ollama` (healthy) and `ollama-init` (completed)
- [x] Mark `ollama-init` with `restart: "no"` — it must not loop
- [x] Verify volume is shared between `ollama` and `ollama-init` so pulled models are visible to the runtime

### docker-compose.yml snippet — Init service

```yaml
  ollama-init:
    image: ollama/ollama:latest
    container_name: groupscout_ollama_init
    restart: "no"
    volumes:
      - ollama_data:/root/.ollama
    networks:
      - groupscout_net
    entrypoint: ["/bin/sh", "-c"]
    command:
      - |
        echo "Waiting for Ollama runtime..."
        until OLLAMA_HOST=http://ollama:11434 ollama list > /dev/null 2>&1; do
          sleep 3
        done
        echo "Pulling mistral (required)..."
        OLLAMA_HOST=http://ollama:11434 ollama pull mistral
        echo "Pulling llama3.1:8b (optional, alert copy)..."
        OLLAMA_HOST=http://ollama:11434 ollama pull llama3.1:8b
        echo "Pulling phi3:mini (fallback)..."
        OLLAMA_HOST=http://ollama:11434 ollama pull phi3:mini
        echo "Model pull complete."
    depends_on:
      ollama:
        condition: service_healthy
    environment:
      - OLLAMA_HOST=http://ollama:11434
```

> **Note on `llama3.1:8b`:** It's ~5 GB. Only add it to the pull list if your VPS has 16 GB+ RAM. Mistral 7B alone covers all three GroupScout use cases on a 16 GB server.

### Checking what's loaded

```bash
# From host
docker exec groupscout_ollama ollama list

# From inside the GroupScout container
curl http://ollama:11434/api/tags
```

---

## Phase 3 — Networking (Ollama ↔ GroupScout)

**Goal:** GroupScout must resolve `ollama` as a hostname inside Docker and `localhost` when running natively. `OLLAMA_ENDPOINT` in `.env` is the only change needed between environments.

### Tasks

- [x] Confirm both `groupscout` and `ollama` services share the same Docker network (`groupscout_net`)
- [x] Set `OLLAMA_ENDPOINT=http://ollama:11434` in `.env` for Docker deploys
- [x] Set `OLLAMA_ENDPOINT=http://localhost:11434` in `.env.local` for native dev
- [x] Add a startup log line in GroupScout: `"ollama endpoint: %s"` so it's obvious which URL is in use
- [x] Test DNS resolution inside the GroupScout container:
  ```bash
  docker exec groupscout_app curl -s http://ollama:11434/api/tags | jq .
  ```
- [x] Add `ollama` to `groupscout` service's `depends_on` block with `condition: service_healthy`
- [x] Add `/health` endpoint response field: `"ollama": "ok" | "degraded" | "unavailable"`

### groupscout service depends_on

```yaml
  groupscout:
    build: .
    container_name: groupscout_app
    restart: unless-stopped
    networks:
      - groupscout_net
    depends_on:
      ollama:
        condition: service_healthy
      ollama-init:
        condition: service_completed_successfully
    environment:
      - OLLAMA_ENDPOINT=http://ollama:11434
      - OLLAMA_MODEL=mistral
      - OLLAMA_ENABLED=true
```

---

## Phase 4 — Dev Environment (Windows + WSL2, No Docker)

**Goal:** Run Ollama natively inside WSL2 for faster dev iteration — no Docker overhead, models reload in seconds, Go tests hit `localhost:11434` directly.

### How it works on WSL2

Ollama runs as a process **inside your WSL2 distro** (Ubuntu). It binds to `localhost:11434` within WSL2. Your Go code running inside the same WSL2 session connects to it directly. Docker Desktop on Windows can also reach it, but native WSL2 is faster for the dev loop.

```
Windows Host
└── WSL2 (Ubuntu)
    ├── ollama serve          ← binds 0.0.0.0:11434 inside WSL2
    ├── go run / go test      ← connects to localhost:11434
    └── Docker Desktop        ← can also reach host.docker.internal:11434
```

### Tasks

- [x] Install Ollama inside WSL2 (not Windows — the Linux binary runs in WSL2)
- [x] Configure Ollama to start automatically with WSL2 session via `.bashrc` or a systemd unit
- [x] Pull required models inside WSL2
- [x] Add `.env.local` to `.gitignore` — contains `OLLAMA_ENDPOINT=http://localhost:11434`
- [x] Verify GoLand / PyCharm terminal (WSL2 backend) can reach `localhost:11434`
- [x] Confirm `OLLAMA_ENABLED=false` in `.env` boots GroupScout cleanly with no errors

### Install Ollama inside WSL2

Open your WSL2 terminal (Ubuntu):

```bash
# Install Ollama (Linux binary — run inside WSL2, not PowerShell)
curl -fsSL https://ollama.com/install.sh | sh

# Verify it installed
which ollama
ollama --version
```

### Start Ollama in WSL2

WSL2 does not run systemd by default in older setups. Two options:

**Option A — Manual start (simplest)**

```bash
# Start in background, log to file
ollama serve > ~/.ollama/ollama.log 2>&1 &

# Verify it's running
curl http://localhost:11434/api/tags
```

Add this to your `~/.bashrc` or `~/.zshrc` so it auto-starts with every WSL2 session:

```bash
# ~/.bashrc — auto-start Ollama if not already running
if ! pgrep -x "ollama" > /dev/null; then
  ollama serve > ~/.ollama/ollama.log 2>&1 &
fi
```

**Option B — systemd (WSL2 with systemd enabled)**

If your WSL2 has systemd enabled (`/etc/wsl.conf` has `[boot] systemd=true`):

```bash
# Enable and start the Ollama systemd service
sudo systemctl enable ollama
sudo systemctl start ollama
sudo systemctl status ollama
```

Check if systemd is enabled:
```bash
cat /etc/wsl.conf
# Should contain:
# [boot]
# systemd=true
```

To enable it if missing:
```bash
sudo tee /etc/wsl.conf <<EOF
[boot]
systemd=true
EOF
# Then restart WSL2 from PowerShell:
# wsl --shutdown
# wsl
```

### Pull models inside WSL2

```bash
ollama pull mistral        # ~4 GB — required
ollama pull phi3:mini      # ~2 GB — fast fallback

# Verify models are loaded
ollama list
```

### Where models are stored

Models land at `~/.ollama/models` inside WSL2. This is on the WSL2 virtual disk (`.vhdx` on the Windows filesystem). They persist across WSL2 restarts.

If your WSL2 disk fills up, move the model store:
```bash
# Point Ollama at a different path (e.g., a mounted Windows drive)
export OLLAMA_MODELS=/mnt/d/ollama-models
ollama serve &
```

### GoLand / PyCharm WSL2 backend

If you're running GoLand with the WSL2 Go toolchain, the IDE terminal is a WSL2 shell — `localhost:11434` resolves correctly. No extra config needed.

If GoLand runs on Windows (not WSL2 backend), add this to your Windows-side `.env`:
```
OLLAMA_ENDPOINT=http://localhost:11434
```
Docker Desktop on Windows exposes the WSL2 network at `localhost` on the Windows side, so this usually works. If not, use the WSL2 IP directly:
```bash
# Get WSL2 IP from inside WSL2
hostname -I | awk '{print $1}'
# e.g. 172.28.144.1
```
Then set `OLLAMA_ENDPOINT=http://172.28.144.1:11434` in the Windows `.env`.

### Switching between environments

| Mode | `OLLAMA_ENDPOINT` value |
|---|---|
| WSL2 native (Go inside WSL2) | `http://localhost:11434` |
| Docker Compose (prod-like) | `http://ollama:11434` |
| GoLand on Windows → WSL2 Ollama | `http://localhost:11434` (usually works) or WSL2 IP |
| Remote VPS | `http://<vps-ip>:11434` (only if port published — avoid in prod) |

---

## Phase 5 — GPU Passthrough (Optional)

**Goal:** If your machine has an NVIDIA GPU, pass it through to Ollama. On WSL2 this uses CUDA via the Windows NVIDIA driver — no separate Linux driver needed.

### WSL2 + NVIDIA GPU (the common case for Windows devs)

WSL2 has first-class CUDA support. The Windows NVIDIA driver exposes CUDA to WSL2 directly. You do **not** install a Linux NVIDIA driver — it conflicts.

#### Tasks

- [ ] Confirm GPU is visible in WSL2: `nvidia-smi` (should show your GPU without installing anything)
- [ ] If `nvidia-smi` is missing inside WSL2, update the Windows NVIDIA driver to 470.76+ from [nvidia.com/drivers](https://www.nvidia.com/Download/index.aspx)
- [ ] Install CUDA toolkit inside WSL2 (for Docker GPU passthrough):
  ```bash
  # Add NVIDIA Container Toolkit repo
  curl -fsSL https://nvidia.github.io/libnvidia-container/gpgkey | sudo gpg --dearmor -o /usr/share/keyrings/nvidia-container-toolkit-keyring.gpg
  curl -s -L https://nvidia.github.io/libnvidia-container/stable/deb/nvidia-container-toolkit.list | \
    sed 's#deb https://#deb [signed-by=/usr/share/keyrings/nvidia-container-toolkit-keyring.gpg] https://#g' | \
    sudo tee /etc/apt/sources.list.d/nvidia-container-toolkit.list
  sudo apt-get update && sudo apt-get install -y nvidia-container-toolkit
  sudo nvidia-ctk runtime configure --runtime=docker
  sudo systemctl restart docker
  ```
- [ ] Verify Docker can see the GPU: `docker run --rm --gpus all nvidia/cuda:12.0-base nvidia-smi`
- [ ] Add GPU block to `ollama` service in `docker-compose.yml` (snippet below)
- [ ] For native WSL2 Ollama (no Docker): GPU works automatically — Ollama detects CUDA on startup. Check `ollama serve` logs for `"CUDA detected"`.

#### docker-compose.yml snippet — GPU passthrough

```yaml
  ollama:
    image: ollama/ollama:latest
    # ... existing config ...
    environment:
      - NVIDIA_VISIBLE_DEVICES=all
    deploy:
      resources:
        reservations:
          devices:
            - driver: nvidia
              count: 1
              capabilities: [gpu]
```

> **Native WSL2 shortcut:** If you run Ollama natively inside WSL2 (not in Docker), GPU works with zero config — just `ollama serve`. No Docker GPU setup needed. This is the fastest dev path on a Windows machine with an NVIDIA GPU.

> **CPU-only note:** GroupScout's workload is batch, not real-time chat. Mistral 7B on CPU takes ~8–15s per inference call — acceptable for lead enrichment runs. For disruption alert copy (time-sensitive), consider `phi3:mini` on CPU (2–3s) or enable GPU.

---

## Phase 6 — Health Checks & Startup Ordering

**Goal:** GroupScout must never attempt an Ollama call if Ollama is still loading. Health check + `depends_on` ordering enforces this.

### Tasks

- [ ] Confirm `ollama` service healthcheck uses `ollama list` (not `curl` — the official image does not contain `curl`)
- [ ] Set `start_period: 60s` on the healthcheck — model loading on first run can take 30–45s
- [ ] GroupScout startup: call `OllamaClient.HealthCheck()` after config load; log `"ollama: ready"` or `"ollama: unavailable — running in degraded mode"`
- [ ] If `HealthCheck()` fails and `OLLAMA_ENABLED=true`, log a warning but do **not** exit — fall back to regex/hardcoded paths
- [ ] Add `/health` endpoint response field: `"ollama": "ok" | "degraded" | "unavailable"`
- [ ] Write a `TestHealthCheckDegraded` unit test: mock Ollama returning 503, assert GroupScout reports `"degraded"` not a fatal error

### Startup sequence

```
docker compose up -d

1. ollama container starts
2. ollama healthcheck: GET /api/tags (retries every 30s, up to 5 times)
3. ollama-init waits for ollama healthy, then pulls models
4. ollama-init exits with code 0
5. groupscout starts (depends_on: ollama healthy + ollama-init completed)
6. groupscout calls OllamaClient.HealthCheck() — logs status
7. groupscout begins normal operation
```

---

## Phase 7 — Prod Hardening

**Goal:** In production, Ollama should not be reachable from outside the server, model data should persist across deploys, and resource usage should be bounded.

### Tasks

- [ ] Confirm port `11434` is **not** in `ports:` for the `ollama` service in prod — internal network only
- [ ] Add `ollama_data` volume to backup script / cron — models are large but avoid re-downloading on VPS rebuild
- [ ] Add Ollama container logs to Loki (already in the stack) — use `logging: driver: loki` or Promtail scrape
- [ ] Add Grafana dashboard panel: Ollama container CPU and memory usage (Prometheus `container_cpu_usage_seconds_total{name="groupscout_ollama"}`)
- [ ] Document model update procedure in `DOCKER.md`:
  ```bash
  docker exec groupscout_ollama ollama pull mistral   # pulls latest quantization
  docker restart groupscout_app                       # reload GroupScout after model update
  ```
- [ ] Add `OLLAMA_MODEL` to `.env` — allows swapping models (e.g., `mistral` → `phi3:mini`) without code change
- [ ] Test full stack cold start on a fresh VPS: `docker compose up -d` should result in a working system within 10 minutes (model download time)

---

## Reference: Full docker-compose.yml Snippet

This is the complete Ollama-related block to merge into the existing `docker-compose.yml`.

```yaml
services:

  ollama:
    image: ollama/ollama:latest
    container_name: groupscout_ollama
    restart: unless-stopped
    volumes:
      - ollama_data:/root/.ollama
    networks:
      - groupscout_net
    # Uncomment for local debug access only:
    # ports:
    #   - "11434:11434"
    mem_limit: 12g
    cpus: "2.0"
    healthcheck:
      test: ["CMD", "ollama", "list"]
      interval: 30s
      timeout: 10s
      retries: 5
      start_period: 60s
    # Optional GPU passthrough — requires nvidia-container-toolkit on host:
    # environment:
    #   - NVIDIA_VISIBLE_DEVICES=all
    # deploy:
    #   resources:
    #     reservations:
    #       devices:
    #         - driver: nvidia
    #           count: 1
    #           capabilities: [gpu]

  ollama-init:
    image: ollama/ollama:latest
    container_name: groupscout_ollama_init
    restart: "no"
    volumes:
      - ollama_data:/root/.ollama
    networks:
      - groupscout_net
    entrypoint: ["/bin/sh", "-c"]
    command:
      - |
        echo "Waiting for Ollama runtime..."
        until OLLAMA_HOST=http://ollama:11434 ollama list > /dev/null 2>&1; do
          sleep 3
        done
        echo "Pulling mistral (required)..."
        OLLAMA_HOST=http://ollama:11434 ollama pull mistral
        echo "Pulling phi3:mini (fallback)..."
        OLLAMA_HOST=http://ollama:11434 ollama pull phi3:mini
        echo "Done."
    depends_on:
      ollama:
        condition: service_healthy

  groupscout:
    # ... existing groupscout service config ...
    depends_on:
      ollama:
        condition: service_healthy
      ollama-init:
        condition: service_completed_successfully
    environment:
      - OLLAMA_ENABLED=true
      - OLLAMA_ENDPOINT=http://ollama:11434
      - OLLAMA_MODEL=mistral

volumes:
  ollama_data:
    name: groupscout_ollama_data

networks:
  groupscout_net:
    driver: bridge
```

---

## Reference: Environment Variables

```env
# Ollama connection
OLLAMA_ENABLED=true
OLLAMA_ENDPOINT=http://ollama:11434      # Docker: use service name
# OLLAMA_ENDPOINT=http://localhost:11434 # Native dev: use localhost
OLLAMA_MODEL=mistral

# Feature toggles (can disable individual use cases)
OLLAMA_EXTRACTION_ENABLED=true
OLLAMA_SCORING_ENABLED=true
OLLAMA_ALERT_COPY_ENABLED=true

# Per-use-case timeouts (seconds)
OLLAMA_EXTRACT_TIMEOUT_S=30
OLLAMA_SCORE_TIMEOUT_S=20
OLLAMA_ALERT_COPY_TIMEOUT_S=15
```

---

## Troubleshooting

### Ollama container starts but GroupScout can't connect

```bash
# Check they're on the same network
docker network inspect groupscout_net

# Test DNS from inside the app container
docker exec groupscout_app curl -s http://ollama:11434/api/tags

# Check Ollama logs
docker logs groupscout_ollama --tail 50
```

### Models not found after restart

```bash
# Check the volume exists and has data
docker volume inspect groupscout_ollama_data
docker exec groupscout_ollama ollama list
```

If the list is empty, the volume was recreated (e.g., `docker compose down -v` was run). Re-run the init:

```bash
docker compose run --rm ollama-init
```

### Inference is very slow (CPU-only)

Expected: Mistral 7B on 2 vCPU takes 8–15s per call. This is fine for batch lead enrichment. If it's too slow for alert copy:

- Switch `OLLAMA_MODEL=phi3:mini` (2–3s per call, lower quality)
- Or disable `OLLAMA_ALERT_COPY_ENABLED=false` and use the hardcoded template

### `ollama-init` keeps restarting

Check `restart: "no"` is set on the init service. If it shows as restarting, Docker Compose may be using an older syntax — use `restart: on-failure:0` as an alternative.

### WSL2: `ollama serve` exits immediately or port not reachable

```bash
# Check if something else is on 11434
netstat -tulnp | grep 11434

# Check Ollama log
cat ~/.ollama/ollama.log

# Try binding explicitly
OLLAMA_HOST=0.0.0.0:11434 ollama serve
```

### WSL2: `localhost:11434` not reachable from Windows-side GoLand

WSL2 networking changed in WSL 2.0 (mirrored mode). Try:

```bash
# Check your WSL version
wsl --version

# In WSL2, get the IP Docker Desktop / Windows can use
hostname -I
```

Set `OLLAMA_ENDPOINT=http://<that-ip>:11434` in your Windows-side `.env`.

Alternatively, enable WSL2 mirrored networking in `%USERPROFILE%\.wslconfig`:
```ini
[wsl2]
networkingMode=mirrored
```
Then `wsl --shutdown` and reopen. With mirrored mode, `localhost` works from both sides.

### WSL2: `nvidia-smi` not found inside WSL2

Update your Windows NVIDIA driver to 470.76 or newer. The WSL2 CUDA support ships with the Windows driver — no Linux driver install needed. If `nvidia-smi` still fails after updating, ensure you're on WSL2 (not WSL1): `wsl --list --verbose`.

### Out of memory / OOM kill

Reduce `mem_limit` to what your VPS actually has minus ~2 GB for the OS:

| VPS RAM | Recommended `mem_limit` for Ollama |
|---|---|
| 8 GB | `5g` — Phi-3 Mini only |
| 16 GB | `12g` — Mistral 7B |
| 32 GB | `24g` — Mistral 7B + Llama 3.1 8B |

---

*Last updated: April 2026*  
*Repository: github.com/alvindcastro/groupscout*
