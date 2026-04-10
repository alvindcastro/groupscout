# PROMPTS_PHASE17.md — Airport Disruption Alert System

> Copy-paste prompts for each part of Phase 17.
> Parts must be done in order: A → B → C → D → E.
>
> **Goal:** A separate real-time binary (`cmd/alertd/`) that monitors YVR flight disruptions
> and alerts the hotel team via Slack before stranded passengers hit the front desk.
>
> **Full context + Vancouver-specific tuning:** `docs/groupscout-plan.md` sections 4–6.
>
> **TDD conventions (strictly followed here):**
> - Write the T task test file first. Commit it with all tests failing. Then implement.
> - Standard `testing` package only (no testify)
> - Table-driven tests using `[]struct{ name string; ... }` slices
> - All scrapers/parsers are pure functions — test with fixture data, not live HTTP
> - `httptest.NewServer` for HTTP client tests
> - `t.Errorf()` for assertions

---

## Part A — Data Sources & Weather

**New packages:** `internal/weather/`, `internal/aviation/`

### A1 — ECCC Weather Poller

```
Context:
- ECCC (Environment and Climate Change Canada) publishes weather alerts via REST API
- No API key required — fully public
- Base URL: https://api.weather.gc.ca/collections/alerts/items
- Zone IDs for YVR: BC_14_09 (Metro Vancouver), BC_14_10 (Fraser Valley), BC_14_07 (Howe Sound)
- Response format: GeoJSON FeatureCollection; each feature has .properties.event, .properties.severity

Task A1-T — write internal/weather/eccc_test.go FIRST:
  Fixture JSON: a minimal GeoJSON response with 1 atmospheric river alert + 1 fog alert
  Tests:
  - TestParseWeatherAlerts: confirm both alerts parsed; assert event type, severity, zone
  - TestClassifyAlertType table-driven:
      "SNOWFALL WARNING" → Snow
      "FREEZING FOG ADVISORY" → Fog
      "SPECIAL WEATHER STATEMENT" with "atmospheric" in headline → AtmosphericRiver
      "WIND WARNING" → Wind
      unknown → Unknown
  - TestECCCClient_FetchAlerts: httptest.NewServer returning fixture; assert 2 alerts returned
  All tests must fail before eccc.go exists. Commit failing tests.

Task A1 — create internal/weather/eccc.go:
  type AlertType string  // constants: Snow, Fog, AtmosphericRiver, Wind, Unknown
  type WeatherAlert struct {
      Zone      string
      Event     string
      Severity  string    // Minor | Moderate | Extreme
      Type      AlertType
      StartTime time.Time
  }
  type ECCCClient struct { client *http.Client; baseURL string }
  func NewECCCClient() *ECCCClient  // uses default base URL
  func (c *ECCCClient) FetchAlerts(ctx context.Context, zones []string) ([]WeatherAlert, error)
    - GET {baseURL}?zone={zone} for each zone
    - Parse GeoJSON features into WeatherAlert structs
  func ClassifyAlertType(event, headline string) AlertType
    - "SNOWFALL" or "SNOW" → Snow
    - "FOG" → Fog
    - "atmospheric" (case-insensitive) → AtmosphericRiver
    - "WIND" → Wind

Task A2 — add severity classification:
  Atmospheric river: trigger SPS at lower threshold; lookback window = 48 hrs (sustained event)
  Snow: pre-alert fires when ECCC issues a SNOWFALL WARNING even before flight cancellations
  Fog: weight duration heavily; rapid all-clear when fog lifts
  Single-runway ops (NOTAM): soft alert only, no hard alert
  Add VancouverTuning(alert WeatherAlert) TuningParams struct with these overrides.
  Write test: TestVancouverTuning_SnowPreAlert, TestVancouverTuning_FogDurationWeight

Verify: go test ./internal/weather/...
```

### A3 — YVR Flight Status Scraper

```
Context:
- YVR flight status page: https://www.yvr.ca/en/passengers/flights
- No official API — scrape HTML
- Target: cancellation count, total departures, and current ground stop status
- Use goquery (already a dependency in go.mod)

Task A3-T — write internal/aviation/yvr_test.go FIRST:
  Fixture: minimal HTML snippet with a flights table containing 3 cancelled, 10 total departures
  Tests:
  - TestParseYVRFlightStatus: assert CancelledCount=3, TotalDepartures=10, CancellationRate=0.30
  - TestParseYVRFlightStatus_NoCancellations: all on-time, assert rate=0.0
  - TestParseYVRFlightStatus_MalformedHTML: gracefully returns zero values, no panic
  All tests fail first. Commit.

Task A3 — create internal/aviation/yvr.go:
  type FlightStatus struct {
      CancelledCount   int
      TotalDepartures  int
      CancellationRate float64  // 0.0 – 1.0
      GroundStop       bool
      AsOf             time.Time
  }
  type YVRScraper struct { client *http.Client; url string }
  func NewYVRScraper() *YVRScraper
  func (s *YVRScraper) FetchStatus(ctx context.Context) (*FlightStatus, error)
  func parseYVRFlightStatus(body io.Reader) (*FlightStatus, error)  // pure — testable

Verify: go test ./internal/aviation/...
```

### A4 — NavCanada NOTAM Parser

```
Context:
- NavCanada publishes NOTAMs (Notice to Air Missions) for CYVR
- Public portal: https://notams.aim.faa.gov/notamSearch/ (or NavCanada equivalent)
- CYVR identifier — search for ground stops ("GND STOP") and runway restrictions
- Parse HTML response — no JSON API

Task A4-T — write internal/aviation/navcanada_test.go FIRST:
  Fixture: HTML snippet with one NOTAM containing "GND STOP" and one routine NOTAM
  Tests:
  - TestParseNOTAMs: assert 2 NOTAMs parsed
  - TestDetectGroundStop: NOTAM with "GND STOP" text → GroundStop=true
  - TestDetectGroundStop_NoStop: routine NOTAM → GroundStop=false
  All tests fail first. Commit.

Task A4 — create internal/aviation/navcanada.go:
  type NOTAM struct {
      ID        string
      Text      string
      IssuedAt  time.Time
      GroundStop bool
  }
  type NavCanadaClient struct { client *http.Client; url string }
  func (c *NavCanadaClient) FetchNOTAMs(ctx context.Context, airport string) ([]NOTAM, error)
  func parseNOTAMs(body io.Reader) ([]NOTAM, error)  // pure
  func detectGroundStop(notams []NOTAM) bool  // pure — check for "GND STOP" in Text

Verify: go test ./internal/aviation/...
```

---

## Part B — Stranded Passenger Score (SPS)

```
Context:
- SPS formula: (cancelled/scheduled) × avg_seats × connecting_pax_ratio × time_of_day_multiplier × duration_score
- Thresholds: <20 ignore, 20–60 watch, 60–120 soft alert, >120 hard alert
- avg_seats: assume 160 for YVR mix (narrow + wide body)
- connecting_pax_ratio: 0.58 for YVR (Air Canada hub; high connect ratio)
- time_of_day_multiplier: 9pm–midnight = 1.5; 6pm–9pm = 1.2; 6am–6pm = 1.0; midnight–6am = 0.8
- duration_score: minutes_active / 60, capped at 3.0

Task B-T — write internal/aviation/scorer_test.go FIRST:
  Table-driven tests covering:
  - cancellationRate=0.0 → SPS=0 (ignore)
  - cancellationRate=0.15, daytime → watch band (20–60)
  - cancellationRate=0.50, evening, 60 min active → soft alert band (60–120)
  - cancellationRate=0.71, night, 90 min active → hard alert band (>120)
  Vancouver edge cases:
  - TestSPS_SnowPreAlert: SPS=0 + ECCC snow warning → PreAlert=true (no cancellations yet)
  - TestSPS_FogRapidResolve: fog event, duration < 20 min → no alert even at high cancel rate
  All tests fail first. Commit.

Task B1 — create internal/aviation/scorer.go:
  type SPSInput struct {
      CancellationRate     float64
      AvgSeatsPerFlight    int      // default 160
      ConnectingPaxRatio   float64  // default 0.58 for YVR
      HourOfDay            int      // 0–23
      MinutesActive        int
      WeatherAlert         *weather.WeatherAlert  // nil if no active alert
  }
  type SPSResult struct {
      Score       float64
      State       AlertState  // Ignore | Watch | SoftAlert | HardAlert | PreAlert
      Explanation string
  }
  type AlertState string  // constants: Ignore, Watch, SoftAlert, HardAlert, PreAlert
  func ComputeSPS(input SPSInput) SPSResult

Task B2 — thresholds as constants:
  const (
      SPSIgnore    = 20
      SPSWatch     = 60
      SPSSoftAlert = 120
  )
  func AlertStateFromScore(score float64) AlertState

Task B3 — Vancouver tuning:
  func ApplyVancouverTuning(result *SPSResult, alert *weather.WeatherAlert, minutesActive int)
    - Fog: if minutesActive < 20 → downgrade to Ignore regardless of score
    - Snow: if alert.Type == Snow && result.State == Ignore → PreAlert
    - AtmosphericRiver: lower SPSWatch threshold to 40 (sustained events)

Verify: go test ./internal/aviation/... -v
```

---

## Part C — Alert Lifecycle State Machine

```
Context:
- State machine: Watch → Alert → Update → Resolve
- No hard alert fires until event has been active ≥ 30 minutes
- Slack uses chat.postMessage on first alert, chat.update for all subsequent updates
- All-clear + summary sent on Resolve
- Slack requires a Bot Token (not webhook) for chat.update — use SLACK_BOT_TOKEN env var

Task C1-T — write internal/alert/lifecycle_test.go FIRST:
  type mockSlack that records calls (PostMessage, UpdateMessage, SendResolve)
  Tests:
  - TestLifecycle_WatchToAlertAt30Min: simulate 0→29 min (Watch only), then 30 min → Alert fires
  - TestLifecycle_UpdateOnScoreChange: after Alert, new SPS → UpdateMessage called, not PostMessage
  - TestLifecycle_ResolveSendsAllClear: state → Resolve → SendResolve called with summary
  - TestLifecycle_NoAlertForShortFog: fog event resolves at 15 min → no Alert ever fires
  All tests fail first. Commit.

Task C1 — create internal/alert/lifecycle.go:
  type EventState string  // Watch | Alert | Updating | Resolved
  type DisruptionEvent struct {
      ID          string
      StartedAt   time.Time
      State       EventState
      LastSPS     SPSResult
      SlackTS     string     // message timestamp for chat.update
  }
  type Notifier interface {
      PostMessage(ctx, msg AlertMessage) (ts string, err error)
      UpdateMessage(ctx, ts string, msg AlertMessage) error
      SendResolve(ctx, ts string, summary ResolveSummary) error
  }
  type LifecycleManager struct {
      events   map[string]*DisruptionEvent
      notifier Notifier
  }
  func (m *LifecycleManager) Process(ctx context.Context, airportCode string, sps SPSResult) error
    - if no active event and sps.State == Ignore → do nothing
    - if no active event and sps.State >= Watch → create event, state=Watch
    - if Watch and minutesActive >= 30 and sps.State >= SoftAlert → PostMessage, state=Alert
    - if Alert and new sps → UpdateMessage
    - if Alert and sps.State == Ignore → SendResolve, state=Resolved

Task C2-T — write internal/alert/slack_test.go FIRST:
  mock httptest.NewServer for Slack API
  Tests:
  - TestSlackAlerter_PostMessage: correct JSON body, assert Slack channel, ts returned
  - TestSlackAlerter_UpdateMessage: uses chat.update endpoint with correct ts
  - TestSlackAlerter_SendResolve: sends all-clear message format
  All tests fail first. Commit.

Task C2 — create internal/alert/slack.go:
  type SlackAlerter struct {
      botToken string
      channel  string
      client   *http.Client
  }
  func (a *SlackAlerter) PostMessage(ctx, msg AlertMessage) (string, error)
    - POST https://slack.com/api/chat.postMessage
    - Auth: Authorization: Bearer {botToken}
    - Return ts from response
  func (a *SlackAlerter) UpdateMessage(ctx, ts string, msg AlertMessage) error
    - POST https://slack.com/api/chat.update with ts field
  func (a *SlackAlerter) SendResolve(ctx, ts string, summary ResolveSummary) error

Task C3 — AlertMessage format:
  type AlertMessage struct {
      AirportCode   string
      Cause         string     // e.g., "Atmospheric river — low visibility"
      State         string     // "🔴 Active and growing (52 min)"
      Cancelled     int
      TotalFlights  int
      EstStranded   int
      EarliestClear string     // "~8–10 hrs"
      RoomsAvail    int
      Actions       []string   // ["Pull rooms from Expedia...", ...]
  }
  Build Block Kit JSON from AlertMessage struct.

Verify: go test ./internal/alert/...
```

---

## Part D — Hotel Config & Binary

```
Context:
- Hotel config stored in config/airports.yaml (or unified config/properties.yaml in Phase 21)
- Binary is cmd/alertd/main.go — separate from cmd/server/
- Poll loop: 10 min quiet period, 90 sec during active alert
- Exponential backoff on API errors (max 5 min)

Task D1-T — write config/airports_test.go FIRST:
  Fixture YAML with one hotel entry (Sandman Richmond)
  Tests:
  - TestLoadAirportConfig: assert hotel name, airport code (CYVR), distance_min=8
  - TestLoadAirportConfig_AirlineContacts: assert AC ops phone parsed
  - TestLoadAirportConfig_MissingFile: returns error, not panic
  All tests fail first. Commit.

Task D1 — create config/airports.go:
  type HotelConfig struct {
      ID               string
      Name             string
      SlackChannel     string
      Airports         []AirportRef
      AlertThresholdSPS float64
      DistressedRate   int
      RackRate         int
      AirlineContacts  []AirlineContact
  }
  type AirportRef struct {
      Code        string   // CYVR, CYXX
      DistanceMin int
  }
  type AirlineContact struct {
      Carrier  string
      OpsPhone string
  }
  func LoadAirportConfig(path string) ([]HotelConfig, error)

Task D2 — create cmd/alertd/main.go:
  func main():
    - Load config from ALERTD_CONFIG_PATH or default config/airports.yaml
    - Create ECCC client, YVR scraper, NavCanada client, LifecycleManager
    - Poll loop:
        interval := 10 * time.Minute  // quiet
        if activeAlert { interval = 90 * time.Second }
        tick := time.NewTicker(interval)
        for range tick.C {
            alerts, _ := eccc.FetchAlerts(ctx, zones)
            status, _ := yvr.FetchStatus(ctx)
            notams, _ := navcanada.FetchNOTAMs(ctx, "CYVR")
            sps := scorer.ComputeSPS(...)
            lifecycle.Process(ctx, "CYVR", sps)
        }
  - Exponential backoff helper: on consecutive errors, double wait up to 5 min

Task D3 — add alertd service to docker-compose.yml:
  alertd:
    build: .
    command: ["/alertd"]
    env_file: .env
    volumes:
      - ./config:/config:ro
    depends_on:
      postgres:
        condition: service_healthy
    restart: unless-stopped

Verify:
  go build ./cmd/alertd/
  go test ./internal/weather/... ./internal/aviation/... ./internal/alert/... ./config/...
```

---

## Part E — Inventory Slash Command

```
Context:
- Slack slash command: /inventory 34
- Updates a local KV store (in-memory map or simple DB row) with current room count
- Displayed in subsequent Slack alerts: "34 rooms available (as of 2:14pm)"
- HTTP endpoint in alertd: POST /slack/inventory

Task E1-T — write cmd/alertd/main_test.go FIRST:
  Tests:
  - TestInventoryHandler_UpdatesRoomCount: POST body "payload=command=/inventory&text=34";
    assert KV updated to 34; response 200
  - TestInventoryHandler_InvalidCount: text="abc" → 400 response
  - TestInventoryHandler_ZeroCount: text="0" → valid, KV updated to 0
  All tests fail first. Commit.

Task E1 — add handler to cmd/alertd/main.go:
  var roomInventory int  // or store in DB
  func inventoryHandler(w http.ResponseWriter, r *http.Request):
    - Parse Slack slash command POST body
    - Extract number from r.FormValue("text")
    - Update roomInventory
    - Respond with 200 + ephemeral message: "Room count updated to {n}"
  Mount at /slack/inventory on a separate HTTP server (port from ALERTD_PORT env var)

Task E2 — display in alerts:
  AlertMessage.RoomsAvail int is populated from roomInventory in the poll loop.
  If roomInventory == 0, display "room count not set — use /inventory N" instead.

Verify:
  curl -X POST http://localhost:8081/slack/inventory \
    -d "command=/inventory&text=34&token=..."
  Next alert should include "34 rooms available"
```

---

## Reference files

| File | Role |
|---|---|
| `internal/weather/eccc.go` | ECCC weather poller + alert classifier |
| `internal/aviation/yvr.go` | YVR flight status scraper |
| `internal/aviation/navcanada.go` | NavCanada NOTAM parser |
| `internal/aviation/scorer.go` | SPS formula + Vancouver tuning |
| `internal/alert/lifecycle.go` | Watch → Alert → Update → Resolve state machine |
| `internal/alert/slack.go` | chat.postMessage + chat.update |
| `config/airports.go` | Hotel ↔ airport config loader |
| `cmd/alertd/main.go` | Poll loop + inventory handler |
| `docs/groupscout-plan.md` | Full context: Vancouver patterns, SPS formula, hotel zones |
