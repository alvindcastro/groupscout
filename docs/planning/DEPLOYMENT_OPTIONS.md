# Deployment Options — groupscout

> Evaluated April 2026. Updated when pricing or platform changes warrant it.
>
> **Stack requirements:** Two always-on Go binaries (`server` + `alertd` daemon), PostgreSQL 17 + pgvector, optional: n8n, Redis, Prometheus, Grafana Loki.
>
> **Key constraint:** `alertd` polls every 10–90 seconds. It **cannot** run on a platform that scales to zero between requests (e.g. Cloud Run, Render free tier).

---

## TL;DR Recommendations

| Budget | Choice | Monthly Cost |
|---|---|---|
| **Completely free** | Home server + No-IP DDNS + port forwarding | $0 |
| Free, no port forwarding | Home server + Cloudflare Tunnel | $0 |
| Cheapest cloud full-stack | Hetzner CX32 + Coolify | ~$10 USD |
| Managed PaaS, no ops | Railway Hobby | ~$10–18 USD |
| Managed Postgres only | Neon Launch | $19 USD (DB only) |
| Truly free (risky) | Oracle Cloud Always Free | $0 (unreliable provisioning) |

---

## Platform Comparison

### 0. Home Server + Dynamic DNS ⭐ Completely Free

**Monthly cost:** $0

This is the simplest possible zero-cost path: run groupscout on a machine you already own, punch a hole in your router for the container port, and let a free dynamic DNS service keep the URL pointing to your current residential IP. The container itself is isolated — it has no access to your local files or other machines on the network.

**Tier 1 — Basic: No-IP + Port Forwarding**

```
Internet → Router (port forward 8080) → Docker host → groupscout container
```

1. Create a free [No-IP](https://www.noip.com) account → get a hostname like `groupscout.ddns.net` (free, 3 hostnames, requires a monthly confirmation click to stay active)
2. Install the No-IP DUC (dynamic update client) on the host machine — it updates the DNS record automatically when your residential IP changes
3. On your router: forward external port `8080` (or `443`) to the Docker host's LAN IP on port `8080`
4. In your `.env`: set `EXTERNAL_URL=http://groupscout.ddns.net:8080`
5. Run `docker compose up -d` — the container is now reachable from the internet

**Security:** Docker containers are isolated by default. The container cannot access your host filesystem, other LAN machines, or local services unless you explicitly bind-mount or expose them. For groupscout, the only exposure is the HTTP port for the API endpoints (`/run`, `/digest`, `/health`) — protected by your existing Bearer token auth.

**Alternative free DDNS providers:**
- **DuckDNS** — no monthly confirmation required; `groupscout.duckdns.org`; simpler setup
- **Cloudflare** — if you own a domain, Cloudflare's free plan handles dynamic DNS with a one-line API update script

---

**Tier 2 — Traefik Reverse Proxy (multiple containers, automatic SSL)**

Once you're comfortable with the basics, add Traefik as a reverse proxy in front of all your containers. This is the natural next step and unlocks:
- Automatic SSL via Let's Encrypt (HTTPS for free)
- Hidden ports — only 80 and 443 are exposed; Traefik routes internally
- Subdomain routing: `server.groupscout.ddns.net`, `n8n.groupscout.ddns.net`, `grafana.groupscout.ddns.net`
- All containers accessible through one entry point

Minimal `docker-compose.yml` addition:

```yaml
services:
  traefik:
    image: traefik:v3
    command:
      - "--providers.docker=true"
      - "--entrypoints.web.address=:80"
      - "--entrypoints.websecure.address=:443"
      - "--certificatesresolvers.letsencrypt.acme.email=you@example.com"
      - "--certificatesresolvers.letsencrypt.acme.storage=/letsencrypt/acme.json"
      - "--certificatesresolvers.letsencrypt.acme.tlschallenge=true"
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - "/var/run/docker.sock:/var/run/docker.sock:ro"
      - "letsencrypt:/letsencrypt"

  server:
    # ... your existing config
    labels:
      - "traefik.enable=true"
      - "traefik.http.routers.server.rule=Host(`server.groupscout.ddns.net`)"
      - "traefik.http.routers.server.entrypoints=websecure"
      - "traefik.http.routers.server.tls.certresolver=letsencrypt"
      - "traefik.http.services.server.loadbalancer.server.port=8080"
```

Each container gets its own subdomain via Docker labels — no manual nginx config, no port numbers in URLs. Let's Encrypt certificates are issued and renewed automatically.

| Feature | Support |
|---|---|
| pgvector | ✅ Full control |
| Two binaries (server + alertd) | ✅ Both run as persistent containers |
| Always-on daemon (alertd) | ✅ No scale-to-zero |
| Docker Compose | ✅ Native |
| n8n, Redis, Prometheus, Loki | ✅ All free, same machine |
| Automatic SSL | ✅ Traefik + Let's Encrypt |
| Managed backups | ❌ Manual |
| Uptime SLA | ❌ Home internet dependent |

**Caveats:**
- If your home internet goes down, the service goes down. Acceptable for an internal hotel sales tool.
- Some ISPs block inbound port 80/443. Use port 8443 or check your ISP's terms.
- Some ISPs use CGNAT (carrier-grade NAT) — port forwarding is impossible. Use Cloudflare Tunnel (below) as the fix.
- No-IP free tier requires a monthly "confirm hostname" click or the hostname is deleted. DuckDNS has no such requirement.

---

**Tier 2 Alternative — Cloudflare Tunnel (zero port forwarding, zero IP exposure)**

If your ISP uses CGNAT, blocks inbound ports, or you don't want your residential IP visible to the internet, Cloudflare Tunnel is the cleanest solution. It's completely free and requires no port forwarding.

```
Internet → Cloudflare Edge → Tunnel (outbound-only connection) → cloudflared container → groupscout container
```

1. Create a free Cloudflare account and add your domain (or use a free subdomain service)
2. Add a `cloudflared` container to your `docker-compose.yml`:
   ```yaml
   cloudflared:
     image: cloudflare/cloudflared:latest
     command: tunnel --no-autoupdate run --token ${CLOUDFLARE_TUNNEL_TOKEN}
   ```
3. Create a tunnel in the Cloudflare Zero Trust dashboard → get a token → set `CLOUDFLARE_TUNNEL_TOKEN` in `.env`
4. Map your hostname to `http://server:8080` in the Cloudflare dashboard
5. No router config needed. No open ports. Your residential IP is never exposed.

Cloudflare Tunnel is strictly better than raw port forwarding on every security axis. The only trade-off is a dependency on Cloudflare's free tier (which has been free and stable for years).

---

### 1. Hetzner Cloud + Coolify ⭐ Best Cloud Value

**Monthly cost:** ~€8.99 (~$10 USD) for CX32 (4 vCPU, 8 GB RAM)

| Feature | Support |
|---|---|
| pgvector | ✅ Full control — use `pgvector/pgvector:pg17` image |
| Two binaries (server + alertd) | ✅ Both run as persistent Docker containers |
| Always-on daemon (alertd) | ✅ Persistent container, no scale-to-zero |
| Docker Compose | ✅ Native — Coolify deploys your existing `docker-compose.yml` directly |
| n8n, Redis, Prometheus, Loki | ✅ Run as additional Coolify services at no extra cost |
| Managed DB backups | ❌ Manual (Coolify has a backup UI, but you manage storage) |
| Zero ops | ❌ You own the VPS (OS patches, Coolify updates) |

**How it works:**
- Provision a Hetzner CX32 (~€8.99/month in Falkenstein or Helsinki)
- Install Coolify on it (one `curl | bash` command)
- Add your GitHub repo → Coolify detects `docker-compose.yml` → deploys all services
- SSL via Let's Encrypt, automatic. Deploy on `git push` via GitHub webhook.

**Why it wins:**
- Your existing `docker-compose.yml` deploys as-is — zero reconfiguration
- n8n, Redis, Prometheus, Grafana Loki all run free on the same box
- `alertd` daemon runs as a persistent container with no platform constraints
- Total cost is a single ~$10/month line item

**Caveats:**
- Coolify had 11 CVEs disclosed January 2026 — keep it updated
- You are responsible for the VPS (Ubuntu updates, firewall, backups)
- Single server = single point of failure (acceptable for an internal tool)

**Setup steps:**
1. Create Hetzner account → provision CX32 (Ubuntu 24.04)
2. SSH in → `curl -fsSL https://cdn.coolify.io/install.sh | bash`
3. Visit `http://<server-ip>:8000` → complete Coolify setup
4. Add GitHub repo → deploy from `docker-compose.yml`
5. Set env vars in Coolify UI → done

---

### 2. Railway ⭐ Best Managed PaaS

**Monthly cost:** ~$10–18 USD (Hobby $5/month flat + usage)

| Feature | Support |
|---|---|
| pgvector | ✅ Pre-installed in Railway's Postgres image |
| Two binaries (server + alertd) | ✅ Separate Railway Services |
| Always-on daemon (alertd) | ✅ Background Worker service type |
| Docker Compose | ✅ Railway can deploy from `docker-compose.yml` with some mapping |
| n8n, Redis, Prometheus, Loki | ⚠️ Possible but adds cost per service |
| Managed DB backups | ✅ Automatic |
| Zero ops | ✅ Fully managed |

**How it works:**
- Create a Railway project → add GitHub repo
- Railway detects your Dockerfile(s) and deploys each service
- Add a Postgres database service (pgvector pre-installed)
- `server` runs as a Web Service; `alertd` runs as a Background Worker
- Native Cron Job service for the weekly digest trigger

**Why it's good:**
- No VPS to manage
- pgvector works out of the box — no custom Docker image needed
- Native cron support means you can drop the n8n dependency for scheduling

**Caveats:**
- n8n, Prometheus, Loki would each be additional Railway services, adding cost
- Hobby plan has a $5 usage credit — light usage stays at $5–10/month; heavier usage bills beyond that
- No Docker Compose native mapping — you configure services individually in the UI/YAML

---

### 3. Fly.io

**Monthly cost:** ~$15–25 USD (two machines + self-hosted Postgres + volumes + IPv4)

| Feature | Support |
|---|---|
| pgvector | ✅ Self-hosted Postgres machine only; Managed Postgres ($38/mo) does not include it |
| Two binaries (server + alertd) | ✅ Separate Fly Machines |
| Always-on daemon (alertd) | ✅ Machine stays up if configured correctly |
| Docker Compose | ⚠️ Not supported — each service is a separate `fly.toml` |
| n8n, Redis, Prometheus, Loki | ⚠️ Each is a separate Machine with separate billing |
| Managed DB backups | ❌ Only on Managed Postgres ($38/month); self-hosted is manual |

**Summary:** Capable but cost escalates. Managed Postgres at $38/month is too expensive. Self-hosted Postgres on a Fly Machine brings cost down but adds ops. Railway is a cleaner managed alternative at this budget.

---

### 4. Render

**Monthly cost:** ~$21–35 USD

| Feature | Support |
|---|---|
| pgvector | ✅ Via `CREATE EXTENSION` |
| Two binaries | ✅ Web Service + Background Worker |
| Always-on daemon | ✅ (paid plans only; free tier spins down) |
| Docker Compose | ❌ Not supported |
| n8n, Redis, Prometheus, Loki | ❌ / ⚠️ Not natively |

**Summary:** Clean UX, solid Postgres, but more expensive than Railway for equivalent capability. Not recommended given the budget target.

---

### 5. Hetzner + Kamal

**Monthly cost:** ~$5–12 USD (VPS cost only; Kamal is free)

| Feature | Support |
|---|---|
| pgvector | ✅ Full control via Docker image |
| Two binaries | ✅ Kamal v2 supports multiple roles |
| Always-on daemon | ✅ Persistent container |
| Docker Compose | ❌ Kamal has its own accessory model |
| n8n, Redis, Prometheus, Loki | ⚠️ Kamal accessories — more manual config |
| UI | ❌ CLI-only |

**Summary:** Solid choice if you prefer CLI over a PaaS UI and want zero-downtime deploys via Kamal Proxy. More manual than Coolify but no UI dependency. Works well for Go-only deployments. The Docker Compose preference makes Coolify easier.

---

### 6. Google Cloud Run + Cloud SQL ⚠️ Incompatible with alertd

**Monthly cost:** ~$30–60 USD

| Feature | Support |
|---|---|
| pgvector | ✅ Cloud SQL supports it |
| Two binaries | ⚠️ server runs on Cloud Run; alertd needs Compute Engine VM |
| Always-on daemon (alertd) | ❌ **Cloud Run scales to zero** — fundamentally incompatible |
| Docker Compose | ❌ Cloud Run is container-per-service, no Compose |

**Why it's not recommended:**
- `alertd` polls every 10–90 seconds. Cloud Run bills per request and scales to zero between invocations. A polling daemon cannot run on Cloud Run without significant cost and architectural workarounds.
- To run `alertd` on GCP you would need a Compute Engine VM (~$12–20/month additional) plus Cloud Run for `server` plus Cloud SQL — pushing total to $50+/month.
- The existing Terraform plans (Phase 25) would need significant revision to add a persistent compute resource for `alertd`.

**Recommendation:** Defer GCP unless you have specific compliance, existing credits, or organizational requirements. Hetzner + Coolify at $10/month serves the same purpose at 1/5 the cost.

---

### 7. Oracle Cloud Always Free

**Monthly cost:** $0

| Feature | Support |
|---|---|
| pgvector | ✅ Full Docker control (ARM64 builds needed) |
| Two binaries | ✅ 4 OCPU / 24 GB RAM |
| Always-on daemon | ✅ Persistent VM |
| Docker Compose | ✅ |

**Summary:** Tempting but unreliable. ARM instance provisioning is a lottery (capacity constantly exhausted). Oracle has been known to terminate "idle" free accounts. Go binaries need `GOARCH=arm64` cross-compilation. Good for a staging environment if you already have access, not as a primary target.

---

### 8. Neon (Managed Postgres only)

**Monthly cost:** $0 (free, 0.5 GB) or $19/month (Launch, 10 GB)

| Feature | Support |
|---|---|
| pgvector | ✅ v0.8.1, all plans |
| Serverless scale-to-zero | ⚠️ DB wakes on query (~100–300ms cold start) |
| Branching | ✅ Great for dev/staging |

**Use case:** Pair Neon with a Hetzner CX22 (~$5/month) for compute and Neon Launch ($19/month) for managed Postgres. Total ~$24/month. Gets you managed backups, pgvector, and DB branching without running your own Postgres. Slight overkill for this volume but operationally clean.

---

## Decision Matrix

| Requirement | Home + DDNS | Home + CF Tunnel | Hetzner + Coolify | Railway | GCP Cloud Run |
|---|---|---|---|---|---|
| alertd always-on | ✅ | ✅ | ✅ | ✅ | ❌ |
| pgvector | ✅ | ✅ | ✅ | ✅ | ✅ |
| Docker Compose | ✅ | ✅ | ✅ | ⚠️ | ❌ |
| n8n / Prometheus / Loki | ✅ free | ✅ free | ✅ free | ⚠️ extra | ❌ |
| Managed backups | ❌ manual | ❌ manual | ⚠️ manual | ✅ | ✅ |
| Residential IP exposed | ✅ yes | ❌ hidden | N/A | N/A | N/A |
| Monthly cost | $0 | $0 | ~$10 | ~$10–18 | ~$30–60 |
| Ops required | Low | Low | Low (VPS) | None | Medium (Terraform) |

---

## Deployment Progression

The natural growth path for this project:

```
Stage 1: Home server + No-IP DDNS + port forwarding       → $0, works immediately
Stage 2: Add Traefik + Let's Encrypt                       → $0, proper HTTPS + subdomain routing
Stage 2 alt: Replace port forwarding with Cloudflare Tunnel → $0, no IP exposure, no CGNAT issues
Stage 3: Move to Hetzner CX32 + Coolify                   → ~$10/month, if uptime SLA matters
Stage 4: Railway (managed)                                 → ~$15/month, if you want zero ops
```

groupscout is an internal tool for a small hotel sales team. Stage 1 or 2 is a perfectly valid long-term home if you have a machine running at home (an old laptop, a Raspberry Pi 4/5, or a spare PC).

---

## Chosen Path: Hetzner CX32 + Coolify (cloud) or Home Server + Traefik (free)

Both paths share the same `docker-compose.yml`. The only difference is where the machine lives.

**References:**
- Hetzner Cloud: https://www.hetzner.com/cloud
- Coolify: https://coolify.io
- Railway: https://railway.com/pricing
- Neon: https://neon.com/pricing
- Fly.io: https://fly.io/pricing
