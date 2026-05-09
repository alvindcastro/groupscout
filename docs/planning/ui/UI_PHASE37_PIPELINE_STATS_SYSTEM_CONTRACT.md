# Phase 37 Pipeline Runs, Stats, And System Contract

Phase 37 adds UI-safe operational endpoints without requiring the browser to parse Prometheus `/metrics`.

Implemented endpoints:

- `POST /api/pipeline/runs`
- `GET /api/pipeline/runs`
- `GET /api/stats`
- `GET /api/system`

## Pipeline Runs

`POST /api/pipeline/runs` persists a run record and returns `202 Accepted` immediately:

```json
{
  "run_id": "uuid",
  "status": "running",
  "started_at": "2026-05-09T00:00:00Z"
}
```

The server starts the real pipeline asynchronously in production. Tests inject a blocking fake runner to prove the HTTP response does not wait for the runner to finish.

`GET /api/pipeline/runs` supports:

- `status`
- `limit`
- `cursor`

Runs are ordered by `started_at DESC`, then `created_at DESC`, then `id ASC`.

## Stats

`GET /api/stats` returns supported current-schema summaries:

- `by_status`
- `by_source`
- `score_bands`
- `by_owner`
- `by_week`
- `by_outcome`

## System

`GET /api/system` returns a UI summary with:

- `status`: `healthy` or `degraded`
- `database`
- `ollama`
- `metrics_available`
- `last_pipeline_run`

The endpoint reports metrics availability as a server capability flag. Browser UI must not parse `/metrics` directly.

## Evidence

Red evidence:

- `go test ./internal/storage` failed on missing pipeline run and stats stores.
- `go test ./cmd/server` failed on missing pipeline runner types and `/api/pipeline/runs`, `/api/stats`, and `/api/system` routes.

Green evidence:

- `go test ./internal/storage`
- `go test ./cmd/server`

