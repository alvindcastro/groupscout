# AI-Ready SQL — Coding Prompts

> Phase A implementation prompts for `AI_DATA_STRATEGY.md`.
> Goal: replace scattered `fmt.Sprintf` prompt assembly with a single SQL view that pre-builds LLM context strings from stored lead data.
>
> **Do Phase A tasks in order — each builds on the previous.**

---

## Context

### What exists today

- `internal/enrichment/claude.go` — 6 `*Prompt()` functions, all hand-build strings from `collector.RawProject` using `fmt.Sprintf`
- `DraftOutreach()` in `claude.go` — also hand-builds a string from `storage.Lead`
- Migrations: `001_init.postgres.up.sql` (schema), `003_pgvector.up.sql` (vector table + index)
- `storage.LeadStore` interface — `Insert`, `ListNew`, `ListForDigest`, `UpdateStatus`

### What Phase A adds

1. **`v_lead_context` Postgres view** — joins `leads + raw_projects` into one pre-built context string per lead
2. **`GetContext()` on `LeadStore`** — queries the view; returns the `context_text` string for a given lead ID
3. **Refactor `DraftOutreach()`** — replace manual string building with `GetContext()`; this is the first concrete win
4. **Phase B hook** — `GetContext()` will also be used by the RAG step to inject similar past leads into new permit prompts

---

## Task 1 — Migration: create `v_lead_context` view

**File to create:** `migrations/004_lead_context_view.up.sql`
**File to create:** `migrations/004_lead_context_view.down.sql`

```
Create a Postgres migration that adds a view called v_lead_context.

The view should JOIN leads l LEFT JOIN raw_projects rp ON l.raw_project_id = rp.id.

Select all columns from leads, plus rp.raw_data, plus a computed column called
context_text that concatenates key lead fields into a single dense string suitable
for injecting directly into an LLM prompt.

context_text format (one string, no newlines):
  "Source: {source}. Title: {title}. Location: {location or 'unknown'}.
   Value: ${project_value or 'unknown'} CAD. GC: {general_contractor or 'unknown'}.
   Type: {project_type or 'unknown'}. Crew: ~{estimated_crew_size or '?'}.
   Duration: ~{estimated_duration_months or '?'} months.
   Out-of-town crew likely: {out_of_town_crew_likely}.
   Score: {priority_score}/10. Reason: {priority_reason or 'none'}.
   Notes: {notes or 'none'}."

Use COALESCE and CAST for nullable columns. Use || for Postgres string concatenation.

The down migration should DROP VIEW IF EXISTS v_lead_context.
```

---

## Task 2 — Storage: add `GetContext()` to `LeadStore`

**Files to edit:**
- `internal/storage/leads.go`

```
Add GetContext(ctx context.Context, id string) (string, error) to the LeadStore interface.

Implement it on sqliteLeadStore:
  - Query: SELECT context_text FROM v_lead_context WHERE id = $1
  - Use Rebind() for the placeholder
  - Scan into a string and return it
  - Return a wrapped error if no row found

The view is Postgres-only. For SQLite (DriverName != "pgx"), return an empty string
and nil error — callers should fall back gracefully.
```

---

## Task 3 — Refactor `DraftOutreach()` to use `GetContext()`

**Files to edit:**
- `internal/enrichment/claude.go`
- `internal/enrichment/enricher.go` (if `DraftOutreach` call site needs updating)

```
Current DraftOutreach() signature:
  func (c *ClaudeEnricher) DraftOutreach(ctx context.Context, l storage.Lead) (string, error)

It currently builds the prompt string with fmt.Sprintf from l.Title, l.Location,
l.GeneralContractor, l.ProjectType, l.PriorityReason, l.Notes.

Change:
1. Add a contextStore storage.LeadStore field to ClaudeEnricher (or pass context_text
   as a parameter — prefer the parameter approach to keep ClaudeEnricher stateless).

2. Change DraftOutreach signature to:
   func (c *ClaudeEnricher) DraftOutreach(ctx context.Context, l storage.Lead, contextText string) (string, error)

3. If contextText is non-empty, use it as the "Lead Details" block in the prompt.
   If empty (SQLite / view unavailable), fall back to the current fmt.Sprintf approach
   so SQLite local dev still works.

4. Update the EnricherAI interface in enricher.go to match the new signature.

5. Update the call site in cmd/server/main.go or wherever DraftOutreach is called
   to first call leadStore.GetContext(ctx, l.ID) and pass the result in.
```

---

## Task 4 — Wire `GetContext()` into the outreach draft endpoint

**File to edit:** `cmd/server/main.go`

```
Find the /n8n/webhook or /run handler (or wherever DraftOutreach is invoked).

Before calling DraftOutreach, call:
  contextText, _ := leadStore.GetContext(ctx, lead.ID)

Pass contextText as the third argument to DraftOutreach.

If the call site doesn't exist yet (DraftOutreach is not wired to an HTTP endpoint),
add a POST /draft endpoint:
  - Reads lead_id from query param or JSON body
  - Fetches the lead from the DB (add GetByID to LeadStore if needed)
  - Calls GetContext to get the context string
  - Calls DraftOutreach with the result
  - Returns the draft email text as plain text response
```

---

## Phase B hook (future — do not implement now)

When Phase B (RAG / embeddings) is implemented, `GetContext()` will be called for
*similar past leads* to inject historical context before Claude enriches a new permit:

```go
// In enricher.go processProject(), before calling ai.Enrich():
similarIDs, _ := embStore.Similar(ctx, newEmbedding, 3)
var historicalContext []string
for _, id := range similarIDs {
    if ct, err := leadStore.GetContext(ctx, id); err == nil && ct != "" {
        historicalContext = append(historicalContext, ct)
    }
}
// Pass historicalContext into the enrichment prompt
```

No changes needed now — just keep `GetContext()` in mind as the RAG retrieval step.

---

## Testing checklist

After implementing Tasks 1–4:

```bash
# 1. Rebuild with new migration
docker compose up -d --build

# 2. Verify the view was created
docker exec -it groupscout-postgres-1 psql -U groupscout -d groupscout \
  -c "SELECT id, source, LEFT(context_text, 120) FROM v_lead_context LIMIT 5;"

# 3. Trigger a pipeline run to generate leads
curl -X POST http://localhost:8080/run \
  -H "Authorization: Bearer YOUR_API_TOKEN"

# 4. Test the draft endpoint (once wired)
curl -X POST "http://localhost:8080/draft?lead_id=SOME_UUID" \
  -H "Authorization: Bearer YOUR_API_TOKEN"
```

---

## Reference files

| File | Role |
|---|---|
| `internal/enrichment/claude.go` | All `*Prompt()` functions + `DraftOutreach()` |
| `internal/enrichment/enricher.go` | `EnricherAI` interface |
| `internal/storage/leads.go` | `LeadStore` interface + `sqliteLeadStore` impl |
| `migrations/001_init.postgres.up.sql` | Current schema (leads + raw_projects columns) |
| `migrations/003_pgvector.up.sql` | Vector table — next migration is `004` |
| `docs/AI_DATA_STRATEGY.md` | Full design + RAG plan |
