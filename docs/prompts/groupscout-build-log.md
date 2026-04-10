# Building groupscout — A Lead Intelligence Tool for Hotel Group Sales

> A build log covering the first working pipeline: Richmond building permits → Claude AI enrichment → Slack digest.

---

## The Problem

By day I'm a Programmer Analyst at Douglas College. My wife is a Hotel Sales Manager at the Sandman Hotel Vancouver Airport in Richmond, BC — and I built this tool for her team.

One of the most valuable but hardest-to-find business segments in hotel sales is **construction crew lodging**. When a major project breaks ground nearby — a warehouse, a hotel renovation, a commercial development — the general contractor needs somewhere to house their crew. That's often 20–100+ workers for 3–12 months. Extended stay rates, direct billing, weekly rooms. It's exactly the kind of business a hotel near an industrial area should be chasing.

The problem: by the time you find out about a project, crews are already booked somewhere else.

The signal is public — building permits are published by the City of Richmond every week. The data is there. Nobody's reading it systematically.

So I built **groupscout**: a tool that reads Richmond's weekly building permit reports, filters for the permits that matter, enriches them with AI to score and summarize each lead, and delivers a prioritized digest to Slack every week.

---

## The Stack

I built this in Go. I know Go well and it's the right tool for a background data pipeline — fast, simple, no framework overhead.

| Layer | What I used | Why |
|---|---|---|
| Language | Go 1.26.1 | Know it well; great for pipelines |
| Database | SQLite (`modernc.org/sqlite`) | Zero ops for local dev, pure Go so no C compiler on Windows |
| AI enrichment | Claude Messages API (raw `net/http`) | Haiku is cheap (~$0.001/permit); Sonnet available as upgrade |
| notifications | Slack + SendGrid Email | Slack for instant alerts; SendGrid for weekly HTML digests |
| Logging | Structured Logging (`slog`) | Contextual, JSON/Text toggles, production-grade observability |
| Deployment | Docker & Docker Compose | Containerized for production readiness |
| Config | `.env` file + environment variables | 12-factor: no secrets in code |

No framework, no ORM, no generated code. Just `database/sql`, `net/http`, and the standard library.

---

## Architecture

The design is a simple four-layer pipeline:

```
Collector Layer    — scrape / parse raw permit data
       ↓
Enrichment Layer   — Claude API: score, classify, summarize
       ↓
Storage Layer      — SQLite: dedup + persist raw records and leads
       ↓
Notification Layer — Slack digest: Block Kit card per lead
```

Each layer has a clean interface. The `Collector` interface is the key one:

```go
type Collector interface {
    Name() string
    Collect(ctx context.Context) ([]RawProject, error)
}
```

Adding a new data source (BC Bid contracts, sports team schedules, film permits) means implementing this interface. The pipeline never changes.

---

## Phase 1: Foundation

Before writing any scraper, I built the storage layer. The logic: if the pipeline fails halfway, I don't want to lose data. Raw records go into the DB *before* enrichment, not after.

Three tables:

- `raw_projects` — append-only log of every permit collected, with a SHA-256 dedup hash
- `leads` — enriched, scored records ready for the sales team
- `outreach_log` — manual activity tracking (future use)

The dedup hash is the important piece. Every permit gets a deterministic hash from its folder number + address + issue date. On each run, before spending a Claude API call, the pipeline checks `ExistsByHash`. Same permit appearing in next week's report? Already in the DB, skip it.

I chose `modernc.org/sqlite` over the more common `mattn/go-sqlite3`. The difference: `modernc` is pure Go — no CGO, no need for a C compiler on Windows. Same API, zero friction on my Windows dev machine.

---

## Phase 2: The First Real Pipeline

### Part A — Scraping Richmond's Building Permits

Richmond BC publishes weekly building permit reports. There's no API. The data is in PDFs, published at a known URL.

Step one: scrape the reports page to find the latest PDF link. The HTML is consistent enough that a single regex does the job:

```go
var buildingReportRe = regexp.MustCompile(`/__shared/assets/buildingreport[^"]+\.pdf`)
```

Step two: download the PDF to a temp file, extract text, parse it into permit records.

For PDF text extraction, I initially tried `github.com/ledongthuc/pdf` — a pure Go PDF library. It returned zero rows. Richmond's PDFs use a font encoding the library can't decode. Classic.

The fix: shell out to `pdftotext` (part of the Poppler library, bundled with Git for Windows):

```go
out, err := exec.Command(pdftotext, path, "-").Output()
```

That one line fixed everything. `pdftotext` handles the encoding correctly and produces clean plain text.

### The Parser Bug That Looked Like a Config Bug

After switching to `pdftotext`, the collector ran, parsed 20 permit records, and zero passed the value filter. Every permit had `ValueCAD = 1`.

I thought it was a threshold configuration issue. Lowered the minimum to $100,000. Still zero. Added verbose logging to dump every parsed record. Still all `ValueCAD = 1` or `ValueCAD = 0`.

Then I dumped the raw `pdftotext` output and saw this:

```
25 036523 000 00 B7
Alteration
Issued
2026/03/16
1                      ← this is a permit COUNT row, not a value
CONSTR. VALUE          ← this is a column header
$300,000.00            ← this is the actual value
FOLDER NAME 8640 Alexandra Road
Studio Senbel Architecture Inc
Safara Cladding Inc (416)875-1770
```

The original parser assigned fields by position: line 0 = work proposed, line 1 = status, line 2 = date, **line 3 = value**. But `pdftotext` inserts a bare `1` (the permit count for that sub-type section) between the date and the dollar amount. So the parser assigned `ValueCAD = 1` for every single permit. The `$300,000.00` was landing in the `Address` field.

The fix was rewriting the parser to detect fields by content rather than position:

- Line matches `^\d{4}/\d{2}/\d{2}$` → it's the issue date
- Line matches `^\$[\d,]+\.?\d*$` → it's a dollar value (first one = construction value; subsequent = subtotals, skip)
- Line matches `^FOLDER NAME (.+)` → address extracted from capture group
- Line matches `^\d+$` → bare integer, skip (it's a permit count row)
- Everything else → sequential fill: work proposed → status → applicant → contractor

Both the clean test fixture format and the real `pdftotext` output now work correctly. The parser fills fields by what they look like, not where they appear.

### Part B — Claude Enrichment

Raw permit data tells you: there's a $1.2M warehouse being built at 12500 Vulcan Way. It doesn't tell you:
- Who the general contractor is (just an applicant name)
- How big the crew will be
- Whether they're likely from out of town
- When to reach out

That's Claude's job. Each permit gets sent to the Claude Messages API (Haiku model — fast and cheap) with a system prompt that context-sets it as a hotel lead analyst:

```
You are a lead analyst for the Sandman Hotel Vancouver Airport in Richmond, BC.
Evaluate building permit records to identify projects that will generate
demand for construction crew lodging.
```

Claude returns a structured JSON object:

```json
{
  "general_contractor": "BuildRight Contracting",
  "project_type": "industrial",
  "estimated_crew_size": 80,
  "estimated_duration_months": 6,
  "out_of_town_crew_likely": true,
  "priority_score": 9,
  "priority_reason": "Large new industrial build near YVR — likely out-of-province steel crew",
  "suggested_outreach_timing": "Reach out now — crews mobilizing in 4–6 weeks",
  "notes": "GC is BuildRight Contracting. Check LinkedIn for travel coordinator."
}
```

One design choice worth noting: the raw permit data (applicant name, contractor name, phone number) is passed through to the lead record *as-is*, in addition to Claude's inferences. Claude's `general_contractor` is its best guess at who the GC is. The raw `contractor` field from the permit — "Safara Cladding Inc (416)875-1770" — is preserved separately so the phone number isn't lost.

### Part C — Slack Notification

The output is a Slack digest using Block Kit. Each lead gets a card:

```
🔥 Warehouse — 12500 Vulcan Way         Score: 9/10
📍 12500 Vulcan Way  |  💰 $1,200,000 CAD  |  🏢 GC: BuildRight Contracting
📞 Contractor: BuildRight Contracting (604)555-0199  |  Applicant: ABC Developments Ltd
🕐 Outreach: Reach out now — crews mobilizing in 4–6 weeks
📝 GC is BuildRight. Check LinkedIn for travel coordinator.
📄 View source document
```

The score emoji tiers: 🔥 (9–10), ⚡ (7–8), 👀 (5–6), 📌 (<5). The `📄 View source document` link goes directly to the Richmond PDF the permit came from.

The notifier also has a smoketest flag — `go run ./cmd/smoketest/ -testslack` — that sends two fake leads to the webhook so you can verify the Block Kit layout without running the full pipeline.

---

## Phase 4: Expanding the Net — Creative BC and VCC

Once the Richmond and Delta permit pipelines were stable, I moved on to "early signal" sources: projects that are *planned* but haven't hit the permit stage yet.

### Creative BC "In Production" List
Film and TV productions are massive lodging leads. A single TV series block (8–10 episodes) brings 100+ crew members for 4–6 months. Creative BC publishes a weekly "In Production" list as a Salesforce-hosted web page. 

I built a collector that scrapes this list, filters for **Feature Films** and **TV Series** (high out-of-town crew potential), and uses Claude to estimate the crew size and hotel night potential. 

### Phase 4-A1 & 4-A4 & 4-B: Infrastructure News, Eventbrite, and High-Priority Alerts
**Implementation: April 3, 2026**

With the core permit and convention centre scrapers in place, we expanded the system to capture earlier signals and specialized event leads.

#### 1. Google News RSS Monitor (Phase 4-A1)
We implemented a `NewsCollector` using `github.com/mmcdole/gofeed` to monitor Google News RSS feeds for construction and infrastructure keywords in the Richmond and YVR areas. This provides early signals for projects that might not have hit the permit stage yet.
*   **Keywords:** "Richmond BC construction", "Metro Vancouver infrastructure contract awarded", "YVR expansion".
*   **Filtering:** Basic keyword relevance and negative matching (skipping sports/politics).

#### 2. Eventbrite Professional Events (Phase 4-A4)
Since Eventbrite's public API is restricted, we implemented an HTML scraper using `github.com/PuerkitoBio/goquery`. It targets professional conferences and conventions in Vancouver, filtering out consumer shows, concerts, and hobbyist gatherings.
*   **Target:** Professional services, conferences, and conventions.
*   **Logic:** Card-based extraction with fallback to broad selectors.

#### 3. Priority Alert Threshold (Phase 4-B)
To help the sales team prioritize, we added a `PriorityAlertThreshold` (defaulting to 9) in the enrichment pipeline. Leads exceeding this score are flagged in logs and can be used for instant notifications.

#### 4. Bug Fix: Boundary Condition in Richmond Scraper
While testing the new collectors, we identified and fixed a boundary condition bug in the `RichmondCollector.isRelevant` function. Previously, permits with a value exactly at the threshold were being included; they are now correctly excluded (`ValueCAD <= minValue`).

---

### Vancouver Convention Centre (VCC) Events
Professional conventions are the "white whale" of hotel sales. A multi-day medical or tech congress brings 2,000+ out-of-town attendees. I built a scraper for the VCC event calendar that identifies professional gatherings and filters out consumer-only shows (like boat shows or warehouse sales).

The VCC website structure was more fluid than the permit PDFs, so I had to implement a **multi-selector fallback** logic in the scraper to ensure it consistently captures event titles, dates, and categories even if the markup shifts slightly.

### BC Bid — Automated RSS & Hybrid Integration
Government contracts are a major pillar of construction lodging leads. While the main BC Bid portal is protected by Captcha, many BC local governments aggregate their opportunities on **CivicInfo BC**. 

I updated the BC Bid collector to be fully automated by fetching categorized RSS feeds from CivicInfo BC (Construction and Road Maintenance categories). 

The collector now follows a dual-path strategy:
1. **Automated:** Every pipeline run fetches the latest award notices from CivicInfo BC RSS feeds.
2. **Hybrid/Manual:** Still accepts raw HTML/JSON via the `/run` endpoint for complex cases handled by external Playwright scrapers in n8n.

This ensures that the sales team gets a steady stream of government award leads automatically, while maintaining the flexibility to handle the more complex "legacy" BC Bid portal when needed.

---

## Phase 3: Delta BC Permits

After Richmond was working, I added a second building permit source: Delta, BC — the municipality directly south of Richmond, home to major warehouse and industrial corridors.

Delta's data is published differently: no weekly listing page, just a single PDF at a known URL stored in the `DELTA_PERMITS_URL` env var. The same `pdftotext -layout` approach works, but the PDF format is completely different from Richmond's.

The key parsing challenge: Delta's PDF uses a tabular layout where columns can wrap across lines and decimal values like `515,000.00` appear inline. I wrote:

- `extractDeltaBuilder()` — strips date/permit/decimal columns by position to isolate the builder name
- `parseDeltaDecimal()` — parses "515,000.00" → 515000
- `isDeltaRelevant()` — filters by occupancy type: industrial, commercial, assembly, institutional

The `DeltaCollector` wires in via the same `Collector` interface. In `cmd/server/main.go`, it's opt-in — only activated when `DELTA_PERMITS_URL` is set in the environment.

### Part D — Rule-Based Pre-Scoring (Phase 3-B)

As the number of data sources grew, so did the number of records. Many permits are for small residential renovations or minor repairs that don't need a construction crew. Sending every single one to Claude was wasting API tokens.

The fix: a points-based `Scorer` that evaluates a record in Go *before* enrichment.

```go
func (s *Scorer) Score(p *RawProject) int {
    score := 0
    // +2 for industrial/commercial types
    // +1 for high-value permits (> $1M)
    // +1 for specific "crew-heavy" keywords (warehouse, demolition, shoring)
    // -1 for "minor" keywords (repair, interior only)
    return score
}
```

The enrichment pipeline now has a threshold (default `1`). If a project's score is below the threshold, it's saved as a "skipped" lead. It's still in the database if we need it later, but we didn't spend a cent on AI to know it wasn't a priority.

---

## Phase 3-D: Hybrid Server & n8n Automation

Initially, `groupscout` was a simple CLI tool meant to be run manually or via a basic cron job. But to scale, I wanted to integrate it with **n8n** for more complex orchestration (e.g., "only run if it's a weekday and I haven't run it yet today").

I refactored the application from a pure CLI into a **Hybrid CLI/Server**. By default, it now boots an HTTP server that listens for a trigger.

### Secure POST Trigger

The core pipeline logic was moved into a shared `runPipeline` function. It can be triggered two ways:
1.  **CLI:** `go run ./cmd/server --run-once` (for manual runs/dev).
2.  **HTTP:** `POST /run` (for automation).

To prevent anyone from triggering the pipeline, I implemented **Bearer Token authentication**. You must set an `API_TOKEN` environment variable, and the caller (n8n) must provide it in the `Authorization` header.

```go
if r.Header.Get("Authorization") != "Bearer " + config.Get().APIToken {
    http.Error(w, "Unauthorized", http.StatusUnauthorized)
    return
}
```

This change turns `groupscout` from a local script into a real service that can be part of a larger automated workflow.

---

## Phase 4: Creative BC Film & TV Productions

Construction crews are one signal. Film and TV productions are another — and often a better one for the Sandman YVR specifically.

A feature film in production runs 150–400 crew. A TV series block runs 80–200. US studio productions import department heads and specialty crew who need accommodation near set, often for months. Richmond and Surrey are common shooting locations.

**Finding the data source** took some digging. Creative BC's main site (`creativebc.com`) requires JavaScript to render — a headless browser would be needed, which I didn't want to add to the stack. Their public "In Production" list is the authoritative weekly source but it's buried in a Salesforce Experience Cloud page.

The breakthrough: the Salesforce Visualforce endpoint at `https://knowledgehub.creativebc.com/apex/In_Production_List?isdtp=p1`. Visualforce pages are server-rendered — plain `net/http` GET returns full HTML. `isdtp=p1` strips the navigation chrome.

There's one TLS wrinkle: the Knowledge Hub uses a Salesforce intermediate CA that's not in Go's default root pool on Linux. I scoped `InsecureSkipVerify: true` to the Creative BC HTTP client only. No credentials are sent; this is just certificate chain validation.

**The HTML structure** wasn't what I expected. The page doesn't use a data table — it uses `<h5>` category headers ("Feature", "TV Series", "Doc Series"...) followed by production entries, each with `<h3 class="production">` for the title and labeled `<b>` elements for fields. My first parser assumption (table walker) was wrong; I fetched and inspected the actual HTML before writing the final version.

The parser (`parseCreativeBCHTML`) walks the tree tracking category state. Fields extracted per production:
- `Local Production Company` → studio name
- `Schedule` → date range ("3/9/2026 - 4/10/2026") → `IssuedAt`
- `Production Address` → city extracted for location scoring
- `Production Manager` → surfaced in Claude enrichment prompt

One formatting detail: titles on the page are ALL CAPS ("FARADAY", "WHITE ELEPHANT"). `toTitleCase()` converts them to readable form.

**Filter:** only `Feature Film` and `TV Series` pass. Animation, Documentary, New Media, and Mini Series are filtered out — local-crew work that doesn't generate hotel demand.

**Dedup hash:** `SHA256("creativebc|{lower(title)}|{lower(type)}")` — stable across weekly list reorders. Productions are identified by content, not position.

**Claude enrichment:** `buildRequest()` in `claude.go` branches on `p.Source == "creativebc"` and calls `creativeBCPrompt()` instead of `permitPrompt()`. Score boosters in the prompt: +2 for US/international studio name, +1 if address is in Richmond or Surrey, +1 if schedule start is within 4 weeks.

First real run: 23 productions parsed, 7 passed filter, all 7 enriched and posted to Slack.
- White Elephant (Surrey) — scored 9
- Tracker Season 3 — scored 8
- Yellowjackets Season 4 — scored 8

---

### Phase 4-A3: Vancouver Convention Centre (VCC) Scraper

**Status:** Completed ✅

We successfully expanded the project's data sources to include professional events and conventions.

*   **VCC Scraper:** Implemented a new collector for the Vancouver Convention Centre events calendar. The scraper uses `github.com/PuerkitoBio/goquery` to parse the VCC website and identify conferences, summits, and trade shows.
*   **Intelligent Filtering:** Since the VCC hosts everything from medical congresses to wedding shows, we implemented a keyword-based filter to isolate "professional" gatherings from consumer-facing ones.
*   **Enhanced Scoring:** Updated the rule-based pre-scorer to award points for conference-related keywords, ensuring these high-value leads are prioritised for enrichment.
*   **Robust Selectors:** Overcame challenges with dynamic HTML by implementing a multi-selector fallback system that accurately extracts event titles and metadata even when the exact CSS classes change.
*   **Pipeline Integration:** The VCC source is fully integrated behind a `VCC_ENABLED` environment toggle, allowing the sales team to start receiving professional group leads immediately alongside construction permits.

### Phase 4-C: Infrastructure Announcements & Major Projects

To capture the "biggest" leads (multi-year infrastructure projects), we implemented a specialized `AnnouncementsCollector`. This scraper targets the newsrooms and project pages of major B.C. infrastructure entities:

*   **BCIB (BC Infrastructure Benefits):** Tracks major government infrastructure projects (Highway 1 upgrades, Pattullo Bridge Replacement) where Community Benefits Agreements are in place—a massive signal for unionized crew lodging.
*   **TransLink Capital Projects:** Monitors transit expansion and maintenance projects across Metro Vancouver.
*   **YVR Newsroom:** Scrapes Vancouver International Airport's official news for terminal expansions and infrastructure upgrades.

These scrapers use a robust "last-modified" check to ensure only the latest announcements are processed, preventing duplicate alerts for old news.

### Phase 4-D: Weekly Email Digest & AI Outreach Drafting

While Slack is great for instant alerts, the sales team needed a way to review the week's leads in one place and start the outreach process immediately.

#### 1. HTML Email Digest (SendGrid)
We integrated SendGrid to deliver a beautifully formatted HTML email digest every week (accessible via the `/digest` endpoint). It aggregates all leads from the past 7 days, categorized by source and score, providing a clean executive summary of the pipeline's output.

#### 2. Automated Outreach Drafting
We added a `DraftOutreach` feature to the Claude enrichment layer. For every lead, Claude now generates a **personalized cold email template**.
*   **Context-Aware:** The draft references specific details from the permit or announcement (e.g., "I saw your recent $1.2M warehouse permit on Vulcan Way...").
*   **Value-Propositions:** Claude highlights the Sandman YVR's specific advantages for that project type (e.g., "ample parking for oversized crew trucks" or "proximity to the work site").
*   **Ready-to-Send:** These drafts are included in the weekly email digest, allowing sales managers to copy-paste-edit and reach out in seconds.

### Phase 5: Containerization & Production Readiness

To move from "script running on a laptop" to a "production service," we containerized the entire application.

*   **Multi-Stage Dockerfile:** A lean Docker image that compiles the Go binary and bundles the necessary `pdftotext` dependencies.
*   **Docker Compose:** Orchestrates the Go server and manages the SQLite database volume, making deployment to a VPS or Railway trivial.
*   **Standardized Logging:** Implemented structured logging across all modules to make production troubleshooting easier.

---

## Current State

The pipeline is a mature, production-ready lead generation engine. The full stack:

1. **Triggers:** n8n schedules a `POST /run` twice a week (Wednesday + Sunday, 9am). Can also be triggered manually via `curl`.
2. **Collects:** 8 active collectors — Richmond permits, Delta permits, Creative BC, VCC, BCIB, TransLink/YVR announcements, BC Bid RSS, Eventbrite, and Google News RSS.
3. **Scores:** A rule-based Go pre-scorer filters out noise before spending any API tokens.
4. **Enriches:** Claude Haiku analyzes each lead — crew size estimate, GC identification, outreach timing, personalized cold email draft.
5. **Notifies:** Instant Block Kit cards to Slack; weekly HTML digest via SendGrid.

**Infrastructure (all running in Docker Compose):**

| Service | Purpose |
|---|---|
| groupscout | Go HTTP server (`:8080`) |
| n8n | Workflow scheduler — fires `/run` on schedule |
| Prometheus | Metrics scraping |
| Grafana | Dashboards and log visualization |
| Loki | Structured log aggregation |
| Sentry | Runtime error tracking and alerting |

The full stack boots with `docker-compose up -d`. All logs are structured JSON (via `slog`), collected by Loki, and browsable in Grafana.

---

## What's Next

**Phase 6 — Productionization:**
- Swap SQLite → PostgreSQL for better concurrency.
- Formal database migrations (using `golang-migrate`).
- Final deployment to production VPS.
- Fix `pdftotext` inside Docker — add Poppler to the Alpine image so the permit scrapers work in the containerized environment.
- Smart Refresh: avoid re-fetching PDFs that haven't changed since the last run.

**Phase 7+ — Long-term:**
- Lead history tracking and sentiment trend analysis.
- CRM synchronization (Salesforce/HubSpot).
- Expand to sports teams, film permits from the province, DND/CBSA government contractors near YVR.
- Interactive Dashboard UI.

## What I Learned

**PDF parsing is never as simple as it looks.** Every PDF library has edge cases. When in doubt, shell out to a battle-tested tool (`pdftotext`) rather than fighting encoding issues in a pure Go library.

**Content-aware parsing beats positional parsing for messy real-world data.** The PDF format changes slightly between reports. Detecting fields by what they look like (date regex, dollar regex, keyword prefix) is more resilient than counting lines.

**Dedup first, enrich second.** Persisting the raw record to the DB before calling Claude means a failed enrichment doesn't lose the data — it just means no lead was created yet. Re-running after fixing an issue only re-enriches the missed records.

**Claude Haiku is remarkably good at structured extraction for ~$0.001 per call.** For a background pipeline running weekly on ~5 permits, it costs almost nothing and produces actionable output.

**Match your Go version in your Dockerfile.** `go.mod` declares `go 1.26` — if your Docker image is `golang:1.24-alpine`, `go mod download` silently fails with exit code 1. The version floor in `go.mod` is enforced at build time.

**Docker is where "works on my machine" bugs surface.** `pdftotext` works fine locally (bundled with Git for Windows) but isn't in Alpine Linux. The tool ran, reported success, and the permit scrapers silently returned zero results. Always check `docker-compose logs app` after a first containerized run — don't assume a clean exit means clean output.

**Running the whole stack in Docker Compose changes the development loop.** `docker-compose logs -f app` becomes your new best friend. Structured logging (`slog` with JSON output) makes those logs queryable in Grafana/Loki immediately — no extra instrumentation needed.

---

*groupscout is in active development. Code: [github.com/alvindcastro/groupscout](https://github.com/alvindcastro/groupscout)*
