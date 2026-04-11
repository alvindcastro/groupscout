# HOME_DEPLOY.md — Home Server Deployment Guide

> **Goal:** Make groupscout reachable from the internet at zero cost, running on a machine you already own.
>
> **Stack:** DuckDNS (dynamic DNS) → Router port forwarding OR Cloudflare Tunnel → Traefik v3 (reverse proxy + Let's Encrypt TLS) → Docker Compose (groupscout + alertd + Postgres).

---

## Prerequisites

- A machine running Docker and Docker Compose (Linux preferred; Windows with WSL2 works)
- Your existing `docker-compose.yml` already runs locally
- A free [DuckDNS](https://www.duckdns.org) account (sign in with GitHub/Google)
- A domain token from DuckDNS (shown on your dashboard after login)
- Router admin access (for port forwarding path only)

---

## Tier 1 — DDNS + Port Forwarding + HTTP (15 minutes)

The simplest path. No TLS yet, but proves the machine is reachable.

### Step 1 — Register a DuckDNS hostname

1. Log in at [duckdns.org](https://www.duckdns.org)
2. Create a subdomain: e.g. `groupscout` → your hostname becomes `groupscout.duckdns.org`
3. Copy your **token** from the dashboard (UUID format)

### Step 2 — Install the DuckDNS updater

The updater runs as a cron job that pings DuckDNS whenever your IP changes.

**Linux (cron method):**

```bash
# Create updater script
mkdir -p ~/duckdns
cat > ~/duckdns/duck.sh << 'EOF'
echo url="https://www.duckdns.org/update?domains=groupscout&token=YOUR_TOKEN&ip=" | curl -k -o ~/duckdns/duck.log -K -
EOF
chmod +x ~/duckdns/duck.sh

# Run every 5 minutes
(crontab -l 2>/dev/null; echo "*/5 * * * * ~/duckdns/duck.sh >/dev/null 2>&1") | crontab -
```

Replace `YOUR_TOKEN` with your DuckDNS token. Run `~/duckdns/duck.sh` once manually and check `cat ~/duckdns/duck.log` — it should print `OK`.

**Docker method (alternative):**

```yaml
# Add to docker-compose.yml
  duckdns:
    image: lscr.io/linuxserver/duckdns:latest
    environment:
      - SUBDOMAINS=groupscout
      - TOKEN=${DUCKDNS_TOKEN}
    restart: unless-stopped
```

Add `DUCKDNS_TOKEN=your_token` to your `.env`.

### Step 3 — Port forward on your router

1. Find your Docker host's LAN IP: `hostname -I | awk '{print $1}'` (e.g. `192.168.1.42`)
2. Log in to your router admin panel (usually `192.168.1.1` or `192.168.0.1`)
3. Navigate to Port Forwarding / NAT / Virtual Servers
4. Add two rules:
   - External port `80` → Internal IP `192.168.1.42` port `80`
   - External port `443` → Internal IP `192.168.1.42` port `443`
5. Save and apply

> **Can't port forward?** Your ISP may use CGNAT. Check: if your router's WAN IP starts with `100.64.x.x` or `10.x.x.x`, you're behind CGNAT. Skip to [Tier 2 Alt — Cloudflare Tunnel](#tier-2-alt--cloudflare-tunnel-cgnat--no-port-forwarding).

### Step 4 — Verify basic connectivity

Temporarily expose port 8080 directly (no Traefik yet):

```yaml
# In docker-compose.yml, ensure app has:
ports:
  - "8080:8080"
```

```bash
docker compose up -d app postgres
curl http://groupscout.duckdns.org:8080/health
# Expected: {"status":"ok"} or similar
```

If this returns a response, DDNS and port forwarding are working. Proceed to Tier 2 for HTTPS.

---

## Tier 2 — Traefik + Let's Encrypt (HTTPS + Subdomain Routing)

Adds automatic TLS and hides port numbers. Every container gets its own subdomain via Docker labels.

### Step 5 — Update docker-compose.yml

Replace your current `docker-compose.yml` with the Traefik-enabled version below. Key changes:
- Add `traefik` service
- Remove direct port bindings from `app` and `alertd` (Traefik routes internally)
- Add `labels:` to `app` and `alertd`
- Add `letsencrypt` volume for cert storage

```yaml
services:
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

  app:
    build: .
    # No ports: section — Traefik handles routing
    depends_on:
      postgres:
        condition: service_healthy
    environment:
      - DATABASE_URL=postgres://groupscout:groupscout@postgres:5432/groupscout
      - CLAUDE_API_KEY=${CLAUDE_API_KEY}
      - SLACK_WEBHOOK_URL=${SLACK_WEBHOOK_URL}
      - API_TOKEN=${API_TOKEN}
      - RICHMOND_PERMITS_URL=${RICHMOND_PERMITS_URL}
      - DELTA_PERMITS_URL=${DELTA_PERMITS_URL}
      - CREATIVEBC_ENABLED=true
      - VCC_ENABLED=true
      - BCBID_ENABLED=true
      - NEWS_ENABLED=true
      - ANNOUNCEMENTS_ENABLED=true
      - EVENTBRITE_ENABLED=true
      - MIN_PERMIT_VALUE_CAD=500000
    labels:
      - "traefik.enable=true"
      - "traefik.http.routers.app.rule=Host(`server.groupscout.duckdns.org`)"
      - "traefik.http.routers.app.entrypoints=websecure"
      - "traefik.http.routers.app.tls.certresolver=letsencrypt"
      - "traefik.http.services.app.loadbalancer.server.port=8080"
    restart: unless-stopped

  alertd:
    build: .
    command: ["./alertd"]
    # No ports: section — Traefik handles routing
    env_file: .env
    volumes:
      - ./config:/app/config:ro
    depends_on:
      postgres:
        condition: service_healthy
    labels:
      - "traefik.enable=true"
      - "traefik.http.routers.alertd.rule=Host(`alertd.groupscout.duckdns.org`)"
      - "traefik.http.routers.alertd.entrypoints=websecure"
      - "traefik.http.routers.alertd.tls.certresolver=letsencrypt"
      - "traefik.http.services.alertd.loadbalancer.server.port=8081"
    restart: unless-stopped

  postgres:
    image: pgvector/pgvector:pg17
    environment:
      - POSTGRES_DB=groupscout
      - POSTGRES_USER=groupscout
      - POSTGRES_PASSWORD=${POSTGRES_PASSWORD:-groupscout}
    volumes:
      - pgdata:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U groupscout"]
      interval: 10s
      timeout: 5s
      retries: 5
    restart: unless-stopped

  n8n:
    image: n8nio/n8n:latest
    env_file: .env
    environment:
      - N8N_HOST=n8n.groupscout.duckdns.org
      - N8N_PORT=5678
      - N8N_PROTOCOL=https
      - WEBHOOK_URL=https://n8n.groupscout.duckdns.org/
    volumes:
      - n8n_data:/home/node/.n8n
    labels:
      - "traefik.enable=true"
      - "traefik.http.routers.n8n.rule=Host(`n8n.groupscout.duckdns.org`)"
      - "traefik.http.routers.n8n.entrypoints=websecure"
      - "traefik.http.routers.n8n.tls.certresolver=letsencrypt"
      - "traefik.http.services.n8n.loadbalancer.server.port=5678"
    restart: unless-stopped

  prometheus:
    image: prom/prometheus:latest
    volumes:
      - prometheus_data:/prometheus
    labels:
      - "traefik.enable=true"
      - "traefik.http.routers.prometheus.rule=Host(`prometheus.groupscout.duckdns.org`)"
      - "traefik.http.routers.prometheus.entrypoints=websecure"
      - "traefik.http.routers.prometheus.tls.certresolver=letsencrypt"
      - "traefik.http.services.prometheus.loadbalancer.server.port=9090"
    restart: unless-stopped

  grafana:
    image: grafana/grafana:latest
    volumes:
      - grafana_data:/var/lib/grafana
    labels:
      - "traefik.enable=true"
      - "traefik.http.routers.grafana.rule=Host(`grafana.groupscout.duckdns.org`)"
      - "traefik.http.routers.grafana.entrypoints=websecure"
      - "traefik.http.routers.grafana.tls.certresolver=letsencrypt"
      - "traefik.http.services.grafana.loadbalancer.server.port=3000"
    restart: unless-stopped

  loki:
    image: grafana/loki:latest
    restart: unless-stopped

volumes:
  pgdata:
  n8n_data:
  prometheus_data:
  grafana_data:
  letsencrypt:
```

### Step 6 — Add required env vars

Add to your `.env`:

```env
# Traefik Let's Encrypt
ACME_EMAIL=your@email.com

# DuckDNS (if using Docker updater method)
DUCKDNS_TOKEN=xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx

# Postgres (use a real password in production)
POSTGRES_PASSWORD=change_me_in_production
```

### Step 7 — Validate compose file before deploying

```bash
docker compose config --quiet
# Must exit 0 (no output = valid)
```

### Step 8 — Deploy

```bash
docker compose up -d
# Watch Traefik request the cert (first boot only, takes ~10s)
docker compose logs traefik -f --tail=50
```

Look for: `Obtained certificate from ACME provider` in Traefik logs.

### Step 9 — Verify (A4 gate)

```bash
# Health check — must return {"status":"ok"} over HTTPS
curl -fs https://server.groupscout.duckdns.org/health

# Verify TLS certificate issuer
curl -sv https://server.groupscout.duckdns.org/health 2>&1 | grep "issuer"
# Expected: issuer: C=US, O=Let's Encrypt, CN=R...

# Verify HTTP redirects to HTTPS
curl -Iv http://server.groupscout.duckdns.org/health 2>&1 | grep "301\|Location"
# Expected: 301 redirect to https://...
```

All three must pass before proceeding to A5 or calling Part A complete.

---

## Tier 2 Alt — Cloudflare Tunnel (CGNAT / No Port Forwarding)

Use this instead of port forwarding if:
- Your ISP uses CGNAT (router WAN IP is `100.64.x.x` or `10.x.x.x`)
- Your ISP blocks inbound ports 80/443
- You don't want your residential IP visible in DNS records

### How it works

```
Internet → Cloudflare Edge → Tunnel (outbound-only) → cloudflared container → Traefik → app / alertd
```

Your machine makes an outbound connection to Cloudflare. No ports need to be opened on your router. Your IP is never exposed.

### Prerequisites

- A domain you own added to Cloudflare (free plan works), OR use a Cloudflare Pages subdomain
- A [Cloudflare Zero Trust](https://one.dash.cloudflare.com) account (free)

### Step 1 — Create a tunnel

1. Go to Cloudflare Zero Trust → Networks → Tunnels → Create a tunnel
2. Name it `groupscout`
3. Copy the tunnel token (long string starting with `eyJ...`)
4. Set `CLOUDFLARE_TUNNEL_TOKEN=<token>` in your `.env`
5. In the tunnel's Public Hostnames tab, add:
   - `server.yourdomain.com` → `http://app:8080`
   - `alertd.yourdomain.com` → `http://alertd:8081`
   - `n8n.yourdomain.com` → `http://n8n:5678`

### Step 2 — Add cloudflared to docker-compose.yml

```yaml
  cloudflared:
    image: cloudflare/cloudflared:latest
    command: tunnel --no-autoupdate run --token ${CLOUDFLARE_TUNNEL_TOKEN}
    restart: unless-stopped
```

With Cloudflare Tunnel, you **do not need Traefik** for TLS — Cloudflare terminates TLS at the edge and proxies HTTP internally. You can remove the Traefik service and the `labels:` from your containers if using this path.

### Step 3 — Verify

```bash
curl -fs https://server.yourdomain.com/health
# Expected: {"status":"ok"}
```

---

## Security Checklist

Before going live, confirm:

- [ ] `API_TOKEN` is set to a strong random string: `openssl rand -hex 32`
- [ ] `POSTGRES_PASSWORD` is not the default `groupscout`
- [ ] Traefik dashboard is not exposed publicly (no router label for it)
- [ ] Prometheus and Grafana are behind at minimum an IP allowlist or Cloudflare Access rule
- [ ] `.env` is in `.gitignore` and never committed
- [ ] Docker socket is mounted read-only (`:ro`) in Traefik's volumes

---

## Backup

Postgres data lives in the `pgdata` Docker volume. Back it up with:

```bash
# Dump to a file
docker exec groupscout-postgres-1 pg_dump -U groupscout groupscout > backup_$(date +%Y%m%d).sql

# Restore from a file
cat backup_20260410.sql | docker exec -i groupscout-postgres-1 psql -U groupscout groupscout
```

Run this weekly via cron:

```bash
0 2 * * 0 cd /path/to/groupscout && docker exec groupscout-postgres-1 pg_dump -U groupscout groupscout > backups/backup_$(date +\%Y\%m\%d).sql
```

---

## Troubleshooting

**Cert not issuing:**
- Confirm port 443 is forwarded and `curl -v telnet://groupscout.duckdns.org:443` reaches your machine
- Check Traefik logs: `docker compose logs traefik | grep -i acme`
- Let's Encrypt rate limits: 5 failed cert requests per hostname per hour — wait 1 hour if you see `too many certificates` errors

**DuckDNS not updating:**
- Run `~/duckdns/duck.sh` manually and check `duck.log` — should print `OK`
- Verify your external IP: `curl ifconfig.me` — compare to DuckDNS dashboard

**CGNAT check:**
```bash
# If your router WAN IP is private, you're behind CGNAT
# Compare: router WAN IP vs. your actual public IP
curl ifconfig.me
# If these differ (and router shows 100.64.x.x / 10.x.x.x), you're on CGNAT
```

**Alertd not routing:**
- Confirm `alertd` container is running: `docker compose ps`
- Confirm Traefik sees it: `docker compose logs traefik | grep alertd`
- Test internal routing: `docker compose exec traefik wget -qO- http://alertd:8081/health`

---

## Reference

- Deployment options comparison: `docs/planning/DEPLOYMENT_OPTIONS.md`
- Moving to cloud (Hetzner + Coolify): see Phase 25 Part B in `docs/planning/PHASES.md`
- Architecture overview: `docs/ARCHITECTURE.md`
- DuckDNS setup guide: https://www.duckdns.org/install.jsp

---

*groupscout — group lodging demand intelligence*
*Sandman Hotel Vancouver Airport, Richmond BC*
