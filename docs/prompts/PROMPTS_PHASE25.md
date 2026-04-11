# PROMPTS_PHASE25.md — Production Deployment & Event-Driven Ingestion

> Copy-paste prompts for each part of Phase 25.
> Parts must be done in order: A → B → C → D.
>
> **Goal:** Ship the full stack to production. Home server first (free, immediate). Hetzner + Coolify as the cloud upgrade path. Event-driven `/ingest` endpoint for single-lead enrichment without a full batch run.
>
> **Deployment options analysis:** `docs/planning/DEPLOYMENT_OPTIONS.md`
> **Home server step-by-step:** `docs/guides/HOME_DEPLOY.md`
>
> **TDD conventions for this phase:**
> - Part A and B are infrastructure — no new Go code. TDD = scripted verification checkpoints (`curl`, `docker compose config --quiet`, `openssl s_client`).
> - Part C introduces Go code (`/ingest` handler + `EnrichOne()`). Write tests FIRST, commit failing, then implement.
> - Part D is Terraform IaC — optional; no automated tests required.
> - Standard `testing` package only (no testify). Table-driven tests. `httptest.NewServer` for HTTP handler tests.
> - Each infra step has a documented "verify" command — treat these as the test gate. Do not proceed to the next step until the verify command passes.

---

## Part A — Home Server Deploy (Free, Start Here)

**Context:**
- Target machine: any Linux box or Windows machine with WSL2 running Docker Compose
- DDNS provider: DuckDNS (preferred over No-IP — no monthly confirmation required)
- Reverse proxy: Traefik v3 (Docker provider, automatic Let's Encrypt TLS via TLS-ALPN challenge)
- Fallback for CGNAT ISPs: Cloudflare Tunnel (outbound-only, no port forwarding needed)
- Full guide: `docs/guides/HOME_DEPLOY.md`

---

### A1 — DDNS Registration + Updater

```
Context:
- DuckDNS is free, no monthly confirmation, no CC required
- Subdomain format: groupscout.duckdns.org
- Token is shown on duckdns.org dashboard after login
- Two updater methods: cron script OR Docker container (lscr.io/linuxserver/duckdns)

Task A1 — set up DuckDNS:
  1. Register subdomain at duckdns.org → note token UUID
  2. Choose updater method:

  Method A — cron script (host-level, no container):
    mkdir -p ~/duckdns
    Write ~/duckdns/duck.sh:
      echo url="https://www.duckdns.org/update?domains=groupscout&token=TOKEN&ip=" | curl -k -o ~/duckdns/duck.log -K -
    chmod +x ~/duckdns/duck.sh
    Add crontab entry: */5 * * * * ~/duckdns/duck.sh >/dev/null 2>&1

  Method B — Docker container (preferred, runs alongside stack):
    Add to docker-compose.yml:
      duckdns:
        image: lscr.io/linuxserver/duckdns:latest
        environment:
          - SUBDOMAINS=groupscout
          - TOKEN=${DUCKDNS_TOKEN}
        restart: unless-stopped
    Add to .env: DUCKDNS_TOKEN=<your-token>

Verify A1:
  Run updater once. Check output.
  Script method: cat ~/duckdns/duck.log → must print "OK"
  Docker method: docker compose logs duckdns → look for "Your IP was set to..."
  DNS check: nslookup groupscout.duckdns.org → must resolve to your current public IP
  Public IP check: curl ifconfig.me (compare to nslookup result)

CGNAT check before proceeding:
  Run: curl ifconfig.me
  Compare to your router's WAN IP.
  If router shows 100.64.x.x or 10.x.x.x → you're on CGNAT.
  CGNAT path: skip A2 and go directly to A5 (Cloudflare Tunnel).
```

---

### A2 — Router Port Forwarding

```
Context:
- Only needed if NOT on CGNAT (most home connections are fine)
- Need to forward ports 80 and 443 to the Docker host
- Docker host LAN IP: run `hostname -I | awk '{print $1}'` on the host machine
- Router admin panel: usually 192.168.1.1 or 192.168.0.1

Task A2 — configure port forwarding:
  1. Find Docker host LAN IP: hostname -I | awk '{print $1}'  → e.g. 192.168.1.42
  2. Log in to router admin panel
  3. Navigate to: Port Forwarding / NAT / Virtual Servers (name varies by router)
  4. Create two rules:
     - External port 80  → Internal 192.168.1.42:80   (TCP)
     - External port 443 → Internal 192.168.1.42:443  (TCP)
  5. Save and apply

Verify A2 (before Traefik, use a temp HTTP server):
  # On the Docker host, run a one-liner HTTP server temporarily:
  docker run --rm -p 80:80 traefik/whoami
  # From a phone (not same network) or external checker:
  curl http://groupscout.duckdns.org/
  # Expected: shows Hostname, IP, and request headers from whoami
  # Stop the temp server after verify: docker stop <container_id>

Note: if ISP blocks port 80, test with 8080 first, then move to Traefik TLS challenge (port 443 only).
```

---

### A3 — Traefik + Let's Encrypt in docker-compose.yml

```
Context:
- Current docker-compose.yml: app on 8080, alertd on 8081, n8n on 5678, postgres on 5432
- Goal: add traefik:v3 in front; remove direct port bindings from app/alertd; add labels
- TLS challenge: tlschallenge (uses port 443 only — no port 80 required for cert issuance)
- HTTP→HTTPS redirect: built into Traefik entrypoint config
- Each container gets its own subdomain via Docker labels
- exposedbydefault=false: only containers with traefik.enable=true are routed
- Docker socket mounted read-only for security
- New volume: letsencrypt (stores acme.json with certs)

Subdomains to configure:
  server.groupscout.duckdns.org  → app:8080
  alertd.groupscout.duckdns.org  → alertd:8081
  n8n.groupscout.duckdns.org     → n8n:5678
  prometheus.groupscout.duckdns.org → prometheus:9090
  grafana.groupscout.duckdns.org → grafana:3000

Required .env additions:
  ACME_EMAIL=your@email.com       ← Let's Encrypt contact email
  POSTGRES_PASSWORD=change_me     ← replace default "groupscout"

Task A3 — update docker-compose.yml:
  1. Validate current compose file: docker compose config --quiet  (must exit 0)
  2. Add traefik service with the config below
  3. Remove `ports:` from app and alertd (Traefik routes them internally)
  4. Add `labels:` to app, alertd, n8n, prometheus, grafana
  5. Add `letsencrypt:` to the volumes block
  6. Do NOT remove postgres ports (5432 stays for direct DB access during dev)

Traefik service definition:
  traefik:
    image: traefik:v3
    command:
      - "--log.level=INFO"
      - "--providers.docker=true"
      - "--providers.docker.exposedbydefault=false"
      - "--entrypoints.web.address=:80"
      - "--entrypoints.web.http.redirections.entrypoint.to=websecure"
      - "--entrypoints.web.http.redirections.entrypoint.scheme=https"
      - "--entrypoints.websecure.address=:443"
      - "--certificatesresolvers.letsencrypt.acme.email=${ACME_EMAIL}"
      - "--certificatesresolvers.letsencrypt.acme.storage=/letsencrypt/acme.json"
      - "--certificatesresolvers.letsencrypt.acme.tlschallenge=true"
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - "/var/run/docker.sock:/var/run/docker.sock:ro"
      - "letsencrypt:/letsencrypt"
    restart: unless-stopped

Label pattern for each container (substitute service name and port):
  labels:
    - "traefik.enable=true"
    - "traefik.http.routers.<name>.rule=Host(`<subdomain>.groupscout.duckdns.org`)"
    - "traefik.http.routers.<name>.entrypoints=websecure"
    - "traefik.http.routers.<name>.tls.certresolver=letsencrypt"
    - "traefik.http.services.<name>.loadbalancer.server.port=<port>"

TDD gate — validate before deploying:
  docker compose config --quiet       ← must exit 0
  docker compose config | grep "traefik.enable=true"  ← must show 5 services

Deploy:
  docker compose up -d
  docker compose logs traefik -f --tail=100
  # Watch for: "Obtained certificate from ACME provider"
  # Cert issuance takes 5–30 seconds on first boot.
```

---

### A4 — Verify HTTPS + Subdomain Routing

```
Context:
- This is the acceptance test for Part A (Tier 2)
- All three verify commands must pass before Part A is complete
- TLS certificate issuer must be Let's Encrypt (not self-signed)
- HTTP must redirect to HTTPS (Traefik redirect entrypoint active)

Verify A4 — run all three checks:

  Check 1 — HTTPS health endpoint:
    curl -fs https://server.groupscout.duckdns.org/health
    Expected: {"status":"ok"} (or equivalent JSON with status field)

  Check 2 — TLS certificate issuer:
    curl -sv https://server.groupscout.duckdns.org/health 2>&1 | grep -i "issuer"
    Expected: issuer: C=US, O=Let's Encrypt, CN=R...

  Check 3 — HTTP redirects to HTTPS:
    curl -Iv http://server.groupscout.duckdns.org/health 2>&1 | grep -E "301|Location"
    Expected: HTTP/1.1 301 and Location: https://...

  All three checks must pass. If any fail:
  - Check 1 fails → Traefik not routing; check `docker compose logs traefik`
  - Check 2 fails → cert not issued yet; wait 30s and retry; check rate limits
  - Check 3 fails → HTTP redirect not configured; verify entrypoint redirect labels

  Also verify alertd routing:
    curl -fs https://alertd.groupscout.duckdns.org/health
    Expected: {"status":"ok"}
```

---

### A5 — Cloudflare Tunnel (Optional — CGNAT / No Port Forwarding)

```
Context:
- Use INSTEAD OF port forwarding when ISP uses CGNAT or blocks inbound ports
- Cloudflare Tunnel = outbound-only connection from your machine to Cloudflare edge
- Your residential IP is never exposed in DNS records (Cloudflare shows their IPs)
- Completely free (Zero Trust free plan supports up to 50 users)
- With Cloudflare Tunnel you do NOT need Traefik for TLS — Cloudflare handles TLS at edge
- Internal routing is plain HTTP (Cloudflare → cloudflared container → app/alertd)

Prerequisites:
  - A domain you own (e.g. yourdomain.com) added to Cloudflare, OR
  - A free Cloudflare Pages subdomain

Task A5 — configure Cloudflare Tunnel:
  1. Log in to Cloudflare Zero Trust (one.dash.cloudflare.com)
  2. Networks → Tunnels → Create a tunnel → name it "groupscout"
  3. Copy the tunnel token (long JWT string)
  4. Add to .env: CLOUDFLARE_TUNNEL_TOKEN=<token>
  5. In tunnel's "Public Hostnames" tab, add routes:
     server.<yourdomain.com>  → http://app:8080
     alertd.<yourdomain.com>  → http://alertd:8081
     n8n.<yourdomain.com>     → http://n8n:5678
  6. Add cloudflared service to docker-compose.yml:

     cloudflared:
       image: cloudflare/cloudflared:latest
       command: tunnel --no-autoupdate run --token ${CLOUDFLARE_TUNNEL_TOKEN}
       restart: unless-stopped

  7. If using Cloudflare Tunnel (not port forwarding), remove Traefik service from compose.
     Restore direct port bindings on app/alertd if you want local access:
       ports:
         - "127.0.0.1:8080:8080"   ← loopback only, not exposed to LAN

Verify A5:
  docker compose logs cloudflared | grep -i "registered\|connected"
  # Expected: "Registered tunnel connection" message

  curl -fs https://server.yourdomain.com/health
  # Expected: {"status":"ok"}

  # Confirm IP is Cloudflare's, not yours:
  curl -s https://dns.google/resolve?name=server.yourdomain.com&type=A | jq '.Answer[].data'
  # Expected: Cloudflare IP range (104.x.x.x or 172.x.x.x — not your home IP)
```

---

### A6 — HOME_DEPLOY.md Guide

```
Context:
- The guide has been created at docs/guides/HOME_DEPLOY.md
- This task is to review the guide for completeness and update PHASES.md

Checklist for A6:
  [ ] docs/guides/HOME_DEPLOY.md covers all three tiers:
      Tier 1: DuckDNS + port forwarding + basic HTTP
      Tier 2: Traefik + Let's Encrypt
      Tier 2 Alt: Cloudflare Tunnel
  [ ] Security checklist present
  [ ] Backup instructions present
  [ ] Troubleshooting section covers: cert not issuing, DDNS not updating, CGNAT check
  [ ] All verify/test commands are correct
  [ ] Cross-references to DEPLOYMENT_OPTIONS.md and PHASES.md

Mark A1-A6 complete in docs/planning/PHASES.md.
Add CHANGELOG.md entry for Phase 25 Part A.

Suggested commit message:
  docs: Phase 25 Part A — home server deployment guide + Traefik compose config
```

---

## Part B — Hetzner + Coolify Cloud Deploy

> Proceed to Part B only if uptime SLA matters (home internet outages are unacceptable).
> Part B is the cloud alternative — same docker-compose.yml, different machine.

```
Context:
- Target: Hetzner CX32 (4 vCPU, 8 GB RAM, €8.99/month in Falkenstein/Helsinki)
- Coolify: open-source PaaS that runs on your VPS; deploys from docker-compose.yml directly
- GitHub webhook triggers redeploy on git push
- SSL via Let's Encrypt (Coolify handles this, same Traefik labels work)
- Coolify had 11 CVEs disclosed January 2026 — keep it updated

Task B1 — Provision Hetzner VPS:
  1. Create account at hetzner.com/cloud
  2. Create project → Add server: CX32, Ubuntu 24.04, Falkenstein or Helsinki
  3. Add your SSH public key during provisioning
  4. Note the server's public IPv4

Task B2 — Install Coolify:
  ssh root@<server-ip>
  curl -fsSL https://cdn.coolify.io/install.sh | bash
  # Coolify installs Docker, sets up its own Traefik, and starts on port 8000
  Visit: http://<server-ip>:8000 to complete initial setup

Task B3 — Deploy from GitHub:
  1. In Coolify: Sources → Add GitHub → Authorize
  2. Projects → Add Project → Add Resource → Docker Compose
  3. Select your groupscout repo and branch (main)
  4. Set all environment variables in Coolify UI (same as .env)
  5. Set domain names in Coolify UI (they map to Traefik labels automatically)
  6. Deploy

Verify B3:
  curl -fs https://server.yourdomain.com/health  → {"status":"ok"}
  docker ps on VPS → confirm app, alertd, postgres, n8n, prometheus, grafana all running

Task B4 — docs/guides/COOLIFY.md:
  Create guide covering: provision, install, connect repo, set env vars, set domains, deploy, update
```

---

## Part C — Event-Driven Ingestion (`POST /ingest`)

> Part C adds Go code. TDD applies strictly: write tests first, commit failing, then implement.

```
Context:
- Current pipeline: batch job runs all collectors → enriches all → notifies
- New: POST /ingest accepts a single RawProject payload → enriches it → stores it → optional notify
- Use case: n8n workflow finds a lead manually → posts to /ingest → appears in Slack immediately
- No full batch run needed
- Platform-agnostic: works on home server AND Hetzner/Railway
- Auth: same API_TOKEN Bearer auth as existing endpoints

Task C-T — write internal/enrichment/enricher_test.go additions FIRST:
  Add TestEnrichOne:
    Creates a mock LeadStore and Notifier
    Calls EnrichOne(ctx, rawProject) with a fixture RawProject
    Asserts: LeadStore.Insert called once
    Asserts: Notifier.Send called if priority_score >= threshold
    Asserts: returns error on Claude API failure (mock HTTP 500)
    All assertions must fail before EnrichOne() exists. Commit failing tests.

  Add TestIngestHandler:
    Uses httptest.NewServer
    POST /ingest with valid JSON body → expects 200 + {"id": "<uuid>"}
    POST /ingest with missing required fields → expects 400
    POST /ingest without Bearer token → expects 401
    POST /ingest with malformed JSON → expects 400
    All must fail before handler exists. Commit failing tests.

Task C1 — internal/enrichment/enricher.go:
  Add: func (e *Enricher) EnrichOne(ctx context.Context, raw collector.RawProject) (*storage.Lead, error)
    - Calls e.scorer.Score(raw) → skip if below threshold (same as batch pipeline)
    - Calls e.claude.Enrich(ctx, raw) → get EnrichedLead
    - Calls e.leads.Insert(ctx, lead) → persist
    - If priority_score >= config.PriorityAlertThreshold: calls e.notifier.Send(lead)
    - Returns lead, nil on success

Task C2 — cmd/server/main.go:
  Add: POST /ingest handler
    - Verify Bearer token (same middleware as /run, /digest)
    - Decode JSON body into collector.RawProject
    - Validate required fields: source, title (return 400 if missing)
    - Call enricher.EnrichOne(ctx, raw)
    - Return 200 {"id": "<lead.ID>", "priority_score": <score>}

Task C3 — api/swagger.yaml:
  Add /ingest endpoint:
    POST /ingest
    Security: bearerAuth
    RequestBody: RawProject schema (source, title, location, project_value, raw_data)
    Response 200: IngestResult schema (id, priority_score)
    Response 400: Error
    Response 401: Unauthorized

Verify C:
  # Valid ingest
  curl -X POST https://server.groupscout.duckdns.org/ingest \
    -H "Authorization: Bearer $API_TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"source":"manual","title":"Test Project","location":"Richmond BC","project_value":1000000,"raw_data":{}}'
  # Expected: {"id":"<uuid>","priority_score":<int>}

  # Missing auth
  curl -X POST https://server.groupscout.duckdns.org/ingest \
    -H "Content-Type: application/json" \
    -d '{}'
  # Expected: 401

  go test ./internal/enrichment/... ./cmd/server/...
  # Expected: all tests pass
```

---

## Part D — Terraform IaC for GCP (Optional)

> GCP Cloud Run is incompatible with `alertd` (scales to zero). If GCP is required,
> `alertd` must run on a Compute Engine VM — not Cloud Run. This adds ~$12–20/month.
> Only pursue Part D if there is a specific compliance or organizational requirement for GCP.

```
Context:
- Recommended path is Hetzner + Coolify (Part B) — 1/5 the cost
- GCP IaC adds complexity for no operational benefit at this scale
- If required: server → Cloud Run; alertd → Compute Engine e2-micro; Postgres → Cloud SQL (pgvector included)

Task D1 — terraform/main.tf:
  Provider: google
  Resources:
    google_cloud_run_service "server"    ← groupscout server binary
    google_compute_instance "alertd"     ← persistent VM for daemon
    google_sql_database_instance "db"    ← Cloud SQL Postgres with pgvector
    google_secret_manager_secret × N     ← one secret per env var

  Variables: project_id, region, image_tag, api_token, claude_api_key, slack_webhook_url

Task D2 — terraform/outputs.tf:
  output "server_url"  ← Cloud Run service URL
  output "alertd_ip"   ← Compute Engine external IP

Task D3 — docs/guides/GCP_TERRAFORM.md:
  Guide: gcloud auth, terraform init/plan/apply, set secrets, verify deployment
  Note prominently: alertd cannot run on Cloud Run; Compute Engine VM is required
```

---

*groupscout — group lodging demand intelligence*
*Sandman Hotel Vancouver Airport, Richmond BC*
