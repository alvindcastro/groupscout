# UI_STRATEGY.md - GroupScout operator UI

> Planning source of truth for the future GroupScout admin/operator interface.
> This is a product and integration plan, not an implementation record.
> UI-specific Markdown should live in `docs/planning/ui/` or include `UI` in the filename.

---

## Current Position

GroupScout already has a strong back-end pipeline: collectors, raw input audit storage, AI enrichment, Slack/email notification, lead persistence, and the separate `alertd` airport disruption monitor.

The current human interface is Slack plus email. That is good for interruption and daily rhythm, but it is weak as the durable workspace for triage, verification, owner assignment, outreach history, and analytics.

The UI strategy is therefore:

- Keep Slack as the alerting and quick-action channel.
- Add a small operator web UI as the system of record for review, ownership, source evidence, and outcomes.
- Build the UI behind explicit `/api/*` contracts instead of letting browser code call automation endpoints or query the database directly.
- Ship lead triage before broad analytics, CRM sync, or auto-sending outreach.

---

## Users And Jobs

| User | Core jobs |
|---|---|
| Hotel sales rep | Triage new leads, claim ownership, review evidence, start outreach, log outcome. |
| Sales manager | See pipeline coverage, aging leads, source quality, won/lost outcomes, and upcoming demand. |
| Revenue or ops lead | Monitor airport disruption alerts, room inventory, and suggested actions. |
| System operator | Check collector runs, failed webhooks, AI quality, cost, latency, and source drift. |

The first UI should serve sales reps and managers. `alertd` can appear later as a read-only console or stay Slack-first until the lead workflow is stable.

---

## Operator Flow

### Daily lead triage

1. A scheduled n8n workflow, cron, or manual API call triggers `POST /run`.
2. GroupScout collects sources, deduplicates raw records, stores audit inputs, enriches qualified leads, and posts Slack/email notifications.
3. The operator opens the lead inbox or follows a Slack link to the lead detail.
4. The operator reviews score, timing, room-night signal, source, rationale, and raw audit evidence.
5. The operator chooses one next action: claim, dismiss, snooze, or flag for verification.
6. Claimed leads move to outreach. The operator edits the draft, finds or confirms contact details, and logs contacted/won/lost/no-response outcomes.
7. Weekly analytics roll up source yield, hit rate, outcomes, aging leads, and demand timing.

### Verification flow

1. A lead with weak evidence, conflicting fields, or a high score on low-confidence input is flagged.
2. The reviewer opens the raw audit payload through `GET /leads/{id}/raw` or its future authenticated `/api/*` alias.
3. The reviewer either verifies the lead, corrects specific fields, or dismisses it.
4. Corrections should preserve original AI output plus reviewer edits for auditability. Do not silently overwrite source-backed extraction without recording who changed what and why.

### Disruption alert flow

1. `alertd` monitors weather, YVR flight state, and NOTAMs.
2. Slack remains the primary interrupt channel for watch/alert/update/resolve messages.
3. A future UI can show current SPS, evidence, room inventory, alert state, and action history, but it should not block the lead-management MVP.

---

## Information Architecture

| Area | Purpose |
|---|---|
| Today | Command center for new high-score leads, aging claimed leads, active disruption alerts, failed jobs. |
| Leads | Filterable table by status, score, source, segment, property, owner, and timing. |
| Lead Detail | Evidence, raw audit link, enrichment rationale, outreach draft, notes, owner, status transitions. |
| Verification Queue | Leads needing source review, corrections, reviewer notes, or confidence checks. |
| Outreach | Editable draft, contact fields, outreach attempt log, outcome capture. |
| Demand Calendar | Upcoming demand by week, segment, property, crew estimate, and likely room-night value. |
| Sources And Runs | Collector health, last run, failures, source yield, parse warnings, manual run controls. |
| Analytics | Source attribution, claimed/won/lost rates, score distribution, lead aging, verification quality. |
| Settings | Properties, thresholds, Slack/email routing, CRM settings, API/session configuration. |

---

## Primary Screens

### Lead Inbox

Dense operational table. Default sort should put urgent, unowned, high-score leads at the top.

Phase 33 keeps this route mocked until `GET /api/leads` exists. The first frontend implementation should load deterministic lead summary fixtures through a replaceable data adapter that mirrors the planned `/api/leads` query and response shape. Do not wire the inbox to automation endpoints, direct database reads, or raw audit payload bodies.

Expected columns:

- Score
- Title
- Segment or project type
- Location/property fit
- Source
- Estimated crew and duration
- Suggested outreach timing
- Status
- Owner
- Created date
- Evidence or verification state

Expected controls:

- Search by title, organization, location, source URL, or notes.
- Filters for status, source, min score, created date, property, owner, and verification state.
- Bulk actions only after the status model is stable.

### Lead Detail

The detail view is where AI output becomes reviewable evidence.

Phase 34 keeps this route mocked until `GET /api/leads/{id}` exists. The first frontend implementation should load deterministic detail fixtures through a replaceable adapter that mirrors the planned `{lead, audit, outreach_summary, activity}` response shape. The default detail load must not include raw payload bodies; raw audit access stays a deliberate link to the existing `GET /leads/{id}/raw` behavior or its future authenticated `/api/leads/{id}/raw` alias.

Required sections:

- Summary: title, score, timing, room-night signal, property fit.
- Source evidence: source name, source URL, raw audit link, collected timestamp.
- AI enrichment: contractor/applicant, project type, crew size, duration, rationale, uncertainty.
- Actions: claim, dismiss, snooze, flag for verification, contacted, won, lost.
- Outreach: editable draft, contact fields, copied/sent/logged state.
- Activity: status history, notes, outreach attempts, reviewer corrections.

### Verification Queue

Use this for leads that are high value but not trustworthy enough for immediate outreach.

Queue triggers can include:

- Missing source URL or raw audit record.
- High priority score with weak rationale.
- Contradiction between raw source and enriched fields.
- Low confidence collector parse.
- Manual operator flag.

### Pipeline Monitor

This should stay compact. It exists to answer whether the system is healthy, not to replace Grafana.

Show:

- Last run time and result.
- Collector-level collected/skipped/enriched counts.
- Recent collector failures.
- LLM provider, latency, and error counts.
- Slack/email/webhook delivery failures.
- Links to logs or Grafana where available.

---

## Lead State Model

The code and docs currently mention several overlapping states. Before UI implementation, converge on one state machine.

Recommended v1:

| State | Meaning | Allowed next actions |
|---|---|---|
| new | Created but not reviewed by a human. | claim, dismiss, snooze, flag |
| notified | Sent to Slack/email but not yet owned. | claim, dismiss, snooze, flag |
| claimed | Owned by a rep. | contacted, won, lost, no_response, snooze |
| contacted | Outreach has happened. | won, lost, no_response, follow_up |
| snoozed | Temporarily hidden until a future date. | reopen, dismiss |
| flagged | Needs source or quality review. | verified, corrected, dismiss |
| verified | Source-backed and trusted. | claim, dismiss |
| dismissed | Not actionable. | reopen |
| won | Converted to business. | reopen |
| lost | Closed without business. | reopen |
| no_response | Outreach attempted without reply. | follow_up, lost, snooze |

Implementation note: verification status may become a separate field instead of a lead status. If so, keep commercial workflow status and source-review status independent.

---

## Frontend Architecture

Recommended implementation:

- Add a future `web/` app with TypeScript, React, React Router, TanStack Query, and an OpenAPI-generated client.
- Keep the UI same-origin in production where possible. Either the Go server serves the built assets, or a small web container/proxy serves assets and forwards `/api/*` to the Go server.
- Use `api/swagger.yaml` as the contract source before generating frontend types.
- Keep all database access behind Go storage APIs. The browser never queries Postgres/SQLite directly.
- Do not expose `API_TOKEN` to browser JavaScript. It is for automation clients such as n8n, not end-user sessions.

Current separate UI repo note: `/mnt/c/Users/alvin/WebstormProjects/groupscout-ui` now has a D4 lightweight Node production server that serves `web/dist` and proxies `/api/*` server-side to `http://groupscout:8080`. The product renderer and backend `/api/*` implementations are still planned, so this is a same-origin runtime boundary, not a completed operator UI.

Suggested config when implemented:

| Setting | Purpose |
|---|---|
| `UI_ENABLED` | Enables serving the built UI in deployments that want it. |
| `UI_BASE_PATH` | Supports `/` or `/admin` mounting. |
| `UI_SESSION_SECRET` | Signs admin sessions if built-in auth is used. |
| `CORS_ALLOWED_ORIGINS` | Development-only cross-origin access if the Vite server is separate. |

---

## API Contracts Needed For UI

Existing useful endpoints:

| Endpoint | Status | UI use |
|---|---|---|
| `GET /health` | Implemented | Basic uptime and DB check. |
| `GET /metrics` | Implemented | Prometheus/Grafana, not direct UI data. |
| `POST /run` | Implemented | Automation trigger; wrap before exposing in browser UI. |
| `POST /digest` | Implemented | Digest trigger; wrap before exposing in browser UI. |
| `POST /n8n/webhook` | Implemented | External lead ingestion. |
| `GET /leads/{id}/raw` | Implemented | Raw audit review; needs auth/session wrapper for UI. |

Minimum UI-facing `/api/*` contracts:

| Endpoint | Purpose |
|---|---|
| `GET /api/leads?status=&source=&min_score=&q=&limit=&cursor=` | Lead inbox with filters and pagination. |
| `GET /api/leads/{id}` | Lead detail with enrichment, status, notes, and audit metadata. |
| `PATCH /api/leads/{id}` | Status, owner, notes, snooze date, and safe field corrections. |
| `GET /api/leads/{id}/raw` | Authenticated alias for raw audit payload. |
| `POST /api/leads/{id}/outreach` | Log outreach attempt, channel, contact, notes, and outcome. |
| `GET /api/leads/{id}/outreach` | Show activity history on the lead detail. |
| `POST /api/pipeline/runs` | Start a run without blocking the browser for the whole pipeline. |
| `GET /api/pipeline/runs` | Recent run history and run status. |
| `GET /api/stats` | Counts by status, source, score band, owner, and week. |
| `GET /api/system` | UI-friendly health summary built from existing health/metrics signals. |

---

## MVP Scope

Build in this order:

1. Lead list/detail API with filtering, pagination, status updates, and notes.
2. Outreach log API around the existing `outreach_log` table.
3. OpenAPI update and generated TypeScript client.
4. React lead inbox with filters and detail view.
5. Raw audit evidence viewer or safe download/open link.
6. Status actions: claim, dismiss, snooze, flag, contacted, won, lost.
7. Pipeline controls and compact health panel.
8. Basic source/status analytics.
9. Session/auth wrapper and same-origin deployment.

Out of scope for the first UI:

- Full CRM replacement.
- Auto-sending outreach emails.
- Complex multi-user permission matrix.
- Custom dashboard builder.
- Direct database access from the browser.
- Portfolio-wide multi-property UX beyond fields needed by the lead table.
- Full `alertd` console unless lead triage is already stable.

---

## Visual And Interaction Principles

- Build an operations tool, not a marketing site.
- Prefer dense tables, detail drawers, tabs, segmented filters, menus, and keyboard-friendly controls.
- Use color sparingly for status, priority, verification confidence, and alert severity.
- Always pair AI claims with source evidence and reviewer controls.
- Make common workflows one or two clicks: claim, dismiss, snooze, log contacted, mark won/lost.
- Preserve context when moving from Slack to web UI. Slack links should land on the exact lead detail.
- Keep analytics explainable. Source hit rate should be based on explicit outcome definitions, not vague engagement.

---

## Documentation Ownership

| Doc | Ownership |
|---|---|
| `docs/planning/ui/UI_STRATEGY.md` | Product flow, screen map, frontend architecture, and UI API plan. |
| `docs/planning/ui/UI_DESIGN_SYSTEM.md` | Visual and interaction adaptation of the external `groupscout-ui/DESIGN.md` reference. |
| `docs/planning/ui/UI_PHASE31_DESIGN_SYSTEM_CONTRACT.md` | Phase 31 token, primitive, component-test, accessibility, static-asset safety, and no-marketing-hero contract. |
| `docs/planning/ui/UI_PHASE32_APP_SHELL_ROUTING_CONTRACT.md` | Phase 32 app shell, route map, responsive navigation, right-rail capacity, build, and component-test contract. |
| `docs/planning/ui/UI_PHASE33_MOCKED_LEAD_INBOX_CONTRACT.md` | Phase 33 mocked lead inbox fixture, table, interaction, accessibility, responsive, and replaceable API-boundary contract. |
| `docs/planning/ui/UI_PHASE34_LEAD_DETAIL_EVIDENCE_CONTRACT.md` | Phase 34 lead detail, evidence review, raw audit safety, outreach/activity, status action, and correction-control contract. |
| `docs/planning/ui/UI_API_ENDPOINTS.md` | Brainstormed browser-facing `/api/*` endpoint contracts and schema gaps. |
| `docs/planning/ui/UI_TDD_PHASE_PLAN.md` | Phase-by-phase strict TDD checklist for UI and backend contract implementation. |
| `docs/planning/ui/BACKEND_FOR_UI_TESTING.md` | Backend startup paths, current endpoint status, and frontend proxy notes for UI testing. |
| `docs/planning/ui/BACKEND_FRONTEND_DOCKER_E2E.md` | Current Docker smoke path for running the backend stack with the separate UI D4 static/proxy image. |
| `docs/prompts/PROMPTS_PHASE31_UI.md` | Copy-paste TDD prompts for future UI and `/api/*` implementation slices. |
| `docs/prompts/PROMPTS_PHASE32_UI.md` | Copy-paste TDD prompts for Phase 32 route, app shell, responsive layout, build, and component-test implementation. |
| `docs/prompts/PROMPTS_PHASE33_UI.md` | Copy-paste TDD prompts for Phase 33 mocked lead inbox implementation. |
| `docs/prompts/PROMPTS_PHASE34_UI.md` | Copy-paste TDD prompts for Phase 34 lead detail and evidence review implementation. |
| `docs/planning/PHASES.md` | Atomic implementation checklist. |
| `docs/planning/ROADMAP.md` | Big-picture milestones and sequencing. |
| `docs/DATA_FLOW.md` | System and human workflow diagram. |
| `docs/API_CONFIG.md` | Implemented and planned endpoint contracts. |
| `docs/guides/VERIFICATION.md` | Operator/source review runbooks. |
| `docs/guides/POSTGRES_QUERIES.md` | Manual triage, status, and analytics queries until the UI exists. |

---

## Open Decisions

- Should v1 ship Slack actions first, the admin UI first, or both together?
- Is verification a lead status or a separate source-review status?
- Who can claim, verify, correct, dismiss, reopen, and mark won/lost?
- Should corrected fields overwrite lead columns, live in a corrections table, or both?
- Is built-in cookie auth required, or will the UI sit behind an auth proxy?
- Is GroupScout or the future CRM the source of truth for outreach outcomes?
- Does `alertd` appear in the same UI, or remain Slack/runbook-only for the first release?
- What exactly counts as source hit rate: claimed/total, won/claimed, won/total, or another metric?
- What raw audit payloads can operators view, and what must be redacted before display?
