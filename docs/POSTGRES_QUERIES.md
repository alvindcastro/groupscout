# GroupScout — PostgreSQL Query Guide

How to connect to and query the Postgres database when running via Docker Compose.

---

## Connect with DataGrip

1. Open DataGrip → **New Data Source** → **PostgreSQL**
2. Fill in the connection details:

| Field | Value |
|---|---|
| Host | `localhost` |
| Port | `5432` |
| Database | `groupscout` |
| User | `groupscout` |
| Password | `groupscout` |

3. Click **Test Connection** — should show "Successful"
4. Click **OK**

> These credentials match the defaults in `docker-compose.yml`. If you changed them in `.env`, use those values instead.

> **Driver:** DataGrip will prompt you to download the PostgreSQL driver on first use — click Download.

---

## Connect via psql

```bash
docker exec -it groupscout-postgres-1 psql -U groupscout -d groupscout
```

Exit with `\q`.

---

## Useful psql meta-commands

| Command | What it does |
|---|---|
| `\dt` | List all tables |
| `\d leads` | Describe the leads table (columns + types) |
| `\x` | Toggle expanded (vertical) row display — useful for wide rows |
| `\q` | Quit |

---

## Leads

### All leads, newest first
```sql
SELECT id, source, title, priority_score, status, created_at
FROM leads
ORDER BY created_at DESC
LIMIT 20;
```

### High-priority leads (score ≥ 7)
```sql
SELECT title, source, priority_score, priority_reason, status
FROM leads
WHERE priority_score >= 7
ORDER BY priority_score DESC;
```

### Leads by status
```sql
-- new leads only
SELECT title, source, priority_score, created_at
FROM leads
WHERE status = 'new'
ORDER BY priority_score DESC;

-- skipped by pre-scorer
SELECT title, source, priority_score, priority_reason
FROM leads
WHERE status = 'skipped'
ORDER BY created_at DESC;
```

### Leads from a specific source
```sql
SELECT title, priority_score, status, created_at
FROM leads
WHERE source = 'bcbid'
ORDER BY created_at DESC;
```

### Full lead detail (use \x first for readable output)
```sql
\x
SELECT * FROM leads WHERE title ILIKE '%your search term%';
\x
```

### Lead counts by source
```sql
SELECT source, status, COUNT(*) AS total
FROM leads
GROUP BY source, status
ORDER BY source, status;
```

### Leads from the last 7 days
```sql
SELECT title, source, priority_score, status, created_at
FROM leads
WHERE created_at >= NOW() - INTERVAL '7 days'
ORDER BY created_at DESC;
```

---

## Raw Projects

### Recent raw project ingests
```sql
SELECT source, external_id, collected_at
FROM raw_projects
ORDER BY collected_at DESC
LIMIT 20;
```

### Count by source
```sql
SELECT source, COUNT(*) AS total
FROM raw_projects
GROUP BY source
ORDER BY total DESC;
```

### Check if a specific hash exists (dedup check)
```sql
SELECT id, source, external_id, collected_at
FROM raw_projects
WHERE hash = 'your_hash_here';
```

---

## Outreach Log

```sql
SELECT l.title, o.contact, o.channel, o.outcome, o.logged_at
FROM outreach_log o
JOIN leads l ON l.id = o.lead_id
ORDER BY o.logged_at DESC;
```

---

## Schema Migrations

Check which migrations have been applied:
```sql
SELECT version, dirty FROM schema_migrations;
```

---

## Wipe and reset (dev only)

Drops all data and re-runs migrations on next app start:
```bash
docker compose down --volumes
docker compose up -d
```
