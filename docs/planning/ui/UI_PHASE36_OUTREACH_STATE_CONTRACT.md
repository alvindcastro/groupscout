# Phase 36 Outreach And Lead State Contract

Phase 36 adds schema-backed outreach logging and validated operator lead actions to the UI API.

Implemented endpoints:

- `GET /api/leads/{id}/outreach`
- `POST /api/leads/{id}/outreach`
- `PATCH /api/leads/{id}` with action payloads

## Outreach

`POST /api/leads/{id}/outreach` accepts:

```json
{
  "contact": "gc@example.test",
  "channel": "email",
  "notes": "Sent intro",
  "outcome": "sent"
}
```

The response is `{outreach, lead}`. `GET /api/leads/{id}/outreach` returns `{items, next_cursor}` ordered newest first.

## Lead Actions

`PATCH /api/leads/{id}` now supports action payloads:

```json
{
  "action": "claim",
  "owner": "alex@example.test",
  "notes": "Taking this lead"
}
```

Supported actions:

- `claim`
- `dismiss`
- `snooze`
- `flag`
- `contacted`
- `won`
- `lost`
- `no-response`
- `reopen`

Action validation rejects unsupported actions, missing claim owners, missing or past snooze dates, and transitions from terminal states except `reopen`. Verification state is stored separately as `verification_state` and is not changed by commercial workflow actions.

## Storage

Phase 36 adds lead workflow fields:

- `owner`
- `snoozed_until`
- `flagged`
- `verification_state`

It also adds `OutreachStore` over the existing `outreach_log` table.

## Evidence

Red evidence:

- `go test ./internal/storage` failed on missing `NewOutreachStoreWithDSN`, `OutreachEvent`, `LeadAction`, and workflow fields.
- `go test ./cmd/server` failed on missing `VerificationState` and outreach/action routes.

Green evidence:

- `go test ./internal/storage`
- `go test ./cmd/server`

