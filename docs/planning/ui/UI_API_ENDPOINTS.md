# UI API Endpoint Brainstorm

> Brainstormed API contracts for the future GroupScout operator UI.
> This is not an implementation record. Production code must be built through the strict TDD prompt pack in `../../prompts/PROMPTS_PHASE31_UI.md`.

## Current Backend Endpoints

| Endpoint | Status | UI note |
|---|---|---|
| `GET /health` | Implemented | Returns JSON with `status`, `database`, and `ollama`. Swagger currently describes a plain-text response, so contract docs need correction before generated clients rely on it. |
| `GET /metrics` | Implemented | Prometheus output. Useful for observability, not a direct UI data source. |
| `POST /run` | Implemented | Bearer-token protected when `API_TOKEN` is set. Blocking pipeline trigger. |
| `POST /digest?to=` | Implemented | Bearer-token protected when `API_TOKEN` is set. Sends email digest. |
| `POST /n8n/webhook` | Implemented | Bearer-token protected when `API_TOKEN` is set. Accepts a lead-shaped payload. |
| `GET /leads/{id}/raw` | Implemented | Raw audit payload lookup. No UI auth/session wrapper yet. |
| `POST /slack/inventory` | Implemented in `alertd` | Slack slash-command endpoint, separate from the lead UI MVP. |

## Contract Principles

- Browser-facing routes should live under same-origin `/api/*`.
- Browser code must not receive `API_TOKEN`.
- API responses should be JSON except raw audit payload downloads or previews.
- Pagination should use cursors before offsets if the lead list will grow.
- Field corrections must preserve original extracted values and reviewer edits. Do not silently overwrite source-backed extraction.
- Commercial workflow state and verification state may need to be separate fields.

## Proposed Endpoints

| Endpoint | Purpose | Request | Response |
|---|---|---|---|
| `GET /api/leads` | Lead inbox | `status`, `source`, `min_score`, `q`, `owner`, `verification`, `limit`, `cursor` query params | `{items:[lead_summary], next_cursor, total_estimate?, filters}` |
| `GET /api/leads/{id}` | Lead detail | path ID | `{lead, audit, outreach_summary, activity}` |
| `PATCH /api/leads/{id}` | Safe lead update | `{status?, notes?, owner?, snoozed_until?, corrections?}` | `{lead, changed_fields, updated_at}` |
| `GET /api/leads/{id}/raw` | Authenticated raw evidence alias | path ID, optional `mode=download|preview` | raw bytes or `{payload_type, source_url, collector_name, collected_at, payload_preview}` |
| `GET /api/leads/{id}/outreach` | Outreach history | path ID, `limit`, `cursor` | `{items:[outreach_event], next_cursor}` |
| `POST /api/leads/{id}/outreach` | Log outreach attempt | `{contact, channel, notes, outcome}` | `{outreach, lead}` |
| `POST /api/pipeline/runs` | Browser-safe run trigger | `{sources?, bcbid_raw_input?, dry_run?}` | `{run_id, status, started_at}` |
| `GET /api/pipeline/runs` | Run history | `status`, `limit`, `cursor` | `{items:[pipeline_run], next_cursor}` |
| `GET /api/stats` | UI summaries | `window`, `group_by` | `{window, counts, series, score_bands}` |
| `GET /api/system` | UI health summary | none | `{status, database, ollama, metrics_available, version?, last_pipeline_run?}` |

## Response Shape Notes

`lead_summary` should include:

- `id`
- `title`
- `source`
- `location`
- `project_value`
- `priority_score`
- `priority_reason`
- `status`
- `owner`
- `verification_state`
- `created_at`
- `updated_at`
- `has_raw`
- `audit_source_url`

`lead` should include full current storage fields plus UI-safe audit metadata. Do not include raw payload bodies by default.

`outreach_event` should include:

- `id`
- `lead_id`
- `contact`
- `channel`
- `notes`
- `outcome`
- `logged_at`

`pipeline_run` should include:

- `id`
- `status`
- `started_at`
- `finished_at`
- `sources`
- `counts`
- `errors`

## Schema Gaps To Handle Explicitly

The current database schema supports core lead fields, status, notes, raw input IDs, and `outreach_log`. It does not yet support every planned UI field.

| Planned UI field | Current status | Planning action |
|---|---|---|
| `owner` | Missing | Add through a migration-backed phase. |
| `snoozed_until` | Missing | Add through a migration-backed phase. |
| `verification_state` | Missing | Decide separate verification field versus status value before implementation. |
| `corrections` | Missing | Needs audit-safe correction model before UI edit controls are live. |
| `pipeline_runs` | Missing | Needs persistence or an explicitly temporary dev-only run tracker. |
