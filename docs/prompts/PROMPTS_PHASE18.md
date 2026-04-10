# PROMPTS_PHASE18.md — Contact Enrichment + Budget Signals

> Copy-paste prompts for Phase 18.
> Tasks: Hunter.io client → enrich → schema migration → Slack block → budget tier.
>
> **Goal:** Auto-surface a decision-maker contact alongside each lead, and add a budget tier
> label so the sales team instantly knows whether to escalate.
>
> **TDD conventions:**
> - Write the T task test file first. Commit with all tests failing. Then implement.
> - Standard `testing` package only (no testify)
> - All Hunter.io tests use `httptest.NewServer` — no real API calls in test suite
> - Table-driven tests for all ranking/scoring logic
> - `t.Errorf()` for assertions

---

## 18.1 — Hunter.io API Client

```
Context:
- Hunter.io Domain Search: GET https://api.hunter.io/v2/domain-search?domain={domain}&api_key={key}
- Hunter.io Email Finder: GET https://api.hunter.io/v2/email-finder?domain={domain}&first_name=...&api_key={key}
- Free tier: 25 searches/month — use domain search, not per-email lookup
- Response: { "data": { "emails": [{ "value": "...", "type": "personal|generic", "first_name": "...", "last_name": "...", "position": "..." }] } }
- Rate limit: 429 status code → retry after Retry-After header seconds

Task 18-T — write internal/enrichment/hunter_test.go FIRST:
  Fixture JSON: domain-search response with 3 contacts (PM, accountant, CEO)
  Tests:
  - TestHunterClient_DomainSearch: httptest.NewServer returning fixture;
    assert 3 contacts returned, domain passed correctly in URL
  - TestRankContactByTitle table-driven:
      "Project Manager" → rank 1 (highest)
      "Travel Coordinator" → rank 1
      "Operations Manager" → rank 2
      "Site Supervisor" → rank 2
      "CEO" → rank 3
      "Accountant" → rank 4 (lowest relevant)
  - TestHunterClient_RateLimitRetry: server returns 429 with Retry-After: 1;
    assert client retries once and succeeds
  - TestHunterClient_OrgToDoamin: "PCL Construction" → "pcl.ca" (or best guess);
    mock domain-search on that domain
  All tests fail first. Commit.

Task 18.1 — create internal/enrichment/hunter.go:
  type HunterContact struct {
      Email     string
      FirstName string
      LastName  string
      Position  string
      Rank      int    // lower = higher priority
  }
  type HunterClient struct {
      apiKey string
      client *http.Client
  }
  func NewHunterClient(apiKey string) *HunterClient
  func (c *HunterClient) SearchByDomain(ctx context.Context, domain string) ([]HunterContact, error)
    - GET https://api.hunter.io/v2/domain-search?domain={domain}&api_key={key}
    - Handle 429: read Retry-After header, sleep, retry once
    - Map response emails array to []HunterContact
  func (c *HunterClient) TopContact(contacts []HunterContact) *HunterContact
    - Call RankContactByTitle() on each; return lowest-rank contact
  func RankContactByTitle(position string) int
    - Rank 1: "project manager", "travel coordinator", "travel manager", "travel planner"
    - Rank 2: "operations manager", "site supervisor", "construction manager", "camp coordinator"
    - Rank 3: "CEO", "president", "owner", "principal"
    - Rank 4: all others (accountant, HR, marketing, etc.)
  func OrgToDomain(orgName string) string
    - Best-guess domain from org name (strip "Inc", "Ltd", "Corp", "Construction", etc.)
    - "PCL Construction" → "pcl.ca"  (or use Hunter's company search as fallback)
    - Simple slug: lowercase, remove punctuation, append ".com" if not Canadian
    - Note: this is heuristic — hit rate will be ~40–60%; log misses

Verify: go test ./internal/enrichment/... -run TestHunter
```

---

## 18.2 — Wire Contact Enrichment into Enricher

```
Context:
- enricher.go processProject() flow: collect → dedup → score → LLM enrich → store → notify
- Hunter lookup goes AFTER LLM enrichment (we need general_contractor from Claude first)
- Hunter is optional — if HUNTER_API_KEY not set, skip silently
- Log hit rate: how many leads got a contact match vs. total enriched

Task 18.2 — update internal/enrichment/enricher.go:
  Add HunterClient field to Enricher struct (nil if no API key)
  In processProject(), after storing the lead:
    if e.hunter != nil {
        domain := OrgToDomain(lead.GeneralContractor)
        contacts, err := e.hunter.SearchByDomain(ctx, domain)
        if err == nil && len(contacts) > 0 {
            top := e.hunter.TopContact(contacts)
            lead.ContactName = top.FirstName + " " + top.LastName
            lead.ContactTitle = top.Position
            lead.ContactEmail = top.Email
            e.store.leads.UpdateContact(ctx, lead.ID, ...)
        }
        // log hit/miss for metrics
    }

  Update NewEnricher() to accept optional *HunterClient:
    func NewEnricher(llm LLMClient, store *Storage, hunter *HunterClient) *Enricher

  Note: hunter can be nil — all hunter calls are guarded by nil check.

Verify: unit test in enricher_test.go — TestEnricher_AttachesContact when hunter returns match;
        TestEnricher_SkipsHunterWhenNil when hunter is nil.
```

---

## 18.3 — Schema: Contact Fields Migration

```
Context:
- Lead struct needs 3 new fields: contact_name, contact_title, contact_email
- Add a new migration: 004_contact_fields.up.sql
- Maintain backwards compatibility: columns nullable, default NULL

Task 18.3 — create migrations/004_contact_fields.up.sql:
  ALTER TABLE leads ADD COLUMN IF NOT EXISTS contact_name  TEXT;
  ALTER TABLE leads ADD COLUMN IF NOT EXISTS contact_title TEXT;
  ALTER TABLE leads ADD COLUMN IF NOT EXISTS contact_email TEXT;

Task 18.3b — create migrations/004_contact_fields.down.sql:
  ALTER TABLE leads DROP COLUMN IF EXISTS contact_name;
  ALTER TABLE leads DROP COLUMN IF EXISTS contact_title;
  ALTER TABLE leads DROP COLUMN IF EXISTS contact_email;

Task 18.3c — update internal/storage/leads.go:
  - Add ContactName, ContactTitle, ContactEmail string fields to Lead struct
  - Update INSERT and SELECT queries to include these columns
  - Add UpdateContact(ctx, id string, name, title, email string) error method

Verify:
  go test ./internal/storage/...
  docker compose up -d
  Migration runs cleanly on existing Postgres instance.
```

---

## 18.4 — Slack Contact Block

```
Context:
- Existing lead Slack message uses Block Kit
- When ContactEmail is set, append a context block showing the contact
- Format: "👤 Jane Smith — Project Manager  |  jane.smith@pcl.ca"
- When empty: show nothing (don't clutter with "No contact found")

Task 18.4-T — update internal/notify/slack_test.go FIRST:
  Add test cases to existing TestFormatLeadMessage (or create new):
  - TestFormatLeadMessage_WithContact: lead.ContactEmail = "jane@pcl.ca";
    assert output contains "Jane Smith" and "jane@pcl.ca"
  - TestFormatLeadMessage_NoContact: lead.ContactEmail = "";
    assert no contact section in output
  Tests fail first. Commit.

Task 18.4 — update internal/notify/slack.go:
  In formatLeadBlocks(), after existing blocks:
  if lead.ContactEmail != "" {
      append context block:
      {
          "type": "context",
          "elements": [{
              "type": "mrkdwn",
              "text": "👤 *" + lead.ContactName + "* — " + lead.ContactTitle + "  |  " + lead.ContactEmail
          }]
      }
  }

Verify: go test ./internal/notify/...
```

---

## 18.5 — Config

```
Task 18.5 — update config/config.go:
  Add:
    HunterAPIKey string  // from HUNTER_API_KEY env var; empty = Hunter disabled

Task 18.5b — update .env.example:
  Add section:
    # Contact Enrichment (Hunter.io)
    # HUNTER_API_KEY=your-hunter-api-key-here   # free tier: 25 searches/month

Task 18.5c — update cmd/server/main.go:
  if cfg.HunterAPIKey != "" {
      hunter := enrichment.NewHunterClient(cfg.HunterAPIKey)
      enricher = enrichment.NewEnricher(llm, store, hunter)
  } else {
      enricher = enrichment.NewEnricher(llm, store, nil)
  }

Verify: run pipeline with HUNTER_API_KEY unset — no crash, no Hunter calls in logs.
```

---

## 18.6 — Enrichment Hit Rate Logging

```
Task 18.6 — update internal/enrichment/enricher.go:
  Track in pipeline run stats:
    type RunStats struct {
        LeadsEnriched    int
        HunterAttempts   int
        HunterHits       int
        HunterMisses     int
    }
  Log at end of RunPipeline():
    slog.Info("pipeline complete",
        "leads_enriched", stats.LeadsEnriched,
        "hunter_hit_rate", fmt.Sprintf("%.0f%%", float64(stats.HunterHits)/float64(stats.HunterAttempts)*100),
    )
  If HunterAttempts == 0, skip the hit rate log line.

Verify: run pipeline, check log output includes hunter_hit_rate.
```

---

## 18.7 — Budget Tier Signal

```
Context:
- Construction permit values range from $500K to $500M+
- Budget tier label helps the sales team gauge spend without interpreting raw numbers
- Tier labels: Low (<$1M), Medium ($1M–$10M), High ($10M–$50M), Major (>$50M)

Task 18.7-T — update internal/enrichment/scorer_test.go FIRST:
  Add TestBudgetTierScore table-driven:
  - 499_999 → "Low"
  - 1_000_000 → "Medium"
  - 10_000_000 → "High"
  - 50_000_001 → "Major"
  - 0 → "Unknown"
  Tests fail first. Commit.

Task 18.7 — update internal/enrichment/scorer.go:
  func BudgetTier(projectValue int64) string:
    switch {
    case projectValue <= 0:          return "Unknown"
    case projectValue < 1_000_000:   return "Low"
    case projectValue < 10_000_000:  return "Medium"
    case projectValue < 50_000_000:  return "High"
    default:                         return "Major"
    }

Task 18.8 — update internal/notify/slack.go:
  In formatLeadBlocks(), update the project value field:
    display: "$2.4M (Medium)" instead of "$2400000"
    Use humanize helper or manual formatting: value / 1_000_000 with "M" suffix
    Append BudgetTier label in parentheses.

Verify: go test ./internal/enrichment/... ./internal/notify/...
```

---

## Reference files

| File | Role |
|---|---|
| `internal/enrichment/hunter.go` | Hunter.io domain search + contact ranking |
| `internal/enrichment/enricher.go` | Wire Hunter after LLM enrichment |
| `internal/enrichment/scorer.go` | Add `BudgetTier()` helper |
| `internal/storage/leads.go` | Add contact fields + `UpdateContact()` |
| `internal/notify/slack.go` | Contact block + budget tier label |
| `migrations/004_contact_fields.up.sql` | ALTER TABLE adds contact columns |
| `config/config.go` | HUNTER_API_KEY env var |
| `docs/AI_DATA_STRATEGY.md` | Provider context (Hunter is not an LLM — it's a contact API) |
