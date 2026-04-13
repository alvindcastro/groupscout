# GroupScout — Data Verification Guide

This guide provides steps to verify that the pipeline is running correctly and that the data in the database is accurate and enriched.

---

## 1. Quick Verification (Start-to-Finish)

To reset everything and run the entire pipeline once to check the current flow:

```bash
make start-fresh
```

This command will:
1. Stop all Docker containers and remove their volumes (clearing the Postgres database).
2. Remove any local `groupscout.db` SQLite files.
3. Start the Postgres and Ollama containers.
4. Run the pipeline pass once using the Go server.

---

## 2. Verifying the Pipeline Steps

As the pipeline runs, check the logs for the following key events:

1. **Database Migration**: `database ready` log with your Postgres URL.
2. **Scraping**: Logs from `RichmondPermits`, `DeltaPermits`, `CreativeBC`, etc., showing how many items were collected.
3. **Deduplication**: Logs showing `already exists, skipping enrichment` for projects that haven't changed.
4. **Enrichment**: Logs showing `enriching project` followed by the AI provider used (Claude, Gemini, or Ollama).
5. **Notification**: Logs showing `Slack notification sent` or `email digest sent`.

---

## 3. Verifying Data in Postgres

To manually inspect the data, you can use `psql` or any SQL client (like TablePlus, DBeaver, or JetBrains GoLand's Database tab).

### Common Verification Queries

Check the number of raw projects collected:
```sql
SELECT count(*) FROM raw_projects;
```

Check the number of leads generated:
```sql
SELECT count(*) FROM leads;
```

Inspect the latest enriched leads:
```sql
SELECT title, location, project_value, priority_score, priority_reason 
FROM leads 
ORDER BY created_at DESC 
LIMIT 10;
```

Verify that rationale is being populated:
```sql
SELECT rationale 
FROM leads 
WHERE rationale IS NOT NULL 
LIMIT 5;
```

Check for any errors in the audit trail:
```sql
SELECT * FROM raw_inputs ORDER BY created_at DESC LIMIT 10;
```

---

## 4. Verifying Slack Notifications

1. Check your configured Slack channel.
2. You should see a "Weekly Sales Lead Digest" (even if run manually).
3. Verify that the "Priority Alert" section contains leads with a `priority_score` above the threshold (default is 9).
4. Check that buttons (e.g., "View Source", "Open in GoLand") are working as expected.

---

## 5. Verifying AI Enrichment Quality

If leads have a `priority_score` of 1 but clearly should be higher, or if the `rationale` is generic, check:
1. **AI Provider**: Are you using Claude, Gemini, or Ollama? (Claude is currently the highest quality).
2. **Prompts**: Check `internal/enrichment/prompts.go` (if applicable) or the embedded prompts in the code.
3. **Raw Data**: Check `raw_projects.raw_data` to see if the scraper is actually getting enough text for the AI to work with.
