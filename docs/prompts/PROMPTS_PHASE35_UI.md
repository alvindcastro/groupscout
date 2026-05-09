# Phase 35 UI API Contract Prompts

Use these prompts only for follow-up refinement of the implemented Phase 35 backend contract. Beads remains canonical for live task tracking.

## 35-A Storage Filters

Write failing tests first for `LeadStore.ListFiltered` before changing storage code.

Acceptance:

- Filters by `status`, `source`, `min_score`, and `q`.
- Orders by `priority_score DESC`, `created_at DESC`, then `id ASC`.
- Supports `limit` and `cursor`.
- Rejects invalid cursors.
- Does not expose raw payload bodies.

## 35-B Lead HTTP Contracts

Write failing HTTP tests first for `GET /api/leads`, `GET /api/leads/{id}`, `PATCH /api/leads/{id}`, and `GET /api/leads/{id}/raw`.

Acceptance:

- `GET /api/leads` returns `{items,next_cursor,filters}` with safe lead summaries.
- `GET /api/leads/{id}` returns `{lead,audit,outreach_summary,activity}` without raw payload body or internal raw IDs.
- `PATCH /api/leads/{id}` allows only `status` and `notes` until schema-backed owner, snooze, verification, and correction fields exist.
- Unsafe fields are rejected with `400` and do not change stored source-backed data.
- `GET /api/leads/{id}/raw` requires bearer auth when `API_TOKEN` is configured.

## 35-C Contract Docs And Types

Update `api/swagger.yaml`, `UI_PHASE35_API_CONTRACT.md`, and `UI_PHASE35_FRONTEND_TYPES.md` only after tests pin behavior.

Acceptance:

- OpenAPI matches implemented status codes and response shapes.
- Frontend type docs match snake_case JSON responses.
- Schema gaps are documented instead of silently modeled as working fields.
- Changelog records what, where, when, why, and how for the task.

