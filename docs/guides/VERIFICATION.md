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

---

## 6. Verifying GQ4 Runtime Telemetry

Run the EvalOps package tests before enabling new production-like telemetry paths:

```bash
go test -v ./internal/evalops
```

The GQ4 telemetry helpers cover:

1. **Trace events**: Collector, enrichment, LLM, notification, and alert events preserve `trace_id`, `case_id`, stage, operation, status, and safe attributes.
2. **Redaction**: Trace payloads and review samples redact API keys, tokens, Slack webhook URLs, emails, phone numbers, raw PII, and oversized source excerpts before they are safe for logs, Loki, Sentry breadcrumbs, or OpenTelemetry attributes.
3. **Metrics**: `RuntimeMetrics` uses an injected Prometheus registry for collector failures, enrichment skipped counts, LLM errors, alert decisions, stage latency, token counts, and estimated cost counters.
4. **Review samples**: `ReviewSampleWriter` writes append-only JSONL when enabled, requires review metadata, rejects unredacted sensitive attributes, and keeps samples suitable as draft eval-case inputs.

### Dashboard and Alert Checklist

Use the following candidates for Grafana, Sentry, Loki, or alert rules:

| Risk | Signal | Suggested alert |
|---|---|---|
| Hallucination or unsupported claim | Review samples with `severity=critical` or scorer messages about unsupported evidence | Page/review when any critical sample appears after a prompt, collector, or provider change. |
| Source drift | Rising `groupscout_pipeline_collector_failures_total` or repeated parse warning samples by collector | Alert when one collector fails repeatedly in a run window or when case metadata points to a changed source type. |
| Cost drift | `groupscout_pipeline_llm_tokens_total` or `groupscout_pipeline_llm_cost_cents_total` exceeds the expected baseline | Alert on unexpected provider/model spend growth or live-provider usage in deterministic eval contexts. |
| Collector failure | `groupscout_pipeline_collector_failures_total{collector,reason}` increases across scheduled runs | Route to pipeline operations with the collector name and trace ID. |
| Webhook failure | Notification-stage trace events or Sentry breadcrumbs with `status=failed` | Alert when Slack or email notification failures repeat, without storing raw webhook URLs. |

---

## 7. Verifying GQ5 Feedback Loop

Run the draft-case helper tests when changing review sample, eval fixture, or calibration behavior:

```bash
go test -v ./internal/evalops -run 'TestDraftCasesFromReviewSamples|TestWriteDraftCasesJSONL'
```

The GQ5 feedback loop should preserve these guarantees:

1. **Review gate**: Only `ReviewSample` entries with `review_required=true` convert into draft cases.
2. **Traceability**: Draft cases preserve `trace_id` at the top level, in raw metadata, and in owner notes.
3. **Human TODOs**: Generated cases include `TODO_REVIEW_*` expected fields and `expected.release_blocking=false`.
4. **Promotion guard**: `LoadCases` rejects draft TODO decisions until a reviewer fills expected behavior.
5. **No auto-commit**: Generated drafts stay outside `data/evals/groupscout/` until reviewed, promoted, counted, and recorded in `docs/CHANGELOG.md`.
