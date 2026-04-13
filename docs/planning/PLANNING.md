# PLANNING.md — groupscout

> Group lodging demand intelligence for hotel sales teams.
> GitHub: https://github.com/alvindcastro/groupscout

---

## Problem Statement

Hotels with strong group sales potential — like Sandman Hotel Vancouver Airport in Richmond, BC — miss high-value bookings because they wait for inbound RFPs. Construction crews, sports teams, film productions, and government contractors regularly need extended-stay room blocks, but they're rarely proactively targeted.

**groupscout flips that dynamic.** It monitors public data sources for signals that indicate incoming groups, enriches them with AI intelligence, and delivers prioritized leads to the sales team before competitors are even aware.

---

## Target Hotel (Phase 3)

**Sandman Hotel Vancouver Airport — Richmond, BC**

Key selling points for construction crews:
- Minutes from YVR — easy fly-in/fly-out for rotating crews
- Large parking lot for work trucks
- Shark Club on-site restaurant for meal plans / per-diem billing
- Extended stay weekly/monthly rates
- Direct company billing (critical for GCs)
- Early breakfast service

---

## Phased Roadmap

> **Current Status:** Phase 3 completed; transitioning to Advanced Audit & Intelligence.
> For the live big-picture roadmap, see [ROADMAP.md](./ROADMAP.md).
> For atomic task tracking, see [PHASES.md](./PHASES.md).

- [x] **Phase 1** — Manual Intelligence (Concept Validated)
- [x] **Phase 2** — Lightweight Automation (Concept Validated)
- [x] **Phase 3** — Go Backend (Current Foundation)
- [x] **Phase 15** — Postgres & pgvector Migration
- [x] **Phase 27** — Input Audit & Verification Trail
- [ ] **Phase 30** — Advanced Audit & Verification (Next)

---

## Phase 3 — Detailed Plan

### Overview

Build a Go backend that runs on a schedule, collects raw project data from public sources, enriches each project via Claude API, stores structured leads in a database, and delivers a Slack digest to the sales team.

**No UI in this phase. Slack is the interface.**

---

### Mini Phase A — Collector Layer

**Goal:** Pull raw construction project data from public sources into a normalized Go struct.

**Collector interface — all sources implement this:**
```go
type Collector interface {
    Name() string
    Collect(ctx context.Context) ([]RawProject, error)
}
```

#### A1 — Richmond Open Data Permit Scraper
- Source: City of Richmond open data portal (building permits)
- Filter: commercial/industrial, value > $500K
- Output: `RawProject` with address, type, value, issue date, description
- Schedule: nightly

#### A2 — Creative BC "In Production" Scraper ✅
- Source: Creative BC "In Production" list (Salesforce Visualforce page)
- Filter: Feature Films and TV Series (high crew potential)
- Output: Production title, studio, type, location, schedule
- Claude: Estimates crew size, hotel potential, and outreach timing
- Implementation: `internal/collector/creativebc.go`

#### A3 — BC Bid Contract Award Parser ✅
- Source: CivicInfo BC RSS feeds (automated) or manual HTML/JSON (n8n/trigger)
- Filter: construction and road maintenance awards
- Output: contract title, awardee (GC), value, region, award date
- Implementation: `internal/collector/bcbid.go`

#### A4 — Vancouver Convention Centre (VCC) Event Scraper ✅
- Source: VCC event calendar (HTML)
- Filter: Professional gatherings (conferences, summits, trade shows) vs. consumer events
- Output: Event name, dates, category
- Claude: Estimates attendee count and hotel night potential
- Implementation: `internal/collector/vcc.go`

#### A5 — Google News RSS Monitor
- Source: Google News RSS for curated queries:
  - `"Richmond BC" construction 2026`
  - `"Metro Vancouver" infrastructure contract awarded`
  - `"YVR" expansion project`
  - `"George Massey" construction`
- Output: title, URL, source, published date, snippet
- Schedule: daily

#### A6 — Announcement Page Scrapers
- Sources: BCIB project list, TransLink capital projects, YVR newsroom
- Detect new entries by comparing against previously seen URLs
- Output: title, URL, source, detected date
- Schedule: weekly

**Go packages:**
- `github.com/PuerkitoBio/goquery` — HTML parsing (used in VCC/Creative BC)
- `github.com/mmcdole/gofeed` — RSS/Atom parsing (used in BC Bid)
- `net/http` + `encoding/json` — API calls
- `os/exec` — Calling `pdftotext` for PDF parsing

### Mini Phase B — Enrichment Layer

**Goal:** Call Claude API per raw project, return structured lead intelligence.

#### B1 — Claude Enrichment
Input: project title, description, location, value, source
Output JSON:
```json
{
  "general_contractor": "string or unknown",
  "project_type": "civil|commercial|industrial|utility|residential|unknown",
  "estimated_crew_size": 80,
  "estimated_duration_months": 6,
  "out_of_town_crew_likely": true,
  "priority_score": 9,
  "priority_reason": "Large civil project 3km from hotel, groundbreaking imminent",
  "suggested_outreach_timing": "Reach out now — crews mobilizing in 4–6 weeks",
  "notes": "GC likely PCL or EllisDon based on project scale."
}
```

#### B2 — Rule-Based Pre-Scorer (Go-side, runs before Claude)
Fast filtering to avoid unnecessary Claude API calls:
- Richmond/YVR postal codes → priority bump
- Project value > $10M → large crew likely
- Keywords boost: `pipeline`, `civil`, `electrical`, `steel`, `concrete`
- Keywords deprioritize: `interior renovation`, `residential`, `landscaping`
- Skip Claude entirely if pre-score is below threshold

#### B3 — Deduplication
- Hash each raw record (source + external ID or title + date)
- Check DB before enriching — skip if already processed
- Update existing record if project status changed

#### B4 — Contact Enrichment (lightweight)
- Given GC name, generate a pre-filled LinkedIn search URL
- Flag as "suggested action" on the lead — human follows up manually
- No auto-scraping of contact info

**Go notes:**
- Use `anthropic-sdk-go` or raw `net/http` POST to Claude API
- Unmarshal Claude JSON response into `EnrichedLead` struct
- Retry with exponential backoff on API errors

---

### Mini Phase C — Storage Layer

**Goal:** Persist raw and enriched data. Simple schema. UI-ready for when that phase comes.

**Start with SQLite, migrate to Postgres for deployment.**

#### Schema

```sql
-- Raw ingested projects (append-only log)
CREATE TABLE raw_projects (
  id           UUID PRIMARY KEY,
  source       TEXT NOT NULL,
  external_id  TEXT,
  raw_data     JSONB NOT NULL,
  collected_at TIMESTAMPTZ NOT NULL,
  hash         TEXT UNIQUE NOT NULL
);

-- Enriched leads
CREATE TABLE leads (
  id                        UUID PRIMARY KEY,
  raw_project_id            UUID REFERENCES raw_projects(id),
  source                    TEXT,
  title                     TEXT,
  location                  TEXT,
  project_value             BIGINT,
  general_contractor        TEXT,
  project_type              TEXT,
  estimated_crew_size       INT,
  estimated_duration_months INT,
  out_of_town_crew_likely   BOOLEAN,
  priority_score            INT,
  priority_reason           TEXT,
  suggested_outreach_timing TEXT,
  notes                     TEXT,
  status                    TEXT DEFAULT 'new', -- new|contacted|proposal|booked|lost
  created_at                TIMESTAMPTZ NOT NULL,
  updated_at                TIMESTAMPTZ NOT NULL
);

-- Outreach activity log
CREATE TABLE outreach_log (
  id        UUID PRIMARY KEY,
  lead_id   UUID REFERENCES leads(id),
  contact   TEXT,
  channel   TEXT,    -- email|linkedin|phone
  notes     TEXT,
  outcome   TEXT,
  logged_at TIMESTAMPTZ NOT NULL
);
```

**Go packages:**
- `github.com/jackc/pgx` — PostgreSQL driver
- `github.com/mattn/go-sqlite3` — SQLite for local dev
- `golang-migrate/migrate` — schema migrations
- Repository pattern — DB logic behind interfaces, storage is swappable

---

### Mini Phase D — Notification Layer

**Goal:** Push actionable lead summaries to the sales team. No login required.

#### D1 — Weekly Slack Digest
- Every Monday at 8am, query leads from past 7 days where `priority_score >= 7`
- Slack Block Kit format:
```
🏗️ New Construction Leads This Week
──────────────────────────────────
1. Capstan Village Mixed-Use — Richmond
   GC: PCL Construction | Crew: ~80 | Score: 9/10
   Timing: Outreach now — groundbreaking in 6 weeks

2. BC Hydro Substation Upgrade — Delta
   GC: Unknown | Crew: ~30 | Score: 7/10
   Timing: Project starts Q3 — follow up in 4 weeks
```
- Delivered via Slack incoming webhook (no Slack app needed)

#### D2 — Instant Alert for Score ≥ 9
- Immediate Slack ping when a high-priority lead is detected
- Message: `🚨 High-priority lead: [title] — reach out before competitors do`

#### D3 — Email Digest (fallback)
- Same content as Slack, HTML email format
- SendGrid or AWS SES
- Weekly cadence, BCC to sales manager

#### D4 — Outreach Draft in Notification
- Claude generates a draft cold email per lead
- Included in the Slack/email digest
- Sales manager reads, tweaks, sends — no extra tool needed

---

### Build Order & Timeline

#### Week 1 — End-to-End Pipeline (One Source)
1. `A1` — Richmond permit scraper returning real data
2. `C` — DB schema + migrations + leads repo
3. `B1` — Claude enrichment for one permit record
4. Persist enriched lead to DB
5. `D1` — Slack message triggered manually showing one lead

**Goal: Full pipeline working with one source by end of week.**

#### Week 2 — Harden + Add Sources
1. `B3` — Deduplication
2. `A2` — BC Bid parser
3. `B2` — Rule-based pre-scorer
4. Scheduler wired — nightly collect, weekly digest
5. `D1` — Slack digest fully automated

**Goal: Real leads flowing nightly, digest hitting Slack every Monday.**

#### Week 3 — Remaining Sources + Notifications
1. `A3` — News RSS monitor
2. `A4` — BCIB/TransLink/YVR scrapers
3. `D2` — Instant high-priority alert
4. `D3/D4` — Email digest + outreach draft in notification

#### Week 4 — Productionize
1. Docker containerization
2. Env config hardening
3. Deploy to VPS or Railway
4. Smoke test all collectors end-to-end

**Total estimate: 6–8 weeks part-time**

---

## Project Structure

```
groupscout/
├── cmd/
│   └── server/
│       └── main.go
├── internal/
│   ├── collector/
│   │   ├── collector.go       # Collector interface
│   │   ├── richmond.go        # Richmond permit scraper
│   │   ├── bcbid.go           # BC Bid parser
│   │   ├── news.go            # RSS/news monitor
│   │   └── announcements.go   # BCIB/TransLink/YVR
│   ├── enrichment/
│   │   ├── claude.go          # Claude API client
│   │   ├── scorer.go          # rule-based pre-scorer
│   │   └── enricher.go        # orchestrates enrichment
│   ├── storage/
│   │   ├── db.go              # connection + migrations
│   │   ├── raw.go             # raw_projects repo
│   │   └── leads.go           # leads repo
│   ├── notify/
│   │   ├── slack.go           # Slack webhook
│   │   └── email.go           # SendGrid/SES
│   └── scheduler/
│       └── scheduler.go       # cron wiring
├── migrations/
│   ├── 001_init.up.sql
│   └── 001_init.down.sql
├── config/
│   └── config.go
├── CLAUDE.md
├── PLANNING.md
├── go.mod
├── go.sum
├── Dockerfile
└── README.md
```

---

## Environment Variables

```env
DATABASE_URL=postgres://...
CLAUDE_API_KEY=sk-ant-...
SLACK_WEBHOOK_URL=https://hooks.slack.com/...
SENDGRID_API_KEY=...
RICHMOND_PERMITS_URL=https://...
BCBID_RSS_URL=https://...
NEWS_API_KEY=...
ENRICHMENT_ENABLED=true
PRIORITY_ALERT_THRESHOLD=9
DIGEST_DAY=monday
DIGEST_HOUR=8
```

---

## Key Design Decisions

| Decision | Rationale |
|---|---|
| SQLite first, Postgres later | Zero-ops local dev; swap for production |
| Claude for enrichment only | Scraping stays deterministic; AI for interpretation |
| Rule-based pre-scorer | Filter noise before spending Claude API tokens |
| Slack as UI | Good digest = no dashboard needed yet |
| Manual outreach | AI drafts, human sends — no auto-send risk |
| Repository pattern | DB logic behind interfaces; storage is swappable |
| Collector interface | New sources are additive; core pipeline unchanged |
| 12-factor config | All secrets via env vars; easy to deploy anywhere |

---

## Future Segments (Phase 4+)

The Collector interface makes expansion purely additive:

| Segment | Data Source |
|---|---|
| Sports teams | Rogers Arena schedule, BC Lions, Vancouver FC fixtures |
| Film / TV crews | BC film permit registry |
| Government contractors | DND, CBSA, Transport Canada near YVR |
| Touring acts | Venue booking announcements |
| Conferences | Vancouver Convention Centre event calendar |

---

*groupscout — group lodging demand intelligence*
*Sandman Hotel Vancouver Airport, Richmond BC*
*Phase 3 — Go backend, no UI*
