# Troubleshooting — Missing Leads & Pipeline Gaps

This guide helps you diagnose why the GroupScout pipeline may return "0 new leads" or fewer leads than expected.

## 🔍 Core Pipeline Flow

GroupScout filters data at three distinct stages. Understanding these is key to troubleshooting:

1.  **Collector-level Filter**: Each source (Richmond, Delta, BCBid, etc.) has its own relevance rules. For example, Richmond building permits are filtered by a **minimum value** (default $500,000) and **commercial sub-types** (e.g., "hotel", "warehouse", "apartment"). Anything else is skipped before it even reaches the main pipeline.
2.  **Database Deduplication**: Before processing a lead, the system checks the `raw_projects` table for a matching SHA256 hash (based on title, location, and date). If it's already in the database, it's skipped.
3.  **Pre-scoring (Enrichment Threshold)**: The Go pre-scoring engine calculates a score based on location (Richmond/YVR gets +2) and keywords. If the score is below the `ENRICHMENT_THRESHOLD` (default: 1), it is marked as `skipped` and **will not** be enriched by AI or sent to Slack.

---

## 🛠 Troubleshooting Steps

### 1. Check the Logs
If you run `go run cmd/server/main.go --run-once`, look for these log markers:

- `parsed records from PDF count=102`: How many raw entries were found before filtering.
- `filtering complete source=richmond passed=5 skipped_low_value=90 skipped_residential=7`: Detailed breakdown of why permits were excluded.
- `skipping duplicate`: The permit was already processed in a previous run.
- `skipping enrichment: low score score=0`: The project was recorded but wasn't interesting enough to justify AI costs.

### 2. Inspect the Database
Use `sqlite3` (or your preferred DB viewer) to see what's happening under the hood:

```bash
# See the total count of collected projects vs. those that were enriched
sqlite3 groupscout.db "SELECT status, count(*) FROM leads GROUP BY status;"

# Look at the most recent "skipped" leads and why they were skipped
sqlite3 groupscout.db "SELECT title, priority_score, priority_reason FROM leads WHERE status = 'skipped' ORDER BY created_at DESC LIMIT 10;"

# Check if the collectors are finding anything at all
sqlite3 groupscout.db "SELECT source, count(*) FROM raw_projects GROUP BY source;"
```

### 3. Adjust Thresholds
If the filters are too strict, you can relax them in your `.env` file or environment variables:

- `MIN_PERMIT_VALUE_CAD`: Lower this (e.g., to `100000`) to include smaller construction projects.
- `ENRICHMENT_THRESHOLD`: Set this to `0` to enrich **every** project that passes the collector filter, regardless of keywords or location.
- `PRIORITY_ALERT_THRESHOLD`: Set this to a lower value (e.g., `5`) if you want more "instant alerts" in Slack for moderate-priority leads.

### 4. Verify External Tools
- **Richmond/Delta Permits**: These require `pdftotext` (from `poppler-utils`) to be installed on your system or inside your Docker container. If it's missing, these collectors will log an error and return 0 leads.
- **n8n / API Triggers**: If you are triggering the pipeline via `/run`, ensure your `API_TOKEN` is correct.

---

## ❓ FAQ

### Why is there only 1 lead?
This usually happens if:
- You've already run the pipeline today, and all other permits are now **duplicates**.
- The latest weekly report only has 1 permit that meets both the **commercial sub-type** AND the **$500k+ value** criteria.
- Most other permits have a **score of 0** (meaning they aren't in Richmond/YVR and didn't contain any high-value keywords like "pipeline", "film", or "concrete").

### How do I re-process old leads?
If you want to force the pipeline to re-process everything (e.g., after changing the scoring logic), you must clear the database:
```bash
# WARNING: This deletes all historical data!
rm groupscout.db
```
*(Or manually delete rows from `raw_projects` and `leads` tables).*
