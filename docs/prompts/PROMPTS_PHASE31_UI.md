# PROMPTS_PHASE31_UI.md - Operator UI And API TDD Prompts

> Copy-paste prompts for the GroupScout operator UI and `/api/*` backend contract work.
> Use these only when implementation starts. No production code should be written from this planning pass.
>
> Parts must be done in order unless a beads issue explicitly scopes a smaller slice.
> Every part follows Red -> Green -> Refactor.

## TDD Rules For Every Prompt

- [ ] Read `docs/planning/ui/UI_STRATEGY.md`, `docs/planning/ui/UI_DESIGN_SYSTEM.md`, `docs/planning/ui/UI_PHASE31_DESIGN_SYSTEM_CONTRACT.md`, `docs/planning/ui/UI_PHASE32_APP_SHELL_ROUTING_CONTRACT.md`, `docs/planning/ui/UI_PHASE33_MOCKED_LEAD_INBOX_CONTRACT.md`, `docs/planning/ui/UI_PHASE34_LEAD_DETAIL_EVIDENCE_CONTRACT.md`, `docs/planning/ui/UI_API_ENDPOINTS.md`, and `docs/planning/ui/UI_TDD_PHASE_PLAN.md`.
- [ ] Read existing tests near the target files before adding new tests.
- [ ] Write the smallest failing test first.
- [ ] Run the narrow test and capture the expected red failure.
- [ ] Implement only enough production code to pass.
- [ ] Run the narrow test again and capture green.
- [ ] Run the broader relevant suite.
- [ ] Refactor only while green.
- [ ] Do not call live Slack, email, LLM, browser, CRM, public websites, or paid APIs from unit tests.
- [ ] Use fakes, fixtures, `httptest`, component test harnesses, and deterministic data.

## Phase 31 - UI Design System Adaptation

```text
Task 31-A1 - Design tokens and primitives

Context:
- The UI design reference is `/mnt/c/Users/alvin/WebstormProjects/groupscout-ui/DESIGN.md`.
- GroupScout should use the dense docs/product surface language, not marketing hero patterns.
- The implementation contract is `docs/planning/ui/UI_PHASE31_DESIGN_SYSTEM_CONTRACT.md`.
- If this repository still has no checked-in frontend package, do not invent untested UI code; create the frontend harness first, then make the contract tests fail before implementing components.

TDD steps:
1. Write failing tests for token availability: typography, colors, spacing, radii, focus states, and semantic statuses.
2. Write failing component tests for Button, Badge, Input, Tabs, Table, EvidenceBlock, and raw-data code blocks.
3. Write failing accessibility tests for labels, keyboard tab order, focus-visible treatment, selected tab state, icon-button names, and status labels that do not rely on color alone.
4. Write a failing no-hero-pattern test proving the operator workspace does not render marketing hero classes, atmospheric gradients, cloud/rocket imagery, decorative orbs, or oversized hero display type.
5. Run the narrow tests and confirm the expected red failures.
6. Implement the smallest token/component layer needed to pass.
7. Verify no browser-visible config exposes secrets with a static asset scan after build.

Acceptance:
- Components are compact and operations-friendly.
- Evidence and code-like content can use mono styling.
- Mint green is reserved for active, focused, verified, or confirmation states.
- Browser-visible assets do not contain `API_TOKEN`, database URLs, Slack/email/LLM provider keys, session secrets, or real `.env` values.
- Marketing hero patterns remain out of the operator workspace.
```

## Phase 32 - App Shell And Routing

```text
Task 32-A1 - Operator shell

Context:
- The first UI screen is the operator workspace, not a landing page.
- The app shell and route contract is `docs/planning/ui/UI_PHASE32_APP_SHELL_ROUTING_CONTRACT.md`.
- The shell needs navigation for Today, Leads, Verification, Pipeline, and Settings.

TDD steps:
1. Write failing route tests for each primary screen.
2. Write failing layout tests for left nav, top utility bar, main content, and right rail.
3. Write failing responsive tests for narrow viewport collapse.
4. Implement only enough shell/routing code to pass.
5. Run component tests and build.

Acceptance:
- Routes render predictable landmarks.
- Layout does not rely on API data.
- The shell supports a future detail/evidence rail.
```

## Phase 33 - Mocked Lead Inbox

> Detailed Phase 33 prompt pack: [PROMPTS_PHASE33_UI.md](./PROMPTS_PHASE33_UI.md).

```text
Task 33-A1 - Lead inbox with fixture data

Context:
- `/api/leads` is planned but not implemented yet.
- The lead inbox can be designed against typed fixtures first.
- The implementation contract is `docs/planning/ui/UI_PHASE33_MOCKED_LEAD_INBOX_CONTRACT.md`.

TDD steps:
1. Write failing tests for loading, empty, error, and populated states.
2. Write failing tests for search, status filter, source filter, score sorting, and row selection.
3. Write failing accessibility tests for filter controls and row actions.
4. Implement the table using fixture-backed data only.
5. Keep the data access boundary replaceable by an API client.

Acceptance:
- The table is dense and scan-friendly.
- Columns include score, title, source, status, owner, verification, timing, and created date where data exists.
- No backend token or database access appears in browser code.
```

## Phase 34 - Lead Detail And Evidence Review

> Detailed Phase 34 prompt pack: [PROMPTS_PHASE34_UI.md](./PROMPTS_PHASE34_UI.md).

```text
Task 34-A1 - Lead detail and source evidence

Context:
- AI output must be reviewable beside source evidence.
- Raw audit payloads should not load by default in the detail response.
- The implementation contract is `docs/planning/ui/UI_PHASE34_LEAD_DETAIL_EVIDENCE_CONTRACT.md`.

TDD steps:
1. Write failing tests for summary, score, timing, source metadata, AI rationale, raw audit link, notes, and activity sections.
2. Write failing tests for missing evidence, weak confidence, and contradictory fields.
3. Write failing tests showing correction controls disabled or mocked when backend support is absent.
4. Implement the smallest detail view using fixture data.
5. Verify evidence stays adjacent to AI claims.

Acceptance:
- Operators can see what the model claimed and what source evidence supports it.
- Missing or weak evidence is visible.
- Detail layout remains usable without live API data.
```

## Phase 35 - UI API Contracts

```text
Task 35-A1 - Lead read API

Context:
- Current backend has `GET /leads/{id}/raw` but no UI-facing `/api/leads`.
- Browser routes should use same-origin `/api/*`.

TDD steps:
1. Write failing storage tests for filtered lead listing: status, source, minimum score, search query, limit, and cursor.
2. Write failing HTTP tests for `GET /api/leads`: JSON shape, filters, pagination, empty result, and invalid query.
3. Implement only the storage/list and handler behavior needed to pass.
4. Write failing HTTP tests for `GET /api/leads/{id}`: found, not found, audit metadata included, raw payload excluded.
5. Implement detail handler behavior.
6. Update OpenAPI only after tests define the contract.

Acceptance:
- `GET /api/leads` returns `{items, next_cursor, total_estimate?, filters}`.
- `GET /api/leads/{id}` returns lead detail plus audit metadata.
- Raw payload body is not included in default detail.
```

```text
Task 35-A2 - Lead update API

Context:
- Current schema supports status and notes.
- Owner, snooze, verification state, and corrections need schema decisions before live support.

TDD steps:
1. Write failing HTTP tests for `PATCH /api/leads/{id}` with status and notes.
2. Write failing tests rejecting unsafe updates to source-backed extracted fields.
3. Write failing tests for invalid status transitions if the state machine is in scope.
4. Implement only status and notes if no migration is included.
5. File beads follow-ups for owner, snooze, verification, or corrections if needed.

Acceptance:
- Safe updates work.
- Unsupported fields are rejected clearly.
- Source-backed extraction is not silently overwritten.
```

```text
Task 35-A3 - Raw evidence UI alias

Context:
- Existing `GET /leads/{id}/raw` returns raw bytes and lacks a UI session boundary.

TDD steps:
1. Write failing tests for `GET /api/leads/{id}/raw`: found, missing lead, missing raw input, invalid raw ID, and content type.
2. Write failing tests for the chosen UI auth/session boundary.
3. Decide preview JSON envelope versus raw-byte download for this endpoint.
4. Implement the smallest authenticated alias over the audit store.

Acceptance:
- Operators can reach raw evidence safely from lead detail.
- Missing audit data returns a clear 404-style response.
- Browser auth/session boundary is explicit.
```

## Phase 36 - Outreach And Lead State Actions

```text
Task 36-A1 - Outreach log API

Context:
- The database has `outreach_log`, but UI storage and HTTP methods are not complete.

TDD steps:
1. Write failing storage tests for inserting and listing outreach events.
2. Write failing HTTP tests for `GET /api/leads/{id}/outreach`.
3. Write failing HTTP tests for `POST /api/leads/{id}/outreach`: validation, insert, response shape, and optional lead status transition.
4. Implement storage and handlers only after red tests are observed.

Acceptance:
- Operators can view and log outreach history.
- Invalid channel/outcome values are rejected.
- Logged outreach can transition a lead only through allowed status rules.
```

```text
Task 36-A2 - Lead state actions

Context:
- UI actions include claim, dismiss, snooze, flag, contacted, won, lost, no-response, follow-up, and reopen.
- Verification state may become separate from commercial workflow status.

TDD steps:
1. Write failing tests for allowed transitions.
2. Write failing tests for rejected transitions.
3. Write failing UI tests for disabled or error states.
4. Implement transition validation in the narrowest layer in scope.
5. Keep migration-backed fields out of scope unless the task includes those migrations.

Acceptance:
- Invalid transitions fail closed.
- UI and API agree on state names.
- Commercial and verification state are not conflated without a documented decision.
```

## Phase 37 - Pipeline Runs, Stats, And System Health

```text
Task 37-A1 - Pipeline runs API

Context:
- Existing `POST /run` blocks until the pipeline completes.
- Browser UI needs an async run trigger and run history.

TDD steps:
1. Write failing tests proving `POST /api/pipeline/runs` returns quickly with a `run_id`.
2. Write failing tests for run status, completion, error capture, and history ordering.
3. Add run persistence or document an explicitly dev-only in-memory tracker.
4. Implement the smallest async wrapper around the existing pipeline path.

Acceptance:
- Browser requests do not block for the whole pipeline.
- Run history is queryable.
- Errors are visible without requiring log access.
```

```text
Task 37-A2 - Stats and system API

Context:
- The UI needs a compact system panel and simple lead rollups.
- Browser UI should not parse Prometheus output directly.

TDD steps:
1. Write failing storage tests for counts by status, source, score band, owner, and week where schema supports it.
2. Write failing HTTP tests for `GET /api/stats` window and grouping behavior.
3. Write failing HTTP tests for `GET /api/system` healthy and degraded states.
4. Implement simple DB-backed stats and UI-friendly health summary.

Acceptance:
- `/api/stats` can power Today and analytics summaries.
- `/api/system` distinguishes healthy, degraded, and unavailable dependencies.
- Prometheus remains for observability tooling, not direct UI rendering.
```

## Phase 38 - Docker Runtime And End-To-End Smoke

```text
Task 38-A1 - UI runtime smoke

Context:
- The separate UI production container serves static assets and proxies `/api/*`.
- Current runbooks distinguish backend `404` from proxy `502`.

TDD steps:
1. Write failing smoke checks for `/`, `/healthz`, static assets, and `/api/*` proxy behavior.
2. Add Playwright smoke for first viewport, lead inbox, lead detail, and responsive navigation.
3. Implement runtime wiring only after the smoke checks fail for the expected reason.
4. Verify static assets contain no secrets.

Acceptance:
- UI production image serves the app and health endpoint.
- `/api/*` reaches the backend or reports a clear proxy failure.
- Smoke tests cover both desktop and mobile layouts.
```

## Final Evidence Template

```text
Files changed:
Tests added/updated:
Red evidence:
Green evidence:
Broader command:
Behavior added:
Known residual risk:
Follow-up beads:
```
