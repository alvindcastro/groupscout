# Phase 33 Mocked Lead Inbox Contract

> Implementation contract for the future GroupScout lead inbox.
> This repository still has no checked-in frontend package, UI test harness, or static UI build directory, and the backend does not expose `GET /api/leads` today. Phase 33 is complete here as a mocked-inbox contract. The first frontend package must convert this contract into failing table, fixture, interaction, accessibility, and API-boundary tests before production UI code.

## Current Scope

Phase 33 defines the `/leads` route behavior after the Phase 32 app shell exists and before the Phase 35 live `/api/leads` contract exists.

Do not add production UI code that silently calls existing automation endpoints, reads the database, or depends on `GET /api/leads`. Until Phase 35 implements the UI API, the lead inbox must use deterministic fixture data behind a replaceable data boundary.

## Mock Data Boundary

The first frontend implementation must keep fixtures and UI data access behind a narrow boundary so a generated or typed `/api/leads` client can replace it later without rewriting the table.

| Boundary item | Required behavior | Acceptance |
|---|---|---|
| Fixture source | Lead inbox data comes from local deterministic fixtures only. | Tests can switch between loading, populated, empty, and error fixtures without network calls. |
| Data adapter | The inbox route calls a local adapter such as `listLeadSummaries(params)` instead of importing fixture arrays directly in table components. | Replacing the adapter with a generated `/api/leads` client preserves component props and test coverage. |
| Query shape | Search, filters, sort, limit, and cursor-like fields are represented in typed params that mirror the planned `GET /api/leads` query. | No browser code references `POST /run`, `POST /n8n/webhook`, direct SQL, or server-only config. |
| Response shape | The fixture response mirrors `{items, next_cursor, total_estimate?, filters}` from `UI_API_ENDPOINTS.md`. | Empty and error responses use the same boundary as populated responses. |
| Static safety | Browser-visible code uses relative `/api/*` paths only when a live client is later introduced. | Static asset scans fail on `API_TOKEN`, database URLs, Slack/email/LLM keys, session secrets, or real `.env` values. |

## Lead Summary Fixture Shape

Fixtures should include the fields needed to test dense lead triage while staying compatible with the planned `lead_summary` response.

| Field | Required in mocked inbox | Notes |
|---|---|---|
| `id` | Yes | Stable link target for `/leads/:leadId`; use deterministic fake IDs. |
| `title` | Yes | Primary row label and search target. |
| `source` | Yes | Filter and source badge. |
| `source_url` or `audit_source_url` | Yes when available | Search target and evidence indicator; do not load raw payloads. |
| `status` | Yes | Commercial workflow state such as `new`, `notified`, `claimed`, `contacted`, `snoozed`, `flagged`, `dismissed`, `won`, `lost`, or `no_response`. |
| `priority_score` | Yes | Sortable score column and score band badge. |
| `priority_reason` | Recommended | Tooltip or detail-ready metadata; not required as a visible table column. |
| `owner` | Yes, nullable | Use `Unassigned` or an explicit empty owner state in UI copy. Current backend schema does not yet persist this field. |
| `verification_state` | Yes | Separate from `status`; examples: `unverified`, `source_backed`, `missing_evidence`, `weak_confidence`, `contradiction`, `review_requested`. |
| `created_at` | Yes | Sortable created-date column. |
| `suggested_outreach_timing` | Yes | Timing column for outreach urgency. |
| `location` | Recommended | Search and property-fit display. |
| `project_type` | Recommended | Segment/type label. |
| `estimated_crew_size` | Recommended | Supports later value and room-night cues. |
| `estimated_duration_months` | Recommended | Supports timing and room-night cues. |
| `has_raw` | Yes | Indicates whether raw audit evidence exists without loading it. |

Fixture data must include at least:

- one high-score unassigned new lead;
- one claimed lead with an owner;
- one lead with missing evidence;
- one lead with weak confidence;
- one lead with a contradiction state;
- one lower-score or dismissed lead for filtering and sorting contrast;
- one source URL or note value that proves search reaches non-title fields.

## Table Contract

The lead inbox is table-first. It must not degrade into a marketing page, hero section, or decorative card grid.

| Column | Required behavior | Acceptance |
|---|---|---|
| Score | Sortable numeric column with a text score or score band. | Operators can sort ascending and descending; color is not the only score cue. |
| Lead | Title plus compact secondary metadata such as segment/type or location. | Title is keyboard reachable through row selection or an explicit detail link. |
| Status | Commercial workflow status. | Status text or icon/shape distinguishes states; not color alone. |
| Source | Source badge or text plus evidence availability. | Source filter values match source labels. |
| Owner | Owner name or unassigned state. | Unassigned is explicit and searchable/filterable where the control supports owner. |
| Verification | Verification state separate from commercial status. | Missing evidence, weak confidence, and contradiction states are visible by default. |
| Created | Sortable created date. | Uses stable test data and deterministic formatting. |
| Timing | Suggested outreach timing or urgency. | Empty timing is represented intentionally, not by a blank ambiguous cell. |
| Row actions | Link to detail and future action capacity. | Actions have accessible names and do not require live backend mutations. |

Default ordering should put urgent, unowned, high-score leads before lower-score, owned, dismissed, or stale leads. Tests must define the exact deterministic order for the fixture set.

## Interaction Contract

| Interaction | Required behavior | Acceptance |
|---|---|---|
| Loading state | Route shows shell, table skeleton or loading rows, and disabled controls where appropriate. | Loading does not remove navigation, heading, filter labels, or table region. |
| Populated state | Fixtures render all required columns and row counts. | Every visible cell is derived from typed fixture fields or documented display defaults. |
| Empty state | Empty fixture response shows a compact operational empty state. | Empty state preserves filters and offers clear reset behavior without claiming live API success. |
| Error state | Error fixture response shows a recoverable table-level error. | Error copy identifies the inbox data boundary and keeps the shell usable. |
| Search | Search covers title, organization/contact fields where present, location, source URL, notes, and source label. | Search is case-insensitive and deterministic in tests. |
| Status filter | Filters by commercial workflow state. | Verification state is not conflated with status. |
| Source filter | Filters by source. | Filter options come from fixture metadata or a typed constant, not hard-coded unrelated labels. |
| Owner filter | Filters assigned and unassigned leads. | Unassigned remains an explicit selectable state. |
| Verification filter | Filters verification states. | Missing evidence, weak confidence, and contradiction states are test-covered. |
| Minimum score filter | Filters by score threshold. | Boundary values are tested. |
| Created date filter | Filters by created date or date band. | Date comparisons use deterministic fixture dates. |
| Sorting | Supports score, created date, status, source, owner, verification, and timing where practical. | Sort controls expose `aria-sort` or equivalent selected-sort state. |
| Row selection | Pointer and keyboard selection preserve selected row state. | Selected state is visible and exposed with text/state, not color alone. |
| Reset filters | Clears search, filters, and sort to the default inbox view. | Reset is keyboard reachable and has an accessible name. |

## Accessibility Contract

The first frontend implementation must add failing tests for:

| Area | Required test coverage |
|---|---|
| Landmarks | `/leads` renders inside the Phase 32 operator shell with one main landmark and an accessible lead inbox region. |
| Table semantics | Header cells, sortable headers, row headers or detail links, row selection, loading, empty, and error states use accessible semantics. |
| Filter controls | Search input, selects, segmented controls, and date/score controls have labels, selected states, and keyboard interaction. |
| Keyboard path | Operators can tab through search, filters, sort headers, rows, row actions, reset, and pagination or cursor controls. |
| Focus states | Every interactive control uses the Phase 31 visible focus treatment. |
| Status distinction | Status and verification badges include text or shape/icon labels and are distinguishable without color. |
| Responsive behavior | Narrow viewports keep filters, row labels, status, verification, and timing readable without horizontal page overlap. |

## Replaceable API Contract

Phase 33 must not implement the live `/api/leads` backend. It should prepare for Phase 35 by keeping the mocked adapter compatible with the planned endpoint.

| Future API concern | Phase 33 mocked behavior |
|---|---|
| Query params | Model `status`, `source`, `min_score`, `q`, `owner`, `verification`, `created`, `sort`, `direction`, `limit`, and `cursor` in typed params. |
| Response envelope | Use `items`, `next_cursor`, optional `total_estimate`, and `filters`. |
| Generated client | Keep table components independent from fetch details so a generated OpenAPI client can replace the fixture adapter. |
| Auth/session | Do not introduce browser-visible `API_TOKEN`. Future UI auth/session work must be explicit. |
| Raw audit | Do not load raw payload bodies in the inbox. Use `has_raw` and safe metadata only. |

## Static Asset Safety

When static assets exist, run a build-time scan equivalent to:

```bash
rg -n "API_TOKEN|DATABASE_URL|SLACK_WEBHOOK|SLACK_BOT_TOKEN|RESEND_API_KEY|CLAUDE_API_KEY|ANTHROPIC_API_KEY|GEMINI_API_KEY|OPENAI_API_KEY|OLLAMA_ENDPOINT|UI_SESSION_SECRET|postgres://|sqlite://|Bearer [A-Za-z0-9._-]+" web/dist
```

The command should return no matches. If the frontend build directory is not `web/dist`, update the path but keep the same intent.

## Phase 33 Done Criteria

Phase 33 is complete for this repository when:

| Criterion | Current result |
|---|---|
| Failing-test requirements exist for table rendering, sorting, searching, filtering, loading, empty, and error states. | Covered by the table, interaction, and accessibility contracts above. |
| Mocked fixture-only behavior is required until `/api/leads` exists. | Covered by the current scope, mock data boundary, and replaceable API contract. |
| Required columns are specified. | Covered by the table contract: status, score, source, owner, verification, created date, and timing are required. |
| Keyboard navigation and accessible filter controls are specified. | Covered by the accessibility contract. |
| API boundary is replaceable with generated or typed clients. | Covered by the mock data boundary and replaceable API contract. |
| Current backend limitations are explicit. | This repository has `GET /leads/{id}/raw` but no `GET /api/leads` route or frontend package today. |
