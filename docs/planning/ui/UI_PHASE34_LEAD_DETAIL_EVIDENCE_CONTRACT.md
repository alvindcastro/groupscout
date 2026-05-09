# Phase 34 Lead Detail And Evidence Review Contract

> Implementation contract for the future GroupScout lead detail route.
> This repository still has no checked-in frontend package, UI test harness, static UI build directory, or UI-facing `GET /api/leads/{id}` backend route. Phase 34 is complete here as a lead detail and evidence review contract. The first frontend package must convert this contract into failing detail, evidence, activity, accessibility, responsive, API-boundary, and static-asset safety tests before production UI code.

## Current Scope

Phase 34 defines `/leads/:leadId` behavior after the Phase 32 shell exists and after the Phase 33 mocked inbox can link to a lead detail route.

Do not wire the detail view to direct database reads, `POST /run`, `POST /n8n/webhook`, or the implemented raw-payload endpoint as the default data source. Until Phase 35 implements `GET /api/leads/{id}`, lead detail must use deterministic fixtures behind a replaceable adapter such as `getLeadDetail(id)`.

The backend currently has lead storage fields and `GET /leads/{id}/raw`, but it does not have structured evidence records, verification state persistence, correction storage, status transition validation, owner/snooze fields, or outreach HTTP APIs. Phase 34 UI controls must represent those gaps explicitly.

## Detail Fixture Boundary

The first frontend implementation must keep lead detail fixtures behind a narrow boundary that can later be replaced by a generated or typed `/api/leads/{id}` client.

| Boundary item | Required behavior | Acceptance |
|---|---|---|
| Fixture source | Lead detail data comes from local deterministic fixtures only. | Tests can switch between loading, populated, missing lead, missing evidence, weak confidence, contradiction, and error states without network calls. |
| Data adapter | The route calls a local adapter such as `getLeadDetail(leadId)` instead of importing fixture objects directly in page components. | Replacing the adapter with a generated `/api/leads/{id}` client preserves component props and test coverage. |
| Route param | `/leads/:leadId` passes the lead ID to the adapter and exposes a not-found state when no fixture exists. | Tests prove the route does not fetch raw payload bytes by default. |
| Response shape | The fixture mirrors `{lead, audit, outreach_summary, activity}` from `UI_API_ENDPOINTS.md`. | Raw payload bodies are excluded from this response. |
| Static safety | Browser-visible code uses relative `/api/*` paths only when a live client is later introduced. | Static asset scans fail on `API_TOKEN`, database URLs, Slack/email/LLM keys, session secrets, or real `.env` values. |

## Lead Detail Fixture Shape

Fixtures should mirror current storage fields where possible and use documented fixture-only fields for evidence review states that the backend does not persist yet.

| Field | Required in mocked detail | Notes |
|---|---|---|
| `lead.id` | Yes | Stable route target from the Phase 33 inbox. |
| `lead.title` | Yes | Detail heading and summary label. |
| `lead.source` | Yes | Source badge and evidence heading. |
| `lead.source_url` | Yes when available | Source evidence link; absence must render a missing-evidence state. |
| `lead.status` | Yes | Commercial workflow state such as `new`, `notified`, `claimed`, `contacted`, `snoozed`, `flagged`, `dismissed`, `won`, `lost`, or `no_response`. |
| `lead.priority_score` | Yes | Summary score and action prioritization cue. |
| `lead.priority_reason` | Yes when available | AI or rules reason paired with evidence. |
| `lead.rationale` | Yes when available | AI rationale block; empty rationale must be explicit. |
| `lead.suggested_outreach_timing` | Yes | Summary timing and outreach planning cue. |
| `lead.location` | Recommended | Property-fit context. |
| `lead.project_value` | Recommended | Value signal when known; unknown value is explicit. |
| `lead.project_type` | Recommended | Segment/type context. |
| `lead.estimated_crew_size` | Recommended | Room-night signal when present. |
| `lead.estimated_duration_months` | Recommended | Room-night signal when present. |
| `lead.out_of_town_crew_likely` | Recommended | Lodging-fit signal. |
| `lead.general_contractor` | Recommended | Reviewable extracted party. |
| `lead.applicant` | Optional | May contain contact data; fixtures must avoid real PII. |
| `lead.contractor` | Optional | May contain contact data; fixtures must avoid real PII. |
| `lead.notes` | Yes, nullable | Notes area and activity context. |
| `lead.created_at` / `lead.updated_at` | Yes | Summary metadata and activity ordering. |
| `audit.has_raw` | Yes | Indicates raw audit availability without loading payload bytes. |
| `audit.raw_link` | Yes when `has_raw` is true | Link target for future authenticated `/api/leads/{id}/raw`; in fixtures it must not contain raw bytes. |
| `audit.source_url` | Yes when available | UI-safe source URL metadata. |
| `audit.collector_name` | Recommended | Collector metadata when known. |
| `audit.collected_at` | Recommended | Collected timestamp when known. |
| `audit.payload_type` | Recommended | Metadata only, not body content. |
| `verification_state` | Yes | Fixture-only until persisted: `source_backed`, `missing_evidence`, `weak_confidence`, `contradiction`, or `review_requested`. |
| `evidence_items` | Yes | Fixture-only claim/evidence pairs for UI tests. |
| `outreach_summary` | Yes | Fixture-only draft/log summary until outreach APIs exist. |
| `activity` | Yes | Fixture-only status, note, correction, and outreach events until backend activity exists. |

Fixtures must include at least:

- one source-backed detail with source URL, raw audit metadata, rationale, and activity;
- one missing-evidence detail with no source URL or raw audit record;
- one weak-confidence detail where score or rationale needs review;
- one contradiction detail where a source value conflicts with an AI claim;
- one detail with no outreach activity yet;
- one detail with mocked correction controls disabled because correction storage does not exist.

## Layout Contract

The lead detail view is the review surface where operators compare model output with source evidence before acting.

| Area | Required behavior | Acceptance |
|---|---|---|
| Summary | Shows title, score, timing, room-night signal, property fit, status, owner placeholder when relevant, created and updated timestamps. | Summary data comes from typed fixtures or documented defaults; unknown fields render intentionally. |
| Source evidence | Shows source name, source URL, raw audit metadata/link, collector, collected timestamp, and source-backed facts. | Source evidence sits adjacent to the AI claim it supports. |
| AI rationale | Shows priority reason, rationale, extracted fields, uncertainty, and confidence/verification state. | Rationale is never presented as fact without nearby evidence. |
| Evidence states | Missing evidence, weak confidence, contradictions, and review-requested states are visible by default. | These states are not hidden behind collapsed panels or color-only badges. |
| Raw audit link | Provides a deliberate link or action for raw audit access without embedding payload bytes in the default detail data. | Tests fail if fixture/default detail data includes `payload`, `raw_body`, `raw_data`, or equivalent body content. |
| Outreach | Shows fixture-backed draft/status/contact/log summary capacity. | No send or log action calls live email, Slack, CRM, or outreach endpoints in Phase 34. |
| Activity | Shows fixture-backed status history, notes, outreach attempts, and reviewer correction events. | Activity ordering is deterministic in tests. |
| Status actions | Renders claim, dismiss, snooze, flag for verification, contacted, won, lost, no-response, and reopen capacity where appropriate. | Actions are disabled, mocked, or clearly fixture-only until Phase 36 defines validated transitions. |
| Corrections | Shows correction controls for extracted fields only as disabled or mocked controls. | Controls explain unavailable backend support through accessible state, not through hidden copy. |

Use dense rows, split panes, tabs, and right-rail activity patterns. Do not build a marketing hero, decorative card grid, or oversized presentation page.

## Evidence Pairing Contract

Every important AI claim must be paired with source context close enough for review.

| AI claim | Required adjacent evidence |
|---|---|
| Priority score and priority reason | Source type, source URL or missing-source state, rationale, and review/confidence state. |
| Contractor, applicant, or organization | Source field, raw audit availability, and correction control state. |
| Project type | Source field or explicit inferred/unknown state. |
| Crew size and duration | Source text or rationale excerpt plus confidence/uncertainty state. |
| Out-of-town crew likelihood | Evidence or rationale explaining the lodging signal. |
| Suggested outreach timing | Source date, created timestamp, or explicit unknown timing basis. |
| Property fit or room-night signal | Location, crew size, duration, and uncertainty cues. |

When evidence is missing, weak, or contradictory, the UI must show that before primary workflow actions so operators do not act on unsupported model output.

## State Contract

| State | Required behavior | Acceptance |
|---|---|---|
| Loading | Shell, route heading, detail skeleton, and disabled action controls remain visible. | Loading does not fetch raw audit bytes. |
| Populated | Summary, evidence, rationale, raw audit link metadata, outreach, activity, and actions render. | Required fields have deterministic formatting. |
| Not found | Unknown lead ID renders a shell-safe not-found detail state. | Navigation and recovery link back to `/leads` remain available. |
| Missing evidence | Source URL and/or raw audit metadata are absent and shown as a blocking review state. | Primary outreach/send-like actions are disabled or fixture-only. |
| Weak confidence | Rationale or score is marked weak and verification state is visible. | The route recommends review without conflating commercial `status` with verification state. |
| Contradiction | Source value and AI claim conflict. | The contradiction is visible beside the claim and source evidence. |
| Error | Adapter error renders recoverable route-level error content. | Shell and route controls remain usable. |

## Accessibility Contract

The first frontend implementation must add failing tests for:

| Area | Required test coverage |
|---|---|
| Landmarks | `/leads/:leadId` renders inside the Phase 32 operator shell with one main landmark and an accessible detail region. |
| Heading structure | Lead title, summary, evidence, AI rationale, outreach, and activity headings form a usable hierarchy. |
| Evidence semantics | Claim/evidence pairs use list, table, definition-list, or labeled region semantics that screen readers can traverse. |
| Status and verification | Commercial status and verification state are separate and not conveyed by color alone. |
| Raw audit link | The raw link has an accessible name and communicates whether it opens a preview or download route. |
| Disabled controls | Correction and mutation controls expose disabled or mocked state programmatically. |
| Keyboard path | Operators can tab through back link, summary actions, evidence links, tabs/panels, correction controls, outreach controls, activity filters, and raw audit link. |
| Focus states | Every interactive control uses the Phase 31 visible focus treatment. |
| Responsive behavior | Narrow viewports keep summary, evidence, contradiction, raw audit link, outreach, and activity readable without overlap. |

## Replaceable API Contract

Phase 34 must not implement the live `/api/leads/{id}` backend. It should prepare for Phase 35 by keeping the mocked adapter compatible with the planned endpoint.

| Future API concern | Phase 34 mocked behavior |
|---|---|
| Detail endpoint | Model a `getLeadDetail(id)` boundary that can become `GET /api/leads/{id}`. |
| Response envelope | Use `{lead, audit, outreach_summary, activity}` and exclude raw payload bodies. |
| Raw audit | Use `has_raw`, `payload_type`, `source_url`, `collector_name`, `collected_at`, and `raw_link` metadata only. |
| Raw body access | Reserve raw bytes or previews for future authenticated `GET /api/leads/{id}/raw`; do not fetch by default. |
| Status actions | Keep mutation actions mocked or disabled until `PATCH /api/leads/{id}` and Phase 36 transitions exist. |
| Corrections | Keep correction controls disabled or mocked until migration-backed correction storage exists. |
| Outreach activity | Use fixture activity until `GET/POST /api/leads/{id}/outreach` exists. |
| Auth/session | Do not introduce browser-visible `API_TOKEN`. Future UI auth/session work must be explicit. |

## Static Asset Safety

When static assets exist, run a build-time scan equivalent to:

```bash
rg -n "API_TOKEN|DATABASE_URL|SLACK_WEBHOOK|SLACK_BOT_TOKEN|RESEND_API_KEY|CLAUDE_API_KEY|ANTHROPIC_API_KEY|GEMINI_API_KEY|OPENAI_API_KEY|OLLAMA_ENDPOINT|UI_SESSION_SECRET|postgres://|sqlite://|Bearer [A-Za-z0-9._-]+" web/dist
```

The command should return no matches. If the frontend build directory is not `web/dist`, update the path but keep the same intent.

## Phase 34 Done Criteria

Phase 34 is complete for this repository when:

| Criterion | Current result |
|---|---|
| Failing-test requirements exist for summary, source evidence, AI rationale, raw audit link, outreach activity, and status actions. | Covered by the fixture, layout, state, accessibility, and prompt contracts above. |
| Source evidence is required adjacent to AI claims. | Covered by the evidence pairing contract. |
| Missing evidence, weak confidence, and contradictory data states are represented. | Covered by the fixture, layout, and state contracts. |
| Correction controls stay disabled or mocked until backend correction storage exists. | Covered by the layout and replaceable API contracts. |
| No raw payload body loads into the default detail response. | Covered by the fixture boundary, raw audit link, replaceable API, and static safety contracts. |
| Current backend limitations are explicit. | This repository has `GET /leads/{id}/raw`, lead storage, and raw audit storage, but no `GET /api/leads/{id}`, no structured evidence model, no correction storage, and no outreach HTTP API today. |
