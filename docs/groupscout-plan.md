# GroupScout — Product & Technical Planning

> Group lodging demand intelligence for hotel sales teams.  
> Go backend · Work in progress · Vancouver-focused expansion

---

## Table of Contents

1. [What GroupScout Does](#1-what-groupscout-does)
2. [Competitive Landscape](#2-competitive-landscape)
3. [Improvement Roadmap](#3-improvement-roadmap)
4. [Airport Disruption Alerts](#4-airport-disruption-alerts)
5. [Real-Time Demand Alert System](#5-real-time-demand-alert-system)
6. [Vancouver Context](#6-vancouver-context)
7. [Ollama Integration](#7-ollama-integration)
8. [Architecture Overview](#8-architecture-overview)
9. [MVP Scopes](#9-mvp-scopes)

---

## 1. What GroupScout Does

Monitors public data sources for signals that indicate incoming groups needing hotel room blocks — construction crews, sports teams, film productions, government contractors — and delivers prioritized leads to the sales team via Slack.

**Core signal types (current):**
- Construction / infrastructure project awards
- Sports team travel schedules
- Film & TV production permits
- Government contractor deployments

**Delivery:** Slack alerts with lead summary and suggested action  
**Stack:** Go backend, single binary, early-stage

---

## 2. Competitive Landscape

| Tool | Approach | Delivery | Focus |
|---|---|---|---|
| **GroupScout** | Monitors public data signals proactively | Slack | Room block demand (crews, teams, productions) |
| **Lead Shark** | AI platform, 100M+ companies profiled | Web app | Corporate LNR + group, verified contacts |
| **Cvent / Planner Navigator** | RFP network + planner behavior tracking | Web platform | Meetings & events planners |
| **Knowland** | Historical meetings/events intelligence | Web platform | Group history, comp set analysis |
| **HopSkip** | RFP marketplace, two-sided | Web platform | Planner-to-hotel matching |
| **GitGo** | Outsourced human + tech outreach | Managed service | Full-service lead generation |

**GroupScout's edge:**
- Proactive signal monitoring — catches demand before it becomes an RFP
- Targets underserved segments (crews, productions, contractors) ignored by Cvent/Knowland
- Slack-native — no login friction for sales reps
- Low-cost, self-hostable Go backend

**Current gaps vs. competitors:**
- No contact enrichment layer (biggest gap)
- No lead scoring or CRM integration
- No analytics / feedback loop

---

## 3. Improvement Roadmap

### Signal Sources & Detection

- **Permit data** — building/construction permits from city open data portals via API
- **Film commission databases** — many provinces/states publish active production permits
- **Sports schedules** — scrape league schedules, cross-reference away team travel distances to flag overnight needs
- **LinkedIn job postings** — companies hiring "site supervisors" or "field crews" signal incoming transient workers
- **Government contract awards** — federal contract awards with location and duration (Canada: MERX, Buyandsell.gc.ca)
- **News & press releases** — NLP on local business journals for expansion announcements, groundbreakings, tournament hosting
- **Event permits** — city event calendars for concerts, marathons, conventions not on Cvent

### Lead Intelligence & Enrichment

- **Contact enrichment** — biggest gap; auto-surface a decision-maker (project manager, production coordinator, travel manager) alongside each lead. Apollo.io or Hunter.io on company name as v1.
- **Lead scoring** — rank leads by estimated room nights, duration, lead time, and proximity to hotel
- **Company size / budget signals** — use contract value or production budget tier to estimate spend
- **Repeat pattern detection** — flag organizations that brought groups to the market in prior years

### Workflow & Integrations

- **CRM push** — write leads directly to Salesforce, HubSpot, or Delphi (hospitality-specific)
- **Slack actions** — add "Claim" / "Dismiss" / "Snooze" buttons so reps act without leaving Slack
- **Deduplication** — prevent the same project surfacing as multiple leads from different sources
- **Lead aging** — resurface ignored leads as their arrival date gets closer

### Analytics & Feedback Loop

- **Win/loss tracking** — let reps mark outcomes so the model learns which signal types convert
- **Market dashboard** — upcoming demand density by week so managers can forecast outreach sprints
- **Source attribution** — track which data source generates the most closed business

### Infrastructure & Scale

- **Multi-property support** — configure multiple hotels with different segments and geographies
- **Configurable alert thresholds** — minimum room nights, minimum duration, max lead time
- **Webhook output** — generic webhook so any downstream tool can consume leads
- **Rate-limited polite scraping + caching** — avoid blocks; cache raw source data to replay/re-score

---

## 4. Airport Disruption Alerts

### The Signal

Flight disruptions create **immediate, unplanned demand** for hotel rooms near airports. Hotels near airports are often caught flat-footed — the opportunity is to alert them as demand forms.

### Two Distinct Use Cases

**B2B — Airline Distressed Passenger Contracts**  
Airlines pre-negotiate distressed passenger room blocks with nearby hotels. GroupScout could:
- Monitor airline ops news and contract award notices
- Alert hotel sales teams when an airline is re-sourcing distressed passenger agreements
- Surface the right contact at each carrier's ops/procurement team

This is a slow-cycle sale — useful for proactive prospecting, not real-time ops.

**B2C-adjacent — Real-Time Demand Surge Alerts**  
When a ground stop hits during a storm, hotels within range are about to get flooded. The alert enables:
- Pulling rooms back from OTAs to avoid commission on demand they'd get anyway
- Activating a distressed traveler rate card
- Staffing up ahead of the rush

### GroupScout vs. Airport Alerts

| Dimension | Current GroupScout | Airport Emergency Leads |
|---|---|---|
| Lead time | Days to weeks | Minutes to hours |
| Demand certainty | Inferred from signals | Near-certain |
| Decision maker | Project manager, travel coordinator | Airline ops desk / walk-in guests |
| Sales motion | Outbound prospecting | Inbound readiness + rapid outreach |

This requires a fundamentally different response speed — closer to an **alert system** than a lead pipeline. Run as a separate binary.

---

## 5. Real-Time Demand Alert System

### Stranded Passenger Score (SPS)

Raw disruption signal alone isn't useful. Score each event:

```
SPS = (flights_cancelled / flights_scheduled)   # cancellation rate
    × avg_seats_per_flight                       # passenger volume
    × connecting_pax_ratio                       # stranded vs. reroutable
    × time_of_day_multiplier                     # 9pm >> 9am (fewer options)
    × duration_score                             # still growing vs. resolving
```

**Thresholds:**

| SPS | State | Action |
|---|---|---|
| < 20 | Normal | Ignore |
| 20–60 | Watch | No alert, monitor |
| 60–120 | Soft alert | "Disruption developing" |
| > 120 | Hard alert | "Act now" |

### Alert Lifecycle

Updates the **same Slack message** via `chat.update` — no channel spam.

```
T+0 min    WATCH    Ground stop declared, <30 min duration
T+30 min   ALERT    Cancellations crossing threshold — fire Slack alert
T+60 min   UPDATE   Score rising or falling — update original message
T+3 hrs    RESOLVE  Disruption clearing — send all-clear + summary
```

### Slack Alert Format

```
✈️  DISRUPTION ALERT — YVR (Vancouver International)
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Cause:    Atmospheric river — low visibility, approach restrictions
Status:   🔴 Active and growing (started 52 min ago)

Impact:
  • 71% of departures cancelled (168 of 236 flights)
  • Est. 13,200 stranded passengers
  • Connecting pax ratio: 58% — most cannot self-reroute

Earliest recovery: ~8–10 hrs (ECCC alert clears ~3am)

Your hotel:  8 min from terminal (Delta Hotels Richmond)
Rooms avail: 34 (as of last sync)

Suggested actions:
  ① Pull rooms from Expedia/Booking.com — avoid OTA commission
  ② Activate distressed traveler rate ($199 vs rack $309)
  ③ Call Air Canada ops desk: (604) 555-0190

[Claim Alert]  [Update Inventory]  [Dismiss]
```

### Go Architecture

Run as a **separate binary** from the main lead pipeline — different cadence, different failure modes.

```
cmd/
  alertd/              # new binary, separate from groupscout
    main.go

internal/
  aviation/
    navcanada.go       # NOTAM parsing
    yvr.go             # YVR flight status scraper
    flightaware.go     # cancellation rate (CYVR)
    scorer.go          # SPS calculation
  weather/
    eccc.go            # Environment Canada API — no key required
  alert/
    lifecycle.go       # watch → alert → update → resolve state machine
    slack.go           # chat.postMessage + chat.update
  config/
    airports.go        # hotel → airport mapping, thresholds, inventory
```

**Polling intervals:**
- Quiet period: every 10 minutes
- Active alert: every 90 seconds
- Exponential backoff on API errors

### Hotel Config (YAML)

```yaml
hotels:
  - id: delta-richmond
    name: Delta Hotels Vancouver Airport
    airports:
      - code: CYVR
        distance_min: 8
      - code: CYXX          # Abbotsford secondary
        distance_min: 52
    alert_threshold_sps: 60
    distressed_rate: 199
    rack_rate: 309
    airline_contacts:
      - carrier: AC
        ops_phone: "604-555-0190"
      - carrier: WS
        ops_phone: "604-555-0191"
    slack_channel: "#revenue-alerts"
```

### Engineering Risks

- **FlightAware free tier** — 500 requests/month. Cache aggressively; only poll FlightAware when ECCC alert is already active. Use ECCC as trigger, FlightAware as confirmation.
- **Cancellation rate lag** — airlines delay official cancellations 1–2 hrs. FAA/NavCanada ground stop data leads; flight status data lags. Trigger on ops data, confirm with cancellation rate.
- **Inventory sync** — without a PMS integration, room count is manual. Start with a `/inventory 34` Slack slash command that updates a local key-value store. Good enough for v1.
- **False positives** — short ground stops (<30 min) resolve before guests strand. Weight duration heavily in SPS; don't alert until event is at least 30 minutes old.

---

## 6. Vancouver Context

### Airport Coverage

| Airport | Code | Distance | Priority |
|---|---|---|---|
| Vancouver International | CYVR | Richmond, 20 min from downtown | **Primary** — 25M pax/year |
| Abbotsford International | CYXX | 50km east | Secondary — budget carriers |
| Harbour Air / Helijet | — | Downtown terminals | Low volume, high-value pax |

YVR is the only airport that matters for real disruption volume.

### Vancouver-Specific Disruption Patterns

**Atmospheric rivers** (November–February)  
Multi-day Pacific storms. Don't look dramatic but grind operations steadily. SPS builds slowly and sustains 12–48 hrs. Lower trigger threshold, alert earlier.

**Fraser Valley fog**  
Ground fog from the Fraser River valley can close YVR approaches suddenly with clear skies everywhere else. Very localized, unpredictable, causes sudden ground stops. Needs rapid detection and rapid all-clear.

**Snow events** (rare but high impact)  
YVR has limited deicing capacity. A 5cm snowfall that's routine in Toronto can cancel 40% of flights. Pre-alert based on ECCC snowfall warning *before* cancellations start. Highest-priority pattern.

**Single runway operations**  
Frequently drops to one runway for maintenance. Immediately halves capacity, cascades into delays within 2 hours. Worth a soft alert only.

**Air Canada hub cascades**  
YVR is an AC focus city. System-wide IT or crew issues hit YVR harder than average.

### Hotel Geography

```
Zone A — 0–10 min (highest demand capture)
  Richmond: Fairmont Vancouver Airport (in-terminal),
            Delta Hotels Marriott, Sandman Richmond

Zone B — 10–20 min
  Richmond/Brighouse: Holiday Inn, Hilton Garden Inn
  Burnaby: Metrotown area (Canada Line accessible)

Zone C — 20–35 min
  Downtown Vancouver: major hotels, viable when Zone A fills
  Surrey: budget options, serves CYXX overflow

Zone D — CYXX catchment
  Abbotsford, Langley, Chilliwack: separate config
```

**SkyTrain note:** Canada Line runs YVR → downtown in ~26 min. Stranded passengers in Vancouver have a transit option that most North American cities lack. Downtown hotels capture more overflow than in car-dependent markets — factor this into zone weighting.

### Canadian Data Sources (replacing FAA)

| Source | What it provides | API |
|---|---|---|
| **ECCC** `api.weather.gc.ca` | Weather alerts, Metro Vancouver + Fraser Valley zones | REST, free, no key |
| **NavCanada** `notams.faa.gov` equivalent | NOTAMs, airspace restrictions for CYVR | Public portal, HTML parse |
| **YVR Flight Status** `yvr.ca/en/passengers/flights` | Live delays and cancellations by flight | Scrape — no official API |
| **FlightAware AeroAPI** | Cancellation rate for CYVR | REST, free tier 500 req/month |

**Start with ECCC** — free, REST, well-documented, covers the exact weather patterns that drive YVR disruptions.

### ECCC Zone IDs for Vancouver

```go
zones := []string{
    "BC_14_09",  // Metro Vancouver
    "BC_14_10",  // Fraser Valley
    "BC_14_07",  // Howe Sound / Sea-to-Sky (can affect approaches)
}
```

### Vancouver-Tuned SPS Thresholds

| Event type | Adjustment |
|---|---|
| Atmospheric river | Lower trigger, longer lookback window (sustained 12–48 hrs) |
| Fog event | Rapid detection, also rapid all-clear — weight duration heavily |
| Snow event | Pre-alert on ECCC snowfall warning before flight data confirms |
| Single runway ops | Soft alert only, no hard alert |
| AC system cascade | Standard thresholds, flag carrier in alert |

---

## 7. Ollama Integration

### What Ollama Is

A local LLM runtime and model manager. Install it, run a background service, interact via CLI or OpenAI-compatible HTTP endpoint. Downloads and serves quantized models (Llama 3, Mistral, Phi-3, Gemma) optimized for CPU/GPU — entirely offline.

Think of it as **"Homebrew for LLMs."**

### Why It Fits GroupScout

All data is sensitive (hotel ops, lead intelligence). Running Ollama locally means no lead data leaves the server — a real selling point for enterprise hotel groups and a natural fit for PIPEDA compliance in Canada.

### Three Integration Points

**1. NLP signal extraction**  
Run a local model to parse unstructured text from permit filings, press releases, and news articles — extracting structured fields like organization name, location, dates, crew size, and project type. No cloud API cost per document.

**2. Lead scoring with reasoning**  
Use a Modelfile to create a "hotel sales analyst" persona that scores each raw signal and writes a short rationale explaining *why* it's a strong lead — making Slack messages more actionable than a raw score.

**3. Disruption alert copy**  
Instead of hardcoded alert text, pipe raw disruption data through a local model. Output gets injected into the Slack alert's "Suggested actions" block — different tone and specificity based on storm type, duration, and time of day.

### Modelfile Example

```
FROM mistral

SYSTEM You are a hotel revenue manager assistant at a Vancouver-area 
airport hotel. Given a flight disruption report, write 2–3 concise, 
specific action items for the front desk manager. Be direct. 
Include timing and dollar amounts where relevant.
```

### Recommended Models

| Model | Size | Use case |
|---|---|---|
| **Mistral 7B** | ~4GB | Signal extraction, lead scoring rationale |
| **Llama 3.1 8B** | ~5GB | Alert copy generation, more nuanced output |
| **Phi-3 Mini** | ~2GB | Fast extraction on low-memory servers |

### Go Integration

```go
// Ollama is OpenAI-compatible — minimal code change
type OllamaClient struct {
    Endpoint string // http://localhost:11434
    Model    string // "mistral"
}

func (c *OllamaClient) ExtractSignal(rawText string) (*LeadSignal, error) {
    prompt := fmt.Sprintf(`Extract the following fields from this text as JSON:
org_name, location, start_date, duration_days, crew_size, project_type.
Text: %s
Respond ONLY with valid JSON, no other text.`, rawText)

    // POST to /api/generate or /v1/chat/completions
    // Parse JSON response into LeadSignal struct
}
```

LangChainGo and community Go SDKs are available — no language switch required.

### Hardware Requirements

| RAM | Max model size |
|---|---|
| 8GB | 3B models |
| 16GB | 7B models (Mistral, Llama 3.1 8B) |
| 32GB | 13B+ models |

A standard $20/month VPS with 16GB RAM can run Mistral 7B comfortably for GroupScout's workload (batch processing, not real-time chat).

---

## 8. Architecture Overview

```
┌─────────────────────────────────────────────────────────┐
│                     GroupScout                          │
│                                                         │
│  cmd/groupscout/     — lead pipeline (existing)         │
│  cmd/alertd/         — disruption alerts (new)          │
│                                                         │
│  internal/                                              │
│    signals/          — permit, contract, sports scrapers│
│    aviation/         — navcanada, yvr, flightaware      │
│    weather/          — eccc API                         │
│    scoring/          — SPS + lead scoring               │
│    enrichment/       — contact lookup (Apollo/Hunter)   │
│    ollama/           — local LLM client                 │
│    alert/            — lifecycle state machine          │
│    slack/            — postMessage + update             │
│    config/           — hotel, airport, threshold YAML   │
│    store/            — sqlite: leads, alerts, inventory │
└─────────────────────────────────────────────────────────┘

External data (all free/public for Vancouver):
  ECCC api.weather.gc.ca       — weather alerts
  NavCanada notams portal      — NOTAM parse
  YVR yvr.ca/flights           — flight status scrape
  FlightAware AeroAPI (CYVR)   — cancellation rate
  City of Vancouver open data  — building permits
  BC Film Commission           — production permits
  Buyandsell.gc.ca             — federal contracts

Local:
  Ollama + Mistral 7B          — NLP extraction + copy
  SQLite                       — leads, alert state, inventory
```

---

## 9. MVP Scopes

### MVP 1 — Disruption Alert (1–2 weekends)

1. ECCC weather alert poller for Metro Vancouver / Fraser Valley zones (no API key, start here)
2. YVR flight status scraper — cancellation rate every 5 minutes
3. SPS calculation with hardcoded thresholds
4. Slack `postMessage` on threshold breach
5. Slack `chat.update` as disruption evolves
6. `/inventory N` slash command for manual room count updates
7. Single hotel config in YAML

*FlightAware, Ollama copy generation, and Slack action buttons are v2.*

### MVP 2 — Ollama Signal Extraction (1 weekend)

1. Install Ollama + pull Mistral 7B on the host server
2. `internal/ollama` Go client — simple POST to `/api/generate`
3. Modelfile for "permit extraction" persona
4. Wire into existing signal scraper pipeline
5. Compare extracted structured fields vs. current regex approach

### MVP 3 — Contact Enrichment (2–3 weekends)

1. Add Hunter.io API client — look up company domain by org name
2. Surface most likely decision-maker title based on project type (hardcoded mapping first)
3. Append contact info block to existing Slack lead messages
4. Track enrichment hit rate by signal source

### MVP 4 — Lead Scoring + Feedback Loop (ongoing)

1. Score each lead on room nights × duration × proximity × lead time
2. Add "Won" / "Lost" / "No response" Slack buttons to each lead
3. Store outcomes in SQLite
4. Weekly digest: which signal sources are converting

---

*Last updated: April 2026*  
*Repository: github.com/alvindcastro/groupscout*
