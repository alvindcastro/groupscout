# Phase 35 UI API Contract

> Implemented backend contract for the first same-origin GroupScout operator UI endpoints.
> The current repository still has no checked-in frontend package, so frontend types are maintained in `UI_PHASE35_FRONTEND_TYPES.md` until a generated client exists.

## Scope

Phase 35 turns the Phase 33 and Phase 34 fixture boundaries into live `/api/*` backend contracts backed by the current lead and raw-audit schema.

Implemented:

- `GET /api/leads`
- `GET /api/leads/{id}`
- `PATCH /api/leads/{id}`
- `GET /api/leads/{id}/raw`

Not implemented in this phase:

- `owner`, `snoozed_until`, `verification_state`, and `corrections`, because they require migration-backed storage design.
- Outreach history endpoints, which belong to Phase 36.
- Pipeline run, stats, and system endpoints, which belong to Phase 37.

## `GET /api/leads`

Lists lead summaries for the inbox.

Query parameters:

- `status`: exact lead workflow status.
- `source`: exact collector/source name.
- `min_score`: minimum `priority_score`, integer `0` or higher.
- `q`: case-insensitive search across title, location, and priority reason.
- `limit`: page size from `1` to `100`; storage defaults to `50`.
- `cursor`: opaque pagination cursor. The current implementation uses an offset string and may change behind the same response field.

Response:

```json
{
  "items": [
    {
      "id": "uuid",
      "title": "Airport hotel tower",
      "source": "richmond_permits",
      "location": "Richmond, BC",
      "project_value": 12000000,
      "priority_score": 9,
      "priority_reason": "Large hotel-adjacent construction near YVR",
      "status": "new",
      "created_at": "2026-05-09T00:00:00Z",
      "updated_at": "2026-05-09T00:00:00Z",
      "has_raw": true,
      "audit_source_url": "https://example.test/permit"
    }
  ],
  "next_cursor": "",
  "filters": {
    "status": "new",
    "source": "richmond_permits",
    "min_score": 8,
    "q": "airport",
    "limit": 25,
    "cursor": ""
  }
}
```

The summary response intentionally omits `raw_input_id` and raw payload bodies.

## `GET /api/leads/{id}`

Returns the detail payload for the operator detail screen.

Response shape:

```json
{
  "lead": {
    "id": "uuid",
    "source": "richmond_permits",
    "title": "Airport hotel tower",
    "location": "Richmond, BC",
    "project_value": 12000000,
    "general_contractor": "PCL",
    "applicant": "",
    "contractor": "",
    "source_url": "https://example.test/permit",
    "project_type": "",
    "estimated_crew_size": 0,
    "estimated_duration_months": 0,
    "out_of_town_crew_likely": false,
    "priority_score": 9,
    "priority_reason": "Large hotel-adjacent construction near YVR",
    "rationale": "Crew lodging likely.",
    "suggested_outreach_timing": "",
    "notes": "Review this week",
    "status": "new",
    "created_at": "2026-05-09T00:00:00Z",
    "updated_at": "2026-05-09T00:00:00Z"
  },
  "audit": {
    "has_raw": true,
    "raw_link": "/api/leads/{id}/raw",
    "payload_type": "application/json",
    "source_url": "https://example.test/source",
    "collector_name": "richmond_permits",
    "collected_at": "2026-05-09T00:00:00Z"
  },
  "outreach_summary": {
    "count": 0,
    "latest_at": null
  },
  "activity": []
}
```

The detail response intentionally omits raw payload bodies and internal raw IDs. Operators use the authenticated `raw_link` only when they explicitly need the original evidence.

## `PATCH /api/leads/{id}`

Updates safe operator-editable fields that are backed by the current schema.

Allowed request fields:

```json
{
  "status": "contacted",
  "notes": "Called GC"
}
```

Rejected fields:

- `owner`
- `snoozed_until`
- `verification_state`
- `corrections`
- source-backed extracted fields such as `title`, `project_value`, `source_url`, `applicant`, and `contractor`
- raw audit identifiers or payload fields

Response:

```json
{
  "lead": {
    "id": "uuid",
    "status": "contacted",
    "notes": "Called GC"
  },
  "changed_fields": ["status", "notes"],
  "updated_at": "2026-05-09T00:00:00Z"
}
```

The actual `lead` object in the response uses the full safe detail shape from `GET /api/leads/{id}`.

## `GET /api/leads/{id}/raw`

Returns raw audit bytes for a lead. This endpoint is authenticated when `API_TOKEN` is configured and returns `401` for missing or incorrect bearer tokens.

Responses:

- `200`: raw bytes with the stored `Content-Type`.
- `401`: missing or invalid bearer token when `API_TOKEN` is configured.
- `404`: lead not found, lead has no raw input, or the audit row is missing.
- `500`: internal storage inconsistency, such as an invalid stored raw input ID.

This is a UI alias for raw evidence and keeps browser-visible JavaScript away from `API_TOKEN`. A deployed browser UI should call it through a server-side session boundary or auth proxy.

## Test Evidence

Red evidence:

- `go test ./internal/storage` failed because `LeadListFilter` and `ListFiltered` did not exist.
- `go test ./cmd/server` failed because `newUIAPIHandler` did not exist.

Green evidence:

- `go test ./internal/storage`
- `go test ./cmd/server`

Residual risk and follow-up:

- `groupscout-bxt`: migration-backed UI workflow fields for owner, snooze, verification, corrections, and pipeline run history.
- `groupscout-al3`: Phase 36 outreach and lead state API contracts.
